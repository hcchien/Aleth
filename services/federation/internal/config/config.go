package config

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Config holds all runtime configuration for the federation service.
type Config struct {
	Port string

	// DatabaseURL is the Postgres DSN for the federation schema.
	DatabaseURL string

	// Domain is the bare hostname (no scheme, no trailing slash) used to
	// construct ActivityPub URLs, e.g. "aleth.social".
	Domain string

	// AuthServiceURL is the base URL of the auth service, e.g. "http://localhost:8081".
	AuthServiceURL string

	// ContentServiceURL is the base URL of the content service.
	ContentServiceURL string

	// PlatformKeySecret is a 32-byte key (decoded from hex) used to
	// AES-256-GCM encrypt actor private keys at rest.
	PlatformKeySecret []byte
}

// Load reads configuration from environment variables prefixed with FEDERATION_.
func Load() Config {
	viper.SetEnvPrefix("FEDERATION")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8084")
	viper.SetDefault("AUTH_URL", "http://localhost:8081")
	viper.SetDefault("CONTENT_URL", "http://localhost:8082")

	domain := viper.GetString("DOMAIN")
	if domain == "" {
		log.Fatal().Msg("FEDERATION_DOMAIN is required (e.g. aleth.social)")
	}
	dbURL := viper.GetString("DATABASE_URL")
	if dbURL == "" {
		log.Fatal().Msg("FEDERATION_DATABASE_URL is required")
	}
	keyHex := viper.GetString("PLATFORM_KEY_SECRET")
	if keyHex == "" {
		log.Fatal().Msg("FEDERATION_PLATFORM_KEY_SECRET is required (32-byte hex string)")
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil || len(keyBytes) != 32 {
		log.Fatal().Msg("FEDERATION_PLATFORM_KEY_SECRET must be a 64-character hex string (32 bytes)")
	}

	return Config{
		Port:              viper.GetString("PORT"),
		DatabaseURL:       dbURL,
		Domain:            domain,
		AuthServiceURL:    viper.GetString("AUTH_URL"),
		ContentServiceURL: viper.GetString("CONTENT_URL"),
		PlatformKeySecret: keyBytes,
	}
}
