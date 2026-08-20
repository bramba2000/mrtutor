package tutorshttp

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

type Service interface {
	Create(context.Context, tutors.CreateIn) (tutors.Tutor, error)
	GetByID(context.Context, int) (tutors.Tutor, error)
	GetAll(context.Context) ([]tutors.Tutor, error)
	Update(context.Context, tutors.UpdateIn) (tutors.Tutor, error)
	Delete(context.Context, int) error
}

var _ Service = tutors.Service{}

type Handler struct {
	service       Service
	authenticator authhttp.Authenticator
	logger        *slog.Logger
}

func NewHandler(service Service, authenticator authhttp.Authenticator, logger *slog.Logger) Handler {
	return Handler{
		service:       service,
		logger:        logger,
		authenticator: authenticator,
	}
}

func decodeTutorID(r *http.Request) (int, error) {
	val := r.PathValue("id")
	domainErr := errs.Domain("invalid", "invalid tutor id", errs.Invalid)
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
	group := router.Group("/tutors", authhttp.RequireSession(h.authenticator, h.logger))
	group.Handle("GET /", httpx.WrapNoInput(
		h.service.GetAll,
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /{id}", httpx.WrapUnvalidated(
		decodeTutorID,
		h.service.GetByID,
		httpx.OK,
		h.logger,
	))
	group.Handle("POST /", httpx.Wrap(
		httpx.BodyDecoder,
		h.service.Create,
		httpx.Created,
		h.logger,
	))
	group.Handle("DELETE /{id}", httpx.WrapUnvalidated(
		decodeTutorID,
		func(ctx context.Context, id int) (struct{}, error) {
			return struct{}{}, h.service.Delete(ctx, id)
		},
		httpx.NoContent,
		h.logger,
	))
	group.Handle("PUT /{id}", httpx.Wrap(
		func(r *http.Request) (tutors.UpdateIn, error) {
			id, err := decodeTutorID(r)
			if err != nil {
				return tutors.UpdateIn{}, err
			}
			tutor, err := httpx.BodyDecoder[tutors.UpdateIn](r)
			if err != nil {
				return tutors.UpdateIn{}, err
			}
			tutor.ID = id
			return tutor, nil
		},
		h.service.Update,
		httpx.OK,
		h.logger,
	))
}
