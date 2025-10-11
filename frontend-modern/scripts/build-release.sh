#!/bin/bash
set -e

# Get version from VERSION file
VERSION=$(cat VERSION)

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Building Pulse v${VERSION} release binaries${NC}"

# Create release directory
mkdir -p release

# Build frontend first
echo -e "${BLUE}Building frontend...${NC}"
cd frontend-modern
npm run build
cd ..

# Copy frontend to internal/api for embedding
echo -e "${BLUE}Copying frontend for embedding...${NC}"
rm -rf internal/api/frontend-modern
mkdir -p internal/api/frontend-modern
cp -r frontend-modern/dist internal/api/frontend-modern/

# Build for multiple architectures
echo -e "${BLUE}Building linux/amd64...${NC}"
GOOS=linux GOARCH=amd64 go build -o release/pulse-linux-amd64 ./cmd/pulse

echo -e "${BLUE}Building linux/arm64...${NC}"
GOOS=linux GOARCH=arm64 go build -o release/pulse-linux-arm64 ./cmd/pulse

echo -e "${BLUE}Building linux/arm (v7)...${NC}"
GOOS=linux GOARCH=arm GOARM=7 go build -o release/pulse-linux-armv7 ./cmd/pulse

# Create tarballs for each architecture
echo -e "${BLUE}Creating architecture-specific tarballs...${NC}"

# AMD64
cd release
mkdir -p temp-amd64
cp pulse-linux-amd64 temp-amd64/pulse
cp ../VERSION temp-amd64/VERSION
mkdir -p temp-amd64/frontend-modern
cp -r ../internal/api/frontend-modern/dist/* temp-amd64/frontend-modern/
tar -czf pulse-v${VERSION}-linux-amd64.tar.gz -C temp-amd64 .
rm -rf temp-amd64

# ARM64
mkdir -p temp-arm64
cp pulse-linux-arm64 temp-arm64/pulse
cp ../VERSION temp-arm64/VERSION
mkdir -p temp-arm64/frontend-modern
cp -r ../internal/api/frontend-modern/dist/* temp-arm64/frontend-modern/
tar -czf pulse-v${VERSION}-linux-arm64.tar.gz -C temp-arm64 .
rm -rf temp-arm64

# ARMv7
mkdir -p temp-armv7
cp pulse-linux-armv7 temp-armv7/pulse
cp ../VERSION temp-armv7/VERSION
mkdir -p temp-armv7/frontend-modern
cp -r ../internal/api/frontend-modern/dist/* temp-armv7/frontend-modern/
tar -czf pulse-v${VERSION}-linux-armv7.tar.gz -C temp-armv7 .
rm -rf temp-armv7

# Create universal tarball with all binaries
echo -e "${BLUE}Creating universal tarball...${NC}"
mkdir -p temp-universal
cp pulse-linux-amd64 temp-universal/
cp pulse-linux-arm64 temp-universal/
cp pulse-linux-armv7 temp-universal/
ln -sf pulse-linux-amd64 temp-universal/pulse
cp ../VERSION temp-universal/VERSION
mkdir -p temp-universal/frontend-modern
cp -r ../internal/api/frontend-modern/dist/* temp-universal/frontend-modern/
tar -czf pulse-v${VERSION}.tar.gz -C temp-universal .
rm -rf temp-universal

# Generate checksums
echo -e "${BLUE}Generating checksums...${NC}"
sha256sum pulse-v${VERSION}-linux-amd64.tar.gz > checksums.txt
sha256sum pulse-v${VERSION}-linux-arm64.tar.gz >> checksums.txt
sha256sum pulse-v${VERSION}-linux-armv7.tar.gz >> checksums.txt
sha256sum pulse-v${VERSION}.tar.gz >> checksums.txt

cd ..

echo -e "${GREEN}✓ Build complete!${NC}"
echo ""
echo "Release files:"
ls -lh release/
echo ""
echo "Checksums:"
cat release/checksums.txt
