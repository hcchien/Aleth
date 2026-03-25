package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/aleth/feed/internal/cache"
	"github.com/aleth/feed/internal/db"
)

const (
	feedCacheTTL    = 60 * time.Second  // personalized feed: 60 s
	exploreCacheTTL = 90 * time.Second  // explore feed: 90 s
)

const defaultLimit = 20
const maxLimit = 50
const defaultFriendReactorLimit = 5
const maxFriendReactorLimit = 20

// cacher abstracts the cache layer so a fake can be injected in tests.
type cacher interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, src any, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// nopCache is a no-op cacher used when Redis is not configured.
type nopCache struct{}

func (nopCache) Get(_ context.Context, _ string, _ any) (bool, error)          { return false, nil }
func (nopCache) Set(_ context.Context, _ string, _ any, _ time.Duration) error { return nil }
func (nopCache) Del(_ context.Context, _ ...string) error                       { return nil }

// authDB abstracts the auth store for feed queries (enables testing without a real DB).
type authDB interface {
	GetFolloweeIDs(ctx context.Context, followerID uuid.UUID) ([]uuid.UUID, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.AuthUser, error)
}

// feedContentDB abstracts the content store for feed queries.
type feedContentDB interface {
	GetFollowedPageIDs(ctx context.Context, viewerID uuid.UUID) ([]uuid.UUID, error)
	ListFeedPosts(ctx context.Context, params db.FeedPostsParams) ([]db.FeedPost, error)
	ListExplorePosts(ctx context.Context, params db.ExplorePostsParams) ([]db.FeedPost, error)
	GetPostReactorEmotions(ctx context.Context, postID uuid.UUID, candidateIDs []uuid.UUID, limit int) (map[uuid.UUID]string, error)
}

// FeedResult holds the result of a feed query including pre-loaded author info.
type FeedResult struct {
	Posts      []db.FeedPost
	Authors    map[uuid.UUID]db.AuthUser
	NextCursor *uuid.UUID
	HasMore    bool
}

// FeedService orchestrates personalized and explore feed queries.
type FeedService struct {
	auth    authDB
	content feedContentDB
	cache   cacher
}

func NewFeedService(auth *db.AuthStore, content *db.ContentStore) *FeedService {
	return &FeedService{auth: auth, content: content, cache: nopCache{}}
}

// SetCache wires in an optional Redis client for feed caching.
func (s *FeedService) SetCache(c *cache.Client) {
	if c == nil {
		s.cache = nopCache{}
		return
	}
	s.cache = c
}

// feedCacheKey returns the Redis key for a personalized feed page.
func feedCacheKey(viewerID uuid.UUID, cursor string) string {
	return fmt.Sprintf("feed:%s:%s", viewerID, cursor)
}

// exploreCacheKey returns the Redis key for the explore feed.
func exploreCacheKey(viewerID *uuid.UUID) string {
	if viewerID == nil {
		return "explore:anon"
	}
	return "explore:" + viewerID.String()
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > maxLimit {
		return defaultLimit
	}
	return limit
}

// GetFeed returns a personalized feed of posts from the users that viewerID follows
// (plus viewerID's own posts). Falls back to explore feed if viewerID is nil or
// has no followees. Results are cached in Redis for feedCacheTTL (60 s).
func (s *FeedService) GetFeed(ctx context.Context, viewerID *uuid.UUID, after *string, limit int) (*FeedResult, error) {
	limit = clampLimit(limit)

	// Cache only authenticated users' first page (cursor == "").
	// Subsequent pages and anonymous visitors always hit the DB.
	cursorStr := ""
	if after != nil {
		cursorStr = *after
	}
	if viewerID != nil && cursorStr == "" {
		key := feedCacheKey(*viewerID, cursorStr)
		var cached FeedResult
		if hit, err := s.cache.Get(ctx, key, &cached); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("feed cache get error")
		} else if hit {
			return &cached, nil
		}

		result, err := s.getFeedFromDB(ctx, viewerID, nil, limit)
		if err != nil {
			return nil, err
		}
		go func() {
			if err := s.cache.Set(context.Background(), key, result, feedCacheTTL); err != nil {
				log.Warn().Err(err).Str("key", key).Msg("feed cache set error")
			}
		}()
		return result, nil
	}

	// Parse cursor for non-first pages.
	var cursor *uuid.UUID
	if cursorStr != "" {
		id, err := uuid.Parse(cursorStr)
		if err == nil {
			cursor = &id
		}
	}
	return s.getFeedFromDB(ctx, viewerID, cursor, limit)
}

