package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/counter/internal/config"
	"github.com/aleth/counter/internal/db"
	"github.com/aleth/counter/internal/service"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	// ─── DB ───────────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	store, err := db.New(ctx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		log.Fatal().Err(err).Msg("connect to database")
	}
	defer store.Close()

	svc := service.New(store)

	// ─── Pub/Sub subscriber ───────────────────────────────────────────────────
	subCtx, subCancel := context.WithCancel(context.Background())
	defer subCancel()

	psClient, err := pubsub.NewClient(subCtx, cfg.PubSubProjectID)
	if err != nil {
		log.Fatal().Err(err).Msg("create pubsub client")
	}
	defer psClient.Close()

	sub := psClient.Subscription(cfg.PubSubSubscription)
	go func() {
		log.Info().Str("subscription", cfg.PubSubSubscription).Msg("starting Pub/Sub subscriber")
		err := sub.Receive(subCtx, func(ctx context.Context, msg *pubsub.Message) {
			eventType := msg.Attributes["event_type"]
			svc.HandleEvent(ctx, eventType, msg.Data)
			msg.Ack()
		})
		if err != nil && subCtx.Err() == nil {
			log.Error().Err(err).Msg("pubsub receive error")
		}
	}()

	// ─── Health endpoint ──────────────────────────────────────────────────────
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: mux,
	}

	go func() {
		log.Info().Int("port", cfg.Port).Msg("counter service listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server error")
		}
	}()

	// ─── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down counter service")
	subCancel() // stop Pub/Sub receive

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
}
