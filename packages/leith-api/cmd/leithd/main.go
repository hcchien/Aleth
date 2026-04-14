package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	middleware_chi "github.com/go-chi/chi/v5/middleware"
	"github.com/leith/api/internal/api"
	"github.com/leith/api/internal/middleware"
	"github.com/leith/api/internal/service/content"
	"github.com/leith/api/internal/service/reputation"
	"github.com/leith/api/internal/store"
)

// Server implements the generated api.ServerInterface
type Server struct {
	api.Unimplemented
	store      store.Store
	contentSvc *content.Service
	repSvc     *reputation.Service
}

// Handle L0 Guest Sign In via OAuth
func (s *Server) PostAuthOauth(w http.ResponseWriter, r *http.Request) {
	var req api.OAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Provider == "" || req.Token == "" {
		http.Error(w, "provider and token are required", http.StatusBadRequest)
		return
	}

	// Mock DID derivation from provider/token.
	did := fmt.Sprintf("oauth:%s:%s", req.Provider, req.Token)
	did = strings.ReplaceAll(did, " ", "")

	if err := s.store.CreateUser(&store.User{
		DID:       did,
		OAuthID:   req.Token,
		TrustTier: store.L0_GUEST,
	}); err != nil {
		http.Error(w, "failed to provision oauth user", http.StatusInternalServerError)
		return
	}

	// Parse the OAuthRequest (e.g. from Google/Apple)
	// Verify the JWT signature using the provider's JWKS

	// Mock: Assign an L0 Guest Session Token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.AuthSuccess{
		Did:       did,
		TrustTier: int(store.L0_GUEST),
	})
}

func (s *Server) PostPosts(w http.ResponseWriter, r *http.Request) {
	var req api.CreatePostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	did, _ := r.Context().Value("user_did").(string)
	if did == "" {
		http.Error(w, "missing auth context DID", http.StatusUnauthorized)
		return
	}
	if did != req.AuthorDid {
		http.Error(w, "authorDid must match authenticated DID", http.StatusForbidden)
		return
	}

	post, err := s.contentSvc.CreatePost(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toAPIPost(*post))
}

func (s *Server) GetPosts(w http.ResponseWriter, r *http.Request, params api.GetPostsParams) {
	limit := 20
	offset := 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	posts, err := s.repSvc.GetPublicFeed(limit, offset)
	if err != nil {
		http.Error(w, "failed to load posts", http.StatusInternalServerError)
		return
	}

	resp := make([]api.Post, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, toAPIPost(p))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func toAPIPost(p store.Post) api.Post {
	postID := strconv.FormatInt(p.ID, 10)
	var parentID *string
	if p.ParentID.Valid {
		v := strconv.FormatInt(p.ParentID.Int64, 10)
		parentID = &v
	}

	return api.Post{
		Id:              postID,
		Body:            p.Body,
		MediaHashes:     &p.MediaHashes,
		ParentId:        parentID,
		Timestamp:       int(p.Timestamp),
		AuthorDid:       p.AuthorDID,
		Signature:       p.Signature,
		VisibilityScore: ptrFloat32(float32(p.VisibilityScore)),
	}
}

func ptrFloat32(v float32) *float32 {
	return &v
}

func initStoreFromEnv() (store.Store, error) {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("LEITH_STORE_BACKEND")))
	if backend == "" {
		backend = "memory"
	}

	switch backend {
	case "memory":
		return store.NewMemoryStore(), nil
	case "sql":
		driver := strings.TrimSpace(os.Getenv("LEITH_DB_DRIVER"))
		if driver == "" {
			driver = "sqlite3"
		}
		dsn := strings.TrimSpace(os.Getenv("LEITH_DB_DSN"))
		if dsn == "" {
			dsn = "./leith.db"
		}
		dbStore, err := store.OpenSQLStore(driver, dsn)
		if err != nil {
			return nil, fmt.Errorf("sql backend init failed (driver=%s dsn=%s): %w", driver, dsn, err)
		}
		return dbStore, nil
	default:
		return nil, fmt.Errorf("unsupported LEITH_STORE_BACKEND=%q", backend)
	}
}

func configuredStoreBackend() string {
	backend := strings.ToLower(strings.TrimSpace(os.Getenv("LEITH_STORE_BACKEND")))
	if backend == "" {
		return "memory"
	}
	return backend
}

func newHandler(db store.Store) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware_chi.Logger)
	r.Use(middleware.AuthContext)
	r.Use(middleware.RateLimiter(db))

	serverImpl := &Server{
		store:      db,
		contentSvc: content.NewService(db),
		repSvc:     reputation.NewService(db),
	}
	api.HandlerFromMux(serverImpl, r)
	return r
}

func main() {
	db, err := initStoreFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}
	handler := newHandler(db)

	port := 8080
	log.Printf("Store backend initialized: %s", configuredStoreBackend())
	fmt.Printf("Starting VeriFlow API Server on :%d...\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
