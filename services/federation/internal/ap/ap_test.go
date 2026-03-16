package ap

import (
	"strings"
	"testing"
	"time"

	"github.com/aleth/federation/internal/client"
)

func TestBuildWebFinger(t *testing.T) {
	wf := BuildWebFinger("example.com", "alice")
	if wf.Subject != "acct:alice@example.com" {
		t.Fatalf("unexpected subject: %v", wf.Subject)
	}
	if len(wf.Links) != 1 || wf.Links[0].Rel != "self" {
		t.Fatalf("unexpected links: %v", wf.Links)
	}
	if wf.Links[0].Href != "https://example.com/@alice" {
		t.Fatalf("unexpected href: %v", wf.Links[0].Href)
	}
}

func TestBuildActor(t *testing.T) {
	actor := BuildActor("example.com", "alice", "Alice A", "did:aleth:123", "-----BEGIN PUBLIC KEY-----\n...")
	if actor["type"] != "Person" {
		t.Fatalf("unexpected type: %v", actor["type"])
	}
	if actor["preferredUsername"] != "alice" {
		t.Fatalf("unexpected preferredUsername")
	}
	pk, _ := actor["publicKey"].(map[string]any)
	if pk == nil || pk["publicKeyPem"] != "-----BEGIN PUBLIC KEY-----\n..." {
		t.Fatalf("unexpected publicKey: %v", pk)
	}
	aka, _ := actor["alsoKnownAs"].([]string)
	if len(aka) != 1 || aka[0] != "did:aleth:123" {
		t.Fatalf("unexpected alsoKnownAs: %v", aka)
	}
	// displayName fallback: empty display name uses username
	actor2 := BuildActor("example.com", "bob", "", "did:aleth:456", "pem")
	if actor2["name"] != "bob" {
		t.Fatalf("expected username as name fallback, got: %v", actor2["name"])
	}
}

func TestBuildOutboxIndex(t *testing.T) {
	idx := BuildOutboxIndex("example.com", "alice", 42)
	if idx["type"] != "OrderedCollection" {
		t.Fatalf("unexpected type: %v", idx["type"])
	}
	if idx["totalItems"] != 42 {
		t.Fatalf("unexpected totalItems: %v", idx["totalItems"])
	}
	first, _ := idx["first"].(string)
	if !strings.Contains(first, "page=true") {
		t.Fatalf("first link should contain page=true: %v", first)
	}
}

func TestBuildOutboxPage(t *testing.T) {
	now := time.Now()
	posts := []client.ContentPost{
		{ID: "p1", Content: "hello", CreatedAt: now},
		{ID: "p2", Content: "world", CreatedAt: now.Add(-time.Hour)},
	}
	before := now.Add(-2 * time.Hour)
	page := BuildOutboxPage("example.com", "alice", posts, &before)
	if page["type"] != "OrderedCollectionPage" {
		t.Fatalf("unexpected type: %v", page["type"])
	}
	items, _ := page["orderedItems"].([]any)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	item0, _ := items[0].(map[string]any)
	if item0 == nil || item0["type"] != "Create" {
		t.Fatalf("unexpected item type: %v", item0)
	}
	next, _ := page["next"].(string)
	if !strings.Contains(next, "before=") {
		t.Fatalf("expected next cursor in page: %v", next)
	}

	// No next cursor when nextBefore is nil
	pageNoNext := BuildOutboxPage("example.com", "alice", posts, nil)
	if _, ok := pageNoNext["next"]; ok {
		t.Fatalf("expected no next when nextBefore is nil")
	}
}

func TestBuildCreateActivity(t *testing.T) {
	post := client.ContentPost{ID: "abc123", Content: "test post", CreatedAt: time.Now()}
	act := BuildCreateActivity("example.com", "alice", post)
	if act["type"] != "Create" {
		t.Fatalf("unexpected type: %v", act["type"])
	}
	if act["actor"] != "https://example.com/@alice" {
		t.Fatalf("unexpected actor: %v", act["actor"])
	}
	obj, _ := act["object"].(map[string]any)
	if obj == nil || obj["type"] != "Note" {
		t.Fatalf("unexpected object: %v", obj)
	}
	if obj["content"] != "test post" {
		t.Fatalf("unexpected content: %v", obj["content"])
	}
}

