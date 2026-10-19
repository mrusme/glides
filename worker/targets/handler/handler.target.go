package handler

import (
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
)

type Target struct {
	Handlers

	rt  *runtime.Runtime
	def config.Target
}

func New(
	rt *runtime.Runtime,
	def config.Target,
) (t *Target, err error) {
	t = new(Target)

	t.rt = rt
	t.def = def

	return t, nil
}

func (t *Target) Load() error {
	t.rt.Info("load target", "handler", "id", t.def.ID)
	return nil
}

func (t *Target) Run() error {
	t.rt.Info("run target", "handler", "id", t.def.ID)
	return nil
}

func (t *Target) Shutdown() error {
	t.rt.Info("shutdown target", "handler", "id", t.def.ID)
	return nil
}
