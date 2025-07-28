#!/bin/bash

# Uninstall Pulse systemd services

echo "Stopping Pulse services..."
sudo systemctl stop pulse-backend.service
sudo systemctl stop pulse-frontend.service

echo "Disabling services..."
sudo systemctl disable pulse-backend.service
sudo systemctl disable pulse-frontend.service

echo "Removing service files..."
sudo rm -f /etc/systemd/system/pulse-backend.service
sudo rm -f /etc/systemd/system/pulse-frontend.service

sudo systemctl daemon-reload

echo "Services uninstalled!"