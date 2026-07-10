import { describe, expect, it } from 'vitest';

import type { PBSBackup } from '@/types/api';
import type { Resource } from '@/types/resource';

import {
  buildProxmoxBackupRecoveryModel,
  CURRENT_RECOVERY_MS,
  getRecoveryAgeBand,
  STALE_RECOVERY_MS,
} from '../proxmoxBackupRecoveryModel';

// ---------------------------------------------------------------------------
// Fixture builders (same shapes as the sibling test file)
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
    lastSeen: Date.parse('2026-07-10T00:00:00Z'),
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

const NOW = Date.parse('2026-07-10T00:00:00Z');

const emptyModel = (pbsBackups: PBSBackup[], workloads: Resource[] = []) =>
  buildProxmoxBackupRecoveryModel({
    workloads,
    pbsBackups,
    archives: [],
    snapshots: [],
    tasks: [],
    nowMs: NOW,
  });

// ---------------------------------------------------------------------------
// getRecoveryAgeBand — threshold boundaries & non-finite inputs
// ---------------------------------------------------------------------------

describe('getRecoveryAgeBand boundary coverage', () => {
  it('returns "current" at exactly the 7-day boundary (inclusive <=)', () => {
    expect(getRecoveryAgeBand(NOW - CURRENT_RECOVERY_MS, NOW)).toBe('current');
  });

  it('returns "aging" one millisecond past the current boundary', () => {
    expect(getRecoveryAgeBand(NOW - CURRENT_RECOVERY_MS - 1, NOW)).toBe('aging');
  });

  it('returns "aging" at exactly the 30-day boundary (inclusive <=)', () => {
    expect(getRecoveryAgeBand(NOW - STALE_RECOVERY_MS, NOW)).toBe('aging');
  });

  it('returns "stale" one millisecond past the stale boundary', () => {
    expect(getRecoveryAgeBand(NOW - STALE_RECOVERY_MS - 1, NOW)).toBe('stale');
  });

  it('returns "current" for a zero-age artifact (createdMs === nowMs)', () => {
    expect(getRecoveryAgeBand(NOW, NOW)).toBe('current');
  });
});

describe('getRecoveryAgeBand non-finite inputs', () => {
  it('returns "unknown" for NaN createdMs', () => {
    expect(getRecoveryAgeBand(NaN, NOW)).toBe('unknown');
  });

  it('returns "unknown" for Infinity createdMs', () => {
    expect(getRecoveryAgeBand(Infinity, NOW)).toBe('unknown');
  });

  it('returns "unknown" when the computed age is non-finite (NaN nowMs)', () => {
    // createdMs is finite so it passes the first guard, but ageMs = NaN - 1000 = NaN
    expect(getRecoveryAgeBand(1000, NaN)).toBe('unknown');
  });
});

// ---------------------------------------------------------------------------
// parseTimestampMs — seconds vs ms vs unparseable (observed via createdMs)
// ---------------------------------------------------------------------------

