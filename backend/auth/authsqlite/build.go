package authsqlite

import (
	"database/sql"

	"github.com/bramba2000/mrtutor/backend/auth"
	"github.com/bramba2000/mrtutor/backend/sqlite"
)

type authStorage struct {
	PrincipalStore auth.PrincipalStore
	SessionStore   auth.SessionStore
	UnitOfWork     auth.UnitOfWork
}

func Build(db *sqlite.DB) authStorage {
	return authStorage{
		PrincipalStore: &PrincipalStore{r: db.R, w: db.W},
		SessionStore:   &SessionStore{r: db.R, w: db.W},
		UnitOfWork: sqlite.BuildUow(db, func(tx *sql.Tx) (stores auth.Stores) {
			stores.Principal = &PrincipalStore{r: tx, w: tx}
			stores.Session = &SessionStore{r: tx, w: tx}
			return
		}),
	}
}
