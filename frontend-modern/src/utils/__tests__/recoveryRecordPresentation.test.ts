import { describe, expect, it } from 'vitest';
import type { ProtectionRollup, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import {
  getRecoveryPointDetailsSummary,
  getRecoveryPointRepositoryLabel,
  getRecoveryPointSubjectLabel,
  getRecoveryPointTimestampMs,
  getRecoveryRollupSubjectLabel,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';

describe('recoveryRecordPresentation', () => {
  it('derives rollup and point subject labels from linked resources and refs', () => {
    const resources = new Map<string, Resource>([
      ['res-1', { id: 'res-1', type: 'vm', name: 'db-01' } as Resource],
    ]);

    const rollup = {
      rollupId: 'rollup-1',
      subjectResourceId: 'res-1',
    } as ProtectionRollup;
    const point = {
      id: 'point-1',
      subjectResourceId: 'res-1',
    } as RecoveryPoint;

    expect(getRecoveryRollupSubjectLabel(rollup, resources)).toBe('db-01');
    expect(getRecoveryPointSubjectLabel(point, resources)).toBe('db-01');
  });

  it('falls back to refs, repository labels, and details summary', () => {
    const resources = new Map<string, Resource>();
    const point = {
      id: 'point-2',
      subjectRef: { namespace: 'prod', name: 'api' },
      repositoryRef: { class: 'pbs', type: 'datastore' },
      display: { detailsSummary: 'Immutable and encrypted' },
    } as RecoveryPoint;

    expect(getRecoveryPointSubjectLabel(point, resources)).toBe('prod/api');
    expect(getRecoveryPointRepositoryLabel(point)).toBe('pbs:datastore');
    expect(getRecoveryPointDetailsSummary(point)).toBe('Immutable and encrypted');
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
});
