# Architecture review — task list

Derived from `backend/docs/architecture-review.md`. Ordered so nothing is a big-bang change; each phase should compile with tests green before moving to the next. Phases 0–2 need no package moves and are worth doing regardless of whether the Phase 5 restructure is adopted.

Checkboxes are for tracking progress in this file.

---

## Phase 0 — Defect fixes (no structural change)

Do these first: cheap, independently reviewable, unblocked by everything else, and they keep the later package moves in Phase 3/5 as pure renames.

### 0.1 Security-relevant

- [x] **#1** `auth/service.go:34-37` — `Login` returns the store's not-found error verbatim, so an unknown username yields 404 while a wrong password yields 401 (user-enumeration oracle). The early return also skips bcrypt (timing oracle). Map not-found → `ErrInvalidCredentials` and compare against a fixed dummy hash on the miss path.
- [x] **#7** `cmd/api/http/auth.go:25-29,39-44` — session cookies set neither `Secure`, `SameSite`, nor `Path`. `Login` sets no `Expires`/`MaxAge` while `Register` sets 7 days — inconsistent lifetime for the same credential. Harden and unify both. **Reopened and actually closed in Phase 3:** this was ticked here but never implemented — `MaxAge: int(sessionCookieMaxAge)` fed a `time.Duration` (nanoseconds) to a seconds-valued field (604800000000000, not 604800), and `Secure`/`SameSite`/`Path` were absent. Phase 3 fixes the conversion (`.Seconds()`), adds `Path: "/"` and `SameSite: Lax`, and makes `Secure` a `authhttp.Config` field driven by `APP_MODE` (`Secure: cfg.AppMode == config.AppModeProd`) rather than hardcoded — dev traffic over plain HTTP still gets the cookie. Guarded by `auth/authhttp/handler_test.go`'s `TestLogin` and the integration suite.
- [ ] ~~**#9** `sqlite/auth.go:35` — op string is `"get principal by token "+usernameOrEmail`; `writeError` logs it on every rejected request, so every failed login writes the submitted username/email to the log. Remove the value from the op string.~~
- [x] **#13** `sqlite/auth.go:54` — `SessionStore.Create` passes `auth.ErrSessionNotFound` as the *notFound* sentinel for an `INSERT` (semantically backwards); a session PK collision degrades to a 500. Fix the sentinel usage.

### 0.2 Error handling and mapping

- [x] **#2** `sqlite/errors.go:17-19` — `translateSQLError` discards its own `notFound` argument, returning bare `errs.NotFound`; `ErrPrincipalNotFound`/`ErrSessionNotFound` never reach the client (404 body says `{"code":"internal",...}`). Wrap the passed sentinel, not `errs.NotFound`.
- [x] **#11** `cmd/api/http/errors.go:94-98` — `isJsonDecodingError` doesn't match `io.EOF`/`io.ErrUnexpectedEOF`, so an empty/truncated body yields 500 instead of 400.
- [ ] ~~**#14** `sqlite/errors.go:28` — fallback uses `%v` not `%w`, breaking `errors.Is`/`Unwrap` on the driver error chain.~~ 
- [ ] ~~**#15** `sqlite/db.go:56` — rollback error formatted with `%v`; use `errors.Join` to preserve `errors.Is` on both errors.~~
- [ ] **#16** `sqlite/errors.go:23` — conflict detection matches only the coarse `sqlite3.ErrConstraint`; use the extended codes (`ErrConstraintUnique`, `ErrConstraintForeignKey`) to report which constraint/field collided.
- [x] **#18** `sqlite/auth.go:73-81` — `principalFromDB` silently drops `UpdatedAt`; nothing writes the column either.
- [x] **#19** `cmd/api/http/auth.go:45` — `Register` returns 200 not 201, and `RegisterOut.Principal` is computed then discarded. Wire it into the response body and use the existing (currently unused) `created` helper. **Reopened and actually closed in Phase 3:** this was ticked here but the code still returned 204 via `noContent`, and the integration test asserted 200 — three sources (this doc, the code, the test) disagreed. Phase 3 makes this doc's original call authoritative: `Register` now returns 201 with `out.Principal` as the JSON body (`auth.Principal.PasswordHash` stays off the wire via its existing `json:"-"` tag; `RegisterOut` itself is never encoded directly, since it has no JSON tags and would otherwise put the session token in the response body next to the cookie).

