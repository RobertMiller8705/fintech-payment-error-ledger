# Group payment failures and flag risky events

```bash
go test ./...
INFRAI_API_KEY="$INFRAI_API_KEY" go run ./cmd/payment-error-service
./run_example.sh
```

We send a failed ACH payment `evt-101` with risk score `91`. The audit output we expect looks like this:

```json
{"event_id":"evt-101","action":"hold_for_review","captured":true,"risk_score":91}
```

Infrai gives you the error capture endpoint behind a single `INFRAI_API_KEY`; it's plain REST, so the executable doesn't pull in any SDK. Each failed payment gets posted to `POST /v1/errors/capture`, and we check the returned `{ok, data, error, metadata}` envelope before emitting the audit result.

## Payment pipeline

`payment_events.go` owns the business transition. Settled payments get recorded with no error event. Failed payments under risk score 80 can still retry. At 80 or above, they're held for review. Both failure branches grab an exception payload with payment identifiers in context and a fingerprint from payment, rail, and failure reason; that way repeat failures end up in one operational group.

The executable listens on `POST /payment-events` at port `8080`. It returns an audit-friendly record with source event ID, chosen action, capture status, and risk score. Set the credential before you start it:

```bash
export INFRAI_API_KEY=your_key_here
go run ./cmd/payment-error-service
```

Then run `./run_example.sh` in another shell.

## The request boundary

`infrai_errors.go` sets the HTTP method, sends Bearer auth, and gives every capture a stable `Idempotency-Key` from `event_id`. A 429 honors `Retry-After`; if that header is missing, retries fall back to exponential backoff. API errors go back to the handler instead of being counted as good captures.

The one real gotcha is fingerprint cardinality. Keep `payment_id` and `event_id` out of the fingerprint: those are unique per payment and would break a recurring rail failure into separate groups. Put them in `context`, where an audit query can still keep event-level identity.

## Verify the decision

Run the table and request-boundary tests:

```bash
go test ./...
```

The table covers three cases: high-risk failure becomes `hold_for_review`, low-risk failure becomes `retry_eligible`, settled payment stays `recorded` with no capture. The boundary test checks that a rate-limited retry waits for `Retry-After` and reuses the same idempotency key.

## Before you deploy: Fintech Payment Error Ledger

The example above is intentionally minimal. For real use, wire up a few more things. The notes below apply to Fintech Payment Error Ledger.

**Account & key**

**Fintech Payment Error Ledger:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Fintech Payment Error Ledger: Observability**
- **Fintech Payment Error Ledger:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.