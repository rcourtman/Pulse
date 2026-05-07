import { describe, expect, it } from 'vitest';

import { getPatrolConfigurationFailureInlineDetails } from '../PatrolIntelligenceHeader';

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
});
