# ⚠️ DO NOT EDIT FRONTEND FILES HERE ⚠️

This `frontend-modern` directory is **AUTO-GENERATED** during builds.

## The REAL frontend location is
### `${PULSE_REPOS_DIR}/pulse/frontend-modern`

## Why does this exist?
- Go's `embed` directive cannot access files outside the module
- The build process copies the frontend here for embedding
- This directory is in `.gitignore` and not committed

## What happens if you edit files here?
- **YOUR CHANGES WILL BE LOST** on the next build
- The Makefile deletes and recreates this directory

## How to edit frontend code
1. Edit files in `${PULSE_REPOS_DIR}/pulse/frontend-modern/src/`
2. The dev server (port 7655) will hot-reload
3. When building for production, the Makefile copies it here

---
This file exists to prevent confusion. The directory structure is intentional and required by Go's limitations.