func TestBuildPageActor(t *testing.T) {
	actor := BuildPageActor("example.com", "my-band", "My Band", "-----BEGIN PUBLIC KEY-----\n...")
	if actor["type"] != "Group" {
		t.Fatalf("expected type Group, got %v", actor["type"])
	}
	if actor["preferredUsername"] != "p.my-band" {
		t.Fatalf("unexpected preferredUsername: %v", actor["preferredUsername"])
	}
	if actor["id"] != "https://example.com/p/my-band" {
		t.Fatalf("unexpected id: %v", actor["id"])
	}
	inbox, _ := actor["inbox"].(string)
	if !strings.Contains(inbox, "/p/my-band/inbox") {
		t.Fatalf("unexpected inbox: %v", inbox)
	}
	pk, _ := actor["publicKey"].(map[string]any)
	if pk == nil || pk["publicKeyPem"] != "-----BEGIN PUBLIC KEY-----\n..." {
		t.Fatalf("unexpected publicKey: %v", pk)
	}
	// displayName fallback
	actor2 := BuildPageActor("example.com", "empty-name", "", "pem")
	if actor2["name"] != "empty-name" {
		t.Fatalf("expected slug as name fallback, got: %v", actor2["name"])
	}
}

func TestBuildPageOutboxIndex(t *testing.T) {
	idx := BuildPageOutboxIndex("example.com", "my-band", 10)
	if idx["type"] != "OrderedCollection" {
		t.Fatalf("unexpected type: %v", idx["type"])
	}
	if idx["totalItems"] != 10 {
		t.Fatalf("unexpected totalItems: %v", idx["totalItems"])
	}
	first, _ := idx["first"].(string)
	if !strings.Contains(first, "/p/my-band/outbox") {
		t.Fatalf("first link should reference /p/my-band/outbox: %v", first)
	}
}

func TestBuildPageOutboxPage(t *testing.T) {
	now := time.Now()
	posts := []client.ContentPost{
		{ID: "p1", Content: "hello from page", CreatedAt: now},
	}
	before := now.Add(-time.Hour)
	page := BuildPageOutboxPage("example.com", "my-band", posts, &before)
	if page["type"] != "OrderedCollectionPage" {
		t.Fatalf("unexpected type: %v", page["type"])
	}
	items, _ := page["orderedItems"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	actor, _ := items[0]["actor"].(string)
	if !strings.Contains(actor, "/p/my-band") {
		t.Fatalf("activity actor should contain /p/my-band: %v", actor)
	}
	next, _ := page["next"].(string)
	if !strings.Contains(next, "before=") {
		t.Fatalf("expected next cursor: %v", next)
	}
}

func TestBuildAccept(t *testing.T) {
	follow := map[string]any{
		"id":     "https://remote.example/follows/1",
		"type":   "Follow",
		"actor":  "https://remote.example/users/bob",
		"object": "https://example.com/@alice",
	}
	accept := BuildAccept("example.com", "alice", follow)
	if accept["type"] != "Accept" {
		t.Fatalf("unexpected type: %v", accept["type"])
	}
	if accept["actor"] != "https://example.com/@alice" {
		t.Fatalf("unexpected actor: %v", accept["actor"])
	}
	obj, _ := accept["object"].(map[string]any)
	if obj == nil {
		t.Fatalf("expected object to be the follow activity")
	}
	// ID should be a fresh UUID-based URL
	id, _ := accept["id"].(string)
	if !strings.HasPrefix(id, "https://example.com/") {
		t.Fatalf("unexpected accept id: %v", id)
	}
}
