package ap

import "github.com/google/uuid"

// BuildAccept wraps an incoming Follow activity in an Accept response.
// The Accept is sent back to the follower's inbox so their server records the
// relationship as confirmed.
func BuildAccept(domain, username string, followActivity map[string]any) map[string]any {
	actorURL := "https://" + domain + "/@" + username
	return map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       "https://" + domain + "/activities/" + uuid.New().String(),
		"type":     "Accept",
		"actor":    actorURL,
		"object":   followActivity,
	}
}
