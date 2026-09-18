package dispatch

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_dispatch"

func Register(rt *runtime.Runtime) (srv *Dispatch, err error) {
	rt.Debug("new", "dispatch")

	redisCfg, err := rt.Config().Redis()
	if err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	targetsCfg, err := rt.Config().Targets()
	if err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	if srv, err = New(redisCfg, targetsCfg); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Dispatch {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Dispatch)
	}

	return nil
}
