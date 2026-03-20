package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// ─── fake store ───────────────────────────────────────────────────────────────

type call struct {
	op string
	id uuid.UUID
}

type fakeStore struct {
	calls []call
	err   error
}

func (f *fakeStore) IncrPostCommentCount(_ context.Context, id uuid.UUID) error {
	f.calls = append(f.calls, call{"incr_post_comment", id})
	return f.err
}
func (f *fakeStore) IncrArticleCommentCount(_ context.Context, id uuid.UUID) error {
	f.calls = append(f.calls, call{"incr_article_comment", id})
	return f.err
}
func (f *fakeStore) UpdatePostReactionCounts(_ context.Context, id uuid.UUID) error {
	f.calls = append(f.calls, call{"update_post_reactions", id})
	return f.err
}
func (f *fakeStore) IncrPagePostCount(_ context.Context, id uuid.UUID) error {
	f.calls = append(f.calls, call{"incr_page_post", id})
	return f.err
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func makeEnvelope(payload any) []byte {
	raw, _ := json.Marshal(payload)
	env, _ := json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
	}{Payload: raw})
	return env
}

// ─── post.created ─────────────────────────────────────────────────────────────

func TestHandlePostCreated_Reply_IncrementsParentCommentCount(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	parentID := uuid.New()
	parentIDStr := parentID.String()
	data := makeEnvelope(postCreatedPayload{Kind: "reply", ParentID: &parentIDStr})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "incr_post_comment" {
		t.Errorf("op: got %s want incr_post_comment", fs.calls[0].op)
	}
	if fs.calls[0].id != parentID {
		t.Errorf("id: got %s want %s", fs.calls[0].id, parentID)
	}
}

func TestHandlePostCreated_RootPost_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	data := makeEnvelope(postCreatedPayload{Kind: "post"})
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for root post, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_Reshare_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	data := makeEnvelope(postCreatedPayload{Kind: "reshare"})
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for reshare, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_BadEnvelope_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	svc.HandleEvent(context.Background(), "post.created", []byte(`not json`))

	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for bad envelope, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_InvalidParentID_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	bad := "not-a-uuid"
	data := makeEnvelope(postCreatedPayload{Kind: "reply", ParentID: &bad})
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for invalid parent_id, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_DBError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{err: errors.New("db down")}
	svc := New(fs)

	parentIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{Kind: "reply", ParentID: &parentIDStr})

	// Must not panic; error is logged.
	svc.HandleEvent(context.Background(), "post.created", data)
}

// ─── comment.created ──────────────────────────────────────────────────────────

func TestHandleCommentCreated_IncrementsArticleCommentCount(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	articleID := uuid.New()
	data := makeEnvelope(commentCreatedPayload{ArticleID: articleID.String()})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "incr_article_comment" {
		t.Errorf("op: got %s want incr_article_comment", fs.calls[0].op)
	}
	if fs.calls[0].id != articleID {
		t.Errorf("id: got %s want %s", fs.calls[0].id, articleID)
	}
}

func TestHandleCommentCreated_BadEnvelope_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)
	svc.HandleEvent(context.Background(), "comment.created", []byte(`not json`))
	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls, got %d", len(fs.calls))
	}
}

func TestHandleCommentCreated_InvalidArticleID_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)
	data := makeEnvelope(commentCreatedPayload{ArticleID: "not-a-uuid"})
	svc.HandleEvent(context.Background(), "comment.created", data)
	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for invalid article_id, got %d", len(fs.calls))
	}
}

// ─── reaction.upserted ────────────────────────────────────────────────────────

func TestHandleReactionUpserted_UpdatesReactionCounts(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	postID := uuid.New()
	data := makeEnvelope(reactionPayload{PostID: postID.String()})

	svc.HandleEvent(context.Background(), "reaction.upserted", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "update_post_reactions" {
		t.Errorf("op: got %s want update_post_reactions", fs.calls[0].op)
	}
	if fs.calls[0].id != postID {
		t.Errorf("id: got %s want %s", fs.calls[0].id, postID)
	}
}

func TestHandleReactionUpserted_InvalidPostID_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)
	data := makeEnvelope(reactionPayload{PostID: "not-a-uuid"})
	svc.HandleEvent(context.Background(), "reaction.upserted", data)
	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for invalid post_id, got %d", len(fs.calls))
	}
}

// ─── reaction.removed ─────────────────────────────────────────────────────────

