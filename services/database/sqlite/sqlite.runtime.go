package sqlite

import (
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/database"
)

func Register(rt *runtime.Runtime, opts Options) (db *database.Database, err error) {
	rt.Debug("new", "database")

	if !opts.InMemory {
		opts.Path = rt.Config().DatabasePath()
	}

	if db, err = New(rt.Logger(), opts); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(database.ServiceID, db)

	return db, nil
}
