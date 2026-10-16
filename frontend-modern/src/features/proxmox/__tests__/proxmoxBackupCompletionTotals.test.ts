import { describe, expect, it } from 'vitest';
import { buildProxmoxBackupRecoveryModel } from '../proxmoxBackupRecoveryModel';
import {
  BACKUP_IDENTITY_NOW,
  identityArchive,
  identityGuest,
  identityPBS,
  identitySnapshot,
} from '../__fixtures__/backupIdentity';

const input = () => ({
  workloads: [identityGuest('vm-100', 'east', 'pve-1')],
  pbsBackups: [identityPBS()],
  archives: [identityArchive()],
  snapshots: [identitySnapshot({ sizeBytes: 512 })],
  tasks: [],
  nowMs: BACKUP_IDENTITY_NOW,
});

describe('restore-point completion totals', () => {
  it.each([
    ['running PBS', { inProgress: true, writeActivityObserved: true, writeActive: true }],
    ['failed PBS', { inProgress: true, writeActivityObserved: true, writeActive: false }],
    ['unobserved PBS write', { inProgress: true }],
  ])('does not count %s as a restore point or PBS coverage', (_name, state) => {
    const model = buildProxmoxBackupRecoveryModel({
      ...input(),
      pbsBackups: [identityPBS(state)],
      archives: [],
      snapshots: [],
    });
    expect(model.coverageSummary.recoverableArtifacts).toBe(0);
    expect(model.coverageSummary).toMatchObject({
      incompleteArtifacts: 1,
      totalBytes: 0,
      withPBS: 0,
    });
    expect(model.recoverableArtifacts).toHaveLength(1);
    expect(model.coverageRows[0].artifacts).toHaveLength(1);
    expect(model.coverageRows[0].latestBackup).toBeUndefined();
  });

  it('keeps a running PVE file visible without adding recoverable bytes or points', () => {
    const model = buildProxmoxBackupRecoveryModel({
      ...input(),
      pbsBackups: [],
      snapshots: [],
      archives: [identityArchive({ inProgress: true })],
    });
    expect(model.coverageSummary.recoverableArtifacts).toBe(0);
    expect(model.coverageSummary).toMatchObject({ incompleteArtifacts: 1, totalBytes: 0 });
    expect(model.recoverableArtifacts[0].running).toBe(true);
  });

  it('counts completed PBS, PVE and guest-local snapshots separately from incomplete entries', () => {
    const model = buildProxmoxBackupRecoveryModel({
      ...input(),
      pbsBackups: [
        identityPBS(),
        identityPBS({ id: 'partial-pbs', inProgress: true, size: 999_999 }),
      ],
      archives: [
        identityArchive(),
        identityArchive({ id: 'partial-pve', inProgress: true, size: 999_999 }),
      ],
    });
    expect(model.coverageSummary.recoverableArtifacts).toBe(3);
    expect(model.coverageSummary).toMatchObject({
      incompleteArtifacts: 2,
      totalBytes: 3584,
      withPBS: 1,
    });
    expect(model.recoverableArtifacts).toHaveLength(5);
    expect(model.coverageRows[0].latestBackup?.nativeId).toBe('file-100');
    expect(model.coverageRows[0].latestSnapshot?.nativeId).toBe('snapshot-100');
  });

  it('does not confuse unknown dates or missing verification with incomplete writes', () => {
    const model = buildProxmoxBackupRecoveryModel({
      ...input(),
      pbsBackups: [identityPBS({ backupTime: '', verified: false })],
    });
    expect(model.coverageSummary.recoverableArtifacts).toBe(3);
    expect(model.coverageSummary).toMatchObject({
      incompleteArtifacts: 0,
      totalBytes: 3584,
      withPBS: 1,
    });
    expect(model.coverageRows[0].ageUnknown?.backup).toBe(true);
  });
});
