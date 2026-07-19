import { describe, expect, it } from 'vitest';

import type { BackupTask, PBSBackup } from '@/types/api';
import type { ProtectionPosture, ProtectionState } from '@/types/recovery';
import type { Resource } from '@/types/resource';

import {
  buildProxmoxBackupRecoveryModel,
  getWorkloadRecoveryPostureLabel,
  type ProxmoxBackupRecoveryModel,
  type WorkloadRecoveryPosture,
} from '../proxmoxBackupRecoveryModel';

// ---------------------------------------------------------------------------
// Constants & helpers
// ---------------------------------------------------------------------------

const DAY_MS = 24 * 60 * 60 * 1000;
const NOW = Date.parse('2026-07-10T00:00:00Z');
const isoDaysAgo = (days: number): string => new Date(NOW - days * DAY_MS).toISOString();

// ---------------------------------------------------------------------------
// Fixture builders (same shapes as the sibling test files)
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

const posture = (resourceId: string, state: ProtectionState): ProtectionPosture => ({
  subjectResourceId: resourceId,
  state,
  freshness: state === 'protected' ? 'current' : 'unknown',
  verification: state === 'protected' ? 'verified' : 'unknown',
  coverage: state === 'unprotected' ? 'none' : state === 'unknown' ? 'unknown' : 'complete',
  providerStates: [],
  repositoryResourceIds: [],
  evidenceIds: [],
  explanation: `Canonical ${state} fixture`,
  evaluatedAt: new Date(NOW).toISOString(),
});

const buildModel = (input: Partial<ModelInput>): ProxmoxBackupRecoveryModel => {
  const workloads = input.workloads ?? [];
  const protectionPostures =
    input.protectionPostures ??
    new Map(workloads.map((resource) => [resource.id, posture(resource.id, 'protected')]));
  return buildProxmoxBackupRecoveryModel({
    workloads: [],
    pbsBackups: [],
    archives: [],
    snapshots: [],
    tasks: [],
    nowMs: NOW,
    ...input,
    protectionPostures,
  });
};

// ---------------------------------------------------------------------------
// getWorkloadRecoveryPostureLabel — uncovered switch cases
// (sibling tests already exercise 'current' and 'failed')
// ---------------------------------------------------------------------------

describe('getWorkloadRecoveryPostureLabel uncovered switch cases', () => {
  it.each<[WorkloadRecoveryPosture, string]>([
    ['protected', 'Protected'],
    ['attention', 'Needs attention'],
    ['unprotected', 'Unprotected'],
    ['unknown', 'Unknown'],
  ])('renders posture %s as %j', (posture, expected) => {
    expect(getWorkloadRecoveryPostureLabel(posture)).toBe(expected);
  });
});

