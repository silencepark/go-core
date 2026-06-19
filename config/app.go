package config

// AppConfig 服务基础配置.
type AppConfig struct {
	Name        string   `mapstructure:"name"`
	Port        int      `mapstructure:"port"`
	AdminPort   int      `mapstructure:"admin_port"`
	Mode        string   `mapstructure:"mode"`
	UploadDir   string   `mapstructure:"upload_dir"`
	CORSOrigins []string `mapstructure:"cors_origins"`
}
