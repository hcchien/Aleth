package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/federation/internal/client"
	"github.com/aleth/federation/internal/config"
	"github.com/aleth/federation/internal/db"
	"github.com/aleth/federation/internal/handler"
	"github.com/aleth/federation/internal/worker"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	// ─── DB ───────────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to database")
	}
	defer pool.Close()

	// ─── Clients ──────────────────────────────────────────────────────────────
	authClient := client.NewAuthClient(cfg.AuthServiceURL)
	contentClient := client.NewContentClient(cfg.ContentServiceURL)

	// ─── Handler ──────────────────────────────────────────────────────────────
	h := handler.New(cfg, pool, authClient, contentClient)

	// ─── Delivery worker ──────────────────────────────────────────────────────
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	dw := worker.New(cfg, pool)
	go dw.Run(workerCtx)

	// ─── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/.well-known/webfinger", h.ServeWebFinger)
	r.Get("/@{username}", h.ServeActor)
	r.Get("/@{username}/outbox", h.ServeOutbox)
	r.Post("/@{username}/inbox", h.ServeInbox)

	// Internal endpoint: called by content service after post creation.
	r.Post("/internal/post-created", h.NotifyPostCreated)

	// ─── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Str("domain", cfg.Domain).Msg("federation service starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Fatal().Err(err).Msg("forced shutdown")
	}
	log.Info().Msg("federation service stopped")
}
