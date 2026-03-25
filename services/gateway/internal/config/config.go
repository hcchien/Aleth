package config

import (
	"log"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for the gateway.
type Config struct {
	Port              string
	AuthServiceURL    string
	ContentServiceURL string
	FeedServiceURL    string
	NotificationURL   string
	FederationURL     string
	AccessTokenSecret string
	AllowedOrigin     string
}

// Load reads configuration from environment variables (prefixed GATEWAY_).
func Load() Config {
	viper.SetEnvPrefix("GATEWAY")
	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("AUTH_SERVICE_URL", "http://localhost:8081")
	viper.SetDefault("CONTENT_SERVICE_URL", "http://localhost:8082")
	viper.SetDefault("FEED_SERVICE_URL", "http://localhost:8083")
	viper.SetDefault("NOTIFICATION_URL", "http://localhost:8086")
	viper.SetDefault("FEDERATION_URL", "http://localhost:8087")
	viper.SetDefault("ALLOWED_ORIGIN", "")
	cfg := Config{
		Port:              viper.GetString("PORT"),
		AuthServiceURL:    viper.GetString("AUTH_SERVICE_URL"),
		ContentServiceURL: viper.GetString("CONTENT_SERVICE_URL"),
		FeedServiceURL:    viper.GetString("FEED_SERVICE_URL"),
		NotificationURL:   viper.GetString("NOTIFICATION_URL"),
		FederationURL:     viper.GetString("FEDERATION_URL"),
		AccessTokenSecret: viper.GetString("ACCESS_TOKEN_SECRET"),
		AllowedOrigin:     viper.GetString("ALLOWED_ORIGIN"),
	}
	if cfg.AllowedOrigin == "" || cfg.AllowedOrigin == "*" {
		log.Fatal("GATEWAY_ALLOWED_ORIGIN must be set to a specific origin (not '*')")
	}
	if cfg.AccessTokenSecret == "" {
		log.Fatal("GATEWAY_ACCESS_TOKEN_SECRET must be set")
	}
	return cfg
}
