# Service layer template

`backend/features/<name>/service.go`. Same package as `domain.go`. This is where validation lives — the repository trusts its input completely; the service is the only layer that doesn't.

```go
package <name>

import (
	"context"

	"github.com/bramba2000/mrtutor/backend/validation"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return Service{repo: repo}
}

// <Entity>Data holds every field settable through Create or Update.
type <Entity>Data struct {
	<Field> <Type> `json:"<field>"`
}

func validate<Entity>Data(d <Entity>Data) validation.Errors {
	return validation.Errors{
		// validation.Validate(...) for a required field, validation.Optional(...) for
		// an optional one. Chain validation.NotBlank, validation.MaxLength[string](N),
		// validation.Email, validation.Phone, validation.Date, validation.Min/Max, etc.
		// per field, matching the migration's NOT NULL/nullable choice.
		"<field>": validation.Validate(d.<Field>, validation.NotBlank, validation.MaxLength[string](256)),
	}
}

type CreateIn <Entity>Data

func (in CreateIn) Validate() error {
	return validate<Entity>Data(<Entity>Data(in)).Err()
}

// Create creates a new <entity> and returns it with its assigned ID.
func (s Service) Create(ctx context.Context, in CreateIn) (<Entity>, error) {
	e := <Entity>{
		ID: 0,
		// copy every field from in
	}
	return s.repo.Save(ctx, e)
}

// GetByID retrieves a <entity> by its ID.
func (s Service) GetByID(ctx context.Context, id int) (<Entity>, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll retrieves every <entity>.
func (s Service) GetAll(ctx context.Context) ([]<Entity>, error) {
	return s.repo.GetAll(ctx)
}

type UpdateIn struct {
	<Field> <Type> `json:"<field>"`
	ID      int    `json:"-"`
}

func (in UpdateIn) Validate() error {
	err := validation.Errors{
		"id": validation.Validate(in.ID, validation.Min(1)),
	}
	err.Merge(validate<Entity>Data(<Entity>Data{ /* copy fields */ }))
	return err.Err()
}

// Update updates an existing <entity> and returns the updated value.
func (s Service) Update(ctx context.Context, in UpdateIn) (<Entity>, error) {
	e := <Entity>{
		ID: in.ID,
		// copy every field from in
	}
	return s.repo.Save(ctx, e)
}

// Delete removes a <entity> by its ID.
func (s Service) Delete(ctx context.Context, id int) error {
	return s.repo.Delete(ctx, id)
}
```

## Extra ops

- An extra **read** op (`GetByEmail`, `ListByClass`...) is a thin pass-through exactly like `GetByID`/`GetAll` — no `*In` type, no `Validate`, unless it takes a filter value worth validating (e.g. reject a blank email before hitting the DB), in which case give it its own tiny `*In` struct with `Validate()` the same shape as `CreateIn`.
- An extra **write** op (`Archive`, a status transition, a bulk action...) gets its own `*In` type with `Validate()` following the `CreateIn`/`UpdateIn` pattern — validate everything the op accepts, then call the matching `Repository` method. Don't fold it into `Create`/`Update` with an optional field; a distinct operation gets a distinct method.
- `validate<Entity>Data` is the single source of truth for field rules — `CreateIn` and `UpdateIn` (and any extra write op's input that shares fields) call into it and `.Merge()` their own extra checks (like the `id` check in `UpdateIn.Validate`) on top. Never duplicate a field's validator list in more than one place.
