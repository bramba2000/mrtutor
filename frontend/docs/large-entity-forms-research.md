# Research: handling large entity forms in React (and beyond)

> Research notes, not a spec. Written 2026-08-18 against the frontend stack in this repo
> (React 19 + TanStack Router + TanStack Query + Mantine, see `frontend/README.md`) to inform
> how a "large-ish" entity form (e.g. the `students` feature, `backend/features/students/domain.go`
> — 7 editable fields plus server-owned `id`/`createdAt`/`modifiedAt`) should be built. Placed in
> `frontend/docs/` to mirror the existing `backend/docs/` convention (`architecture-review.md`,
> `architecture-tasks.md`) for point-in-time design/research documents, since no `docs/research/`
> or repo-root `docs/` convention exists yet.

Sources are primary wherever one exists (library docs, spec text, official repos). Where a
library's docs page returned HTTP 403 to automated fetching (all of `react-hook-form.com`
returned 403), the claim is instead sourced from a `WebSearch` result whose result snippet
quotes the same official page — the URL cited is still the primary docs page, and quotes are
kept short/paraphrased accordingly. This is flagged inline.

---

## 1. Form libraries: headless vs UI-coupled, validation integration, performance

### Headless vs UI-coupled

- **React Hook Form (RHF)** is explicitly built around **uncontrolled inputs**: `register`
  attaches a DOM `ref` directly to the input rather than driving its `value` via React state.
  Per the official docs (via `https://react-hook-form.com/get-started` and
  `https://react-hook-form.com/docs/useform`, fetched via search-result snippet due to 403 on
  direct fetch): *"React Hook Form relies on an uncontrolled form, which is why the register
  function captures a ref directly instead of value/onChange,"* and this *"reduces the number of
  re-renders that occur due to a user typing in an input or other form values changing at the
  root of your form."* Components using `register` also *"mount to the page faster than
  controlled components because they have less overhead."* For fields that must stay controlled
  (e.g. wrapping a Mantine `TextInput`/`Select`), `Controller`/`useController`
  (`https://react-hook-form.com/docs/usecontroller`) **isolate the re-render to just that field**
  instead of the whole form.
- **TanStack Form** is also headless by design: *"headless UI components, and a
  framework-agnostic design"* working across React/Vue/Angular/Solid/Svelte/Lit
  (`https://tanstack.com/form/latest/docs/overview`). Its reactivity model is **granular
  subscription per field** rather than uncontrolled refs — the comparison page
  (`https://tanstack.com/form/latest/docs/comparison`) lists "Granular reactivity" as a
  differentiator versus Formik/Redux Form, and each `<form.Field>` subscribes to its own slice of
  state so typing in one field doesn't re-render others.
