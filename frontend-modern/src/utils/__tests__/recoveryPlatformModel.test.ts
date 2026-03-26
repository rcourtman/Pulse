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
        subjectResourceId: 'vm-123',
        subjectRef: {
          type: 'proxmox-vm',
          name: 'Archive VM',
        },
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
      itemResourceId: 'vm-123',
      itemRef: {
        type: 'proxmox-vm',
        name: 'Archive VM',
      },
      display: {
        itemLabel: 'Archive VM',
        itemType: 'proxmox-vm',
      },
    });

    expect(
      normalizeRecoveryRollup({
        rollupId: 'rollup-1',
        lastOutcome: 'success',
        subjectResourceId: 'vm-123',
        subjectRef: {
          type: 'truenas-dataset',
          name: 'Legacy Dataset',
        },
        providers: ['proxmox-pbs', 'kubernetes'],
        display: {
          subjectLabel: 'Legacy Dataset',
          subjectType: 'truenas-dataset',
        },
      }),
    ).toEqual({
      rollupId: 'rollup-1',
      lastOutcome: 'success',
      itemResourceId: 'vm-123',
      itemRef: {
        type: 'truenas-dataset',
        name: 'Legacy Dataset',
      },
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
            subjectResourceId: 'res-1',
            subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
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
            itemResourceId: 'res-1',
            itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
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
            subjectResourceId: 'res-1',
            subjectRef: { type: 'truenas-dataset', name: 'tank/apps' },
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
            itemResourceId: 'res-1',
            itemRef: { type: 'truenas-dataset', name: 'tank/apps' },
            platforms: ['truenas'],
          },
      ],
      meta: { page: 1, limit: 100, total: 1, totalPages: 1 },
    });
  });

  it('preserves display fallback data when degraded recovery metadata fields are omitted', () => {
    expect(
      normalizeRecoveryPoint({
        id: 'point-malformed',
        provider: 'kubernetes',
        kind: 'snapshot',
        mode: 'snapshot',
        outcome: 'success',
        subjectResourceId: 'pvc-1',
        display: {
          subjectLabel: 'default/data',
          subjectType: 'k8s-pvc',
          detailsSummary: 'Immutable copy',
        },
      }),
    ).toEqual({
      id: 'point-malformed',
      platform: 'kubernetes',
      kind: 'snapshot',
      mode: 'snapshot',
      outcome: 'success',
      itemResourceId: 'pvc-1',
      display: {
        itemLabel: 'default/data',
        itemType: 'k8s-pvc',
        detailsSummary: 'Immutable copy',
      },
    });

    expect(
      normalizeRecoveryRollup({
        rollupId: 'rollup-malformed',
        lastOutcome: 'warning',
        subjectResourceId: 'pvc-1',
        providers: ['kubernetes'],
        display: {
          subjectLabel: 'default/data',
          subjectType: 'k8s-pvc',
        },
      }),
    ).toEqual({
      rollupId: 'rollup-malformed',
      lastOutcome: 'warning',
      itemResourceId: 'pvc-1',
      platforms: ['kubernetes'],
      display: {
        itemLabel: 'default/data',
        itemType: 'k8s-pvc',
      },
    });
  });
});
