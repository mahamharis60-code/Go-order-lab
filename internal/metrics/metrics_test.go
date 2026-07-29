package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestStoreRendersHTTPRequestAndBusinessMetrics(t *testing.T) {
	store := NewStore()

	store.ObserveHTTPRequest("POST", "/api/orders", 202, 120*time.Millisecond)
	store.IncBusinessEvent("activity_order", "accepted")

	output := store.RenderPrometheus()

	checks := []string{
		`go_order_http_requests_total{method="POST",path="/api/orders",status="202"} 1`,
		`go_order_http_request_duration_seconds_count{method="POST",path="/api/orders",status="202"} 1`,
		`go_order_business_events_total{event="activity_order",result="accepted"} 1`,
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("expected metrics output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestStoreEscapesLabelValues(t *testing.T) {
	store := NewStore()

	store.IncBusinessEvent(`bad"event\name`, "ok")

	output := store.RenderPrometheus()
	want := `go_order_business_events_total{event="bad\"event\\name",result="ok"} 1`
	if !strings.Contains(output, want) {
		t.Fatalf("expected escaped label value %q, got:\n%s", want, output)
	}
}
