import { describe, expect, it } from 'vitest';
import {
  getPatrolProviderSettingsAction,
  getPatrolSetupAction,
  getPatrolSetupHint,
  isPatrolConfigLevelCause,
  isPatrolConnectCause,
  PATROL_BUDGET_EXHAUSTED_CAUSE,
  PATROL_PROVIDER_SETTINGS_ACTION,
} from '@/utils/patrolRuntimeActions';

describe('patrolRuntimeActions', () => {
  it('names the causes the Patrol page can fix in place by connecting a provider', () => {
    expect(isPatrolConfigLevelCause('provider_not_configured')).toBe(true);
    expect(isPatrolConfigLevelCause('assistant_disabled')).toBe(true);
    expect(isPatrolConfigLevelCause('provider_auth')).toBe(false);
    expect(isPatrolConnectCause('provider_auth')).toBe(true);
    expect(isPatrolConnectCause('provider_not_configured')).toBe(true);
    expect(isPatrolConnectCause('model_unsupported_tools')).toBe(false);
    for (const cause of [
      'model_unsupported_tools',
      PATROL_BUDGET_EXHAUSTED_CAUSE,
      'none',
      '',
      undefined,
    ]) {
      expect(isPatrolConfigLevelCause(cause)).toBe(false);
    }
  });

  it('centralizes the Patrol provider settings action', () => {
    expect(PATROL_PROVIDER_SETTINGS_ACTION).toEqual({
      label: 'Check Patrol model',
      href: '/settings/pulse-intelligence/patrol',
    });

    const action = getPatrolProviderSettingsAction();
    expect(action).toEqual(PATROL_PROVIDER_SETTINGS_ACTION);
    expect(action).not.toBe(PATROL_PROVIDER_SETTINGS_ACTION);
  });

  it('routes config-level causes to Provider & Models instead of the model check', () => {
    // A refused key is fixed where the key lives, not by re-running the model check.
    for (const cause of ['assistant_disabled', 'provider_not_configured', 'provider_auth']) {
      expect(getPatrolSetupAction(cause)).toEqual({
        label: 'Open Provider & Models',
        href: '/settings/pulse-intelligence/provider',
      });
    }
  });

  it('keeps the model check action for model-level and unknown causes', () => {
    for (const cause of ['model_not_selected', 'model_unsupported_tools', undefined, '']) {
      expect(getPatrolSetupAction(cause)).toEqual(PATROL_PROVIDER_SETTINGS_ACTION);
    }
  });

  it('routes an exhausted cost budget to the budget field, not the model check (#1789)', () => {
    expect(getPatrolSetupAction(PATROL_BUDGET_EXHAUSTED_CAUSE)).toEqual({
      label: 'Raise the cost budget',
      href: '/settings/pulse-intelligence/provider',
    });
    expect(getPatrolSetupHint(PATROL_BUDGET_EXHAUSTED_CAUSE)).toBe('');
  });

  it('suppresses the tool-check hint for config-level causes', () => {
    expect(getPatrolSetupHint('assistant_disabled')).toBe('');
    expect(getPatrolSetupHint('provider_not_configured')).toBe('');
    expect(getPatrolSetupHint('model_not_selected')).toContain('run the model check');
    expect(getPatrolSetupHint(undefined)).toContain('run the model check');
  });
});
