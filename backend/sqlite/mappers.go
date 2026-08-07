package sqlite

import (
	"database/sql"
	"time"
)

func NullTimeToPointer(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}
