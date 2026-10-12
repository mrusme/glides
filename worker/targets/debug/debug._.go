package debug

import (
	"fmt"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/worker/targets/handler"
	"xn--gckvb8fzb.com/glides/worker/targets/tmpl"
)

type Debug struct {
	handler.Handlers

	rt        *runtime.Runtime
	def       config.Target
	tmplCache *tmpl.Cache
}

func New(
	rt *runtime.Runtime,
	def config.Target,
) (t *Debug, err error) {
	var spec tmpl.Spec
	switch def.Debug.Emulates {
	case config.TargetTypeEmail:
		spec = tmpl.EmailSpec
	case config.TargetTypeXMPP:
		spec = tmpl.XMPPSpec
	default:
		return nil, fmt.Errorf("%w: %s",
			errs.ErrNoSuchTargetType, def.Debug.Emulates)
	}

	templates := rt.GetEmbed("templates")
	if templates == nil {
		return nil, fmt.Errorf("%w: templates", errs.ErrEmbedNotFound)
	}

	t = new(Debug)

	t.rt = rt
	t.def = def
	t.tmplCache = tmpl.NewCache(templates, spec)

	return t, nil
}

func (t *Debug) Load() error {
	t.rt.Info("load target", "debug", "id", t.def.ID,
		"emulates", t.def.Debug.Emulates)
	t.rt.Debug("config", t.def)
	return nil
}

func (t *Debug) Run() error {
	t.rt.Info("run target", "debug", "id", t.def.ID,
		"emulates", t.def.Debug.Emulates,
		"path", t.def.Debug.Path)
	return nil
}

func (t *Debug) Shutdown() error {
	t.rt.Info("shutdown target", "debug", "id", t.def.ID)
	return nil
}
