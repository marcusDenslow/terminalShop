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

func SlackOrderPaid(orderID uint, amountCents int) {
	url := os.Getenv("SLACK_WEBHOOK_URL")
	if url == "" {
		return
	}

	text := fmt.Sprintf(":coffee: Order #%d paid - $%.2f", orderID, float64(amountCents)/100)
	body, _ := json.Marshal(map[string]string{"text": text})

	resp, err := slackClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Warn("slack notify failed", "error", err, "order_id", orderID)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("slack notify non-2xx", "status", resp.StatusCode, "order_id", orderID)
	}
}
