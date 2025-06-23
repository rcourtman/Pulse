# Claude Code Instructions for Pulse

This document provides specific instructions for Claude Code when working on the Pulse project.

## Release Process

When asked to create a new release, follow the `RELEASE_CHECKLIST.md` file step by step. The release process is **completely manual** - do not attempt to automate it with GitHub Actions or complex scripts.

### Key Points:
1. **Always use the checklist**: Open and follow `RELEASE_CHECKLIST.md` for every release
2. **Ask for version**: Always ask the user what version they want to release
3. **Create draft releases only**: Never publish releases directly
4. **Manual changelog**: Generate the changelog by analyzing git commits yourself
5. **No automation**: Do not create or use GitHub Actions workflows

### Common Release Commands:
```bash
# Check current version
node -p "require('./package.json').version"

# List recent releases  
gh release list --limit 5

# Create release tarball (after updating version and creating changelog)
./scripts/create-release.sh

# Create GitHub draft release
gh release create vX.X.X --draft --prerelease --title "vX.X.X" --notes-file CHANGELOG.md pulse-vX.X.X.tar.gz
```

## Development Guidelines

### Code Style
- Use existing code patterns and conventions
- Don't add comments unless specifically requested
- Prefer editing existing files over creating new ones

### Testing
- Run linting after making changes: `npm run lint` (if available)
- Run type checking: `npm run typecheck` (if available)
- Test the application: `npm start` or `systemctl restart pulse`

### Git Workflow
- **Work directly on the `main` branch** - all commits go to main
- **Simple workflow** - no branch protection or complex rules
- Create descriptive commit messages focusing on the "why"
- Tag releases directly from main branch

### Important Files
- `/opt/pulse/` - Main application directory
- `package.json` - Version and dependencies
- `RELEASE_CHECKLIST.md` - Step-by-step release guide
- `scripts/create-release.sh` - Creates release tarballs

### Things to Avoid
- Creating GitHub Actions workflows
- Automating the release process
- Creating new documentation files unless requested
- Adding emojis unless requested
- Making version jumps (follow semantic versioning)

## Changelog Guidelines

When creating changelogs:
1. Analyze ALL commits since the last stable release
2. Filter out development-only changes:
   - CI/CD changes
   - Documentation updates  
   - Dependency updates
   - Test additions
   - Refactoring
   - Workflow changes
3. Focus on user-visible changes:
   - New features
   - Improvements to existing features
   - Bug fixes
   - UI/UX changes
4. Write clear, concise descriptions that end users will understand
5. Group into categories: Features, Improvements, Fixes

## Remember
- The user prefers simple, working solutions over complex automation
- Always check the release checklist when creating releases
- Create draft releases, never published ones
- Ask for clarification when unsure rather than making assumptions