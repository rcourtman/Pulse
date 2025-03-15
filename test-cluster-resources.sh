#!/bin/bash

# Copy the test environment file to .env
cp .env.cluster-test .env

# Start the development server
echo "Starting development server with mock data and cluster resources endpoint enabled..."
npm run dev