- **Formik** is explicitly **controlled**: it manages `values`/`errors`/`touched` in React state
  and drives inputs through `handleChange`/`handleBlur` (`https://formik.org/docs/overview`).
  Formik's own docs justify this design (*"form state is inherently ephemeral and local"* — no
  external store needed) but do not claim it scales past controlled-input re-render costs; the
  practical implication (confirmed independently by TanStack's comparison page framing) is that
  **every keystroke re-renders the field's owning component tree** unless the consumer manually
  memoizes subcomponents.
- **Conform** (`https://conform.guide/`) takes a third approach: it is **not** primarily a
  React-state library at all — it captures values via the native `FormData` Web API and syncs
  state through event delegation, explicitly to progressively enhance plain HTML forms rather
  than replace browser form behavior. It advertises "fine-grained subscription" and "automatic
  type coercion with Zod" via `parseWithZod()`, shared between client and server parsing.
- **Native React 19 (`useActionState` + `<form action>`)**: react.dev
  (`https://react.dev/reference/react-dom/components/form`,
  `https://react.dev/reference/react/useActionState`) documents passing a function directly to a
  `<form>`'s `action` prop. React then: runs the submission inside a `Transition`; passes the
  submitted `FormData` as an argument; auto-resets uncontrolled fields on success; and, combined
  with `useActionState`, exposes `[state, dispatchAction, isPending]` with `isPending` tracked
  automatically (no manual `startTransition`). This is **not a full form-state library** — there
  is no field registry, no per-field error map — it's a primitive for wiring one submit handler
  to pending/error state, most valuable when paired with Server Functions for progressive
  enhancement (form keeps working before hydration via the optional `permalink` argument). For a
  fully client-rendered SPA with no server actions (this repo's setup — see `frontend/README.md`,
  Vite proxies `/api` to a separate Go server, so there are no React Server Functions), this
  buys pending-state ergonomics but nothing for multi-field validation/defaults; you'd still pair
  it with a schema and manual field wiring.

### Validation integration (schema resolvers)

- RHF integrates schema libraries through a `resolver` option; **Zod** is the most common pairing
  via `@hookform/resolvers`'s `zodResolver`
  (`https://github.com/react-hook-form/resolvers`). The resolvers repo documents a
  `standardSchemaResolver` implementing the **Standard Schema** spec, letting one resolver work
  with Zod, Valibot, ArkType, etc. interchangeably.
- **TanStack Form** natively supports **Standard Schema** without an adapter package: *"TanStack
  Form natively supports all libraries following the Standard Schema specification"*
  (`https://tanstack.com/form/latest/docs/framework/react/guides/validation`), and schemas can be
  attached either per-field (each `<Field>` takes its own `validators`) or once at the form level
  (`useForm({ validators: { onChange: schema } })`), with form-level errors auto-propagating to
  fields — but a field-level validator's result **overrides** the form-level one if both exist.
- **Formik** integrates **Yup** specifically via its `validationSchema` prop, which Formik
  transforms into Formik's own `errors`/`touched` shape automatically
  (`https://formik.org/docs/overview`).
- **Conform**'s `parseWithZod()` (`https://conform.guide/`) is notable for running the *same* Zod
  schema on the client (for instant feedback) and on the server (as the actual authority) —
  collapsing "wire validation" and "form validation" into one schema, which is only really
  coherent when client and server are in the same language/runtime (Remix/Next.js), not
  applicable as-is to a Go backend.
- A **type-safety subtlety** worth carrying into any Zod+RHF usage: when a Zod schema
  transforms/coerces (`z.coerce.number()`, `.transform()`), the schema has two shapes —
  `z.input<T>` (pre-transform, what the user types) and `z.output<T>`/`z.infer<T>` (post-parse).
  RHF's field state holds the **input** shape; the resolver's output — what `onSubmit` receives —
  is the **output** shape. RHF's own maintainers document threading this through
  `useForm`'s third generic (`TTransformedValues`):
  `useForm<z.input<typeof schema>, any, z.output<typeof schema>>({ resolver: zodResolver(schema) })`
  (sourced from `https://github.com/orgs/react-hook-form/discussions/8496` and
  `https://react-hook-form.com/docs/useform/handlesubmit`, both on the official
  `react-hook-form` GitHub org). This is directly relevant to angle 2 below — it's the
  library-level mechanism for "the form's internal shape and the shape sent over the wire are
  legitimately different types."

### Performance characteristics for large forms

- The core, repeatedly-stated tradeoff: **controlled = simpler code, worse re-render scaling as
  field count grows; uncontrolled/granular-subscription = faster, more moving parts.** RHF's own
  positioning (`get-started` page, via search snippet) is explicit that avoiding re-renders "at
  the root of your form" was a primary design motivation. Formik's docs
  (`https://formik.org/docs/overview`) instead lean on being lighter-weight than Redux-Form
  (12.7 kB vs 22.5 kB gzipped) and warn that Redux-Form-style architectures cause "input latency
  [to] continue to increase" as forms grow — the same underlying problem (too much re-render
  surface per keystroke), addressed by a different lever (avoid a global store, not avoid
  controlled state).
