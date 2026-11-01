import { describe, expect, it } from 'vitest';
import type { ProtectionPosture } from '@/types/recovery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import type { WorkloadCoverageRow, WorkloadRecoveryPosture } from '../proxmoxBackupRecoveryModel';
import { backupExceptionGroupKey, buildBackupVerdict } from '../proxmoxBackupVerdict';

const row = (
  label: string,
  posture: WorkloadRecoveryPosture,
  reason?: string,
  extra: Partial<WorkloadCoverageRow> = {},
): WorkloadCoverageRow => {
  const assessment: ProtectionPosture | undefined =
    posture === 'checking' || posture === 'not-evaluated'
      ? undefined
      : { ...protectionPosture(`vm:${label}`), state: posture, reason };
  return {
    key: `resource:${label}`,
    workload: {
      key: `resource:${label}`,
      resourceId: `vm:${label}`,
      type: 'vm',
      typeLabel: 'VM',
      vmid: '100',
      label,
      name: label,
    },
    artifacts: [],
    pbsCount: 0,
    archiveCount: 0,
    snapshotCount: 0,
    posture,
    postureRank: 0,
    protectionPosture: assessment,
    isOrphaned: false,
    ...extra,
  };
};

describe('buildBackupVerdict', () => {
  it('says all are protected when nothing is an exception', () => {
    const verdict = buildBackupVerdict(
      [row('a', 'protected', 'recent-verified'), row('b', 'protected', 'recent')],
      protectionPolicy,
    );
    expect(verdict.headline).toBe('All 2 protected');
    expect(verdict.tone).toBe('protected');
    expect(verdict.exceptions).toEqual([]);
  });

  it('groups exceptions by reason, worst state first, then by size', () => {
    const verdict = buildBackupVerdict(
      [
        row('web-01', 'protected', 'recent-verified'),
        row('db-02', 'attention', 'verification-overdue'),
        row('db-01', 'attention', 'verification-overdue'),
        row('mail', 'attention', 'stale'),
        row('scratch', 'unprotected', 'no-backup'),
        row('lab', 'unknown', 'history-unreadable'),
      ],
      protectionPolicy,
    );
    expect(verdict.headline).toBe('1 of 6 protected');
    expect(verdict.tone).toBe('unprotected');
    expect(verdict.exceptions.map((group) => [group.label, group.rows.length])).toEqual([
      ['No successful backup', 1],
      ['Not verified recently', 2],
      ['No backup in 7 days', 1],
      ['Backup history unreadable', 1],
    ]);
    expect(verdict.exceptions[1].rows.map((r) => r.workload.label)).toEqual(['db-01', 'db-02']);
  });

  it('falls back to one group per state when the reason is missing or unknown', () => {
    const verdict = buildBackupVerdict(
      [row('old', 'attention'), row('future', 'attention', 'a-reason-from-a-newer-server')],
      protectionPolicy,
    );
    expect(verdict.exceptions).toHaveLength(1);
    expect(verdict.exceptions[0]).toMatchObject({ key: 'attention', label: 'Needs attention' });
    expect(verdict.tone).toBe('attention');
  });

  it('leaves checking, not-evaluated and orphaned rows out of the rating', () => {
    const verdict = buildBackupVerdict(
      [
        row('a', 'protected', 'recent'),
        row('b', 'checking'),
        row('host', 'not-evaluated'),
        row('gone', 'attention', 'stale', { isOrphaned: true }),
      ],
      protectionPolicy,
    );
    expect(verdict).toMatchObject({
      headline: '1 of 1 protected',
      rated: 1,
      checking: 1,
      notEvaluated: 1,
      exceptions: [],
    });
  });

  it('says it is still checking before any assessment arrives', () => {
    const verdict = buildBackupVerdict([row('a', 'checking')], protectionPolicy);
    expect(verdict.headline).toBe('Checking backups…');
    expect(verdict.tone).toBe('checking');
  });

  it('names the stale window only when the policy gives one', () => {
    const verdict = buildBackupVerdict([row('a', 'attention', 'stale')], null);
    expect(verdict.exceptions[0].label).toBe('Backup overdue');
  });
});

describe('backupExceptionGroupKey', () => {
  it('has no group for protected or presentation-only states', () => {
    expect(backupExceptionGroupKey(row('a', 'protected', 'recent'))).toBeNull();
    expect(backupExceptionGroupKey(row('b', 'checking'))).toBeNull();
    expect(backupExceptionGroupKey(row('c', 'not-evaluated'))).toBeNull();
    expect(backupExceptionGroupKey(row('d', 'unprotected', 'snapshots-only'))).toBe(
      'snapshots-only',
    );
  });
});
