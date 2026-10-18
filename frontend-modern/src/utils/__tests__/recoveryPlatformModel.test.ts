import { describe, expect, it } from 'vitest';

import {
  getRecoveryPointPlatform,
  normalizeRecoveryPoint,
  normalizeRecoveryPointsResponse,
} from '@/utils/recoveryPlatformModel';

describe('recoveryPlatformModel', () => {
  it('prefers canonical platform fields when present', () => {
    expect(
      getRecoveryPointPlatform({
        platform: 'truenas',
        provider: 'proxmox-pve',
      }),
    ).toBe('truenas');
  });

  it('falls back to legacy provider fields when canonical fields are absent', () => {
    expect(
      getRecoveryPointPlatform({
        provider: 'proxmox-pbs',
      }),
    ).toBe('proxmox-pbs');
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
  });
});
