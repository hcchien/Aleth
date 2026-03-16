package events

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
)

// PubSubPublisher publishes events to a GCP Pub/Sub topic.
type PubSubPublisher struct {
	client *pubsub.Client
	topic  *pubsub.Topic
}

// NewPubSubPublisher creates a publisher connected to the given GCP project and topic.
// The caller is responsible for closing the publisher when done.
func NewPubSubPublisher(ctx context.Context, projectID, topicID string) (*PubSubPublisher, error) {
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("create pubsub client: %w", err)
	}
	topic := client.Topic(topicID)
	return &PubSubPublisher{client: client, topic: topic}, nil
}

// Close flushes pending messages and closes the underlying client.
func (p *PubSubPublisher) Close() {
	p.topic.Stop()
	p.client.Close()
}

// Publish serialises the event and publishes it to the configured Pub/Sub topic.
// The call blocks until GCP acknowledges receipt of the message.
func (p *PubSubPublisher) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	result := p.topic.Publish(ctx, &pubsub.Message{
		Data: data,
		Attributes: map[string]string{
			"event_type": event.Type,
		},
	})

	if _, err := result.Get(ctx); err != nil {
		return fmt.Errorf("publish to pubsub: %w", err)
	}

	log.Debug().Str("type", event.Type).Str("id", event.ID).Msg("event published to Pub/Sub")
	return nil
}
