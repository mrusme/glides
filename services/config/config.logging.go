package config

func (cfg *Config) LoggingLevel() string {
	return cfg.k.Load().String("Logging.Level")
}
