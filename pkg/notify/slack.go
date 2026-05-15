package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

var slackClient = &http.Client{Timeout: 5 * time.Second}

type slackMessage struct {
	Channel string       `json:"channel"`
	Text    string       `json:"text"`
	Blocks  []slackBlock `json:"blocks,omitempty"`
}

type slackBlock map[string]any

func SlackOrderPaid(orderID uint, amountCents int) {
	token := os.Getenv("SLACK_BOT_TOKEN")
	channel := os.Getenv("SLACK_CHANNEL")
	if token == "" || channel == "" {
		return
	}

	text := fmt.Sprintf(":coffee: Order #%d paid - $%.2f", orderID, float64(amountCents)/100)
	msg := slackMessage{
		Channel: channel,
		Text:    text,
		Blocks: []slackBlock{
			{
				"type": "section",
				"text": map[string]string{"type": "mrkdwn", "text": text},
			},
			{
				"type": "actions",
				"elements": []slackBlock{
					{
						"type":      "button",
						"text":      map[string]string{"type": "plain_text", "text": "Buy Label"},
						"value":     fmt.Sprintf("%d", orderID),
						"action_id": "buy_label",
						"style":     "primary",
					},
				},
			},
		},
	}

	body, _ := json.Marshal(msg)
	req, _ := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := slackClient.Do(req)
	if err != nil {
		slog.Warn("slack notify failed", "error", err, "order_id", orderID)
		return
	}
	defer resp.Body.Close()

	var parsed struct {
		Ok    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	if !parsed.Ok {
		slog.Warn("slack notify non-ok", "error", parsed.Error, "order_id", orderID)
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
