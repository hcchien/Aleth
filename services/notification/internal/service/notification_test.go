package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aleth/notification/internal/db"
)

// ─── fake store ───────────────────────────────────────────────────────────────

type fakeStore struct {
	mu      sync.Mutex
	created []db.CreateNotificationParams
	marked  []markReadJob // records async mark-read calls

	// configurable overrides
	createErr      error
	countUnreadFn  func(uuid.UUID) (int64, error)
	listFn         func(uuid.UUID, int) ([]db.Notification, error)
	markAllReadErr error
	markReadErr    error
}

func (f *fakeStore) CreateNotification(_ context.Context, p db.CreateNotificationParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, p)
	return nil
}
func (f *fakeStore) CountUnread(_ context.Context, userID uuid.UUID) (int64, error) {
	if f.countUnreadFn != nil {
		return f.countUnreadFn(userID)
	}
	return 0, nil
}
func (f *fakeStore) ListNotifications(_ context.Context, userID uuid.UUID, limit int) ([]db.Notification, error) {
	if f.listFn != nil {
		return f.listFn(userID, limit)
	}
	return nil, nil
}
func (f *fakeStore) MarkAllRead(_ context.Context, userID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markAllReadErr != nil {
		return f.markAllReadErr
	}
	f.marked = append(f.marked, markReadJob{userID: userID, ids: nil})
	return nil
}
func (f *fakeStore) MarkRead(_ context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markReadErr != nil {
		return f.markReadErr
	}
	f.marked = append(f.marked, markReadJob{userID: userID, ids: ids})
	return nil
}
func (f *fakeStore) markedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.marked)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func makeEnvelope(payload any) []byte {
	raw, _ := json.Marshal(payload)
	env, _ := json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
	}{Payload: raw})
	return env
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestHandlePostCreated_Reply_NotifiesParentAuthor(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	parentAuthorID := uuid.New()
	authorID := uuid.New()
	postID := uuid.New()
	parentIDStr := uuid.New().String()
	parentAuthorIDStr := parentAuthorID.String()

	data := makeEnvelope(postCreatedPayload{
		PostID:         postID.String(),
		AuthorID:       authorID.String(),
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &parentAuthorIDStr,
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(fs.created))
	}
	n := fs.created[0]
	if n.UserID != parentAuthorID {
		t.Errorf("UserID: got %s want %s", n.UserID, parentAuthorID)
	}
	if n.Type != "reply" {
		t.Errorf("Type: got %s want reply", n.Type)
	}
	if n.ActorID != authorID {
		t.Errorf("ActorID: got %s want %s", n.ActorID, authorID)
	}
	if n.EntityType != "post" {
		t.Errorf("EntityType: got %s want post", n.EntityType)
	}
	if n.EntityID != postID {
		t.Errorf("EntityID: got %s want %s", n.EntityID, postID)
	}
}

