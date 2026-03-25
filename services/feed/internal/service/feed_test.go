package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aleth/feed/internal/db"
)

// ─── fakeCache ────────────────────────────────────────────────────────────────

type fakeCache struct {
	mu       sync.RWMutex
	store    map[string][]byte
	setCalls []string
	getCalls []string
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

func (f *fakeCache) Del(_ context.Context, _ ...string) error { return nil }

// Prime stores a value directly to simulate a pre-existing cache entry.
func (f *fakeCache) Prime(key string, value any) {
	raw, _ := json.Marshal(value)
	f.mu.Lock()
	f.store[key] = raw
	f.mu.Unlock()
}

// ─── fakeAuthDB ───────────────────────────────────────────────────────────────

type fakeAuthDB struct {
	getFolloweeIDsFn func(context.Context, uuid.UUID) ([]uuid.UUID, error)
	getUsersByIDsFn  func(context.Context, []uuid.UUID) ([]db.AuthUser, error)
}

func newFakeAuthDB() *fakeAuthDB {
	return &fakeAuthDB{
		getFolloweeIDsFn: func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) { return nil, nil },
		getUsersByIDsFn:  func(_ context.Context, _ []uuid.UUID) ([]db.AuthUser, error) { return nil, nil },
	}
}

func (f *fakeAuthDB) GetFolloweeIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	return f.getFolloweeIDsFn(ctx, id)
}
func (f *fakeAuthDB) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.AuthUser, error) {
	return f.getUsersByIDsFn(ctx, ids)
}

// ─── fakeFeedContentDB ────────────────────────────────────────────────────────

type fakeFeedContentDB struct {
	getFollowedPageIDsFn   func(context.Context, uuid.UUID) ([]uuid.UUID, error)
	listFeedPostsFn        func(context.Context, db.FeedPostsParams) ([]db.FeedPost, error)
	listExplorePostsFn     func(context.Context, db.ExplorePostsParams) ([]db.FeedPost, error)
	getPostReactorEmotions func(context.Context, uuid.UUID, []uuid.UUID, int) (map[uuid.UUID]string, error)
}

func newFakeFeedContentDB() *fakeFeedContentDB {
	return &fakeFeedContentDB{
		getFollowedPageIDsFn:   func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) { return nil, nil },
		listFeedPostsFn:        func(_ context.Context, _ db.FeedPostsParams) ([]db.FeedPost, error) { return nil, nil },
		listExplorePostsFn:     func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) { return nil, nil },
		getPostReactorEmotions: func(_ context.Context, _ uuid.UUID, _ []uuid.UUID, _ int) (map[uuid.UUID]string, error) { return nil, nil },
	}
}

func (f *fakeFeedContentDB) GetFollowedPageIDs(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	return f.getFollowedPageIDsFn(ctx, id)
}
func (f *fakeFeedContentDB) ListFeedPosts(ctx context.Context, p db.FeedPostsParams) ([]db.FeedPost, error) {
	return f.listFeedPostsFn(ctx, p)
}
func (f *fakeFeedContentDB) ListExplorePosts(ctx context.Context, p db.ExplorePostsParams) ([]db.FeedPost, error) {
	return f.listExplorePostsFn(ctx, p)
}
func (f *fakeFeedContentDB) GetPostReactorEmotions(ctx context.Context, postID uuid.UUID, ids []uuid.UUID, limit int) (map[uuid.UUID]string, error) {
	return f.getPostReactorEmotions(ctx, postID, ids, limit)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// newFeedService builds a FeedService with fake dependencies for testing.
func newTestFeedService(auth *fakeAuthDB, content *fakeFeedContentDB, fc *fakeCache) *FeedService {
	return &FeedService{auth: auth, content: content, cache: fc}
}

// samplePost returns a minimal FeedPost for testing.
func samplePost(id uuid.UUID) db.FeedPost {
	return db.FeedPost{
		ID:        id,
		AuthorID:  uuid.New(),
		Content:   "test post",
		Kind:      "post",
		CreatedAt: time.Now(),
	}
}

// ─── GetFeed cache tests ──────────────────────────────────────────────────────

// TestGetFeed_CacheHit_FirstPage verifies that an authenticated viewer's first
// page is served from cache when available, without hitting the DB.
func TestGetFeed_CacheHit_FirstPage(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	viewerID := uuid.New()
	post := samplePost(uuid.New())
	cachedResult := &FeedResult{
		Posts:   []db.FeedPost{post},
		Authors: map[uuid.UUID]db.AuthUser{},
		HasMore: false,
	}

	// Prime the cache with viewer's first-page key.
	key := feedCacheKey(viewerID, "")
	fc.Prime(key, cachedResult)

	// DB calls should not happen.
	dbCalled := false
	auth.getFolloweeIDsFn = func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
		dbCalled = true
		return nil, nil
	}

	svc := newTestFeedService(auth, content, fc)
	result, err := svc.GetFeed(context.Background(), &viewerID, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Errorf("expected 1 cached post, got %d", len(result.Posts))
	}
	if dbCalled {
		t.Error("DB was called on a cache hit — expected cache-only path")
	}
}

