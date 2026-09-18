package markdown

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_markdown"

func Register(rt *runtime.Runtime) (srv *Markdown, err error) {
	rt.Debug("new", "markdown")

	if srv, err = New(); err != nil {
		rt.Error("status", "error", "error", err)
		return nil, err
	}

	rt.Register(ServiceID, srv)

	return srv, nil
}

func From(rt *runtime.Runtime) *Markdown {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Markdown)
	}

	return nil
}
