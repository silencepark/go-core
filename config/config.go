package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config 框架层面的全部配置。
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	MySQL    MySQLConfig    `mapstructure:"mysql"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	RabbitMQ RabbitMQConfig `mapstructure:"rabbitmq"`
	Log      LogConfig      `mapstructure:"log"`
	JWT      JWTConfig      `mapstructure:"jwt"`
}

func (c *Config) SetDefaults() {
	if c.App.Mode == "" { c.App.Mode = "debug" }
	if c.JWT.ExpireHours <= 0 { c.JWT.ExpireHours = 24 }
	if c.JWT.RefreshHours <= 0 { c.JWT.RefreshHours = 168 }
	if c.Log.Level == "" { c.Log.Level = "info" }
	if c.Log.Path == "" { c.Log.Path = "logs/app.log" }
}

func (c *Config) Validate() error {
	if c.App.Name == "" { return fmt.Errorf("app.name is required") }
	if c.App.Port <= 0 || c.App.Port > 65535 { return fmt.Errorf("app.port must be between 1 and 65535") }
	if _, ok := c.MySQL["platform"]; !ok { return fmt.Errorf("mysql.platform is required") }
	for name, src := range c.MySQL {
		if src.Host == "" { return fmt.Errorf("mysql.%s.host is required", name) }
		if src.Port <= 0 { return fmt.Errorf("mysql.%s.port is required", name) }
		if src.Database == "" { return fmt.Errorf("mysql.%s.database is required", name) }
	}
	if len(c.Redis.Addrs) == 0 { return fmt.Errorf("redis.addrs must not be empty") }
	if c.JWT.Secret == "" { return fmt.Errorf("jwt.secret is required") }
	return nil
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if path != "" { v.AddConfigPath(path) }
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("/app/configs")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	cfg.SetDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}
