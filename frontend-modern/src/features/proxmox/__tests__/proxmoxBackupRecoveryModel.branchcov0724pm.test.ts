import { describe, expect, it } from 'vitest';

import type { BackupTask, GuestSnapshot, PBSBackup, StorageBackup } from '@/types/api';
import type { Resource } from '@/types/resource';

import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  recoverableArtifactMatchesSearch,
} from '../proxmoxBackupRecoveryModel';

// ---------------------------------------------------------------------------
// Constants & helpers
// ---------------------------------------------------------------------------

const DAY_MS = 24 * 60 * 60 * 1000;
const NOW = Date.parse('2026-07-10T00:00:00Z');
const isoDaysAgo = (days: number): string => new Date(NOW - days * DAY_MS).toISOString();

// ---------------------------------------------------------------------------
// Fixture builders (same shapes as the sibling recovery-model test files)
// ---------------------------------------------------------------------------

const workload = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'vm-100',
    type: 'vm',
    name: 'web-01',
    displayName: 'web-01',
    platformId: 'pve-a',
    platformType: 'proxmox-pve',
    sourceType: 'api',
    status: 'running',
    lastSeen: NOW,
    proxmox: { vmid: 100, node: 'node-a' },
    ...overrides,
  }) as Resource;

const pbsBackup = (overrides: Partial<PBSBackup> = {}): PBSBackup => ({
  id: 'pbs/main/ns1/vm/100/2026-07-09T00:00:00Z',
  instance: 'pbs',
  datastore: 'main',
  namespace: 'ns1',
  backupType: 'vm',
  vmid: '100',
  backupTime: '2026-07-09T00:00:00Z',
  size: 1_000_000,
  protected: false,
  verified: true,
  files: ['index.json.blob'],
  owner: 'backup@pbs',
  ...overrides,
});

const archive = (overrides: Partial<StorageBackup> = {}): StorageBackup => ({
  id: 'archive-100',
  storage: 'local',
  node: 'node-a',
  instance: 'inst-a',
  type: 'ct',
  vmid: 100,
  time: isoDaysAgo(2),
  ctime: 0,
  size: 4096,
  format: 'zst',
  protected: false,
  volid: 'local:backup/vzdump-lxc-100.tar.zst',
  isPBS: false,
  verified: false,
  ...overrides,
});

const snapshot = (overrides: Partial<GuestSnapshot> = {}): GuestSnapshot => ({
  id: 'snap-100',
  name: 'pre-upgrade',
  node: 'node-a',
  instance: 'inst-a',
  type: 'ct',
  vmid: 100,
  time: isoDaysAgo(3),
  vmstate: false,
  ...overrides,
});

const task = (overrides: Partial<BackupTask> = {}): BackupTask => ({
  id: 'task-100',
  node: 'node-a',
  instance: 'inst-a',
  type: 'vm',
  vmid: 100,
  status: 'OK',
  startTime: isoDaysAgo(5),
  ...overrides,
});

type ModelInput = Parameters<typeof buildProxmoxBackupRecoveryModel>[0];

const buildModel = (
  input: Partial<ModelInput>,
): ReturnType<typeof buildProxmoxBackupRecoveryModel> =>
  buildProxmoxBackupRecoveryModel({
    workloads: [],
    pbsBackups: [],
    archives: [],
    snapshots: [],
    tasks: [],
    nowMs: NOW,
    ...input,
  });

// ---------------------------------------------------------------------------
// archive artifact fallback chains — location / detail / verified
// (proxmoxBackupRecoveryModel.ts lines 505, 506, 508). Existing specs always
// supply storage, node, volid and format, and use isPBS:false, so every
// fall-through arm of these `||` / ternary chains is uncovered.
// ---------------------------------------------------------------------------

describe('archive artifact location fallback chain (||)', () => {
  it('falls through to archive.node when storage is blank', () => {
    const model = buildModel({
      archives: [archive({ id: 'a-no-storage', storage: '', node: 'arch-node' })],
    });
    expect(model.recoverableArtifacts[0].location).toBe('arch-node');
  });

  it('falls through to the em-dash sentinel when both storage and node are blank', () => {
    const model = buildModel({
      archives: [archive({ id: 'a-blank-loc', storage: '', node: '' })],
    });
    expect(model.recoverableArtifacts[0].location).toBe('—');
  });
});

describe('archive artifact detail fallback chain (||)', () => {
  it('falls through to archive.format when volid is blank', () => {
    const model = buildModel({
      archives: [archive({ id: 'a-no-volid', volid: '', format: 'tgz' })],
    });
    expect(model.recoverableArtifacts[0].detail).toBe('tgz');
  });

  it('falls through to the source fallback label when volid and format are both blank', () => {
    const model = buildModel({
      archives: [archive({ id: 'a-blank-detail', volid: '', format: '' })],
    });
    expect(model.recoverableArtifacts[0].detail).toBe('PVE backup file');
  });
});