- For a form with 7–15 fields (`students` scale), the re-render difference between an RHF-style
  uncontrolled form and a naively-controlled one is unlikely to be perceptible; the performance
  argument for headless/uncontrolled libraries matters most at **50+ fields, dynamic field
  arrays, or fields with expensive per-keystroke side effects** (live formatting, cross-field
  recalculation). Below that, correctness/DX/bundle-size concerns dominate the choice more than
  raw re-render counts — none of the fetched primary sources contradict this; they simply don't
  make performance claims scoped to small forms at all.
- This repo's stack currently has **no form library installed** (`frontend/package.json` has no
  `react-hook-form`, `@tanstack/react-form`, `formik`, or `zod`; only `@mantine/core`/hooks,
  `@tanstack/react-query`/`react-router`). Mantine ships its own headless-ish hook,
  **`@mantine/form`**'s `useForm` (`https://mantine.dev/form/use-form/`), which supports both a
  "controlled" and an "uncontrolled" mode (the docs' own wording) and integrates directly with
  Mantine inputs with no wrapper needed — the path of least friction given `@mantine/core` is
  already a dependency, at the cost of being Mantine-specific rather than headless/portable.

---

## 2. Wire (DTO) model vs form model — is a mapping layer justified?

This repo already has a documented rule that is directly relevant here, from
`frontend/README.md`: `types.ts` per feature holds **wire types, hand-mirrored from the Go
DTOs**, with a comment naming the Go source so drift is findable (e.g. `// mirrors auth.Principal
in backend/auth/models.go`). That's the wire-model half of the pattern already in place for
`auth`; the question this research angle answers is whether the *form* model should be a further,
separate shape from that wire model.

- **Yes, in general, once dates/numbers or optionality are involved** — and the `students` entity
  has both: `BirthDate string // in the format of YYYY-MM-DD` (a string on the wire, but a date
  picker component wants a `Date`/similar client type) and required-vs-optional shape (Go's
  `Student` struct has no `omitempty`/pointer fields — every field is a plain non-nullable
  string/int on the wire — but the *create* form's in-progress state before submit is naturally
  "possibly empty," which is a different type than "validated, ready to POST").
- **Zod's own vocabulary formalizes exactly this split**: `.optional()` allows `undefined`,
  `.nullable()` allows `null`, `.nullish()` allows either (`https://zod.dev/` API reference,
  fetched directly). These are deliberately different concepts from "not required in the UI" —
  a backend DTO's Go `string` (never null, but can be `""`) does not map 1:1 onto either.
  Handling this well means the *wire* schema should mirror the Go JSON tags' actual nullability
  (nothing is ever `null` here, per the `students` struct's plain `string`/`time.Time` fields with
  no pointers), while the *form* schema can independently choose to allow an empty/undefined
  transient state while the user is filling it in.
- **`.partial()` is Zod's primitive for PATCH-style updates**: calling `.partial()` on an object
  schema makes every key optional, or `.partial({ field: true })` targets specific keys
  (`https://zod.dev/`). This is the natural schema-level expression of "same entity, different
  wire contract for create (full payload) vs patch (sparse payload)" — i.e. one base schema plus
  a derived partial variant, rather than two hand-maintained schemas.
- **`.preprocess()` / `z.coerce.*`** are Zod's mechanism for the date/number formatting mismatch
  specifically: form inputs arrive as strings (from `<input>` or `FormData`), and
  `z.coerce.number()` / a `z.preprocess` step converts them before the "real" schema validates
  (`https://zod.dev/`). For `students.BirthDate`, this is the natural place to convert a date
  object back to the `YYYY-MM-DD` string format the Go handler expects, keeping that
  serialization concern in one schema-adjacent spot rather than scattered through the component.
- **RHF's generic-typed resolver mechanism** (`useForm<TFieldValues, TContext,
  TTransformedValues>`, see §1) is the concrete embodiment of "form model in, wire model out" at
  the library level: the component tree works with `TFieldValues` (the pre-transform/form shape),
  and only the code that actually calls the mutation receives `TTransformedValues` (the
  wire-ready shape) — enforced by the type system, not just convention.
  (`https://github.com/orgs/react-hook-form/discussions/8496`,
  `https://react-hook-form.com/docs/useform/handlesubmit`).
