package sqlite

import (
	"context"
	"database/sql"
	"errors"
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

	if err := fn(u.buildStores(tx)); err != nil {
		return errors.Join(err, tx.Rollback())
	}

	return tx.Commit()
}

func BuildUow[Stores any](db *DB, buildStores func(tx *sql.Tx) Stores) *uow[Stores] {
	return &uow[Stores]{
		db:          db,
		buildStores: buildStores,
	}
}
