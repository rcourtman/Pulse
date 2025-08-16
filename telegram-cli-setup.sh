#!/bin/bash

# Telegram CLI Setup via Homebrew for Pulse
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================"
echo "Telegram CLI Setup for Pulse"
echo "======================================${NC}"
echo ""

# Check OS
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo -e "${GREEN}✓ macOS detected${NC}"
    
    # Check if Homebrew is installed
    if ! command -v brew &> /dev/null; then
        echo -e "${RED}Homebrew not found. Installing...${NC}"
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    else
        echo -e "${GREEN}✓ Homebrew found${NC}"
    fi
    
    # Install telegram-cli
    echo ""
    echo -e "${YELLOW}Installing telegram-cli via Homebrew...${NC}"
    
    # Check if already installed
    if brew list telegram-cli &>/dev/null; then
        echo -e "${GREEN}✓ telegram-cli already installed${NC}"
    else
        echo "Installing telegram-cli..."
        brew install telegram-cli
    fi
    
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo -e "${GREEN}✓ Linux detected${NC}"
    
    # For Linux, try different package managers
    if command -v apt-get &> /dev/null; then
        echo "Installing telegram-cli via apt..."
        sudo apt-get update
        sudo apt-get install -y telegram-cli
    elif command -v yum &> /dev/null; then
        echo "Installing telegram-cli via yum..."
        sudo yum install -y telegram-cli
    else
        echo -e "${RED}Package manager not supported. Building from source...${NC}"
        
        # Build from source
        sudo apt-get install -y libreadline-dev libconfig-dev libssl-dev lua5.2 liblua5.2-dev libevent-dev libjansson-dev libpython-dev make
        
        cd /tmp
        git clone --recursive https://github.com/vysheng/tg.git
        cd tg
        ./configure
        make
        sudo make install
        cd /opt/pulse
    fi
else
    echo -e "${RED}Unsupported OS: $OSTYPE${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✓ telegram-cli installed${NC}"
echo ""

# Create telegram-cli config
echo -e "${YELLOW}Creating telegram-cli configuration...${NC}"

mkdir -p ~/.telegram-cli

# Create config file
cat > ~/.telegram-cli/config << 'EOF'
# Telegram CLI Configuration
default_profile = "default";

default = {
  config_directory = ".telegram-cli";
  use_ipv6 = false;
  make_auth_key_on_start = true;
};
EOF

echo -e "${GREEN}✓ Configuration created${NC}"
echo ""

# Create bot creation script for telegram-cli
cat > /tmp/create_bot.lua << 'EOF'
-- Lua script for telegram-cli to create a bot
function on_msg_receive (msg)
    if msg.text then
        print("Message received: " .. msg.text)
    end
end

function on_our_id (id)
    print("Our ID: " .. id)
end

function on_secret_chat_created (peer)
    print("Secret chat created")
end

function on_user_update (user)
end

function on_chat_update (chat)
end

function on_get_difference_end ()
end

function on_binlog_replay_end ()
    -- Start bot creation process
    send_msg("@BotFather", "/newbot", ok_cb, false)
end
EOF

# Create interactive setup script
cat > /tmp/setup_telegram_bot.sh << 'SCRIPT'
#!/bin/bash

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Starting telegram-cli...${NC}"
echo ""
echo "IMPORTANT: First time setup:"
echo "1. You'll be asked for your phone number"
echo "2. Enter it with country code (e.g., +1234567890)"
echo "3. You'll receive a code via Telegram"
echo "4. Enter the code when prompted"
echo ""
read -p "Press ENTER to continue..."

# Start telegram-cli in interactive mode
echo ""
echo "Starting Telegram CLI..."
echo "Once logged in, we'll create your bot"
echo ""

# Create expect script for automation
cat > /tmp/telegram_bot_create.expect << 'EXPECT'
#!/usr/bin/expect -f

set timeout 30

spawn telegram-cli -N -W

