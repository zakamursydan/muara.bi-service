package entity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AlertRecipient is the delivery target stored in recipient_config. The keys
// here are the ones the dispatcher reads, so a saved alert and a sent alert are
// checked the same way.
type AlertRecipient struct {
	WebhookURL string
	BotToken   string
	ChatID     string
	EmailList  []string
}

// ParseAlertRecipient decodes a recipient_config object. An empty config is
// legal JSON but carries no target, so it parses to zero fields and is rejected
// by ValidateFor.
func ParseAlertRecipient(raw json.RawMessage) (*AlertRecipient, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		trimmed = []byte("{}")
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	// UseNumber keeps a numeric chat_id exact: chat ids are large and decoding
	// through float64 can rewrite their digits.
	dec.UseNumber()

	var fields map[string]any
	if err := dec.Decode(&fields); err != nil {
		return nil, fmt.Errorf("recipient_config must be a JSON object: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("recipient_config must be a JSON object")
	}

	return &AlertRecipient{
		WebhookURL: stringField(fields, "webhook_url"),
		BotToken:   stringField(fields, "bot_token"),
		ChatID:     stringField(fields, "chat_id"),
		EmailList:  stringListField(fields, "email_list"),
	}, nil
}

// ValidateFor reports the config a channel needs before it can ever deliver.
func (r *AlertRecipient) ValidateFor(channel string) error {
	ch := strings.ToUpper(strings.TrimSpace(channel))
	switch ch {
	case ChannelTelegram:
		if r.BotToken == "" {
			return fmt.Errorf("recipient_config.bot_token is required for TELEGRAM")
		}
		if r.ChatID == "" {
			return fmt.Errorf("recipient_config.chat_id is required for TELEGRAM")
		}
		return nil

	case ChannelSlack, ChannelWebhook:
		if r.WebhookURL == "" {
			return fmt.Errorf("recipient_config.webhook_url is required for %s", ch)
		}
		if !isHTTPSEndpoint(r.WebhookURL) {
			return fmt.Errorf("recipient_config.webhook_url must be an http(s) URL")
		}
		return nil

	case ChannelEmail:
		if len(r.EmailList) == 0 {
			return fmt.Errorf("recipient_config.email_list needs at least one address")
		}
		for _, addr := range r.EmailList {
			if !isEmail(addr) {
				return fmt.Errorf("%q is not an email address", addr)
			}
		}
		return nil

	default:
		return fmt.Errorf("channel %q cannot deliver notifications", channel)
	}
}

func stringField(fields map[string]any, key string) string {
	switch v := fields[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func stringListField(fields map[string]any, key string) []string {
	switch v := fields[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	default:
		return nil
	}
}

func isHTTPSEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	// A scheme-only value ("https://") or an absolute path without a host
	// ("hooks.slack.example/x", which parses as a path) both deliver nowhere.
	return u.Host != ""
}

func isEmail(addr string) bool {
	if strings.ContainsAny(addr, " \t") || strings.Count(addr, "@") != 1 {
		return false
	}
	at := strings.Index(addr, "@")
	if at <= 0 || at == len(addr)-1 {
		return false
	}
	domain := addr[at+1:]
	dot := strings.LastIndex(domain, ".")
	return dot > 0 && dot < len(domain)-1
}
