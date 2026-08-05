# mrtutor backend — architecture review

## 1. Scope and method

Reviewed at commit `9dddb6e`, against the working tree (which contains an in-progress move of `cmd/api/server.go` → `cmd/api/http/server.go`).

- **Size:** 31 Go files, 3,075 lines, 19 commits.
- **Toolchain:** Go 1.26.3. Three direct dependencies: `mattn/go-sqlite3`, `pressly/goose/v3`, `golang.org/x/crypto`. sqlc v1.31.1 isolated in `go.tool.mod`.
- **Features implemented end-to-end:** one — auth (login, register).
- **Read in full:** every non-generated `.go` file, `sqlc.yml`, `Taskfile.yml`, both migrations, `go.mod`.

Stated goal: **scalable but flexible, idiomatic Go 1.26+, built on dependency injection.** Everything below is measured against that.

## 2. Verdict

The layering is right and the wiring is wrong.

The domain core is properly isolated with consumer-side ports, the error model is better than most production Go I have read, and the SQLite configuration is correct in the ways that are easy to get wrong. None of that needs to change.

What does not hold up is everything in the composition layer: config is package-level global state that library packages reach into directly, the composition root requires editing three shared files per feature, there is no middleware layer at all, and the transaction abstraction was designed but never connected — which leaves a live data-integrity bug in `Register`.

There are also **19 concrete defects**, six of them security-relevant, including a user-enumeration oracle on the login endpoint and startup failures that exit with status 0.

Critically: **the auth feature is half-built.** Sessions are minted and never verified. `SessionStore` has exactly one method, `Create`. Nothing in the codebase can authenticate a request. That is missing functionality on the critical path, not a refactor.

---

## 3. What is already right — preserve these through any refactor

Listed because a restructuring must not regress them, and because several are load-bearing for the recommendation in §7.

**Consumer-side port interfaces.** `auth/storage.go` declares `PrincipalStore` / `SessionStore` in the package that consumes them; `sqlite/auth.go:40,59` asserts conformance producer-side. This is the single most important correct decision in the codebase, and it is why `auth` imports zero infrastructure.

**Dependency direction.** `sqlite` → `auth`, never the reverse. `auth` imports only `validation`, `errs`, and `bcrypt`.

**The `errs` model.** Two orthogonal axes in one small type: `Unwrap() → kind` drives HTTP status via `errors.Is`, while `Code()`/`Public()` carry a stable client contract. `writeError` **fails closed** — it starts from `{"code":"internal","message":"internal error"}` and only overrides when an error explicitly opts into being public. That default is correct and rarer than it should be.

**`validation.Errors.Is(errs.Invalid)`** (`validation/errors.go:81`). Every validation error maps to 400 with per-field detail and no plumbing at the call site.

**`wrap[In, Out]`.** `auth.Service.Login` and `.Register` match `fn func(context.Context, In) (Out, error)` exactly, so the domain method *is* the handler body — zero adapter boilerplate, and error handling cannot drift per endpoint. This is the best idea in the codebase.

**`writeJSON` buffers before committing** (`codec.go:43-53`): encode into a `bytes.Buffer`, only then write status + body, so an encode failure leaves the response uncommitted and `writeError` can still substitute a 5xx. `recordingWriter` (`helpers_test.go:20-25`) exists specifically to pin this, because `httptest.ResponseRecorder` cannot distinguish "wrote 200" from "wrote nothing". Most codebases get this wrong and never find out.

**SQLite configuration.** Separate `W`/`R` handles, `SetMaxOpenConns(1)` on the writer, WAL, `_txlock=immediate` on write and `deferred` on read. This is *the* correct pattern for SQLite under concurrency, and it was clearly deliberate.

**The `principals` view over `users`** (`0001_create_principal.sql:13`). Decouples domain vocabulary from physical schema at zero runtime cost.

**Migration reversibility testing.** `sqlite/migrations_test.go` drives all migrations up *and down*. Most projects never test `Down`.

**`go.tool.mod`.** Isolating sqlc's 45-dependency tree keeps the application's `go.sum` at 14 lines. Correct use of Go 1.24+ tool modules.

**Stdlib-only posture.** `net/http.ServeMux` with Go 1.22+ method patterns, hand-written fakes, no testify, no router, no DI framework. This is the right call and nothing in this review requires changing it.

---

## 4. Code organization problems

### 4.1 `config` globals are the one hole in the DI story — highest structural priority

`config/*.go` declares package-level `var`s evaluated at package-init time:

```go
// config/db.go
var (
	ReadPoolSize = getInt("READ_POOL_SIZE", 4)
	DatabaseFile = getString("DATABASE_FILE", "data.db")
)
```

Everything else in the codebase is constructor-injected. Config is not — and library packages reach into it directly:

- `sqlite/db.go:133,140-141` reads `config.ReadPoolSize`
- ~~`cmd/api/http/server.go:29` reads `config.LogLevel`~~ — already resolved independently by `55c5e77`, ahead of this fix; `cmd/api/http` took an options struct and stopped importing `config` before Phase 2 landed.

Four consequences:

