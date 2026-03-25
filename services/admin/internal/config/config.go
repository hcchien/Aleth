package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port int

	// Auth DB — read+write (admin_users, audit_log, user management).
	AuthDatabaseURL string
	// Content DB — read + limited writes (reports, posts).
	ContentDatabaseURL string

	// Separate JWT secret — admin tokens cannot be used on the main platform.
	JWTSecret string

	// Allowed origin for the admin frontend (CORS).
	AllowedOrigin string
}

func Load() Config {
	v := viper.New()
	v.SetEnvPrefix("ADMIN")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("port", 8087)
	v.SetDefault("allowed_origin", "http://localhost:3001")

	return Config{
		Port:               v.GetInt("port"),
		AuthDatabaseURL:    v.GetString("auth_database_url"),
		ContentDatabaseURL: v.GetString("content_database_url"),
		JWTSecret:          v.GetString("jwt_secret"),
		AllowedOrigin:      v.GetString("allowed_origin"),
	}
}
