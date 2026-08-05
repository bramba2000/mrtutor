# Architecture review — task list

Derived from `backend/docs/architecture-review.md`. Ordered so nothing is a big-bang change; each phase should compile with tests green before moving to the next. Phases 0–2 need no package moves and are worth doing regardless of whether the Phase 5 restructure is adopted.

Checkboxes are for tracking progress in this file.

---

## Phase 0 — Defect fixes (no structural change)

Do these first: cheap, independently reviewable, unblocked by everything else, and they keep the later package moves in Phase 3/5 as pure renames.

### 0.1 Security-relevant

- [x] **#1** `auth/service.go:34-37` — `Login` returns the store's not-found error verbatim, so an unknown username yields 404 while a wrong password yields 401 (user-enumeration oracle). The early return also skips bcrypt (timing oracle). Map not-found → `ErrInvalidCredentials` and compare against a fixed dummy hash on the miss path.
- [x] **#7** `cmd/api/http/auth.go:25-29,39-44` — session cookies set neither `Secure`, `SameSite`, nor `Path`. `Login` sets no `Expires`/`MaxAge` while `Register` sets 7 days — inconsistent lifetime for the same credential. Harden and unify both.
- [ ] ~~**#9** `sqlite/auth.go:35` — op string is `"get principal by token "+usernameOrEmail`; `writeError` logs it on every rejected request, so every failed login writes the submitted username/email to the log. Remove the value from the op string.~~
- [x] **#13** `sqlite/auth.go:54` — `SessionStore.Create` passes `auth.ErrSessionNotFound` as the *notFound* sentinel for an `INSERT` (semantically backwards); a session PK collision degrades to a 500. Fix the sentinel usage.

### 0.2 Error handling and mapping

- [x] **#2** `sqlite/errors.go:17-19` — `translateSQLError` discards its own `notFound` argument, returning bare `errs.NotFound`; `ErrPrincipalNotFound`/`ErrSessionNotFound` never reach the client (404 body says `{"code":"internal",...}`). Wrap the passed sentinel, not `errs.NotFound`.
- [x] **#11** `cmd/api/http/errors.go:94-98` — `isJsonDecodingError` doesn't match `io.EOF`/`io.ErrUnexpectedEOF`, so an empty/truncated body yields 500 instead of 400.
- [ ] ~~**#14** `sqlite/errors.go:28` — fallback uses `%v` not `%w`, breaking `errors.Is`/`Unwrap` on the driver error chain.~~ 
- [ ] ~~**#15** `sqlite/db.go:56` — rollback error formatted with `%v`; use `errors.Join` to preserve `errors.Is` on both errors.~~
- [ ] **#16** `sqlite/errors.go:23` — conflict detection matches only the coarse `sqlite3.ErrConstraint`; use the extended codes (`ErrConstraintUnique`, `ErrConstraintForeignKey`) to report which constraint/field collided.
- [x] **#18** `sqlite/auth.go:73-81` — `principalFromDB` silently drops `UpdatedAt`; nothing writes the column either.
- [x] **#19** `cmd/api/http/auth.go:45` — `Register` returns 200 not 201, and `RegisterOut.Principal` is computed then discarded. Wire it into the response body and use the existing (currently unused) `created` helper.

### 0.3 Remaining defects, polish, and test gaps

