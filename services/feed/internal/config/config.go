package config

import "github.com/spf13/viper"

// Config holds all runtime configuration for the feed service.
type Config struct {
	Port               string
	ContentDatabaseURL string
	AuthDatabaseURL    string
	AccessTokenSecret  string

	// RedisURL enables feed caching (e.g. "redis://localhost:6379/0").
	// Optional — leave empty to disable caching (useful in development).
	RedisURL string
}

// Load reads configuration from environment variables (prefixed FEED_).
func Load() Config {
	viper.SetEnvPrefix("FEED")
	viper.AutomaticEnv()
	viper.SetDefault("PORT", "8083")
	return Config{
		Port:               viper.GetString("PORT"),
		ContentDatabaseURL: viper.GetString("CONTENT_DATABASE_URL"),
		AuthDatabaseURL:    viper.GetString("AUTH_DATABASE_URL"),
		AccessTokenSecret:  viper.GetString("ACCESS_TOKEN_SECRET"),
		RedisURL:           viper.GetString("REDIS_URL"),
	}
}
