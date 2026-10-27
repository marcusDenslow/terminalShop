package notify

import (
	"testing"

	"terminalShop/pkg/models"
)

// hadActionsBlock reports whether the Block Kit slice contains an "actions"
// block, the row that carries the mark-status buttons.
func hasActionsBlock(blocks []map[string]any) bool {
	for _, b := range blocks {
		if b["type"] == "actions" {
			return true
		}
	}
	return false
}

func TestLabelPurchasedBlocksButtonGating(t *testing.T) {
	cases := []struct {
		name        string
		order       models.Order
		wantButtons bool
	}{
		{"shippo us order with tracking shows buttons", models.Order{ID: 7, Carrier: "USPS", TrackingNumber: "9400111899223817"}, true},
		{"bring orders with tracking shows buttons", models.Order{ID: 8, Carrier: "BRING", TrackingNumber: "CONS123456"}, true},
		{"order without tracking shows no buttons", models.Order{ID: 9, Carrier: "USPS", TrackingNumber: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := labelPurchaseBlocks(&tc.order, "headline")
			if got := hasActionsBlock(blocks); got != tc.wantButtons {
				t.Fatalf("hasActionsBlock = %v, want %v", got, tc.wantButtons)
			}
			if len(blocks) == 0 || blocks[0]["type"] != "section" {
				t.Fatalf("expected first block to be the headline section, got %v", blocks)
			}
		})
	}
}