- [x] **#8** `cmd/api/http/codec.go:15-21` — no `http.MaxBytesReader`; request bodies are unbounded.
- [x] **#10** `auth/service_test.go:83,93` — `mockPrincipalStore.Create` has a value receiver but mutates `m.count`; the increment is lost, so every principal gets `ID = 1` and overwrites `db[1]`. Fix the fixture and add a test that seeds two principals into one store (this is also why #2 was never caught).
- [ ] **#12** `config/loaders.go:18,28` — `getDuration`/`getInt` swallow parse errors and silently return the default (e.g. `READ_POOL_SIZE=abc` → 4, no error). Superseded properly by the Phase 2 `Config` rework, but don't leave it silently broken in the meantime.
- [x] Polish: `bodyDecoder` ignores trailing content (check `dec.More()`); no `Content-Type` check (415 for non-JSON); `validation.Email` leaks the raw `mail.ParseAddress` message, inconsistent with other validators; `validation.MinLength[T ~string | ~[]any]` can't accept typed slices; `errs.Domain` returns an unexported `*domainError` callers can't name; `cmd/api/main.go:57` shadows `cancel`.
- [ ] Test gaps: `cmd/api/http/auth_test.go` is empty (0 bytes) — write handler-level unit tests once the `Service` seam exists (Phase 3); integration test calls handlers directly rather than through the mux — add a test that drives `/api/v1/...` through the real mux; `TestAuth` subtests share one DB with order dependence; assert the session cookie on successful login; `sqlite/db_test.go` touches disk without a `-short` gate.

---

## Phase 1 — `run(ctx) error` seam and server lifecycle

