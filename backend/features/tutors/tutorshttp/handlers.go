package tutorshttp

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/bramba2000/mrtutor/backend/features/auth"
	"github.com/bramba2000/mrtutor/backend/features/auth/authhttp"
	"github.com/bramba2000/mrtutor/backend/features/enrollments"
	"github.com/bramba2000/mrtutor/backend/features/students"
	"github.com/bramba2000/mrtutor/backend/features/tutors"
	"github.com/bramba2000/mrtutor/backend/httpx"
)

type Service interface {
	Create(context.Context, tutors.CreateIn) (tutors.Tutor, error)
	GetByID(context.Context, int) (tutors.Tutor, error)
	GetAll(context.Context) ([]tutors.Tutor, error)
	Update(context.Context, tutors.UpdateIn) (tutors.Tutor, error)
	Delete(context.Context, int) error
	GetByUserID(context.Context, int) (tutors.Tutor, error)
}

var _ Service = tutors.Service{}

// EnrollmentsService is the consumer-side port onto [enrollments.Service]
// needed to resolve which students are enrolled with a tutor.
type EnrollmentsService interface {
	GetByTutorID(context.Context, int) ([]enrollments.Enrollment, error)
}

var _ EnrollmentsService = enrollments.Service{}

// StudentsService is the consumer-side port onto [students.Service] needed
// to hydrate enrollments into full student records.
type StudentsService interface {
	GetByID(context.Context, int) (students.Student, error)
}

var _ StudentsService = students.Service{}

type Handler struct {
	service       Service
	enrollments   EnrollmentsService
	students      StudentsService
	authenticator authhttp.Authenticator
	logger        *slog.Logger
}

func NewHandler(service Service, enrollments EnrollmentsService, students StudentsService, authenticator authhttp.Authenticator, logger *slog.Logger) Handler {
	return Handler{
		service:       service,
		enrollments:   enrollments,
		students:      students,
		logger:        logger,
		authenticator: authenticator,
	}
}

// getStudentsByTutorID returns every student enrolled with the given tutor.
func (h Handler) getStudentsByTutorID(ctx context.Context, tutorID int) ([]students.Student, error) {
	links, err := h.enrollments.GetByTutorID(ctx, tutorID)
	if err != nil {
		return nil, err
	}
	result := make([]students.Student, 0, len(links))
	for _, link := range links {
		student, err := h.students.GetByID(ctx, link.StudentID)
		if err != nil {
			return nil, err
		}
		result = append(result, student)
	}
	return result, nil
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
	group.Handle("GET /me", httpx.WrapNoInput(
		func(ctx context.Context) (tutors.Tutor, error) {
			principal, ok := auth.FromContext(ctx)
			if !ok {
				return tutors.Tutor{}, auth.ErrUnauthenticated
			}
			return h.service.GetByUserID(ctx, principal.ID)
		},
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /{id}", httpx.WrapUnvalidated(
		decodeTutorID,
		h.service.GetByID,
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /{id}/students", httpx.WrapUnvalidated(
		decodeTutorID,
		h.getStudentsByTutorID,
		httpx.OK,
		h.logger,
	))
	group.Handle("POST /", httpx.Wrap(
		func(r *http.Request) (tutors.CreateIn, error) {
			in, err := httpx.BodyDecoder[tutors.CreateIn](r)
			if err != nil {
				return tutors.CreateIn{}, err
			}
			principal, ok := auth.FromContext(r.Context())
			if !ok {
				return tutors.CreateIn{}, auth.ErrUnauthenticated
			}
			in.UserId = principal.ID
			return in, nil
		},
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
