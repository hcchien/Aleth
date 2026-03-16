package ap

import (
	"fmt"
	"time"

	"github.com/aleth/federation/internal/client"
)

// BuildOutboxIndex returns the root OrderedCollection for a user's outbox.
// It does not contain items directly — clients follow `first` to get paged items.
func BuildOutboxIndex(domain, username string, totalItems int) map[string]any {
	outboxURL := "https://" + domain + "/@" + username + "/outbox"
	return map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         outboxURL,
		"type":       "OrderedCollection",
		"totalItems": totalItems,
		"first":      outboxURL + "?page=true",
	}
}

// BuildOutboxPage returns an OrderedCollectionPage containing Create(Note) activities.
// nextBefore is the cursor for the next page; nil means this is the last page.
func BuildOutboxPage(domain, username string, posts []client.ContentPost, nextBefore *time.Time) map[string]any {
	outboxURL := "https://" + domain + "/@" + username + "/outbox"
	pageID := outboxURL + "?page=true"

	items := make([]any, 0, len(posts))
	for _, p := range posts {
		items = append(items, BuildCreateActivity(domain, username, p))
	}

	page := map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           pageID,
		"type":         "OrderedCollectionPage",
		"partOf":       outboxURL,
		"orderedItems": items,
	}

	if nextBefore != nil {
		cursor := fmt.Sprintf("%s?page=true&before=%s",
			outboxURL,
			nextBefore.UTC().Format(time.RFC3339),
		)
		page["next"] = cursor
	}

	return page
}
