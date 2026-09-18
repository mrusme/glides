package config

func (cfg *Config) DatabaseConnection() string {
	return cfg.k.Load().String("Database.Connection")
}
