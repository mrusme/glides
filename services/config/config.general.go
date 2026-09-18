package config

import "strings"

func (cfg *Config) GeneralMode() string {
	return strings.ToLower(cfg.k.Load().String("General.Mode"))
}
