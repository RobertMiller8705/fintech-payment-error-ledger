package paymenterrors

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func TestCaptureRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	var calls int
	var keys []string
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		keys = append(keys, req.Header.Get("Idempotency-Key"))
		if req.Method != http.MethodPost || req.URL.Path != "/v1/errors/capture" {
			t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		if calls == 1 {
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"2"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":{"message":"rate limited"}}`))}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{"event_id":"evt-1"},"metadata":{}}`))}, nil
	})
	client := NewErrorClient("test-key")
	client.BaseURL = "https://example.test"
	client.HTTPClient = &http.Client{Transport: transport}
	var waited time.Duration
	client.Sleep = func(_ context.Context, delay time.Duration) error { waited = delay; return nil }

	_, err := client.CaptureError(context.Background(), "evt-1", ErrorCapture{Exception: map[string]any{"type": "PaymentFailure"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || waited != 2*time.Second || keys[0] != keys[1] || keys[0] != "payment-error:evt-1" {
		t.Fatalf("calls=%d waited=%s keys=%v", calls, waited, keys)
	}
}
