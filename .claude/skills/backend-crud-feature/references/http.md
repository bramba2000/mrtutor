# HTTP layer template

`backend/features/<name>/<name>http/handlers.go`. This is the only layer that imports `net/http`, `httpx`, and `authhttp` — everything below it stays transport-free.

```go
package <name>http

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/<name>"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

// Service is the consumer-side interface: only what this handler calls,
// declared here rather than imported from <name>, so a fake can stand in
// for tests without a database.
type Service interface {
	Create(context.Context, <name>.CreateIn) (<name>.<Entity>, error)
	GetByID(context.Context, int) (<name>.<Entity>, error)
	GetAll(context.Context) ([]<name>.<Entity>, error)
	Update(context.Context, <name>.UpdateIn) (<name>.<Entity>, error)
	Delete(context.Context, int) error
	// one line per extra op, matching Service.<Op> in service.go
}

var _ Service = <name>.Service{}

type Handler struct {
	service       Service
	authenticator authhttp.Authenticator
	logger        *slog.Logger
}

func NewHandler(service Service, authenticator authhttp.Authenticator, logger *slog.Logger) Handler {
	return Handler{service: service, authenticator: authenticator, logger: logger}
}

func decode<Entity>ID(r *http.Request) (int, error) {
	val := r.PathValue("id")
	domainErr := errs.Domain("invalid", "invalid <entity> id", errs.Invalid)
	if val == "" {
		return 0, domainErr
	}
	id, err := strconv.Atoi(val)
	if err != nil {
		return 0, domainErr
	}
	return id, nil
}

func (h Handler) Mount(router *httpx.Router) {
	group := router.Group("/<name>", authhttp.RequireSession(h.authenticator, h.logger))

	group.Handle("GET /", httpx.WrapNoInput(h.service.GetAll, httpx.OK, h.logger))

	group.Handle("GET /{id}", httpx.WrapUnvalidated(decode<Entity>ID, h.service.GetByID, httpx.OK, h.logger))

	group.Handle("POST /", httpx.Wrap(httpx.BodyDecoder, h.service.Create, httpx.Created, h.logger))

	group.Handle("DELETE /{id}", httpx.WrapUnvalidated(
		decode<Entity>ID,
		func(ctx context.Context, id int) (struct{}, error) {
			return struct{}{}, h.service.Delete(ctx, id)
		},
		httpx.NoContent,
		h.logger,
	))

	group.Handle("PUT /{id}", httpx.Wrap(
		func(r *http.Request) (<name>.UpdateIn, error) {
			id, err := decode<Entity>ID(r)
			if err != nil {
				return <name>.UpdateIn{}, err
			}
			body, err := httpx.BodyDecoder[<name>.UpdateIn](r)
			if err != nil {
				return <name>.UpdateIn{}, err
			}
			body.ID = id
			return body, nil
		},
		h.service.Update,
		httpx.OK,
		h.logger,
	))

	// One group.Handle per extra op — see below for the two shapes.
}
```

## Choosing `Wrap` vs `WrapUnvalidated` vs `WrapNoInput`

- `WrapNoInput` — the route takes nothing from the request (`GET /`-style list, or an op with no parameters).
- `WrapUnvalidated` — the route decodes something (a path param, a query string) that has nothing to `Validate()` — an int ID is either well-formed or a 400 from the decoder itself, there's no separate validation step. Also used for `DELETE` here, matching `students`.
- `Wrap` — the route decodes a JSON body (or path param + body) into a type that implements `httpx.Validable`. Always use this for anything that reaches `CreateIn`, `UpdateIn`, or an extra write op's `*In` type — never wire a validated input type through `WrapUnvalidated`, or `Validate()` silently never runs (this was a real regression in `students` — see its handler tests).

## Extra ops

- Extra **read** with no input beyond the path/query: `WrapUnvalidated` with a small decode func, route method `GET`.
- Extra **read/write** with a body: `Wrap` with `httpx.BodyDecoder[YourInType]`, same shape as `POST /`.
- Route path: keep `/<name>` as the group prefix for everything; a sub-resource op gets its own path under that group (`GET /by-email/{email}`, `POST /{id}/archive`) — don't invent a second top-level group for one feature.
- Every route this handler registers must appear in `handlers_test.go`'s `TestRouting`-style coverage and in the integration test's lifecycle — an op with no route test is an op that can silently regress to 404/405.
