package notifications

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"unicode/utf16"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Telegram's sendMessage budget is after HTML entity parsing. Count in UTF-16
// units so supplementary characters cannot make an ostensibly short batch too big.
const telegramTextLimit = 4096

// Headroom for fixed labels, heading, link text, group counts, dates, duration,
// and two fixed-decimal float64 values (each can take over 300 characters).
// Variable labels and the actual group message are counted separately.
const telegramFixedTextBudget = 1024

func usesBuiltInTelegramBody(webhook WebhookConfig) bool {
	return webhook.Service == "telegram" && strings.TrimSpace(webhook.Template) == ""
}

func sizesTelegramDeliveries(webhook WebhookConfig) bool {
	// The URL template can depend on any field of the original group and
	// determine the recipient. Never recalculate it with a different group.
	return usesBuiltInTelegramBody(webhook) && !strings.Contains(webhook.URL, "{{")
}

func telegramTextUnits(s string) int {
	units := 0
	for _, r := range s {
		units += utf16.RuneLen(r)
	}
	return units
}

func telegramDetailsBound(level, kind, name, node string, resolved bool) int {
	n := telegramFixedTextBudget + telegramTextUnits(kind) + telegramTextUnits(name) + telegramTextUnits(node)
	if resolved {
		return n + telegramTextUnits(name) // also in the heading
	}
	return n + telegramTextUnits(level)
}

// telegramAlertBatches keeps alert membership and order, never edits a stored
// alert, and plans one provider attempt per batch. A message that cannot share
// space is a single delivery; the single-alert renderer fits its presentation.
func telegramAlertBatches(list []*alerts.Alert, event notificationEvent) [][]*alerts.Alert {
	if len(list) <= 1 {
		return [][]*alerts.Alert{list}
	}
	for _, alert := range list {
		if alert == nil {
			return [][]*alerts.Alert{list} // leave invalid retained rows to the existing path
		}
	}
	var batches [][]*alerts.Alert
	resolved := event == eventResolved
	for start := 0; start < len(list); {
		leader := list[start]
		units := telegramDetailsBound(string(leader.Level), leader.Type, leader.ResourceName, leader.Node, resolved)
		if resolved {
			units += telegramTextUnits(resolvedAlertListEntry(leader))
		} else {
			units += telegramTextUnits(leader.Message)
		}
		end := start + 1
		for ; end < len(list); end++ {
			var entry string
			if resolved {
				entry = resolvedAlertListEntry(list[end])
			} else {
				entry = groupedWebhookAlertLine(list[end])
			}
			units += telegramTextUnits(entry) + 2 // newline or a semicolon and space
			if units > telegramTextLimit {
				break
			}
		}
		batches = append(batches, list[start:end])
		start = end
	}
	return batches
}

func telegramVisibleUnits(body []byte) (int, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, err
	}
	// Only the fixed Telegram template adds markup. Its literal text may
	// contain "<" or "&" after decoding, so do not strip tags from decoded text.
	units := 0
	for rest := payload.Text; ; {
		literal, tag, found := strings.Cut(rest, "<")
		units += telegramTextUnits(html.UnescapeString(literal))
		if !found {
			return units, nil
		}
		if _, rest, found = strings.Cut(tag, ">"); !found {
			return 0, fmt.Errorf("unterminated built-in Telegram tag")
		}
	}
}

func ellipsizeTelegramText(s string, budget int) string {
	if telegramTextUnits(s) <= budget {
		return s
	}
	if budget < 1 {
		return ""
	}
	units := 0
	for i, r := range s {
		units += utf16.RuneLen(r)
		if units >= budget {
			return s[:i] + "…"
		}
	}
	return s
}

// Render an oversized singleton without changing its occurrence, route, URL
// template inputs, or stored evidence. Never cut rendered HTML or JSON.
func (n *NotificationManager) renderBuiltInTelegram(template string, data WebhookPayloadData, mode webhookRenderMode) ([]byte, error) {
	render := func(values WebhookPayloadData) ([]byte, error) {
		return n.generatePayloadFromTemplateWithService(template, values, "telegram")
	}
	resolved := mode == webhookRenderModeResolved
	if data.AlertCount > 1 ||
		telegramDetailsBound(data.Level, data.Type, data.ResourceName, data.Node, resolved)+telegramTextUnits(data.Message) <= telegramTextLimit {
		return render(data)
	}

	message := data.Message
	data.Message = ""
	// No individual label longer than the whole budget can fit unchanged.
	// Bound the size of the measurement in that case without trimming normal labels.
	data.ResourceName = ellipsizeTelegramText(data.ResourceName, telegramTextLimit)
	data.Node = ellipsizeTelegramText(data.Node, telegramTextLimit)
	data.Type = ellipsizeTelegramText(data.Type, telegramTextLimit)
	data.Level = ellipsizeTelegramText(data.Level, telegramTextLimit)
	measure := func() (int, error) {
		body, err := render(data)
		if err != nil {
			return 0, err
		}
		return telegramVisibleUnits(body)
	}
	fixed, err := measure()
	if err != nil {
		return nil, err
	}
	if fixed >= telegramTextLimit {
		// Pathological labels alone fill the message. Keep identifying
		// prefixes and reserve room for the message and dashboard pointer.
		data.ResourceName = ellipsizeTelegramText(data.ResourceName, 256)
		data.Node = ellipsizeTelegramText(data.Node, 256)
		data.Type = ellipsizeTelegramText(data.Type, 256)
		data.Level = ellipsizeTelegramText(data.Level, 256)
		fixed, err = measure()
		if err != nil {
			return nil, err
		}
		if fixed >= telegramTextLimit {
			return nil, fmt.Errorf("built-in Telegram details exceed the text budget")
		}
	}
	data.Message = ellipsizeTelegramText(message, telegramTextLimit-fixed)
	return render(data)
}
