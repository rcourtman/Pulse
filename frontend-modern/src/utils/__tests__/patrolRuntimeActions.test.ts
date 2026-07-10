import { describe, expect, it } from 'vitest';
import {
  getPatrolProviderSettingsAction,
  getPatrolSetupAction,
  getPatrolSetupHint,
  PATROL_PROVIDER_SETTINGS_ACTION,
} from '@/utils/patrolRuntimeActions';

describe('patrolRuntimeActions', () => {
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
    for (const cause of ['assistant_disabled', 'provider_not_configured']) {
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

  it('suppresses the tool-check hint for config-level causes', () => {
    expect(getPatrolSetupHint('assistant_disabled')).toBe('');
    expect(getPatrolSetupHint('provider_not_configured')).toBe('');
    expect(getPatrolSetupHint('model_not_selected')).toContain('run the model check');
    expect(getPatrolSetupHint(undefined)).toContain('run the model check');
  });
});
