package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Security  SecurityConfig  `mapstructure:"security"`
	Site      SiteConfig      `mapstructure:"site"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type SecurityConfig struct {
	JWTSecret    string `mapstructure:"jwt_secret"`
	AESKeyBase64 string `mapstructure:"aes_key_base64"`
}

type SiteConfig struct {
	APIKeyPrefix          string   `mapstructure:"api_key_prefix"`
	CurrencyQuotaPerYuan  int64    `mapstructure:"currency_quota_per_yuan"`
	DefaultNewUserBalance int64    `mapstructure:"default_new_user_balance"`
	CORSOrigins           []string `mapstructure:"cors_origins"`
}

type BootstrapConfig struct {
	AdminEmail       string `mapstructure:"admin_email"`
	AdminPassword    string `mapstructure:"admin_password"`
	DefaultGroupName string `mapstructure:"default_group_name"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("TRACEHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.mode", "release")
	v.SetDefault("site.api_key_prefix", "sk-th-")
	v.SetDefault("site.currency_quota_per_yuan", 10000)
	v.SetDefault("site.default_new_user_balance", 0)
	v.SetDefault("site.cors_origins", []string{"*"})
	v.SetDefault("bootstrap.default_group_name", "基础分组")

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	if c.Database.DSN == "" {
		c.Database.DSN = v.GetString("database.dsn")
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = v.GetString("redis.addr")
	}
	if c.Security.JWTSecret == "" {
		c.Security.JWTSecret = v.GetString("security.jwt_secret")
	}
	return &c, nil
}
