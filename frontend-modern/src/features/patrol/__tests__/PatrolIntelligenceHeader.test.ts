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

  it('keeps trust counters out of the page header chrome', () => {
    // The Patrol assessment strip owns the default status readout. The header
    // should stay focused on title, recency, and controls rather than adding
    // another active/regressed summary row.
    expect(headerSource).not.toContain('aria-label="Patrol trust summary header"');
    expect(headerSource).not.toContain('state.patrolStatus()?.trust');
    expect(headerSource).not.toContain('trust.currently_active');
    expect(headerSource).not.toContain('trust.regressed_at_least_once');
    expect(headerSource).not.toContain('trust.fix_verified');
  });
});
