package config

// JWTConfig JWT 签发配置.
type JWTConfig struct {
	Secret       string `mapstructure:"secret"`
	ExpireHours  int    `mapstructure:"expire_hours"`
	RefreshHours int    `mapstructure:"refresh_hours"`
}
