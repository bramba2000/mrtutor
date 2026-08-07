package sqlite

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"
)

func TranslateSQLError(op string, err error, notFound, conflict error) error {
	if err == nil {
		return nil
	}

	if notFound != nil && errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, notFound)
	}

	if serr, ok := errors.AsType[sqlite3.Error](err); ok {
		switch code := serr.Code; {
		case conflict != nil && code == sqlite3.ErrConstraint:
			return fmt.Errorf("%s: %w", op, conflict)
		}
	}

	return fmt.Errorf("%s: %v", op, err)
}
