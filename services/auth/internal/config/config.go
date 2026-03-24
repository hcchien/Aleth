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

	// Social OAuth reputation providers
	// Twitter OAuth 2.0 (requires PKCE)
	TwitterClientID     string
	TwitterClientSecret string
	// Facebook server-side OAuth (App ID is FacebookAppID; secret is needed for code exchange)
	FacebookClientSecret string
	// Instagram Basic Display API / Instagram Login
	InstagramClientID     string
	InstagramClientSecret string
	// LinkedIn OAuth 2.0 (OpenID Connect)
	LinkedInClientID     string
	LinkedInClientSecret string

	// OAuthCallbackBase is the public base URL of this auth service, used to
	// build redirect_uri values for each provider's OAuth callback.
	// Example: "https://auth.example.com" or "http://localhost:8081"
	OAuthCallbackBase string
	// FrontendURL is the base URL of the Next.js frontend.  After a successful
	// (or failed) OAuth callback the user is redirected here.
	// Example: "https://example.com" or "http://localhost:3000"
	FrontendURL string
}

func Load() Config {
	viper.SetEnvPrefix("AUTH")
	viper.AutomaticEnv()

	viper.SetDefault("PORT", "8081")
	viper.SetDefault("ACCESS_TOKEN_TTL", "15m")
	viper.SetDefault("REFRESH_TOKEN_TTL", "168h") // 7 days

	viper.SetDefault("OAUTH_CALLBACK_BASE", "http://localhost:8081")
	viper.SetDefault("FRONTEND_URL", "http://localhost:3000")

	cfg := Config{
		Port:           viper.GetString("PORT"),
		DatabaseURL:    viper.GetString("DATABASE_URL"),
		GoogleClientID: viper.GetString("GOOGLE_CLIENT_ID"),
		FacebookAppID:  viper.GetString("FACEBOOK_APP_ID"),
		PasskeyRPID:    viper.GetString("PASSKEY_RP_ID"),

		TwitterClientID:      viper.GetString("TWITTER_CLIENT_ID"),
		TwitterClientSecret:  viper.GetString("TWITTER_CLIENT_SECRET"),
		FacebookClientSecret: viper.GetString("FACEBOOK_CLIENT_SECRET"),
		InstagramClientID:    viper.GetString("INSTAGRAM_CLIENT_ID"),
		InstagramClientSecret: viper.GetString("INSTAGRAM_CLIENT_SECRET"),
		LinkedInClientID:     viper.GetString("LINKEDIN_CLIENT_ID"),
		LinkedInClientSecret: viper.GetString("LINKEDIN_CLIENT_SECRET"),
		OAuthCallbackBase:    viper.GetString("OAUTH_CALLBACK_BASE"),
		FrontendURL:          viper.GetString("FRONTEND_URL"),
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