1. **`sqlite` has a hard compile-time dependency on `config`.** The persistence layer cannot be used with a different pool size, and cannot be imported by anything unwilling to accept the env-var contract.
2. **Untestable.** Because the values are bound at import time, `t.Setenv` has no effect. There is no way to test `Open` at pool size 1, or the server at a different log level.
3. **Parse failures are silent.** `READ_POOL_SIZE=abc` yields 4 with no error. `LOG_LEVEL` must be numeric (`-4` for debug, per `Taskfile.yml:11`), so the natural `LOG_LEVEL=DEBUG` silently becomes Info.
4. **No validation and no single failure point.** A misconfigured deployment starts happily and misbehaves later.

This is the fix that unblocks most of the others.

**Status: resolved.** `config.Load(lookup) (Config, error)` replaces the package vars, accumulating every parse/validation failure into a `validation.Errors` keyed by env var name instead of defaulting silently — closing consequences 3 and 4 (defect **#12**) at once. `sqlite.Open` now takes a `sqlite.Options{Path, Logger, ReadPoolSize}` (defaulting `ReadPoolSize` to 4 internally), closing consequences 1 and 2 — `go list -deps ./sqlite ./cmd/api/http | grep config` returns nothing. See `docs/architecture-tasks.md` Phase 2.

### 4.2 The composition root does not scale

```go
// cmd/api/service.go — the entire DI container
type Services struct{ Auth auth.Service }

func createServices(db *sqlite.DB) Services {
	principalStore := sqlite.NewPrincipalStore(db)
	sessionStore := sqlite.NewSessionStore(db)
	auth := auth.NewService(principalStore, sessionStore)   // shadows the package name
	return Services{Auth: auth}
}
```

Adding a feature requires editing `cmd/api/service.go`, `cmd/api/routes.go`, **and** adding a file to `cmd/api/http/`. Three shared files per feature. No feature is ever self-contained or independently reviewable.

### 4.3 `sqlite/` mixes infrastructure with adapters

`sqlite/` holds connection management, migrations, error translation, **and** the auth adapter. At ten features it is `sqlite/{auth,courses,lessons,scheduling,billing}.go` — one package importing every domain package, with one flat namespace holding `PrincipalStore`, `CourseStore`, `principalFromDB`, `courseFromDB`, and so on. Two costs: `go test ./sqlite` runs every feature's adapter tests and recompiles on any domain change, and no domain package can ever import `sqlite` without a cycle. Today that last constraint is invisible; it is being accepted silently.

### 4.4 `sqlite/internal` — a name mismatch, and a hard block on any future split

The directory is `internal`, the package is `gen`, so every import needs an alias: `gen "github.com/.../backend/sqlite/internal"`.

More importantly — and this is the decisive fact for §7 — Go's internal rule means `backend/sqlite/internal/...` is importable **only from within `backend/sqlite/...`**. The encapsulation is real and correct. But it also means a per-feature storage adapter package **cannot compile against `gen`** until codegen is relocated. The current directory layout does not merely favour the layered design; it enforces it.

### 4.5 `authHandler` blocks HTTP-level unit testing

```go
type authHandler struct {            // unexported…
	Login    http.HandlerFunc
	Register http.HandlerFunc
}
func NewAuthHandler(svc auth.Service, logger *slog.Logger) authHandler   // …from an exported ctor
```

An exported constructor returning an unexported type gives callers a value they cannot name in a signature or struct field. And `NewAuthHandler` takes `auth.Service` as a **concrete value** — there is no seam for a fake, which is precisely why `cmd/api/http/auth_test.go` is a **0-byte file**. The handlers' cookie logic, status codes, and error mapping have no unit coverage at all; the only exercise they get is an integration test requiring a real database.

**Status: resolved (Phase 3), with one correction.** The handler moved to `auth/authhttp` (`NewHandler` returning the now-exported `Handler`) behind a consumer-side `authhttp.Service` interface, satisfied structurally by `auth.Service` — no adapter. `auth/authhttp/handler_test.go` covers cookie attributes, status codes, and error mapping against a fake `Service`. Correction: `cmd/api/http/auth_test.go` was never a 0-byte file — the path does not exist on disk at all, here or in `docs/architecture-tasks.md` §0.3's test-gaps line, which repeats the same claim.

### 4.6 Domain models carry transport concerns

`auth/models.go` puts JSON tags on domain types. `Session.TokenHash [32]byte` is tagged `json:"id"` — and `[32]byte` marshals as an array of 32 integers, so marshalling a `Session` would put the session hash on the wire. `Principal.PasswordHash` depends on `json:"-"` to stay private, which means the domain model doubles as the wire model and every field added later is exposed by default. The `encode` parameter of `wrap` is already the right seam for response DTOs.

### 4.7 Smaller points

- **`isShuttingDown atomic.Bool`** is a package-level global in `main` (`main.go:18`) read by `healthHandler` (`routes.go:22`) — process state shared through a global rather than injected. **Status: resolved (Phase 3).** Both the global and `healthHandler`/`GET /health` are deleted; nothing wrote the flag by the time this landed (a `Readiness` value introduced in Phase 1 had already superseded it), so `/health` was unconditionally 200 — a probe that cannot fail is worse than no probe.
- **Liveness and readiness are conflated** on one `GET /health`. **Status: resolved (Phase 1/3).** Split into `/livez` and `/readyz`; the latter was mounted at `/healthz` until Phase 3 renamed it, and its handler had an inverted branch (503 while healthy, 200 while draining) until Phase 3 fixed that too — see the new defect recorded in `docs/architecture-tasks.md`'s Phase 3 section.
- **`cmd/api/http` is named `http`**, forcing an alias at every import site — and the tree already contains two spellings, `ehttp` (`main.go:13`) and `handlers` (`test/integration/auth_test.go:15`). **Status: resolved (Phase 3).** Renamed to `httpx`; a third spelling this bullet didn't catch, `httpstdlib` (aliasing `net/http` in `server_test.go` to avoid the collision), is also gone. No import site of the HTTP kit or `auth/authhttp` uses an alias.
- **Dead code:** `auth.Stores` and `auth.UnitOfWork` (zero implementations), `sqlite.DB.InTx` (zero callers), `ok`/`created`/`noContent` (tests only), `Session.RevokedAt` and `sessions.revoked_at` (never read or written). **Partial update:** `ok`/`created`/`noContent` (now exported `OK`/`Created`/`NoContent`) are no longer tests-only — `Created` is live in `authhttp.NewHandler`'s Register path since Phase 3. The rest is unchanged and awaits Phase 6/7.

---

## 5. Scalability issues

### 5.1 There is no middleware layer — the largest structural gap

Grep for `func(http.Handler) http.Handler` returns zero hits outside tests. There is nowhere to hang:

- request ID / correlation ID
- panic recovery — today a handler panic is caught by `net/http` per connection: no structured log, no JSON 500, connection dropped
- access logging — **successful requests are never logged at all**; logging happens only inside `writeError`
- per-request timeouts and body size limits (`bodyDecoder` reads an unbounded body)
- CORS — mandatory the moment a browser frontend exists
- **session authentication**

This is the difference between an app with two endpoints and an app that can grow. It needs no dependency.

### 5.2 Sessions are write-only — auth is half-built

`SessionStore` has one method, `Create`. There is no `GetActive`, `Revoke`, or `DeleteExpired`, and `PrincipalStore` has no `GetByID`. Therefore:

- **No request can be authenticated.** Tokens are minted and never consumed.
- **No logout.** `revoked_at` exists in the schema and is never touched.
- **No server-side expiry.** `sessionCookieMaxAge` is a *cookie* attribute, which is client-controlled — a stolen token is valid forever.
- No column or index supports expiry cleanup.

### 5.3 `Register` is not atomic — the dead `UnitOfWork` has a real cost

`auth.UnitOfWork` and `auth.Stores` are declared (`auth/storage.go:5-7,21-23`) with zero implementations. `sqlite.DB.InTx` and the generated `Queries.WithTx` both exist with zero callers. The halves were never joined.

The consequence: `Service.Register` (`service.go:108-139`) creates the principal, then creates the session in a **separate transaction**. If the session insert fails, the client gets a 500 while the account already exists — and retrying returns 409 Conflict. The user is permanently wedged out of that username and email.

The blocker is structural. The stores bind to `db.W` / `db.R` at each call site (`gen.New(p.db.W)`), so they cannot join a caller's transaction. `auth.Stores` also lacks a `Sessions` field, so it could not express this use case even if implemented.

There is a related trap waiting in the current `InTx` signature. It hands out a bare `*sql.Tx`, which leaves a store free to keep reading from `db.R`. Under WAL, `db.R` is a *different physical connection* and cannot see uncommitted rows — so a read-your-own-write inside a transaction would silently return stale data. Whatever replaces this must make the correct handle the only reachable one.

### 5.4 `main()` has no testable seam, and failures exit 0

```go
db, err := sqlite.Open(rootCtx, config.DatabaseFile, logger)
if err != nil {
	logger.Error("Failed to open database", "error", err)
	return          // ← exit status 0
}
```

A database that will not open, or migrations that fail, **terminate the process successfully**. Kubernetes, systemd, and CI all read exit codes: this looks like a clean shutdown, so nothing restarts and nothing alerts. There is also no `run(ctx) error`, so the entire startup and shutdown sequence is untestable.

### 5.5 `Server.Start()` swallows the listen error → zombie process

```go
func (s *Server) Start() {
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error", "error", err)
		}
	}()
}
```

If the port is in use this logs once and the goroutine exits. `main` then blocks on `<-rootCtx.Done()` forever: a live process serving nothing. The root cause is that `ListenAndServe` binds *inside* the goroutine, so the failure cannot be returned. (Also `err != http.ErrServerClosed` should be `errors.Is`.)

### 5.6 The HTTP server sets no timeouts

`NewServer` sets `Addr`, `Handler`, `ErrorLog`, and `BaseContext` — and nothing else. `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` are all zero, meaning **no limit**. A handful of slow-header connections can hold goroutines indefinitely (Slowloris). `ReadHeaderTimeout` is the one that matters most and costs one line.

### 5.7 Observability stops at logging

No metrics, no tracing, no `net/http/pprof`, no request IDs — so there is no way to correlate the log lines belonging to one request. A request-scoped logger has to come from the context, which requires middleware (§5.1).

### 5.8 Delivery gaps

- **No CI, no Dockerfile, no lint, no vet, no build task, no coverage.** `Taskfile.yml` has `run`, `test`, `unit-test`, `sqlc`.
- **No API contract** — no OpenAPI spec, no generated client. There is no frontend yet and the repo root has an obvious empty slot for one. Choosing the contract mechanism before the frontend exists is far cheaper than after.
- **cgo is now an accepted trade-off** (decision made: stay on `mattn/go-sqlite3`). Worth recording the consequences so they are not rediscovered: no `CGO_ENABLED=0` static binary, a C toolchain required in the build image, and cross-compilation needs a cross C toolchain. Budget for a multi-stage Dockerfile on a glibc base rather than `scratch`.
- **SQLite write ceiling.** The single-writer design is correct, but note that **`Login` is a write** — it inserts a session. The authentication path contends on the one write connection and cannot be served by a read replica.

---

## 6. Defect inventory

19 items, ordered by severity. Each is independently fixable without any restructuring.

| # | Location | Defect |
|---|---|---|
| 1 | `auth/service.go:34-37` | **User enumeration.** `Login` returns the store's not-found error verbatim, so an unknown username yields **404** while a wrong password yields **401** — a trivial account oracle. The early return also skips bcrypt, adding a timing oracle. Map not-found → `ErrInvalidCredentials` and compare against a fixed dummy hash on the miss path. |
| 2 | `sqlite/errors.go:17-19` | **`translateSQLError` discards its own `notFound` argument**, returning bare `errs.NotFound`. So the response is 404 with body `{"code":"internal","message":"internal error"}` — `statusFor` matches the kind, but `errors.As` finds no `Code()`/`Public()`. `ErrPrincipalNotFound` and `ErrSessionNotFound` are effectively dead. The conflict branch does this correctly, and the integration test asserts only the conflict path, so the asymmetry is untested. |
| 3 | `cmd/api/main.go:30,40` | **Startup failures exit 0** (§5.4). |
| 4 | `cmd/api/http/server.go:39-46` | **Listen error swallowed** → live process serving nothing (§5.5). |
| 5 | `auth/service.go:108-139` | **`Register` is not atomic** → orphaned accounts that cannot be retried (§5.3). |
| 6 | `cmd/api/http/server.go:26-33` | **No server timeouts** — Slowloris exposure (§5.6). |
| 7 | `cmd/api/http/auth.go:25-29,39-44` | **Cookie hardening.** Neither cookie sets `Secure`, `SameSite`, or `Path`. Login sets no `Expires`/`MaxAge` (browser-session cookie) while Register sets 7 days — inconsistent lifetime for the same credential. **Status: resolved (Phase 3).** Also fixed in the same pass: `MaxAge` was computed as `int(sessionCookieMaxAge)` on a `time.Duration`, i.e. nanoseconds (604800000000000) fed to a seconds field — a defect this table never caught. `Secure` is now an `authhttp.Config` field sourced from `APP_MODE`, not hardcoded, so local dev over plain HTTP still receives the cookie. |
| 8 | `cmd/api/http/codec.go:15-21` | **No `http.MaxBytesReader`** — request bodies are unbounded. Still open; scheduled for Phase 4's middleware set alongside the other body-size and timeout concerns, not fixed piecemeal. |
| 9 | `sqlite/auth.go:35` | **PII in logs.** The op string is `"get principal by token "+usernameOrEmail`, and `writeError` logs the error on every rejected request — so every failed login writes the submitted username or email to the log. |
| 10 | `auth/service_test.go:83,93` | **Broken test fixture.** `mockPrincipalStore.Create` has a **value receiver** but does `m.count += 1`; the increment is discarded, so every principal gets `ID = 1` and each call overwrites `db[1]`. It passes only because no test seeds two principals into one store. The same fake returns the *correct* `auth.ErrPrincipalNotFound` — which is exactly why defect #2 was never caught by a unit test. |
| 11 | `cmd/api/http/errors.go:94-98` | **Empty or truncated body → 500 instead of 400.** `isJsonDecodingError` matches only `*json.SyntaxError` and `*json.UnmarshalTypeError`, not `io.EOF` / `io.ErrUnexpectedEOF`. `curl -X POST /api/v1/auth/login` with no body returns 500 today. **Status: resolved** (landed ahead of Phase 3, per `docs/architecture-tasks.md`). The comment pointer to `errors_test.go:39-56` is now stale: those two cases named `"(known gap: should be 400, is 500)"` still asserted the *pre-fix* 500, leaving `go test ./...` red until Phase 3 corrected them to the 400/`invalid.json` the fix already produced. |
| 12 | `config/loaders.go:18,28` | **Parse errors swallowed** — malformed values silently become defaults (§4.1). |
| 13 | `sqlite/auth.go:54` | `SessionStore.Create` passes `auth.ErrSessionNotFound` as the *notFound* sentinel and `nil` for conflict — backwards for an `INSERT`. A session PK collision degrades to a 500. |
| 14 | `sqlite/errors.go:28` | Fallback uses `%v`, not `%w`, so driver errors are unwrappable and `errors.Is` is broken for callers. `writeError` already refuses to expose non-public messages, so `%w` is safe here and strictly better for logs. |
| 15 | `sqlite/db.go:56` | Rollback failure formatted with `%v`, losing the chain. `errors.Join` preserves `errors.Is` on both errors. |
| 16 | `sqlite/errors.go:23` | Conflict detection matches the coarse `sqlite3.ErrConstraint`, so UNIQUE, FK, and CHECK violations are indistinguishable — a duplicate email reports "principal already exists" without naming the field. The extended codes (`ErrConstraintUnique`, `ErrConstraintForeignKey`) are the discriminators. |
| 17 | `cmd/api/http/wrap.go:47` | **Validation fails open.** The `Validable` check is a runtime type assertion on `In`, so a `Validate` declared on `*T` is silently skipped when `In` is `T`. Documented and tested — but a missing validator should be a compile error, not a no-op. **Status: resolved (Phase 3).** `Wrap[In Validable, Out any]` constrains `In` directly; a type whose `Validate` is pointer-receiver-only now fails to compile against `Wrap`, verified via `go vet`. `WrapUnvalidated`/`WrapNoInput` cover inputs with nothing to validate. |
| 18 | `sqlite/auth.go:73-81` | `principalFromDB` silently drops `UpdatedAt`; nothing writes the column either. |
| 19 | `cmd/api/http/auth.go:45` | Register returns **200**, not 201, and `RegisterOut.Principal` is computed then discarded. The `created` helper exists and is unused. **Status: resolved (Phase 3).** Returns 201 with `out.Principal` as the body (`PasswordHash` stays off the wire via its existing `json:"-"` tag); `RegisterOut` itself is never encoded directly, since it carries no JSON tags and would otherwise put the session token in the body next to the cookie. |

**Lower-severity polish.** `bodyDecoder` ignores trailing content (`{"a":1}{"b":2}` is accepted — check `dec.More()`); no `Content-Type` check, so non-JSON should be 415; `validation.Email` returns the raw `mail.ParseAddress` message (`mail: missing '@' or angle-addr`), which is client-visible in the `fields` array and stylistically inconsistent with `"is required"` / `"must not be blank"`; `validation.MinLength[T ~string | ~[]any]` cannot accept `[]string` or any other typed slice, so it is string-only in practice; `errs.Domain` returns an unexported `*domainError` that callers cannot name; `cmd/api/main.go:57` shadows `cancel` and pairs it with an earlier explicit `cancel()`.

**Test coverage gaps.** ~~`cmd/api/http/auth_test.go` is empty.~~ **Correction:** that path was never a 0-byte file; it does not exist on disk. **Status: resolved (Phase 3)** for the handler-level gap specifically — `auth/authhttp/handler_test.go` now covers cookie attributes, status codes, and error mapping against a fake `Service`, and the integration suite's successful-login case now asserts the cookie's `HttpOnly`/`Path`/`MaxAge` (closing the "never asserts the cookie" item below). Still open: the integration test calls handlers directly rather than through the mux, so `RegisterRoutes`, `StripPrefix`, and method routing are **never exercised**; `TestAuth` subtests share one database with order dependence and would break under `t.Parallel()`; `sqlite/db_test.go` touches disk without a `-short` gate.

### Found after the review, while implementing Phase 3

Not in the 19-item inventory above — these surfaced only once the Phase 3 file moves were underway, and are recorded here rather than folded into the numbered list, so "19 items" stays an accurate description of what this review originally found.

- **Readiness handler inverted** (`cmd/api/http/readiness.go`, now `httpx/readiness.go`). `Handler()` read `if r.Ready() { 503 }` — serving 503 while healthy and 200 while draining, the exact opposite of a readiness probe's contract. Zero test coverage before Phase 3 added `httpx/readiness_test.go`.
- **Two "resolved" defects had reopened but untested code paths.** #7's cookie hardening and #19's Register status code were both marked done in `docs/architecture-tasks.md` before Phase 3, but neither had actually landed in the code — #7's `MaxAge` conversion bug (see the table row above) and #19's 204-vs-200-vs-201 three-way disagreement between the code, the doc, and the integration test were only caught because Phase 3 touched those files again. The standing lesson, recorded in `docs/architecture-tasks.md`'s Phase 0.3 notes: a fix must update the test that documented the bug, in the same commit — two packages' test suites were left red for a full phase because the Content-Type gate and defect #11's fix landed without that.

---

## 7. The architecture decision

### 7.1 The four facts that constrain the choice

Established by reading the tree, not by preference:

1. **`sqlite/internal` currently makes the layered design mandatory.** Go's internal rule restricts `backend/sqlite/internal/...` to importers within `backend/sqlite/...`. A per-feature storage adapter cannot compile against `gen` until codegen relocates. Any split therefore starts with a `sqlc.yml` change — for *any* option that moves adapters.
2. **Migrations must stay central.** goose requires one globally ordered version sequence. No feature can fully own its schema. Some shared storage package always survives.
3. **`translateSQLError` must be shared.** Every feature's adapter needs it. Duplicating it per feature is strictly worse.
4. **Cross-feature transactions are likely and cheap here.** This is a single-process monolith on one SQLite file. In a tutoring domain, "book a lesson" plausibly touches lessons, scheduling, and credits atomically. Any transaction abstraction must compose across features.

### 7.2 The options

**A — Keep layered, fix seams only.** Leave packages alone; fix the composition root, config, and middleware.

**B — Split storage adapters only.** `auth/authsqlite/`, `sqlite/` reduced to infra; handlers stay together with the HTTP helpers in one package.

**C — Feature-first.** `auth/` domain, `auth/authhttp/`, `auth/authsqlite/`; a domain-free `httpx/` kit; `sqlite/` as infra leaf.

### 7.3 Analysis

**A is genuinely defensible and I am rejecting it on one specific ground.** At 3,000 lines and one feature, "leave it alone" is often the right answer, and premature package proliferation is a well-known Go failure mode. But fact 1 cuts the other way: the cost of moving *later* is not constant. Every feature added under A puts another adapter in `sqlite/` and another handler in `cmd/api/http/`, and every one of those must be moved if the layout ever changes. The move is cheapest now, at one feature, and it is mechanical: `git mv`, export a few identifiers, add a `sqlc.yml` block. A is the correct answer for a codebase that will stay at two or three features. It is the wrong answer given a stated goal of scalability.

**B looks like the moderate option and does not survive scrutiny.** Its whole appeal is that `wrap`, `bodyDecoder`, `writeError`, and `statusFor` stay **unexported** and therefore free to evolve — which matters, because `wrap`'s `Validable` design is unresolved (defect #17). But that benefit only holds while the kit and the handlers occupy the *same package*. And fusing them is precisely what makes that package unbounded: it ends up holding `wrap` + `codec` + `errors` + `router` + middleware + one handler file per feature. The moment you separate the kit from the handlers — which you must, to stop the growth — the kit has to be exported anyway, and B's only advantage evaporates. B is C with the split left half-finished.

There is a second problem specific to auth. Session middleware must import `auth`, so under B either the shared transport package imports every domain (making it a fan-in hub, exactly what B claims to avoid) or the middleware lands somewhere arbitrary.

**C is the recommendation.** The reasoning below is the substance.

### 7.4 Recommendation: feature-first, with a domain-free HTTP kit

```
backend/
  cmd/api/
    main.go          main() → run(ctx) error, exit codes
    run.go
    app.go           THE composition root
  config/            Config struct + Load(); imported only by cmd/api
  errs/              unchanged
  validation/        unchanged
  httpx/             HTTP kit — imports NO domain package
    server.go router.go middleware.go recover.go requestid.go
    accesslog.go timeout.go maxbytes.go cors.go readiness.go
    wrap.go codec.go errors.go
  sqlite/            INFRA ONLY — imports NO domain package
    db.go tx.go migrate.go sqlerr.go
    migrations/      stays central (fact 2)
    sqlitetest/
  auth/              domain: models.go service.go storage.go
    authhttp/        handler.go routes.go middleware.go context.go
    authsqlite/      store.go uow.go queries/*.sql internal/gen/
    authtest/        in-memory stores + pass-through UoW
  courses/           future features: identical shape
  test/integration/
```

Resulting graph — acyclic by construction rather than by discipline:

```
cmd/api ─┬─> auth/authhttp   ──> auth ──> errs, validation
         │                   ──> httpx
         ├─> auth/authsqlite ──> auth
         │                   ──> sqlite
         │                   ──> auth/authsqlite/internal/gen
         ├─> httpx           ──> errs, validation
         └─> config
```

### 7.5 Winning points

**1. `sqlite` drops from hub to leaf, permanently.** It becomes ~150 lines importing no domain package. The class of problem where a domain type and a storage helper need each other becomes unreachable rather than merely discouraged. The acceptance test is one command: `go list -deps ./sqlite | grep mrtutor` must return nothing.

This works without `sqlite` importing generated code, because `DBTX` is a *structural* interface. `sqlite` declares its own identically-shaped type and features pass it straight to their own `gen.New`:

```go
// sqlite/tx.go

// Handle is the intersection of *sql.DB and *sql.Tx. It is deliberately
// identical in shape to the DBTX interface sqlc generates, so a Handle can be
// passed to any feature's gen.New without sqlite importing any generated code.
type Handle interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}
```

**2. Encapsulation of generated code gets stronger, not weaker.** Today `sqlite/internal/gen` is reachable from all of `sqlite/`. After the move, `auth/authsqlite/internal/gen` is reachable only from `auth/authsqlite/`. With `omit_unused_structs: true`, each feature materialises only the tables its own queries touch — which also eliminates the current duplicate `Principal`/`User` structs.

```yaml
version: "2"
sql:
  - engine: sqlite
    schema: "sqlite/migrations/*.sql"        # central — fact 2
    queries: "auth/authsqlite/queries/*.sql"
    gen:
      go:
        package: gen
        out: "auth/authsqlite/internal/gen"
        omit_unused_structs: true
```

**3. The composition root stops being contended.** Adding a feature becomes: new directories, a new migration, one `sqlc.yml` block, and **one line** in `app.go`. Nothing shared is edited.

**4. Cross-feature transactions compose (fact 4).** The key design choice is that the shared transaction currency is a **`Conn`, not a `Stores`**. A per-feature `Stores` aggregate cannot express a transaction spanning two features; a `Conn` can, because each feature builds its own stores from it:

```go
db.InTx(ctx, func(c sqlite.Conn) error {
    lessons := lessonsqlite.NewStores(c)
    credits := billingsqlite.NewStores(c)
    // …orchestration lives in whoever owns the use case
})
```

Each feature keeps its own narrow `UnitOfWork` for single-feature atomicity, so `auth` stays pure. Cross-feature orchestration lives in the layer that owns the use case. Neither arrangement drags a domain import back into `sqlite`.

**5. It fixes the untestable-handler problem as a side effect.** With handlers in `auth/authhttp`, the consumer-side interface convention already used for stores extends naturally to the transport boundary:

```go
// auth/authhttp/handler.go

// Service is the slice of the auth service the HTTP layer uses. Declared here
// rather than depending on auth.Service directly, mirroring the convention
// auth.PrincipalStore already establishes — and this is what makes these
// handlers unit-testable against a fake.
//
// Only Login and Register exist on auth.Service as of Phase 3, when this
// interface was introduced; Authenticate and Logout are Phase 7 additions.
// Declaring all four from the start — as an earlier draft of this section
// did — means nothing satisfies the interface until Phase 7 lands, breaking
// the Phase 3-through-6 build. Grow the interface with the domain type, not
// ahead of it.
type Service interface {
	Login(ctx context.Context, in auth.LoginIn) (string, error)
	Register(ctx context.Context, in auth.RegisterIn) (auth.RegisterOut, error)
}
```

`auth.Service` satisfies this structurally — no adapter, no change to the domain. `auth/authhttp/handler_test.go` (Phase 3) is what that seam unblocks — not `cmd/api/http/auth_test.go`, a path that never existed on disk.

**6. Session middleware lands in the only place it can.** It imports `auth`, so it belongs in `auth/authhttp`. If it lived in `httpx`, then `httpx` would import `auth` and become the new hub. Feature-first is what keeps the kit domain-free.

**7. Package names stop fighting the tooling.** `authhttp` / `authsqlite` — prefixed, not `auth/http` / `auth/sqlite`, because `app.go` imports both `sqlite` and a feature adapter, and colliding package names would force aliases forever. That is today's `ehttp` problem; do not rebuild it.

### 7.6 Honest costs

- **Three packages per feature instead of one.** This is the real risk, and the mitigation is a hard boundary: three directories (`feature/`, `featurehttp/`, `featuresqlite/`), and the domain package stays flat. No `entity/`, `usecase/`, `dto/`, `repository/` ceremony — that is the Clean-Architecture-in-Go trap and it is a different proposal from this one.
- **`httpx` becomes public API.** Real, and the sequencing below neutralises it: **fix `wrap` before exporting it, not after.** Freezing a known-broken signature (defect #17) into a public API and then changing it is the one genuinely avoidable mistake available here.
- **Migrations remain central.** A documented, deliberate exception to feature ownership, forced by goose. Do not fight it.
- **`sqlite.Translate` must be exported.** A shared storage package survives; it just stops being a hub.

### 7.7 Rejected alternatives, and why

- **`init()`-based feature registry.** Tempting as a zero-bottleneck answer to §4.2. Reject it: an `init` function has no access to config, logger, or database, so the feature cannot receive its dependencies without globals or a service locator — the opposite of DI. Wiring becomes invisible, and tests cannot build an app with a subset of features. One explicit line in `app.go` is the correct floor.
- **A shared `core/` or `shared/` package for cross-feature types.** Becomes a dumping ground within months. Use a consumer-side interface owned by the consumer — the pattern `auth/storage.go` already demonstrates.
- **Transaction smuggled through `context.Value`.** Makes it impossible to tell from a signature whether a call is transactional. Pass the `Conn`.
- **`store.WithTx(tx)` on every store.** Duplicates a method per store and leaves the store free to keep reading from `db.R`, which is the stale-read trap in §5.3. Rebuilding stores from a `Conn` makes the correct choice the only reachable one.

---

## 8. Prioritised roadmap

Ordered so that nothing is a big-bang rewrite; every phase compiles with tests green. **Phases 0–2 deliver most of the risk reduction and require no package moves** — they are worth doing regardless of whether §7 is adopted.

**Phase 0 — defects, no structural change.** All 19 items in §6. First, because they are cheap, independently reviewable, and unblocked. Doing them before any code moves keeps the moves pure renames. Start with #1, #2, #3, #4, #7 — the security and operability items.

**Phase 1 — `run(ctx) error` and server lifecycle.** `main`/`run` split with real exit codes (reserve a distinct code for misconfiguration: it tells an operator not to restart). `Run(ctx)` / `Serve(ctx, ln)` with an explicit `net.Listen` so bind failures are synchronous. Retire the `isShuttingDown` global for an injected readiness value; split `/livez` from `/readyz`. Two rules: `run` must **not** return `ctx.Err()` — a clean SIGTERM is success, and returning `context.Canceled` puts every graceful shutdown into CrashLoopBackOff — and the shutdown context must use `context.WithoutCancel`, a property `main.go:57` already gets right today.

**Phase 2 — `Config` struct.** `Load(lookup func(string) (string, bool)) (Config, error)` accumulating errors into a `validation.Errors` — free leverage, since it already satisfies `errors.Is(err, errs.Invalid)`. Options structs for `sqlite.Open` and the server. Delete the package-level vars in the same commit; there are only three consumers. Verify with `go list -deps ./sqlite ./cmd/api/http | grep config` returning nothing.

**Phase 3 — fix `wrap`, then move it to `httpx`.** Constrain the type parameter so the compiler enforces validation:

```go
// Wrap turns a typed function into an http.HandlerFunc.
//
// In is constrained to Validable so validation is enforced by the compiler
// rather than by a runtime assertion that can fail open. If Validate is
// declared on *T, Wrap[T] does not compile — which is the point.
func Wrap[In Validable, Out any](...) http.HandlerFunc

// WrapUnvalidated is for inputs with nothing to validate. The name is
// deliberately unpleasant: reach for Wrap unless you can say why not.
func WrapUnvalidated[In, Out any](...) http.HandlerFunc
```

`LoginIn` and `RegisterIn` already have value-receiver `Validate()`, so both compile unchanged. **Do this before the rename, not after** (§7.6). Then `git mv cmd/api/http httpx`, export the kit, and move `auth.go` straight to `auth/authhttp` so it moves once.

**Status: done, as planned above, plus two deviations from this section's original wording and one unplanned repair.** `authhttp.Service` ships with `Login`/`Register` only — the four-method version this section originally sketched (see §7.5 point 5's corrected code block) doesn't compile until Phase 7 adds `Authenticate`/`Logout` to `auth.Service`. The planned `Register(*httpx.Router)` method for Phase 5 is renamed `Mount` to avoid colliding with `authhttp.Handler.Register`, the field holding the register handler. Unplanned: `go test ./...` was red on entry to this phase in two packages, for reasons unrelated to the rename — a Content-Type gate and defect #11's fix had landed without updating the tests that encoded the pre-fix behaviour. Phase 3 repaired both before doing the move, so the moves stayed pure renames as intended.

**Phase 4 — middleware and router.** A `Middleware func(http.Handler) http.Handler` chain plus a thin `Router` over `ServeMux` adding middleware scoping, which `ServeMux` has no notion of. Purely additive. Two details that are easy to get wrong:

- `Router.Group` must `slices.Clone` the middleware slice. Without it, two sibling groups appending to a chain with spare capacity write into the same backing array, and one group silently inherits the other's auth middleware. That is a security bug and the most common defect in hand-rolled Go routers.
- A `ResponseWriter` wrapper for status capture must implement `Unwrap() http.ResponseWriter`, or it hides `http.Flusher` / `http.Hijacker` from handlers.

Order is load-bearing and outermost-first: `RequestID` → `AccessLog` → `Recover`, so the log carries the ID and a panic is still recorded with the 500 it produced. `Recover` must re-panic on `http.ErrAbortHandler`.

**Phase 5 — the feature-first move.** Relocate codegen per §7.5; `sqlite/auth.go` → `auth/authsqlite/store.go`; `sqlite/errors.go` → `sqlite/sqlerr.go` with `Translate` exported; `authhttp` gains its `Service` interface and `Register(*httpx.Router)`. Gate: `go list -deps ./sqlite | grep mrtutor` returns nothing.

**Phase 6 — transactions.** `sqlite.Conn{W, R Handle}` where **both fields point at the same `*sql.Tx`** inside a transaction (§5.3); stores built from a `Conn`; `authsqlite.UnitOfWork`; `auth.Stores` gains `Sessions`. Hash passwords and mint tokens *before* opening the transaction — bcrypt is ~60ms of CPU and the write pool holds one connection, so hashing inside a transaction blocks every other writer for that whole time. Do **not** route `Login` through the UoW: it is a read plus one single-statement insert, and wrapping it would serialise all logins for no atomicity gain. The regression test that proves the phase: force the session insert to fail, assert no `users` row survives. That test fails against today's code.

**Phase 7 — session authentication.** Migration `0003` adding `expires_at` plus an index, so expiry is a database invariant rather than a Go computation. `SessionStore.GetActive` / `Revoke` / `DeleteExpired`, `PrincipalStore.GetByID`, `Service.Authenticate` / `Logout`, a new `ErrInvalidSession`, and `RequireSession` middleware in `authhttp`. Two design points: every failure mode must collapse to one error so the endpoint is not a token oracle, and the principal goes into the context under an unexported zero-size key type with a typed accessor, never a bare string key.

**Phase 8 — delivery.** CI running `go vet`, `go test -race`, `golangci-lint`, and a build. A multi-stage Dockerfile on a glibc base (cgo). Response DTOs at the HTTP boundary, removing JSON tags from domain models. An OpenAPI decision before the frontend lands.

---

## 9. Verification

- **`sqlite` is a leaf:** `go list -deps ./sqlite | grep mrtutor` returns nothing.
- **`config` is not a library dependency:** `go list -deps ./sqlite ./httpx | grep config` returns nothing.
- **Failures are visible:** `DATABASE_FILE=/nonexistent/x.db go run ./cmd/api; echo $?` prints non-zero.
- **Port conflict is fatal:** two instances on the same port — the second exits non-zero promptly.
- **No enumeration:** login with an unknown user and with a wrong password return byte-identical status and body.
- **`Register` is atomic:** with the session insert forced to fail, no `users` row survives.
- **Session lifecycle:** register → call an authenticated route with the cookie → logout → same call now 401.
- **Routing is covered:** at least one test drives the real mux through `/api/v1/...` rather than calling a handler directly.
- **Regression suite:** `task test` and `task unit-test` green; add `-race`.
- **The scaling smoke test:** adding a second feature must touch only its own new directories, a new migration, one `sqlc.yml` block, and one line in `app.go`. If it requires editing `sqlite/`, `httpx/`, or a shared routes file, the layout failed and should be revisited before feature three.