describe('canonical protection posture ownership', () => {
  it('uses the server posture even when raw artifacts suggest a different answer', () => {
    const resource = workload({});
    const model = buildModel({
      workloads: [resource],
      pbsBackups: [pbsBackup({ backupTime: isoDaysAgo(1), verified: true })],
      protectionPostures: new Map([[resource.id, posture(resource.id, 'attention')]]),
    });
    expect(model.coverageRows[0].posture).toBe('attention');
    expect(model.coverageRows[0].postureRank).toBe(0);
    expect(model.coverageRows[0].protectionPosture?.explanation).toBe(
      'Canonical attention fixture',
    );
  });

  it('reports unknown when no canonical posture is available', () => {
    const model = buildModel({
      workloads: [workload({})],
      protectionPostures: new Map(),
    });
    expect(model.coverageRows[0].posture).toBe('unknown');
    expect(model.coverageRows[0].postureRank).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// matchWorkloadByHints — all branches (private; exercised through tasks whose
// type maps to "unknown", e.g. vzdump, forcing the hint-matching path).
// ---------------------------------------------------------------------------

describe('matchWorkloadByHints branches (via vzdump tasks)', () => {
  it('returns the hint-matched candidate among several same-vmid workloads', () => {
    const model = buildModel({
      workloads: [
        workload({ id: 'vm-420a', proxmox: { vmid: 420, node: 'alpha' } }),
        workload({ id: 'vm-420b', proxmox: { vmid: 420, node: 'beta' } }),
      ],
      tasks: [
        task({
          id: 't-420',
          type: 'vzdump',
          vmid: 420,
          node: 'alpha',
          instance: '',
          status: 'OK',
        }),
      ],
    });
    const alpha = model.coverageRows.find((row) => row.workload.node === 'alpha');
    const beta = model.coverageRows.find((row) => row.workload.node === 'beta');
    expect(alpha?.latestTask?.label).toBe('OK');
    expect(beta?.latestTask).toBeUndefined();
  });

  it('returns undefined when multiple candidates share a vmid but no hint matches', () => {
    const model = buildModel({
      workloads: [
        workload({ id: 'vm-430a', proxmox: { vmid: 430, node: 'alpha' } }),
        workload({ id: 'vm-430b', proxmox: { vmid: 430, node: 'beta' } }),
      ],
      tasks: [
        task({
          id: 't-430',
          type: 'vzdump',
          vmid: 430,
          node: 'gamma',
          instance: '',
          status: 'OK',
        }),
      ],
    });
    expect(model.coverageRows.every((row) => row.latestTask === undefined)).toBe(true);
  });

  it('falls back to the only candidate when its hints do not match', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-440', proxmox: { vmid: 440, node: 'alpha' } })],
      tasks: [
        task({
          id: 't-440',
          type: 'vzdump',
          vmid: 440,
          node: 'gamma',
          instance: '',
          status: 'OK',
        }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.label).toBe('OK');
  });

  it('falls back to the only candidate when hints are all blank', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-450', proxmox: { vmid: 450, node: 'alpha' } })],
      tasks: [
        task({
          id: 't-450',
          type: 'vzdump',
          vmid: 450,
          node: '',
          instance: '',
          status: 'OK',
        }),
      ],
    });
    expect(model.coverageRows[0].latestTask?.label).toBe('OK');
  });

  it('yields no row when no candidate exists for the task vmid', () => {
    const model = buildModel({
      tasks: [
        task({
          id: 't-460',
          type: 'vzdump',
          vmid: 460,
          node: 'alpha',
          instance: '',
          status: 'OK',
        }),
      ],
    });
    expect(model.coverageRows).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// resourceBackupType — oci-container and unknown branches (private; via
// candidate type). Sibling tests cover 'vm' and 'system-container'.
// ---------------------------------------------------------------------------

describe('resourceBackupType branches (via candidate type)', () => {
  it('maps an oci-container resource to workload type "ct"', () => {
    const model = buildModel({
      workloads: [
        workload({ id: 'oci-560', type: 'oci-container', proxmox: { vmid: 560, node: 'n' } }),
      ],
    });
    expect(model.coverageRows[0].workload.type).toBe('ct');
    expect(model.coverageRows[0].workload.typeLabel).toBe('LXC');
  });

  it('drops a non-workload resource type as an unknown candidate', () => {
    const model = buildModel({
      workloads: [
        workload({ id: 'storage-570', type: 'storage', proxmox: { vmid: 570, node: 'n' } }),
      ],
    });
    expect(model.coverageRows).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// resourceNode — nodeName / node / parentName / none arms (private; via
// candidate node).
// ---------------------------------------------------------------------------

describe('resourceNode branches (via candidate node)', () => {
  it('prefers proxmox.nodeName when present', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-721', proxmox: { vmid: 721, nodeName: 'named-a' } })],
    });
    expect(model.coverageRows[0].workload.node).toBe('named-a');
  });

  it('falls back to proxmox.node when nodeName is absent', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-722', proxmox: { vmid: 722, node: 'nodeonly' } })],
    });
    expect(model.coverageRows[0].workload.node).toBe('nodeonly');
  });

  it('falls back to parentName when no proxmox node field is set', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-723', parentName: 'parent-x', proxmox: { vmid: 723 } })],
    });
    expect(model.coverageRows[0].workload.node).toBe('parent-x');
  });

  it('returns undefined when no node source is available', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-724', proxmox: { vmid: 724 } })],
    });
    expect(model.coverageRows[0].workload.node).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// resourceInstance — meta / platformData / none arms (private; via candidate
// instance).
// ---------------------------------------------------------------------------

describe('resourceInstance branches (via candidate instance)', () => {
  it('reads the instance from proxmox meta when present', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-700',
          proxmox: { vmid: 700, node: 'n', instance: 'meta-inst' },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.instance).toBe('meta-inst');
  });

  it('reads the instance from platformData.proxmox when meta lacks it', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-701',
          proxmox: { vmid: 701, node: 'n' },
          platformData: { proxmox: { instance: 'plat-inst' } },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.instance).toBe('plat-inst');
  });

  it('returns undefined when neither meta nor platformData carries an instance', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-710', proxmox: { vmid: 710, node: 'n' } })],
    });
    expect(model.coverageRows[0].workload.instance).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// buildCandidateFromResource — null-returning guards (private; via row
