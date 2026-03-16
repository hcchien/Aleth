package config

import "github.com/spf13/viper"

// Config holds all configuration read from environment variables.
type Config struct {
	Port              int
	DatabaseURL       string
	PubSubProjectID   string
	PubSubSubscription string
}

// Load reads configuration from environment variables with COUNTER_ prefix.
func Load() Config {
	v := viper.New()
	v.SetEnvPrefix("COUNTER")
	v.AutomaticEnv()

	v.SetDefault("PORT", 8085)

	return Config{
		Port:               v.GetInt("PORT"),
		DatabaseURL:        v.GetString("DATABASE_URL"),
		PubSubProjectID:    v.GetString("PUBSUB_PROJECT_ID"),
		PubSubSubscription: v.GetString("PUBSUB_SUBSCRIPTION"),
	}
}
