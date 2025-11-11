#!/bin/bash
#
# Setup script for Pulse update integration tests
# Prepares the test environment and installs dependencies
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "==================================="
echo "Pulse Update Integration Test Setup"
echo "==================================="
echo ""

# Check prerequisites
echo "Checking prerequisites..."

# Check Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first."
    exit 1
fi
echo "✅ Docker is available"

# Check Docker Compose
if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi
echo "✅ Docker Compose is available"

# Check Node.js
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js 18+ first."
    exit 1
fi

NODE_VERSION=$(node -v | cut -d'v' -f2 | cut -d'.' -f1)
if [ "$NODE_VERSION" -lt 18 ]; then
    echo "❌ Node.js version 18 or higher is required (found: $(node -v))"
    exit 1
fi
echo "✅ Node.js $(node -v) is available"

# Check Go
if ! command -v go &> /dev/null; then
    echo "⚠️  Go is not installed. Mock server build may fail."
else
    echo "✅ Go $(go version | awk '{print $3}') is available"
fi

echo ""
echo "Installing npm dependencies..."
cd "$TEST_ROOT"

if [ -f "package-lock.json" ]; then
    npm ci
else
    npm install
fi

echo ""
echo "Installing Playwright browsers..."
npx playwright install chromium
npx playwright install-deps chromium

echo ""
echo "Building mock GitHub server..."
cd "$TEST_ROOT/mock-github-server"

if [ -f "go.mod" ]; then
    go mod download
    echo "✅ Go dependencies downloaded"
fi

echo ""
echo "Building Docker images..."
cd "$TEST_ROOT"

# Build mock GitHub server image
docker build -t pulse-mock-github:test ./mock-github-server
echo "✅ Mock GitHub server image built"

# Build Pulse test image (from root of repo)
cd "$TEST_ROOT/../.."
if [ -f "Dockerfile" ]; then
    docker build -t pulse:test -f Dockerfile .
    echo "✅ Pulse test image built"
else
    echo "⚠️  Pulse Dockerfile not found. Using published image instead."
fi

echo ""
echo "==================================="
echo "✅ Setup complete!"
echo "==================================="
echo ""
echo "Next steps:"
echo "  1. Run all tests:    npm test"
echo "  2. Run specific test: npx playwright test tests/01-happy-path.spec.ts"
echo "  3. View UI:          npm run test:ui"
echo "  4. Debug mode:       npm run test:debug"
echo ""
echo "Docker commands:"
echo "  Start services:  npm run docker:up"
echo "  Stop services:   npm run docker:down"
echo "  View logs:       npm run docker:logs"
echo ""
