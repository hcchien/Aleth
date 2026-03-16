package config

import (
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Config struct {
	Port string

	DatabaseURL string

	AccessTokenSecret  string
	RefreshTokenSecret string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration

	GoogleClientID string
	FacebookAppID  string
	PasskeyRPID    string
}

func Load() Config {
	viper.SetEnvPrefix("AUTH")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8081")
	viper.SetDefault("ACCESS_TOKEN_TTL", "15m")
	viper.SetDefault("REFRESH_TOKEN_TTL", "168h") // 7 days

	cfg := Config{
		Port:           viper.GetString("PORT"),
		DatabaseURL:    viper.GetString("DATABASE_URL"),
		GoogleClientID: viper.GetString("GOOGLE_CLIENT_ID"),
		FacebookAppID:  viper.GetString("FACEBOOK_APP_ID"),
		PasskeyRPID:    viper.GetString("PASSKEY_RP_ID"),
	}

	cfg.AccessTokenSecret = viper.GetString("ACCESS_TOKEN_SECRET")
	cfg.RefreshTokenSecret = viper.GetString("REFRESH_TOKEN_SECRET")

	if cfg.AccessTokenSecret == "" {
		log.Fatal().Msg("AUTH_ACCESS_TOKEN_SECRET is required")
	}
	if cfg.RefreshTokenSecret == "" {
		log.Fatal().Msg("AUTH_REFRESH_TOKEN_SECRET is required")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal().Msg("AUTH_DATABASE_URL is required")
	}

	cfg.AccessTokenTTL = viper.GetDuration("ACCESS_TOKEN_TTL")
	cfg.RefreshTokenTTL = viper.GetDuration("REFRESH_TOKEN_TTL")
	if cfg.PasskeyRPID == "" {
		cfg.PasskeyRPID = "localhost"
	}

	return cfg
}
