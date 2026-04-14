package reputation

import (
	"fmt"

	"github.com/leith/api/internal/store"
)

// Weights for Trust Tiers when an interaction occurs (like, repost, reply)
var TierInteractionWeight = map[store.TrustTier]float64{
	store.L0_GUEST:     0.0, // L0 interactions have zero mathematical weight
	store.L1_DEVICE:    1.0,
	store.L2_SOCIAL:    10.0,
	store.L3_TEMPORAL:  100.0,
	store.L4_AUTHORITY: 1000.0, // L4 interactions easily suppress bots
}

// Service manages the reputation logic
type Service struct {
	db store.Store
}

func NewService(db store.Store) *Service {
	return &Service{db: db}
}

// CalculateInteractionScore takes a "like" or "repost" and returns the weight
// that should be added to the post's visibility score based on the actor's Tier.
func (s *Service) CalculateInteractionScore(actorDID string) (float64, error) {
	actor, err := s.db.GetUserByDID(actorDID)
	if err != nil || actor == nil {
		return 0, fmt.Errorf("actor not found")
	}

	weight, ok := TierInteractionWeight[actor.TrustTier]
	if !ok {
		return 1.0, nil // Fallback to L1
	}

	return weight, nil
}

// ProcessInteraction adds the weighted score to a post's total visibility.
func (s *Service) ProcessInteraction(actorDID string, postID int64) error {
	scoreToAdd, err := s.CalculateInteractionScore(actorDID)
	if err != nil {
		return err
	}

	// This assumes the DB atomicly updates: UPDATE posts SET visibility_score = visibility_score + ? WHERE id = ?
	return s.db.UpdateVisibilityScore(postID, scoreToAdd)
}

// SlashContent applies the "Slashing" penalty when content is provably false.
// It reverses the weights of interactions + applies a penalty to all who interacted.
// Note: This logic requires a tracking table of `interactions(post_id, actor_did)` to work in production.
func (s *Service) SlashContent(postID int64, authorityDID string) error {
	// 1. Verify authorityDID is actually an L4_AUTHORITY
	authority, err := s.db.GetUserByDID(authorityDID)
	if err != nil || authority == nil || authority.TrustTier != store.L4_AUTHORITY {
		return fmt.Errorf("only L4 authorities can trigger a slash")
	}

	// 2. Heavy negative score to bury the content immediately
	slashPenalty := -999999.0

	err = s.db.UpdateVisibilityScore(postID, slashPenalty)
	if err != nil {
		return fmt.Errorf("failed to slash post: %w", err)
	}

	// 3. TODO: Query all actors who previously "liked/reposted" this post,
	// and proportionally deduct their personal reputation score to discourage
	// blind endorsement of fake news.

	return nil
}

// CalculateFeed returns a list of posts sorted dynamically
// In production, this would be handled mostly via SQL:
// SELECT * FROM posts ORDER BY visibility_score DESC, created_at DESC LIMIT X
func (s *Service) GetPublicFeed(limit, offset int) ([]store.Post, error) {
	return s.db.GetPosts(limit, offset)
}
