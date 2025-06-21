# Git Workflow Protection

This repository has git hooks in place to prevent accidental branch contamination.

## Protected Workflows

### Pre-merge Protection
The `pre-merge-commit` hook prevents accidentally merging `main` into `develop`, which can overwrite newer code with older versions.

**Correct merge flow:**
- `develop` → `main` (when releasing)
- `main` → `develop` (only for hotfixes that main has but develop needs)

**If you need to override:**
```bash
# Use --no-verify flag
git merge main --no-verify

# Or temporarily disable the hook
mv .git/hooks/pre-merge-commit .git/hooks/pre-merge-commit.disabled
# Do your merge
mv .git/hooks/pre-merge-commit.disabled .git/hooks/pre-merge-commit
```

### Pre-push Protection
The `pre-push` hook warns when pushing merges from main into develop, giving you a chance to review before pushing.

## Setting Up Hooks on a New Clone

Git hooks are not tracked by git. To set up these protections on a new clone:

```bash
# Copy the hooks from this documentation
cp .github/hooks/* .git/hooks/
chmod +x .git/hooks/*
```

## Branch Strategy

1. **main**: Production-ready code
2. **develop**: Active development, always ahead of main
3. **feature/***: Feature branches (merge into develop)
4. **hotfix/***: Emergency fixes (merge into main, then main into develop)

## Preventing This Issue on GitHub

For additional protection, consider setting up branch protection rules on GitHub:

1. Go to Settings → Branches
2. Add rule for `develop`
3. Enable:
   - "Require pull request reviews before merging"
   - "Dismiss stale pull request approvals when new commits are pushed"
   - "Require branches to be up to date before merging"

This ensures all merges into develop go through PR review.