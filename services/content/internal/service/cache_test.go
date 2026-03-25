package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/aleth/content/internal/db"
)

// ─── fakeCache ────────────────────────────────────────────────────────────────

// fakeCache is an in-memory cacher for unit tests.
// It stores JSON-serialised blobs keyed by string, exactly as the real Redis
// client does, which means Get/Set round-trips exercise the json marshal path.
type fakeCache struct {
	mu       sync.RWMutex
	store    map[string][]byte
	getCalls []string // keys passed to Get
	setCalls []string // keys passed to Set
	delCalls []string // keys passed to Del
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: make(map[string][]byte)}
}

func (f *fakeCache) Get(_ context.Context, key string, dst any) (bool, error) {
	f.mu.Lock()
	f.getCalls = append(f.getCalls, key)
	f.mu.Unlock()

	f.mu.RLock()
	raw, ok := f.store[key]
	f.mu.RUnlock()
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(raw, dst)
}

func (f *fakeCache) Set(_ context.Context, key string, src any, _ time.Duration) error {
	raw, err := json.Marshal(src)
	if err != nil {
		return err
	}
	f.mu.Lock()
	f.setCalls = append(f.setCalls, key)
	f.store[key] = raw
	f.mu.Unlock()
	return nil
}

func (f *fakeCache) Del(_ context.Context, keys ...string) error {
	f.mu.Lock()
	f.delCalls = append(f.delCalls, keys...)
	for _, k := range keys {
		delete(f.store, k)
	}
	f.mu.Unlock()
	return nil
}