func TestHandleReactionRemoved_UpdatesReactionCounts(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	postID := uuid.New()
	data := makeEnvelope(reactionPayload{PostID: postID.String()})

	svc.HandleEvent(context.Background(), "reaction.removed", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "update_post_reactions" {
		t.Errorf("op: got %s want update_post_reactions", fs.calls[0].op)
	}
	if fs.calls[0].id != postID {
		t.Errorf("id: got %s want %s", fs.calls[0].id, postID)
	}
}

func TestHandleReactionRemoved_InvalidPostID_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)
	data := makeEnvelope(reactionPayload{PostID: "not-a-uuid"})
	svc.HandleEvent(context.Background(), "reaction.removed", data)
	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for invalid post_id, got %d", len(fs.calls))
	}
}

// ─── reaction idempotency ─────────────────────────────────────────────────────

func TestHandleReactionEvent_DBError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{err: errors.New("db down")}
	svc := New(fs)

	data := makeEnvelope(reactionPayload{PostID: uuid.New().String()})
	// Must not panic; error is logged.
	svc.HandleEvent(context.Background(), "reaction.upserted", data)
	svc.HandleEvent(context.Background(), "reaction.removed", data)
}

// ─── unknown event type ───────────────────────────────────────────────────────

func TestHandleUnknownEventType_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)
	svc.HandleEvent(context.Background(), "something.unknown", []byte(`{}`))
	if len(fs.calls) != 0 {
		t.Errorf("expected no DB calls for unknown event, got %d", len(fs.calls))
	}
}

// ─── page post counter ────────────────────────────────────────────────────────

func TestHandlePostCreated_PagePost_IncrementsPagePostCount(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	pageID := uuid.New()
	pageIDStr := pageID.String()
	data := makeEnvelope(postCreatedPayload{Kind: "post", PageID: &pageIDStr})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "incr_page_post" {
		t.Errorf("op: got %q want %q", fs.calls[0].op, "incr_page_post")
	}
	if fs.calls[0].id != pageID {
		t.Errorf("page_id: got %s want %s", fs.calls[0].id, pageID)
	}
}

func TestHandlePostCreated_NoteOnPage_IncrementsPagePostCount(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	pageID := uuid.New()
	pageIDStr := pageID.String()
	data := makeEnvelope(postCreatedPayload{Kind: "note", PageID: &pageIDStr})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 1 {
		t.Fatalf("expected 1 DB call, got %d", len(fs.calls))
	}
	if fs.calls[0].op != "incr_page_post" {
		t.Errorf("op: got %q want %q", fs.calls[0].op, "incr_page_post")
	}
}

func TestHandlePostCreated_ReplyOnPage_DoesNotIncrementPagePostCount(t *testing.T) {
	// Replies increment the parent's comment count, not the page post count.
	fs := &fakeStore{}
	svc := New(fs)

	parentID := uuid.New().String()
	pageID := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{Kind: "reply", ParentID: &parentID, PageID: &pageID})

	svc.HandleEvent(context.Background(), "post.created", data)

	// Should only increment comment count, not page post count.
	for _, c := range fs.calls {
		if c.op == "incr_page_post" {
			t.Error("incr_page_post must NOT be called for a reply")
		}
	}
}

func TestHandlePostCreated_RootPost_NoPageID_NoPagePostCount(t *testing.T) {
	// Root post without PageID → neither comment count nor page post count.
	fs := &fakeStore{}
	svc := New(fs)

	data := makeEnvelope(postCreatedPayload{Kind: "post", PageID: nil})
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 0 {
		t.Errorf("expected 0 DB calls, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_InvalidPageID_NoDBCall(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs)

	bad := "not-a-uuid"
	data := makeEnvelope(postCreatedPayload{Kind: "post", PageID: &bad})

	// Must not panic; error is returned and swallowed by HandleEvent.
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.calls) != 0 {
		t.Errorf("expected 0 DB calls for invalid page_id, got %d", len(fs.calls))
	}
}

func TestHandlePostCreated_PageDBError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{err: errors.New("db down")}
	svc := New(fs)

	pageIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{Kind: "post", PageID: &pageIDStr})

	// Must not panic.
	svc.HandleEvent(context.Background(), "post.created", data)
}

func TestHandlePostCreated_ReplyAndPageSet_OnlyIncrementsCommentCount(t *testing.T) {
	// Edge case: reply that also has PageID — only comment counter fires.
	fs := &fakeStore{}
	svc := New(fs)

	parentIDStr := uuid.New().String()
	pageIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{Kind: "reply", ParentID: &parentIDStr, PageID: &pageIDStr})

	svc.HandleEvent(context.Background(), "post.created", data)

	for _, c := range fs.calls {
		if c.op == "incr_page_post" {
			t.Error("incr_page_post must NOT be called for reply, even when PageID is set")
		}
	}
	found := false
	for _, c := range fs.calls {
		if c.op == "incr_post_comment" {
			found = true
		}
	}
	if !found {
		t.Error("incr_post_comment should have been called for reply")
	}
}
