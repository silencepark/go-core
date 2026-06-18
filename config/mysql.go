package config

import "fmt"

// MySQLConfig 支持多数据源，key 为数据源名称，如 platform / user / order.
type MySQLConfig map[string]SourceConfig

// SourceConfig 单个 MySQL 数据源配置.
type SourceConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	Database    string `mapstructure:"database"`
	Charset     string `mapstructure:"charset"`
	MaxIdle     int    `mapstructure:"max_idle"`
	MaxOpen     int    `mapstructure:"max_open"`
	MaxLifetime int    `mapstructure:"max_lifetime"`
}

// DSN 返回业务库连接串.
func (s SourceConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		s.Username, s.Password, s.Host, s.Port, s.Database, s.Charset)
}
