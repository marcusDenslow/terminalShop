package shipping

type LabelResult struct {
	TransactionID  string
	RateID         string
	Carrier        string
	ServiceLevel   string
	TrackingNumber string
	TrackingURL    string
	LabelURL       string
	CostCents      int
}
