package config

import "log/slog"

type Redis struct {
	Addrs      []string `koanf:"Addresses"`
	Database   int      `koanf:"Database"`
	MasterName string   `koanf:"MasterName"`
	Username   string   `koanf:"Username"`
	Password   string   `koanf:"Password"`
	Poolsize   int      `koanf:"Poolsize"`
	Reset      bool     `koanf:"Reset"`
}

func (cfg *Config) Redis() (r Redis, err error) {
	err = cfg.k.Load().Unmarshal("Redis", &r)
	return r, err
}

func (r Redis) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Any("addresses", r.Addrs),
		slog.Int("database", r.Database),
		slog.String("master_name", r.MasterName),
		slog.String("username", r.Username),
		slog.Int("poolsize", r.Poolsize),
		slog.Bool("reset", r.Reset),
	)
}
