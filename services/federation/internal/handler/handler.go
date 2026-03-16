// Package handler contains the HTTP handlers for the federation service.
package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/aleth/federation/internal/ap"
	"github.com/aleth/federation/internal/client"
	"github.com/aleth/federation/internal/config"
	"github.com/aleth/federation/internal/db"
	"github.com/aleth/federation/internal/httpsig"
	"github.com/aleth/federation/internal/keys"
)

// Handler holds all dependencies for the federation HTTP handlers.
type Handler struct {
	cfg     config.Config
	db      *db.Pool
	auth    *client.AuthClient
	content *client.ContentClient
}

// New creates a fully-wired Handler.
func New(cfg config.Config, pool *db.Pool, auth *client.AuthClient, content *client.ContentClient) *Handler {
	return &Handler{cfg: cfg, db: pool, auth: auth, content: content}
}

// ─── WebFinger ────────────────────────────────────────────────────────────────

// ServeWebFinger handles GET /.well-known/webfinger?resource=acct:{user}@{domain}.
func (h *Handler) ServeWebFinger(w http.ResponseWriter, r *http.Request) {
	resource := r.URL.Query().Get("resource")
	if resource == "" {
		http.Error(w, "resource query parameter required", http.StatusBadRequest)
		return
	}

	// Expect "acct:{username}@{domain}"
	if !strings.HasPrefix(resource, "acct:") {
		http.Error(w, "unsupported resource scheme", http.StatusBadRequest)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(resource, "acct:"), "@", 2)
	if len(parts) != 2 {
		http.Error(w, "malformed acct: resource", http.StatusBadRequest)
		return
	}
	username, domain := parts[0], parts[1]

	if domain != h.cfg.Domain {
		http.Error(w, "domain not found", http.StatusNotFound)
		return
	}

	// Page actor: acct:p.{slug}@{domain}
	if strings.HasPrefix(username, "p.") {
		h.servePageWebFinger(w, r, strings.TrimPrefix(username, "p."))
		return
	}

	// Verify the user exists and has AP enabled.
	user, err := h.auth.GetUserByUsername(r.Context(), username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("webfinger: auth lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil || !user.APEnabled {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	resp := ap.BuildWebFinger(h.cfg.Domain, username)
	w.Header().Set("Content-Type", "application/jrd+json")
	json.NewEncoder(w).Encode(resp)
}

// ─── Actor ────────────────────────────────────────────────────────────────────

// ServeActor handles GET /@{username}.
// Returns the ActivityPub Person JSON-LD when Accept: application/activity+json,
// otherwise redirects to the web profile.
func (h *Handler) ServeActor(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "application/activity+json") &&
		!strings.Contains(accept, "application/ld+json") {
		http.Redirect(w, r, "https://"+h.cfg.Domain+"/"+username, http.StatusFound)
		return
	}

	user, err := h.auth.GetUserByUsername(r.Context(), username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("actor: auth lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil || !user.APEnabled {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Lazy key generation — idempotent via INSERT ON CONFLICT DO NOTHING.
	actorKey, err := h.db.GetActorKeyByUsername(r.Context(), username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("actor: key lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if actorKey == nil {
		userUUID, err := uuid.Parse(user.ID)
		if err != nil {
			log.Error().Err(err).Str("id", user.ID).Msg("actor: invalid user UUID")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		pubPEM, privEnc, err := keys.GenerateActorKeyPair(h.cfg.PlatformKeySecret)
		if err != nil {
			log.Error().Err(err).Msg("actor: key generation failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := h.db.EnsureActorKey(r.Context(), userUUID, username, pubPEM, privEnc); err != nil {
			log.Error().Err(err).Msg("actor: key store failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// Re-fetch to get the authoritative stored record.
		actorKey, err = h.db.GetActorKeyByUsername(r.Context(), username)
		if err != nil || actorKey == nil {
			log.Error().Err(err).Msg("actor: key re-fetch failed")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	displayName := ""
	if user.DisplayName != nil {
		displayName = *user.DisplayName
	}

	actor := ap.BuildActor(h.cfg.Domain, username, displayName, user.DID, actorKey.PublicKeyPem)
	w.Header().Set("Content-Type", "application/activity+json")
	json.NewEncoder(w).Encode(actor)
}

// ─── Outbox ───────────────────────────────────────────────────────────────────

// ServeOutbox handles GET /@{username}/outbox and /@{username}/outbox?page=true.
func (h *Handler) ServeOutbox(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	user, err := h.auth.GetUserByUsername(r.Context(), username)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("outbox: auth lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if user == nil || !user.APEnabled {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json")

	if r.URL.Query().Get("page") != "true" {
		// Return the index OrderedCollection.
		// We don't have a fast totalItems count here; use 0 to indicate unknown.
		idx := ap.BuildOutboxIndex(h.cfg.Domain, username, 0)
		json.NewEncoder(w).Encode(idx)
		return
	}

	// Parse optional cursor.
	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = &t
		}
	}

	const pageSize = 20
	posts, err := h.content.GetPublicPosts(r.Context(), user.ID, pageSize, before)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("outbox: content lookup failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// If we got a full page, set the next-page cursor to the oldest item's timestamp.
	var nextBefore *time.Time
	if len(posts) == pageSize {
		t := posts[len(posts)-1].CreatedAt
		nextBefore = &t
	}

	page := ap.BuildOutboxPage(h.cfg.Domain, username, posts, nextBefore)
	json.NewEncoder(w).Encode(page)
}

// ─── Inbox ────────────────────────────────────────────────────────────────────

// ServeInbox handles POST /@{username}/inbox.
// Currently processes Follow and Undo(Follow) activities. All other types are
// acknowledged (202) and discarded.
func (h *Handler) ServeInbox(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")

	// Guard: local user must exist and have AP enabled.
	user, err := h.auth.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil || !user.APEnabled {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var activity map[string]any
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	activityType, _ := activity["type"].(string)

	switch activityType {
	case "Follow":
		h.handleFollow(w, r, username, activity)
	case "Undo":
		inner, _ := activity["object"].(map[string]any)
		if inner != nil {
			if t, _ := inner["type"].(string); t == "Follow" {
				h.handleUndo(w, r, username, inner)
				return
			}
		}
		w.WriteHeader(http.StatusAccepted)
	default:
		// Unhandled activity types are silently accepted.
		w.WriteHeader(http.StatusAccepted)
	}
}

// handleFollow records the new remote follower and sends an Accept back.
func (h *Handler) handleFollow(w http.ResponseWriter, r *http.Request, username string, activity map[string]any) {
	actorURL, _ := activity["actor"].(string)
	if actorURL == "" {
		http.Error(w, "missing actor", http.StatusBadRequest)
		return
	}

	// Fetch the remote actor to get their inbox URL.
	inboxURL, err := fetchRemoteInbox(r.Context(), actorURL)
	if err != nil {
		log.Error().Err(err).Str("actor", actorURL).Msg("inbox: fetch remote actor failed")
		http.Error(w, "could not resolve remote actor", http.StatusBadGateway)
		return
	}

	if err := h.db.AddRemoteFollower(r.Context(), username, actorURL, inboxURL); err != nil {
		log.Error().Err(err).Msg("inbox: add remote follower failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Send Accept(Follow) back asynchronously so we don't block the response.
	accept := ap.BuildAccept(h.cfg.Domain, username, activity)
	go h.deliverActivity(username, inboxURL, accept)

	w.WriteHeader(http.StatusAccepted)
}

// handleUndo removes the remote follower on Undo(Follow).
func (h *Handler) handleUndo(w http.ResponseWriter, r *http.Request, username string, followActivity map[string]any) {
	actorURL, _ := followActivity["actor"].(string)
	if actorURL == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err := h.db.RemoveRemoteFollower(r.Context(), username, actorURL); err != nil {
		log.Error().Err(err).Msg("inbox: remove remote follower failed")
	}
	w.WriteHeader(http.StatusAccepted)
}

// ─── Internal: post-created notification ─────────────────────────────────────

// NotifyPostCreated handles POST /internal/post-created.
// The content service calls this after a top-level post is published.
// The handler fans out Create(Note) activities to all remote followers.
func (h *Handler) NotifyPostCreated(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorUsername string `json:"authorUsername"`
		PostID         string `json:"postId"`
		Content        string `json:"content"`
		CreatedAt      string `json:"createdAt"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	followers, err := h.db.ListRemoteFollowers(r.Context(), req.AuthorUsername)
	if err != nil {
		log.Error().Err(err).Str("username", req.AuthorUsername).Msg("notify: list followers failed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if len(followers) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	t, _ := time.Parse(time.RFC3339, req.CreatedAt)
	post := client.ContentPost{ID: req.PostID, Content: req.Content, CreatedAt: t}
	activity := ap.BuildCreateActivity(h.cfg.Domain, req.AuthorUsername, post)

	for _, f := range followers {
		inbox := f.InboxURL
		act := activity // capture
		if err := h.db.EnqueueDelivery(r.Context(), req.AuthorUsername, inbox, act); err != nil {
			log.Error().Err(err).Str("inbox", inbox).Msg("notify: enqueue delivery failed")
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// ─── Delivery helpers ─────────────────────────────────────────────────────────

// deliverActivity signs and POSTs a single activity to targetInbox.
// Used for immediate fire-and-forget responses (e.g. Accept(Follow)).
func (h *Handler) deliverActivity(localUsername, targetInbox string, activity map[string]any) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := h.sendSignedActivity(ctx, localUsername, targetInbox, activity); err != nil {
		log.Error().Err(err).Str("inbox", targetInbox).Msg("deliver: send failed")
	}
}

// sendSignedActivity marshals, signs, and POSTs an activity to targetInbox.
func (h *Handler) sendSignedActivity(ctx context.Context, localUsername, targetInbox string, activity map[string]any) error {
	body, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}

	// Build Digest header (SHA-256 of body, base64).
	sum := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetInbox, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("Digest", digest)

	// Look up actor key.
	actorKey, err := h.db.GetActorKeyByUsername(ctx, localUsername)
	if err != nil || actorKey == nil {
		return fmt.Errorf("no actor key for %s", localUsername)
	}
	privKey, err := keys.DecryptPrivateKey(actorKey.PrivateKeyEnc, h.cfg.PlatformKeySecret)
	if err != nil {
		return fmt.Errorf("decrypt private key: %w", err)
	}

	var keyID string
	if strings.HasPrefix(localUsername, "p:") {
		slug := strings.TrimPrefix(localUsername, "p:")
		keyID = "https://" + h.cfg.Domain + "/p/" + slug + "#main-key"
	} else {
		keyID = "https://" + h.cfg.Domain + "/@" + localUsername + "#main-key"
	}
	if err := httpsig.SignRequest(req, keyID, privKey); err != nil {
		return fmt.Errorf("sign request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("post to inbox: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("inbox returned %d", resp.StatusCode)
	}
	return nil
}

// ─── Page actor handlers ──────────────────────────────────────────────────────

func (h *Handler) servePageWebFinger(w http.ResponseWriter, r *http.Request, slug string) {
	page, err := h.content.GetPageInfo(r.Context(), slug)
	if err != nil || page == nil || !page.APEnabled {
		http.NotFound(w, r)
		return
	}
	actorURL := "https://" + h.cfg.Domain + "/p/" + slug
	resp := map[string]any{
		"subject": "acct:p." + slug + "@" + h.cfg.Domain,
		"links": []map[string]string{{
			"rel":  "self",
			"type": "application/activity+json",
			"href": actorURL,
		}},
	}
	w.Header().Set("Content-Type", "application/jrd+json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ServePageActor(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	// Must be AP accept header to serve JSON-LD
	accept := r.Header.Get("Accept")
	if !strings.Contains(accept, "application/activity+json") &&
		!strings.Contains(accept, "application/ld+json") {
		http.Redirect(w, r, "https://"+h.cfg.Domain+"/p/"+slug, http.StatusFound)
		return
	}

	page, err := h.content.GetPageInfo(r.Context(), slug)
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("get page info")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if page == nil || !page.APEnabled {
		http.NotFound(w, r)
		return
	}

	// Synthetic stable UUID for actor_keys table
	pageUUID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("p:"+slug))
	localUsername := "p:" + slug

	actorKey, err := h.db.GetActorKeyByUsername(r.Context(), localUsername)
	if err != nil {
		log.Error().Err(err).Msg("get page actor key")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if actorKey == nil {
		pubPEM, privEnc, err := keys.GenerateActorKeyPair(h.cfg.PlatformKeySecret)
		if err != nil {
			log.Error().Err(err).Msg("generate page actor key")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := h.db.EnsureActorKey(r.Context(), pageUUID, localUsername, pubPEM, privEnc); err != nil {
			log.Error().Err(err).Msg("store page actor key")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		actorKey, err = h.db.GetActorKeyByUsername(r.Context(), localUsername)
		if err != nil || actorKey == nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	actor := ap.BuildPageActor(h.cfg.Domain, slug, page.Name, actorKey.PublicKeyPem)
	w.Header().Set("Content-Type", "application/activity+json")
	json.NewEncoder(w).Encode(actor)
}

func (h *Handler) ServePageOutbox(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	page, err := h.content.GetPageInfo(r.Context(), slug)
	if err != nil || page == nil || !page.APEnabled {
		http.NotFound(w, r)
		return
	}

	if r.URL.Query().Get("page") != "true" {
		w.Header().Set("Content-Type", "application/activity+json")
		json.NewEncoder(w).Encode(ap.BuildPageOutboxIndex(h.cfg.Domain, slug, 0))
		return
	}

	var before *time.Time
	if b := r.URL.Query().Get("before"); b != "" {
		if t, err := time.Parse(time.RFC3339, b); err == nil {
			before = &t
		}
	}

	posts, err := h.content.GetPageFeed(r.Context(), slug, 20, before)
	if err != nil {
		log.Error().Err(err).Str("slug", slug).Msg("get page feed")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var nextBefore *time.Time
	if len(posts) == 20 {
		t := posts[len(posts)-1].CreatedAt
		nextBefore = &t
	}

	w.Header().Set("Content-Type", "application/activity+json")
	json.NewEncoder(w).Encode(ap.BuildPageOutboxPage(h.cfg.Domain, slug, posts, nextBefore))
}

func (h *Handler) ServePageInbox(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	localUsername := "p:" + slug

	page, err := h.content.GetPageInfo(r.Context(), slug)
	if err != nil || page == nil || !page.APEnabled {
		http.NotFound(w, r)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var activity map[string]any
	if err := json.Unmarshal(body, &activity); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	actType, _ := activity["type"].(string)
	switch actType {
	case "Follow":
		h.handlePageFollow(w, r, localUsername, activity)
	case "Undo":
		obj, _ := activity["object"].(map[string]any)
		if obj == nil {
			http.Error(w, "missing object", http.StatusBadRequest)
			return
		}
		if t, _ := obj["type"].(string); t == "Follow" {
			h.handlePageUnfollow(w, r, localUsername, obj)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
	default:
		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) handlePageFollow(w http.ResponseWriter, r *http.Request, localUsername string, activity map[string]any) {
	actorURL, _ := activity["actor"].(string)
	if actorURL == "" {
		http.Error(w, "missing actor", http.StatusBadRequest)
		return
	}

	inboxURL, err := fetchRemoteInbox(r.Context(), actorURL)
	if err != nil || inboxURL == "" {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if err := h.db.AddRemoteFollower(r.Context(), localUsername, actorURL, inboxURL); err != nil {
		log.Error().Err(err).Msg("store page remote follower")
	}

	slug := strings.TrimPrefix(localUsername, "p:")
	accept := ap.BuildAccept(h.cfg.Domain, localUsername, activity)
	// Override the actor URL in Accept to use page URL
	accept["actor"] = "https://" + h.cfg.Domain + "/p/" + slug

	if err := h.sendSignedActivity(r.Context(), localUsername, inboxURL, accept); err != nil {
		log.Error().Err(err).Str("inbox", inboxURL).Msg("send page Accept")
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) handlePageUnfollow(w http.ResponseWriter, r *http.Request, localUsername string, followActivity map[string]any) {
	actorURL, _ := followActivity["actor"].(string)
	if actorURL != "" {
		if err := h.db.RemoveRemoteFollower(r.Context(), localUsername, actorURL); err != nil {
			log.Error().Err(err).Msg("remove page remote follower")
		}
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) NotifyPagePostCreated(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PageSlug  string    `json:"pageSlug"`
		PostID    string    `json:"postId"`
		Content   string    `json:"content"`
		CreatedAt time.Time `json:"createdAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	localUsername := "p:" + req.PageSlug
	post := client.ContentPost{
		ID:        req.PostID,
		Content:   req.Content,
		CreatedAt: req.CreatedAt,
	}

	slug := req.PageSlug
	actorURL := "https://" + h.cfg.Domain + "/p/" + slug
	activity := ap.BuildCreateActivity(h.cfg.Domain, "p."+slug, post)
	// Override actor URL to use /p/{slug}
	activity["actor"] = actorURL
	if obj, ok := activity["object"].(map[string]any); ok {
		obj["attributedTo"] = actorURL
		obj["cc"] = []string{actorURL + "/followers"}
		activity["object"] = obj
	}
	activity["cc"] = []string{actorURL + "/followers"}

	followers, err := h.db.ListRemoteFollowers(r.Context(), localUsername)
	if err != nil {
		log.Error().Err(err).Str("page", req.PageSlug).Msg("list remote page followers")
		w.WriteHeader(http.StatusOK)
		return
	}

	for _, f := range followers {
		if err := h.db.EnqueueDelivery(r.Context(), localUsername, f.InboxURL, activity); err != nil {
			log.Error().Err(err).Str("inbox", f.InboxURL).Msg("enqueue page delivery")
		}
	}

	w.WriteHeader(http.StatusOK)
}

// fetchRemoteInbox retrieves the inbox URL from a remote AP actor document.
func fetchRemoteInbox(ctx context.Context, actorURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, actorURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/activity+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var actor map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&actor); err != nil {
		return "", fmt.Errorf("decode remote actor: %w", err)
	}

	inbox, _ := actor["inbox"].(string)
	if inbox == "" {
		return "", fmt.Errorf("remote actor %s has no inbox", actorURL)
	}
	return inbox, nil
}