### 0.3 Remaining defects, polish, and test gaps

- [ ] **#8** `cmd/api/http/codec.go:15-21` — no `http.MaxBytesReader`; request bodies are unbounded. **Unticked:** there is no `MaxBytesReader` anywhere in the tree, contrary to the mark this line carried. It is genuinely scheduled — Phase 4's middleware set already lists it ("body-size limit via `MaxBytesReader` (closes **#8**)") — so it stays open until that phase, not marked done early.
- [x] **#10** `auth/service_test.go:83,93` — `mockPrincipalStore.Create` has a value receiver but mutates `m.count`; the increment is lost, so every principal gets `ID = 1` and overwrites `db[1]`. Fix the fixture and add a test that seeds two principals into one store (this is also why #2 was never caught).
- [x] **#12** `config/loaders.go:18,28` — `getDuration`/`getInt` swallow parse errors and silently return the default (e.g. `READ_POOL_SIZE=abc` → 4, no error). Closed by the Phase 2 `Config` rework: `config.Load` now records a parse or validation failure per env var into a `validation.Errors` and returns it, so a bad value fails startup (exit 2) instead of silently defaulting.
- [x] Polish, landed: `bodyDecoder`'s `Content-Type` check (415 for non-JSON); `validation.MinLength[T ~string | ~[]any]`; `cmd/api/main.go:57`'s `cancel` shadow.
- [ ] Polish, still open (this line previously claimed all of the below were done; they are not): `bodyDecoder` still ignores trailing content — no `dec.More()` check exists, and `httpx/codec_test.go`'s "Success when body has trailing content" subtest now documents that leniency as *intended* rather than a gap, so either implement the check or leave this box open on purpose; `validation.Email` still leaks the raw `mail.ParseAddress` message; `errs.Domain` still returns an unexported `*domainError` (`errs/domain.go:14`) callers can't name.
- [x] Test gaps, landed in Phase 3: handler-level unit tests now exist at `auth/authhttp/handler_test.go` (not `cmd/api/http/auth_test.go` — that path never existed on disk; it wasn't a 0-byte file, it was absent) covering cookie attributes, status codes, and error mapping against a fake `Service`; the integration suite's successful-login case now asserts the session cookie's `HttpOnly`/`Path`/`MaxAge`.
- [ ] Test gaps, still open: integration test calls handlers directly rather than through the mux — add a test that drives `/api/v1/...` through the real mux (needs a `cmd/api/routes_test.go` inside `package main`, since `RegisterRoutes` is unexported from a package no external test can import); `TestAuth` subtests share one DB with order dependence; `sqlite/db_test.go` touches disk without a `-short` gate.

**Standing rule this phase's stale-test episode implies:** a fix must update the test that documented the bug, in the same commit. `httpx`'s Content-Type gate (closing part of #8/polish) and defect #11's fix both landed without updating the tests that encoded the pre-fix behaviour, leaving `go test ./...` red across two packages until Phase 3 repaired them (`httpx/codec_test.go`'s missing `Content-Type` headers; `httpx/errors_test.go`'s two `"known gap"` cases for `io.EOF`/`io.ErrUnexpectedEOF`, which #11 had already closed).

---

## Phase 1 — `run(ctx) error` seam and server lifecycle

- [x] **#3** Split `main()` into `main()` + `run(ctx, stdout, stderr, lookupEnv) error` with real exit codes; reserve a distinct exit code for misconfiguration. Fixes: startup failures currently `return` and exit 0.
- [x] **#4** `cmd/api/http/server.go` — split bind from serve: `Run(ctx)` calls `net.Listen`, then `Serve(ctx, ln)`, so a listen error (port in use) is returned synchronously instead of only logged from a goroutine while the process keeps running serving nothing. Use `errors.Is(err, http.ErrServerClosed)`, not `==`.
- [ ] **#6** Add `ReadHeaderTimeout`/`ReadTimeout`/`WriteTimeout`/`IdleTimeout` to the `http.Server` (currently all zero → Slowloris exposure).
- [x] Retire the `isShuttingDown atomic.Bool` global (`main.go:18`, read in `routes.go:22`) for an injected `Readiness` value; split liveness (`/livez`) from readiness (`/readyz`). **Reopened and actually closed in Phase 3:** the `Readiness` value existed but `main.go`'s `isShuttingDown` global and `routes.go`'s `GET /health`/`healthHandler` reading it were never deleted — dead code nothing wrote to, so `/health` was unconditionally 200. Also, `run.go` mounted the `Readiness` handler at `/healthz`, not `/readyz` as stated here. Phase 3 deletes the global and `/health`, and renames the route to `/readyz`. Separately (and more severely — see the new defect below), `Readiness.Handler()`'s branch was inverted, so `/healthz` was permanently serving 503 while healthy; also fixed in Phase 3, with a new `httpx/readiness_test.go` (previously zero coverage).
- [x] Preserve: `run()` must **not** return `ctx.Err()` — a clean SIGTERM is success; returning `context.Canceled` would put every graceful shutdown into CrashLoopBackOff. The shutdown context must use `context.WithoutCancel` (already correct today at `main.go:57` — don't regress it).

---

## Phase 2 — `Config` struct

- [x] Replace the package-level vars in `config/{app,db,server,loaders}.go` with a `Config` struct assembled by `Load(lookup func(string) (string, bool)) (Config, error)`.
- [x] Accumulate parse errors into a `validation.Errors` (already satisfies `errors.Is(err, errs.Invalid)` for free) instead of silently defaulting on bad values — closes **#12**.
- [x] Add an options struct for `sqlite.Open` (replacing the direct `config.ReadPoolSize` read at `sqlite/db.go:133,140-141`) and for the HTTP server. The HTTP server options struct (`cmd/api/http/server.go:13-27`, `Config.LogLevel`) already landed in `55c5e77`, ahead of this phase; `sqlite.Open(ctx, sqlite.Options{Path, Logger, ReadPoolSize})` closes the remaining gap, defaulting `ReadPoolSize` to `DefaultReadPoolSize` (4) so `sqlite` stands alone.
- [x] Delete the package-level vars in the same commit — only two live consumers by the time this landed (`cmd/api/run.go`, `sqlite/db.go`), no compatibility shim needed.
- [x] Verify: `go list -deps ./sqlite ./cmd/api/http | grep config` returns nothing.
- [x] Bonus, closed as part of this phase: `LOG_LEVEL` now accepts slog level names (`DEBUG`/`INFO`/`WARN`/`ERROR`) as well as the numeric form `Taskfile.yml` uses, fixing the silent-Info fallback described in §4.1 point 3; `LOG_FORMAT` went from dead config to an actual `text`/`json` selector; `SHUTDOWN_HARD_TIMEOUT` (declared, never consumed) was removed rather than migrated.

---

## Phase 3 — Fix `wrap`, then rename `cmd/api/http` → `httpx`

Fix the generic before exporting it — freezing a known-broken signature into public API and changing it later is the one avoidable mistake here.

- [x] **#17** Constrain `wrap`'s type parameter so validation is compiler-enforced: `func Wrap[In Validable, Out any](...)`. If `Validate` is declared on `*T`, `Wrap[T]` must not compile. Add `WrapUnvalidated[In, Out any]` for inputs with nothing to validate (deliberately unpleasant name). `auth.LoginIn`/`RegisterIn` already have value-receiver `Validate()`, so both compile unchanged. Landed as designed; `wrapNoInput` was also exported as `WrapNoInput`, rebuilt on `WrapUnvalidated` since `struct{}` cannot satisfy `Validable` (not called out in this line originally, but Phase 7's `Logout` needs a no-input wrapper reachable from outside the package). Verified with `go vet`: a type whose `Validate` is pointer-receiver-only fails to compile against `Wrap` with `"does not satisfy Validable (method Validate has pointer receiver)"`.
- [x] `git mv cmd/api/http httpx` (package `http` → `httpx`); export `Wrap`, `BodyDecoder`, `OK`/`Created`/`NoContent`, `WriteError`, `StatusFor`. Also dropped the `httpstdlib` alias in `server_test.go` (needed only because of the old `http`/`net/http` name collision) and the `ehttp` alias at both `cmd/api` import sites.
- [x] Move `cmd/api/http/auth.go` straight to `auth/authhttp` (do this move once, not again in Phase 5). Fix `authHandler` visibility: `NewAuthHandler` currently returns an unexported type from an exported constructor, and takes `auth.Service` by concrete value (blocking fakes). Introduce an `authhttp.Service` interface (`Login`/`Register`/`Authenticate`/`Logout`) satisfied structurally by `auth.Service`. This unblocks writing `cmd/api/http/auth_test.go` (currently 0 bytes).
  - **Deviation 1:** `authhttp.Service` ships with only `Login`/`Register` — `auth.Service` has no `Authenticate`/`Logout` yet (those are Phase 7). Declaring all four now would mean nothing satisfies the interface and the `cmd/api` wiring wouldn't compile. Phase 7 adds the other two methods here and to the fake.
  - **Deviation 2:** the target file was never `cmd/api/http/auth_test.go` — that path doesn't exist on disk (not a 0-byte file, as both this doc and the review claimed; simply absent). The tests landed at `auth/authhttp/handler_test.go` instead, covering cookie attributes (including a direct regression guard for the `MaxAge` nanoseconds bug and the new `Config.Secure` flag), status codes, and error mapping against a fake `Service`.
  - **Deviation 3, forward note for Phase 5:** the review's planned `Register(*httpx.Router)` method on this type would collide with the `Register` *field* holding the handler. Phase 5 should name it `Mount(*httpx.Router)` instead.
  - Constructor renamed `NewAuthHandler` → `NewHandler` and the handler type `authHandler` → `Handler`, since `authhttp.NewAuthHandler` stutters once the package is no longer named `http`.
  - Picked up in the same commit, since the file was already being rewritten: `authhttp.Config{Secure, MaxAge}` (see the #7/#19 notes above), and `NewHandler`/`RegisterRoutes` now take it — sourced in `cmd/api/run.go` from `cfg.AppMode == config.AppModeProd`.
- [x] Update `test/integration/auth_test.go`'s import (currently aliased `handlers`). Now imports `auth/authhttp` unaliased.
- [x] Repaired the test suite, which was red on entry to this phase for reasons unrelated to the rename itself — see the "Test gaps" and "Polish" notes above (0.3) for what was actually stale and why.
- [x] Fixed in the same pass, in files this phase already touches (see the reopened items above): the readiness-handler inversion (`httpx/readiness.go`), the cookie `MaxAge`/`Secure`/`SameSite`/`Path` defects (#7), Register's status code (#19), and the dead `isShuttingDown` global / `GET /health` route.

### Defects found during Phase 3 (not in the original §6 inventory of 19)

The review's defect table (§6) enumerated 19 items; these surfaced only while doing this phase's file moves and are recorded here rather than renumbered into that table, so the "19 items" count in the review stays honest as a historical fact.

- [x] **Readiness handler inverted.** `httpx/readiness.go`'s `Handler()` read `if r.Ready() { 503 }` — it served 503 while healthy and 200 while draining, the exact opposite of a readiness probe's contract. A Kubernetes readiness probe would have pulled the pod out of rotation the instant it became healthy. Zero test coverage before this phase; `httpx/readiness_test.go` now covers both states.
- [x] **Session cookie `MaxAge` was in nanoseconds.** `int(sessionCookieMaxAge)` on a `time.Duration` constant produced `604800000000000`, not `604800` seconds — folded into the #7 fix above.

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
- [ ] `authhttp`'s `Service` interface already landed in Phase 3 (with only `Login`/`Register`; Phase 7 adds `Authenticate`/`Logout`). Add a `Mount(*httpx.Router)` method — named `Mount`, not `Register`, because `authhttp.Handler` already has a `Register` field holding the register handler and the two would collide.
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

## Phase 9 — Background scheduler

- [x] New `scheduler/` package, a domain-free kit sibling to `httpx`/`sqlite` — verified by `go list -deps ./scheduler | grep mrtutor` returning only `errs` and `validation`. In-process, goroutine-based: one goroutine per registered `Task`, so two runs of the same task can never overlap (skipping an overrun fire is a consequence of that loop's shape, not a policy it checks for).
- [x] `Schedule` is a pure interface, `Next(start, last time.Time) (time.Time, bool)`, so schedules are shareable values and testable as an unordered table. Built-ins: `Once`/`OnceAt` (fire exactly once), `Every` (optionally `.After(delay)`), and `Periodic.Within(Window)` — a recurring wall-clock span that expresses suspension ("every 5m, but only weekdays 09:00–18:00") declaratively, with no runtime `Suspend`/`Resume` state. Cron expression *parsing* is out of scope; the interface is shaped so `Cron(expr)` drops in later using `last` as its base and `start` as the fallback on the first call, with no change to the run loop.
- [x] A task returning an error wrapping `scheduler.ErrFatal` stops the whole `Scheduler` and propagates the error out of `Run`; a panic is always recovered, logged with its stack, and never fatal. Both are `runOnce`'s job, alongside `Task.Timeout`.
- [x] Shutdown drains in-flight runs on a live (`context.WithoutCancel`) task context for up to `Config.DrainPeriod`, and only cancels them if that elapses; the whole sequence is bounded by `Config.ShutdownTimeout`. A clean shutdown returns nil, never `ctx.Err()` — the same rule Phase 1 established for `httpx.Server`.
- [x] `cmd/api/run.go` now runs `httpx.Server` and `scheduler.Scheduler` under one `errgroup.WithContext`, promoting `golang.org/x/sync` from indirect to direct — the first place this repo needed two long-lived components to tear each other down. `cmd/api/tasks.go` mirrors `routes.go`: the one line a future task addition touches. No task is registered yet.
- [x] `config.Scheduler{Location, DrainPeriod, ShutdownTimeout}`, loaded via the existing `get` helper under a `SCHEDULER_` prefix — the one departure from this file's flat-name convention, because `DRAIN_PERIOD`/`SHUTDOWN_TIMEOUT` are already taken by `Server`'s own settings.
- [x] Tests use `testing/synctest` for deterministic timing (fires, skips, drain/cancel) — the first use of it in this repo, and still no `Clock` seam, consistent with `auth.Service`'s direct `time.Now()` calls.

### Defect found while manually verifying this phase

- [x] **First fire silently skipped in production.** `nextFireTime`'s skip-catch-up loop compared the very first computed fire time against a freshly sampled `time.Now()`. For any schedule with no initial delay, that first fire equals `start`, which is captured once at the top of `Run` — before any task goroutine is spawned. By the time a task's goroutine actually samples `now`, real wall-clock time has already ticked forward by whatever scheduling latency the runtime introduced, so `next.Before(now)` was always true and the task's legitimate first run was discarded as a bogus "previous run overran" catch-up. `testing/synctest`'s fake clock never advances during plain CPU work, so no test caught this — only running the real binary did. Fixed by never applying the skip check when `last` is still zero (nothing has run yet, so there is nothing to have overrun); regression-tested with a `ScheduleFunc` that fabricates an already-past first fire deterministically, since the real race can't be reproduced under the fake clock.

---

## Verification checklist (from the review, §9)

- [ ] `sqlite` is a leaf: `go list -deps ./sqlite | grep mrtutor` → nothing.
- [x] `config` is not a library dependency: `go list -deps ./sqlite ./httpx | grep config` → nothing. `httpx` now exists (Phase 3) and re-verifies clean; `go list -deps ./httpx | grep mrtutor` returns only `errs` and `validation` — `httpx` is domain-free.
- [ ] Failures are visible: `DATABASE_FILE=/nonexistent/x.db go run ./cmd/api; echo $?` → non-zero.
- [ ] Port conflict is fatal: run two instances on the same port — the second exits non-zero promptly.
- [ ] No enumeration: login with an unknown user and with a wrong password return byte-identical status and body.
- [ ] `Register` is atomic: with the session insert forced to fail, no `users` row survives.
- [ ] Session lifecycle: register → call an authenticated route with the cookie → logout → same call now 401.
- [ ] Routing is covered: at least one test drives the real mux through `/api/v1/...` rather than calling a handler directly.
- [ ] Regression suite: `task test` and `task unit-test` green; add `-race`.
- [ ] Scaling smoke test: adding a second feature touches only its own new directories, a new migration, one `sqlc.yml` block, and one line in `app.go`. If it requires editing `sqlite/`, `httpx/`, or a shared routes file, the layout failed and should be revisited before feature three.
- [x] `scheduler` is domain-free: `go list -deps ./scheduler | grep mrtutor` → only `errs` and `validation`.
- [x] SIGTERM with a job in flight drains it and exits 0; a task returning `scheduler.ErrFatal` tears the HTTP server down too and exits 1 (verified against the compiled binary, not `go run`, whose own signal handling isn't representative — see Phase 9).
