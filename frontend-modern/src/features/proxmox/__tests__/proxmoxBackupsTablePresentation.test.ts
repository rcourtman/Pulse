import { describe, expect, it } from 'vitest';

import {
  getBackupServerColumns,
  getBackupServerColumnWidthStyle,
  getBackupServerLayoutForContainer,
  getCoverageColumns,
  getCoverageColumnWidthStyle,
  getCoverageLayoutForContainer,
  getRecoverableColumns,
  getRecoverableColumnWidthStyle,
  getRecoverableLayoutForContainer,
  isCoverageEvidenceColumnVisible,
} from '../proxmoxBackupsTablePresentation';

const ids = <T extends string>(columns: readonly { id: T }[]) => columns.map((column) => column.id);

describe('Proxmox backups responsive table presentation', () => {
  it('selects layouts from the table container instead of the viewport', () => {
    expect(getBackupServerLayoutForContainer(519)).toBe('compact');
    expect(getBackupServerLayoutForContainer(520)).toBe('basic');
    expect(getBackupServerLayoutForContainer(719)).toBe('basic');
    expect(getBackupServerLayoutForContainer(720)).toBe('operational');
    expect(getBackupServerLayoutForContainer(899)).toBe('operational');
    expect(getBackupServerLayoutForContainer(900)).toBe('expanded');
    expect(getBackupServerLayoutForContainer(1119)).toBe('expanded');
    expect(getBackupServerLayoutForContainer(1120)).toBe('full');

    for (const resolver of [getCoverageLayoutForContainer, getRecoverableLayoutForContainer]) {
      expect(resolver(479)).toBe('compact');
      expect(resolver(480)).toBe('basic');
      expect(resolver(719)).toBe('basic');
      expect(resolver(720)).toBe('operational');
      expect(resolver(879)).toBe('operational');
      expect(resolver(880)).toBe('expanded');
      expect(resolver(1119)).toBe('expanded');
      expect(resolver(1120)).toBe('full');
    }
  });

  it('keeps server reachability and datastore exhaustion risk visible first', () => {
    expect(ids(getBackupServerColumns('compact'))).toEqual([
      'server',
      'status',
      'datastore',
      'used',
      'backups',
    ]);
    expect(getBackupServerColumnWidthStyle('server', 'compact')).toEqual({ width: '40%' });
    expect(getBackupServerColumnWidthStyle('used', 'compact')).toEqual({ width: '20%' });
    expect(ids(getBackupServerColumns('basic'))).toEqual([
      'server',
      'status',
      'datastore',
      'used',
      'backups',
    ]);
    expect(ids(getBackupServerColumns('operational'))).toEqual([
      'server',
      'status',
      'cpu',
      'memory',
      'datastore',
      'used',
      'backups',
    ]);
    expect(ids(getBackupServerColumns('full'))).toHaveLength(10);
  });

  it('prioritizes workload posture, restore freshness, and task failures in coverage', () => {
    const everySource = { pbs: true, archive: true, snapshot: true, task: true };
    expect(ids(getCoverageColumns('compact', everySource))).toEqual([
      'workload',
      'posture',
      'latest',
      'pbs',
      'task',
    ]);
    expect(
      getCoverageColumnWidthStyle(
        'workload',
        'compact',
        ids(getCoverageColumns('compact', everySource)),
      ),
    ).toEqual({
      width: '40%',
    });
    expect(ids(getCoverageColumns('operational', everySource))).toEqual([
      'workload',
      'type',
      'node',
      'posture',
      'latest',
      'task',
    ]);
    expect(ids(getCoverageColumns('expanded', everySource))).toEqual([
      'workload',
      'type',
      'node',
      'posture',
      'latest',
      'pbs',
      'archive',
      'snapshot',
      'task',
    ]);
    expect(ids(getCoverageColumns('full', everySource))).toHaveLength(10);
  });

  it('does not reserve empty optional coverage columns', () => {
    expect(
      ids(
        getCoverageColumns('full', {
          pbs: true,
          archive: false,
          snapshot: false,
          task: true,
        }),
      ),
    ).toEqual(['workload', 'type', 'targetId', 'node', 'posture', 'latest', 'pbs', 'task']);
  });

  it('keeps the recovery-point answer visible before verbose metadata', () => {
    expect(ids(getRecoverableColumns('compact'))).toEqual([
      'workload',
      'source',
      'location',
      'created',
      'state',
    ]);
    expect(getRecoverableColumnWidthStyle('workload', 'compact')).toEqual({ width: '40%' });
    expect(getRecoverableColumnWidthStyle('state', 'compact')).toEqual({ width: '16%' });
    expect(ids(getRecoverableColumns('basic'))).toEqual([
      'workload',
      'source',
      'location',
      'created',
      'state',
    ]);
    expect(ids(getRecoverableColumns('expanded'))).not.toContain('details');
    expect(ids(getRecoverableColumns('full'))).toHaveLength(9);
  });

  it('progressively reveals restore-evidence detail columns', () => {
    expect(isCoverageEvidenceColumnVisible('compact', 'location')).toBe(false);
    expect(isCoverageEvidenceColumnVisible('basic', 'location')).toBe(true);
    expect(isCoverageEvidenceColumnVisible('basic', 'size')).toBe(false);
    expect(isCoverageEvidenceColumnVisible('operational', 'size')).toBe(true);
    expect(isCoverageEvidenceColumnVisible('operational', 'details')).toBe(false);
    expect(isCoverageEvidenceColumnVisible('expanded', 'details')).toBe(true);
  });
});
