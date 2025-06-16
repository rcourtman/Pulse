# Pulse Development Workflow

## Branch Structure

- **`main`** - Stable releases only. Users install from here.
- **`develop`** - Daily development work. RC releases are created from here.

## Daily Workflow

### 1. Always work on develop branch
```bash
git checkout develop
git pull origin develop
```

### 2. Make your changes and commit frequently
```bash
# Edit files...
git add .
git commit -m "fix: your change description"
git push
```

### 3. RC releases are automatic
- Every push to `develop` creates a new RC release (if there are changes)
- Share RC version with users who need to test: `v3.24.0-rc1`, `v3.24.0-rc2`, etc.

### 4. Creating a stable release
When RC testing is complete and you're ready for a stable release:

```bash
# 1. Ensure develop is up to date
git checkout develop
git pull

# 2. Merge to main
git checkout main
git merge develop

# 3. Update version in package.json (remove any -rc suffix)
# Edit package.json to bump version if needed

# 4. Commit and tag
git add package.json
git commit -m "chore: release v3.25.0"
git tag v3.25.0
git push origin main --tags

# 5. Go back to develop for daily work
git checkout develop
git merge main  # Keep develop in sync
git push
```

## For Users Reporting Issues

1. User reports issue on v3.24.0
2. You fix it on `develop` branch
3. Automatic RC is created (e.g., v3.24.1-rc1)
4. Ask user to test: "Can you test with v3.24.1-rc1?"
5. If good, merge to main for stable v3.24.1

## Key Benefits

- ✅ Stable users stay on tested versions
- ✅ You can commit frequently without affecting stable users  
- ✅ Easy testing workflow with automatic RC builds
- ✅ Professional release management
- ✅ Clear separation between development and production