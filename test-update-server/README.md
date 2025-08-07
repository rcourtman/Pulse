# Pulse Update Testing Server

This directory contains a mock GitHub API server for testing Pulse's update functionality locally without creating real GitHub releases.

## Quick Start

1. **Set up test releases:**
   ```bash
   ./setup-test-release.sh
   ```

2. **Start the mock server:**
   ```bash
   python3 server.py 8888
   ```

3. **Run Pulse with test server:**
   ```bash
   export PULSE_UPDATE_SERVER=http://localhost:8888
   ./pulse
   ```

4. **Test update check:**
   ```bash
   curl http://localhost:7655/api/updates/check
   ```

## How It Works

- `server.py` - Mock GitHub API server that serves fake release information
- `setup-test-release.sh` - Creates test release tarballs in `files/` directory
- The server responds to `/repos/rcourtman/Pulse/releases/latest` just like GitHub
- Set `PULSE_UPDATE_SERVER` environment variable to use the test server

## Customizing Test Releases

Edit `server.py` to change:
- Version number (default: v4.0.99)
- Release notes
- Release date
- Prerelease status

## Testing Different Scenarios

### Test Update Available
- Default configuration shows v4.0.99 as available

### Test No Updates
- Edit `server.py` to return a version lower than current

### Test Prerelease
- Set `"prerelease": true` in server.py

### Test Update Process
1. Check for updates: `curl http://localhost:7655/api/updates/check`
2. Apply update: `curl -X POST http://localhost:7655/api/updates/apply -d '{"url":"http://localhost:8888/files/pulse-v4.0.99-linux-amd64.tar.gz"}'`
3. Check status: `curl http://localhost:7655/api/updates/status`

## Resetting

To create a new test release:
```bash
./setup-test-release.sh
```

To stop the server:
```bash
pkill -f "python3 server.py"
```