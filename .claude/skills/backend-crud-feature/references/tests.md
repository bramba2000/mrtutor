# Test templates

Four files, four different fakes, all following `students`' shape exactly. Every extra op needs the same three cases as a base op: happy path, the domain-error path (typically `ErrNotFound`), and — for anything with a `Validate()` — the validation-failure path.

## 1. `service_test.go` (package `<name>_test`)

An in-memory `fakeRepo` implementing `<name>.Repository`, with one `*Fn` override field per method (nil means "use the default in-memory behavior"), plus `*Called`/`Got*` fields so tests can assert on the exact value passed down without a mock library.

```go
type fakeRepo struct {
	db     map[int]<name>.<Entity>
	nextID int

	SaveFn    func(context.Context, <name>.<Entity>) (<name>.<Entity>, error)
	GetByIDFn func(context.Context, int) (<name>.<Entity>, error)
	GetAllFn  func(context.Context) ([]<name>.<Entity>, error)
	DeleteFn  func(context.Context, int) error

	SaveCalled bool
	GotSave    <name>.<Entity>
	// ... one *Called/Got* pair per method actually asserted on
}

func newFakeRepo() *fakeRepo { return &fakeRepo{db: make(map[int]<name>.<Entity>)} }

// Save mirrors the real upsert: zero ID assigns a new one + sets CreatedAt;
// a known non-zero ID preserves CreatedAt and sets ModifiedAt; an unknown
// non-zero ID inserts a new row under that ID (matches the SQL ON CONFLICT).
func (r *fakeRepo) Save(ctx context.Context, e <name>.<Entity>) (<name>.<Entity>, error) {
	r.SaveCalled = true
	r.GotSave = e
	if r.SaveFn != nil {
		return r.SaveFn(ctx, e)
	}
	if existing, ok := r.db[e.ID]; ok {
		e.CreatedAt = existing.CreatedAt
		e.ModifiedAt = time.Now().UTC()
	} else {
		if e.ID == 0 {
			r.nextID++
			e.ID = r.nextID
		}
		e.CreatedAt = time.Now().UTC()
	}
	r.db[e.ID] = e
	return e, nil
}

// GetByID/GetAll/Delete follow the same shape as students.fakeRepo — see
// backend/features/students/service_test.go if you need the exact bodies,
// but do not copy its ErrNotFound-on-missing-key convention verbatim
// without checking it matches this feature's ErrNotFound.

var _ <name>.Repository = (*fakeRepo)(nil)
```

Required `TestService` subtests, one `t.Run` block per method:

- **Create**: sets a non-zero ID and `CreatedAt`, leaves `ModifiedAt` zero, passes ID `0` to `Save`, propagates a repo error via `errors.Is`.
- **CreateIn.Validate**: table-driven — succeeds with only required fields, succeeds with every optional field populated, fails (and reports the right field name) for each individual required/format rule from `validate<Entity>Data`. Every failure asserts `errors.Is(err, errs.Invalid)`.
- **GetByID**: passes the returned value through, propagates `ErrNotFound`.
- **GetAll**: passes the returned slice through, propagates a repo error.
- **Update**: sets `ModifiedAt`, preserves `CreatedAt` from an existing row, creates a row when the ID is unknown (upsert), propagates a repo error.
- **UpdateIn.Validate**: same table-driven shape as `CreateIn.Validate` plus the `id < 1` case. Include a regression case asserting a fully valid `UpdateIn` returns `nil` without panicking — `validate<Entity>Data` returning a nil `validation.Errors` and a caller blindly re-wrapping it is a real bug class here.
- **Delete**: passes the ID through, propagates `ErrNotFound`.
- One block per extra op, same happy-path/error-path split.

## 2. `<name>sqlite/repository_test.go` (package `<name>sqlite_test`, external)

```go
func newRepo(t *testing.T) *<name>sqlite.Repository {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	db := sqlitetest.OpenTemp(t)
	return <name>sqlite.NewRepository(db)
}
```

Every method call goes through a real temp SQLite DB with migrations applied — this is what actually exercises the SQL. Required coverage: `Save` (create sets ID/CreatedAt and leaves ModifiedAt zero; update preserves CreatedAt and sets ModifiedAt; an unknown non-zero ID upserts a new row at that ID; an optional field round-trips as empty/zero when omitted; a malformed format field — e.g. a bad date string — is rejected with `errors.Is(err, errs.Invalid)`), `GetByID` (round-trips a seeded row; missing ID gives `errors.Is(err, <name>.ErrNotFound)` *and* `errors.Is(err, errs.NotFound)`), `GetAll` (multiple rows all present; empty table gives an empty, non-nil slice), `Delete` (row gone after delete; deleting a missing ID gives `ErrNotFound`). Same two assertions — the domain sentinel and the `errs` kind — for every extra op that can fail with a translated SQL error.

