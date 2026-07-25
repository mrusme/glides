package runtime

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	runt "runtime"
	"strings"

	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/services/cron"
	"xn--gckvb8fzb.com/glides/services/database"
	"xn--gckvb8fzb.com/glides/services/dispatch"
	"xn--gckvb8fzb.com/glides/services/intnat"
	"xn--gckvb8fzb.com/glides/services/markdown"
	"xn--gckvb8fzb.com/glides/services/storage"
)

type Build struct {
	version string
	commit  string
	date    string
	hash    string
}

type Services struct {
	Database bool
	Storage  bool
	Intnat   bool
	Markdown bool
	Dispatch bool
	Cron     bool
}

type Hook func() error

type Runtime struct {
	build  Build
	embeds map[string]*embed.FS

	services map[string]any

	loggerLevel slog.Level
	logger      *slog.Logger
	ALogger     AsyncLogger

	onStartup  []Hook
	onShutdown []Hook
}

const (
	ModeDevelopment string = "development"
	ModeProduction  string = "production"
)

type Opts struct {
	Cfgstr   string
	Version  string
	Commit   string
	Date     string
	Services Services
}

func New(opts Opts) (rt *Runtime, err error) {
	var srv any
	rt = new(Runtime)

	rt.build.version = opts.Version
	rt.build.commit = opts.Commit
	rt.build.date = opts.Date
	rt.build.hash = rt.computeBuildHash(rt.build.commit, rt.build.date)

	rt.embeds = make(map[string]*embed.FS)

	rt.services = make(map[string]any)

	if srv, err = config.New(opts.Cfgstr); err != nil {
		return nil, err
	}
	rt.AddService("_config", srv)

	rt.loggerLevel = slog.Level(0)
	if err = rt.loggerLevel.UnmarshalText(rt.Config().LoggingLevel()); err != nil {
		return nil, err
	}

	srv = slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: rt.loggerLevel,
		}),
	)
	rt.AddService("_logger", srv)

	rt.ALogger = NewAsyncLogger(rt.Logger())

	rt.Debug("status", "exec")

	if opts.Services.Database {
		rt.Debug("new", "database")
		if srv, err = database.New(
			rt.Logger(),
			rt.Config().DatabaseConnection(),
		); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_database", srv)
	}

	if opts.Services.Storage {
		rt.Debug("new", "storagescfg")
		storagesCfg, err := rt.Config().Storages()
		if err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.Debug("new", "storage")
		if srv, err = storage.New(storagesCfg); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_storage", srv)
	}

	if opts.Services.Intnat {
		rt.Debug("new", "intnat")
		if srv, err = intnat.New(); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_intnat", srv)
	}

	if opts.Services.Markdown {
		rt.Debug("new", "markdown")
		if srv, err = markdown.New(); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_markdown", srv)
	}

	if opts.Services.Dispatch {
		rt.Debug("new", "rediscfg")
		redisCfg, err := rt.Config().Redis()
		if err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.Debug("new", "targets")
		targetsCfg, err := rt.Config().Targets()
		if err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.Debug("new", "dispatch")
		if srv, err = dispatch.New(redisCfg, targetsCfg); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_dispatch", srv)
	}

	if opts.Services.Cron {
		rt.Debug("new", "cron")
		if srv, err = cron.New(); err != nil {
			rt.Error("status", "error", "error", err)
			return nil, err
		}
		rt.AddService("_cron", srv)
	}

	rt.Info("status", "ok")
	return rt, err
}

func (rt *Runtime) SetBuild(version string, commit string, date string) {
	rt.build.version = version
	rt.build.commit = commit
	rt.build.date = date
	rt.build.hash = rt.computeBuildHash(commit, date)
}

func (rt *Runtime) GetBuild() (
	version string,
	commit string,
	date string,
	hash string,
) {
	return rt.build.version,
		rt.build.commit,
		rt.build.date,
		rt.build.hash
}

func (rt *Runtime) GetLogLevel() (lvl slog.Level) {
	return rt.loggerLevel
}

func (rt *Runtime) NilOrDie(err error) {
	if err != nil {
		fn := rt.getLogFnName()
		rt.Logger().Error(fn, "error", err)
		rt.Exit(1)
	}
}

func (rt *Runtime) Exit(code int) {
	rt.Shutdown()
	os.Exit(code)
}

func (rt *Runtime) getLogFnName() string {
	pc, _, _, ok := runt.Caller(2)
	if !ok {
		return "Unknown"
	}
	fn := runt.FuncForPC(pc)
	if fn == nil {
		return "Unknown"
	}
	fullName := trimClosures(trimTypeArgs(fn.Name()))
	fullSplit := strings.Split(fullName, ".")
	fSL := len(fullSplit)

	if fSL == 1 {
		return fullSplit[0]
	} else if fSL > 1 {
		pkg := fullSplit[fSL-2]
		if strings.Index(pkg, "/") > -1 {
			pkgs := strings.Split(pkg, "/")
			pkg = pkgs[len(pkgs)-1]
		}
		if strings.Index(pkg, "(") > -1 {
			pkg = strings.ReplaceAll(pkg, "(", "")
			pkg = strings.ReplaceAll(pkg, ")", "")
			pkg = strings.ReplaceAll(pkg, "*", "")
		}
		mtd := fullSplit[fSL-1]
		return fmt.Sprintf("%s.%s", pkg, mtd)
	}

	return "Unknown"
}

func trimTypeArgs(name string) string {
	open := strings.Index(name, "[")
	if open == -1 {
		return name
	}

	close := strings.LastIndex(name, "]")
	if close < open {
		return name[:open]
	}

	return name[:open] + name[close+1:]
}

func trimClosures(name string) string {
	for {
		idx := strings.LastIndex(name, ".")
		if idx == -1 {
			return name
		}

		last := name[idx+1:]
		if !strings.HasPrefix(last, "func") {
			return name
		}
		if strings.Trim(last[len("func"):], "0123456789") != "" {
			return name
		}

		name = name[:idx]
	}
}

func (rt *Runtime) Debug(args ...any) {
	fn := rt.getLogFnName()
	rt.Logger().Debug(fn, args...)
}

func (rt *Runtime) Info(args ...any) {
	fn := rt.getLogFnName()
	rt.Logger().Info(fn, args...)
}

func (rt *Runtime) Warn(args ...any) {
	fn := rt.getLogFnName()
	rt.Logger().Warn(fn, args...)
}

func (rt *Runtime) Error(args ...any) {
	fn := rt.getLogFnName()
	rt.Logger().Error(fn, args...)
}

func (rt *Runtime) computeBuildHash(args ...string) string {
	h := sha256.New()
	h.Write([]byte(strings.Join(args, "")))
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func (rt *Runtime) IsDevelopmentMode() bool {
	if rt.Config().GeneralMode() == ModeDevelopment {
		return true
	}

	return false
}
