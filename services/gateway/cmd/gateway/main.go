package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/gateway/graph"
	"github.com/aleth/gateway/internal/client"
	"github.com/aleth/gateway/internal/config"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	authClient := client.NewAuthClient(cfg.AuthServiceURL)
	feedClient := client.NewFeedClient(cfg.FeedServiceURL)
	contentClient := client.NewContentClient(cfg.ContentServiceURL)
	notifClient := client.NewNotificationClient(cfg.NotificationURL)
	federationClient := client.NewFederationClient(cfg.FederationURL)

	gqlSchema := graph.NewSchema(authClient, contentClient, feedClient, notifClient, federationClient)
	gqlHandler := &relay.Handler{Schema: gqlSchema}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(authMiddleware(cfg.AccessTokenSecret))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Handle("/graphql", gqlHandler)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("gateway starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("forced shutdown")
	}
	log.Info().Msg("gateway stopped")
}

// jwtClaims mirrors the payload issued by the Auth service.
type jwtClaims struct {
	jwt.RegisteredClaims
	Username   string `json:"username"`
	TrustLevel int    `json:"trust_level"`
}

// authMiddleware validates a Bearer JWT (if present) and injects claims + auth header into context.
// Requests without a token pass through so public mutations (register, login) work.
func authMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			ctx := graph.WithAuthHeader(r.Context(), authHeader)

			if strings.HasPrefix(authHeader, "Bearer ") && secret != "" {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(secret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(*jwtClaims); ok {
						ctx = graph.WithUserClaims(ctx, graph.UserClaims{
							UserID:     claims.Subject,
							Username:   claims.Username,
							TrustLevel: claims.TrustLevel,
						})
					}
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
