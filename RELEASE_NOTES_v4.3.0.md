# Pulse v4.3.0 Release Notes

## ⚠️ BREAKING CHANGES

### Frontend Now Embedded in Binary
Starting with v4.3.0, the frontend is embedded directly in the Pulse binary. This is a significant architectural change that simplifies deployment but has some implications:

**What Changed:**
- Frontend files are now compiled into the Go binary at build time
- No separate `frontend-modern/` directory needed
- Single binary deployment - truly portable
- Smaller release tarballs

**Migration Notes:**
- **For manual installations**: Simply replace your binary. The old `frontend-modern/` directory can be deleted
- **For Docker users**: No changes needed, containers will work as normal
- **For developers**: Frontend changes now require recompiling the Go binary (use `scripts/rebuild-frontend.sh`)
- **Auto-updates**: Will work seamlessly from v4.2.1 to v4.3.0+

**Benefits:**
- Eliminates redirect loop issues (#304)
- No more path resolution problems
- Simpler installation and deployment
- Consistent behavior across all installation methods
- Reduced deployment size

## Features
- Embedded frontend for simplified single-binary deployment
- Improved installation experience

## Bug Fixes
- Fixed redirect loops in various installation scenarios (#304)
- Fixed path resolution issues with community scripts vs manual installs

## Technical Details
The frontend is embedded using Go's `embed` package. During the build process:
1. Frontend is built with npm
2. Built files are copied to `internal/api/frontend-modern/`
3. Go compiler embeds these files into the binary
4. Runtime serves embedded files directly from memory

This change makes Pulse a true single-binary application, eliminating an entire class of deployment issues.