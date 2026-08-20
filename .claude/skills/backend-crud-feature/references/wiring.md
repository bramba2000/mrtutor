# Composition-root wiring

This codebase's scaling gate (see `backend/docs/architecture-review.md` §7/§9 if you want the full argument): adding a feature must touch only its own new directories, one migration, one `sqlc.yml` block, and exactly these two files. Nothing under `httpx/`, nothing under `sqlite/` besides the migration, no other feature's files.

## `backend/cmd/api/service.go`

Add the field and the two build lines — don't restructure the function, just extend it:

```go
type Services struct {
	Auth     auth.Service
	Students students.Service
	<Name>   <name>.Service   // add
}

func createServices(db *sqlite.DB) Services {
	authStorage := authsqlite.Build(db)
	auth := auth.NewService(authStorage.PrincipalStore, authStorage.SessionStore, authStorage.UnitOfWork)

	studentsStorage := studentssqlite.NewRepository(db)
	students := students.NewService(studentsStorage)

	<name>Storage := <name>sqlite.NewRepository(db)  // add
	<name> := <name>.NewService(<name>Storage)       // add

	return Services{
		Auth:     auth,
		Students: students,
		<Name>:   <name>,                              // add
	}
}
```

Plus the two new imports (`.../features/<name>`, `.../features/<name>/<name>sqlite`).

## `backend/cmd/api/routes.go`

Add one line to the `handlers` slice in `registerRoutes`:

```go
handlers := []interface{ Mount(*httpx.Router) }{
	authhttp.NewHandler(services.Auth, authhttp.Config{Secure: cfg.AppMode == config.AppModeProd}, logger),
	studentshttp.NewHandler(services.Students, services.Auth, logger),
	<name>http.NewHandler(services.<Name>, services.Auth, logger), // add
}
```

Plus the new import (`.../features/<name>/<name>http`).

## Why nothing else changes

- `httpx/` is domain-free by design — a feature never has a reason to edit it. If you find yourself wanting to, the new op probably fits one of `Wrap`/`WrapUnvalidated`/`WrapNoInput` already; re-read `http.md`.
- `sqlite/` (the package itself, not `sqlite/migrations/`) holds only connection management, migration running, and `TranslateSQLError` — all cross-feature infrastructure. A feature's own generated code and repository live under its own directory, never here.
- Migrations are the one deliberate exception: goose needs one globally ordered sequence, so the migration file is the only new content that lands outside the feature's own three directories.

## Verification (the gate, made checkable)

```
go build ./...      # from backend/ — compiles the new wiring
go vet ./...
go test -short ./...
go test ./...        # runs the integration test too
```

If any of these requires an edit outside the feature's directories + the migration + the three lines above, stop and flag it before making that edit — it means this feature doesn't fit the established shape, which is worth surfacing to the user rather than quietly patching around.