describe('archive artifact verified ternary (isPBS ? archive.verified : undefined)', () => {
  it('carries the verified flag through only for PBS-backed archives', () => {
    const model = buildModel({
      archives: [
        archive({ id: 'a-pbs-verified', isPBS: true, verified: true }),
        archive({ id: 'a-pve-only', isPBS: false, verified: true }),
      ],
    });
    const byId = new Map(model.recoverableArtifacts.map((a) => [a.nativeId, a]));
    expect(byId.get('a-pbs-verified')?.verified).toBe(true);
    // isPBS:false forces verified to undefined regardless of the raw flag.
    expect(byId.get('a-pve-only')?.verified).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// snapshot artifact fallback chains — location / detail
// (lines 529, 530). Existing specs always supply node and a snapshot name.
// ---------------------------------------------------------------------------

describe('snapshot artifact location fallback chain (||)', () => {
  it('falls through to snapshot.instance when node is blank', () => {
    const model = buildModel({
      snapshots: [snapshot({ id: 's-no-node', node: '', instance: 'snap-inst' })],
    });
    expect(model.recoverableArtifacts[0].location).toBe('snap-inst');
  });

  it('falls through to the em-dash sentinel when node and instance are blank', () => {
    const model = buildModel({
      snapshots: [snapshot({ id: 's-blank-loc', node: '', instance: '' })],
    });
    expect(model.recoverableArtifacts[0].location).toBe('—');
  });
});

describe('snapshot artifact detail fallback chain (||)', () => {
  it('falls back to the source label when description and name are both blank', () => {
    const model = buildModel({
      snapshots: [snapshot({ id: 's-blank-detail', name: '', description: undefined })],
    });
    expect(model.recoverableArtifacts[0].detail).toBe('Guest snapshot');
  });
});

// ---------------------------------------------------------------------------
// latestTask replacement comparison (line 548). Existing specs add at most one
// task per workload, so `!row.latestTask` always short-circuits and the
// `(candidateTask.startedMs ?? 0) > (row.latestTask.startedMs ?? 0)` comparison
// is never evaluated. We add multiple tasks for one workload.
// ---------------------------------------------------------------------------

describe('latestTask replacement comparison', () => {
  it('replaces the stored task when a newer one arrives for the same workload', () => {
    const model = buildModel({
      workloads: [workload({})],
      tasks: [
        task({ id: 't-old', startTime: isoDaysAgo(5) }),
        task({ id: 't-new', startTime: isoDaysAgo(1) }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.id).toBe('t-new');
  });

  it('keeps the newer task when an older one arrives afterwards', () => {
    const model = buildModel({
      workloads: [workload({})],
      tasks: [
        task({ id: 't-new', startTime: isoDaysAgo(1) }),
        task({ id: 't-old', startTime: isoDaysAgo(5) }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.id).toBe('t-new');
  });

  it('treats a task with an unparseable start time as epoch (0) when comparing', () => {
    // startedMs is undefined for the garbage timestamp -> ?? 0 -> always older.
    const model = buildModel({
      workloads: [workload({})],
      tasks: [
        task({ id: 't-valid', startTime: isoDaysAgo(2) }),
        task({ id: 't-garbage', startTime: 'not-a-date' }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.id).toBe('t-valid');
    expect(model.coverageRows[0].latestTask?.startedMs).toBe(Date.parse(isoDaysAgo(2)));
  });

  it('treats the stored task as epoch (0) when its start time is unparseable', () => {
    // The first task wins the slot with an undefined startedMs; the second
    // task's comparison then evaluates `(row.latestTask.startedMs ?? 0)`.
    const later = isoDaysAgo(1);
    const model = buildModel({
      workloads: [workload({})],
      tasks: [
        task({ id: 't-garbage-first', startTime: 'not-a-date' }),
        task({ id: 't-valid-later', startTime: later }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.id).toBe('t-valid-later');
    expect(model.coverageRows[0].latestTask?.startedMs).toBe(Date.parse(later));
  });
});

// ---------------------------------------------------------------------------
// recoverableArtifacts sort with undefined createdMs (line 589). Existing specs
// only ever produce artifacts with parseable timestamps, so neither `?? 0`
// default in the sort comparator is reached.
// ---------------------------------------------------------------------------

describe('recoverableArtifacts sort with undefined createdMs', () => {
  it('sorts a defined-createdMs artifact ahead of undefined-createdMs ones (?? 0)', () => {
    const model = buildModel({
      pbsBackups: [
        pbsBackup({ id: 'pbs-real', vmid: '100', backupTime: isoDaysAgo(1) }),
        pbsBackup({ id: 'pbs-bad-a', vmid: '100', backupTime: 'not-a-date' }),
        pbsBackup({ id: 'pbs-bad-b', vmid: '100', backupTime: '' }),
      ],
    });
    // Defined timestamp sorts first; the two undefined-createdMs artifacts
    // compare equal (0 - 0) and retain their stable insertion order.
    expect(model.recoverableArtifacts.map((a) => a.nativeId)).toEqual([
      'pbs-real',
      'pbs-bad-a',
      'pbs-bad-b',
    ]);
    expect(model.recoverableArtifacts[1].createdMs).toBeUndefined();
    expect(model.recoverableArtifacts[2].createdMs).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// coverageRowMatchesSearch / recoverableArtifactMatchesSearch — uncovered arms
// in the haystack builders: empty search term short-circuit (610, 638),
// `nativeNodeAliases ?? []` default for fallback/orphan workloads (618, 646),
// and the verified/protected ternary arms (653, 657).
// ---------------------------------------------------------------------------

describe('search short-circuit for an empty term', () => {
  it('coverageRowMatchesSearch returns true for an empty or whitespace term', () => {
    const model = buildModel({
      workloads: [workload({})],
      pbsBackups: [pbsBackup({ vmid: '100' })],
    });
    const row = model.coverageRows[0];
    expect(coverageRowMatchesSearch(row, '')).toBe(true);
    expect(coverageRowMatchesSearch(row, '   ')).toBe(true);
  });

  it('recoverableArtifactMatchesSearch returns true for an empty term', () => {
    const model = buildModel({
      pbsBackups: [pbsBackup({ vmid: '100' })],
    });
    expect(recoverableArtifactMatchesSearch(model.recoverableArtifacts[0], '')).toBe(true);
  });
});

describe('search haystack handles orphan workloads whose nativeNodeAliases are undefined', () => {
  it('matches an orphan coverage row by vmid (nativeNodeAliases ?? [])', () => {
    // vmid 777 has no inventory guest -> fallback workload with no
    // nativeNodeAliases field, exercising the `?? []` default.
    const model = buildModel({
      pbsBackups: [pbsBackup({ id: 'pbs-777', vmid: '777', backupType: 'ct' })],
    });
    const orphanRow = model.coverageRows[0];
    expect(orphanRow.workload.nativeNodeAliases).toBeUndefined();
    expect(coverageRowMatchesSearch(orphanRow, '777')).toBe(true);
    expect(coverageRowMatchesSearch(orphanRow, 'absent-token-xyz')).toBe(false);
  });

  it('matches an orphan recoverable artifact by vmid (nativeNodeAliases ?? [])', () => {
    const model = buildModel({
      pbsBackups: [pbsBackup({ id: 'pbs-888', vmid: '888', backupType: 'ct' })],
    });
    const orphanArtifact = model.recoverableArtifacts[0];
    expect(orphanArtifact.workload.nativeNodeAliases).toBeUndefined();
    expect(recoverableArtifactMatchesSearch(orphanArtifact, '888')).toBe(true);
  });
});

describe('recoverableArtifactMatchesSearch verified/protected ternary arms', () => {
  it('exposes the "verified" token only for a verified artifact', () => {
    const model = buildModel({
      pbsBackups: [
        pbsBackup({ id: 'pbs-verified', vmid: '100', verified: true }),
        pbsBackup({ id: 'pbs-unverified', vmid: '100', verified: false }),
      ],
    });
    const byId = new Map(model.recoverableArtifacts.map((a) => [a.nativeId, a]));
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-verified')!, 'verified')).toBe(true);
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-verified')!, 'unverified')).toBe(false);
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-unverified')!, 'unverified')).toBe(true);
  });

  it('matches neither verified token when the artifact verification state is unknown', () => {
    // A snapshot artifact always has verified === undefined (neither true nor
    // false), so the nested ternary collapses to the `undefined` arm and no
    // verification token is added to the haystack.
    const model = buildModel({
      snapshots: [snapshot({ id: 'snap-unknown', vmid: 100, name: 'pre-upgrade' })],
    });
    const artifact = model.recoverableArtifacts[0];
    expect(artifact.verified).toBeUndefined();
    expect(recoverableArtifactMatchesSearch(artifact, 'verified')).toBe(false);
    expect(recoverableArtifactMatchesSearch(artifact, 'unverified')).toBe(false);
    // The artifact is still discoverable through other haystack fields.
    expect(recoverableArtifactMatchesSearch(artifact, 'pre-upgrade')).toBe(true);
  });

  it('exposes the "protected" token for a protected artifact and "unprotected" otherwise', () => {
    const model = buildModel({
      pbsBackups: [
        pbsBackup({ id: 'pbs-protected', vmid: '100', protected: true }),
        pbsBackup({ id: 'pbs-open', vmid: '100', protected: false }),
      ],
    });
    const byId = new Map(model.recoverableArtifacts.map((a) => [a.nativeId, a]));
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-protected')!, 'protected')).toBe(true);
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-protected')!, 'unprotected')).toBe(false);
    expect(recoverableArtifactMatchesSearch(byId.get('pbs-open')!, 'unprotected')).toBe(true);
  });
});
