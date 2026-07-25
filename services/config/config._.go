package config

import (
	"net/url"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"xn--gckvb8fzb.com/glides/errs"

	"github.com/knadh/koanf/v2"
)

type Config struct {
	cfgstr   string
	k        *koanf.Koanf
	provider koanf.Provider
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

	cfg.k = koanf.New(".")
	cfg.provider = file.Provider(path)

	if err = cfg.k.Load(cfg.provider, toml.Parser()); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (cfg *Config) Startup() (err error) {
	cfg.provider.(*file.File).Watch(func(event interface{}, werr error) {
		if werr != nil {
			// TODO: Handle error
			return
		}

		cfg.k = koanf.New(".")
		cfg.k.Load(cfg.provider, toml.Parser())
	})

	return nil
}

func (cfg *Config) Shutdown() error {
	cfg.provider.(*file.File).Unwatch()
	return nil
}

func (cfg *Config) Koanf() *koanf.Koanf {
	return cfg.k
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
	return cfg.k.Strings(v)
}

func (cfg *Config) Unmarshal(path string, o any) (x any, err error) {
	err = cfg.k.Unmarshal(path, o)
	return o, err
}
