package targets

import (
	"context"
	"fmt"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/models/asyncjob"
	"xn--gckvb8fzb.com/glides/runtime"
	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/worker/targets/debug"
	"xn--gckvb8fzb.com/glides/worker/targets/email"
	"xn--gckvb8fzb.com/glides/worker/targets/xmpp"
)

type ITarget interface {
	Load() error
	Run() error
	Execute(ctx context.Context, j asyncjob.AsyncJob) error
	Shutdown() error
}

type ITargets map[string]ITarget

type Register func(id string, t ITarget) error

type Targets struct {
	rt      *runtime.Runtime
	targets ITargets
	defs    config.Targets
}

func NewTarget(
	rt *runtime.Runtime,
	def config.Target,
) (t ITarget, err error) {
	switch def.Type {
	case config.TargetTypeEmail:
		t, err = email.New(rt, def)
	case config.TargetTypeXMPP:
		t, err = xmpp.New(rt, def)
	case config.TargetTypeDebug:
		t, err = debug.New(rt, def)
	default:
		return nil, fmt.Errorf("%w: %s", errs.ErrNoSuchTargetType, def.Type)
	}
	if err != nil {
		return nil, err
	}

	return t, nil
}

func New(
	rt *runtime.Runtime,
	register Register,
) (*Targets, error) {
	var err error

	ts := new(Targets)

	ts.rt = rt
	ts.targets = make(ITargets)
	ts.defs, err = ts.rt.Config().Targets()
	if err != nil {
		return nil, err
	}

	for _, tcfg := range ts.defs {
		if _, exists := ts.targets[tcfg.ID]; exists {
			return nil, fmt.Errorf("%w: %s", errs.ErrTargetIDDuplicate, tcfg.ID)
		}

		target, err := NewTarget(rt, tcfg)
		if err != nil {
			return nil, err
		}
		ts.targets[tcfg.ID] = target

		if register == nil {
			continue
		}

		if err = register(tcfg.ID, target); err != nil {
			return nil, err
		}
	}

	return ts, nil
}

func (ts *Targets) LoadAll() error {
	for _, tcfg := range ts.defs {
		if err := ts.targets[tcfg.ID].Load(); err != nil {
			return err
		}
	}

	return nil
}

func (ts *Targets) RunAll() error {
	var running []string

	for _, tcfg := range ts.defs {
		if err := ts.targets[tcfg.ID].Run(); err != nil {
			for _, tnamerunning := range running {
				ts.targets[tnamerunning].Shutdown()
			}
			return err
		}
		running = append(running, tcfg.ID)
	}

	return nil
}

func (ts *Targets) Execute(
	ctx context.Context,
	id string,
	j asyncjob.AsyncJob,
) error {
	target, ok := ts.targets[id]
	if !ok {
		return fmt.Errorf("%w: %s", errs.ErrTargetIDNotFound, id)
	}

	return target.Execute(ctx, j)
}

func (ts *Targets) ShutdownAll() (bool, map[string]error) {
	var errs map[string]error = make(map[string]error)
	var ok bool = true

	for _, tcfg := range ts.defs {
		if err := ts.targets[tcfg.ID].Shutdown(); err != nil {
			errs[tcfg.ID] = err
			ok = false
		}
	}

	return ok, errs
}
