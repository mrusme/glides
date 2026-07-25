package dispatch

import (
	"github.com/hibiken/asynq"
	"xn--gckvb8fzb.com/glides/services/config"
)

func RedisConnOpt(cfg config.Redis) asynq.RedisConnOpt {
	if len(cfg.Addrs) > 1 {
		return asynq.RedisClusterClientOpt{
			Addrs:    cfg.Addrs,
			Username: cfg.Username,
			Password: cfg.Password,
		}
	}

	if cfg.MasterName != "" {
		return asynq.RedisFailoverClientOpt{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.Addrs,
			DB:            cfg.Database,
			Username:      cfg.Username,
			Password:      cfg.Password,
			PoolSize:      cfg.Poolsize,
		}
	}

	var addr string
	if len(cfg.Addrs) == 1 {
		addr = cfg.Addrs[0]
	}

	return asynq.RedisClientOpt{
		Addr:     addr,
		DB:       cfg.Database,
		Username: cfg.Username,
		Password: cfg.Password,
		PoolSize: cfg.Poolsize,
	}
}
