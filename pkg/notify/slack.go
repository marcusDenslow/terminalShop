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

// slack client
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
// It looks up the thread ts from the database and returns an error when the
// reply cannot be posted.
func SlackPostToOrderThread(orderID uint, text string) error {
	return slackPostToOrderThread(orderID, text, false)
}

// SlackPostToOrderThreadBroadcast posts a thread reply that is also surfaced
// to the channel (reply_broadcast=true) so channel members get a normal
// notification. Use for messages that need immediate operator attention.
func SlackPostToOrderThreadBroadcast(orderID uint, text string) error {
	return slackPostToOrderThread(orderID, text, true)
}

func slackPostToOrderThread(orderID uint, text string, broadcast bool) error {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return fmt.Errorf("slack is not configured")
	}

	var order models.Order
	if err := database.GetDB().Select("id", "slack_thread_ts").Where("id = ?", orderID).First(&order).Error; err != nil {
		slog.Warn("failed to look up order for thread post", "order_id", orderID, "error", err)
		return fmt.Errorf("failed to look up order thread: %w", err)
	}
	if order.SlackThreadTS == nil || *order.SlackThreadTS == "" {
		return fmt.Errorf("order has no slack thread")
	}

	payload := map[string]any{
		"channel":   channel,
		"thread_ts": *order.SlackThreadTS,
		"text":      text,
	}
	if broadcast {
		payload["reply_broadcast"] = true
	}

	if _, ok := postSlackMessage(token, payload); !ok {
		slog.Warn("failed to post slack thread reply", "order_id", orderID)
		return fmt.Errorf("failed to post slack thread reply")
	}
	return nil
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

	_, _ = postSlackMessage(token, map[string]any{
		"channel":   channel,
		"thread_ts": *order.SlackThreadTS,
		"text":      headLine,
		"blocks":    labelPurchaseBlocks(&order, headLine),
	})
}

// labelPurchaseBlocks assembles the Block Kit payload for the post-label thread
// reply: a headline section plus the manual mark-status buttons for any labeled
// order. The buttons are gtated on a present tracking number (carrier-agnostic)
func labelPurchaseBlocks(order *models.Order, headline string) []map[string]any {
	blocks := []map[string]any{
		{
			"type": "section",
			"text": map[string]any{"type": "mrkdwn", "text": headline},
		},
	}
	if order.TrackingNumber != "" {
		blocks = append(blocks, markStatusActionBlock(order.ID))
	}
	return blocks
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

func SlackUnshippedReminder(orders []models.Order) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" || len(orders) == 0 {
		return
	}

	workspace := os.Getenv("SLACK_WORKSPACE_SUBDOMAIN")

	lines := []string{fmt.Sprintf(":hourglass_flowing_sand: %d order(s) awaiting label", len(orders))}
	for _, order := range orders {
		lines = append(lines, " "+unshippedOrderLine(order, channel, workspace))
	}

	_, _ = postSlackMessage(token, map[string]any{
		"channel": channel,
		"text":    strings.Join(lines, "\n"),
	})
}

func unshippedOrderLine(order models.Order, channel, workspace string) string {
	age := time.Since(order.CreatedAt).Round(time.Hour)
	location := order.ShippingCountry
	if order.ShippingCity != "" {
		location = order.ShippingCity + ", " + order.ShippingCountry
	}

	label := fmt.Sprintf("Order #%d", order.ID)
	if order.SlackThreadTS != nil && *order.SlackThreadTS != "" && workspace != "" {
		ts := strings.ReplaceAll(*order.SlackThreadTS, ".", "")
		permalink := fmt.Sprintf("https://%s.slack.com/archives/%s/p%s", workspace, channel, ts)
		label = fmt.Sprintf("<%s|Order #%d>", permalink, order.ID)
	}

	tag := ""
	if order.ShippingCountry == "NO" {
		tag = " _(NO * Bring blocked)_"
	}

	return fmt.Sprintf("%s * %s old * %s * %s%s", label, age, dollars(order.Total), location, tag)
}