- **Conform's stance is the outlier and worth naming as a contrast**: by running literally the
  same Zod schema via `parseWithZod()` on both client and server
  (`https://conform.guide/`), it argues *against* maintaining two schemas at all — but that only
  works when client and server share a runtime/schema language. With a Go backend, the "server"
  half of that equation doesn't exist in TypeScript, so this repo is structurally in the
  two-schema camp regardless of library choice: a Go-owned wire contract, mirrored by hand into
  `types.ts` per the existing README rule, and (if a schema-driven form library is adopted) a
  second, form-owned schema that's allowed to diverge for exactly the reasons above.

**Recommendation implied by the above, not asserted by any single source**: keep the existing
`types.ts` (wire) / component (form) split this repo already uses for `auth`, but if/when a
schema library is introduced for the `students` form, add a `schema.ts` (or equivalent) holding a
form-shaped Zod schema distinct from the hand-mirrored wire type — with an explicit mapping
function at the `api.ts` boundary (already the file with "the ONLY file with URL strings," per
`frontend/README.md`) converting one to the other, rather than trying to make one schema serve
both jobs.

---

## 3. UI patterns for large entity forms: modal/dialog vs always-editable vs inline click-to-edit

- **Modal/dialog forms.** The authoritative source is the W3C ARIA Authoring Practices Guide
  Dialog (Modal) pattern (`https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/`), which
  requires: `role="dialog"` with `aria-modal="true"`; a visible title referenced by
  `aria-labelledby` (or `aria-label`); focus moved to an element inside the dialog on open —
  either the first focusable control, or a static element (`tabindex="-1"`) such as the title
  when the dialog's content is complex enough that jumping straight to the first input would be
  disorienting; a contained tab sequence (Tab/Shift+Tab never leave the dialog while open); Escape
  closes it; and on close, focus **returns to the invoking element**, or to another logical
  element if the invoker no longer exists. The APG explicitly extends this to forms: "all elements
  required to operate the dialog are descendants" of the dialog container, i.e. the modal's
  focus trap must contain every form control, not just a subset.
  - Tradeoff: a modal is good for a bounded, all-or-nothing action ("create/edit this student")
    because it visually and semantically scopes the task and its own cancel/confirm actions, but
    it's a poor fit for a form long enough to need scrolling *inside* the trap, since users lose
    page-level scroll affordances and the trap makes escaping to check something outside the
    modal deliberately hard (by design, per the APG).
- **Always-editable inline fields ("no separate edit mode").** No single primary spec covers this
  pattern by name; it's a consequence of applying the same WCAG success criteria that govern any
  form. The relevant ones fetched directly from W3C: **SC 3.3.1 Error Identification**
  (`https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html`) requires that "the
  item that is in error is identified and the error is described to the user in text" — not color
  or icon alone — which matters more, not less, for always-visible fields, because there's no
  discrete "submit" moment to anchor a single review-and-fix step; errors need to surface
  per-field, continuously. And **`aria-live`** (WAI-ARIA 1.2,
  `https://www.w3.org/TR/wai-aria-1.2/#aria-live`) is the mechanism for announcing autosave
  confirmations or async validation results to screen reader users without moving their focus —
  necessary infrastructure for an always-editable/autosave pattern, since there's no page
  navigation or dialog-close event to naturally announce "saved."
  - Tradeoff: best for entities edited frequently in small increments (a settings page) where the
    cost of a full save flow per change is higher than the cost of building autosave +
    per-field validation + live-region status messaging.
