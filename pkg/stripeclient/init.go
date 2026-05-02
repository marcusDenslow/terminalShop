package stripeclient

import (
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v78"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// InitOTel wires the Stripe SDK to use an HTTP client whose transport emits
// OpenTelemetry client spans. Call once at startup, after tracing is initialized.
func InitOTel() {
	httpClient := &http.Client{
		Timeout:   80 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	backends := stripe.NewBackends(httpClient)
	stripe.SetBackend(stripe.APIBackend, backends.API)
	stripe.SetBackend(stripe.UploadsBackend, backends.Uploads)
	stripe.SetBackend(stripe.ConnectBackend, backends.Connect)
}
