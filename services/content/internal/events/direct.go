package events

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
)

// HandlerFunc processes a domain event synchronously in-process.
type HandlerFunc func(ctx context.Context, event Event) error

// DirectPublisher delivers events to registered in-process handlers.
// It is intended for local development and testing — no external queue is required.
// Subscriber logic can be registered via Register and will be called synchronously
// on every Publish, making it easy to test the full flow without GCP Pub/Sub.
type DirectPublisher struct {
	mu       sync.RWMutex
	handlers []HandlerFunc
}

// Register adds a handler that will be called for every published event.
func (d *DirectPublisher) Register(h HandlerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, h)
}

// Publish calls all registered handlers synchronously.
// Handler errors are logged but do not propagate — a single failing handler
// will not prevent other handlers from running.
func (d *DirectPublisher) Publish(ctx context.Context, event Event) error {
	d.mu.RLock()
	handlers := d.handlers
	d.mu.RUnlock()

	if len(handlers) == 0 {
		log.Debug().Str("type", event.Type).Str("id", event.ID).Msg("event published (no handlers registered)")
		return nil
	}

	for _, h := range handlers {
		if err := h(ctx, event); err != nil {
			log.Error().Err(err).Str("type", event.Type).Str("id", event.ID).Msg("direct event handler failed")
		}
	}
	return nil
}
