---
name: run-mrtutor
description: Build, run, and drive the mrtutor app (Go backend + Vite/React/Mantine frontend) with a real headless Chromium, including under WSL. Use when asked to start mrtutor, run its dev servers, take a screenshot of its UI, log in, or interact with/test a page or form end to end.
---

mrtutor is a Go backend + Vite/React/Mantine SPA, run together via `task dev`
(backend on :8080, Vite on :3000, proxied — see repo root `README.md`). Drive
it with **`playwright cli`** (bundled with the `playwright` npm package,
invoked below as `bunx playwright cli`) — it's the off-the-shelf agent-facing
CLI equivalent of `chromium-cli`, which isn't available in this environment.
It manages its own persistent background browser session, so no tmux/REPL
wrapper is needed: every command below is a plain, synchronous shell call.
It works fine headless under WSL — Chromium's headless mode needs no display
server; the only WSL-specific requirement is the one-time native dependency
install below.

For the full command reference beyond what's used here, read
`node_modules/playwright-core/lib/tools/skills/playwright-cli/SKILL.md`
(installed by Setup, below) — it's the tool's own bundled agent skill.

All paths below are relative to the repo root (`mrtutor/`).

## Prerequisites

Playwright's own bundled Chromium, plus the shared libs it needs (`libnss3`,
`libatk-bridge2.0-0`, `libgbm1`, etc.) — this needs `sudo`, so run it in a
terminal you control rather than letting an agent run it:

```bash
bunx playwright install --with-deps chromium
```

This downloads to `~/.cache/ms-playwright/`, shared across projects — the
`bun install` below does NOT re-download it.

## Setup

Keep the CLI's own tiny `package.json` out of `frontend/`'s dependency tree
(it's agent tooling, not app code):

```bash
cd .claude/skills/run-mrtutor && bun install
```

All commands below assume you're in this directory (so `bunx playwright cli`
resolves the local install without a separate download).

## Run (agent path)

**Check for an already-running dev server first** — a human may already have
`task dev` up in another terminal. Do NOT blindly kill whatever is listening;
only stop processes you started yourself this run.

```bash
curl -sf http://localhost:8080/ >/dev/null && curl -sf http://localhost:3000/ >/dev/null && echo "already running"
```

If that fails, start it from the repo root and wait for both ports:

```bash
(task dev > /tmp/mrtutor-dev.log 2>&1 &)
timeout 40 bash -c 'until curl -sf http://localhost:8080/ >/dev/null; do sleep 1; done'
timeout 40 bash -c 'until curl -sf http://localhost:3000/ >/dev/null; do sleep 1; done'
```

Then, from `.claude/skills/run-mrtutor/`, open a **named** session (`-s=`)
so you don't collide with any other browser session on the machine —
`--browser=chromium` is required, since the CLI otherwise defaults to a
branded Chrome that isn't installed here:

```bash
bunx playwright cli -s=mrtutor open --browser=chromium http://localhost:3000
bunx playwright cli -s=mrtutor snapshot
```

Read the printed accessibility snapshot for `ref=`s and act on them —
`click <ref>`, `fill <ref> <value>`, `type <text>`. There's no seeded test
account; register one first:

```bash
bunx playwright cli -s=mrtutor goto http://localhost:3000/register
bunx playwright cli -s=mrtutor snapshot                     # find the field refs
bunx playwright cli -s=mrtutor fill <username-ref> myuser123
bunx playwright cli -s=mrtutor fill <email-ref> myuser@example.com
bunx playwright cli -s=mrtutor fill <password-ref> 'Sup3rSecret!23'   # needs upper+lower+digit+special
bunx playwright cli -s=mrtutor click <register-button-ref>
```

A successful register/login redirects synchronously — no manual URL polling
needed, unlike a raw Playwright script. Then drive the app the same way:
`snapshot` → read `ref`s → `click`/`fill`/`type`. When you're done:

```bash
bunx playwright cli -s=mrtutor screenshot --filename=/tmp/mrtutor-shots/final.png   # optional sanity artifact
bunx playwright cli -s=mrtutor close
```

