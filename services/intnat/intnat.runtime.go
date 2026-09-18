package intnat

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_intnat"

func Register(rt *runtime.Runtime) (srv *Intnat, err error) {
	rt.Debug("new", "intnat")

	if srv, err = New(); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Intnat {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Intnat)
	}

	return nil
}