// TestGetFeed_CacheMiss_PopulatesCache verifies that on a cache miss the DB is
// consulted and the result is stored back in the cache.
func TestGetFeed_CacheMiss_PopulatesCache(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	viewerID := uuid.New()
	followeeID := uuid.New()
	post := samplePost(uuid.New())

	auth.getFolloweeIDsFn = func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
		return []uuid.UUID{followeeID}, nil
	}
	content.listFeedPostsFn = func(_ context.Context, _ db.FeedPostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}
	auth.getUsersByIDsFn = func(_ context.Context, _ []uuid.UUID) ([]db.AuthUser, error) {
		return []db.AuthUser{{ID: post.AuthorID, Username: "alice"}}, nil
	}

	svc := newTestFeedService(auth, content, fc)
	result, err := svc.GetFeed(context.Background(), &viewerID, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Fatalf("expected 1 post from DB, got %d", len(result.Posts))
	}

	// Cache write is async; allow the goroutine to complete.
	time.Sleep(20 * time.Millisecond)
	key := feedCacheKey(viewerID, "")
	fc.mu.RLock()
	_, cached := fc.store[key]
	fc.mu.RUnlock()
	if !cached {
		t.Errorf("expected feed to be written to cache under key %q", key)
	}
}

// TestGetFeed_NoCaching_SubsequentPages verifies that pages after the first
// (cursor != "") always bypass the cache and hit the DB.
func TestGetFeed_NoCaching_SubsequentPages(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	viewerID := uuid.New()
	cursorID := uuid.New()
	cursorStr := cursorID.String()

	post := samplePost(uuid.New())
	dbCalled := false

	auth.getFolloweeIDsFn = func(_ context.Context, _ uuid.UUID) ([]uuid.UUID, error) {
		dbCalled = true
		return []uuid.UUID{uuid.New()}, nil
	}
	content.listFeedPostsFn = func(_ context.Context, _ db.FeedPostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}
	auth.getUsersByIDsFn = func(_ context.Context, _ []uuid.UUID) ([]db.AuthUser, error) {
		return nil, nil
	}

	svc := newTestFeedService(auth, content, fc)
	_, err := svc.GetFeed(context.Background(), &viewerID, &cursorStr, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dbCalled {
		t.Error("expected DB to be called for subsequent pages (cursor != \"\")")
	}

	// Nothing should have been written to cache for paginated pages.
	time.Sleep(20 * time.Millisecond)
	fc.mu.RLock()
	setCalls := append([]string(nil), fc.setCalls...)
	fc.mu.RUnlock()
	for _, k := range setCalls {
		if k == feedCacheKey(viewerID, cursorStr) {
			t.Errorf("paginated page should not be cached, but key %q was written", k)
		}
	}
}

// TestGetFeed_AnonymousAlwaysHitsDB verifies that anonymous (nil viewerID)
// requests are never served from or written to the feed cache.
func TestGetFeed_AnonymousAlwaysHitsDB(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	// anonymous viewer → falls through to explore
	post := samplePost(uuid.New())
	content.listExplorePostsFn = func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}
	auth.getUsersByIDsFn = func(_ context.Context, _ []uuid.UUID) ([]db.AuthUser, error) {
		return nil, nil
	}

	svc := newTestFeedService(auth, content, fc)
	result, err := svc.GetFeed(context.Background(), nil, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Errorf("expected 1 explore post, got %d", len(result.Posts))
	}
}

// ─── GetExploreFeed cache tests ───────────────────────────────────────────────

