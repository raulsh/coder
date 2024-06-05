package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"cdr.dev/slog"
	"github.com/coder/coder/v2/coderd/database"
	"github.com/coder/coder/v2/coderd/notifications/types"
	"github.com/coder/coder/v2/codersdk"
	"github.com/google/uuid"
	"golang.org/x/xerrors"
)

type WebhookDispatcher struct {
	cfg codersdk.NotificationsWebhookConfig
	log slog.Logger

	cl *http.Client
}

type WebhookPayload struct {
	Version          int          `json:"version"`
	MsgID            uuid.UUID    `json:"msg_id"`
	Labels           types.Labels `json:"labels"`
	Title            string       `json:"title"`
	Body             string       `json:"body"`
	NotificationType string       `json:"notification_type"`
}

const (
	labelNotificationType = "notification_type"
	labelTitle            = "title"

	// TODO: configurable
	webhookTimeout = time.Second * 30
)

func NewWebhookDispatcher(cfg codersdk.NotificationsWebhookConfig, log slog.Logger) *WebhookDispatcher {
	return &WebhookDispatcher{cfg: cfg, log: log, cl: &http.Client{Timeout: webhookTimeout}}
}

func (w *WebhookDispatcher) Name() string {
	// TODO: don't use database types
	return string(database.NotificationMethodWebhook)
}

// Validate returns a bool indicating whether the required labels for the Send operation are present, as well as
// a slice of missing labels.
func (w *WebhookDispatcher) Validate(input types.Labels) (bool, []string) {
	missing := input.Missing(labelNotificationType, labelTitle, labelBody)
	return len(missing) == 0, missing
}

// Send delivers the notification.
// The first return param indicates whether a retry can be attempted (i.e. a temporary error), and the second returns
// any error that may have arisen.
// If (false, nil) is returned, that is considered a successful dispatch.
func (w *WebhookDispatcher) Send(ctx context.Context, msgID uuid.UUID, input types.Labels) (bool, error) {
	valid, missing := w.Validate(input)
	if !valid {
		return false, xerrors.Errorf("missing labels: %v", missing)
	}

	// Set required label.
	url := w.cfg.Endpoint.String()

	// Extract fields from labels
	ntype := input.Cut(labelNotificationType)
	title := input.Cut(labelTitle)
	body := input.Cut(labelBody)

	// Prepare payload.
	payload := WebhookPayload{
		Version:          1,
		MsgID:            msgID,
		Title:            title,
		Body:             body,
		Labels:           input,
		NotificationType: ntype,
	}
	m, err := json.Marshal(payload)
	if err != nil {
		return false, xerrors.Errorf("marshal payload: %v", err)
	}

	// Prepare request.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(m))
	if err != nil {
		return false, xerrors.Errorf("create HTTP request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send request.
	resp, err := w.cl.Do(req)
	if err != nil {
		return true, xerrors.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	// Handle response.
	if resp.StatusCode/100 > 2 {
		var limitedResp []byte
		_, err = io.LimitReader(resp.Body, 100).Read(limitedResp)
		if err != nil {
			return true, xerrors.Errorf("non-200 response (%d), read body: %w", resp.StatusCode, err)
		}
		return true, xerrors.Errorf("non-200 response (%d): %s", resp.StatusCode, body)
	}

	return false, nil
}
