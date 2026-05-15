package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"terminalShop/pkg/models"
)

var slackClient = &http.Client{Timeout: 5 * time.Second}

func dollars(cents int) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100)
}

// SlackOrderPaid posts a receipt-style Block Kit message with item lines,
// totals, shipping address, and a "Buy Label" button. Caller must preload
// order.Items before calling. No-op if SLACK_BOT_TOKEN or SLACK_CHANNEL unset.
func SlackOrderPaid(order *models.Order) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	fallback := fmt.Sprintf(":coffee: Order #%d paid - %s", order.ID, dollars(order.Total))

	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{"type": "plain_text", "text": fallback, "emoji": true},
		},
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

	body, _ := json.Marshal(map[string]any{
		"channel": channel,
		"text":    fallback,
		"blocks":  blocks,
	})

	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := slackClient.Do(req)
	if err != nil {
		slog.Warn("slack notify failed", "error", err, "order_id", order.ID)
		return
	}
	defer resp.Body.Close()

	var parsed struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if !parsed.Ok {
		slog.Warn("slack notify non-ok", "error", parsed.Error, "order_id", order.ID)
	}
}

func SlackPostToResponseURL(responseURL, text string) {
	if responseURL == "" {
		return
	}
	body, _ := json.Marshal(map[string]any{
		"replace_original": false,
		"text":             text,
	})
	resp, err := slackClient.Post(responseURL, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("slack response_url post failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("slack response_url non-2xx", "status", resp.StatusCode)
	}
}

var _ = time.Second