- **Inline "click text to edit" pattern.** This has **no W3C/WAI-ARIA-named pattern** — it doesn't
  appear in the APG's pattern list, and targeted search
  (`GOV.UK Design System`, `design-system.service.gov.uk`) found no dedicated official pattern
  page either; the closest documented real-world precedent found was the UK Ministry of Justice
  Forms team's use of a plain `contenteditable` attribute to let users click text to edit it
  in-place (via search result summarizing `xgovformbuilder.github.io`'s MoJ Forms update, itself
  a government engineering-team writeup, not a W3C spec — flagged as the weakest-sourced claim in
  this document). In the absence of a dedicated spec, the applicable *general* requirements are
  the same WCAG criteria as any other control: the clickable text must be a real, keyboard-focusable, correctly-named
  interactive element (a `<button>`, not a `<span onClick>`) so it satisfies **name/role/value**
  requirements, and the transition into "edit mode" must be announced (`aria-live` or focus
  management, as above) since sighted users get an obvious visual cue that screen reader/keyboard
  users otherwise miss entirely.
  - Tradeoff: lowest visual footprint for read-heavy views with occasional edits to individual
    fields, but the highest *implementation* accessibility burden per-field (every field
    independently needs its own accessible edit-mode toggle, save/cancel affordance, and
    validation surfacing) — the opposite cost profile from a single modal, which pays that
    accessibility cost once for the whole entity.
