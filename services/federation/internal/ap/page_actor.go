package ap

import (
	"time"

	"github.com/aleth/federation/internal/client"
)

// BuildPageActor returns the ActivityPub Group JSON-LD object for a fan page.
// AP type "Group" is the closest semantic fit for a fan page / organization.
func BuildPageActor(domain, slug, displayName, pubKeyPEM string) map[string]any {
	actorURL := "https://" + domain + "/p/" + slug
	name := displayName
	if name == "" {
		name = slug
	}
	return map[string]any{
		"@context": []any{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1",
		},
		"id":                actorURL,
		"type":              "Group",
		"preferredUsername": "p." + slug,
		"name":              name,
		"inbox":             actorURL + "/inbox",
		"outbox":            actorURL + "/outbox",
		"followers":         actorURL + "/followers",
		"publicKey": map[string]any{
			"id":           actorURL + "#main-key",
			"owner":        actorURL,
			"publicKeyPem": pubKeyPEM,
		},
	}
}

// BuildPageOutboxIndex returns the root OrderedCollection for a page's outbox.
func BuildPageOutboxIndex(domain, slug string, totalItems int) map[string]any {
	outboxURL := "https://" + domain + "/p/" + slug + "/outbox"
	return map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         outboxURL,
		"type":       "OrderedCollection",
		"totalItems": totalItems,
		"first":      outboxURL + "?page=true",
	}
}

// BuildPageOutboxPage returns an OrderedCollectionPage for page posts.
func BuildPageOutboxPage(domain, slug string, posts []client.ContentPost, nextBefore *time.Time) map[string]any {
	actorURL := "https://" + domain + "/p/" + slug
	outboxURL := actorURL + "/outbox"

	items := make([]map[string]any, 0, len(posts))
	for _, post := range posts {
		items = append(items, BuildCreateActivity(domain, "p."+slug, post))
	}

	page := map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           outboxURL + "?page=true",
		"type":         "OrderedCollectionPage",
		"partOf":       outboxURL,
		"orderedItems": items,
	}

	// Override actor URLs in each activity to use /p/{slug} base
	for i, item := range items {
		item["actor"] = actorURL
		if obj, ok := item["object"].(map[string]any); ok {
			obj["attributedTo"] = actorURL
			obj["cc"] = []string{actorURL + "/followers"}
			item["object"] = obj
		}
		item["cc"] = []string{actorURL + "/followers"}
		items[i] = item
	}
	page["orderedItems"] = items

	if nextBefore != nil {
		page["next"] = outboxURL + "?page=true&before=" + nextBefore.Format(time.RFC3339)
	}
	return page
}
