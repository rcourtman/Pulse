import { describe, expect, it } from 'vitest';
import {
  getPatrolProviderSettingsAction,
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
});
