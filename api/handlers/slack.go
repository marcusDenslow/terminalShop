package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"terminalShop/pkg/notify"
	"terminalShop/pkg/utils"
	"time"
)

type SlackHandler struct {
	signingSecret string
	adminKey      string
	apiPort       string
	httpClient    *http.Client
}

func NewSlackHandler(signingSecret, adminKey, apiPort string) *SlackHandler {
	return &SlackHandler{
		signingSecret: signingSecret,
		adminKey:      adminKey,
		apiPort:       apiPort,
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
	default:
		notify.SlackPostToResponseURL(p.ResponseURL, fmt.Sprintf("Unknown action: %s", action.ActionID))
	}
}

func (h *SlackHandler) handleBuyLabel(orderIDStr string, p slackInteractivePayload) {
	orderID, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		notify.SlackPostToResponseURL(p.ResponseURL, fmt.Sprintf(":x: Invalid order id `%s`", orderIDStr))
		return
	}

	url := fmt.Sprintf("http://localhost:%s/api/v1/orders/%d/label", h.apiPort, orderID)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	req.Header.Set("X-Admin-Key", h.adminKey)

	resp, err := h.httpClient.Do(req)
	if err != nil {
		notify.SlackPostToResponseURL(p.ResponseURL, fmt.Sprintf(":x: Label purchase failed: %v", err))
		return
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			TrackingNumber string `json:"tracking_number"`
			Carrier        string `json:"carrier"`
			LabelURL       string `json:"label_url"`
			CostCents      int `json:"cost_cents"`
		} `json:"data"`
		Error struct {
			Code string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)

	if resp.StatusCode >= 300 {
		msg := fmt.Sprintf(":x: Order #%d label failed: `%s` %s", orderID, result.Error.Code, result.Error.Message)
		notify.SlackPostToResponseURL(p.ResponseURL, msg)
		return
	}

	msg := fmt.Sprintf(
		":white_check_mark: Order #%d labeled — %s `%s` ($%.2f). <%s|Download label PDF>",
		orderID,
		result.Data.Carrier,
		result.Data.TrackingNumber,
		float64(result.Data.CostCents)/100,
		result.Data.LabelURL,
		)
	notify.SlackPostToResponseURL(p.ResponseURL, msg)
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