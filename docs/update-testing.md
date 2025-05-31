# Update Mechanism Testing

This document describes how to test the Pulse update mechanism without creating real GitHub releases.

## Test Mode

The update system includes a test mode that simulates available updates without requiring actual releases.

### How to Use

1. **Start the server in test mode:**
   ```bash
   ./scripts/test-update.sh
   ```
   
   Or with a custom version:
   ```bash
   ./scripts/test-update.sh 5.0.0
   ```

2. **Open Pulse in your browser** and go to Settings

3. **Click "Check for Updates"** - you should see version 99.99.99 (or your custom version) available

4. **Click "Apply Update"** to test the update process

### What Happens in Test Mode

- The update check returns a mock release with version 99.99.99 (or custom)
- A mock tarball is created on-the-fly from current application files
- The update process runs normally but with the test package
- You can test the entire flow: download, backup, extraction, restart

### Environment Variables

- `UPDATE_TEST_MODE=true` - Enables test mode
- `UPDATE_TEST_VERSION=X.X.X` - Sets the mock version number (default: 99.99.99)

### Manual Testing

You can also manually set the environment variables:

```bash
UPDATE_TEST_MODE=true UPDATE_TEST_VERSION=10.0.0 npm run dev:server
```

### Debugging

When in test mode, check the console for:
- `[UpdateManager] Test mode enabled, returning mock update info` - Confirms test mode is active
- Asset names and download URLs in browser console if update fails
- Server logs for the update process steps

### Notes

- The mock tarball excludes node_modules, .git, temp, and backup directories
- The test download URL is: `http://localhost:3000/api/test/mock-update.tar.gz`
- This endpoint only works when `UPDATE_TEST_MODE=true`