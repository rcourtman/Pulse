#!/bin/bash

# Automated Telegram Bot Setup for Pulse
# This script helps create and configure a Telegram bot

set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}==================================="
echo "Telegram Bot Auto-Setup for Pulse"
echo "===================================${NC}"
echo ""

# Check if we have a saved bot token
TOKEN_FILE="/opt/pulse/.telegram_bot_token"
CHAT_FILE="/opt/pulse/.telegram_chat_id"

if [ -f "$TOKEN_FILE" ]; then
    echo -e "${YELLOW}Found existing bot token${NC}"
    BOT_TOKEN=$(cat "$TOKEN_FILE")
    echo "Token: ${BOT_TOKEN:0:10}...${BOT_TOKEN: -5}"
else
    echo -e "${YELLOW}No existing bot token found${NC}"
    echo ""
    echo "To create a new bot automatically, I need you to:"
    echo "1. Open Telegram"
    echo "2. Search for @BotFather"
    echo "3. Send: /newbot"
    echo "4. Choose a name (e.g., 'Pulse Monitor')"
    echo "5. Choose a username ending in 'bot' (e.g., 'PulseMonitor_bot')"
    echo ""
    read -p "Enter the bot token from BotFather: " BOT_TOKEN
    
    # Save the token
    echo "$BOT_TOKEN" > "$TOKEN_FILE"
    chmod 600 "$TOKEN_FILE"
    echo -e "${GREEN}✓ Bot token saved${NC}"
fi

# Test the bot token
echo ""
echo "Testing bot token..."
API_RESPONSE=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getMe")

if echo "$API_RESPONSE" | grep -q '"ok":true'; then
    BOT_USERNAME=$(echo "$API_RESPONSE" | grep -o '"username":"[^"]*' | cut -d'"' -f4)
    BOT_NAME=$(echo "$API_RESPONSE" | grep -o '"first_name":"[^"]*' | cut -d'"' -f4)
    echo -e "${GREEN}✓ Bot verified: $BOT_NAME (@$BOT_USERNAME)${NC}"
else
    echo -e "${RED}✗ Invalid bot token!${NC}"
    rm -f "$TOKEN_FILE"
    exit 1
fi

# Get or find chat ID
if [ -f "$CHAT_FILE" ]; then
    echo -e "${YELLOW}Found existing chat ID${NC}"
    CHAT_ID=$(cat "$CHAT_FILE")
    echo "Chat ID: $CHAT_ID"
else
    echo ""
    echo -e "${YELLOW}Getting your chat ID...${NC}"
    echo ""
    echo "Please do the following NOW:"
    echo "1. Open Telegram"
    echo "2. Search for: @$BOT_USERNAME"
    echo "3. Click 'Start' or send any message"
    echo ""
    read -p "Press ENTER after you've messaged the bot..."
    
    # Get updates
    UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates")
    
    # Extract chat IDs
    CHAT_IDS=$(echo "$UPDATES" | grep -o '"chat":{"id":[0-9-]*' | grep -o '[0-9-]*' | sort -u)
    
    if [ -z "$CHAT_IDS" ]; then
        echo -e "${RED}No messages found. Trying alternative method...${NC}"
        
        # Try with offset
        UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates?offset=-1")
        CHAT_IDS=$(echo "$UPDATES" | grep -o '"chat":{"id":[0-9-]*' | grep -o '[0-9-]*' | sort -u)
    fi
    
    if [ -z "$CHAT_IDS" ]; then
        echo -e "${RED}Still no chat ID found!${NC}"
        echo "Manual steps:"
        echo "1. Open: https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"
        echo "2. Look for 'chat' -> 'id' in the JSON"
        echo "3. Enter it manually"
        read -p "Enter your chat ID: " CHAT_ID
    else
        CHAT_ID=$(echo "$CHAT_IDS" | head -n1)
        echo -e "${GREEN}✓ Found chat ID: $CHAT_ID${NC}"
    fi
    
    # Save chat ID
    echo "$CHAT_ID" > "$CHAT_FILE"
    chmod 600 "$CHAT_FILE"
fi

# Send test message
echo ""
echo "Sending test message..."

TEST_MESSAGE='{
    "chat_id": "'$CHAT_ID'",
    "text": "🎉 *Pulse Integration Successful!*\n\nYour Telegram bot is now connected to Pulse monitoring.\n\n✅ Bot: @'$BOT_USERNAME'\n✅ Chat ID: '$CHAT_ID'\n✅ Status: Ready\n\nYou will receive alerts here when thresholds are triggered.",
    "parse_mode": "Markdown"
}'

TEST_RESULT=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage" \
    -H "Content-Type: application/json" \
    -d "$TEST_MESSAGE")

if echo "$TEST_RESULT" | grep -q '"ok":true'; then
    echo -e "${GREEN}✓ Test message sent!${NC}"
else
    echo -e "${RED}✗ Failed to send test message${NC}"
    echo "$TEST_RESULT"
fi

# Configure webhook in Pulse
echo ""
echo -e "${GREEN}==================================="
echo "Configuring Pulse Webhook"
echo "===================================${NC}"

# Create webhook configuration
WEBHOOK_CONFIG=$(cat << EOF
{
    "name": "Telegram Alerts",
    "enabled": true,
    "url": "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage",
    "method": "POST",
    "payloadTemplate": {
        "chat_id": "${CHAT_ID}",
        "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%\\n• Duration: {{.Duration}}",
        "parse_mode": "Markdown",
        "disable_web_page_preview": true
    }
}
EOF
)

# Save webhook config
echo "$WEBHOOK_CONFIG" > /opt/pulse/telegram-webhook.json

echo ""
echo -e "${GREEN}==================================="
echo "SETUP COMPLETE!"
echo "===================================${NC}"
echo ""
echo "Webhook URL for Pulse:"
echo -e "${YELLOW}https://api.telegram.org/bot${BOT_TOKEN}/sendMessage${NC}"
echo ""
echo "Custom Payload (copy this exactly):"
cat << EOF
{
    "chat_id": "${CHAT_ID}",
    "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%",
    "parse_mode": "Markdown"
}
EOF
echo ""
echo -e "${GREEN}Files saved:${NC}"
echo "• Bot token: $TOKEN_FILE"
echo "• Chat ID: $CHAT_FILE"
echo "• Webhook config: /opt/pulse/telegram-webhook.json"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "1. Go to Pulse web interface"
echo "2. Navigate to Alerts > Webhooks"
echo "3. Add new webhook with the configuration above"
echo "4. Enable the webhook"
echo "5. Set up alert thresholds to trigger notifications"