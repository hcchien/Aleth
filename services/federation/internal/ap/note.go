package ap

import "github.com/aleth/federation/internal/client"

const apPublic = "https://www.w3.org/ns/activitystreams#Public"

// BuildCreateActivity wraps a content post in a Create{Note} ActivityPub activity.
// Post ID: https://{domain}/posts/{post.ID}
// Activity ID: https://{domain}/activities/{post.ID}/create
func BuildCreateActivity(domain, username string, post client.ContentPost) map[string]any {
	actorURL := "https://" + domain + "/@" + username
	postURL := "https://" + domain + "/posts/" + post.ID
	activityURL := "https://" + domain + "/activities/" + post.ID + "/create"
	followersURL := actorURL + "/followers"
	published := post.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")

	note := map[string]any{
		"id":           postURL,
		"type":         "Note",
		"attributedTo": actorURL,
		"content":      post.Content,
		"published":    published,
		"to":           []string{apPublic},
		"cc":           []string{followersURL},
	}

	return map[string]any{
		"id":        activityURL,
		"type":      "Create",
		"actor":     actorURL,
		"published": published,
		"to":        []string{apPublic},
		"cc":        []string{followersURL},
		"object":    note,
	}
}
