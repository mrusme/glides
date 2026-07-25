package http

import (
	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/runtime"
)

type Iface interface {
	Startup() error
	Run() error
	Shutdown() error
}

type HTTP struct {
	rt    *runtime.Runtime
	iface Iface
}

func New(
	rt *runtime.Runtime,
	iface Iface,
) (srv *HTTP, err error) {
	if iface == nil {
		return nil, errs.ErrIfaceInvalid
	}

	srv = new(HTTP)
	srv.rt = rt
	srv.iface = iface

	return srv, nil
}

func (srv *HTTP) Startup() (err error) {
	srv.rt.Debug("status", "exec")

	if err = srv.iface.Startup(); err != nil {
		srv.rt.Error("status", "error", "error", err)
		return err
	}

	srv.rt.Info("status", "ok")

	return nil
}

func (srv *HTTP) Run() (err error) {
	srv.rt.Debug("status", "exec")

	if err = srv.iface.Run(); err != nil {
		srv.rt.Error("status", "error", "error", err)
		return err
	}

	srv.rt.Info("status", "ok")

	return nil
}

func (srv *HTTP) Shutdown() (err error) {
	srv.rt.Debug("status", "exec")

	if err = srv.iface.Shutdown(); err != nil {
		srv.rt.Error("status", "error", "error", err)
		return err
	}

	srv.rt.Info("status", "ok")

	return nil
}
