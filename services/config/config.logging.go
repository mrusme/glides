package config

func (cfg *Config) LoggingLevel() []byte {
	return cfg.k.Load().Bytes("Logging.Level")
}
