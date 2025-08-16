#!/bin/bash

# Telegram Webhook Test Script for Pulse
# This helps you test your Telegram bot setup

echo "==================================="
echo "Telegram Webhook Setup for Pulse"
echo "==================================="
echo ""

# Get bot token
read -p "Enter your Bot Token (from BotFather): " BOT_TOKEN

# Get chat ID
echo ""
echo "Getting updates to find your chat ID..."
echo "Make sure you've sent a message to your bot first!"
echo ""

UPDATES_URL="https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"
echo "Fetching from: $UPDATES_URL"
echo ""

# Get updates and pretty print
RESPONSE=$(curl -s "$UPDATES_URL")

if echo "$RESPONSE" | grep -q '"ok":true'; then
    echo "✅ Bot token is valid!"
    echo ""
    
    # Try to extract chat IDs
    CHAT_IDS=$(echo "$RESPONSE" | grep -o '"chat":{"id":[0-9-]*' | grep -o '[0-9-]*' | sort -u)
    
    if [ -z "$CHAT_IDS" ]; then
        echo "❌ No chat IDs found. Please send a message to your bot and try again."
        echo ""
        echo "Instructions:"
        echo "1. Open Telegram"
        echo "2. Search for your bot by username"
        echo "3. Start a conversation and send any message"
        echo "4. Run this script again"
    else
        echo "Found chat ID(s):"
        echo "$CHAT_IDS"
        echo ""
        
        # Use the first chat ID
        CHAT_ID=$(echo "$CHAT_IDS" | head -n1)
        echo "Using Chat ID: $CHAT_ID"
        echo ""
        
        # Test sending a message
        echo "Testing webhook..."
        TEST_URL="https://api.telegram.org/bot${BOT_TOKEN}/sendMessage"
        
        TEST_PAYLOAD='{
            "chat_id": "'$CHAT_ID'",
            "text": "🎉 *Pulse Telegram Integration Test*\n\nIf you see this message, your Telegram webhook is working!\n\n✅ Bot Token: Valid\n✅ Chat ID: '$CHAT_ID'\n✅ Connection: Working\n\nYou can now configure this in Pulse.",
            "parse_mode": "Markdown"
        }'
        
        TEST_RESPONSE=$(curl -s -X POST "$TEST_URL" \
            -H "Content-Type: application/json" \
            -d "$TEST_PAYLOAD")
        
        if echo "$TEST_RESPONSE" | grep -q '"ok":true'; then
            echo "✅ Test message sent successfully!"
            echo ""
            echo "==================================="
            echo "CONFIGURATION FOR PULSE:"
            echo "==================================="
            echo ""
            echo "Service Type: Telegram"
            echo "Webhook URL: https://api.telegram.org/bot${BOT_TOKEN}/sendMessage"
            echo ""
            echo "Custom Payload Template:"
            cat << 'EOF'
{
    "chat_id": "CHAT_ID_HERE",
    "text": "🚨 *Pulse Alert: {{.Level}}*\n\n{{.Message}}\n\n📊 *Details:*\n• Resource: {{.ResourceName}}\n• Node: {{.Node}}\n• Value: {{.Value}}%\n• Threshold: {{.Threshold}}%",
    "parse_mode": "Markdown"
}
EOF
            echo ""
            echo "IMPORTANT: Replace CHAT_ID_HERE with: $CHAT_ID"
            echo ""
            echo "==================================="
        else
            echo "❌ Failed to send test message"
            echo "Response: $TEST_RESPONSE"
        fi
    fi
else
    echo "❌ Invalid bot token or API error"
    echo "Response: $RESPONSE"
fi