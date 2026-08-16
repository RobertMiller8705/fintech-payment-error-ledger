package paymenterrors

import (
	"context"
	"encoding/json"
	"testing"
)

type captureSpy struct {
	calls   int
	eventID string
	capture ErrorCapture
}

func (s *captureSpy) CaptureError(_ context.Context, eventID string, capture ErrorCapture) (json.RawMessage, error) {
	s.calls++
	s.eventID = eventID
	s.capture = capture
	return json.RawMessage(`{"event_id":"stored"}`), nil
}

func TestProcessorRiskDecision(t *testing.T) {
	tests := []struct {
		name         string
		event        PaymentEvent
		wantAction   string
		wantCaptures int
	}{
		{"high-risk failure is held", PaymentEvent{"evt-101", "pay-7", "ach", "failed", "account_closed", 91}, "hold_for_review", 1},
		{"low-risk failure can retry", PaymentEvent{"evt-102", "pay-8", "card", "failed", "issuer_declined", 24}, "retry_eligible", 1},
		{"settled payment is recorded", PaymentEvent{"evt-103", "pay-9", "card", "settled", "", 88}, "recorded", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spy := &captureSpy{}
			got, err := (Processor{Errors: spy, ReviewAtScore: 80}).Process(context.Background(), tt.event)
			if err != nil {
				t.Fatal(err)
			}
			if got.Action != tt.wantAction || spy.calls != tt.wantCaptures {
				t.Fatalf("action=%q captures=%d, want %q and %d", got.Action, spy.calls, tt.wantAction, tt.wantCaptures)
			}
			if spy.calls == 1 && (spy.eventID != tt.event.EventID || len(spy.capture.Fingerprint) != 3) {
				t.Fatalf("capture boundary lost event identity: %#v", spy.capture)
			}
		})
	}
}
