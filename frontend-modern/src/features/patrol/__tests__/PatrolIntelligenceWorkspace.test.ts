import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

const workspaceSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceWorkspace.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceWorkspace trust strip', () => {
  it('does not render a standalone Trust strip above the workspace tabs', () => {
    // The Patrol assessment strip owns the default active/regressed status
    // readout. The workspace should move directly into Findings/Runs instead
    // of repeating the same counters in a second strip.
    expect(workspaceSource).not.toContain('state.patrolStatus()?.trust');
    expect(workspaceSource).not.toContain('aria-label="Patrol trust summary"');
    expect(workspaceSource).not.toContain('trust.fix_verified');
    expect(workspaceSource).not.toContain('trust.auto_resolved');
    expect(workspaceSource).not.toContain('trust.dismissed_as_noise');
  });

  it('does not invent trust signals not on FindingsTrustSummary', () => {
    // No mention of arbitrary keys like "patrol_score" or "health_grade".
    expect(workspaceSource).not.toMatch(/trust\.patrol_score/);
    expect(workspaceSource).not.toMatch(/trust\.health_grade/);
  });
});
