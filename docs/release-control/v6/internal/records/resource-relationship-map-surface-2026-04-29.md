# Resource Relationship Map Surface

Date: 2026-04-29
Lane: L19 resource change intelligence
Follow-up: `resource-change-intelligence-post-rc-hardening`

## Outcome

The resource drawer now surfaces canonical resource relationships as a first-class Relationship map, using `resource.relationships` from the unified resource payload and the shared `ResourceCorrelationSummary` / `resourceCorrelationPresentation` ownership path. The relationship map sits beside change/action history instead of being hidden in the AI investigation disclosure, so deterministic infrastructure relationships are visible without requiring AI context.

The resource facet endpoint also returns the selected resource's canonical relationships and capabilities, so the drawer can hydrate the relationship map from the same resource-facet contract that owns recent changes and facet counts.

## Proof

- `npm --prefix frontend-modern test -- src/components/Infrastructure/__tests__/ResourceCorrelationSummary.test.tsx src/utils/__tests__/resourceCorrelationPresentation.test.ts src/components/Infrastructure/__tests__/ResourceDetailDrawer.history.test.tsx`
- `npm --prefix frontend-modern run lint:eslint -- --quiet`
- `python3 scripts/release_control/status_audit.py --pretty`
- `python3 scripts/release_control/registry_audit.py --check --staged`
- In-browser infrastructure drawer inspection on the local v6 app confirmed the existing drawer still renders cleanly and does not show an empty relationship map when the selected resource has no relationship facets.
