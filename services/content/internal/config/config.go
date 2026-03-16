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
	SigningSecret     string

	// FederationURL is the base URL of the federation service.
	// When set, the content service notifies federation after each top-level post.
	// Empty string disables federation fan-out.
	FederationURL string

	// PubSubEnabled controls whether events are published to GCP Pub/Sub.
	// When false (default), a DirectPublisher is used — subscriber handlers
	// registered in-process receive events synchronously, which is useful for
	// local development and testing without a live GCP project.
	PubSubEnabled   bool
	PubSubProjectID string
	PubSubTopic     string
}

func Load() Config {
	viper.SetEnvPrefix("CONTENT")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8082")
	viper.SetDefault("PUBSUB_ENABLED", false)
	viper.SetDefault("PUBSUB_TOPIC", "content-events")

	cfg := Config{
		Port:              viper.GetString("PORT"),
		DatabaseURL:       viper.GetString("DATABASE_URL"),
		AccessTokenSecret: viper.GetString("ACCESS_TOKEN_SECRET"),
		SigningSecret:     viper.GetString("SIGNING_SECRET"),
		FederationURL:     viper.GetString("FEDERATION_URL"),
		PubSubEnabled:     viper.GetBool("PUBSUB_ENABLED"),
		PubSubProjectID:   viper.GetString("PUBSUB_PROJECT_ID"),
		PubSubTopic:       viper.GetString("PUBSUB_TOPIC"),
	}

	if cfg.DatabaseURL == "" {
		log.Fatal().Msg("CONTENT_DATABASE_URL is required")
	}
	if cfg.AccessTokenSecret == "" {
		log.Fatal().Msg("CONTENT_ACCESS_TOKEN_SECRET is required")
	}
	if cfg.SigningSecret == "" {
		cfg.SigningSecret = cfg.AccessTokenSecret
	}
	if cfg.PubSubEnabled && cfg.PubSubProjectID == "" {
		log.Fatal().Msg("CONTENT_PUBSUB_PROJECT_ID is required when CONTENT_PUBSUB_ENABLED=true")
	}

	return cfg
}
