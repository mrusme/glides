package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"xn--gckvb8fzb.com/glides/errs"
	"xn--gckvb8fzb.com/glides/services/config"
)

func (rt *Runtime) AddService(id string, service any) {
	rt.services[id] = service
}

func (rt *Runtime) Register(id string, service Service) {
	if _, exists := rt.services[id]; !exists {
		rt.order = append(rt.order, id)
	}
	rt.services[id] = service
}

func (rt *Runtime) GetService(id string) (service any, err error) {
	var exists bool = false

	if service, exists = rt.services[id]; exists == false {
		return nil, fmt.Errorf("%w: %s", errs.ErrServiceNotFound, id)
	}

	return service, nil
}

func (rt *Runtime) Config() (service *config.Config) {
	return rt.cfg
}

func (rt *Runtime) Logger() (service *slog.Logger) {
	return rt.logger
}

func (rt *Runtime) OnStartup(hooks ...Hook) {
	rt.onStartup = append(rt.onStartup, hooks...)
}

func (rt *Runtime) OnShutdown(hooks ...Hook) {
	rt.onShutdown = append(rt.onShutdown, hooks...)
}

func (rt *Runtime) Startup() (err error) {
	rt.Debug("status", "exec")

	rt.Debug("startup", "config")
	if err = rt.cfg.Startup(); err != nil {
		rt.Error("status", "error", "error", err)
		return err
	}
	rt.configStarted = true

	for _, id := range rt.order {
		rt.Debug("startup", id)
		if err = rt.services[id].(Service).Startup(); err != nil {
			rt.Error("status", "error", "service", id, "error", err)
			return errors.Join(err, rt.shutdown())
		}
		rt.started = append(rt.started, id)
	}
	rt.hooksReached = true

	for idx, hook := range rt.onStartup {
		rt.Debug("startup", "hook", "index", idx)
		if err = hook(); err != nil {
			rt.Error("status", "error", "hook", idx, "error", err)
			return errors.Join(err, rt.shutdown())
		}
	}

	rt.Info("status", "ok")

	return nil
}

func (rt *Runtime) Shutdown() (err error) {
	rt.Debug("status", "exec")

	if err = rt.shutdown(); err != nil {
		rt.Error("status", "error", "error", err)
		return err
	}

	rt.Info("status", "ok")

	return nil
}

func (rt *Runtime) shutdown() error {
	var failures []error

	if rt.hooksReached {
		for idx, hook := range slices.Backward(rt.onShutdown) {
			rt.Debug("shutdown", "hook", "index", idx)
			if herr := hook(); herr != nil {
				rt.Error("status", "error", "hook", idx, "error", herr)
				failures = append(failures, herr)
			}
		}
		rt.hooksReached = false
	}

	for _, id := range slices.Backward(rt.started) {
		rt.Debug("shutdown", id)
		if serr := rt.services[id].(Service).Shutdown(); serr != nil {
			rt.Error("status", "error", "service", id, "error", serr)
			failures = append(failures, serr)
		}
	}
	rt.started = nil

	if rt.configStarted {
		rt.Debug("shutdown", "config")
		if cerr := rt.cfg.Shutdown(); cerr != nil {
			rt.Error("status", "error", "error", cerr)
			failures = append(failures, cerr)
		}
		rt.configStarted = false
	}

	return errors.Join(failures...)
}
