package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureServer records the requests a provider received so a test can assert
// on the URL path, headers and body rather than only on "no error".
type captureServer struct {
	mu       sync.Mutex
	t        *testing.T
	requests []capturedRequest
	status   int
	answer   string
}

type capturedRequest struct {
	path        string
	contentType string
	body        map[string]any
}

func newCaptureServer(t *testing.T, status int, answer string) *captureServer {
	return &captureServer{t: t, status: status, answer: answer}
}

func (c *captureServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			body = nil
		}
		c.mu.Lock()
		c.requests = append(c.requests, capturedRequest{
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		})
		c.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(c.status)
		fmt.Fprint(w, c.answer)
	})
}

func (c *captureServer) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.requests)
}

func (c *captureServer) oneRequest() capturedRequest {
	c.t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.requests) != 1 {
		c.t.Fatalf("provider received %d requests, want 1", len(c.requests))
	}
	return c.requests[0]
}

// useTelegramFake points the Telegram delivery path at a local server. Bot
// tokens must never leave the test process, and this also proves the URL is
// still built as /bot<token>/sendMessage.
func useTelegramFake(t *testing.T, url string) {
	t.Helper()
	original := TelegramAPIBase
	TelegramAPIBase = url
	t.Cleanup(func() { TelegramAPIBase = original })
}

func newNotifier() *Notifier {
	return NewNotifier(&http.Client{Timeout: 2 * time.Second}, nil)
}

func TestSendTextSlack(t *testing.T) {
	srv := httptest.NewServer(newTextProvider(t))
	defer srv.Close()

	err := newNotifier().SendText(context.Background(), "SLACK",
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), "revenue dropped")
	if err != nil {
		t.Fatalf("SendText: %v", err)
	}
}

func TestSendTextSlackSendsTextField(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, `{"ok":true}`)
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	msg := "revenue dropped below threshold"
	if err := newNotifier().SendText(context.Background(), "SLACK",
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), msg); err != nil {
		t.Fatalf("SendText: %v", err)
	}

	req := c.oneRequest()
	if req.contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", req.contentType)
	}
	if got, _ := req.body["text"].(string); got != msg {
		t.Errorf("text = %q, want %q", got, msg)
	}
}

func TestSendTextTelegramUsesBotPath(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, `{"ok":true,"result":{}}`)
	srv := httptest.NewServer(c.handler())
	defer srv.Close()
	useTelegramFake(t, srv.URL)

	if err := newNotifier().SendText(context.Background(), "TELEGRAM",
		`{"bot_token":"123:secret-token","chat_id":"-1001"}`, "digest ready"); err != nil {
		t.Fatalf("SendText: %v", err)
	}

	req := c.oneRequest()
	if req.path != "/bot123:secret-token/sendMessage" {
		t.Errorf("path = %q, want /bot123:secret-token/sendMessage", req.path)
	}
	if got, _ := req.body["chat_id"].(string); got != "-1001" {
		t.Errorf("chat_id = %q, want -1001", got)
	}
	if got, _ := req.body["text"].(string); got != "digest ready" {
		t.Errorf("text = %q, want %q", got, "digest ready")
	}
}

func TestChannelIsCaseInsensitive(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, "")
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	if err := newNotifier().SendText(context.Background(), " slack ",
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), "hello"); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	c.oneRequest()
}

// A revoked Slack webhook answers 401. Treating that as delivered is the bug
// this guards: the alert would be recorded as sent and never retried.
func TestNonOKStatusIsDeliveryFailure(t *testing.T) {
	c := newCaptureServer(t, http.StatusUnauthorized, `{"ok":false,"error":"invalid_token"}`)
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	err := newNotifier().SendText(context.Background(), "SLACK",
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), "hello")
	if err == nil {
		t.Fatal("want error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid_token") {
		t.Errorf("error = %q, want the status and provider message", err)
	}
}

// Telegram answers 200 even when it refuses a message, so the body decides.
func TestTelegramRejectedWithOKFalse(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, `{"ok":false,"description":"chat not found"}`)
	srv := httptest.NewServer(c.handler())
	defer srv.Close()
	useTelegramFake(t, srv.URL)

	err := newNotifier().SendText(context.Background(), "telegram",
		`{"bot_token":"12:abc","chat_id":"99"}`, "hello")
	if err == nil {
		t.Fatal("want error when telegram answers ok:false, got nil")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("error = %q, want the telegram description", err)
	}
}

