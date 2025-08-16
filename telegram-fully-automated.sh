#!/bin/bash

# Fully Automated Telegram Bot Setup
# This does EVERYTHING possible without manual intervention

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================"
echo "Fully Automated Telegram Setup"
echo "======================================${NC}"
echo ""

# Generate unique bot username
TIMESTAMP=$(date +%s)
BOT_NAME="Pulse Monitor Alert Bot"
BOT_USERNAME="PulseMonitor_${TIMESTAMP}_bot"

echo -e "${YELLOW}Unfortunately, Telegram requires manual bot creation for security.${NC}"
echo -e "${YELLOW}But I'll make it as easy as possible!${NC}"
echo ""
echo -e "${GREEN}Here's a one-click solution:${NC}"
echo ""

# Create a pre-filled URL for BotFather
echo "1. Click this link to open BotFather:"
echo -e "${BLUE}https://t.me/BotFather${NC}"
echo ""
echo "2. Copy and paste these THREE messages quickly:"
echo ""
echo -e "${GREEN}/newbot${NC}"
echo -e "${GREEN}${BOT_NAME}${NC}"
echo -e "${GREEN}${BOT_USERNAME}${NC}"
echo ""
echo "3. BotFather will respond with a token. It looks like:"
echo "   1234567890:ABCdefGHIjklMNOpqrsTUVwxyz"
echo ""

# Wait for token
read -p "Paste your bot token here: " BOT_TOKEN

# Verify the token immediately
echo ""
echo "Verifying token..."
RESPONSE=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getMe")

if ! echo "$RESPONSE" | grep -q '"ok":true'; then
    echo -e "${RED}Invalid token! Please check and try again.${NC}"
    exit 1
fi

BOT_INFO=$(echo "$RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data['result']['username'])")
echo -e "${GREEN}✓ Bot created successfully: @${BOT_INFO}${NC}"

# Save token
mkdir -p /opt/pulse/.telegram
echo "$BOT_TOKEN" > /opt/pulse/.telegram/bot_token
chmod 600 /opt/pulse/.telegram/bot_token

# Now get chat ID
echo ""
echo -e "${YELLOW}Step 2: Getting your Chat ID${NC}"
echo ""
echo "Click this link to message your bot:"
echo -e "${BLUE}https://t.me/${BOT_INFO}${NC}"
echo ""
echo "Send the message: /start"
echo ""
read -p "Press ENTER after sending /start to your bot..."

# Get chat ID
UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates")
CHAT_ID=$(echo "$UPDATES" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if data['ok'] and data['result']:
        for update in data['result']:
            if 'message' in update:
                print(update['message']['chat']['id'])
                break
except: pass
")

if [ -z "$CHAT_ID" ]; then
    echo -e "${YELLOW}Waiting for your message...${NC}"
    sleep 3
    UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates")
    CHAT_ID=$(echo "$UPDATES" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)
    if data['ok'] and data['result']:
        for update in data['result']:
            if 'message' in update:
                print(update['message']['chat']['id'])
                break
except: pass
")
fi

if [ -z "$CHAT_ID" ]; then
    echo -e "${RED}No message found yet.${NC}"
    echo "Please make sure you sent /start to @${BOT_INFO}"
    echo ""
    echo "Manual check URL:"
    echo "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"
    echo ""
    read -p "Enter your chat ID manually: " CHAT_ID
else
    echo -e "${GREEN}✓ Found your chat ID: ${CHAT_ID}${NC}"
fi

# Save chat ID
echo "$CHAT_ID" > /opt/pulse/.telegram/chat_id
chmod 600 /opt/pulse/.telegram/chat_id

# Send test message
echo ""
echo "Sending test message..."
TEST_MSG=$(cat <<EOF
{
    "chat_id": "$CHAT_ID",
    "text": "🎉 *Pulse Telegram Integration Complete!*\n\nYour alerts will appear here.\n\n✅ Bot: @${BOT_INFO}\n✅ Chat ID: ${CHAT_ID}\n✅ Status: Ready",
    "parse_mode": "Markdown"
}
EOF
)

SEND_RESULT=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage" \
    -H "Content-Type: application/json" \
    -d "$TEST_MSG")

if echo "$SEND_RESULT" | grep -q '"ok":true'; then
    echo -e "${GREEN}✓ Test message sent! Check Telegram${NC}"
else
    echo -e "${RED}Failed to send test message${NC}"
fi

# Create Pulse webhook configuration
echo ""
echo -e "${BLUE}======================================"
echo "Pulse Webhook Configuration"
echo "======================================${NC}"
echo ""

WEBHOOK_URL="https://api.telegram.org/bot${BOT_TOKEN}/sendMessage"
PAYLOAD_TEMPLATE=$(cat <<EOF
{
    "chat_id": "$CHAT_ID",
    "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%\\n• Duration: {{.Duration}}",
    "parse_mode": "Markdown",
    "disable_web_page_preview": true
}
EOF
)

# Save configuration
cat > /opt/pulse/telegram-webhook.json <<EOF
{
    "name": "Telegram Alerts",
    "url": "$WEBHOOK_URL",
    "method": "POST",
    "headers": {
        "Content-Type": "application/json"
    },
    "payloadTemplate": $(echo "$PAYLOAD_TEMPLATE" | python3 -c "import sys, json; print(json.dumps(sys.stdin.read()))")
}
EOF

echo -e "${GREEN}Configuration saved to: /opt/pulse/telegram-webhook.json${NC}"
echo ""
echo "Copy this for Pulse Webhook setup:"
echo ""
echo -e "${YELLOW}Webhook URL:${NC}"
echo "$WEBHOOK_URL"
echo ""
echo -e "${YELLOW}Custom Payload Template:${NC}"
echo "$PAYLOAD_TEMPLATE"
echo ""
echo -e "${GREEN}✓ Setup Complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Go to Pulse web interface"
echo "2. Navigate to Alerts → Webhooks"
echo "3. Use the configuration above"
echo "4. Enable the webhook"