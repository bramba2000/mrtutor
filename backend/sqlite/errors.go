package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/bramba2000/mrtutor/backend/errs"
	"github.com/mattn/go-sqlite3"
)

func translateSQLError(op string, err error, notFound, conflict error) error {
	if err == nil {
		return nil
	}

	if notFound != nil && errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, errs.NotFound)
	}

	if serr, ok := errors.AsType[sqlite3.Error](err); ok {
		switch code := serr.Code; {
		case conflict != nil && code == sqlite3.ErrConstraint:
			return fmt.Errorf("%s: %w", op, conflict)
		}
	}

	return fmt.Errorf("%s: %v", op, err)
}
