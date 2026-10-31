package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"muara.bi/core/internal/domain/entity"
	"muara.bi/core/pkg/logging"
)

// TelegramAPIBase is the only host a bot token is ever sent to. It is a var so
// the dispatch tests can point at a local fake instead of the real service.
var TelegramAPIBase = "https://api.telegram.org"

// maxErrorBody caps how much of a provider response is copied into an error.
// Alert payloads can contain user data, and a provider error page can be huge.
const maxErrorBody = 200

// maxReadBody caps how much of a provider response is read at all, so a
// misconfigured webhook cannot stream an unbounded body into worker memory.
const maxReadBody = 4096

// Notifier delivers alert and digest messages to the channel saved on the alert.
// Both workers use it so a channel only has to be correct in one place.
type Notifier struct {
	httpClient *http.Client
	logger     *logging.Logger
}

func NewNotifier(httpClient *http.Client, logger *logging.Logger) *Notifier {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Notifier{httpClient: httpClient, logger: logger}
}

// SendText posts the rendered message to a text channel. Anything that could
// stop the message arriving is returned as an error so Asynq retries instead of
// marking the alert as delivered.
func (n *Notifier) SendText(ctx context.Context, channel, recipientJSON, message string) error {
	recipient, err := entity.ParseAlertRecipient(json.RawMessage(recipientJSON))
	if err != nil {
		return err
	}

	ch := strings.ToUpper(strings.TrimSpace(channel))
	if err := recipient.ValidateFor(ch); err != nil {
		return err
	}

	switch ch {
	case entity.ChannelSlack:
		body, err := json.Marshal(map[string]string{"text": message})
		if err != nil {
			return fmt.Errorf("failed to encode slack message: %w", err)
		}
		return n.post(ctx, "slack", recipient.WebhookURL, body)

	case entity.ChannelTelegram:
		body, err := json.Marshal(map[string]string{"chat_id": recipient.ChatID, "text": message})
		if err != nil {
			return fmt.Errorf("failed to encode telegram message: %w", err)
		}
		// The token is part of the path, so the URL is never written into an
		// error string or a log line.
		apiURL := fmt.Sprintf("%s/bot%s/sendMessage", strings.TrimSuffix(TelegramAPIBase, "/"), recipient.BotToken)
		return n.post(ctx, "telegram", apiURL, body)

	case entity.ChannelEmail:
		// No SMTP sender exists yet. This stays a warning rather than an error so
		// an email alert does not retry forever, but it is no longer silent.
		n.warn("EMAIL alerts are stored but not delivered: no SMTP sender is implemented")
		return nil

	default:
		return fmt.Errorf("channel %q cannot send a text message", channel)
	}
}

// PostJSON delivers a structured body to recipient_config.webhook_url.
func (n *Notifier) PostJSON(ctx context.Context, recipientJSON string, body any) error {
	recipient, err := entity.ParseAlertRecipient(json.RawMessage(recipientJSON))
	if err != nil {
		return err
	}
	if err := recipient.ValidateFor(entity.ChannelWebhook); err != nil {
		return err
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode webhook body: %w", err)
	}
	return n.post(ctx, "webhook", recipient.WebhookURL, encoded)
}

// post treats any non-2xx answer as a delivery failure. Accepting 200 alone was
// the old behaviour's weak point: a 401 from a revoked Slack webhook looked like
// a successful alert.
func (n *Notifier) post(ctx context.Context, kind, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: request could not be built: %w", kind, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: delivery failed: %s", kind, redactProviderURL(err))
	}
	defer resp.Body.Close()

	// Enough of the body to decode a provider answer, but never all of it: a
	// misconfigured webhook can point at an endpoint that streams back a lot.
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxReadBody))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s: provider answered %d: %s", kind, resp.StatusCode, trimSnippet(responseBody))
	}

	// Telegram reports failures with HTTP 200 and ok:false.
	if kind == "telegram" {
		var answer struct {
			OK  bool   `json:"ok"`
			Err string `json:"description"`
		}
		if err := json.Unmarshal(responseBody, &answer); err == nil && !answer.OK {
			return fmt.Errorf("telegram rejected the message: %s", trimSnippet([]byte(answer.Err)))
		}
	}
	return nil
}

func trimSnippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > maxErrorBody {
		return s[:maxErrorBody] + "..."
	}
	return s
}

// redactProviderURL keeps a transport error readable without echoing the whole
// URL: Go wraps it with the request URL, which for Telegram is the bot token.
func redactProviderURL(err error) string {
	msg := err.Error()
	if idx := strings.Index(msg, TelegramAPIBase); idx >= 0 {
		return msg[:idx] + "<telegram url redacted>"
	}
	return msg
}

func (n *Notifier) warn(message string) {
	if n.logger != nil {
		n.logger.Warn(message)
	}
}
