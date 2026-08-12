# frontend

The mrtutor SPA: React 19 + TanStack Router (file-based routing) + TanStack Query + Mantine.
In production it's embedded into the Go binary (`backend/web`); in dev, Vite proxies `/api` to
the Go server so the browser only ever sees one origin (see the repo root `README.md`).

## Getting started

```bash
bun install
bun run dev
```

Scripts: `dev`, `build`, `preview`, `generate-routes` (regenerate `src/routeTree.gen.ts` after
adding/moving a route file — the dev server also does this automatically), `lint`, `format`
(prettier + eslint --fix), `check` (prettier --check, used in CI).

## Import alias

Use the `#/` subpath import (declared in both `tsconfig.json` `paths` and `package.json`
`imports`), e.g. `import { apiFetch } from '#/lib/api'`. Don't use `@/` — it resolves via
tsconfig-paths but isn't the convention used anywhere in the codebase.

## Style

Formatting is enforced by `prettier.config.js`: 2-space indent, single quotes, no semicolons,
trailing commas. Run `bun run format` before committing; `bun run check` is what CI runs.
`verbatimModuleSyntax` is on, so type-only imports must use `import type`.

## API layer

Talking to the backend follows one convention, laid out below and demonstrated end-to-end by
`src/features/auth/`. Copy that layout for every new feature (`courses/`, etc. — see
`backend/docs/architecture-review.md` §7 for what's planned).

```
src/
  lib/
    api.ts        transport only — fetch, ApiError, base path. Zero domain knowledge.
    query.ts      the QueryClient singleton + its defaults.
  features/
    <feature>/
      types.ts    wire types, hand-mirrored from the Go DTOs
      api.ts      one function per endpoint; the ONLY file with URL strings
      queries.ts  query-key factory + queryOptions + useMutation hooks
      components/ feature UI that consumes the hooks above
  routes/         thin: search validation, auth/data guards, and rendering a feature component
```

Rules, in order of how much they matter:

1. **`api.ts` is the only file that knows URLs.** No React, no `@tanstack/react-query` import.
   Each export is `(request) => Promise<Response>` built on `apiFetch`/`post` from `lib/api.ts`.
   Trivially testable, reusable outside a component.
2. **`queries.ts` is the only file that knows about caching.** It owns the key factory, every
   `queryOptions`, and every `useMutation` hook — including its `invalidateQueries` calls. If a
   cache key is invalidated anywhere, it's in this file.
3. **Route files never fetch.** No `queryOptions`, no `apiFetch`, no `fetch` under `src/routes/`.
   A route may only import a feature's `queries.ts` (for `ensureQueryData` in `beforeLoad`/
   `loader`) and its `components/`.
4. **`types.ts` mirrors the Go side**, with a comment naming the source so drift is findable:
   `// mirrors auth.Principal in backend/auth/models.go`.
5. **Never import another feature's internals.** Cross-feature use goes through `queries.ts`/
   `types.ts` only, never another feature's `api.ts`.

### Data loading pattern

Reads are kicked off in a route's `loader`/`beforeLoad` via `queryClient.ensureQueryData(...)`
and consumed in the component with `useSuspenseQuery` — no loading branch needed, since the
router already waited for it. Auth/authorization guards live in `beforeLoad` and `throw
redirect(...)` (note: **throw**, not just call — a bare `redirect(...)` is silently a no-op).
See `src/routes/_authenticated.tsx` for the guard and `src/routes/(auth)/login.tsx` for the
"redirect away if already logged in" case.

### Error handling

`apiFetch`/`post` throw `ApiError { code, message, status, fields? }` on any non-2xx response.
`fields` is populated on backend validation failures (`fields: [{field, message}]`); use
`error.fieldErrors()` to get a `{field: message}` map for a form library's `errors` prop.

Note the backend has one quirk: request-validation failures return HTTP 400 with **`code:
"internal"`** (validation errors aren't wrapped as a `publicError` — see
`backend/httpx/errors.go`) — so branch on `status === 400 && fields?.length`, not on `code`, to
detect them. The `fields` array itself is always populated correctly regardless.

No CSRF token exists and none is planned (no CORS either — session cookie is `SameSite=Lax`
under a single origin). Sessions are HttpOnly cookies with a 3h idle timeout and no refresh
endpoint; the only way to detect expiry is a 401 from a real request, which is why `me`'s query
options set `refetchOnWindowFocus: true`.

## Routing

File-based routes live in `src/routes/`. `src/routes/_authenticated.tsx` is a pathless layout
route that guards everything nested under `src/routes/_authenticated/` behind a valid session;
`src/routes/(auth)/` holds the unauthenticated login/register routes. Add a new route by adding
a file — `bun run generate-routes` (or the running dev server) regenerates
`src/routeTree.gen.ts`, which is generated and should never be hand-edited.
