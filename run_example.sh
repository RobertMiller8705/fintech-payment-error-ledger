#!/bin/sh
set -eu

curl --fail-with-body --silent --show-error \
  --request POST http://localhost:8080/payment-events \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"evt-101","payment_id":"pay-7","rail":"ach","status":"failed","failure_reason":"account_closed","risk_score":91}'
