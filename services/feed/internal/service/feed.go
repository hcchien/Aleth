package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/aleth/feed/internal/db"
)

const defaultLimit = 20
const maxLimit = 50
const defaultFriendReactorLimit = 5
const maxFriendReactorLimit = 20

// FeedResult holds the result of a feed query including pre-loaded author info.
type FeedResult struct {
	Posts      []db.FeedPost
	Authors    map[uuid.UUID]db.AuthUser
	NextCursor *uuid.UUID
	HasMore    bool
}

// FeedService orchestrates personalized and explore feed queries.
type FeedService struct {
	auth    *db.AuthStore
	content *db.ContentStore
}

func NewFeedService(auth *db.AuthStore, content *db.ContentStore) *FeedService {
	return &FeedService{auth: auth, content: content}
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > maxLimit {
		return defaultLimit
	}
	return limit
}

// GetFeed returns a personalized feed of posts from the users that viewerID follows
// (plus viewerID's own posts). Falls back to explore feed if viewerID is nil or
// has no followees.
func (s *FeedService) GetFeed(ctx context.Context, viewerID *uuid.UUID, after *string, limit int) (*FeedResult, error) {
	limit = clampLimit(limit)

	var cursor *uuid.UUID
	if after != nil && *after != "" {
		id, err := uuid.Parse(*after)
		if err == nil {
			cursor = &id
		}
	}

	var followeeIDs []uuid.UUID
	if viewerID != nil {
		ids, err := s.auth.GetFolloweeIDs(ctx, *viewerID)
		if err != nil {
			return nil, err
		}
		// Include viewer's own posts in their feed.
		followeeIDs = append(ids, *viewerID)
	}

	var posts []db.FeedPost
	var err error

	if len(followeeIDs) > 0 {
		posts, err = s.content.ListFeedPosts(ctx, db.FeedPostsParams{
			FolloweeIDs: followeeIDs,
			ViewerID:    viewerID,
			Cursor:      cursor,
			Limit:       limit + 1,
		})
	} else {
		// No follows → fall back to explore feed.
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
func (s *FeedService) GetExploreFeed(ctx context.Context, viewerID *uuid.UUID, after *string, limit int) (*FeedResult, error) {
	limit = clampLimit(limit)

	posts, err := s.content.ListExplorePosts(ctx, db.ExplorePostsParams{
		ViewerID: viewerID,
		Limit:    limit + 1,
	})
	if err != nil {
		return nil, err
	}

	return s.buildResult(ctx, posts, limit)
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