- **Interaction with autosave/validation.** All three patterns can pair with autosave, but the
  *validation timing* differs by pattern: a modal naturally validates on submit/close (bounded
  action, SC 3.3.1's "the submission failed" framing applies directly); always-editable and
  inline-click patterns don't have a submit moment, so validation has to run on blur/change and
  be paired with `aria-live` status updates to remain WCAG-conformant without a page reload or
  dialog-close event to hang feedback on.

For the `students` entity specifically (7 fields, create-or-edit, not a frequently-drip-edited
settings-style entity), the modal/dialog pattern is the best-supported-by-spec choice: it's the
only one of the three with a complete, authoritative accessibility pattern (the APG dialog
pattern) rather than one assembled from general WCAG principles.

---

## 4. Default values: where they come from, and async-loaded defaults for edit forms

- **Client-side/schema-derived defaults** are literally what Zod's `.default()` is for: it
  supplies a fallback returned whenever the input is `undefined`, and can take a function that
  re-evaluates per call (`z.number().default(Math.random)`) rather than a fixed value
  (`https://zod.dev/`). This is the natural home for constants like "new student form starts with
  `class: ''`" if a schema-first approach is used — the default lives with the field definition,
  not duplicated in a component.
- **RHF: `defaultValues` (static, sync) vs `values` (reactive, for async data).** RHF's own docs
  (`https://react-hook-form.com/docs/useform`, confirmed via search snippet since direct fetch was
  blocked) distinguish two options that are easy to conflate: `defaultValues` sets the form's
  initial state **once**, at first render — if the real data for an edit form arrives
  asynchronously after that first render, `defaultValues` will not pick it up, and the form stays
  empty. `values` was added specifically to solve this: it's a prop RHF **keeps in sync** with an
  external source (e.g. a TanStack Query result) without an imperative `reset()` call — "if you
  refresh the data every 5 seconds, the form will auto-update." The alternative, imperative route
  is calling `reset(newData)` once data lands, which docs
  (`https://react-hook-form.com/docs/useform/reset`, via snippet) note also **updates what
  `defaultValues` means going forward** — a later bare `reset()` call reverts to the *new*
  defaults, not the original empty ones. By default, an async update to `values`/`defaultValues`
  resets the form's current values too; `resetOptions.keepDirtyValues: true` is the documented
  escape hatch to preserve in-progress user edits against an incoming data refresh.
- **TanStack Form: the documented pattern is "loading-gate, then mount the form."** The
  dedicated guide, fetched directly
  (`https://tanstack.com/form/latest/docs/framework/react/guides/async-initial-values`), pairs
  `useQuery` with `useForm`, and its stated best practice is to show a loading spinner and
  **not construct the form until the data has arrived** (`if (isLoading) return <p>Loading...</p>`
  before `useForm` is even meaningfully used with real values) — rather than RHF's approach of
  mounting immediately with placeholder defaults and letting `values` reactively update
  underneath. The guide also states the general fallback approach when a form-first render is
  unavoidable: give every field an "empty" version of its type while the request is pending — an
  empty string for text, today's date for dates, zero for numbers — rather than `undefined`.
- **Mantine's `useForm`** (`https://mantine.dev/form/use-form/`) takes a third, more manual
  approach: `initialValues` sets the baseline at construction, and for async-loaded data the
  documented flow is to call `form.setInitialValues(fetchedData)` once it arrives (updating what
  "initial" means, analogous to RHF's `reset(newData)`) followed by `form.reset()` to actually
  apply those as the current values — two explicit calls rather than either RHF's reactive
  `values` prop or TanStack Form's loading-gate.
- **Practical synthesis for an edit-existing-entity form** (relevant to a future `students` edit
  form, sourced by combining the three library docs above rather than any single one): the two
  documented-safe strategies are (a) **don't render the form until the fetch resolves** (TanStack
  Form's stated pattern, trivially compatible with this repo's existing loader convention — routes
  already use `ensureQueryData` + `useSuspenseQuery` per `frontend/README.md`, so the "loading"
  branch is already handled at the router level before the form component even mounts), or
  (b) **mount immediately with empty/placeholder defaults and reactively sync** once data arrives
  (RHF's `values` prop, or Mantine's `setInitialValues` + `reset()` pair). Given this repo's
  routes already suspend on `ensureQueryData` before rendering feature components, option (a) is
  free — the async-loading problem these libraries' docs are solving is largely already solved
  one layer up, at the router boundary, for this codebase specifically.

---

## Notable source-access caveats

- Every direct `WebFetch` to `react-hook-form.com` (and its `www.` variant) returned HTTP 403.
  All RHF claims above are sourced via `WebSearch` result snippets that quote or closely
  paraphrase the named docs page; the URLs cited are the actual official pages, not secondary
  write-ups, but the exact wording could not be independently re-verified by fetching the page
  body directly in this session.
- No primary W3C/WAI-ARIA pattern exists for "click text to edit inline" — this is stated
  explicitly in §3 rather than glossed over, since the task's source requirements call for
  flagging where only secondary/weak sources exist.
- The GitHub discussions cited (`react-hook-form/discussions/8496`) are hosted on the official
  `react-hook-form` GitHub organization and represent maintainer-endorsed usage patterns, but are
  community discussion threads, not versioned docs pages — treated here as primary-adjacent
  (owned by the library's own org) rather than fully equivalent to a docs page.

## Sources

- React Hook Form: `https://react-hook-form.com/get-started`, `/docs/useform`,
  `/docs/useform/reset`, `/docs/useform/handlesubmit`, `/docs/usecontroller`
  (all via search-snippet due to 403 on direct fetch)
- React Hook Form resolvers: `https://github.com/react-hook-form/resolvers`
- React Hook Form Discussions: `https://github.com/orgs/react-hook-form/discussions/8496`
- TanStack Form: `https://tanstack.com/form/latest/docs/overview`, `/docs/comparison`,
  `/docs/framework/react/guides/validation`,
  `/docs/framework/react/guides/async-initial-values`
- Formik: `https://formik.org/docs/overview`
- Conform: `https://conform.guide/`
- Zod: `https://zod.dev/` (API reference: `.default()`, `.optional()`, `.nullable()`,
  `.nullish()`, `.partial()`, `.preprocess()`, `.coerce`)
- Mantine: `https://mantine.dev/form/use-form/`
- React: `https://react.dev/reference/react/useActionState`,
  `https://react.dev/reference/react-dom/components/form`
- W3C ARIA Authoring Practices Guide — Dialog (Modal) pattern:
  `https://www.w3.org/WAI/ARIA/apg/patterns/dialog-modal/`
- WCAG 2.2 Understanding — Error Identification (SC 3.3.1):
  `https://www.w3.org/WAI/WCAG22/Understanding/error-identification.html`
- WAI-ARIA 1.2 spec — `aria-live`: `https://www.w3.org/TR/wai-aria-1.2/#aria-live`
- Secondary, triangulation-only: MoJ Forms inline-editing writeup (via
  `xgovformbuilder.github.io`), for the "no official inline-edit pattern exists" gap noted in §3