// getFeedFromDB executes the actual DB queries without any caching.
func (s *FeedService) getFeedFromDB(ctx context.Context, viewerID *uuid.UUID, cursor *uuid.UUID, limit int) (*FeedResult, error) {
	var followeeIDs []uuid.UUID
	var followedPageIDs []uuid.UUID
	if viewerID != nil {
		ids, err := s.auth.GetFolloweeIDs(ctx, *viewerID)
		if err != nil {
			return nil, err
		}
		followeeIDs = append(ids, *viewerID)

		pageIDs, err := s.content.GetFollowedPageIDs(ctx, *viewerID)
		if err != nil {
			return nil, err
		}
		followedPageIDs = pageIDs
	}

	var posts []db.FeedPost
	var err error

	if len(followeeIDs) > 0 || len(followedPageIDs) > 0 {
		posts, err = s.content.ListFeedPosts(ctx, db.FeedPostsParams{
			FolloweeIDs:     followeeIDs,
			FollowedPageIDs: followedPageIDs,
			ViewerID:        viewerID,
			Cursor:          cursor,
			Limit:           limit + 1,
		})
	} else {
		posts, err = s.content.ListExplorePosts(ctx, db.ExplorePostsParams{
			ViewerID: viewerID,
			Limit:    limit + 1,
		})
	}
	if err != nil {
		return nil, err
	}

	return s.buildResult(ctx, posts, limit)
}

// GetExploreFeed returns posts ranked by trust-weighted, time-decayed reach score.
// Results are cached per viewer for exploreCacheTTL (90 s).
func (s *FeedService) GetExploreFeed(ctx context.Context, viewerID *uuid.UUID, after *string, limit int) (*FeedResult, error) {
	limit = clampLimit(limit)

	key := exploreCacheKey(viewerID)
	var cached FeedResult
	if hit, err := s.cache.Get(ctx, key, &cached); err != nil {
		log.Warn().Err(err).Str("key", key).Msg("explore cache get error")
	} else if hit {
		return &cached, nil
	}

	posts, err := s.content.ListExplorePosts(ctx, db.ExplorePostsParams{
		ViewerID: viewerID,
		Limit:    limit + 1,
	})
	if err != nil {
		return nil, err
	}

	result, err := s.buildResult(ctx, posts, limit)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := s.cache.Set(context.Background(), key, result, exploreCacheTTL); err != nil {
			log.Warn().Err(err).Str("key", key).Msg("explore cache set error")
		}
	}()
	return result, nil
}

// GetFriendReactors returns up to limit friends (people viewerID follows) who
// reacted to postID, enriched with user display info and their emotion.
// Returns an empty slice (not an error) when viewerID has no followees or no
// friends reacted.
func (s *FeedService) GetFriendReactors(
	ctx context.Context,
	viewerID uuid.UUID,
	postID uuid.UUID,
	limit int,
) ([]db.PostReactor, error) {
	if limit <= 0 || limit > maxFriendReactorLimit {
		limit = defaultFriendReactorLimit
	}

	// 1. Get the set of people the viewer follows (auth DB).
	followeeIDs, err := s.auth.GetFolloweeIDs(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	if len(followeeIDs) == 0 {
		return nil, nil
	}

	// 2. Find which of those followees reacted to postID (content DB).
	emotionMap, err := s.content.GetPostReactorEmotions(ctx, postID, followeeIDs, limit)
	if err != nil {
		return nil, err
	}
	if len(emotionMap) == 0 {
		return nil, nil
	}

	// 3. Batch-fetch user profiles for the reactor IDs (auth DB).
	reactorIDs := make([]uuid.UUID, 0, len(emotionMap))
	for id := range emotionMap {
		reactorIDs = append(reactorIDs, id)
	}
	users, err := s.auth.GetUsersByIDs(ctx, reactorIDs)
	if err != nil {
		return nil, err
	}

	// 4. Merge user info with emotions.
	out := make([]db.PostReactor, 0, len(users))
	for _, u := range users {
		if emotion, ok := emotionMap[u.ID]; ok {
			out = append(out, db.PostReactor{
				UserID:      u.ID,
				Username:    u.Username,
				DisplayName: u.DisplayName,
				Emotion:     emotion,
			})
		}
	}
	return out, nil
}

// buildResult trims the +1 sentinel, resolves authors, and builds the result.
func (s *FeedService) buildResult(ctx context.Context, posts []db.FeedPost, limit int) (*FeedResult, error) {
	hasMore := len(posts) > limit
	if hasMore {
		posts = posts[:limit]
	}

	// Collect unique author IDs.
	seen := make(map[uuid.UUID]struct{}, len(posts))
	authorIDs := make([]uuid.UUID, 0, len(posts))
	for _, p := range posts {
		if _, ok := seen[p.AuthorID]; !ok {
			seen[p.AuthorID] = struct{}{}
			authorIDs = append(authorIDs, p.AuthorID)
		}
	}

	// Batch fetch author info from auth DB.
	authUsers, err := s.auth.GetUsersByIDs(ctx, authorIDs)
	if err != nil {
		return nil, err
	}
	authors := make(map[uuid.UUID]db.AuthUser, len(authUsers))
	for _, u := range authUsers {
		authors[u.ID] = u
	}

	// Compute cursor: ID of the last post in the page.
	var nextCursor *uuid.UUID
	if hasMore && len(posts) > 0 {
		id := posts[len(posts)-1].ID
		nextCursor = &id
	}

	return &FeedResult{
		Posts:      posts,
		Authors:    authors,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
