#!/usr/bin/env python3
"""
Automated Telegram Bot Setup for Pulse
This script creates a bot and configures it automatically
"""

import os
import sys
import json
import time
import requests

try:
    from telethon import TelegramClient
    from telethon.sessions import StringSession
except ImportError:
    print("Installing required packages...")
    os.system("pip install telethon")
    from telethon import TelegramClient
    from telethon.sessions import StringSession

# Configuration
API_ID = 2040  # Default API ID for Telegram CLI apps
API_HASH = "b18441a1ff607e10a989891a5462e627"  # Default API hash
PULSE_CONFIG_DIR = "/opt/pulse"

def setup_telegram_bot():
    """Interactive Telegram bot setup"""
    
    print("=" * 50)
    print("Automated Telegram Bot Setup for Pulse")
    print("=" * 50)
    print()
    
    # Check for existing configuration
    token_file = os.path.join(PULSE_CONFIG_DIR, ".telegram_bot_token")
    chat_file = os.path.join(PULSE_CONFIG_DIR, ".telegram_chat_id")
    
    if os.path.exists(token_file):
        print("Found existing bot configuration")
        with open(token_file, 'r') as f:
            bot_token = f.read().strip()
        print(f"Bot token: {bot_token[:10]}...{bot_token[-5:]}")
    else:
        print("Creating new Telegram bot...")
        print()
        print("Option 1: Manual Setup")
        print("-" * 30)
        print("1. Open Telegram and search for @BotFather")
        print("2. Send /newbot")
        print("3. Choose a name and username")
        print("4. Copy the token here")
        print()
        print("Option 2: Automated Setup (requires phone number)")
        print("-" * 30)
        
        choice = input("Choose option (1 or 2): ").strip()
        
        if choice == "2":
            # Automated bot creation using Telethon
            phone = input("Enter your phone number (with country code, e.g., +1234567890): ")
            
            client = TelegramClient(StringSession(), API_ID, API_HASH)
            
            async def create_bot():
                await client.start(phone=phone)
                
                # Send message to BotFather
                botfather = await client.get_entity("@BotFather")
                
                # Create new bot
                await client.send_message(botfather, "/newbot")
                time.sleep(1)
                
                # Set bot name
                bot_name = "Pulse Monitor Alert Bot"
                await client.send_message(botfather, bot_name)
                time.sleep(1)
                
                # Set bot username (must be unique)
                import random
                bot_username = f"PulseMonitor_{random.randint(1000, 9999)}_bot"
                await client.send_message(botfather, bot_username)
                time.sleep(2)
                
                # Get the response with token
                messages = await client.get_messages(botfather, limit=1)
                response = messages[0].text
                
                # Extract token from response
                import re
                token_match = re.search(r'[0-9]+:[A-Za-z0-9_-]+', response)
                if token_match:
                    return token_match.group(0), bot_username
                else:
                    print("Could not extract token from BotFather response")
                    return None, None
            
            import asyncio
            bot_token, bot_username = asyncio.run(create_bot())
            
            if not bot_token:
                print("Failed to create bot automatically")
                bot_token = input("Please enter the bot token manually: ").strip()
        else:
            bot_token = input("Enter your bot token: ").strip()
        
        # Save token
        with open(token_file, 'w') as f:
            f.write(bot_token)
        os.chmod(token_file, 0o600)
        print(f"✓ Bot token saved")
    
    # Verify bot token
    print("\nVerifying bot...")
    response = requests.get(f"https://api.telegram.org/bot{bot_token}/getMe")
    
    if response.json().get('ok'):
        bot_info = response.json()['result']
        print(f"✓ Bot verified: {bot_info['first_name']} (@{bot_info['username']})")
    else:
        print("✗ Invalid bot token!")
        sys.exit(1)
    
    # Get chat ID
    if os.path.exists(chat_file):
        with open(chat_file, 'r') as f:
            chat_id = f.read().strip()
        print(f"Found existing chat ID: {chat_id}")
    else:
        print("\nGetting your chat ID...")
        print(f"1. Open Telegram")
        print(f"2. Search for @{bot_info['username']}")
        print(f"3. Send any message to the bot")
        input("\nPress ENTER after sending a message...")
        
        # Get updates
        response = requests.get(f"https://api.telegram.org/bot{bot_token}/getUpdates")
        updates = response.json()
        
        if updates.get('ok') and updates.get('result'):
            # Find chat IDs
            chat_ids = set()
            for update in updates['result']:
                if 'message' in update:
                    chat_ids.add(update['message']['chat']['id'])
            
            if chat_ids:
                chat_id = list(chat_ids)[0]
                print(f"✓ Found chat ID: {chat_id}")
                
                # Save chat ID
                with open(chat_file, 'w') as f:
                    f.write(str(chat_id))
                os.chmod(chat_file, 0o600)
            else:
                print("No chat ID found")
                chat_id = input("Enter your chat ID manually: ").strip()
        else:
            print("Could not get updates")
            chat_id = input("Enter your chat ID manually: ").strip()
    
    # Send test message
    print("\nSending test message...")
    test_message = {
        "chat_id": chat_id,
        "text": "🎉 *Pulse Telegram Integration Successful!*\n\nYour bot is now connected and ready to receive alerts.",
        "parse_mode": "Markdown"
    }
    
    response = requests.post(
        f"https://api.telegram.org/bot{bot_token}/sendMessage",
        json=test_message
    )
    
    if response.json().get('ok'):
        print("✓ Test message sent successfully!")
    else:
        print("✗ Failed to send test message")
        print(response.json())
    
    # Generate Pulse configuration
    print("\n" + "=" * 50)
    print("PULSE WEBHOOK CONFIGURATION")
    print("=" * 50)
    print()
    print(f"Webhook URL: https://api.telegram.org/bot{bot_token}/sendMessage")
    print()
    print("Custom Payload Template:")
    payload = {
        "chat_id": str(chat_id),
        "text": "🚨 *Pulse Alert: {{.Level}}*\\n\\n{{.Message}}\\n\\n📊 *Details:*\\n• Resource: {{.ResourceName}}\\n• Node: {{.Node}}\\n• Value: {{.Value}}%\\n• Threshold: {{.Threshold}}%",
        "parse_mode": "Markdown"
    }
    print(json.dumps(payload, indent=2))
    
    # Save configuration
    config_file = os.path.join(PULSE_CONFIG_DIR, "telegram-webhook.json")
    config = {
        "url": f"https://api.telegram.org/bot{bot_token}/sendMessage",
        "method": "POST",
        "payload": payload
    }
    
    with open(config_file, 'w') as f:
        json.dump(config, f, indent=2)
    
    print(f"\n✓ Configuration saved to: {config_file}")
    print("\nNext steps:")
    print("1. Go to Pulse web interface")
    print("2. Navigate to Alerts > Webhooks")
    print("3. Use the configuration above")

if __name__ == "__main__":
    setup_telegram_bot()