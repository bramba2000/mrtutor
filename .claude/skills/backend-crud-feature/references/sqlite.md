# Migration, sqlc, and sqlite repository templates

Do these four in order — each depends on the previous one existing on disk.

## 1. Migration

`backend/sqlite/migrations/NNNN_create_<table>.sql`. `NNNN` is the highest existing prefix in that directory + 1, zero-padded to 4 digits (check with `ls backend/sqlite/migrations`, don't assume). `<table>` is the plural snake_case table name, same word as the feature name.

```sql
-- +goose Up
-- Create <table> table
CREATE TABLE <table> (
    id INTEGER PRIMARY KEY,
    <column> <SQL_TYPE> [NOT NULL],
    -- one column per field; nullable unless the spec marks it required

    created_at TIMESTAMP NOT NULL,
    modified_at TIMESTAMP
);

-- +goose Down
DROP TABLE <table>;
```

A required field gets `NOT NULL` on the column *and* `validation.NotBlank`/equivalent in the service layer (belt and suspenders — the DB constraint is the last line of defense, the service gives the client a field-level 400 instead of a raw SQL error). An optional string field is nullable with no `NotBlank` validator (see `service.md`).

## 2. `queries.sql`

`backend/features/<name>/<name>sqlite/queries.sql`. One query per base op, one per extra op. Use `sqlc.arg`/named params (`:field`) for `INSERT`/`UPDATE`, `?` for simple lookups — matches the existing files.

```sql
-- name: Get<Entity>ById :one
SELECT * FROM <table> WHERE id = ? LIMIT 1;

-- name: GetAll<Entity>s :many
SELECT * FROM <table>;

-- name: Delete<Entity> :execrows
DELETE FROM <table> WHERE id = ?;

-- name: Save<Entity> :one
-- Tries to insert a new <entity> record. If a record with the same id
-- already exists, updates the existing record instead.
INSERT INTO <table> (id, <column>, ..., created_at)
    VALUES (NULLIF(:id, 0), :<column>, ..., CURRENT_TIMESTAMP)
    ON CONFLICT (id) DO UPDATE SET
        <column> = EXCLUDED.<column>,
        ...
        modified_at = CURRENT_TIMESTAMP
    RETURNING *;

-- One block per extra op, e.g.:
-- name: Get<Entity>ByEmail :one
-- SELECT * FROM <table> WHERE email = ? LIMIT 1;
```

`:execrows` for a delete/affecting query is what lets the repository distinguish "0 rows affected" from a real error — see the repository template below.

## 3. `sqlc.yml`

Append one block to `backend/sqlc.yml` (do not touch the existing blocks):

```yaml
    - engine: sqlite
      schema: "sqlite/migrations/*.sql"
      queries: "features/<name>/<name>sqlite/*.sql"
      gen:
          go:
              package: gen
              out: "features/<name>/<name>sqlite/internal/gen"
              omit_unused_structs: true
```

Then run `task sqlc` (from `backend/`; equivalent to `go tool -modfile=go.tool.mod sqlc generate`) before writing `repository.go` — the generated `gen.Queries` / `gen.<Entity>` / `gen.Save<Entity>Params` types are what the repository compiles against. `internal/gen/` is reachable only from within `<name>sqlite/` (Go's `internal` rule) — that's deliberate encapsulation, not a bug to work around.

## 4. `repository.go`

`backend/features/<name>/<name>sqlite/repository.go`.

```go
package <name>sqlite

import (
	"context"

	"github.com/bramba2000/mrtutor/backend/features/<name>"
	"github.com/bramba2000/mrtutor/backend/features/<name>/<name>sqlite/internal/gen"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type Repository struct {
	R *gen.Queries
	W *gen.Queries
}

func NewRepository(r *sqlite.DB) *Repository {
	return &Repository{
		R: gen.New(r.R),
		W: gen.New(r.W),
	}
}

var _ <name>.Repository = Repository{}

// GetByID implements [<name>.Repository].
func (r Repository) GetByID(ctx context.Context, id int) (<name>.<Entity>, error) {
	got, err := r.R.Get<Entity>ById(ctx, int64(id))
	if err != nil {
		return <name>.<Entity>{}, sqlite.TranslateSQLError("<name>.GetByID", err, <name>.ErrNotFound, nil)
	}
	return to<Entity>(got), nil
}

// GetAll implements [<name>.Repository].
func (r Repository) GetAll(ctx context.Context) ([]<name>.<Entity>, error) {
	got, err := r.R.GetAll<Entity>s(ctx)
	if err != nil {
		return nil, sqlite.TranslateSQLError("<name>.GetAll", err, nil, nil)
	}
	result := make([]<name>.<Entity>, len(got))
	for i, v := range got {
		result[i] = to<Entity>(v)
	}
	return result, nil
}

// Save implements [<name>.Repository].
func (r Repository) Save(ctx context.Context, e <name>.<Entity>) (<name>.<Entity>, error) {
	model, err := r.W.Save<Entity>(ctx, gen.Save<Entity>Params{
		ID: int64(e.ID),
		// map every field; use sql.NullString{Valid: v != "", String: v} for
		// optional strings, sql.NullTime for optional dates parsed with
		// time.Parse(time.DateOnly, ...) (see students/studentssqlite for the
		// exact pattern if a date field needs it) — return
		// fmt.Errorf("<name>.Save: %w: %w", errs.Invalid, err) on parse failure.
	})
	if err != nil {
		return <name>.<Entity>{}, sqlite.TranslateSQLError("<name>.Save", err, nil, nil)
	}
	return to<Entity>(model), nil
}

// Delete implements [<name>.Repository].
func (r Repository) Delete(ctx context.Context, id int) error {
	affected, err := r.W.Delete<Entity>(ctx, int64(id))
	if err != nil {
		return sqlite.TranslateSQLError("<name>.Delete", err, <name>.ErrNotFound, nil)
	}
	if affected == 0 {
		return fmt.Errorf("<name>.Delete: %w", <name>.ErrNotFound)
	}
	return nil
}

func to<Entity>(m gen.<Entity>) <name>.<Entity> {
	return <name>.<Entity>{
		ID: int(m.ID),
		// map every field back, unwrapping sql.NullString/.NullTime as needed
		CreatedAt:  m.CreatedAt,
		ModifiedAt: m.ModifiedAt.Time,
	}
}
```

Every extra op gets the same three-part shape: call the matching generated query, `sqlite.TranslateSQLError(...)` on the way out (pass the feature's `ErrNotFound`/conflict sentinel as the second/third arg whenever `sql.ErrNoRows` or a unique-constraint violation is the expected failure), map through a `to<Entity>` helper on the way back. Never construct a `gen.*` type outside this file, and never let a `gen.*` type leak into `domain.go`, `service.go`, or the HTTP layer.
