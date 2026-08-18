package studentshttp

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

type Service interface {
	Create(context.Context, students.CreateIn) (students.Student, error)
	GetByID(context.Context, int) (students.Student, error)
	GetAll(context.Context) ([]students.Student, error)
	Update(context.Context, students.UpdateIn) (students.Student, error)
	Delete(context.Context, int) error
}

var _ Service = students.Service{}

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

func decodeStudentID(r *http.Request) (int, error) {
	val := r.PathValue("id")
	domainErr := errs.Domain("invalid", "invalid student id", errs.Invalid)
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
	group := router.Group("/students", authhttp.RequireSession(h.authenticator, h.logger))
	group.Handle("GET /", httpx.WrapNoInput(
		h.service.GetAll,
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /{id}", httpx.WrapUnvalidated(
		decodeStudentID,
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
		decodeStudentID,
		func(ctx context.Context, id int) (struct{}, error) {
			return struct{}{}, h.service.Delete(ctx, id)
		},
		httpx.NoContent,
		h.logger,
	))
	group.Handle("PUT /{id}", httpx.Wrap(
		func(r *http.Request) (students.UpdateIn, error) {
			id, err := decodeStudentID(r)
			if err != nil {
				return students.UpdateIn{}, err
			}
			student, err := httpx.BodyDecoder[students.UpdateIn](r)
			if err != nil {
				return students.UpdateIn{}, err
			}
			student.ID = id
			return student, nil
		},
		h.service.Update,
		httpx.OK,
		h.logger,
	))
}
