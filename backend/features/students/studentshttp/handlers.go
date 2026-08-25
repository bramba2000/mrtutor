package studentshttp

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
	Create(context.Context, students.CreateIn) (students.Student, error)
	GetByID(context.Context, int) (students.Student, error)
	GetAll(context.Context) ([]students.Student, error)
	GetDistinctSchools(context.Context) ([]string, error)
	GetDistinctStudyPrograms(context.Context) ([]string, error)
	GetDistinctClasses(context.Context) ([]string, error)
	Update(context.Context, students.UpdateIn) (students.Student, error)
	Delete(context.Context, int) error
}

var _ Service = students.Service{}

// TutorsService is the consumer-side port onto [tutors.Service] needed to
// resolve the tutor profile of the currently signed-in user.
type TutorsService interface {
	GetByUserID(context.Context, int) (tutors.Tutor, error)
}

var _ TutorsService = tutors.Service{}

// EnrollmentsService is the consumer-side port onto [enrollments.Service]
// needed to enroll a newly created/updated student with the signed-in tutor.
type EnrollmentsService interface {
	Link(context.Context, int, int) (enrollments.Enrollment, error)
}

var _ EnrollmentsService = enrollments.Service{}

type Handler struct {
	service       Service
	tutors        TutorsService
	enrollments   EnrollmentsService
	authenticator authhttp.Authenticator
	logger        *slog.Logger
}

func NewHandler(service Service, tutors TutorsService, enrollments EnrollmentsService, authenticator authhttp.Authenticator, logger *slog.Logger) Handler {
	return Handler{
		service:       service,
		tutors:        tutors,
		enrollments:   enrollments,
		logger:        logger,
		authenticator: authenticator,
	}
}

// enrollWithCurrentTutor links the given student with the tutor profile of
// the currently signed-in user.
func (h Handler) enrollWithCurrentTutor(ctx context.Context, studentID int) error {
	principal, ok := auth.FromContext(ctx)
	if !ok {
		return auth.ErrUnauthenticated
	}
	tutor, err := h.tutors.GetByUserID(ctx, principal.ID)
	if err != nil {
		return err
	}
	_, err = h.enrollments.Link(ctx, tutor.ID, studentID)
	return err
}

// create creates a student and enrolls it with the signed-in tutor.
func (h Handler) create(ctx context.Context, in students.CreateIn) (students.Student, error) {
	student, err := h.service.Create(ctx, in)
	if err != nil {
		return students.Student{}, err
	}
	if err := h.enrollWithCurrentTutor(ctx, student.ID); err != nil {
		return students.Student{}, err
	}
	return student, nil
}

// update updates a student and enrolls it with the signed-in tutor.
func (h Handler) update(ctx context.Context, in students.UpdateIn) (students.Student, error) {
	student, err := h.service.Update(ctx, in)
	if err != nil {
		return students.Student{}, err
	}
	if err := h.enrollWithCurrentTutor(ctx, student.ID); err != nil {
		return students.Student{}, err
	}
	return student, nil
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
	group.Handle("GET /schools", httpx.WrapNoInput(
		h.service.GetDistinctSchools,
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /study-programs", httpx.WrapNoInput(
		h.service.GetDistinctStudyPrograms,
		httpx.OK,
		h.logger,
	))
	group.Handle("GET /classes", httpx.WrapNoInput(
		h.service.GetDistinctClasses,
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
		h.create,
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
		h.update,
		httpx.OK,
		h.logger,
	))
}