func TestMissingRecipientConfigIsError(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, "")
	srv := httptest.NewServer(c.handler())
	defer srv.Close()
	useTelegramFake(t, srv.URL)

	cases := []struct {
		name      string
		channel   string
		recipient string
		wantField string
	}{
		{"slack without webhook", "SLACK", `{}`, "webhook_url"},
		{"slack empty webhook", "SLACK", `{"webhook_url":"  "}`, "webhook_url"},
		{"slack non-http webhook", "SLACK", `{"webhook_url":"ftp://example.com"}`, "http(s)"},
		{"telegram without token", "TELEGRAM", `{"chat_id":"1"}`, "bot_token"},
		{"telegram without chat", "TELEGRAM", `{"bot_token":"12:abc"}`, "chat_id"},
		{"telegram unparsable config", "TELEGRAM", `not json`, "recipient_config"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := newNotifier().SendText(context.Background(), tc.channel, tc.recipient, "hello")
			if err == nil {
				t.Fatal("want error for incomplete recipient_config, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("error = %q, want it to name %s", err, tc.wantField)
			}
			if c.count() != 0 {
				t.Fatalf("provider received %d requests, want none", c.count())
			}
		})
	}
}

func TestUnknownChannelIsError(t *testing.T) {
	err := newNotifier().SendText(context.Background(), "PAGER", `{"webhook_url":"http://x"}`, "hello")
	if err == nil {
		t.Fatal("want error for unsupported channel, got nil")
	}
	if !strings.Contains(err.Error(), "PAGER") {
		t.Errorf("error = %q, want it to name the channel", err)
	}
}

// EMAIL has no SMTP sender. It must not fail (an alert would retry forever) but
// must not be reported as delivered either, so it returns nil without a request.
func TestEmailIsAcceptedButNotDelivered(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, "")
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	if err := newNotifier().SendText(context.Background(), "EMAIL", `{"email_list":["a@b.c"]}`, "hello"); err != nil {
		t.Fatalf("SendText(EMAIL): %v", err)
	}
	if c.count() != 0 {
		t.Errorf("provider received %d requests, want none", c.count())
	}
}

func TestPostJSONSendsStructuredBody(t *testing.T) {
	c := newCaptureServer(t, http.StatusOK, "")
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	body := map[string]any{
		"alert_id":     "al-1",
		"metric_value": 42.0,
		"env_tag":      "production",
	}
	if err := newNotifier().PostJSON(context.Background(),
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), body); err != nil {
		t.Fatalf("PostJSON: %v", err)
	}

	req := c.oneRequest()
	if req.path != "/" {
		t.Errorf("path = %q, want the configured webhook path", req.path)
	}
	if got, _ := req.body["alert_id"].(string); got != "al-1" {
		t.Errorf("alert_id = %v, want al-1", req.body["alert_id"])
	}
	if got, _ := req.body["metric_value"].(float64); got != 42 {
		t.Errorf("metric_value = %v, want 42", req.body["metric_value"])
	}
}

func TestPostJSONWithoutWebhookIsError(t *testing.T) {
	err := newNotifier().PostJSON(context.Background(), `{}`, map[string]any{"a": 1})
	if err == nil {
		t.Fatal("want error for missing webhook_url, got nil")
	}
	if !strings.Contains(err.Error(), "webhook_url") {
		t.Errorf("error = %q, want it to name webhook_url", err)
	}
}

func TestBotTokenNeverAppearsInError(t *testing.T) {
	// Unreachable host, so the transport error carries the full request URL.
	useTelegramFake(t, "http://127.0.0.1:1")

	const token = "123456:SUPERSECRETtokenvalue"
	err := newNotifier().SendText(context.Background(), "TELEGRAM",
		fmt.Sprintf(`{"bot_token":%q,"chat_id":"1"}`, token), "hello")
	if err == nil {
		t.Fatal("want error for unreachable provider, got nil")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("error leaks the bot token: %q", err)
	}
	if strings.Contains(err.Error(), "SUPERSECRET") {
		t.Errorf("error leaks part of the bot token: %q", err)
	}
}

func TestProviderBodyIsCapped(t *testing.T) {
	huge := `{"ok":false,"error":"` + strings.Repeat("A", maxReadBody*2) + `"}`
	c := newCaptureServer(t, http.StatusInternalServerError, huge)
	srv := httptest.NewServer(c.handler())
	defer srv.Close()

	err := newNotifier().SendText(context.Background(), "SLACK",
		fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), "hello")
	if err == nil {
		t.Fatal("want error for 500, got nil")
	}
	if len(err.Error()) > maxErrorBody+len("slack: provider answered 500: ")+10 {
		t.Errorf("error is not capped: %d bytes", len(err.Error()))
	}
}

func TestCanceledContextStopsDelivery(t *testing.T) {
	srv := httptest.NewServer(newTextProvider(t))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := newNotifier().SendText(ctx, "SLACK", fmt.Sprintf(`{"webhook_url":%q}`, srv.URL), "hello")
	if err == nil {
		t.Fatal("want error for canceled context, got nil")
	}
}

// newTextProvider answers like a provider that accepts everything.
func newTextProvider(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var discard map[string]any
		_ = json.NewDecoder(r.Body).Decode(&discard)
		w.WriteHeader(http.StatusOK)
	})
}
