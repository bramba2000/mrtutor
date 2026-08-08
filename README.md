# MrTutor

A Go backend (`backend/`) and a Vite + React + TanStack Router single-page app
(`frontend/`), deployed as one binary: `backend/cmd/api` embeds the built SPA
and serves it alongside the JSON API.

- API: `/api/v1/...`
- Everything else: the SPA, with client-side routes falling back to
  `index.html`

See `backend/docs/architecture-review.md` and `backend/docs/architecture-tasks.md`
for the design rationale and phase-by-phase history.

## Prerequisites

- Go (see `backend/go.mod` for the version)
- [Bun](https://bun.sh)
- [Task](https://taskfile.dev) (`go install github.com/go-task/task/v3/cmd/task@latest`)

## Development

```sh
task install   # bun install, once
task dev       # backend on :8080, Vite dev server on :3000
```

Open `http://localhost:3000` — Vite proxies `/api` to the backend, so the
browser sees a single origin and the session cookie works without CORS. Set
`BACKEND_URL` to point the proxy elsewhere (default `http://localhost:8080`).

Backend-only tasks (`task backend:run`, `task backend:test`, …) still work
from inside `backend/` on their own; see `backend/Taskfile.yml`.

## Building the single deployable binary

```sh
task build
```

This builds the frontend into `backend/web/dist` (embedded via
`//go:embed`), then builds the Go binary. `backend/web/dist` is gitignored
except for a placeholder `.gitkeep`, so a fresh clone still compiles before
anyone has run a frontend build — `go build ./cmd/api` in that state
produces a binary that serves the API but answers `503` at `/`.

Starting that binary with `APP_MODE=production` and no frontend build is a
**fatal** startup error instead; `APP_MODE=development` (the default) only
warns and serves `503`, so backend-only iteration doesn't require a frontend
build.

## Verification

```sh
task verify   # frontend lint + format check, then backend vet/build/test -race
```
