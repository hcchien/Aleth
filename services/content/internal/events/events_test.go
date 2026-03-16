package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	. "github.com/aleth/content/internal/events"
)

// ─── DirectPublisher tests ────────────────────────────────────────────────────

func TestDirectPublisher_NoHandlers_ReturnsNil(t *testing.T) {
	dp := &DirectPublisher{}
	evt := makeEvent(TypePostCreated, PostCreatedPayload{PostID: "p1", AuthorID: "a1", Kind: "post"})

	if err := dp.Publish(context.Background(), evt); err != nil {
		t.Fatalf("expected nil error with no handlers, got %v", err)
	}
}

func TestDirectPublisher_SingleHandler_ReceivesEvent(t *testing.T) {
	dp := &DirectPublisher{}

	var got Event
	dp.Register(func(_ context.Context, e Event) error {
		got = e
		return nil
	})

	evt := makeEvent(TypePostCreated, PostCreatedPayload{PostID: "p1", AuthorID: "a1", Kind: "post"})
	if err := dp.Publish(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.ID != evt.ID {
		t.Errorf("event ID mismatch: got %s want %s", got.ID, evt.ID)
	}
	if got.Type != TypePostCreated {
		t.Errorf("event type mismatch: got %s want %s", got.Type, TypePostCreated)
	}
}

func TestDirectPublisher_MultipleHandlers_AllCalled(t *testing.T) {
	dp := &DirectPublisher{}

	callCount := 0
	for range 3 {
		dp.Register(func(_ context.Context, _ Event) error {
			callCount++
			return nil
		})
	}

	evt := makeEvent(TypeCommentCreated, CommentCreatedPayload{CommentID: "c1", ArticleID: "a1", AuthorID: "u1"})
	if err := dp.Publish(context.Background(), evt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 handler calls, got %d", callCount)
	}
}

func TestDirectPublisher_HandlerError_DoesNotPropagate(t *testing.T) {
	dp := &DirectPublisher{}
	dp.Register(func(_ context.Context, _ Event) error {
		return errors.New("handler failure")
	})

	evt := makeEvent(TypeReactionUpserted, ReactionUpsertedPayload{PostID: "p1", UserID: "u1", Emotion: "like"})
	if err := dp.Publish(context.Background(), evt); err != nil {
		t.Fatalf("handler error must not propagate; got %v", err)
	}
}

func TestDirectPublisher_HandlerError_OtherHandlersStillRun(t *testing.T) {
	dp := &DirectPublisher{}
	dp.Register(func(_ context.Context, _ Event) error {
		return errors.New("first handler fails")
	})
	secondCalled := false
	dp.Register(func(_ context.Context, _ Event) error {
		secondCalled = true
		return nil
	})

	evt := makeEvent(TypeReactionRemoved, ReactionRemovedPayload{PostID: "p1", UserID: "u1"})
	_ = dp.Publish(context.Background(), evt)

	if !secondCalled {
		t.Error("second handler must be called even when first handler returns an error")
	}
}

func TestDirectPublisher_ConcurrentPublish_NoDataRace(t *testing.T) {
	dp := &DirectPublisher{}
	dp.Register(func(_ context.Context, _ Event) error { return nil })

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			evt := makeEvent(TypePostCreated, PostCreatedPayload{PostID: "p", AuthorID: "a", Kind: "post"})
			_ = dp.Publish(context.Background(), evt)
		}()
	}
	wg.Wait()
}

func TestDirectPublisher_RegisterAfterPublish_NewHandlerCalledOnNextPublish(t *testing.T) {
	dp := &DirectPublisher{}

	firstCalls := 0
	dp.Register(func(_ context.Context, _ Event) error {
		firstCalls++
		return nil
	})

	evt := makeEvent(TypePostCreated, PostCreatedPayload{PostID: "p", AuthorID: "a", Kind: "post"})
	_ = dp.Publish(context.Background(), evt)

	secondCalled := false
	dp.Register(func(_ context.Context, _ Event) error {
		secondCalled = true
		return nil
	})

	_ = dp.Publish(context.Background(), evt)

	if firstCalls != 2 {
		t.Errorf("first handler: expected 2 calls, got %d", firstCalls)
	}
	if !secondCalled {
		t.Error("handler registered after first publish must be called on second publish")
	}
}

// ─── Event payload serialisation tests ───────────────────────────────────────

func TestEvent_PayloadRoundTrip_PostCreated(t *testing.T) {
	want := PostCreatedPayload{PostID: "p1", AuthorID: "a1", Kind: "reply", ParentID: strPtr("parent1")}
	evt := makeEvent(TypePostCreated, want)

	var got PostCreatedPayload
	if err := json.Unmarshal(evt.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.PostID != want.PostID || got.Kind != want.Kind || *got.ParentID != *want.ParentID {
		t.Errorf("payload mismatch: got %+v want %+v", got, want)
	}
}

func TestEvent_PayloadRoundTrip_ReactionUpserted(t *testing.T) {
	want := ReactionUpsertedPayload{PostID: "p1", UserID: "u1", Emotion: "love"}
	evt := makeEvent(TypeReactionUpserted, want)

	var got ReactionUpsertedPayload
	if err := json.Unmarshal(evt.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got != want {
		t.Errorf("payload mismatch: got %+v want %+v", got, want)
	}
}

func TestEvent_OccurredAt_IsRecent(t *testing.T) {
	before := time.Now()
	evt := makeEvent(TypePostCreated, PostCreatedPayload{PostID: "p", AuthorID: "a", Kind: "post"})
	after := time.Now()

	if evt.OccurredAt.Before(before) || evt.OccurredAt.After(after) {
		t.Errorf("OccurredAt %v is outside expected range [%v, %v]", evt.OccurredAt, before, after)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func makeEvent(eventType string, payload any) Event {
	data, _ := json.Marshal(payload)
	return Event{
		ID:         "test-id",
		Type:       eventType,
		OccurredAt: time.Now().UTC(),
		Payload:    data,
	}
}

func strPtr(s string) *string { return &s }
