package config

// RedisConfig Redis 集群/单节点配置.
type RedisConfig struct {
	Addrs    []string `mapstructure:"addrs"`
	Password string   `mapstructure:"password"`
	DB       int      `mapstructure:"db"`
	PoolSize int      `mapstructure:"pool_size"`
	MinIdle  int      `mapstructure:"min_idle"`
}
