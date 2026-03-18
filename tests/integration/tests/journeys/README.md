# E2E Journey Tests

Orchestrator-authored Playwright tests covering v6 user journeys end-to-end.

These tests are created and maintained by the release orchestrator's L12 lane.
Each file covers one or more related user journeys and is independently runnable.

## Running

```bash
cd tests/integration
npx playwright test tests/journeys/
```

## Conventions

- File naming: `XX-journey-name.spec.ts`
- Use helpers from `../helpers.ts` (`ensureAuthenticated`, `login`, `apiRequest`, `setMockMode`, etc.)
- Mock mode enabled for deterministic data
- Tests run against `localhost:7655` (backend directly)
- Sequential execution (`workers: 1`, `fullyParallel: false`)
