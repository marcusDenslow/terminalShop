// Package notify provides a simple interface to send notifications to Slack.
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
)

var slackClient = &http.Client{Timeout: 5 * time.Second}

// dollars converts cents to dollars
func dollars(cents int) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100)
}

// postSlackMessage POSTs to chat.postMessage and returns (ts, ok). Logs on failure.
func postSlackMessage(token string, payload map[string]any) (string, bool) {
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := slackClient.Do(req)
	if err != nil {
		slog.Warn("slack post failed", "error", err)
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed struct {
		Ok    bool   `json:"ok"`
		TS    string `json:"ts"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if !parsed.Ok {
		slog.Warn("slack post non-ok", "error", parsed.Error)
		return "", false
	}
	return parsed.TS, true
}

func pinMessage(token, channel, ts string) bool {
	return slackPinCall(token, "https://slack.com/api/pins.add", channel, ts, "already_pinned")
}

func unpinMessage(token, channel, ts string) bool {
	return slackPinCall(token, "https://slack.com/api/pins.remove", channel, ts, "no_pin")
}

func slackPinCall(token, endpoint, channel, ts, benignErr string) bool {
	body, _ := json.Marshal(map[string]any{"channel": channel, "timestamp": ts})
	req, _ := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := slackClient.Do(req)
	if err != nil {
		slog.Warn("slack pin call failed", "endpoint", endpoint, "error", err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	var parsed struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if !parsed.Ok && parsed.Error != benignErr {
		slog.Warn("slack pin call non-ok", "endpoint", endpoint, "error", parsed.Error)
		return false
	}
	return true
}

func buildReceiptBlocks(order *models.Order) []map[string]any {
	blocks := []map[string]any{
		{"type": "divider"},
	}
	for _, item := range order.Items {
		line := fmt.Sprintf("*%s* × %d\n%s", item.Name, item.Quantity, dollars(item.Price*item.Quantity))
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": line},
		})
	}
	blocks = append(blocks,
		map[string]any{"type": "divider"},
		map[string]any{
			"type": "section",
			"fields": []map[string]any{
				{"type": "mrkdwn", "text": "*Subtotal:*\n" + dollars(order.Subtotal)},
				{"type": "mrkdwn", "text": "*Shipping:*\n" + dollars(order.ShippingCost)},
				{"type": "mrkdwn", "text": "*Total:*\n" + dollars(order.Total)},
			},
		},
		map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf(":package: *Ship to:*\n%s\n%s\n%s, %s %s",
					order.ShippingName,
					order.ShippingStreet,
					order.ShippingCity,
					order.ShippingZip,
					order.ShippingCountry,
				),
			},
		},
		map[string]any{
			"type": "actions",
			"elements": []map[string]any{
				{
					"type":      "button",
					"text":      map[string]any{"type": "plain_text", "text": "Buy Label", "emoji": true},
					"value":     fmt.Sprintf("%d", order.ID),
					"action_id": "buy_label",
					"style":     "primary",
				},
			},
		},
	)
	return blocks
}

// SlackOrderPaid posts a compact parent message ("Order #N paid - $X.XX") to
// the orders channel, persists the resulting ts onto the order, and posts the
// full receipt as the first thread reply. All subsequent order events
// (label-buy, tracking updates) should be posted as further thread replies
// via SlackPostToOrderThread. No-op if SLACK_BOT_TOKEN or SLACK_CHANNEL unset.
func SlackOrderPaid(order *models.Order) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	parentText := fmt.Sprintf(":coffee: Order #%d paid - %s", order.ID, dollars(order.Total))
	ts, ok := postSlackMessage(token, map[string]any{
		"channel": channel,
		"text":    parentText,
	})
	if !ok {
		return
	}

	if err := database.GetDB().Model(&models.Order{}).Where("id = ?", order.ID).Update("slack_thread_ts", ts).Error; err != nil {
		slog.Warn("failed to save slack thread ts", "order_id", order.ID, "error", err)
	}

	pinMessage(token, channel, ts)

	_, _ = postSlackMessage(token, map[string]any{
		"channel":   channel,
		"thread_ts": ts,
		"text":      parentText,
		"blocks":    buildReceiptBlocks(order),
	})
}

func SlackUnpinOrder(orderID uint) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	var order models.Order
	if err := database.GetDB().Select("id", "slack_thread_ts").Where("id = ?", orderID).First(&order).Error; err != nil {
		slog.Warn("failed to look up order for unpin", "order_id", orderID, "error", err)
		return
	}
	if order.SlackThreadTS == nil || *order.SlackThreadTS == "" {
		return
	}

	unpinMessage(token, channel, *order.SlackThreadTS)
}

// SlackPostToOrderThread posts a plain-text reply inside the order's thread.
// Looks up the thread ts from the database. Silent no-op if the order has no
// saved thread (Slack was disabled when it was created, or post failed earlier).
func SlackPostToOrderThread(orderID uint, text string) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	var order models.Order
	if err := database.GetDB().Select("id", "slack_thread_ts").Where("id = ?", orderID).First(&order).Error; err != nil {
		slog.Warn("failed to look up order for thread post", "order_id", orderID, "error", err)
		return
	}
	if order.SlackThreadTS == nil || *order.SlackThreadTS == "" {
		return
	}

	_, _ = postSlackMessage(token, map[string]any{
		"channel":   channel,
		"thread_ts": *order.SlackThreadTS,
		"text":      text,
	})
}

func SlackPostLabelPurchased(orderID uint, labelURL string, costCents int) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	var order models.Order
	if err := database.GetDB().
		Select("id", "carrier", "tracking_number", "tracking_url", "slack_thread_ts").
		Where("id = ?", orderID).First(&order).Error; err != nil {
		slog.Warn("failed to look up order for label-purchased slack post", "order_id", orderID, "error", err)
		return
	}
	if order.SlackThreadTS == nil || *order.SlackThreadTS == "" {
		return
	}

	headLine := fmt.Sprintf(":white_check_mark: Labeled - %s `%s` (%s). <%s|Download label PDF>", order.Carrier, order.TrackingNumber, dollars(costCents), labelURL)

	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": headLine},
		},
	}

	if strings.EqualFold(order.Carrier, "BRING") {
		blocks = append(blocks, markStatusActionBlock(order.ID))
	}

	_, _ = postSlackMessage(token, map[string]any{
		"channel":   channel,
		"thread_ts": *order.SlackThreadTS,
		"text":      headLine,
		"blocks":    blocks,
	})
}

func markStatusActionBlock(orderID uint) map[string]any {
	id := fmt.Sprintf("%d", orderID)
	return map[string]any{
		"type": "actions",
		"elements": []map[string]any{
			{"type": "button", "text": map[string]any{"type": "plain_text", "text": "Mark Pre-Transit", "emoji": true}, "value": id, "action_id": "mark_pre_transit"},
			{"type": "button", "text": map[string]any{"type": "plain_text", "text": "Mark In Transit", "emoji": true}, "value": id, "action_id": "mark_in_transit", "style": "primary"},
			{"type": "button", "text": map[string]any{"type": "plain_text", "text": "Mark Delivered", "emoji": true}, "value": id, "action_id": "mark_delivered"},
		},
	}
}

var _ = time.Second
