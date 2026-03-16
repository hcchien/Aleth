package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/aleth/auth/graph"
	"github.com/aleth/auth/internal/config"
	"github.com/aleth/auth/internal/db"
	"github.com/aleth/auth/internal/service"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg := config.Load()

	// ─── DB ──────────────────────────────────────────────────────────────────
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to database")
	}
	defer pool.Close()

	// ─── Services ─────────────────────────────────────────────────────────────
	tokenSvc := service.NewTokenService(
		cfg.AccessTokenSecret, cfg.RefreshTokenSecret,
		cfg.AccessTokenTTL, cfg.RefreshTokenTTL,
	)
	authSvc := service.NewAuthService(pool, tokenSvc, cfg.GoogleClientID)
	authSvc.SetFacebookAppID(cfg.FacebookAppID)
	authSvc.SetPasskeyRPID(cfg.PasskeyRPID)

	// ─── GraphQL ──────────────────────────────────────────────────────────────
	gqlSchema := graph.NewSchema(authSvc, tokenSvc)
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

	// GraphQL endpoint — auth middleware injects claims into context when a
	// valid Bearer token is present; unauthenticated requests still reach the
	// handler so public mutations (register, login) work.
	r.Handle("/graphql", authMiddleware(tokenSvc)(gqlHandler))

	// Internal endpoint called by the API gateway to validate access tokens.
	r.Post("/internal/validate", validateHandler(tokenSvc, authSvc))
	// Internal endpoints for gateway user enrichment.
	r.Post("/internal/users", usersHandler(authSvc))
	r.Get("/internal/user", userByUsernameHandler(authSvc))

	// ─── HTTP server ──────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("port", cfg.Port).Msg("auth service starting")
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
	log.Info().Msg("auth service stopped")
}

// authMiddleware validates a Bearer token (if present) and injects claims.
// Requests without a token are allowed through so public mutations work.
func authMiddleware(tokens *service.TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				if claims, err := tokens.ValidateAccessToken(tokenStr); err == nil {
					r = r.WithContext(graph.WithClaims(r.Context(), claims))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

type userLookupService interface {
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.User, error)
	GetUserByUsername(ctx context.Context, username string) (*db.User, error)
}

// userResp is the JSON shape returned by the internal user endpoints.
type userResp struct {
	ID          string  `json:"id"`
	DID         string  `json:"did"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
	TrustLevel  int16   `json:"trustLevel"`
	APEnabled   bool    `json:"apEnabled"`
	CreatedAt   string  `json:"createdAt"`
}

// usersHandler handles POST /internal/users for the API gateway.
// Accepts {"ids": ["uuid1","uuid2",...]} and returns [{...user...}].
func usersHandler(svc userLookupService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		ids := make([]uuid.UUID, 0, len(req.IDs))
		for _, s := range req.IDs {
			if id, err := uuid.Parse(s); err == nil {
				ids = append(ids, id)
			}
		}
		users, err := svc.GetUsersByIDs(r.Context(), ids)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resp := make([]userResp, len(users))
		for i, u := range users {
			resp[i] = userResp{
				ID:          u.ID.String(),
				DID:         u.DID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				Email:       u.Email,
				TrustLevel:  u.TrustLevel,
				APEnabled:   u.APEnabled,
				CreatedAt:   u.CreatedAt.Format(time.RFC3339),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// userByUsernameHandler handles GET /internal/user?username=... for the API gateway.
func userByUsernameHandler(svc userLookupService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		if username == "" {
			http.Error(w, "username required", http.StatusBadRequest)
			return
		}
		u, err := svc.GetUserByUsername(r.Context(), username)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if u == nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		resp := userResp{
			ID:          u.ID.String(),
			DID:         u.DID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Email:       u.Email,
			TrustLevel:  u.TrustLevel,
			APEnabled:   u.APEnabled,
			CreatedAt:   u.CreatedAt.Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// validateHandler handles POST /internal/validate for the API gateway.
func validateHandler(tokens *service.TokenService, authSvc *service.AuthService) http.HandlerFunc {
	type request struct {
		Token string `json:"token"`
	}
	type response struct {
		UserID      string `json:"user_id"`
		Username    string `json:"username"`
		TrustLevel  int32  `json:"trust_level"`
		IsSuspended bool   `json:"is_suspended"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		claims, err := tokens.ValidateAccessToken(req.Token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{
			UserID:     claims.Subject,
			Username:   claims.Username,
			TrustLevel: int32(claims.TrustLevel),
		})
	}
}
