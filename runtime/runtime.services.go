package runtime

import (
	"errors"
	"log/slog"

	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/services/cron"
	"xn--gckvb8fzb.com/glides/services/database"
	"xn--gckvb8fzb.com/glides/services/dispatch"
	"xn--gckvb8fzb.com/glides/services/intnat"
	"xn--gckvb8fzb.com/glides/services/markdown"
	"xn--gckvb8fzb.com/glides/services/storage"
)

func (rt *Runtime) AddService(id string, service any) (err error) {
	rt.services[id] = service
	return nil
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

func (rt *Runtime) Storage() (service *storage.Storage) {
	if srv, _ := rt.GetService("_storage"); srv != nil {
		return srv.(*storage.Storage)
	}
	return nil
}

func (rt *Runtime) Intnat() (service *intnat.Intnat) {
	if srv, _ := rt.GetService("_intnat"); srv != nil {
		return srv.(*intnat.Intnat)
	}
	return nil
}

func (rt *Runtime) Markdown() (service *markdown.Markdown) {
	if srv, _ := rt.GetService("_markdown"); srv != nil {
		return srv.(*markdown.Markdown)
	}
	return nil
}

func (rt *Runtime) Dispatch() (service *dispatch.Dispatch) {
	if srv, _ := rt.GetService("_dispatch"); srv != nil {
		return srv.(*dispatch.Dispatch)
	}
	return nil
}

func (rt *Runtime) Cron() (service *cron.Cron) {
	if srv, _ := rt.GetService("_cron"); srv != nil {
		return srv.(*cron.Cron)
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

	if rt.Storage() != nil {
		rt.Debug("startup", "storage")
		if err = rt.Storage().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Intnat() != nil {
		rt.Debug("startup", "intnat")
		if err = rt.Intnat().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Markdown() != nil {
		rt.Debug("startup", "markdown")
		if err = rt.Markdown().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Dispatch() != nil {
		rt.Debug("startup", "dispatch")
		if err = rt.Dispatch().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Cron() != nil {
		rt.Debug("startup", "cron")
		if err = rt.Cron().Startup(); err != nil {
			rt.Error("status", "error", "error", err)
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

	if rt.Cron() != nil {
		rt.Debug("shutdown", "cron")
		if err = rt.Cron().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Dispatch() != nil {
		rt.Debug("shutdown", "dispatch")
		if err = rt.Dispatch().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Markdown() != nil {
		rt.Debug("shutdown", "markdown")
		if err = rt.Markdown().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Intnat() != nil {
		rt.Debug("shutdown", "intnat")
		if err = rt.Intnat().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
			return err
		}
	}

	if rt.Storage() != nil {
		rt.Debug("shutdown", "storage")
		if err = rt.Storage().Shutdown(); err != nil {
			rt.Error("status", "error", "error", err)
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
