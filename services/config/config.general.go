package config

import "strings"

func (cfg *Config) GeneralMode() string {
	mode := strings.ToLower(cfg.k.Load().String("General.Mode"))
	if mode == "" {
		return ModeProduction
	}

	return mode
}