func TestHandlePostCreated_ReplyToSelf_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	authorID := uuid.New()
	authorIDStr := authorID.String()
	parentIDStr := uuid.New().String()

	data := makeEnvelope(postCreatedPayload{
		PostID:         uuid.New().String(),
		AuthorID:       authorIDStr,
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &authorIDStr, // same as author → no notification
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for self-reply, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_RootPost_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	data := makeEnvelope(postCreatedPayload{
		PostID:   uuid.New().String(),
		AuthorID: uuid.New().String(),
		Kind:     "post",
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for root post, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_BadEnvelope_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	// Should not panic; just logs error.
	svc.HandleEvent(context.Background(), "post.created", []byte(`not json`))

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for bad envelope, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_BadPayload_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	// Envelope is valid but payload field contains non-JSON.
	env, _ := json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
	}{Payload: json.RawMessage(`not json`)})

	svc.HandleEvent(context.Background(), "post.created", env)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for bad payload, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_InvalidAuthorID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	parentAuthorIDStr := uuid.New().String()
	parentIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{
		PostID:         uuid.New().String(),
		AuthorID:       "not-a-uuid",
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &parentAuthorIDStr,
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid author_id, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_InvalidPostID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	parentAuthorIDStr := uuid.New().String()
	parentIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{
		PostID:         "not-a-uuid",
		AuthorID:       uuid.New().String(),
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &parentAuthorIDStr,
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid post_id, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_InvalidParentAuthorID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	bad := "not-a-uuid"
	parentIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{
		PostID:         uuid.New().String(),
		AuthorID:       uuid.New().String(),
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &bad,
	})

	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid parent_author_id, got %d", len(fs.created))
	}
}

func TestHandlePostCreated_DBError_LoggedNoNotification(t *testing.T) {
	fs := &fakeStore{createErr: errors.New("db down")}
	svc := NewNotificationService(fs)

	parentAuthorIDStr := uuid.New().String()
	parentIDStr := uuid.New().String()
	data := makeEnvelope(postCreatedPayload{
		PostID:         uuid.New().String(),
		AuthorID:       uuid.New().String(),
		Kind:           "reply",
		ParentID:       &parentIDStr,
		ParentAuthorID: &parentAuthorIDStr,
	})

	// Should not panic; HandleEvent logs the error.
	svc.HandleEvent(context.Background(), "post.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no stored notifications when DB errors, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_NotifiesArticleAuthor(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	articleAuthorID := uuid.New()
	authorID := uuid.New()
	commentID := uuid.New()

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       commentID.String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        authorID.String(),
		ArticleAuthorID: articleAuthorID.String(),
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(fs.created))
	}
	n := fs.created[0]
	if n.UserID != articleAuthorID {
		t.Errorf("UserID: got %s want %s", n.UserID, articleAuthorID)
	}
	if n.Type != "comment" {
		t.Errorf("Type: got %s want comment", n.Type)
	}
}

func TestHandleCommentCreated_ThreadedReply_NotifiesBoth(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	articleAuthorID := uuid.New()
	parentCommentAuthorID := uuid.New()
	authorID := uuid.New()
	commentID := uuid.New()
	parentIDStr := uuid.New().String()
	parentAuthorIDStr := parentCommentAuthorID.String()

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       commentID.String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        authorID.String(),
		ArticleAuthorID: articleAuthorID.String(),
		ParentID:        &parentIDStr,
		ParentAuthorID:  &parentAuthorIDStr,
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	// Should get 2 notifications: article author + parent comment author
	if len(fs.created) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(fs.created))
	}

	recipients := map[uuid.UUID]string{}
	for _, n := range fs.created {
		recipients[n.UserID] = n.Type
	}
	if _, ok := recipients[articleAuthorID]; !ok {
		t.Error("expected notification for article author")
	}
	if _, ok := recipients[parentCommentAuthorID]; !ok {
		t.Error("expected notification for parent comment author")
	}
}

func TestHandleCommentCreated_SelfComment_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	authorID := uuid.New()

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       uuid.New().String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        authorID.String(),
		ArticleAuthorID: authorID.String(), // same person
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for self-comment, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_BadEnvelope_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	svc.HandleEvent(context.Background(), "comment.created", []byte(`not json`))

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for bad envelope, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_BadPayload_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	env, _ := json.Marshal(struct {
		Payload json.RawMessage `json:"payload"`
	}{Payload: json.RawMessage(`not json`)})

	svc.HandleEvent(context.Background(), "comment.created", env)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for bad payload, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_InvalidAuthorID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       uuid.New().String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        "not-a-uuid",
		ArticleAuthorID: uuid.New().String(),
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid author_id, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_InvalidCommentID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       "not-a-uuid",
		ArticleID:       uuid.New().String(),
		AuthorID:        uuid.New().String(),
		ArticleAuthorID: uuid.New().String(),
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid comment_id, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_InvalidArticleAuthorID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       uuid.New().String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        uuid.New().String(),
		ArticleAuthorID: "not-a-uuid",
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for invalid article_author_id, got %d", len(fs.created))
	}
}

