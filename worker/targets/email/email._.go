package email

import (
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
	"xn--gckvb8fzb.com/glides/worker/targets/tmpl"
)

type Email struct {
	handler.Handlers

	rt        *runtime.Runtime
	def       config.Target
	tmplCache *tmpl.Cache
}

func New(
	rt *runtime.Runtime,
	def config.Target,
) (t *Email, err error) {
	t = new(Email)

	t.rt = rt
	t.def = def
	t.tmplCache = tmpl.NewCache(rt.GetEmbed("templates"), tmpl.EmailSpec)

	return t, nil
}

func (t *Email) Load() error {
	t.rt.Info("load target", "email")
	t.rt.Debug("config", t.def)
	return nil
}

func (t *Email) Run() error {
	t.rt.Info("run target", "email")
	return nil
}

func (t *Email) Shutdown() error {
	t.rt.Info("shutdown target", "email")
	return nil
}
