package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/admin/graph"
	"github.com/aleth/admin/internal/config"
	"github.com/aleth/admin/internal/db"
	"github.com/aleth/admin/internal/service"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	if cfg.AuthDatabaseURL == "" || cfg.ContentDatabaseURL == "" {
		log.Fatal().Msg("ADMIN_AUTH_DATABASE_URL and ADMIN_CONTENT_DATABASE_URL are required")
	}
	if cfg.JWTSecret == "" {
		log.Fatal().Msg("ADMIN_JWT_SECRET is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ─── DB pools ─────────────────────────────────────────────────────────────
	authPool, err := db.NewAuthPool(ctx, cfg.AuthDatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to auth db")
	}
	defer authPool.Close()

	contentPool, err := db.NewContentPool(ctx, cfg.ContentDatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to content db")
	}
	defer contentPool.Close()

	// ─── Service + schema ─────────────────────────────────────────────────────
	adminSvc := service.New(authPool, contentPool, cfg.JWTSecret)
	gqlSchema := graph.NewSchema(adminSvc)
	gqlHandler := &relay.Handler{Schema: gqlSchema}

	// ─── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	// CORS — only allow the admin frontend origin.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", cfg.AllowedOrigin)
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
			if req.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Auth middleware injects admin into context (if token valid).
	r.With(graph.AuthMiddleware(adminSvc)).Post("/graphql", gqlHandler.ServeHTTP)

	// ─── Start ────────────────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Info().Str("addr", addr).Msg("admin service started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
	log.Info().Msg("admin service stopped")
}
