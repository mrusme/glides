package database

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_database"

func Register(rt *runtime.Runtime) (srv *Database, err error) {
	rt.Debug("new", "database")

	if srv, err = New(rt.Logger(), rt.Config().DatabaseConnection()); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Database {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Database)
	}

	return nil
}