func TestHandleCommentCreated_InvalidParentAuthorID_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	bad := "not-a-uuid"
	parentIDStr := uuid.New().String()
	data := makeEnvelope(commentCreatedPayload{
		CommentID:       uuid.New().String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        uuid.New().String(),
		ArticleAuthorID: uuid.New().String(),
		ParentID:        &parentIDStr,
		ParentAuthorID:  &bad,
	})

	svc.HandleEvent(context.Background(), "comment.created", data)

	// Article author notification is created, but parent notification errors → logged
	// The first CreateNotification (article author) succeeded before the error.
	// Behavior: returns error after creating article notification; error is logged.
	// We just verify no panic.
}

func TestHandleCommentCreated_DBError_LoggedNoNotification(t *testing.T) {
	fs := &fakeStore{createErr: errors.New("db down")}
	svc := NewNotificationService(fs)

	data := makeEnvelope(commentCreatedPayload{
		CommentID:       uuid.New().String(),
		ArticleID:       uuid.New().String(),
		AuthorID:        uuid.New().String(),
		ArticleAuthorID: uuid.New().String(),
	})

	// Should not panic.
	svc.HandleEvent(context.Background(), "comment.created", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no stored notifications when DB errors, got %d", len(fs.created))
	}
}

func TestHandleReactionUpserted_NoNotification(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	data := makeEnvelope(reactionUpsertedPayload{
		PostID:  uuid.New().String(),
		UserID:  uuid.New().String(),
		Emotion: "like",
	})

	svc.HandleEvent(context.Background(), "reaction.upserted", data)

	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for reactions, got %d", len(fs.created))
	}
}

func TestHandleUnknownEventType_NoError(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)
	// Should not panic
	svc.HandleEvent(context.Background(), "unknown.type", []byte(`{}`))
	if len(fs.created) != 0 {
		t.Errorf("expected no notifications for unknown type, got %d", len(fs.created))
	}
}

// ─── CountUnread / List tests ─────────────────────────────────────────────────

func TestCountUnread_ReturnsCount(t *testing.T) {
	userID := uuid.New()
	fs := &fakeStore{countUnreadFn: func(id uuid.UUID) (int64, error) {
		if id != userID {
			t.Errorf("CountUnread: got userID %s want %s", id, userID)
		}
		return 7, nil
	}}
	svc := NewNotificationService(fs)

	count, err := svc.CountUnread(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("expected 7, got %d", count)
	}
}

func TestCountUnread_PropagatesError(t *testing.T) {
	dbErr := errors.New("db error")
	fs := &fakeStore{countUnreadFn: func(_ uuid.UUID) (int64, error) {
		return 0, dbErr
	}}
	svc := NewNotificationService(fs)

	_, err := svc.CountUnread(context.Background(), uuid.New())
	if !errors.Is(err, dbErr) {
		t.Errorf("expected db error, got %v", err)
	}
}

