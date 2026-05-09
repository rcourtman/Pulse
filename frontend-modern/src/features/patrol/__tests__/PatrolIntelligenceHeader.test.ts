import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

import { getPatrolConfigurationFailureInlineDetails } from '../PatrolIntelligenceHeader';

const headerSource = readFileSync(
  resolve(__dirname, '..', 'PatrolIntelligenceHeader.tsx'),
  'utf-8',
);

describe('PatrolIntelligenceHeader', () => {
  it('keeps Patrol configuration readiness context visible inline', () => {
    expect(
      getPatrolConfigurationFailureInlineDetails({
        message: 'Patrol configuration could not be saved.',
        code: 'patrol_readiness_not_ready',
        readiness: {
          status: 'not_ready',
          cause: 'model_unsupported_tools',
          summary:
            'The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
          provider: 'openrouter',
          model: 'openrouter:deepseek/deepseek-r1',
        },
      }),
    ).toEqual([
      'patrol_readiness_not_ready · model_unsupported_tools',
      'Readiness: The selected Patrol model is a reasoning-only model family that commonly does not emit tool calls.',
      'Provider: openrouter',
      'Model: openrouter:deepseek/deepseek-r1',
    ]);
  });

  it('falls back to the blocked cause when readiness cause is absent', () => {
    expect(
      getPatrolConfigurationFailureInlineDetails({
        message: 'Patrol configuration could not be saved.',
        code: 'patrol_autonomy_pro_required',
        blockedCause: 'license_required',
      }),
    ).toEqual(['patrol_autonomy_pro_required · license_required']);
  });

  it('renders a compact trust summary in the page header above the runtime status row', () => {
    // Second wedge of the Patrol page IA reframe. The detailed Trust strip
    // in PatrolIntelligenceWorkspace stays as the canonical breakdown, but
    // operators should see "X active, Y regressed, Z fixes verified"
    // immediately under the page title without scrolling into the
    // workspace tabs. Pin the wiring so this trust-at-a-glance line cannot
    // silently regress to an empty header during refactors.
    expect(headerSource).toContain('aria-label="Patrol trust summary header"');
    expect(headerSource).toContain('state.patrolStatus()?.trust');
    expect(headerSource).toContain('trust.currently_active');
    expect(headerSource).toContain('trust.regressed_at_least_once');
    expect(headerSource).toContain('trust.fix_verified');
    // Visibility must gate on at least one non-zero signal so fresh installs
    // don't render an empty header strip.
    expect(headerSource).toContain('trust.currently_active > 0');
    expect(headerSource).toContain('trust.fix_verified > 0');
  });
});
