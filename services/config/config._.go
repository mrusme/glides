package config

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"xn--gckvb8fzb.com/glides/errs"
)

const (
	ModeDevelopment string = "development"
	ModeProduction  string = "production"
)

type Config struct {
	cfgstr   string
	k        atomic.Pointer[koanf.Koanf]
	provider *file.File
	onError  func(error)
}

func New(
	cfgstr string,
) (cfg *Config, err error) {
	cfg = new(Config)
	var path string

	cfg.cfgstr = cfgstr
	if path, err = cfg.parseCfgstr(); err != nil {
		return nil, err
	}

	cfg.provider = file.Provider(path)

	if err = cfg.load(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) load() (err error) {
	k := koanf.New(".")
	if err = k.Load(cfg.provider, toml.Parser()); err != nil {
		return err
	}

	if err = validate(k); err != nil {
		return err
	}

	cfg.k.Store(k)

	return nil
}

func validate(k *koanf.Koanf) error {
	switch mode := strings.ToLower(k.String("General.Mode")); mode {
	case "", ModeDevelopment, ModeProduction:
		return nil
	default:
		return fmt.Errorf("%w: %q", errs.ErrConfigModeInvalid, mode)
	}
}

func (cfg *Config) OnReloadError(fn func(error)) {
	cfg.onError = fn
}

func (cfg *Config) reloadFailed(err error) {
	if cfg.onError != nil {
		cfg.onError(err)
	}
}

func (cfg *Config) Startup() (err error) {
	return cfg.provider.Watch(func(event any, werr error) {
		if werr != nil {
			cfg.reloadFailed(werr)
			return
		}

		if lerr := cfg.load(); lerr != nil {
			cfg.reloadFailed(lerr)
		}
	})
}

func (cfg *Config) Shutdown() error {
	return cfg.provider.Unwatch()
}

func (cfg *Config) Koanf() *koanf.Koanf {
	return cfg.k.Load()
}

func (cfg *Config) parseCfgstr() (path string, err error) {
	u, err := url.Parse(cfg.cfgstr)
	if err != nil {
		return "", err
	}

	switch u.Scheme {
	case "":
		return cfg.cfgstr, nil
	case "file":
		if u.Host != "" {
			return u.Host + u.Path, nil
		}
		return u.Path, nil
	default:
		return "", errs.ErrConfigTypeUnsupported
	}
}

func (cfg *Config) Strings(v string) []string {
	return cfg.k.Load().Strings(v)
}

func (cfg *Config) Unmarshal(path string, o any) error {
	return cfg.k.Load().Unmarshal(path, o)
}
