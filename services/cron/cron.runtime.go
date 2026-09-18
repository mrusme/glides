package cron

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_cron"

func Register(rt *runtime.Runtime) (srv *Cron, err error) {
	rt.Debug("new", "cron")

	if srv, err = New(); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Cron {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Cron)
	}

	return nil
}
