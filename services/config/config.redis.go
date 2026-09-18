package config

type Redis struct {
	Addrs      []string `koanf:"Addresses"`
	Database   int      `koanf:"Database"`
	MasterName string   `koanf:"MasterName"`
	Username   string   `koanf:"Username"`
	Password   string   `koanf:"Password"`
	Poolsize   int      `koanf:"Poolsize"`
}

func (cfg *Config) Redis() (r Redis, err error) {
	err = cfg.k.Load().Unmarshal("Redis", &r)
	return r, err
}

func (cfg *Config) RedisAddresses() []string {
	return cfg.k.Load().Strings("Redis.Addresses")
}

func (cfg *Config) RedisMasterName() string {
	return cfg.k.Load().String("Redis.MasterName")
}

func (cfg *Config) RedisUsername() string {
	return cfg.k.Load().String("Redis.Username")
}

func (cfg *Config) RedisPassword() string {
	return cfg.k.Load().String("Redis.Password")
}

func (cfg *Config) RedisDatabase() int {
	return cfg.k.Load().Int("Redis.Database")
}

func (cfg *Config) RedisReset() bool {
	return cfg.k.Load().Bool("Redis.Reset")
}

func (cfg *Config) RedisPoolsize() int {
	return cfg.k.Load().Int("Redis.Poolsize")
}
