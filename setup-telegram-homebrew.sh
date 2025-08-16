#!/bin/bash

# Simple Telegram Bot Setup using Homebrew
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================"
echo "Telegram Bot Setup via Homebrew"
echo "======================================${NC}"
echo ""

# Step 1: Install telegram-cli if on macOS
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo -e "${GREEN}macOS detected${NC}"
    echo ""
    echo "Installing telegram-cli..."
    echo "Run this command in your terminal:"
    echo ""
    echo -e "${YELLOW}brew install telegram-cli${NC}"
    echo ""
    echo "If you don't have Homebrew, install it first:"
    echo -e "${YELLOW}/bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"${NC}"
    echo ""
    read -p "Press ENTER after installing telegram-cli..."
fi

# Step 2: Since telegram-cli is complex, let's use the simpler API approach
echo ""
echo -e "${BLUE}Creating Telegram Bot${NC}"
echo "======================================"
echo ""
echo "Since telegram-cli requires phone authentication, let's use the simpler approach:"
echo ""
echo -e "${YELLOW}Step 1: Create Your Bot${NC}"
echo "1. Open Telegram (web.telegram.org works too)"
echo "2. Search for: @BotFather"
echo "3. Send: /newbot"
echo "4. Choose a name: Pulse Monitor"
echo "5. Choose username: PulseMonitor_$(date +%s)_bot"
echo "6. Copy the token (looks like: 1234567890:ABCdefGHIjklMNOpqrsTUVwxyz)"
echo ""
read -p "Enter your bot token: " BOT_TOKEN

# Verify token
echo ""
echo "Verifying bot token..."
VERIFY=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getMe")

if echo "$VERIFY" | grep -q '"ok":true'; then
    BOT_NAME=$(echo "$VERIFY" | grep -o '"first_name":"[^"]*' | cut -d'"' -f4)
    BOT_USERNAME=$(echo "$VERIFY" | grep -o '"username":"[^"]*' | cut -d'"' -f4)
    echo -e "${GREEN}✓ Bot verified: $BOT_NAME (@$BOT_USERNAME)${NC}"
    
    # Save token
    echo "$BOT_TOKEN" > ~/.pulse_telegram_token
    chmod 600 ~/.pulse_telegram_token
else
    echo -e "${RED}✗ Invalid token!${NC}"
    exit 1
fi

# Step 3: Get Chat ID
echo ""
echo -e "${YELLOW}Step 2: Get Your Chat ID${NC}"
echo "1. Open Telegram"
echo "2. Search for: @$BOT_USERNAME"
echo "3. Click START or send any message"
echo ""
read -p "Press ENTER after messaging your bot..."

# Get updates
UPDATES=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates")
CHAT_ID=$(echo "$UPDATES" | grep -o '"chat":{"id":[0-9-]*' | grep -o '[0-9-]*' | head -1)

if [ -n "$CHAT_ID" ]; then
    echo -e "${GREEN}✓ Found your chat ID: $CHAT_ID${NC}"
    echo "$CHAT_ID" > ~/.pulse_telegram_chat
    chmod 600 ~/.pulse_telegram_chat
else
    echo -e "${YELLOW}Couldn't find chat ID automatically${NC}"
    echo "Try this URL in your browser:"
    echo "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"
    echo "Look for \"chat\":{\"id\":YOUR_NUMBER"
    read -p "Enter your chat ID: " CHAT_ID
fi

# Step 4: Test
echo ""
echo "Sending test message..."
TEST=$(curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage" \
    -H "Content-Type: application/json" \
    -d "{
        \"chat_id\": \"$CHAT_ID\",
        \"text\": \"🎉 *Pulse Integration Working!*\n\nBot: @$BOT_USERNAME\nChat ID: $CHAT_ID\n\nYou're all set!\",
        \"parse_mode\": \"Markdown\"
    }")

if echo "$TEST" | grep -q '"ok":true'; then
    echo -e "${GREEN}✓ Test message sent! Check Telegram${NC}"
else
    echo -e "${RED}Failed to send test${NC}"
fi

# Step 5: Show Pulse Configuration
echo ""
echo -e "${BLUE}======================================"
echo "Configuration for Pulse"
echo "======================================${NC}"
echo ""
echo -e "${YELLOW}Webhook URL:${NC}"
echo "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage"
echo ""
echo -e "${YELLOW}HTTP Method:${NC} POST"
echo ""
echo -e "${YELLOW}Custom Payload Template:${NC}"
cat << EOF
{
    "chat_id": "$CHAT_ID",
    "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%\\n• Duration: {{.Duration}}",
    "parse_mode": "Markdown",
    "disable_web_page_preview": true
}
EOF

echo ""
echo -e "${GREEN}Setup Complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Go to Pulse web interface"
echo "2. Navigate to Alerts → Webhooks"
echo "3. Click 'Add Webhook'"
echo "4. Select 'Generic' as Service Type"
echo "5. Paste the configuration above"
echo "6. Enable the webhook"
echo "7. Test it!"

# Save config for easy access
cat > ~/pulse-telegram-config.txt << EOF
Telegram Webhook Configuration for Pulse
=========================================

Bot Token: $BOT_TOKEN
Chat ID: $CHAT_ID
Bot Username: @$BOT_USERNAME

Webhook URL:
https://api.telegram.org/bot${BOT_TOKEN}/sendMessage

Custom Payload:
{
    "chat_id": "$CHAT_ID",
    "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%",
    "parse_mode": "Markdown"
}
EOF

echo ""
echo -e "${YELLOW}Configuration saved to: ~/pulse-telegram-config.txt${NC}"