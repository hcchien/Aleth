package middleware

import (
	"context"
	"net/http"
	"strconv"

	"github.com/leith/api/internal/store"
)

// Define rate limits per tier (requests per minute)
var tierLimits = map[store.TrustTier]int{
	store.L0_GUEST:     2,  // Extreme severe limit to prevent OAuth bot spam
	store.L1_DEVICE:    10, // Low limit for pure device keys
	store.L2_SOCIAL:    50,
	store.L3_TEMPORAL:  100,
	store.L4_AUTHORITY: 1000, // High limit for verified authorities
}

// RateLimiter returns a middleware that limits requests based on the user's DID Trust Tier.
// In a real MVP, this would use Redis or an in-memory token bucket.
func RateLimiter(db store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow auth bootstrap endpoints before DID/user exists.
			if len(r.URL.Path) >= len("/auth/") && r.URL.Path[:len("/auth/")] == "/auth/" {
				next.ServeHTTP(w, r)
				return
			}

			// Extract DID from context (set by previous Auth middleware)
			did, ok := r.Context().Value("user_did").(string)
			if !ok || did == "" {
				// Fallback to L1 limit for unauthenticated or parsing errors if route requires it
				// Or reject if strict auth is required.
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			user, err := db.GetUserByDID(did)
			if err != nil || user == nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// In a real app, check against a token bucket or Redis here
			limit := tierLimits[user.TrustTier]

			// Mock check: For now, just passes.
			allowed, err := db.CheckRateLimit(did, user.TrustTier)
			if err != nil || !allowed {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Add limit info to headers (optional)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))

			next.ServeHTTP(w, r)
		})
	}
}

// AuthContext middleware to mock extracting DID from a session/JWT
func AuthContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mock: In practice, extract JWT or session cookie, verify signature, get DID
		did := r.Header.Get("X-Mock-DID") // For L1+ Passkeys

		if did == "" {
			// Check for L0 Guest Token
			oauthToken := r.Header.Get("X-Mock-OAuth-Token")
			if oauthToken != "" {
				did = "oauth:google:" + oauthToken // Mock derivation of an L0 DID
			}
		}

		ctx := context.WithValue(r.Context(), "user_did", did)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