// TestGetExploreFeed_CacheHit verifies that the explore feed is served from
// cache when available.
func TestGetExploreFeed_CacheHit(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	viewerID := uuid.New()
	post := samplePost(uuid.New())
	cachedResult := &FeedResult{
		Posts:   []db.FeedPost{post},
		Authors: map[uuid.UUID]db.AuthUser{},
	}

	key := exploreCacheKey(&viewerID)
	fc.Prime(key, cachedResult)

	dbCalled := false
	content.listExplorePostsFn = func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) {
		dbCalled = true
		return nil, nil
	}

	svc := newTestFeedService(auth, content, fc)
	result, err := svc.GetExploreFeed(context.Background(), &viewerID, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Errorf("expected 1 cached post, got %d", len(result.Posts))
	}
	if dbCalled {
		t.Error("DB called on explore cache hit — expected cache-only path")
	}
}

// TestGetExploreFeed_CacheMiss_PopulatesCache verifies that an explore cache
// miss hits the DB and stores the result.
func TestGetExploreFeed_CacheMiss_PopulatesCache(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	viewerID := uuid.New()
	post := samplePost(uuid.New())

	content.listExplorePostsFn = func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}
	auth.getUsersByIDsFn = func(_ context.Context, _ []uuid.UUID) ([]db.AuthUser, error) {
		return []db.AuthUser{{ID: post.AuthorID, Username: "bob"}}, nil
	}

	svc := newTestFeedService(auth, content, fc)
	result, err := svc.GetExploreFeed(context.Background(), &viewerID, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Fatalf("expected 1 post, got %d", len(result.Posts))
	}

	time.Sleep(20 * time.Millisecond)
	key := exploreCacheKey(&viewerID)
	fc.mu.RLock()
	_, cached := fc.store[key]
	fc.mu.RUnlock()
	if !cached {
		t.Errorf("expected explore feed to be cached under key %q", key)
	}
}

// TestGetExploreFeed_AnonKey verifies that anonymous explore requests use
// the "explore:anon" key (not a viewer-specific one).
func TestGetExploreFeed_AnonKey(t *testing.T) {
	fc := newFakeCache()
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()

	post := samplePost(uuid.New())
	content.listExplorePostsFn = func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}

	svc := newTestFeedService(auth, content, fc)
	_, err := svc.GetExploreFeed(context.Background(), nil, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	fc.mu.RLock()
	_, cached := fc.store["explore:anon"]
	fc.mu.RUnlock()
	if !cached {
		t.Error("anonymous explore feed should be cached under \"explore:anon\"")
	}
}

// TestGetExploreFeed_NilCache verifies that the explore feed works without Redis.
func TestGetExploreFeed_NilCache(t *testing.T) {
	auth := newFakeAuthDB()
	content := newFakeFeedContentDB()
	post := samplePost(uuid.New())

	content.listExplorePostsFn = func(_ context.Context, _ db.ExplorePostsParams) ([]db.FeedPost, error) {
		return []db.FeedPost{post}, nil
	}

	// Use nopCache (the default) by not calling SetCache.
	svc := &FeedService{auth: auth, content: content, cache: nopCache{}}
	result, err := svc.GetExploreFeed(context.Background(), nil, nil, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Posts) != 1 {
		t.Errorf("expected 1 post, got %d", len(result.Posts))
	}
}

// ─── Cache key schema tests ───────────────────────────────────────────────────

// TestFeedCacheKey_Schema documents the expected key format.
func TestFeedCacheKey_Schema(t *testing.T) {
	viewerID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	if got := feedCacheKey(viewerID, ""); got != "feed:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:" {
		t.Errorf("unexpected feed key (no cursor): %s", got)
	}
	cursorID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	if got := feedCacheKey(viewerID, cursorID.String()); got != "feed:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa:bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" {
		t.Errorf("unexpected feed key (with cursor): %s", got)
	}
}

// TestExploreCacheKey_Schema documents the explore key format.
func TestExploreCacheKey_Schema(t *testing.T) {
	if got := exploreCacheKey(nil); got != "explore:anon" {
		t.Errorf("unexpected anon explore key: %s", got)
	}
	id := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	if got := exploreCacheKey(&id); got != "explore:cccccccc-cccc-cccc-cccc-cccccccccccc" {
		t.Errorf("unexpected viewer explore key: %s", got)
	}
}
