// Package config 提供框架配置类型定义与加载。
//
// Config 是框架内置的配置聚合类型，适用于无需扩展字段的项目。
// 需要添加业务自定义字段时，自行定义 struct 组合本包的配置段，调用 LoadInto 加载。
package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// ── 配置聚合（便捷类型）───────────────────────────────────

// Config 框架层面的全部配置。业务可嵌入或自行组合。
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
	if c.App.Mode == "" {
		c.App.Mode = "debug"
	}
	if c.App.AdminPort <= 0 {
		c.App.AdminPort = 8600
	}
	if c.JWT.ExpireHours <= 0 {
		c.JWT.ExpireHours = 24
	}
	if c.JWT.RefreshHours <= 0 {
		c.JWT.RefreshHours = 168
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Path == "" {
		c.Log.Path = "logs/app.log"
	}
}

func (c *Config) Validate() error {
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	if c.App.Port <= 0 || c.App.Port > 65535 {
		return fmt.Errorf("app.port must be between 1 and 65535")
	}
	if c.App.AdminPort > 0 && c.App.AdminPort == c.App.Port {
		return fmt.Errorf("app.admin_port must differ from app.port")
	}
	if _, ok := c.MySQL["platform"]; !ok {
		return fmt.Errorf("mysql.platform is required")
	}
	for name, src := range c.MySQL {
		if src.Host == "" {
			return fmt.Errorf("mysql.%s.host is required", name)
		}
		if src.Port <= 0 {
			return fmt.Errorf("mysql.%s.port is required", name)
		}
		if src.Database == "" {
			return fmt.Errorf("mysql.%s.database is required", name)
		}
	}
	if len(c.Redis.Addrs) == 0 {
		return fmt.Errorf("redis.addrs must not be empty")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("jwt.secret is required")
	}
	return nil
}

// ── 配置加载 ────────────────────────────────────────────

// Load 加载标准 Config。需要扩展字段时使用 LoadInto。
func Load(path string) (*Config, error) {
	var cfg Config
	if err := LoadInto(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadInto 加载 YAML 配置到调用方自定义的结构体。
// cfg 必须是非 nil struct 指针，字段使用 mapstructure tag 匹配 YAML key。
// 若 cfg 实现了 SetDefaults() 或 Validate() error，自动调用。
func LoadInto(path string, cfg interface{}) error {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	if path != "" {
		v.AddConfigPath(path)
	}
	v.AddConfigPath("./configs")
	v.AddConfigPath("../configs")
	v.AddConfigPath("/app/configs")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	if d, ok := cfg.(interface{ SetDefaults() }); ok {
		d.SetDefaults()
	}
	if vd, ok := cfg.(interface{ Validate() error }); ok {
		if err := vd.Validate(); err != nil {
			return fmt.Errorf("validate config: %w", err)
		}
	}
	return nil
}
