# Internal API Package

This directory contains the API server implementation for Pulse.

## Important Note About `frontend-modern/`

The `frontend-modern/` subdirectory that appears here is:
- **AUTO-GENERATED** during builds
- **NOT the source code** - just a build artifact
- **IN .gitignore** - never committed
- **REQUIRED BY GO** - The embed directive needs it here

### Frontend Development Location
👉 **Edit frontend files at: `${PULSE_REPOS_DIR}/pulse/frontend-modern/src/`**

### Why This Structure?
Go's `//go:embed` directive has limitations:
1. Cannot use `../` paths to access parent directories
2. Cannot follow symbolic links
3. Must embed files within the Go module

This is a known Go limitation and our structure works around it.
