#!/bin/bash

# Migration helper for webhooks encryption

echo "Webhook Encryption Migration"
echo "============================"

if [ -f /etc/pulse/webhooks.json ]; then
    echo "Found unencrypted webhooks.json"
    
    # Backup the original
    cp /etc/pulse/webhooks.json /etc/pulse/webhooks.json.backup
    echo "Created backup: webhooks.json.backup"
    
    # The migration will happen automatically on next webhook save
    # Force a save by updating a webhook through the API
    echo ""
    echo "To complete migration:"
    echo "1. Open Pulse UI"
    echo "2. Go to Settings > Webhooks"
    echo "3. Click Save (even without changes)"
    echo ""
    echo "This will encrypt your webhooks to webhooks.enc"
    echo ""
    echo "After migration, webhooks.json can be deleted."
else
    echo "No unencrypted webhooks.json found"
fi

if [ -f /etc/pulse/webhooks.enc ]; then
    echo "✓ Encrypted webhooks.enc already exists"
fi