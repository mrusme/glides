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
	t = new(Debug)

	t.rt = rt
	t.def = def

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

	t.tmplCache = tmpl.NewCache(rt.GetEmbed("templates"), spec)

	return t, nil
}

func (t *Debug) Load() error {
	t.rt.Info("load target", "debug",
		"emulates", t.def.Debug.Emulates)
	t.rt.Debug("config", t.def)
	return nil
}

func (t *Debug) Run() error {
	t.rt.Info("run target", "debug",
		"emulates", t.def.Debug.Emulates,
		"path", t.def.Debug.Path)
	return nil
}

func (t *Debug) Shutdown() error {
	t.rt.Info("shutdown target", "debug")
	return nil
}
