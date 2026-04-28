package gate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dpopsuev/tako/circuit"
)

// ErrWebhookFailed is returned when the webhook HTTP POST fails.
var ErrWebhookFailed = fmt.Errorf("webhook failed")

// WebhookNotifier sends HTTP POST notifications when items are parked
// for approval. Follows CloudEvents envelope format.
type WebhookNotifier struct {
	endpoint string
	source   string
	client   *http.Client
}

var _ Notifier = (*WebhookNotifier)(nil)

// NewWebhookNotifier creates a notifier that POSTs CloudEvents to the given endpoint.
func NewWebhookNotifier(endpoint, source string) *WebhookNotifier {
	return &WebhookNotifier{
		endpoint: endpoint,
		source:   source,
		client: &http.Client{
			Timeout: 10 * time.Second, //nolint:mnd // reasonable HTTP timeout
		},
	}
}

// cloudEvent wraps an ApprovalItem in CloudEvents v1.0 envelope.
type cloudEvent struct {
	SpecVersion string       `json:"specversion"`
	Type        string       `json:"type"`
	Source      string       `json:"source"`
	ID          string       `json:"id"`
	Time        string       `json:"time"`
	Data        ApprovalItem `json:"data"`
}

// Notify sends a CloudEvents HTTP POST for the parked approval item.
func (n *WebhookNotifier) Notify(ctx context.Context, item ApprovalItem) error {
	event := cloudEvent{
		SpecVersion: "1.0",
		Type:        "origami.gate.parked",
		Source:      n.source,
		ID:          item.ID,
		Time:        item.ParkedAt.Format(time.RFC3339),
		Data:        item,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("webhook: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/cloudevents+json; charset=utf-8")
	req.Header.Set("Ce-Specversion", "1.0")
	req.Header.Set("Ce-Type", "origami.gate.parked")
	req.Header.Set("Ce-Source", n.source)
	req.Header.Set("Ce-Id", item.ID)

	resp, err := n.client.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "webhook: post failed",
			slog.Any(circuit.LogKeyNode, item.NodeName),
			slog.Any(circuit.LogKeyError, err.Error()))
		return fmt.Errorf("webhook: post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		slog.WarnContext(ctx, "webhook: non-2xx response",
			slog.Any(circuit.LogKeyNode, item.NodeName),
			slog.Int(circuit.LogKeyStatus, resp.StatusCode))
		return fmt.Errorf("%w: status %d", ErrWebhookFailed, resp.StatusCode)
	}

	slog.DebugContext(ctx, "webhook: notification sent",
		slog.Any(circuit.LogKeyNode, item.NodeName),
		slog.Int(circuit.LogKeyStatus, resp.StatusCode))

	return nil
}
