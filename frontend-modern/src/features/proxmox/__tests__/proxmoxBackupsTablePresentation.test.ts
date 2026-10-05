import { describe, expect, it } from 'vitest';

import {
  getBackupServerColumns,
  getBackupServerColumnWidthStyle,
  getBackupServerLayoutForContainer,
  getBackupPostureRuleText,
  getCoverageColumns,
  getCoverageColumnWidthStyle,
  getCoverageLayoutForContainer,
  getRecoverableColumns,
  getRecoverableColumnWidthStyle,
  getRecoverableLayoutForContainer,
  isCoverageEvidenceColumnVisible,
  selectCoverageRestoreEvidence,
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
    ]);
    expect(getBackupServerColumnWidthStyle('server', 'compact')).toEqual({ width: '40%' });
    expect(getBackupServerColumnWidthStyle('status', 'compact')).toEqual({ width: '18%' });
    expect(getBackupServerColumnWidthStyle('used', 'compact')).toEqual({ width: '25%' });
    expect(getBackupServerColumnWidthStyle('datastore', 'compact')).toEqual({ width: '17%' });
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
    const withTasks = { task: true };
    expect(ids(getCoverageColumns('compact', withTasks))).toEqual([
      'workload',
      'posture',
      'latest',
      'task',
    ]);
    expect(
      getCoverageColumnWidthStyle(
        'workload',
        'compact',
        ids(getCoverageColumns('compact', withTasks)),
      ),
    ).toEqual({
      width: '40%',
    });
    expect(ids(getCoverageColumns('operational', withTasks))).toEqual([
      'workload',
      'type',
      'node',
      'posture',
      'latest',
      'task',
    ]);
    // Per-source ages (PBS snapshot, PVE file, guest snapshot) are expansion
    // detail at every width; the widest layout only adds the target id.
    expect(ids(getCoverageColumns('expanded', withTasks))).toEqual(
      ids(getCoverageColumns('operational', withTasks)),
    );
    expect(ids(getCoverageColumns('full', withTasks))).toEqual([
      'workload',
      'type',
      'targetId',
      'node',
      'posture',
      'latest',
      'task',
    ]);
  });

  it('does not reserve an empty task column', () => {
    expect(ids(getCoverageColumns('full', { task: false }))).toEqual([
      'workload',
      'type',
      'targetId',
      'node',
      'posture',
      'latest',
    ]);
  });

  it('states the backup health rule from the server posture policy', () => {
    expect(getBackupPostureRuleText({ freshnessWindowSeconds: 7 * 86_400 })).toBe(
      'Attention: the newest backup is older than 7 days, a newer backup job failed, expected verification is missing or overdue, or Pulse can see only part of the backup history. Unprotected: Pulse sees the full history and no backup it can count. Guest snapshots alone do not count. Unknown: Pulse cannot read the backup history or has no backup source linked to the guest.',
    );
    expect(getBackupPostureRuleText({ freshnessWindowSeconds: 86_400 })).toContain(
      'older than 1 day,',
    );
    expect(getBackupPostureRuleText({ freshnessWindowSeconds: 3_600 })).toContain(
      'older than 1 hour,',
    );
    expect(getBackupPostureRuleText({ freshnessWindowSeconds: 36 * 3_600 })).toContain(
      'older than 36 hours,',
    );
    expect(getBackupPostureRuleText({ freshnessWindowSeconds: 0 })).toBeNull();
    expect(getBackupPostureRuleText(null)).toBeNull();
  });

  it('keeps the newest restore point of every source in the coverage expansion', () => {
    const snapshots = Array.from({ length: 10 }, (_, index) => ({
      id: `snap-${index}`,
      sourceKind: 'snapshot',
      createdMs: 1_000 + index,
    }));
    const artifacts = [
      { id: 'pbs-old', sourceKind: 'pbs', createdMs: 10 },
      { id: 'pbs-new', sourceKind: 'pbs', createdMs: 500 },
      { id: 'archive', sourceKind: 'archive', createdMs: 400 },
      ...snapshots,
    ];

    const evidence = selectCoverageRestoreEvidence(artifacts);

    expect(evidence).toHaveLength(8);
    expect(evidence.map((artifact) => artifact.id)).toEqual([
      'snap-9',
      'snap-8',
      'snap-7',
      'snap-6',
      'snap-5',
      'snap-4',
      'pbs-new',
      'archive',
    ]);
    expect(selectCoverageRestoreEvidence(artifacts.slice(0, 3))).toHaveLength(3);
  });

  it('keeps the newest completed backup when a newer job is running or failed', () => {
    const snapshots = Array.from({ length: 10 }, (_, index) => ({
      id: `snap-${index}`,
      sourceKind: 'snapshot',
      createdMs: 1_000 + index,
    }));
    const artifacts = [
      { id: 'pbs-done', sourceKind: 'pbs', createdMs: 300 },
      { id: 'pbs-failed', sourceKind: 'pbs', createdMs: 600, failed: true },
      { id: 'pbs-running', sourceKind: 'pbs', createdMs: 700, running: true },
      ...snapshots,
    ];

    const ids = selectCoverageRestoreEvidence(artifacts).map((artifact) => artifact.id);

    expect(ids).toHaveLength(8);
    expect(ids).toContain('pbs-running');
    expect(ids).toContain('pbs-done');
    expect(ids).not.toContain('pbs-failed');
  });

  it('keeps the recovery-point answer visible before verbose metadata', () => {
    expect(ids(getRecoverableColumns('compact'))).toEqual([
      'workload',
      'source',
      'created',
      'state',
    ]);
    expect(getRecoverableColumnWidthStyle('workload', 'compact')).toEqual({ width: '40%' });
    expect(getRecoverableColumnWidthStyle('created', 'compact')).toEqual({ width: '21%' });
    expect(getRecoverableColumnWidthStyle('state', 'compact')).toEqual({ width: '23%' });
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
