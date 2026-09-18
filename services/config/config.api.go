package config

const APIDefaultPort = 3001

func (cfg *Config) APIEnable() bool {
	return cfg.k.Load().Bool("API.Enable")
}

func (cfg *Config) APIBindIP() string {
	return cfg.k.Load().String("API.BindIP")
}

func (cfg *Config) APIPort() int {
	port := cfg.k.Load().Int("API.Port")
	if port == 0 {
		return APIDefaultPort
	}
	return port
}

func (cfg *Config) APIBodyLimit() int {
	return cfg.k.Load().Int("API.BodyLimit")
}

func (cfg *Config) APIConcurrency() int {
	return cfg.k.Load().Int("API.Concurrency")
}

func (cfg *Config) APIProxyHeader() string {
	return cfg.k.Load().String("API.ProxyHeader")
}

func (cfg *Config) APIEnableIPValidation() bool {
	return cfg.k.Load().Bool("API.EnableIPValidation")
}

func (cfg *Config) APITrustProxy() bool {
	return cfg.k.Load().Bool("API.TrustProxy")
}

func (cfg *Config) APITrustLoopback() bool {
	return cfg.k.Load().Bool("API.TrustLoopback")
}

func (cfg *Config) APITrustProxies() []string {
	return cfg.k.Load().Strings("API.TrustProxies")
}

func (cfg *Config) APIReduceMemoryUsage() bool {
	return cfg.k.Load().Bool("API.ReduceMemoryUsage")
}

func (cfg *Config) APIServerHeader() string {
	return cfg.k.Load().String("API.ServerHeader")
}
