package entity

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAlertRecipientReadsEachChannel(t *testing.T) {
	raw := json.RawMessage(`{
		"webhook_url": "https://hooks.slack.example/services/A/B/C",
		"bot_token": "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
		"chat_id": "-1001234567890",
		"email_list": ["ops@example.com", " oncall@example.com "]
	}`)

	got, err := ParseAlertRecipient(raw)
	if err != nil {
		t.Fatalf("ParseAlertRecipient: %v", err)
	}
	if got.WebhookURL == "" || got.BotToken == "" {
		t.Errorf("webhook/token not read: %+v", got)
	}
	if got.ChatID != "-1001234567890" {
		t.Errorf("chat_id = %q, want -1001234567890", got.ChatID)
	}
	want := []string{"ops@example.com", "oncall@example.com"}
	if len(got.EmailList) != len(want) {
		t.Fatalf("email_list = %v, want %v", got.EmailList, want)
	}
	for i := range want {
		if got.EmailList[i] != want[i] {
			t.Errorf("email_list[%d] = %q, want %q", i, got.EmailList[i], want[i])
		}
	}
}

// Telegram chat ids are numbers in most saved configs, and they are too large to
// route through float64 without rewriting the digits.
func TestParseAlertRecipientKeepsNumericChatIDExact(t *testing.T) {
	got, err := ParseAlertRecipient(json.RawMessage(`{"chat_id": -1001234567890}`))
	if err != nil {
		t.Fatalf("ParseAlertRecipient: %v", err)
	}
	if got.ChatID != "-1001234567890" {
		t.Errorf("chat_id = %q, want -1001234567890", got.ChatID)
	}
}

func TestParseAlertRecipientRejectsNonObjects(t *testing.T) {
	for _, raw := range []string{`not json`, `[{"chat_id":"1"}]`, `"a string"`, `null`, `{"chat_id":{"nested":1}}`} {
		got, err := ParseAlertRecipient(json.RawMessage(raw))
		if err != nil {
			continue
		}
		if raw == `{"chat_id":{"nested":1}}` {
			if got.ChatID != "" {
				t.Errorf("object chat_id = %q, want it ignored", got.ChatID)
			}
			continue
		}
		t.Errorf("ParseAlertRecipient(%s) accepted, want an error", raw)
	}
}

func TestValidateForRequiredFields(t *testing.T) {
	cases := []struct {
		name      string
		channel   string
		config    string
		wantField string
	}{
		{"slack needs webhook", "SLACK", `{}`, "webhook_url"},
		{"webhook needs url", "WEBHOOK", `{"bot_token":"12:abc"}`, "webhook_url"},
		{"telegram needs token", "TELEGRAM", `{"chat_id":"1"}`, "bot_token"},
		{"telegram needs chat", "TELEGRAM", `{"bot_token":"12:abc"}`, "chat_id"},
		{"email needs a list", "EMAIL", `{}`, "email_list"},
		{"email rejects bad address", "EMAIL", `{"email_list":["not-an-email"]}`, "not-an-email"},
		{"unknown channel", "PAGER", `{}`, "PAGER"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recipient, err := ParseAlertRecipient(json.RawMessage(tc.config))
			if err != nil {
				t.Fatalf("ParseAlertRecipient: %v", err)
			}
			err = recipient.ValidateFor(tc.channel)
			if err == nil {
				t.Fatalf("want error for %s with %s, got nil", tc.channel, tc.config)
			}
			if !strings.Contains(err.Error(), tc.wantField) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantField)
			}
		})
	}
}

func TestValidateForAcceptsCompleteConfigs(t *testing.T) {
	cases := []struct {
		channel string
		config  string
	}{
		{"SLACK", `{"webhook_url":"https://hooks.slack.example/x"}`},
		{"slack", `{"webhook_url":"http://localhost:9000/hook"}`},
		{"TELEGRAM", `{"bot_token":"123:abc","chat_id":"-1001"}`},
		{"telegram", `{"bot_token":"123:abc","chat_id":-1001}`},
		{"EMAIL", `{"email_list":["ops@example.com"]}`},
		{"WEBHOOK", `{"webhook_url":"https://example.com/hook"}`},
	}

	for _, tc := range cases {
		recipient, err := ParseAlertRecipient(json.RawMessage(tc.config))
		if err != nil {
			t.Fatalf("ParseAlertRecipient(%s): %v", tc.config, err)
		}
		if err := recipient.ValidateFor(tc.channel); err != nil {
			t.Errorf("ValidateFor(%s, %s) = %v, want nil", tc.channel, tc.config, err)
		}
	}
}

// An http(s) host is required so a typo like "hooks.slack.example/x" cannot be
// saved and then fail on every scheduled run.
func TestValidateForRejectsUnusableWebhookURLs(t *testing.T) {
	for _, raw := range []string{
		`{"webhook_url":"hooks.slack.example/x"}`,
		`{"webhook_url":"https://"}`,
		`{"webhook_url":"javascript:alert(1)"}`,
	} {
		recipient, err := ParseAlertRecipient(json.RawMessage(raw))
		if err != nil {
			t.Fatalf("ParseAlertRecipient: %v", err)
		}
		if err := recipient.ValidateFor("SLACK"); err == nil {
			t.Errorf("ValidateFor(%s) = nil, want an error", raw)
		}
	}
}
