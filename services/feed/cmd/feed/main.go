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
	"github.com/google/uuid"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/feed/graph"
	"github.com/aleth/feed/internal/config"
	"github.com/aleth/feed/internal/db"
	"github.com/aleth/feed/internal/service"
)

// jwtClaims mirrors the payload issued by the Auth service.
type jwtClaims struct {
	jwt.RegisteredClaims
	Username   string `json:"username"`
	TrustLevel int    `json:"trust_level"`
}

// authMiddleware extracts a Bearer JWT and injects the viewer UUID into context.
// Requests without a valid token pass through so public queries work.
func authMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") && secret != "" {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, jwt.ErrSignatureInvalid
					}
					return []byte(secret), nil
				})
				if err == nil && token.Valid {
					if claims, ok := token.Claims.(*jwtClaims); ok {
						if id, err := uuid.Parse(claims.Subject); err == nil {
							ctx = graph.WithViewerID(ctx, id)
						}
					}
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// newRouter builds the base chi router with healthz and standard middleware.
// Exported for tests.
func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	return r
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	// ── DB pools ────────────────────────────────────────────────────────────
	contentPool, err := pgxpool.New(context.Background(), cfg.ContentDatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to content DB")
	}
	defer contentPool.Close()

	authPool, err := pgxpool.New(context.Background(), cfg.AuthDatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to auth DB")
	}
	defer authPool.Close()

	// ── Service wiring ──────────────────────────────────────────────────────
	authStore := db.NewAuthStore(authPool)
	contentStore := db.NewContentStore(contentPool)
	svc := service.NewFeedService(authStore, contentStore)

	// ── GraphQL handler ─────────────────────────────────────────────────────
	gqlSchema := graph.NewSchema(svc)
	gqlHandler := &relay.Handler{Schema: gqlSchema}

	// ── Router ──────────────────────────────────────────────────────────────
	r := newRouter()
	r.Handle("/graphql", authMiddleware(cfg.AccessTokenSecret)(gqlHandler))

	// ── HTTP server ─────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("feed service starting")
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
	log.Info().Msg("feed service stopped")
}
