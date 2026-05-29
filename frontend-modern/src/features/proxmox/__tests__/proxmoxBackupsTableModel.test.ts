import { describe, expect, it } from 'vitest';

import type { PBSBackup } from '@/types/api';

import {
  ARCHIVE_SORT_DEFAULT_DIRECTION,
  COVERAGE_SORT_DEFAULT_DIRECTION,
  PBS_SORT_DEFAULT_DIRECTION,
  RECOVERABLE_SORT_DEFAULT_DIRECTION,
  SNAPSHOT_SORT_DEFAULT_DIRECTION,
  TASK_SORT_DEFAULT_DIRECTION,
  classifyTaskStatus,
  cmpBool,
  cmpNumber,
  cmpString,
  formatDuration,
  formatDurationFromSeconds,
  guestLabel,
  pbsRepositoryLabel,
  pbsWorkloadLabel,
} from '../proxmoxBackupsTableModel';

const pbs = (overrides: Partial<PBSBackup>): PBSBackup => overrides as PBSBackup;

describe('proxmoxBackupsTableModel', () => {
  describe('classifyTaskStatus', () => {
    it('maps known statuses to variants and labels', () => {
      expect(classifyTaskStatus('ok')).toMatchObject({ variant: 'success', label: 'OK' });
      expect(classifyTaskStatus('SUCCESS')).toMatchObject({ variant: 'success', label: 'OK' });
      expect(classifyTaskStatus('running')).toMatchObject({ variant: 'warning', label: 'Running' });
      expect(classifyTaskStatus('error')).toMatchObject({ variant: 'danger', label: 'Failed' });
    });

    it('falls back to muted for empty or unknown statuses', () => {
      expect(classifyTaskStatus('')).toMatchObject({ variant: 'muted', label: '—' });
      expect(classifyTaskStatus('paused')).toMatchObject({ variant: 'muted', label: 'paused' });
    });
  });

  describe('label helpers', () => {
    it('formats guest labels by type', () => {
      expect(guestLabel('ct', 101)).toBe('CT 101');
      expect(guestLabel('vm', 202)).toBe('VM 202');
      expect(guestLabel(undefined, 303)).toBe('Guest 303');
    });

    it('formats PBS workload labels including host backups', () => {
      expect(pbsWorkloadLabel(pbs({ backupType: 'ct', vmid: '101' }))).toBe('CT 101');
      expect(pbsWorkloadLabel(pbs({ backupType: 'vm', vmid: '202' }))).toBe('VM 202');
      expect(pbsWorkloadLabel(pbs({ backupType: 'host', vmid: '' }))).toBe('Host');
      expect(pbsWorkloadLabel(pbs({ backupType: 'host', vmid: '9' }))).toBe('Host 9');
    });

    it('formats PBS repository labels with a root namespace fallback', () => {
      expect(pbsRepositoryLabel(pbs({ datastore: 'main', namespace: 'team' }))).toBe('main / team');
      expect(pbsRepositoryLabel(pbs({ datastore: 'main' }))).toBe('main / (root)');
    });
  });

  describe('duration formatting', () => {
    it('formats start/end pairs and rejects invalid ranges', () => {
      expect(formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:00:45Z')).toBe('45s');
      expect(formatDuration('2026-01-01T00:00:00Z', '2026-01-01T00:02:05Z')).toBe('2m 5s');
      expect(formatDuration('2026-01-01T00:00:00Z', '2026-01-01T01:30:00Z')).toBe('1h 30m');
      expect(formatDuration(undefined, '2026-01-01T00:00:45Z')).toBe('—');
      expect(formatDuration('2026-01-01T00:01:00Z', '2026-01-01T00:00:00Z')).toBe('—');
    });

    it('formats raw seconds', () => {
      expect(formatDurationFromSeconds(0)).toBe('—');
      expect(formatDurationFromSeconds(45)).toBe('45s');
      expect(formatDurationFromSeconds(125)).toBe('2m 5s');
      expect(formatDurationFromSeconds(5400)).toBe('1h 30m');
    });
  });

  describe('comparators', () => {
    it('sorts strings case-insensitively and pushes blanks last', () => {
      expect(cmpString('alpha', 'beta', 'asc')).toBeLessThan(0);
      expect(cmpString('alpha', 'beta', 'desc')).toBeGreaterThan(0);
      expect(cmpString('Zeta', 'alpha', 'asc')).toBeGreaterThan(0);
      expect(cmpString('', 'alpha', 'asc')).toBe(1);
      expect(cmpString('alpha', '', 'asc')).toBe(-1);
    });

    it('sorts numbers and pushes missing values last regardless of direction', () => {
      expect(cmpNumber(1, 2, 'asc')).toBeLessThan(0);
      expect(cmpNumber(1, 2, 'desc')).toBeGreaterThan(0);
      expect(cmpNumber(undefined, 5, 'asc')).toBe(1);
      expect(cmpNumber(undefined, 5, 'desc')).toBe(1);
      expect(cmpNumber(Number.NaN, 5, 'asc')).toBe(1);
    });

    it('sorts booleans with true first when descending', () => {
      // Negative means the first arg sorts ahead. desc => true ahead of false.
      expect(cmpBool(true, false, 'desc')).toBeLessThan(0);
      expect(cmpBool(true, false, 'asc')).toBeGreaterThan(0);
      expect(cmpBool(true, true, 'asc')).toBe(0);
    });
  });

  describe('default sort directions', () => {
    it('defaults numeric/timestamp columns to desc and string columns to asc', () => {
      expect(COVERAGE_SORT_DEFAULT_DIRECTION.workload).toBe('asc');
      expect(COVERAGE_SORT_DEFAULT_DIRECTION.latest).toBe('desc');
      expect(RECOVERABLE_SORT_DEFAULT_DIRECTION.size).toBe('desc');
      expect(PBS_SORT_DEFAULT_DIRECTION.verified).toBe('desc');
      expect(SNAPSHOT_SORT_DEFAULT_DIRECTION.guest).toBe('asc');
      expect(ARCHIVE_SORT_DEFAULT_DIRECTION.created).toBe('desc');
      expect(TASK_SORT_DEFAULT_DIRECTION.started).toBe('desc');
    });
  });
});
