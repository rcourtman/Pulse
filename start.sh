#!/bin/bash

# Check if Pulse is already running by looking for the npm run dev process
echo "Checking for existing Pulse instances..."
EXISTING_NODE_PIDS=$(ps aux | grep "node" | grep -v grep | grep -E 'server|client' | awk '{print $2}')

if [ -n "$EXISTING_NODE_PIDS" ]; then
  echo "Found existing Pulse processes with PIDs: $EXISTING_NODE_PIDS"
  echo "Killing existing processes..."
  
  # Kill each process found
  for PID in $EXISTING_NODE_PIDS; do
    echo "Killing process $PID..."
    kill -9 $PID
  done
  
  # Give processes time to terminate
  sleep 2
  echo "Existing processes terminated."
else
  echo "No existing Pulse processes found."
fi

# Navigate to the server directory
cd server

# Check if .env file exists, if not, create it from example
if [ ! -f .env ]; then
  echo "Creating .env file from .env.example..."
  cp .env.example .env
  echo "Please edit the .env file with your Proxmox node details."
  exit 1
fi

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
  echo "Installing server dependencies..."
  npm install
fi

# Navigate to client directory
cd ../client

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
  echo "Installing client dependencies..."
  npm install
fi

# Navigate back to root
cd ..

# Start the application in development mode
echo "Starting Pulse in development mode..."
npm run dev 