describe('parseTimestampMs coverage (via PBS backupTime -> createdMs)', () => {
  it('parses a valid ISO 8601 timestamp into epoch milliseconds', () => {
    const ts = '2026-07-09T12:30:45Z';
    const model = emptyModel([pbsBackup({ backupTime: ts })]);
    expect(model.recoverableArtifacts[0].createdMs).toBe(Date.parse(ts));
  });

  it('returns undefined createdMs for an empty timestamp string', () => {
    const model = emptyModel([pbsBackup({ backupTime: '' })]);
    expect(model.recoverableArtifacts[0].createdMs).toBeUndefined();
  });

  it('returns undefined createdMs for an unparseable garbage string', () => {
    const model = emptyModel([pbsBackup({ backupTime: 'not-a-date' })]);
    expect(model.recoverableArtifacts[0].createdMs).toBeUndefined();
  });

  it('does not interpret a bare epoch-seconds string as a valid timestamp', () => {
    // Date.parse('1752019200') returns NaN — the function must not confuse
    // raw epoch-seconds with an ISO date.
    const model = emptyModel([pbsBackup({ backupTime: '1752019200' })]);
    expect(model.recoverableArtifacts[0].createdMs).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// fallbackWorkloadId — host zero-vmid hint paths (via PBS host backups)
// ---------------------------------------------------------------------------

describe('fallbackWorkloadId coverage (via host PBS backups with no inventory)', () => {
  it('falls back to the first non-root PBS hint when host vmid is zero', () => {
    // hints = [namespace, datastore] = ['root', 'main']; 'root' filtered, 'main' survives
    const model = emptyModel([
      pbsBackup({
        id: 'host-zero-root-ns',
        backupType: 'host',
        vmid: '0',
        namespace: 'root',
        datastore: 'main',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.vmid).toBe('main');
  });

  it('returns empty vmid when every host hint is root-like', () => {
    // firstHostHint filters both 'root' and '(root)'
    const model = emptyModel([
      pbsBackup({
        id: 'host-root-only',
        backupType: 'host',
        vmid: '0',
        namespace: 'root',
        datastore: '(root)',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.vmid).toBe('');
  });

  it('returns empty vmid when host hints are all blank', () => {
    // firstHostHint trims to '' which is falsy
    const model = emptyModel([
      pbsBackup({
        id: 'host-blank-hints',
        backupType: 'host',
        vmid: '0',
        namespace: '',
        datastore: '',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.vmid).toBe('');
  });
});

// ---------------------------------------------------------------------------
// workloadFallbackLabel — label branches for host-no-id, unknown-no-id,
// vm-no-id (existing tests already cover "LXC backup" and "<Type> <id>")
// ---------------------------------------------------------------------------

describe('workloadFallbackLabel coverage (via fallback workloads)', () => {
  it('renders "Host backup" for a host with no resolvable id', () => {
    const model = emptyModel([
      pbsBackup({
        id: 'host-no-id',
        backupType: 'host',
        vmid: '0',
        namespace: '',
        datastore: '',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.label).toBe('Host backup');
  });

  it('renders "Guest" for an unknown backup type with no vmid', () => {
    const model = emptyModel([
      pbsBackup({
        id: 'unknown-no-id',
        backupType: 'backup',
        vmid: '0',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.label).toBe('Guest');
  });

  it('renders "VM backup" for a VM with no vmid', () => {
    const model = emptyModel([
      pbsBackup({
        id: 'vm-no-id',
        backupType: 'qemu',
        vmid: '0',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.label).toBe('VM backup');
  });
});

// ---------------------------------------------------------------------------
// resourceVmid — meta vs platformData precedence (via candidate vmid)
// ---------------------------------------------------------------------------

describe('resourceVmid coverage (via buildCandidateFromResource)', () => {
  it('prefers proxmox meta vmid over platformData', () => {
    const model = emptyModel([], [
      workload({
        id: 'vm-300',
        proxmox: { vmid: 300, node: 'node-a' },
        platformData: { proxmox: { vmid: 999 } },
      }),
    ]);
    expect(model.coverageRows[0].workload.vmid).toBe('300');
  });

  it('reads a numeric vmid from platformData.proxmox when meta vmid is absent', () => {
    const model = emptyModel([], [
      workload({
        id: 'vm-500',
        proxmox: { node: 'node-a' },
        platformData: { proxmox: { vmid: 500 } },
      }),
    ]);
    expect(model.coverageRows[0].workload.vmid).toBe('500');
  });

  it('reads a string vmid from platformData.proxmox when meta vmid is absent', () => {
    const model = emptyModel([], [
      workload({
        id: 'vm-600',
        proxmox: { node: 'node-a' },
        platformData: { proxmox: { vmid: '600' } },
      }),
    ]);
    expect(model.coverageRows[0].workload.vmid).toBe('600');
  });

  it('produces no candidate when neither meta nor platformData provides a vmid', () => {
    const model = emptyModel([], [
      workload({
        id: 'vm-noid',
        proxmox: { node: 'node-a' },
      }),
    ]);
    expect(model.coverageRows).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// backupTypeLabel — qemu/vm/lxc/unknown branches (existing tests cover
// 'ct' and 'host')
// ---------------------------------------------------------------------------

describe('backupTypeLabel coverage (via PBS backupType)', () => {
  it.each([
    ['qemu', 'vm'],
    ['vm', 'vm'],
    ['lxc', 'ct'],
    ['backup', 'unknown'],
  ])('maps backupType "%s" to workload type "%s"', (backupType, expected) => {
    const model = emptyModel([
      pbsBackup({
        id: `pbs-${backupType}`,
        backupType,
        vmid: '0',
      }),
    ]);
    expect(model.recoverableArtifacts[0].workload.type).toBe(expected);
  });
});
