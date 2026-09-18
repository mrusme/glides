package storage

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_storage"

func Register(rt *runtime.Runtime) (srv *Storage, err error) {
	rt.Debug("new", "storage")

	cfg, err := rt.Config().Storages()
	if err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	if srv, err = New(cfg); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Storage {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Storage)
	}

	return nil
}
