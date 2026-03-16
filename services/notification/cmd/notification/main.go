package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/notification/internal/config"
	"github.com/aleth/notification/internal/db"
	"github.com/aleth/notification/internal/service"
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

	svc := service.NewNotificationService(store)

	// ─── Read queue worker ────────────────────────────────────────────────────
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	go svc.StartReadWorker(workerCtx)

	// ─── Pub/Sub subscriber ───────────────────────────────────────────────────
	subCancel := func() {} // no-op if Pub/Sub is disabled
	if cfg.PubSubProjectID != "" {
		subCtx, cancel := context.WithCancel(context.Background())
		subCancel = cancel

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
	}

	// ─── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// All notification routes require authentication.
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware([]byte(cfg.AccessTokenSecret)))

		// GET /notifications/count → {"unread": N}
		r.Get("/notifications/count", func(w http.ResponseWriter, r *http.Request) {
			userID := userIDFromCtx(r.Context())
			count, err := svc.CountUnread(r.Context(), userID)
			if err != nil {
				log.Error().Err(err).Msg("count unread")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]int64{"unread": count})
		})

		// GET /notifications?limit=50 → [{...}, ...]
		r.Get("/notifications", func(w http.ResponseWriter, r *http.Request) {
			userID := userIDFromCtx(r.Context())
			limit := 50
			if l := r.URL.Query().Get("limit"); l != "" {
				fmt.Sscanf(l, "%d", &limit)
			}
			items, err := svc.List(r.Context(), userID, limit)
			if err != nil {
				log.Error().Err(err).Msg("list notifications")
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if items == nil {
				items = []db.Notification{}
			}
			writeJSON(w, items)
		})

		// POST /notifications/mark-read → enqueues a mark-read job; returns 202.
		// Body (optional): {"ids": ["uuid1", "uuid2"]}
		// Omit body or send {} to mark all of the user's unread notifications.
		r.Post("/notifications/mark-read", func(w http.ResponseWriter, r *http.Request) {
			userID := userIDFromCtx(r.Context())

			var body struct {
				IDs []string `json:"ids"`
			}
			if r.ContentLength > 0 {
				json.NewDecoder(r.Body).Decode(&body)
			}

			if len(body.IDs) == 0 {
				svc.EnqueueMarkAllRead(userID)
			} else {
				ids := make([]uuid.UUID, 0, len(body.IDs))
				for _, s := range body.IDs {
					id, err := uuid.Parse(s)
					if err != nil {
						http.Error(w, "invalid id: "+s, http.StatusBadRequest)
						return
					}
					ids = append(ids, id)
				}
				svc.EnqueueMarkRead(userID, ids)
			}
			// 202 Accepted: the request has been queued; the DB write is async.
			w.WriteHeader(http.StatusAccepted)
		})
	})

	// ─── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatal().Err(err).Msg("listen")
	}
	log.Info().Str("port", cfg.Port).Msg("notification service starting")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("serve")
		}
	}()

	<-quit
	subCancel()    // stop Pub/Sub receiver (no-op if disabled)
	workerCancel() // signal read worker to drain and stop

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error().Err(err).Msg("shutdown")
	}
	log.Info().Msg("notification service stopped")
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type contextKey string

const ctxKeyUserID contextKey = "user_id"

func userIDFromCtx(ctx context.Context) uuid.UUID {
	v, _ := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func authMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tokenStr := authHeader[7:]

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return secret, nil
			})
			if err != nil || !token.Valid {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			sub, _ := claims["sub"].(string)
			userID, err := uuid.Parse(sub)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
