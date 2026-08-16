package paymenterrors

import (
	"context"
	"encoding/json"
	"fmt"
)

type PaymentEvent struct {
	EventID       string `json:"event_id"`
	PaymentID     string `json:"payment_id"`
	Rail          string `json:"rail"`
	Status        string `json:"status"`
	FailureReason string `json:"failure_reason"`
	RiskScore     int    `json:"risk_score"`
}

type AuditNotification struct {
	EventID   string `json:"event_id"`
	Action    string `json:"action"`
	Captured  bool   `json:"captured"`
	RiskScore int    `json:"risk_score"`
}

type Capturer interface {
	CaptureError(context.Context, string, ErrorCapture) (json.RawMessage, error)
}

type Processor struct {
	Errors        Capturer
	ReviewAtScore int
}

func (p Processor) Process(ctx context.Context, event PaymentEvent) (AuditNotification, error) {
	note := AuditNotification{EventID: event.EventID, Action: "recorded", RiskScore: event.RiskScore}
	if event.Status != "failed" {
		return note, nil
	}

	capture := ErrorCapture{
		Title:       "payment processing failed",
		Message:     event.FailureReason,
		Level:       "error",
		Fingerprint: []string{"payment", event.Rail, event.FailureReason},
		Exception: map[string]any{
			"type": "PaymentFailure", "value": event.FailureReason,
		},
		Context: map[string]any{
			"event_id": event.EventID, "payment_id": event.PaymentID,
			"rail": event.Rail, "risk_score": event.RiskScore,
		},
	}
	if _, err := p.Errors.CaptureError(ctx, event.EventID, capture); err != nil {
		return AuditNotification{}, fmt.Errorf("capture payment event %s: %w", event.EventID, err)
	}
	note.Captured = true
	if event.RiskScore >= p.ReviewAtScore {
		note.Action = "hold_for_review"
	} else {
		note.Action = "retry_eligible"
	}
	return note, nil
}
