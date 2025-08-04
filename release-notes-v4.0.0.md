# ⚠️ CRITICAL: DO NOT AUTO-UPDATE FROM v3.x ⚠️

**This is a complete rewrite of Pulse from Python to Go. v3.x users MUST NOT auto-update to v4.x as it will break your installation.**

## 🚀 v4.0.0 - Complete Go Rewrite

This is a complete rewrite of Pulse from Python to Go, bringing significant performance improvements, reduced resource usage, and a modern web interface.

### Migration Guide for v3 Users
Please visit https://github.com/rcourtman/Pulse/wiki/v3-to-v4-Migration-Guide for detailed migration instructions.

### 🐳 Docker Support

Docker images are available for multiple architectures:
- **AMD64** (Intel/AMD servers)
- **ARM64** (64-bit ARM systems)
- **ARMv7** (32-bit ARM, including Raspberry Pi)

#### Docker Tags
- `rcourtman/pulse:latest` - Latest stable release (v4.0.0)
- `rcourtman/pulse:4`, `rcourtman/pulse:4.0`, `rcourtman/pulse:4.0.0` - Version-specific tags
- `rcourtman/pulse:v4.0.0` - Full version tag

#### Docker Installation
```bash
# Create data directory
mkdir -p ~/pulse-data

# Run Pulse with Docker
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v ~/pulse-data:/data \
  --restart unless-stopped \
  rcourtman/pulse:latest
```

### ✨ What's New in v4

#### Complete Architecture Overhaul
- **Language**: Rewritten from Python to Go
- **Performance**: 10x faster startup, 5x lower memory usage
- **Binary**: Single static binary, no Python dependencies
- **Database**: Switched from MySQL to embedded SQLite

#### Modern Web Interface
- **React-based UI**: Fast, responsive, and mobile-friendly
- **Real-time updates**: WebSocket-based live data
- **Dark mode**: Built-in theme support
- **Mobile optimized**: Touch-friendly interface

#### Enhanced Features
- **Better agent management**: Improved connection handling and status monitoring
- **Faster alerting**: Sub-second alert detection and notification
- **Improved storage monitoring**: More accurate disk usage tracking
- **Enhanced process monitoring**: Better CPU and memory tracking

#### Installation Improvements
- **Single binary**: Just download and run
- **No dependencies**: No Python, pip, or MySQL required
- **Auto-update**: Built-in update mechanism (within v4.x versions)
- **Multi-platform**: Linux (amd64, arm64, armv7), Docker support

### 📦 Installation Methods

#### Direct Download
```bash
# Download for your architecture
wget https://github.com/rcourtman/Pulse/releases/download/v4.0.0/pulse-linux-amd64
chmod +x pulse-linux-amd64
./pulse-linux-amd64
```

#### Docker
```bash
docker run -d -p 7655:7655 -v ~/pulse-data:/data rcourtman/pulse:latest
```

#### From Source
```bash
git clone https://github.com/rcourtman/Pulse.git
cd Pulse
go build -o pulse cmd/server/main.go
./pulse
```

### 🔄 Breaking Changes
- **Port change**: Default port changed from 5000 to 7655
- **Database**: MySQL replaced with SQLite (automatic migration not available)
- **API**: New RESTful API (v3 API endpoints deprecated)
- **Configuration**: New JSON-based configuration format

### 📝 Full Changelog
For a detailed list of all changes, see [CHANGELOG.md](https://github.com/rcourtman/Pulse/blob/main/CHANGELOG.md)

### 🙏 Thank You
Thank you to all Pulse users for your patience during this major rewrite. Your feedback and support have been invaluable in making Pulse better.