#!/bin/sh
# Docker health check script for Pulse
# Automatically uses HTTPS when HTTPS_ENABLED=true

if [ "$HTTPS_ENABLED" = "true" ] || [ "$HTTPS_ENABLED" = "1" ]; then
    # Use HTTPS with --no-check-certificate to handle self-signed certs
    wget --no-verbose --tries=1 --spider --no-check-certificate https://localhost:7655 || exit 1
else
    wget --no-verbose --tries=1 --spider http://localhost:7655 || exit 1
fi