Then actually open the screenshot if you took one — don't just trust that
the command ran.

`ref`-based targeting (from `snapshot`/`find`) is preferred over CSS
selectors or hand-written `getByRole`/`getByLabel` strings — it's what the
tool itself resolves clicks/fills against, so there's no guessing at
accessible names or dealing with ambiguous matches yourself.

### Useful non-obvious commands

| command | why you'd reach for it here |
|---|---|
| `find <text>` | locate an element by visible text without a full snapshot dump |
| `console [level]` | check for JS errors after driving an interaction |
| `requests` / `request <n>` / `response-body <n>` | confirm an API call (e.g. `/students/schools`) actually fired and what it returned |
| `state-save` / `state-load` | persist a logged-in session across separate CLI invocations/days instead of re-registering |
| `list` / `close-all` / `kill-all` | see/stop stray sessions; `kill-all` for zombie daemons |

## Run (human path)

```bash
task dev   # backend :8080, Vite :3000. Ctrl-C to stop.
```

Open `http://localhost:3000` in a real browser.

## Test

```bash
task backend:test    # go vet + build + test -race — passed clean when run
```

`task verify` also runs `frontend:check` (prettier), which currently fails
on **pre-existing** formatting issues in files unrelated to any given
change (seen: `docs/large-entity-forms-research.md`, `index.html`,
`RouterAncor.tsx`, `LoginForm.tsx`, `RegisterForm.tsx`) — don't mistake that
for a regression you just introduced; check `git diff` before assuming
you broke it.

## Gotchas

- **`open`/`goto`'s default browser is a branded Chrome that isn't
  installed** (`Chromium distribution 'chrome' is not found at
  /opt/google/chrome/chrome`). Always pass `--browser=chromium` on `open`
  (only needed once per session, not on every subsequent command).

- **The snapshot file linked in `open`/`goto`'s own output
  (`### Snapshot\n- [Snapshot](.playwright-cli/page-....yml)`) can be an
  empty 0-byte file** even though the page loaded fine — this looks like a
  timing/lazy-write quirk. Don't trust that linked file; run
  `playwright cli -s=... snapshot` explicitly and read its inline output
  instead.

- **`StudentDialog`'s Modal doesn't reset form state between "Add Student"
  opens** in the same page session. Close-then-reopen (without a full page
  reload) can leave a previously-typed value still in a field — confirmed
  twice, not a fluke — so don't mistake a stale value for a bug in the
  autocomplete feature itself; `fill <ref> ""` to clear before retyping if
  you need a clean field.

- **Two harmless console errors appear on every unauthenticated page
  load**: a 401 on `/api/v1/auth/me` (the app's own "am I logged in?"
  probe, expected before redirecting to `/login`) and a 404 on
  `/favicon.ico` (the app has none). Don't treat either as a regression.

- **Don't assume a dev server isn't already running.** `task dev` binds
  :8080/:3000; if a human's own terminal already has it up, starting a
  second one fails loudly on the backend (`bind: address already in use`)
  but Vite silently falls back to :3001 — so `curl localhost:3000` still
  looks fine while you're actually driving a different, port-shifted
  instance. Always check first (see Run above), and if you do end up
  needing to stop something as cleanup, stop only what you started —
  a port-conflict `kill` aimed too broadly can take out a human's
  pre-existing dev servers.

## Troubleshooting

- **`Error: Daemon process exited with code 1` /
  `Chromium distribution 'chrome' is not found`**: you omitted
  `--browser=chromium` on `open`. See Gotchas above.
- **A driven form submit does nothing and the URL doesn't change**: check
  for a Mantine client-side validation error first (`snapshot` and look —
  Mantine renders the message under the field) before assuming the click
  didn't register. Hit this with the register form's password policy
  (needs upper+lower+digit+special char) rejecting a plain-alphanumeric
  test password.
- **Session seems stuck / won't respond**: `bunx playwright cli list` to
  see what's running, `bunx playwright cli -s=<name> close` to stop it
  cleanly, or `bunx playwright cli kill-all` for a stale/zombie daemon.
