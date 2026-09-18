package runtime

import (
	"errors"
	"log/slog"

	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/services/database"
)

func (rt *Runtime) AddService(id string, service any) (err error) {
	rt.services[id] = service
	return nil
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
		return nil, errors.New("no service found") // TODO: Replace with errs error
	}

	return service, nil
}

func (rt *Runtime) Config() (service *config.Config) {
	if srv, _ := rt.GetService("_config"); srv != nil {
		return srv.(*config.Config)
	}
	return nil
}

func (rt *Runtime) Logger() (service *slog.Logger) {
	if srv, _ := rt.GetService("_logger"); srv != nil {
		return srv.(*slog.Logger)
	}
	return nil
}

func (rt *Runtime) Database() (service *database.Database) {
	if srv, _ := rt.GetService("_database"); srv != nil {
		return srv.(*database.Database)
	}
	return nil
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
	if err = rt.Config().Startup(); err != nil {
		rt.Error("status", "error", "error", err)
		return err
	}

	if rt.Database() != nil {
		rt.Debug("startup", "database")
		if err = rt.Database().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	for _, id := range rt.order {
		rt.Debug("startup", id)
		if err = rt.services[id].(Service).Startup(); err != nil {
			rt.Error("status", "error", "service", id, "error", err)
			return err
		}
	}

	for idx, hook := range rt.onStartup {
		rt.Debug("startup", "hook", "index", idx)
		if err = hook(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	rt.Info("status", "ok")

	return nil
}

func (rt *Runtime) Shutdown() (err error) {
	rt.Debug("status", "exec")

	for idx := len(rt.onShutdown) - 1; idx >= 0; idx-- {
		rt.Debug("shutdown", "hook", "index", idx)
		if err = rt.onShutdown[idx](); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	for idx := len(rt.order) - 1; idx >= 0; idx-- {
		id := rt.order[idx]
		rt.Debug("shutdown", id)
		if err = rt.services[id].(Service).Shutdown(); err != nil {
			rt.Error("status", "error", "service", id, "error", err)
			return err
		}
	}

	if rt.Database() != nil {
		rt.Debug("shutdown", "database")
		if err = rt.Database().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	rt.Debug("shutdown", "config")
	if err = rt.Config().Shutdown(); err != nil {
		rt.Error("status", "error", "error", err)
		return err
	}

	rt.Info("status", "ok")

	return nil
}