// Prime stores a value directly so tests can set up cache hits without going
// through the service layer.
func (f *fakeCache) Prime(key string, value any) {
	raw, _ := json.Marshal(value)
	f.mu.Lock()
	f.store[key] = raw
	f.mu.Unlock()
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newServiceWithCache returns a ContentService wired with the given fake cache
// and a default fakeContentStore.
func newServiceWithCache(fc *fakeCache) (*ContentService, *fakeContentStore) {
	store := newFakeContentStore()
	svc := NewContentService(store)
	svc.cache = fc
	return svc, store
}

// ─── GetPost cache tests ──────────────────────────────────────────────────────

// TestGetPost_CacheHit verifies that a primed cache entry is returned without
// touching the DB.
func TestGetPost_CacheHit(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	cachedPost := db.Post{
		ID:      postID,
		Content: "cached content",
	}
	fc.Prime(postCacheKey(postID, nil), cachedPost)

	// DB should NOT be called.
	dbCalled := false
	store.getPostByIDFn = func(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		dbCalled = true
		return db.Post{}, nil
	}

	got, err := svc.GetPost(context.Background(), postID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Content != "cached content" {
		t.Fatalf("expected cached post, got %+v", got)
	}
	if dbCalled {
		t.Error("DB was called on a cache hit — expected cache-only path")
	}
}

// TestGetPost_CacheMiss verifies that a cache miss falls through to the DB and
// the result is subsequently stored in the cache.
func TestGetPost_CacheMiss(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	dbPost := db.Post{
		ID:        postID,
		AuthorID:  uuid.New(),
		Content:   "db content",
		CreatedAt: time.Now(),
	}

	store.getPostByIDFn = func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return dbPost, nil
	}

	got, err := svc.GetPost(context.Background(), postID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Content != "db content" {
		t.Fatalf("expected DB post, got %+v", got)
	}

	// Cache write is async; give the goroutine a moment to complete.
	time.Sleep(20 * time.Millisecond)
	expectedKey := postCacheKey(postID, nil)
	fc.mu.RLock()
	_, cached := fc.store[expectedKey]
	fc.mu.RUnlock()
	if !cached {
		t.Errorf("expected post to be written to cache under key %q", expectedKey)
	}
}

// TestGetPost_NilCache verifies that the service works with no cache configured
// (nopCache is the default when SetCache is never called).
func TestGetPost_NilCache(t *testing.T) {
	store := newFakeContentStore()
	svc := NewContentService(store) // cache is nopCache{} by default

	postID := uuid.New()
	store.getPostByIDFn = func(_ context.Context, id uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return db.Post{ID: id, Content: "direct"}, nil
	}

	got, err := svc.GetPost(context.Background(), postID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Content != "direct" {
		t.Fatalf("expected direct DB post, got %+v", got)
	}
}

// TestGetPost_NotFound verifies that pgx.ErrNoRows maps to (nil, nil).
func TestGetPost_NotFound(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	store.getPostByIDFn = func(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		return db.Post{}, pgx.ErrNoRows
	}

	got, err := svc.GetPost(context.Background(), uuid.New(), nil)
	if err != nil {
		t.Fatalf("expected nil error on not-found, got %v", err)
	}
	if got != nil {
		t.Errorf("expected nil post on not-found, got %+v", got)
	}
}

// TestGetPost_ViewerScopedKey checks that authenticated and anonymous requests
// use different cache keys so viewer-specific fields (IsLiked, ViewerEmotion)
// are isolated from each other.
func TestGetPost_ViewerScopedKey(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	viewerID := uuid.New()

	anonPost := db.Post{ID: postID, Content: "anon version"}
	viewerPost := db.Post{ID: postID, Content: "viewer version"}
	fc.Prime(postCacheKey(postID, nil), anonPost)
	fc.Prime(postCacheKey(postID, &viewerID), viewerPost)

	store.getPostByIDFn = func(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (db.Post, error) {
		t.Error("DB should not be called when both keys are cached")
		return db.Post{}, nil
	}

	gotAnon, _ := svc.GetPost(context.Background(), postID, nil)
	gotViewer, _ := svc.GetPost(context.Background(), postID, &viewerID)

	if gotAnon.Content != "anon version" {
		t.Errorf("anon cache: want %q, got %q", "anon version", gotAnon.Content)
	}
	if gotViewer.Content != "viewer version" {
		t.Errorf("viewer cache: want %q, got %q", "viewer version", gotViewer.Content)
	}
}

// ─── DeletePost cache eviction tests ─────────────────────────────────────────

// TestDeletePost_EvictsAnonCache verifies that deleting a post evicts the
// anonymous cache key so the deletion is immediately visible to public readers.
func TestDeletePost_EvictsAnonCache(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	authorID := uuid.New()

	fc.Prime(postCacheKey(postID, nil), db.Post{ID: postID, Content: "stale"})

	store.softDeletePostFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }

	if err := svc.DeletePost(context.Background(), postID, authorID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	anonKey := postCacheKey(postID, nil)
	fc.mu.RLock()
	_, anonStillPresent := fc.store[anonKey]
	fc.mu.RUnlock()

	if anonStillPresent {
		t.Errorf("anon cache key %q should be evicted after DeletePost", anonKey)
	}
}

// ─── LikePost / ReactPost / UnlikePost eviction tests ────────────────────────

// TestLikePost_EvictsViewerCache verifies that liking a post evicts the
// viewer-specific cache entry so the updated IsLiked state is reflected.
func TestLikePost_EvictsViewerCache(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	userID := uuid.New()

	fc.Prime(postCacheKey(postID, &userID), db.Post{ID: postID, Content: "before like"})

	store.likePostFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }

	if err := svc.LikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewerKey := postCacheKey(postID, &userID)
	fc.mu.RLock()
	_, present := fc.store[viewerKey]
	fc.mu.RUnlock()

	if present {
		t.Errorf("viewer cache key %q should be evicted after LikePost", viewerKey)
	}
}

// TestReactPost_EvictsViewerCache verifies that reacting to a post evicts the
// viewer-specific cache entry so the updated emotion is reflected.
func TestReactPost_EvictsViewerCache(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	userID := uuid.New()
	fc.Prime(postCacheKey(postID, &userID), db.Post{ID: postID})

	store.reactPostFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ *string) error {
		return nil
	}

	if err := svc.ReactPost(context.Background(), postID, userID, "love", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fc.mu.RLock()
	_, present := fc.store[postCacheKey(postID, &userID)]
	fc.mu.RUnlock()

	if present {
		t.Error("viewer cache should be evicted after ReactPost")
	}
}

// TestUnlikePost_EvictsViewerCache verifies that removing a like evicts the
// viewer-specific cache entry.
func TestUnlikePost_EvictsViewerCache(t *testing.T) {
	fc := newFakeCache()
	svc, store := newServiceWithCache(fc)

	postID := uuid.New()
	userID := uuid.New()
	fc.Prime(postCacheKey(postID, &userID), db.Post{ID: postID})

	store.unlikePostFn = func(_ context.Context, _ uuid.UUID, _ uuid.UUID) error { return nil }

	if err := svc.UnlikePost(context.Background(), postID, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fc.mu.RLock()
	_, present := fc.store[postCacheKey(postID, &userID)]
	fc.mu.RUnlock()

	if present {
		t.Error("viewer cache should be evicted after UnlikePost")
	}
}

// TestPostCacheKey_Schema documents the expected key format. Any change will
// break this test, alerting developers that cached keys need to be invalidated
// in existing deployments.
func TestPostCacheKey_Schema(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	viewer := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	if got := postCacheKey(id, nil); got != "post:11111111-1111-1111-1111-111111111111:anon" {
		t.Errorf("unexpected anon key: %s", got)
	}
	if got := postCacheKey(id, &viewer); got != "post:11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222" {
		t.Errorf("unexpected viewer key: %s", got)
	}
}