## 3. `<name>http/handlers_test.go` (package `<name>http_test`, external)

Three fakes, no database:

```go
type fakeService struct {
	createFn, getByIDFn, getAllFn, updateFn, deleteFn /* one field per method */
	createCalled bool /* one *Called bool + one gotX field per method actually asserted on */
}
// implements <name>http.Service

type fakeAuthenticator struct{}
const validToken = "valid-token"
func (fakeAuthenticator) Authenticate(ctx context.Context, token string) (auth.Principal, error) {
	if token == validToken {
		return auth.Principal{ID: 1, Username: "test"}, nil
	}
	return auth.Principal{}, auth.ErrUnauthenticated
}

func mounted(t *testing.T, svc <name>http.Service) *httpx.Router {
	t.Helper()
	r := httpx.NewRouter("")
	l := slog.New(slog.NewTextHandler(t.Output(), nil))
	<name>http.NewHandler(svc, fakeAuthenticator{}, l).Mount(r)
	return r
}

func authed(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: "session", Value: validToken})
	return req
}
```

Also assert the interface/`Validable` wiring statically:
```go
var (
	_ <name>http.Service = (*fakeService)(nil)
	_ <name>http.Service = <name>.Service{}
	_ httpx.Validable     = <name>.CreateIn{}
	_ httpx.Validable     = <name>.UpdateIn{}
)
```

Per route (`TestGetAll`, `TestGetByID`, `TestCreate`, `TestUpdate`, `TestDelete`, and one per extra op), cover: success (right status, right body, the decoded input reached the fake service), unauthenticated → 401 and the service was never called, the domain not-found error → 404 with the right error `code` in the JSON body, and for write routes: malformed JSON → 400 `invalid.json`, missing `Content-Type` → 415, a validation failure → 400 with the offending field name present in `fields`. Add a `TestRouting` case hitting an unregistered method on `/{id}` and expecting 405. For `PUT`, assert the path ID overrides any ID in the body — that's the one place a `students` regression actually happened (`PUT` was wired through `WrapUnvalidated`, skipping `Validate()` entirely).

## 4. `test/integration/<name>_test.go` (package `integration`)

Real db, real router, real auth — reuses `skipIfNotIntegration`, `newJSONRequest`, `seedPrincipal`, `authenticateRequest` from `test/integration/utils_test.go`; don't redefine those.

```go
func Test<Entity>s(t *testing.T) {
	skipIfNotIntegration(t)

	logger := slog.New(slog.NewTextHandler(t.Output(), &slog.HandlerOptions{Level: slog.LevelDebug}))
	db := sqlitetest.OpenTemp(t)

	authStorage := authsqlite.Build(db)
	authService := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)

	repo := <name>sqlite.NewRepository(db)
	svc := <name>.NewService(repo)

	router := httpx.NewRouter("")
	<name>http.NewHandler(svc, authService, logger).Mount(router)

	principal := seedPrincipal(t, authStorage.PrincipalStore, "<name>", "Test00!")

	// One t.Run per route asserting 401 when unauthenticated (Create, GetAll, Update, Delete, every extra op).

	t.Run("Full lifecycle when authenticated", func(t *testing.T) {
		var id int
		t.Run("Create", func(t *testing.T) { /* POST, assert 201, capture id */ })
		t.Run("Get by id", func(t *testing.T) { /* GET /{id}, assert 200 */ })
		t.Run("Appears in the list", func(t *testing.T) { /* GET /, assert id present */ })
		// one t.Run per extra op here, mid-lifecycle
		t.Run("Update", func(t *testing.T) { /* PUT, assert 200 + fields changed + CreatedAt survived */ })
		t.Run("Update with an invalid body fails validation", func(t *testing.T) { /* assert 400 */ })
		t.Run("Delete", func(t *testing.T) { /* DELETE, assert 204 */ })
		t.Run("Get by id now returns 404", func(t *testing.T) {})
		t.Run("Delete again now returns 404", func(t *testing.T) {})
	})
}
```
