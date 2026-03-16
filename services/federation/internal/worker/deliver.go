// Package worker contains background goroutines for the federation service.
package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aleth/federation/internal/config"
	"github.com/aleth/federation/internal/db"
	"github.com/aleth/federation/internal/httpsig"
	"github.com/aleth/federation/internal/keys"
)

// DeliveryWorker polls the delivery_queue and fans out signed AP activities
// to remote inboxes with exponential back-off on failure.
type DeliveryWorker struct {
	cfg config.Config
	db  *db.Pool
}

// New creates a DeliveryWorker.
func New(cfg config.Config, pool *db.Pool) *DeliveryWorker {
	return &DeliveryWorker{cfg: cfg, db: pool}
}

// Run starts the polling loop. It blocks until ctx is cancelled.
func (w *DeliveryWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Run one sweep immediately at startup so we don't wait 30 s for the first flush.
	w.sweep(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.sweep(ctx)
		}
	}
}

// sweep fetches pending deliveries and processes each one.
func (w *DeliveryWorker) sweep(ctx context.Context) {
	const batchSize = 50

	items, err := w.db.PollPendingDeliveries(ctx, batchSize)
	if err != nil {
		log.Error().Err(err).Msg("delivery worker: poll failed")
		return
	}

	for _, item := range items {
		item := item // capture
		go func() {
			deliverCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			if err := w.send(deliverCtx, item); err != nil {
				log.Warn().
					Err(err).
					Str("id", item.ID.String()).
					Str("inbox", item.TargetInbox).
					Int("attempts", item.Attempts).
					Msg("delivery worker: send failed, scheduling retry")

				if retryErr := w.db.MarkDeliveryRetry(ctx, item.ID, item.Attempts, err.Error()); retryErr != nil {
					log.Error().Err(retryErr).Str("id", item.ID.String()).Msg("delivery worker: mark retry failed")
				}
				return
			}

			if err := w.db.MarkDeliveryDone(ctx, item.ID); err != nil {
				log.Error().Err(err).Str("id", item.ID.String()).Msg("delivery worker: mark done failed")
			}
		}()
	}
}

// send signs and POSTs a single queued activity to its target inbox.
func (w *DeliveryWorker) send(ctx context.Context, item db.DeliveryItem) error {
	// Look up the actor's private key.
	actorKey, err := w.db.GetActorKeyByUsername(ctx, item.LocalUsername)
	if err != nil || actorKey == nil {
		return fmt.Errorf("no actor key for %s", item.LocalUsername)
	}
	privKey, err := keys.DecryptPrivateKey(actorKey.PrivateKeyEnc, w.cfg.PlatformKeySecret)
	if err != nil {
		return fmt.Errorf("decrypt private key: %w", err)
	}

	// Marshal activity JSON.
	body, err := json.Marshal(item.ActivityJSON)
	if err != nil {
		return fmt.Errorf("marshal activity: %w", err)
	}

	// SHA-256 Digest header.
	sum := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(sum[:])

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, item.TargetInbox, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("Digest", digest)

	keyID := "https://" + w.cfg.Domain + "/@" + item.LocalUsername + "#main-key"
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