// presence).
// ---------------------------------------------------------------------------

describe('buildCandidateFromResource null guards', () => {
  it('produces no row when the resource has no vmid', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-noid', proxmox: { node: 'n' } })],
    });
    expect(model.coverageRows).toHaveLength(0);
  });

  it('produces no row when the resource vmid is zero', () => {
    const model = buildModel({
      workloads: [workload({ id: 'vm-zero', proxmox: { vmid: 0, node: 'n' } })],
    });
    expect(model.coverageRows).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// taskLabel — success / completed / error / running / other / empty arms
// (private; via latestTask.label). Sibling tests cover 'ok' and 'failed'.
// ---------------------------------------------------------------------------

describe('taskLabel branches (via latestTask.label)', () => {
  const labelForStatus = (status: string): string | undefined => {
    const model = buildModel({
      workloads: [
        workload({ id: 'vm-600', proxmox: { vmid: 600, node: 'node-a', instance: 'inst-a' } }),
      ],
      pbsBackups: [pbsBackup({ id: 'pbs-600', vmid: '600', backupTime: isoDaysAgo(1) })],
      tasks: [
        task({
          id: 't-600',
          type: 'vm',
          vmid: 600,
          status,
          startTime: isoDaysAgo(5),
        }),
      ],
    });
    return model.coverageRows[0]?.latestTask?.label;
  };

  it.each<[string, string]>([
    ['success', 'OK'],
    ['completed', 'OK'],
    ['error', 'Failed'],
    ['running', 'Running'],
    ['WARNING', 'WARNING'],
    ['', 'Unknown'],
  ])('maps task status %j to label %j', (status, expected) => {
    expect(labelForStatus(status)).toBe(expected);
  });
});

// ---------------------------------------------------------------------------
// buildWorkloadLabel — no-name / name===vmid / name===fallback / differs arms
// (private; via candidate label).
// ---------------------------------------------------------------------------

describe('buildWorkloadLabel branches (via candidate label)', () => {
  it('falls back when the resource has no display name', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-510',
          name: '',
          displayName: '',
          proxmox: { vmid: 510, node: 'n' },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.label).toBe('VM 510');
    expect(model.coverageRows[0].workload.name).toBeUndefined();
  });

  it('falls back when the resource name equals its vmid', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-520',
          name: '520',
          displayName: '520',
          proxmox: { vmid: 520, node: 'n' },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.label).toBe('VM 520');
  });

  it('falls back when the resource name equals the generated fallback', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-540',
          name: 'VM 540',
          displayName: 'VM 540',
          proxmox: { vmid: 540, node: 'n' },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.label).toBe('VM 540');
  });

  it('decorates a distinct name with the fallback in parentheses', () => {
    const model = buildModel({
      workloads: [
        workload({
          id: 'vm-550',
          name: 'web-prod',
          displayName: 'web-prod',
          proxmox: { vmid: 550, node: 'n' },
        }),
      ],
    });
    expect(model.coverageRows[0].workload.label).toBe('web-prod (VM 550)');
  });
});

