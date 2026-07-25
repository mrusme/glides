package targets

import (
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
	Execute(
		j asyncjob.AsyncJob,
	) error
	Shutdown() error
}

type (
	ITargets map[string]ITarget
)

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
		if ts.targets[tcfg.ID], err = NewTarget(rt, tcfg); err != nil {
			return nil, err
		}

		if register == nil {
			continue
		}

		if err = register(tcfg.ID, ts.targets[tcfg.ID]); err != nil {
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
	id string,
	j asyncjob.AsyncJob,
) error {
	if _, ok := ts.targets[id]; !ok {
		return errs.ErrTargetIDNotFound
	}
	return ts.targets[id].Execute(j)
}

func (ts *Targets) ExecuteAll(
	j asyncjob.AsyncJob,
) (bool, map[string]error) {
	var errs map[string]error = make(map[string]error)
	var ok bool = true

	for _, tcfg := range ts.defs {
		if err := ts.targets[tcfg.ID].Execute(j); err != nil {
			errs[tcfg.ID] = err
			ok = false
		}
	}

	return ok, errs
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
