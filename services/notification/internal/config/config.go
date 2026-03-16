package config

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	Port string

	DatabaseURL string

	// AccessTokenSecret is shared with the auth service to validate JWTs locally.
	AccessTokenSecret string

	// PubSubProjectID is the GCP project that owns the topic.
	PubSubProjectID string
	// PubSubSubscription is the Pub/Sub subscription this service pulls from.
	PubSubSubscription string
}

func Load() Config {
	viper.SetEnvPrefix("NOTIFICATION")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8084")
	viper.SetDefault("PUBSUB_SUBSCRIPTION", "notification-content-events")

	cfg := Config{
		Port:               viper.GetString("PORT"),
		DatabaseURL:        viper.GetString("DATABASE_URL"),
		AccessTokenSecret:  viper.GetString("ACCESS_TOKEN_SECRET"),
		PubSubProjectID:    viper.GetString("PUBSUB_PROJECT_ID"),
		PubSubSubscription: viper.GetString("PUBSUB_SUBSCRIPTION"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal().Msg("NOTIFICATION_DATABASE_URL is required")
	}
	if cfg.AccessTokenSecret == "" {
		log.Fatal().Msg("NOTIFICATION_ACCESS_TOKEN_SECRET is required")
	}
	if cfg.PubSubProjectID == "" {
		log.Warn().Msg("NOTIFICATION_PUBSUB_PROJECT_ID not set — Pub/Sub subscriber disabled (local dev mode)")
	}

	return cfg
}
