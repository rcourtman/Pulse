import { describe, expect, it } from 'vitest';

import {
  getRecoveryPointPlatform,
  getRecoveryRollupPlatforms,
  normalizeRecoveryPoint,
  normalizeRecoveryPointsResponse,
  normalizeRecoveryRollup,
  normalizeRecoveryRollupsResponse,
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

  it('normalizes legacy recovery transport records into canonical runtime models', () => {
    expect(
      normalizeRecoveryPoint({
        id: 'point-1',
        provider: 'proxmox-pbs',
        kind: 'backup',
        mode: 'remote',
        outcome: 'success',
        display: {
          subjectLabel: 'Archive VM',
          subjectType: 'proxmox-vm',
        },
      }),
    ).toEqual({
      id: 'point-1',
      platform: 'proxmox-pbs',
      kind: 'backup',
      mode: 'remote',
      outcome: 'success',
      display: {
        itemLabel: 'Archive VM',
        itemType: 'proxmox-vm',
      },
    });

    expect(
      normalizeRecoveryRollup({
        rollupId: 'rollup-1',
        lastOutcome: 'success',
        providers: ['proxmox-pbs', 'kubernetes'],
        display: {
          subjectLabel: 'Legacy Dataset',
          subjectType: 'truenas-dataset',
        },
      }),
    ).toEqual({
      rollupId: 'rollup-1',
      lastOutcome: 'success',
      platforms: ['proxmox-pbs', 'kubernetes'],
      display: {
        itemLabel: 'Legacy Dataset',
        itemType: 'truenas-dataset',
      },
    });
  });

  it('normalizes recovery transport responses to platform-first data', () => {
    expect(
      normalizeRecoveryPointsResponse({
        data: [
          {
            id: 'point-1',
            provider: 'truenas',
            kind: 'snapshot',
            mode: 'snapshot',
            outcome: 'success',
          },
        ],
        meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
      }),
    ).toEqual({
      data: [
        {
          id: 'point-1',
          platform: 'truenas',
          kind: 'snapshot',
          mode: 'snapshot',
          outcome: 'success',
        },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    });

    expect(
      normalizeRecoveryRollupsResponse({
        data: [
          {
            rollupId: 'rollup-1',
            lastOutcome: 'warning',
            providers: ['truenas'],
          },
        ],
        meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
      }),
    ).toEqual({
      data: [
        {
          rollupId: 'rollup-1',
          lastOutcome: 'warning',
          platforms: ['truenas'],
        },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    });
  });
});
