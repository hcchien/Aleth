package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	viper.Reset()
	t.Setenv("AUTH_DATABASE_URL", "postgres://localhost/test")
	t.Setenv("AUTH_ACCESS_TOKEN_SECRET", "access")
	t.Setenv("AUTH_REFRESH_TOKEN_SECRET", "refresh")
	t.Setenv("AUTH_GOOGLE_CLIENT_ID", "gid")
	t.Setenv("AUTH_PORT", "9090")
	t.Setenv("AUTH_ACCESS_TOKEN_TTL", "30m")
	t.Setenv("AUTH_REFRESH_TOKEN_TTL", "24h")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Fatalf("unexpected port: %s", cfg.Port)
	}
	if cfg.GoogleClientID != "gid" {
		t.Fatalf("unexpected google client id: %s", cfg.GoogleClientID)
	}
}
