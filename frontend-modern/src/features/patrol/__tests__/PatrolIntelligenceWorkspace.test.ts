import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const workspaceSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceWorkspace.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceWorkspace trust strip', () => {
  it('renders a Trust strip from state.patrolStatus().trust when at least one signal is non-zero', () => {
    // The Trust strip is the first operator-visible surface for the trust
    // metrics data layer (FindingsStore.GetTrustSummary). It must read from
    // state.patrolStatus()?.trust and gate visibility on at least one
    // non-zero signal so a fresh install does not show an empty strip.
    expect(workspaceSource).toContain('state.patrolStatus()?.trust');
    expect(workspaceSource).toContain('aria-label="Patrol trust summary"');
    expect(workspaceSource).toContain('trust.fix_verified');
    expect(workspaceSource).toContain('trust.auto_resolved');
    expect(workspaceSource).toContain('trust.dismissed_as_noise');
  });

  it('does not invent trust signals not on FindingsTrustSummary', () => {
    // The strip must read only fields defined on FindingsTrustSummary so
    // adding new strip categories goes through the contract update first.
    // No mention of arbitrary keys like "patrol_score" or "health_grade".
    expect(workspaceSource).not.toMatch(/trust\.patrol_score/);
    expect(workspaceSource).not.toMatch(/trust\.health_grade/);
  });
});
