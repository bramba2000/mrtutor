package sqlite

import (
	"context"
	"database/sql"
)

type uow[Stores any] struct {
	db          *DB
	buildStores func(*sql.Tx) Stores
}

func (u *uow[Stores]) RunInTx(ctx context.Context, fn func(Stores) error) error {
	tx, err := u.db.W.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	err = fn(u.buildStores(tx))

	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func BuildUow[Stores any](db *DB, buildStores func(tx *sql.Tx) Stores) *uow[Stores] {
	return &uow[Stores]{
		db:          db,
		buildStores: buildStores,
	}
}
