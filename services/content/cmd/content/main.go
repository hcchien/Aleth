package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	graphql "github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/content/graph"
	"github.com/aleth/content/internal/config"
	"github.com/aleth/content/internal/db"
	"github.com/aleth/content/internal/events"
	"github.com/aleth/content/internal/service"
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

	// ─── Publisher ────────────────────────────────────────────────────────────
	var publisher events.Publisher
	if cfg.PubSubEnabled {
		pub, err := events.NewPubSubPublisher(context.Background(), cfg.PubSubProjectID, cfg.PubSubTopic)
		if err != nil {
			log.Fatal().Err(err).Msg("create pubsub publisher")
		}
		defer pub.Close()
		publisher = pub
		log.Info().Str("project", cfg.PubSubProjectID).Str("topic", cfg.PubSubTopic).Msg("using GCP Pub/Sub publisher")
	} else {
		publisher = &events.DirectPublisher{}
		log.Info().Msg("using DirectPublisher (local mode) — register handlers to receive events in-process")
	}

	// ─── Services ─────────────────────────────────────────────────────────────
	contentSvc := service.NewContentService(pool)
	contentSvc.SetSigningSecret(cfg.SigningSecret)
	contentSvc.SetPublisher(publisher)

	// ─── GraphQL ──────────────────────────────────────────────────────────────
	var gqlSchema *graphql.Schema
	if cfg.FederationURL != "" {
		gqlSchema = graph.NewSchemaWithFederation(contentSvc, cfg.FederationURL)
		log.Info().Str("federationURL", cfg.FederationURL).Msg("federation notifications enabled")
	} else {
		gqlSchema = graph.NewSchema(contentSvc)
	}
	gqlHandler := &relay.Handler{Schema: gqlSchema}

	// ─── Router ───────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Handle("/graphql", authMiddleware([]byte(cfg.AccessTokenSecret))(gqlHandler))

	// Internal endpoint: federation service fetches public posts by author.
	r.Get("/internal/posts", internalPostsHandler(pool))
	r.Get("/internal/pages/{slug}", internalPageHandler(pool))
	r.Get("/internal/pages/{slug}/feed", internalPageFeedHandler(pool))

	// ─── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("content service starting")
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
	log.Info().Msg("content service stopped")
}

// jwtClaims mirrors the auth service JWT payload.
type jwtClaims struct {
	jwt.RegisteredClaims
	Username   string `json:"username"`
	TrustLevel int    `json:"trust_level"`
}

// authMiddleware validates a Bearer token and injects user claims into the
// context. Requests without a valid token are allowed through (public reads).
func authMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := graph.WithClientIP(r.Context(), extractClientIP(r))
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{},
					func(tok *jwt.Token) (interface{}, error) {
						if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
							return nil, jwt.ErrTokenSignatureInvalid
						}
						return secret, nil
					},
					jwt.WithAudience("aleth:app"),
					jwt.WithIssuer("aleth"),
				)
				if err == nil {
					if c, ok := token.Claims.(*jwtClaims); ok && token.Valid {
						ctx = graph.WithClaims(ctx, graph.UserClaims{
							UserID:     c.Subject,
							Username:   c.Username,
							TrustLevel: c.TrustLevel,
						})
					}
				}
			}
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// internalPostHandler handles GET /internal/posts for the federation service.
// Query params: authorID (UUID), limit (int, max 20), before (RFC3339 timestamp).
func internalPostsHandler(pool *db.Pool) http.HandlerFunc {
	type postResp struct {
		ID        string `json:"id"`
		Content   string `json:"content"`
		CreatedAt string `json:"createdAt"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		authorIDStr := r.URL.Query().Get("authorID")
		if authorIDStr == "" {
			http.Error(w, "authorID required", http.StatusBadRequest)
			return
		}
		authorID, err := uuid.Parse(authorIDStr)
		if err != nil {
			http.Error(w, "invalid authorID", http.StatusBadRequest)
			return
		}

		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 20 {
				limit = n
			}
		}

		var before *time.Time
		if b := r.URL.Query().Get("before"); b != "" {
			if t, err := time.Parse(time.RFC3339, b); err == nil {
				before = &t
			}
		}

		posts, err := pool.ListPublicPostsByAuthor(r.Context(), authorID, limit, before)
		if err != nil {
			log.Error().Err(err).Str("authorID", authorIDStr).Msg("internal/posts: db error")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := make([]postResp, len(posts))
		for i, p := range posts {
			resp[i] = postResp{
				ID:        p.ID.String(),
				Content:   p.Content,
				CreatedAt: p.CreatedAt.UTC().Format(time.RFC3339),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func internalPageHandler(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		page, err := pool.GetPageBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		type pageResp struct {
			ID        string `json:"id"`
			Slug      string `json:"slug"`
			Name      string `json:"name"`
			APEnabled bool   `json:"apEnabled"`
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pageResp{
			ID: page.ID.String(), Slug: page.Slug,
			Name: page.Name, APEnabled: page.APEnabled,
		})
	}
}

func internalPageFeedHandler(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		page, err := pool.GetPageBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		limit := 20
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 20 {
				limit = n
			}
		}
		var before *time.Time
		if b := r.URL.Query().Get("before"); b != "" {
			if t, err := time.Parse(time.RFC3339, b); err == nil {
				before = &t
			}
		}
		posts, err := pool.ListPublicPostsByPage(r.Context(), page.ID, limit, before)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		type postResp struct {
			ID        string    `json:"id"`
			Content   string    `json:"content"`
			CreatedAt time.Time `json:"createdAt"`
		}
		out := make([]postResp, 0, len(posts))
		for _, p := range posts {
			out = append(out, postResp{ID: p.ID.String(), Content: p.Content, CreatedAt: p.CreatedAt})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func extractClientIP(r *http.Request) string {
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
