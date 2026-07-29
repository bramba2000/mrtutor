package sqlite

import (
	"database/sql"
	"errors"

	"github.com/mattn/go-sqlite3"
)

type ErrorKind int

const (
	// ErrorKindUnknown represents an unknown error kind.
	ErrorKindUnknown ErrorKind = iota
	// ErrorKindNotFound represents a not found error kind.
	ErrorKindNotFound
	// ErrorKindUniqueConstraint represents a unique constraint violation error kind.
	ErrorKindUniqueConstraint
)

func KindOf(err error) ErrorKind {
	if err == nil {
		return ErrorKindUnknown
	}
	if err == sql.ErrNoRows {
		return ErrorKindNotFound
	}

	if sqliteErr, ok := errors.AsType[*sqlite3.Error](err); ok {
		switch sqliteErr.ExtendedCode {
		case sqlite3.ErrConstraintUnique:
			return ErrorKindUniqueConstraint
		default:
			return ErrorKindUnknown
		}
	}

	return ErrorKindUnknown
}
