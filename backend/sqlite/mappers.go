package sqlite

import (
	"database/sql"
	"time"
)

func nullTimeToPointer(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}
