package postgres

import (
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/database"
)

func Register(rt *runtime.Runtime, opts Options) (db *database.Database, err error) {
	rt.Debug("new", "database")

	if db, err = New(rt.Logger(), rt.Config().DatabaseConnection(), opts); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(database.ServiceID, db)

	return db, nil
}
