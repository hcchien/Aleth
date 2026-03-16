package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoad(t *testing.T) {
	viper.Reset()
	t.Setenv("CONTENT_DATABASE_URL", "postgres://localhost/content")
	t.Setenv("CONTENT_ACCESS_TOKEN_SECRET", "secret")
	t.Setenv("CONTENT_PORT", "9091")

	cfg := Load()
	if cfg.Port != "9091" {
		t.Fatalf("unexpected port: %s", cfg.Port)
	}
	if cfg.DatabaseURL == "" || cfg.AccessTokenSecret == "" {
		t.Fatalf("expected required fields")
	}
	if cfg.SigningSecret == "" {
		t.Fatalf("expected signing secret")
	}
	if cfg.SigningSecret != cfg.AccessTokenSecret {
		t.Fatalf("expected signing secret fallback to access token secret")
	}
}
