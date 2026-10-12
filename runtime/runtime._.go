package runtime

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"

	"xn--gckvb8fzb.com/glides/services/config"
)

const (
	ModeDevelopment string = config.ModeDevelopment
	ModeProduction  string = config.ModeProduction
)

const DefaultLoggingLevel slog.Level = slog.LevelInfo

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

	cfg    *config.Config
	logger *slog.Logger

	services map[string]any
	order    []string

	loggerLevel slog.Level
	ALogger     AsyncLogger

	onStartup  []Hook
	onShutdown []Hook

	configStarted bool
	started       []string
	hooksReached  bool
}

type Opts struct {
	Cfgstr  string
	Version string
	Commit  string
	Date    string
}

func New(opts Opts) (rt *Runtime, err error) {
	rt = new(Runtime)

	rt.build = newBuild(opts.Version, opts.Commit, opts.Date)
	rt.embeds = make(map[string]*embed.FS)
	rt.services = make(map[string]any)

	if rt.cfg, err = config.New(opts.Cfgstr); err != nil {
		return nil, err
	}

	if rt.loggerLevel, err = parseLevel(rt.cfg.LoggingLevel()); err != nil {
		return nil, err
	}

	rt.logger = slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: rt.loggerLevel,
		}),
	)
	rt.ALogger = NewAsyncLogger(rt.logger)

	rt.cfg.OnReloadError(func(rerr error) {
		rt.logger.Error("Config.Reload", "status", "error", "error", rerr)
	})

	rt.Info("status", "ok")
	return rt, nil
}

func parseLevel(text string) (level slog.Level, err error) {
	if text == "" {
		return DefaultLoggingLevel, nil
	}

	err = level.UnmarshalText([]byte(text))
	return level, err
}

func newBuild(version string, commit string, date string) Build {
	return Build{
		version: version,
		commit:  commit,
		date:    date,
		hash:    computeBuildHash(commit, date),
	}
}

func computeBuildHash(args ...string) string {
	h := sha256.New()
	h.Write([]byte(strings.Join(args, "")))
	return hex.EncodeToString(h.Sum(nil))
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
		rt.logger.Error(rt.getLogFnName(2), "error", err)
		rt.Exit(1)
	}
}

func (rt *Runtime) Exit(code int) {
	rt.Shutdown()
	os.Exit(code)
}

func (rt *Runtime) log(level slog.Level, args []any) {
	if !rt.logger.Enabled(context.Background(), level) {
		return
	}

	rt.logger.Log(context.Background(), level, rt.getLogFnName(3), args...)
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

func (rt *Runtime) IsDevelopmentMode() bool {
	return rt.cfg.GeneralMode() == ModeDevelopment
}
