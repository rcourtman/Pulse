import { describe, expect, it } from 'vitest';
import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  getRecoveryPointItemLabel,
  getRecoveryPointItemSecondaryLabel,
  getRecoveryPointDetailsSummary,
  getRecoveryPointKindLabel,
  getRecoveryPointModeLabel,
  getRecoveryPointOutcomeLabel,
  getRecoveryPointRepositoryLabel,
  getRecoveryPointTimestampMs,
  getRecoveryRollupItemLabel,
  getRecoveryRollupItemSecondaryLabel,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';

describe('recoveryRecordPresentation', () => {
  it('derives rollup and point item labels from linked resources and refs', () => {
    const resources = new Map<string, Resource>([
      ['res-1', { id: 'res-1', type: 'vm', name: 'db-01' } as Resource],
    ]);

    const rollup = {
      rollupId: 'rollup-1',
      itemResourceId: 'res-1',
    } as ProtectionRollup;
    const point = {
      id: 'point-1',
      itemResourceId: 'res-1',
    } as RecoveryPoint;

    expect(getRecoveryRollupItemLabel(rollup, resources)).toBe('db-01');
    expect(getRecoveryPointItemLabel(point, resources)).toBe('db-01');
  });

  it('falls back to refs, repository labels, and details summary', () => {
    const resources = new Map<string, Resource>();
    const point = {
      id: 'point-2',
      subjectRef: { namespace: 'prod', name: 'api' },
      repositoryRef: { class: 'pbs', type: 'datastore' },
      display: { detailsSummary: 'Immutable and encrypted' },
    } as RecoveryPoint;

    expect(getRecoveryPointItemLabel(point, resources)).toBe('prod/api');
    expect(getRecoveryPointRepositoryLabel(point)).toBe('pbs:datastore');
    expect(getRecoveryPointDetailsSummary(point)).toBe('Immutable and encrypted');
  });

  it('prefers governed display labels over opaque linked-resource ids', () => {
    const resources = new Map<string, Resource>([
      [
        'res-2',
        {
          id: 'res-2',
          name: 'res-2',
          displayName: 'Payments API',
          type: 'vm',
        } as Resource,
      ],
      [
        'res-3',
        {
          id: 'res-3',
          name: 'res-3',
          type: 'vm',
        } as Resource,
      ],
    ]);

    const rollup = {
      rollupId: 'res:res-3',
      itemResourceId: 'res-3',
      display: { itemLabel: 'Archive VM' },
    } as ProtectionRollup;
    const linkedPoint = {
      id: 'point-3',
      itemResourceId: 'res-2',
      display: { itemLabel: 'billing-api' },
    } as RecoveryPoint;

    expect(getRecoveryRollupItemLabel(rollup, resources)).toBe('Archive VM');
    expect(getRecoveryPointItemLabel(linkedPoint, resources)).toBe('Payments API');
  });

  it('derives secondary entity-id labels without replacing the primary name', () => {
    const rollup = {
      rollupId: 'rollup-2',
      display: { itemLabel: 'Archive VM', itemType: 'vm', entityIdLabel: '101' },
    } as ProtectionRollup;
    const point = {
      id: 'point-4',
      display: { itemLabel: 'debian-go', itemType: 'system-container', entityIdLabel: '112' },
    } as RecoveryPoint;

    expect(getRecoveryRollupItemSecondaryLabel(rollup)).toBe('VMID 101');
    expect(getRecoveryPointItemSecondaryLabel(point)).toBe('CTID 112');
  });

  it('derives timestamps and normalizes mode query values', () => {
    const point = {
      completedAt: '2026-03-09T12:00:00.000Z',
    } as RecoveryPoint;

    expect(getRecoveryPointTimestampMs(point)).toBeGreaterThan(0);
    expect(normalizeRecoveryModeQueryValue('snapshot')).toBe('snapshot');
    expect(normalizeRecoveryModeQueryValue('LOCAL')).toBe('local');
    expect(normalizeRecoveryModeQueryValue('unknown')).toBe('all');
  });

  it('humanizes point kind, method, and outcome labels for operator-facing recovery details', () => {
    expect(getRecoveryPointKindLabel('backup')).toBe('Backup');
    expect(getRecoveryPointKindLabel('replica_sync')).toBe('Replica Sync');
    expect(getRecoveryPointModeLabel('local')).toBe('Local Copy');
    expect(getRecoveryPointModeLabel('zfs_send')).toBe('Zfs Send');
    expect(getRecoveryPointOutcomeLabel('success')).toBe('Success');
    expect(getRecoveryPointOutcomeLabel('FAILURE')).toBe('Failed');
    expect(getRecoveryPointOutcomeLabel('')).toBe('Unknown');
  });
});
