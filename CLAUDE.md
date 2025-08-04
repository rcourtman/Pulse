# Claude Code Access Information

## TypeScript Code Standards
- **NEVER use `any` type** - Always use proper TypeScript types
- **Type everything correctly** - All variables, parameters, and return values must be properly typed
- **Use type guards** - When dealing with union types, use proper type guards (e.g., `'property' in object`)
- **No implicit any** - Ensure TypeScript strict mode catches missing types
- **Prefer interfaces over type aliases** for object shapes
- **Use generics** when appropriate instead of `any`

## Development Environment (Current Machine - debian-go)
This is the main development environment for Pulse. The backend uses a watch script for auto-reload during development.

- **Backend**: Auto-reloads on Go file changes via `/opt/pulse/scripts/backend-watch.sh`
  - Port: 7655 (serves both API and frontend in Go version)
  - Service: `sudo systemctl restart pulse-backend`
  - Logs: `/opt/pulse/pulse.log`
  - Watch script monitors `.go` files and rebuilds/restarts automatically
- **Frontend**: Served by the Go backend on port 7655
  - Modern UI accessible at http://192.168.0.123:7655
  - Frontend is built into the Go binary (embedded)
- **Development workflow**:
  - Make changes to Go files → backend automatically rebuilds and restarts
  - Frontend changes require `cd frontend-modern && npm run build`
- **Manual restart**: `sudo systemctl restart pulse-backend`
- **Status**: `sudo systemctl status pulse-backend`
- **Logs**: `sudo journalctl -u pulse-backend -f` or `tail -f /opt/pulse/pulse.log`

## Available SSH Nodes

Claude Code has SSH access to the following nodes:

### Proxmox Cluster
- **delly** (delly.lan) - Part of a Proxmox cluster with minipc
  - User: root
  - Access: SSH key-based authentication

### Standalone Proxmox Node
- **pimox** (pimox.lan / 192.168.0.2) - Standalone Proxmox VE node
  - User: root
  - Access: SSH key-based authentication

## Test LXC Container

### Pulse Test Container
- **Container ID**: 130 on delly
- **Hostname**: pulse-test
- **IP Address**: 192.168.0.152
- **Pulse Version**: v4.0.0-rc.1
- **Web Interface**: http://192.168.0.152:7655
- **Purpose**: Permanent test container for Pulse development and testing
- **Service**: Running as systemd service (pulse.service)

### Docker Builder Container
- **Container ID**: 135 on delly
- **Hostname**: docker-builder
- **IP Address**: 192.168.0.174
- **Purpose**: Dedicated container for building multi-arch Docker images
- **Docker buildx**: Pre-configured with multiarch builder
- **Architectures**: Builds for linux/amd64, linux/arm64, linux/arm/v7

## Testing Tools

Automated testing tools are available in `/opt/pulse/testing-tools/`:

### Setup Testing Environment
```bash
cd /opt/pulse/testing-tools
npm install
```

### Available Tests
- **Email Configuration**: `npm run test:email` - Tests email setup and persistence
- **API Endpoints**: `npm run test:api` - Validates all API endpoints
- **Button Functionality**: `npm run test:buttons` - UI button testing with Playwright
- **Comprehensive**: `npm run test:comprehensive` - Full system settings test
- **Alerts Testing**: `npm run test:alerts` - Tests alert generation and acknowledgement
- **Threshold Testing**: `npm run test:thresholds` - Tests threshold changes through UI
- **Mobile Dashboard**: `npm run test:mobile-dash` - Tests dashboard mobile responsiveness
- **Mobile Storage**: `npm run test:mobile-storage` - Tests storage tab mobile responsiveness
- **System Status**: `npm run status` - Quick system health check
- **Run All**: `npm run test:all` - Execute all tests

### Quick Test Commands
```bash
# Test if email is working
cd /opt/pulse/testing-tools && npm run test:email

# Check all API endpoints
cd /opt/pulse/testing-tools && npm run test:api

# Test alert system
cd /opt/pulse/testing-tools && npm run test:alerts

# Check system status
cd /opt/pulse/testing-tools && npm run status
```

## Git Repository Workflow

This project uses a private/public repository workflow:

### Repository Structure
- **Private repo**: `pulse-go-rewrite` (https://github.com/rcourtman/pulse-go-rewrite.git) - Development and testing
- **Public repo**: `Pulse` (https://github.com/rcourtman/Pulse.git) - Production releases

### Development Workflow
1. **All development happens in the private repo** on the main branch
2. **Create test releases** in the private repo for testing auto-updates
3. **IMPORTANT: Keep v4 releases in private repo only** until fully tested
   - v3 users check the public repo for updates
   - Publishing v4 to public repo would make it visible to v3 users
   - Only publish v4 releases to public repo when ready for migration
4. **When ready for production**, sync to public repo:
   ```bash
   git push public main --force
   ```

### Important Git Rules
- **NEVER make changes directly to the public repo**
- **Private repo should be exactly how you want the public repo**
- **Test everything in private before pushing to public**
- **Public repo is treated as read-only** (except for syncing from private)

### Remote Configuration
The repository has two remotes configured:
- `origin` → private repo (pulse-go-rewrite)
- `public` → public repo (Pulse)

## Important Instructions
- **NEVER create documentation files** (*.md) unless explicitly requested by the user
- **NEVER create README files** or other docs to explain changes - just explain in the response
- **DO NOT create markdown files to document findings** - Just explain in the response instead
- **DO NOT create analysis or optimization docs** - The user hates unnecessary documentation
- **ALWAYS prefer editing existing files** over creating new ones
- **RUN TESTS** after making significant changes using the testing tools

## Docker Build Process

### Quick Docker Build Commands
```bash
# Build and push multi-arch images for stable release
ssh root@192.168.0.174
cd /root/Pulse
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.X.X \
  -t rcourtman/pulse:4.X.X \
  -t rcourtman/pulse:4.X \
  -t rcourtman/pulse:4 \
  -t rcourtman/pulse:latest \
  --push .

# For RC releases
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t rcourtman/pulse:v4.X.X-rc.X \
  -t rcourtman/pulse:rc \
  --push .
```

### Docker Build Container Setup (if needed)
```bash
# Create new Docker builder container on delly
ssh root@delly.lan
pct create 135 /var/lib/vz/template/cache/debian-12-standard_12.7-1_amd64.tar.zst \
  --hostname docker-builder \
  --memory 8192 \
  --cores 4 \
  --rootfs local-zfs:32 \
  --net0 name=eth0,bridge=vmbr0,ip=dhcp \
  --unprivileged 0 \
  --features nesting=1,keyctl=1

# Install Docker in container
pct start 135
pct exec 135 -- bash -c "curl -fsSL https://get.docker.com | sh"
pct exec 135 -- docker buildx create --name multiarch --use
```
