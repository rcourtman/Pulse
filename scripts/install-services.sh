#!/bin/bash

# Install Pulse systemd services

echo "Installing Pulse systemd services..."

# Copy service files
sudo cp /opt/pulse/pulse-backend.service /etc/systemd/system/
sudo cp /opt/pulse/pulse-frontend.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable services to start on boot
sudo systemctl enable pulse-backend.service
sudo systemctl enable pulse-frontend.service

# Start services
sudo systemctl start pulse-backend.service
sudo systemctl start pulse-frontend.service

echo "Services installed and started!"
echo ""
echo "Backend status:"
sudo systemctl status pulse-backend.service --no-pager
echo ""
echo "Frontend status:"
sudo systemctl status pulse-frontend.service --no-pager
echo ""
echo "Useful commands:"
echo "  sudo systemctl status pulse-backend    # Check backend status"
echo "  sudo systemctl status pulse-frontend   # Check frontend status"
echo "  sudo journalctl -u pulse-backend -f   # Watch backend logs"
echo "  sudo journalctl -u pulse-frontend -f  # Watch frontend logs"
echo "  tail -f /opt/pulse/pulse.log          # Watch backend app logs"