- [ ] **#3** Split `main()` into `main()` + `run(ctx, stdout, stderr, lookupEnv) error` with real exit codes; reserve a distinct exit code for misconfiguration. Fixes: startup failures currently `return` and exit 0.
- [ ] **#4** `cmd/api/http/server.go` — split bind from serve: `Run(ctx)` calls `net.Listen`, then `Serve(ctx, ln)`, so a listen error (port in use) is returned synchronously instead of only logged from a goroutine while the process keeps running serving nothing. Use `errors.Is(err, http.ErrServerClosed)`, not `==`.
- [ ] **#6** Add `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` to the `http.Server` (currently all zero → Slowloris exposure).
- [ ] Retire the `isShuttingDown atomic.Bool` global (`main.go:18`, read in `routes.go:22`) for an injected `Readiness` value; split liveness (`/livez`) from readiness (`/readyz`).
- [ ] Preserve: `run()` must **not** return `ctx.Err()` — a clean SIGTERM is success; returning `context.Canceled` would put every graceful shutdown into CrashLoopBackOff. The shutdown context must use `context.WithoutCancel` (already correct today at `main.go:57` — don't regress it).

---

## Phase 2 — `Config` struct

- [ ] Replace the package-level vars in `config/{app,db,server,loaders}.go` with a `Config` struct assembled by `Load(lookup func(string) (string, bool)) (Config, error)`.
- [ ] Accumulate parse errors into a `validation.Errors` (already satisfies `errors.Is(err, errs.Invalid)` for free) instead of silently defaulting on bad values — closes **#12**.
- [ ] Add an options struct for `sqlite.Open` (replacing the direct `config.ReadPoolSize` read at `sqlite/db.go:133,140-141`) and for the HTTP server (replacing the direct `config.LogLevel` read at `cmd/api/http/server.go:29`).
- [ ] Delete the package-level vars in the same commit — only three consumers, no compatibility shim needed.
- [ ] Verify: `go list -deps ./sqlite ./cmd/api/http | grep config` returns nothing.

---

## Phase 3 — Fix `wrap`, then rename `cmd/api/http` → `httpx`

Fix the generic before exporting it — freezing a known-broken signature into public API and changing it later is the one avoidable mistake here.

- [ ] **#17** Constrain `wrap`'s type parameter so validation is compiler-enforced: `func Wrap[In Validable, Out any](...)`. If `Validate` is declared on `*T`, `Wrap[T]` must not compile. Add `WrapUnvalidated[In, Out any]` for inputs with nothing to validate (deliberately unpleasant name). `auth.LoginIn`/`RegisterIn` already have value-receiver `Validate()`, so both compile unchanged.
- [ ] `git mv cmd/api/http httpx` (package `http` → `httpx`); export `Wrap`, `BodyDecoder`, `OK`/`Created`/`NoContent`, `WriteError`, `StatusFor`.
- [ ] Move `cmd/api/http/auth.go` straight to `auth/authhttp` (do this move once, not again in Phase 5). Fix `authHandler` visibility: `NewAuthHandler` currently returns an unexported type from an exported constructor, and takes `auth.Service` by concrete value (blocking fakes). Introduce an `authhttp.Service` interface (`Login`/`Register`/`Authenticate`/`Logout`) satisfied structurally by `auth.Service`. This unblocks writing `cmd/api/http/auth_test.go` (currently 0 bytes).
- [ ] Update `test/integration/auth_test.go`'s import (currently aliased `handlers`).

---

## Phase 4 — Middleware chain and Router (in `httpx`)

Purely additive — existing routes keep working.

- [ ] Add a `Middleware func(http.Handler) http.Handler` chain, plus a thin `Router` wrapping `http.ServeMux` that adds middleware scoping (`ServeMux` has no notion of this).
- [ ] Middleware set: `RequestID`, `Recover` (must re-panic on `http.ErrAbortHandler`), `AccessLog` (currently successful requests are never logged — only `writeError` logs), `Timeout` (per-request context deadline), body-size limit via `MaxBytesReader` (closes **#8**), `CORS`.
- [ ] Order is load-bearing and outermost-first: `RequestID` → `AccessLog` → `Recover`, so the log carries the request ID and a panic is still recorded with the 500 it produced.
- [ ] `Router.Group` must `slices.Clone` the middleware slice — without it, two sibling groups appending to a chain with spare capacity write into the same backing array, and one group silently inherits another's auth middleware (a security bug, and the most common defect in hand-rolled Go routers).
- [ ] Any `ResponseWriter` wrapper for status capture must implement `Unwrap() http.ResponseWriter`, or it hides `http.Flusher`/`http.Hijacker` from handlers.

---

## Phase 5 — Feature-first restructure

Relocate the auth adapter out of `sqlite/`; reduce `sqlite/` to an infra leaf. Depends on Phase 3 (needs `auth/authhttp` to already exist).

- [ ] Add a per-feature block to `sqlc.yml` (schema stays central in `sqlite/migrations/*.sql` — goose needs one global version sequence); move queries to `auth/authsqlite/queries/*.sql`; generate into `auth/authsqlite/internal/gen` with `omit_unused_structs: true` (also eliminates the current duplicate `Principal`/`User` structs). Delete `sqlite/internal` once migrated.
- [ ] Move `sqlite/auth.go` → `auth/authsqlite/store.go`. Store constructors take a `sqlite.Conn` (see Phase 6) rather than `*sqlite.DB` directly.
- [ ] Move `sqlite/errors.go` → `sqlite/sqlerr.go` and **export** `translateSQLError` as `sqlite.Translate` — every feature's adapter needs it, so it stays shared (the one deliberate hub-like exception).
- [ ] Add `sqlite/tx.go` with a `Handle` interface shaped identically to sqlc's `DBTX` (`ExecContext`/`PrepareContext`/`QueryContext`/`QueryRowContext`) so `sqlite` never imports any generated `gen` package — features pass their `Conn` straight to their own `gen.New`.
- [ ] `authhttp` gains its `Service` interface (if not already added in Phase 3) and a `Register(*httpx.Router)` method.
- [ ] Gate: `go list -deps ./sqlite | grep mrtutor` returns nothing — `sqlite` must not import any domain package.

---

## Phase 6 — Transaction-capable stores and `UnitOfWork`

Fixes the live data-integrity bug (**#5**): `Register` creates the principal then the session in two separate transactions, so a session-insert failure leaves an orphaned account that can't be retried.

- [ ] `sqlite.Conn{W, R Handle}` — inside a transaction **both** fields must point at the same `*sql.Tx` (not `W=tx`/`R=db.R`), otherwise a read-your-own-write inside the transaction silently returns stale data, since `db.R` is a physically different WAL connection. This is the critical correctness point — make the correct handle the only reachable one.
- [ ] Rebuild `authsqlite` stores from a `Conn` (constructor change from Phase 5).
- [ ] Add `authsqlite.UnitOfWork` implementing `auth.UnitOfWork.RunInTx` by calling `db.InTx` and rebuilding stores against the transaction's `Conn`.
- [ ] `auth.Stores` gains a `Sessions` field (currently only has `Principals`, so it can't even express the `Register` use case).
- [ ] Update `auth.Service.Register` to use `RunInTx`, wrapping both the principal `Create` and session `Create` in one transaction. Hash the password and mint the session token **before** opening the transaction — bcrypt is ~60ms of CPU and the write pool holds exactly one connection, so hashing inside the transaction would block every other writer for that whole time.
- [ ] Do **not** route `Login` through the `UnitOfWork` — it's a read plus one single-statement insert; wrapping it would serialize all logins behind the single writer for no atomicity gain.
- [ ] Regression test (the proof this phase worked): force the session insert to fail (e.g. FK violation via a bogus user id) and assert the principal/`users` row does **not** survive. This test fails against today's code.

---

## Phase 7 — Session authentication

Auth is currently half-built: sessions are minted and never verified. `SessionStore` has only `Create`; there is no logout, no server-side expiry, no way to authenticate a request.

- [ ] Migration `0003` adding `expires_at` (plus an index) to `sessions`, so expiry is a database invariant rather than a Go-side computation.
- [ ] `SessionStore.GetActive` / `Revoke` / `DeleteExpired`; `PrincipalStore.GetByID`.
- [ ] `Service.Authenticate` / `Service.Logout`; a new `ErrInvalidSession` sentinel. Every failure mode (missing session, expired, revoked, principal gone) must collapse to this one error so the endpoint is not a token oracle.
- [ ] `RequireSession` middleware in `authhttp`, mounted only on the routes that need it. Attach the authenticated principal to the request context under an **unexported zero-size key type** with a typed accessor — never a bare string key.
- [ ] Wire `RequireSession` into the routes that need it via `Router.Group` (validates the Phase 4 `Group`/`slices.Clone` design against a real consumer).

---

## Phase 8 — Delivery

- [ ] CI running `go vet`, `go test -race`, `golangci-lint`, and a build.
- [ ] Multi-stage Dockerfile on a glibc base (cgo dependency from `mattn/go-sqlite3` — no `CGO_ENABLED=0` static binary; budget for a C toolchain in the build image).
- [ ] Response DTOs at the HTTP boundary; remove JSON tags from domain models (`auth/models.go` — `Session.TokenHash` is currently tagged `json:"id"`, `[32]byte` would marshal as an array of 32 ints if ever serialized).
- [ ] Decide the API contract mechanism (OpenAPI spec / generated client) before a frontend lands — the repo root has an obvious empty slot for one and no contract exists yet.

---

## Verification checklist (from the review, §9)

- [ ] `sqlite` is a leaf: `go list -deps ./sqlite | grep mrtutor` → nothing.
- [ ] `config` is not a library dependency: `go list -deps ./sqlite ./httpx | grep config` → nothing.
- [ ] Failures are visible: `DATABASE_FILE=/nonexistent/x.db go run ./cmd/api; echo $?` → non-zero.
- [ ] Port conflict is fatal: run two instances on the same port — the second exits non-zero promptly.
- [ ] No enumeration: login with an unknown user and with a wrong password return byte-identical status and body.
- [ ] `Register` is atomic: with the session insert forced to fail, no `users` row survives.
- [ ] Session lifecycle: register → call an authenticated route with the cookie → logout → same call now 401.
- [ ] Routing is covered: at least one test drives the real mux through `/api/v1/...` rather than calling a handler directly.
- [ ] Regression suite: `task test` and `task unit-test` green; add `-race`.
- [ ] Scaling smoke test: adding a second feature touches only its own new directories, a new migration, one `sqlc.yml` block, and one line in `app.go`. If it requires editing `sqlite/`, `httpx/`, or a shared routes file, the layout failed and should be revisited before feature three.
