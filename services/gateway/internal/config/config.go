package config

import "github.com/spf13/viper"

// Config holds all runtime configuration for the gateway.
type Config struct {
	Port                string
	AuthServiceURL      string
	ContentServiceURL   string
	FeedServiceURL      string
	NotificationURL     string
	AccessTokenSecret   string
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
	return Config{
		Port:              viper.GetString("PORT"),
		AuthServiceURL:    viper.GetString("AUTH_SERVICE_URL"),
		ContentServiceURL: viper.GetString("CONTENT_SERVICE_URL"),
		FeedServiceURL:    viper.GetString("FEED_SERVICE_URL"),
		NotificationURL:   viper.GetString("NOTIFICATION_URL"),
		AccessTokenSecret: viper.GetString("ACCESS_TOKEN_SECRET"),
	}
}
