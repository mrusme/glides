package runtime

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	runt "runtime"
	"strings"

	"xn--gckvb8fzb.com/glides/services/config"
	"xn--gckvb8fzb.com/glides/services/database"
)

type Build struct {
	version string
	commit  string
	date    string
	hash    string
}

type Service interface {
	Startup() error
	Shutdown() error
}

type Hook func() error

type Runtime struct {
	build  Build
	embeds map[string]*embed.FS

	services map[string]any
	order    []string

	loggerLevel slog.Level
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
	Database bool
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

	rt.Config().OnReloadError(func(rerr error) {
		rt.Logger().Error("Config.Reload", "status", "error", "error", rerr)
	})

	rt.Debug("status", "exec")

	if opts.Database {
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
		fn := rt.getLogFnName(2)
		rt.Logger().Error(fn, "error", err)
		rt.Exit(1)
	}
}

func (rt *Runtime) Exit(code int) {
	rt.Shutdown()
	os.Exit(code)
}

func (rt *Runtime) getLogFnName(skip int) string {
	pc, _, _, ok := runt.Caller(skip)
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
		if strings.Contains(pkg, "/") {
			pkgs := strings.Split(pkg, "/")
			pkg = pkgs[len(pkgs)-1]
		}
		if strings.Contains(pkg, "(") {
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

func (rt *Runtime) log(level slog.Level, args []any) {
	logger := rt.Logger()
	if !logger.Enabled(context.Background(), level) {
		return
	}

	logger.Log(context.Background(), level, rt.getLogFnName(3), args...)
}

func (rt *Runtime) Debug(args ...any) {
	rt.log(slog.LevelDebug, args)
}

func (rt *Runtime) Info(args ...any) {
	rt.log(slog.LevelInfo, args)
}

func (rt *Runtime) Warn(args ...any) {
	rt.log(slog.LevelWarn, args)
}

func (rt *Runtime) Error(args ...any) {
	rt.log(slog.LevelError, args)
}

func (rt *Runtime) computeBuildHash(args ...string) string {
	h := sha256.New()
	h.Write([]byte(strings.Join(args, "")))
	hashBytes := h.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

func (rt *Runtime) IsDevelopmentMode() bool {
	return rt.Config().GeneralMode() == ModeDevelopment
}
