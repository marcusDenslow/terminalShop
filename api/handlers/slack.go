package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"terminalShop/pkg/audit"
	"terminalShop/pkg/database"
	"terminalShop/pkg/models"
	"terminalShop/pkg/notify"
	"terminalShop/pkg/utils"
	"time"
)

type SlackHandler struct {
	signingSecret string
	adminKey      string
	apiPort       string
	shippoSecret  string
	httpClient    *http.Client
}

func NewSlackHandler(signingSecret, adminKey, apiPort, shippoWebhookSecret string) *SlackHandler {
	return &SlackHandler{
		signingSecret: signingSecret,
		adminKey:      adminKey,
		apiPort:       apiPort,
		shippoSecret:  shippoWebhookSecret,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

type slackAction struct {
	ActionID string `json:"action_id"`
	Value    string `json:"value"`
}

type slackInteractivePayload struct {
	Type        string        `json:"type"`
	ResponseURL string        `json:"response_url"`
	Actions     []slackAction `json:"actions"`
	User        struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
}

func (h *SlackHandler) HandleInteractivity(w http.ResponseWriter, r *http.Request) {
	if h.signingSecret == "" {
		utils.RespondError(w, http.StatusServiceUnavailable, "SLACK_NOT_CONFIGURED", "slack signing secret not set", nil)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "READ_FAILED", "could not read body", nil)
		return
	}

	if !h.verifySignature(r, body) {
		utils.RespondError(w, http.StatusUnauthorized, "INVALID_SIGNATURE", "slack signature mismatch", nil)
		return
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "BAD_FORM", "could not parse form body", nil)
		return
	}
	rawPayload := form.Get("payload")
	if rawPayload == "" {
		utils.RespondError(w, http.StatusBadRequest, "MISSING_PAYLOAD", "no payload field", nil)
		return
	}

	var p slackInteractivePayload
	if err := json.Unmarshal([]byte(rawPayload), &p); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "BAD_JSON_PAYLOAD", "could not parse payload", nil)
		return
	}

	if len(p.Actions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}
	action := p.Actions[0]

	w.WriteHeader(http.StatusOK)

	go h.dispatch(action, p)
}

func (h *SlackHandler) dispatch(action slackAction, p slackInteractivePayload) {
	switch action.ActionID {
	case "buy_label":
		h.handleBuyLabel(action.Value, p)
	case "mark_pre_transit":
		h.handleMarkStatus(action.Value, p, "PRE_TRANSIT")
	case "mark_in_transit":
		h.handleMarkStatus(action.Value, p, "TRANSIT")
	case "mark_delivered":
		h.handleMarkStatus(action.Value, p, "DELIVERED")
	default:
		slog.Warn("slack interactivity: unknown action", "action_id", action.ActionID)
	}
}

func (h *SlackHandler) handleBuyLabel(orderIDStr string, p slackInteractivePayload) {
	orderID64, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		slog.Warn("slack buy_label: invalid order id", "value", orderIDStr)
		return
	}
	orderID := uint(orderID64)

	url := fmt.Sprintf("http://localhost:%s/api/v1/orders/%d/label", h.apiPort, orderID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	req.Header.Set("X-Admin-Key", h.adminKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		notify.SlackPostToOrderThread(orderID, fmt.Sprintf(":x: Label purchase failed: %v", err))
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TrackingNumber string `json:"tracking_number"`
			Carrier        string `json:"carrier"`
			LabelURL       string `json:"label_url"`
			CostCents      int    `json:"cost_cents"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)

	if resp.StatusCode >= 300 {
		notify.SlackPostToOrderThread(orderID, fmt.Sprintf(":x: Label purchase failed: `%s` %s", result.Error.Code, result.Error.Message))
		return
	}

	notify.SlackPostLabelPurchased(orderID, result.Data.LabelURL, result.Data.CostCents)

}

func (h *SlackHandler) handleMarkStatus(orderIDStr string, p slackInteractivePayload, status string) {
	orderID64, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		slog.Warn("slack mark_status: invalid order id", "value", orderIDStr)
		return
	}
	orderID := uint(orderID64)

	var order models.Order
	if err := database.GetDB().Select("id", "tracking_number", "carrier").Where("id = ?", orderID).First(&order).Error; err != nil {
		slog.Warn("slack mark_status: order lookup failed", "order_id", orderID, "error", err)
		return
	}
	if order.TrackingNumber == "" {
		notify.SlackPostToOrderThread(orderID, ":x: Cannot mark status, the order has no tracking number yet.")
		return
	}

	detail := defaultDetailFor(status)
	payload := map[string]any{
		"event": "track_updated",
		"data": map[string]any{
			"tracking_number": order.TrackingNumber,
			"carrier":         strings.ToLower(order.Carrier),
			"tracking_status": map[string]any{
				"status":         status,
				"status_date":    time.Now().UTC().Format(time.RFC3339),
				"status_details": detail,
			},
		},
	}
	body, _ := json.Marshal(payload)

	webhookURL := fmt.Sprintf("http://localhost:%s/api/v1/webhooks/shippo?token=%s", h.apiPort, url.QueryEscape(h.shippoSecret))
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		notify.SlackPostToOrderThread(orderID, fmt.Sprintf(":x: Mark %s failed: %v", status, err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		notify.SlackPostToOrderThread(orderID, fmt.Sprintf(":x: Mark %s failed (HTTP %d)", status, resp.StatusCode))
		return
	}

	audit.TrackingMarkedManually(orderID, status, p.User.Username)

	h.editClearActions(p.ResponseURL, status, p.User.Username)
}

func defaultDetailFor(status string) string {
	switch status {
	case "PRE_TRANSIT":
		return "Marked pre-transit by operator"
	case "TRANSIT":
		return "Marked transit by operator"
	case "DELIVERED":
		return "Marked delivered by operator"
	}
	return "Marked by operator"
}

func (h *SlackHandler) editClearActions(responseURL, status, actor string) {
	if responseURL == "" {
		return
	}
	stamp := fmt.Sprintf(":white_check_mark: Marked %s by @%s at %s", strings.ToLower(status), actor, time.Now().UTC().Format("15:04 MST"))
	body, _ := json.Marshal(map[string]any{
		"replace_original": true,
		"text":             stamp,
		"blocks": []map[string]any{
			{"type": "context", "elements": []map[string]any{
				{"type": "mrkdwn", "text": stamp},
			}},
		},
	})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, responseURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		slog.Warn("slack mark_status: response_url edit failed", "error", err)
		return
	}
	resp.Body.Close()
}

func (h *SlackHandler) verifySignature(r *http.Request, body []byte) bool {
	tsHeader := r.Header.Get("X-Slack-Request-Timestamp")
	sigHeader := r.Header.Get("X-Slack-Signature")
	if tsHeader == "" || sigHeader == "" {
		return false
	}

	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return false
	}
	if abs(time.Now().Unix()-ts) > 60*5 {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.signingSecret))
	fmt.Fprintf(mac, "v0:%d:%s", ts, body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

var _ = os.Getenv
