import { describe, expect, it } from 'vitest';

import {
  getRecoveryPointPlatform,
  getRecoveryRollupPlatforms,
} from '@/utils/recoveryPlatformModel';

describe('recoveryPlatformModel', () => {
  it('prefers canonical platform fields when present', () => {
    expect(
      getRecoveryPointPlatform({
        platform: 'truenas',
        provider: 'proxmox-pve',
      }),
    ).toBe('truenas');

    expect(
      getRecoveryRollupPlatforms({
        platforms: ['truenas'],
        providers: ['proxmox-pve'],
      }),
    ).toEqual(['truenas']);
  });

  it('falls back to legacy provider fields when canonical fields are absent', () => {
    expect(
      getRecoveryPointPlatform({
        provider: 'proxmox-pbs',
      }),
    ).toBe('proxmox-pbs');

    expect(
      getRecoveryRollupPlatforms({
        providers: ['proxmox-pbs', 'kubernetes'],
      }),
    ).toEqual(['proxmox-pbs', 'kubernetes']);
  });
});
