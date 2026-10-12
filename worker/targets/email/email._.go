package email

import (
	"fmt"

	"xn--gckvb8fzb.com/glides/errs"
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
	templates := rt.GetEmbed("templates")
	if templates == nil {
		return nil, fmt.Errorf("%w: templates", errs.ErrEmbedNotFound)
	}

	t = new(Email)

	t.rt = rt
	t.def = def
	t.tmplCache = tmpl.NewCache(templates, tmpl.EmailSpec)

	return t, nil
}

func (t *Email) Load() error {
	t.rt.Info("load target", "email", "id", t.def.ID)
	t.rt.Debug("config", t.def)
	return nil
}

func (t *Email) Run() error {
	t.rt.Info("run target", "email", "id", t.def.ID)
	return nil
}

func (t *Email) Shutdown() error {
	t.rt.Info("shutdown target", "email", "id", t.def.ID)
	return nil
}
