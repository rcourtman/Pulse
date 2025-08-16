#!/bin/bash

# Test Pulse webhook endpoint
echo "Testing Pulse Webhook..."

# You'll need to be authenticated - using the API token
API_TOKEN="2674a96979fdcda0367e10b808443928b410b4c094c964f7"

# Test the webhook (adjust the webhook ID if needed)
curl -X POST http://localhost:7655/api/notifications/test \
  -H "X-API-Token: $API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "method": "webhook",
    "webhookId": 1
  }'

echo ""
echo "Check your Telegram for the test message!"