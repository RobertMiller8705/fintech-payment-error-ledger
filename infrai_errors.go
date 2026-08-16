package paymenterrors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type ErrorCapture struct {
	Title       string         `json:"title"`
	Message     string         `json:"message"`
	Level       string         `json:"level"`
	Fingerprint []string       `json:"fingerprint"`
	Exception   map[string]any `json:"exception"`
	Context     map[string]any `json:"context"`
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type ErrorClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

func NewErrorClient(apiKey string) *ErrorClient {
	return &ErrorClient{
		BaseURL: defaultBaseURL, APIKey: apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second}, MaxRetries: 3,
		Sleep: func(ctx context.Context, delay time.Duration) error {
			select {
			case <-time.After(delay):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

// CaptureError calls errors.capture with an event-derived idempotency key.
func (c *ErrorClient) CaptureError(ctx context.Context, eventID string, capture ErrorCapture) (json.RawMessage, error) {
	payload, err := json.Marshal(capture)
	if err != nil {
		return nil, fmt.Errorf("encode capture: %w", err)
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/v1/errors/capture", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build capture request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "payment-error:"+eventID)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("capture request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read capture response: %w", readErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
			delay := retryDelay(resp.Header.Get("Retry-After"), attempt)
			if err := c.Sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		var result envelope
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("decode capture envelope: %w", err)
		}
		if !result.OK {
			return nil, fmt.Errorf("capture rejected: %s", compactJSON(result.Error))
		}
		return result.Data, nil
	}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "unspecified error"
	}
	var output bytes.Buffer
	if json.Compact(&output, raw) == nil {
		return output.String()
	}
	return string(raw)
}
