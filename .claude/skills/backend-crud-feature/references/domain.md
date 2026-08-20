# Domain layer template

`backend/features/<name>/domain.go`. Package `<name>` (e.g. `students`). This file has no dependency on sqlite, sqlc, or net/http — it is the vocabulary every other layer imports.

```go
package <name>

import (
	"context"
	"time"

	"github.com/bramba2000/mrtutor/backend/errs"
)

type <Entity> struct {
	ID int `json:"id"`
	// One field per attribute from the spec. json tags are camelCase.
	<Field> <Type> `json:"<field>"`

	CreatedAt  time.Time `json:"createdAt"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

var (
	ErrNotFound = errs.Domain("<name>.notFound", "<Entity> not found", errs.NotFound)
	// One more per extra domain error an op needs, e.g.:
	// ErrDuplicateEmail = errs.Domain("<name>.duplicateEmail", "Email already in use", errs.Conflict)
)

type Repository interface {
	// GetByID returns a <entity> by ID, or ErrNotFound if not found.
	GetByID(ctx context.Context, id int) (<Entity>, error)
	// GetAll returns all <entity_plural>, or an empty slice if none found.
	GetAll(ctx context.Context) ([]<Entity>, error)
	// Save saves a <entity>. Saving with an existing ID updates that row,
	// otherwise it creates a new one. CreatedAt and ModifiedAt are set by
	// the repository, in UTC.
	Save(ctx context.Context, <entity> <Entity>) (<Entity>, error)
	// Delete deletes a <entity> by ID, or ErrNotFound if not found.
	Delete(ctx context.Context, id int) error

	// One method per extra op from the spec, e.g.:
	// GetByEmail(ctx context.Context, email string) (<Entity>, error)
}
```

## Notes

- `<Entity>.CreatedAt`/`ModifiedAt` json tags must be `createdAt`/`modifiedAt`. (The `students` feature has a `create_at` typo — do not copy it into new features.)
- A field with a domain-specific string format (email, phone, date) stays a plain `string` in the domain struct — parsing/format enforcement happens in `validation` (service layer) and, for dates, at the sqlite boundary (see `sqlite.md`). Don't reach for `time.Time` on a field the spec describes as optional free-form text with a format, only for genuinely temporal fields.
- `Save` is always an upsert keyed on ID, matching every existing feature (`auth` sessions, `students`). Don't introduce a separate `Create`/`Update` pair on the `Repository` interface — that split belongs to the service layer's input types, not the storage contract.
- Every extra *read* op returns `(<Entity>, error)` or `([]<Entity>, error)` plus whatever `ErrNotFound`-style sentinel it needs. Every extra *write* op takes and returns `<Entity>` (or a subset) the same way `Save`/`Delete` do — keep the storage contract in domain vocabulary, never in wire (JSON) or SQL vocabulary.