expect {
    "phone number:" {
        send_user "\nEnter your phone number with country code: "
        expect_user -re "(.*)\n"
        send "$expect_out(1,string)\r"
        exp_continue
    }
    "code:" {
        send_user "\nEnter the verification code: "
        expect_user -re "(.*)\n"
        send "$expect_out(1,string)\r"
        exp_continue
    }
    "> " {
        send_user "\nLogged in! Creating bot...\n"
        
        # Contact BotFather
        send "msg @BotFather /newbot\r"
        sleep 2
        
        # Send bot name
        send "msg @BotFather Pulse Monitor Alert Bot\r"
        sleep 2
        
        # Generate random username
        set timestamp [clock seconds]
        send "msg @BotFather PulseMonitor_${timestamp}_bot\r"
        sleep 3
        
        # Get messages to see the token
        send "history @BotFather 5\r"
        sleep 2
        
        send "quit\r"
    }
    timeout {
        send_user "\nTimeout occurred\n"
        exit 1
    }
}

expect eof
EXPECT

chmod +x /tmp/telegram_bot_create.expect

# Check if expect is installed
if ! command -v expect &> /dev/null; then
    echo "Installing expect..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install expect
    else
        sudo apt-get install -y expect
    fi
fi

# Run expect script
/tmp/telegram_bot_create.expect

echo ""
echo -e "${GREEN}Bot creation process completed!${NC}"
echo ""
echo "Check the output above for your bot token (looks like: 1234567890:ABCdefGHIjklMNOpqrsTUVwxyz)"
echo ""
read -p "Enter your bot token: " BOT_TOKEN

# Save token
echo "$BOT_TOKEN" > /opt/pulse/.telegram_bot_token
chmod 600 /opt/pulse/.telegram_bot_token

echo -e "${GREEN}✓ Bot token saved${NC}"

# Get chat ID
echo ""
echo "Now we need your chat ID..."
echo "1. Open Telegram app"
echo "2. Search for your bot and start a chat"
echo "3. Send any message"
echo ""
read -p "Press ENTER after sending a message to your bot..."

# Get updates to find chat ID
RESPONSE=$(curl -s "https://api.telegram.org/bot${BOT_TOKEN}/getUpdates")
CHAT_ID=$(echo "$RESPONSE" | grep -o '"chat":{"id":[0-9-]*' | grep -o '[0-9-]*' | head -1)

if [ -n "$CHAT_ID" ]; then
    echo -e "${GREEN}✓ Found chat ID: $CHAT_ID${NC}"
    echo "$CHAT_ID" > /opt/pulse/.telegram_chat_id
    chmod 600 /opt/pulse/.telegram_chat_id
else
    echo "Could not find chat ID automatically"
    echo "Visit: https://api.telegram.org/bot${BOT_TOKEN}/getUpdates"
    echo "Look for 'chat' -> 'id'"
    read -p "Enter your chat ID: " CHAT_ID
    echo "$CHAT_ID" > /opt/pulse/.telegram_chat_id
fi

# Test message
echo ""
echo "Sending test message..."
curl -s -X POST "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage" \
    -H "Content-Type: application/json" \
    -d '{
        "chat_id": "'$CHAT_ID'",
        "text": "✅ Pulse Telegram Integration Complete!",
        "parse_mode": "Markdown"
    }' > /dev/null

echo -e "${GREEN}✓ Setup complete!${NC}"
echo ""
echo "Webhook URL for Pulse:"
echo "https://api.telegram.org/bot${BOT_TOKEN}/sendMessage"
echo ""
echo "Payload template:"
echo '{'
echo '    "chat_id": "'$CHAT_ID'",'
echo '    "text": "🚨 *Pulse Alert: {{.Level}}*\n\n{{.Message}}",'
echo '    "parse_mode": "Markdown"'
echo '}'
SCRIPT

chmod +x /tmp/setup_telegram_bot.sh
/tmp/setup_telegram_bot.sh