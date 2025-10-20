# Contributing to Pulse Installer Scripts

## Workflow Overview

1. **Plan** the change (refactor, new feature, bugfix) and confirm whether the
   shared libraries already support your needs.
2. **Implement** in the modular source file (e.g., `scripts/install-foo-v2.sh`).
3. **Add/Update tests** (smoke + integration where applicable).
4. **Bundle** (`make bundle-scripts`) and verify outputs.
5. **Document** any behavioural changes.
6. **Submit PR** with summary, testing results, and rollout considerations.

## Expectations

- Use shared libraries (`scripts/lib/*.sh`) instead of duplicating helpers.
- Maintain backward compatibility; introduce feature flags when needed.
- Keep legacy script versions until rollout completes.
- Ensure `scripts/tests/run.sh` (smoke) and relevant integration tests pass.
- Run `make lint-scripts` (shellcheck) before submitting.
- Update `scripts/bundle.manifest` and regenerate bundles.
- Provide before/after metrics when refactoring (size reduction, test coverage).

## Testing Checklist

- `scripts/tests/run.sh`
- Relevant `scripts/tests/integration/*` scripts (add new ones if needed)
- Manual `--dry-run` invocation of the script when feasible
- Bundle validation: `bash -n dist/<script>.sh` and `dist/<script>.sh --dry-run`

## Useful Commands

```bash
# Lint & format
make lint-scripts

# Run smoke tests
scripts/tests/run.sh

# Run integration tests
scripts/tests/integration/test-<name>.sh

# Rebuild bundles
make bundle-scripts
```

## Resources

- `docs/script-library-guide.md` — detailed patterns and examples
- `scripts/lib/README.md` — library function reference
- `docs/installer-v2-rollout.md` — rollout process for installers
- GitHub Discussions / internal Slack for questions