// ---------------------------------------------------------------------------
// buildProxmoxBackupRecoveryModel — top-level summary, sort, and empty edges.
// ---------------------------------------------------------------------------

describe('buildProxmoxBackupRecoveryModel summary, sort, and empty edges', () => {
  it('returns a fully empty model for empty inputs', () => {
    const model = buildModel({});
    expect(model.coverageRows).toHaveLength(0);
    expect(model.recoverableArtifacts).toHaveLength(0);
    expect(model.coverageSummary).toEqual({
      totalWorkloads: 0,
      protected: 0,
      attention: 0,
      unprotected: 0,
      unknown: 0,
      withPBS: 0,
      recoverableArtifacts: 0,
      totalBytes: 0,
    });
  });

  it('aggregates bytes, PBS-row, current, and attention counts', () => {
    const model = buildModel({
      workloads: [
        workload({ id: 'vm-840', proxmox: { vmid: 840, node: 'a' } }),
        workload({ id: 'vm-830', proxmox: { vmid: 830, node: 'b' } }),
      ],
      pbsBackups: [
        pbsBackup({ id: 'pbs-840', vmid: '840', backupTime: isoDaysAgo(1), size: 2_000 }),
        pbsBackup({ id: 'pbs-830', vmid: '830', backupTime: isoDaysAgo(1), size: 5_000 }),
      ],
    });
    expect(model.coverageSummary.totalBytes).toBe(7_000);
    expect(model.coverageSummary.recoverableArtifacts).toBe(2);
    expect(model.coverageSummary.withPBS).toBe(2);
    expect(model.coverageSummary.protected).toBe(2);
    expect(model.coverageSummary.attention).toBe(0);
  });

  it('sorts rows by postureRank asc, then latestRecovery createdMs desc', () => {
    const postures = new Map([
      ['vm-830', posture('vm-830', 'unprotected')],
      ['vm-840', posture('vm-840', 'protected')],
      ['vm-850', posture('vm-850', 'protected')],
    ]);
    const canonicalModel = buildModel({
      workloads: [
        workload({ id: 'vm-840', proxmox: { vmid: 840, node: 'a' } }),
        workload({ id: 'vm-830', proxmox: { vmid: 830, node: 'b' } }),
        workload({ id: 'vm-850', proxmox: { vmid: 850, node: 'c' } }),
      ],
      pbsBackups: [
        pbsBackup({ id: 'pbs-840', vmid: '840', backupTime: isoDaysAgo(1) }),
        pbsBackup({ id: 'pbs-850', vmid: '850', backupTime: isoDaysAgo(2) }),
      ],
      protectionPostures: postures,
    });
    expect(canonicalModel.coverageRows.map((row) => row.workload.vmid)).toEqual([
      '830',
      '840',
      '850',
    ]);
    expect(canonicalModel.coverageSummary.unprotected).toBe(1);
  });

  it('sorts recoverable artifacts by createdMs desc', () => {
    const model = buildModel({
      workloads: [workload({})],
      pbsBackups: [
        pbsBackup({ id: 'pbs-old', vmid: '100', backupTime: isoDaysAgo(10) }),
        pbsBackup({ id: 'pbs-new', vmid: '100', backupTime: isoDaysAgo(1) }),
      ],
    });
    expect(model.recoverableArtifacts.map((artifact) => artifact.nativeId)).toEqual([
      'pbs-new',
      'pbs-old',
    ]);
  });
});