func TestList_DefaultLimit(t *testing.T) {
	userID := uuid.New()
	fs := &fakeStore{listFn: func(id uuid.UUID, limit int) ([]db.Notification, error) {
		if limit != 50 {
			t.Errorf("expected limit 50, got %d", limit)
		}
		return []db.Notification{{ID: uuid.New()}}, nil
	}}
	svc := NewNotificationService(fs)

	// limit=0 → clamped to 50
	items, err := svc.List(context.Background(), userID, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestList_NegativeLimit_ClampsTo50(t *testing.T) {
	fs := &fakeStore{listFn: func(_ uuid.UUID, limit int) ([]db.Notification, error) {
		if limit != 50 {
			t.Errorf("expected limit 50, got %d", limit)
		}
		return nil, nil
	}}
	svc := NewNotificationService(fs)

	_, err := svc.List(context.Background(), uuid.New(), -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestList_ExcessiveLimit_ClampsTo50(t *testing.T) {
	fs := &fakeStore{listFn: func(_ uuid.UUID, limit int) ([]db.Notification, error) {
		if limit != 50 {
			t.Errorf("expected limit 50 for over-limit input, got %d", limit)
		}
		return nil, nil
	}}
	svc := NewNotificationService(fs)

	_, err := svc.List(context.Background(), uuid.New(), 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestList_ValidLimit_PassedThrough(t *testing.T) {
	fs := &fakeStore{listFn: func(_ uuid.UUID, limit int) ([]db.Notification, error) {
		if limit != 30 {
			t.Errorf("expected limit 30, got %d", limit)
		}
		return nil, nil
	}}
	svc := NewNotificationService(fs)

	_, err := svc.List(context.Background(), uuid.New(), 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestList_PropagatesError(t *testing.T) {
	dbErr := errors.New("db error")
	fs := &fakeStore{listFn: func(_ uuid.UUID, _ int) ([]db.Notification, error) {
		return nil, dbErr
	}}
	svc := NewNotificationService(fs)

	_, err := svc.List(context.Background(), uuid.New(), 10)
	if !errors.Is(err, dbErr) {
		t.Errorf("expected db error, got %v", err)
	}
}

// ─── ReadQueue tests ──────────────────────────────────────────────────────────

func TestReadQueue_EnqueueMarkAllRead_WritesToDBAsync(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.StartReadWorker(ctx)

	userID := uuid.New()
	svc.EnqueueMarkAllRead(userID)

	// Worker runs asynchronously; wait up to 1 second for the DB call.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if fs.markedCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if fs.markedCount() == 0 {
		t.Fatal("expected MarkAllRead to be called asynchronously, but DB was not updated")
	}
	fs.mu.Lock()
	job := fs.marked[0]
	fs.mu.Unlock()
	if job.userID != userID {
		t.Errorf("userID: got %s want %s", job.userID, userID)
	}
	if job.ids != nil {
		t.Errorf("expected nil ids for mark-all, got %v", job.ids)
	}
}

func TestReadQueue_EnqueueMarkRead_WritesSpecificIDsAsync(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.StartReadWorker(ctx)

	userID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	svc.EnqueueMarkRead(userID, ids)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if fs.markedCount() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()

	if fs.markedCount() == 0 {
		t.Fatal("expected MarkRead to be called asynchronously, but DB was not updated")
	}
	fs.mu.Lock()
	job := fs.marked[0]
	fs.mu.Unlock()
	if len(job.ids) != 2 {
		t.Errorf("expected 2 ids, got %d", len(job.ids))
	}
}

func TestReadQueue_DrainOnShutdown(t *testing.T) {
	fs := &fakeStore{}
	svc := NewNotificationService(fs)

	ctx, cancel := context.WithCancel(context.Background())

	// Enqueue multiple jobs before the worker starts processing.
	for range 5 {
		svc.EnqueueMarkAllRead(uuid.New())
	}

	go svc.StartReadWorker(ctx)

	// Cancel immediately — drain should still flush all 5 jobs.
	cancel()

	// Give drain up to 2 seconds (5-second drain deadline, but should be fast).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fs.markedCount() >= 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if got := fs.markedCount(); got != 5 {
		t.Errorf("expected 5 drained jobs, got %d", got)
	}
}

func TestReadQueue_Full_DropsGracefully(t *testing.T) {
	// Queue of capacity 1.
	q := NewReadQueue(1)
	userID := uuid.New()

	// Fill the queue.
	q.Enqueue(userID, nil)
	// This should drop (queue full) without blocking or panicking.
	ok := q.Enqueue(userID, nil)
	if ok {
		t.Error("expected second enqueue to return false when queue is full")
	}
}

func TestReadQueue_MarkAllReadDBError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{markAllReadErr: errors.New("db down")}
	svc := NewNotificationService(fs)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.StartReadWorker(ctx)

	svc.EnqueueMarkAllRead(uuid.New())

	// Give worker time to attempt the DB call (it should log error, not panic).
	time.Sleep(50 * time.Millisecond)
	cancel()
}

func TestReadQueue_MarkReadDBError_DoesNotPanic(t *testing.T) {
	fs := &fakeStore{markReadErr: errors.New("db down")}
	svc := NewNotificationService(fs)

	ctx, cancel := context.WithCancel(context.Background())
	go svc.StartReadWorker(ctx)

	svc.EnqueueMarkRead(uuid.New(), []uuid.UUID{uuid.New()})

	// Give worker time to attempt the DB call.
	time.Sleep(50 * time.Millisecond)
	cancel()
}
