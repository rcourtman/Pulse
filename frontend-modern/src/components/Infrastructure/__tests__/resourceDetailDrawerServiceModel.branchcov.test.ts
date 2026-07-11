import { describe, expect, it } from 'vitest';

import {
  buildPbsActiveTasks,
  buildPbsJobHealthEvidenceModel,
  getPbsActivitySummary,
} from '@/components/Infrastructure/resourceDetailDrawerServiceModel';
import type { PBSPlatformData } from '@/components/Infrastructure/resourceDetailMappers';
import type {
  PBSBackupJob,
  PBSGarbageJob,
  PBSJobHealthEvidence,
  PBSPruneJob,
  PBSSyncJob,
  PBSVerifyJob,
} from '@/types/api';

// The target helpers (formatPbsBackupWorkload, formatPbsEvidenceSource,
// formatPbsEvidenceFamily, buildPbsEvidenceContext, buildPbsEvidenceFreshnessLabel,
// buildPbsTaskLabel, normalizePbsTaskStatus, isPbsTaskStatusActive) are module-private
// and never exported. They are exercised here exclusively through the three exported
// entry points that consume them: buildPbsActiveTasks, getPbsActivitySummary, and
// buildPbsJobHealthEvidenceModel. No private symbol is imported.

const ACTIVE = 'running';

const backupJob = (overrides: Partial<PBSBackupJob> = {}): PBSBackupJob => ({
  id: 'bk-1',
  store: 'fast',
  type: 'vm',
  vmid: '100',
  lastBackup: '',
  nextRun: '',
  status: ACTIVE,
  error: '',
  ...overrides,
});

const syncJob = (overrides: Partial<PBSSyncJob> = {}): PBSSyncJob => ({
  id: 'sync-1',
  store: 'fast',
  remote: 'offsite',
  status: ACTIVE,
  lastSync: '',
  nextRun: '',
  error: '',
  ...overrides,
});

const verifyJob = (overrides: Partial<PBSVerifyJob> = {}): PBSVerifyJob => ({
  id: 'verify-1',
  store: 'fast',
  status: ACTIVE,
  lastVerify: '',
  nextRun: '',
  error: '',
  ...overrides,
});

const pruneJob = (overrides: Partial<PBSPruneJob> = {}): PBSPruneJob => ({
  id: 'prune-1',
  store: 'fast',
  status: ACTIVE,
  lastPrune: '',
  nextRun: '',
  error: '',
  ...overrides,
});

const garbageJob = (overrides: Partial<PBSGarbageJob> = {}): PBSGarbageJob => ({
  id: 'gc-1',
  store: 'fast',
  status: ACTIVE,
  lastGarbage: '',
  nextRun: '',
  removedBytes: 0,
  error: '',
  ...overrides,
});

const evidence = (overrides: Partial<PBSJobHealthEvidence> = {}): PBSJobHealthEvidence => ({
  id: 'ev-1',
  family: 'backup',
  store: 'fast',
  confidence: 'direct',
  ...overrides,
});

// Locate a single active task by id; avoids over-asserting on ordering extras.
const taskWithId = (pbs: PBSPlatformData | undefined, id: string) =>
  buildPbsActiveTasks(pbs).find((task) => task.id === id);

const evidenceWithId = (pbs: PBSPlatformData | undefined, id: string) =>
  buildPbsJobHealthEvidenceModel(pbs).entries.find((entry) => entry.id === id);

describe('isPbsTaskStatusActive / normalizePbsTaskStatus (via buildPbsActiveTasks)', () => {
  it.each([
    ['empty string', ''],
    ['whitespace only', '   '],
    ['inactive token "ok"', 'ok'],
    ['inactive token "failed"', 'failed'],
    ['inactive token "stopped"', 'stopped'],
    ['unrecognized token', 'flurbo'],
    ['uppercase inactive "SUCCESS"', 'SUCCESS'],
  ])('excludes a backup job whose status is not active (%s)', (_label, status) => {
    const pbs: PBSPlatformData = { backupJobs: [backupJob({ id: 'bk', status })] };
    expect(buildPbsActiveTasks(pbs).find((task) => task.id === 'bk')).toBeUndefined();
  });

  it.each([
    ['running', 'running', 'Running'],
    ['queued', 'queued', 'Queued'],
    ['pending', 'pending', 'Pending'],
    ['underscore-delimited active token', 'in_progress', 'In Progress'],
    ['hyphen-delimited active token', 'in-progress', 'In Progress'],
    ['uppercase status', 'RUNNING', 'Running'],
    ['whitespace-padded status', '  starting  ', 'Starting'],
  ])('includes a backup job whose normalized status is active (%s)', (_label, status, label) => {
    const pbs: PBSPlatformData = { backupJobs: [backupJob({ id: 'bk', status })] };
    expect(taskWithId(pbs, 'bk')?.statusLabel).toBe(label);
  });

  it('treats an undefined status as inactive (guard branch)', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', status: undefined as unknown as string })],
    };
    expect(buildPbsActiveTasks(pbs).find((task) => task.id === 'bk')).toBeUndefined();
  });

  it('lets an inactive token override an active token in the same status (precedence)', () => {
    // "running ok" contains both an active ("running") and an inactive ("ok") token;
    // the inactive check short-circuits first, so the job is excluded.
    const pbs: PBSPlatformData = { backupJobs: [backupJob({ id: 'bk', status: 'running ok' })] };
    expect(buildPbsActiveTasks(pbs).find((task) => task.id === 'bk')).toBeUndefined();
  });
});

describe('buildPbsTaskLabel (via buildPbsActiveTasks / buildPbsJobHealthEvidenceModel)', () => {
  it('suffixes the task type with the job id when an id is present', () => {
    const pbs: PBSPlatformData = { verifyJobs: [verifyJob({ id: 'verify-9' })] };
    expect(taskWithId(pbs, 'verify-9')?.label).toBe('Verify verify-9');
  });

  it('returns the bare task type when the id trims to empty', () => {
    const pbs: PBSPlatformData = { pruneJobs: [pruneJob({ id: '   ' })] };
    const tasks = buildPbsActiveTasks(pbs);
    expect(tasks).toHaveLength(1);
    // id was whitespace-only, so the entry is keyed on the trimmed-empty id and the
    // label drops the suffix entirely.
    expect(tasks[0]?.label).toBe('Prune');
  });

  it('uses the "Garbage Collection" task type for garbage jobs', () => {
    const pbs: PBSPlatformData = { garbageJobs: [garbageJob({ id: 'gc-1' })] };
    expect(taskWithId(pbs, 'gc-1')?.label).toBe('Garbage Collection gc-1');
  });
});

describe('formatPbsBackupWorkload (via backup task context)', () => {
  it('renders "VM <vmid>" for vm workloads', () => {
    const pbs: PBSPlatformData = { backupJobs: [backupJob({ id: 'bk', type: 'vm', vmid: '100' })] };
    expect(taskWithId(pbs, 'bk')?.context).toBe('fast · VM 100');
  });

  it('renders "Container <vmid>" for ct workloads', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: 'ct', vmid: '200', store: 'archive' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('archive · Container 200');
  });

  it('title-cases an unrecognized workload type via normalizeDelimitedLabel', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: 'storage', vmid: '300' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('fast · Storage 300');
  });

  it('keeps the type label only when vmid is empty', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: 'vm', vmid: '', store: 'archive' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('archive · VM');
  });

  it('falls back to the "Workload" label when only vmid is present', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: '', vmid: '400' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('fast · Workload 400');
  });

  it('drops the workload segment entirely when both type and vmid are empty', () => {
    // formatPbsBackupWorkload returns null, so context reduces to just the store.
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: '', vmid: '', store: 'fast' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('fast');
  });

  it('produces a null context when store, type and vmid are all empty', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: '', vmid: '', store: '' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBeNull();
  });

  it('normalizes a UPPERCASE type to its canonical label', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [backupJob({ id: 'bk', type: 'CT', vmid: '500' })],
    };
    expect(taskWithId(pbs, 'bk')?.context).toBe('fast · Container 500');
  });
});

describe('buildPbsActiveTasks entry-point guards', () => {
  it('returns an empty array for undefined pbs', () => {
    expect(buildPbsActiveTasks(undefined)).toEqual([]);
  });

  it('returns an empty array when no job arrays are present', () => {
    expect(buildPbsActiveTasks({})).toEqual([]);
  });

  it('handles null job arrays defensively (coalesces to empty)', () => {
    const pbs = {
      backupJobs: undefined,
      syncJobs: null,
      verifyJobs: undefined,
      pruneJobs: null,
      garbageJobs: undefined,
    } as unknown as PBSPlatformData;
    expect(buildPbsActiveTasks(pbs)).toEqual([]);
  });

  it('surfaces a trimmed job error and omits it when blank', () => {
    const pbs: PBSPlatformData = {
      backupJobs: [
        backupJob({ id: 'bk-err', error: '  boom  ' }),
        backupJob({ id: 'bk-clean', error: '' }),
        backupJob({ id: 'bk-ws', error: '   ' }),
      ],
    };
    const tasks = buildPbsActiveTasks(pbs);
    expect(taskWithId(pbs, 'bk-err')?.error).toBe('boom');
    expect(taskWithId(pbs, 'bk-clean')?.error).toBeUndefined();
    expect(taskWithId(pbs, 'bk-ws')?.error).toBeUndefined();
    expect(tasks).toHaveLength(3);
  });

  it('includes active verify, prune and garbage tasks with store-only context', () => {
    const pbs: PBSPlatformData = {
      verifyJobs: [verifyJob({ id: 'v', store: 'vstore' })],
      pruneJobs: [pruneJob({ id: 'p', store: 'pstore' })],
      garbageJobs: [garbageJob({ id: 'g', store: 'gstore' })],
    };
    expect(taskWithId(pbs, 'v')).toEqual({
      id: 'v',
      label: 'Verify v',
      context: 'vstore',
      statusLabel: 'Running',
      error: undefined,
    });
    expect(taskWithId(pbs, 'p')?.context).toBe('pstore');
    expect(taskWithId(pbs, 'g')?.context).toBe('gstore');
  });

  it('renders a sync task without a remote as store-only context', () => {
    const pbs: PBSPlatformData = { syncJobs: [syncJob({ id: 's', remote: '' })] };
    expect(taskWithId(pbs, 's')?.context).toBe('fast');
  });
});

describe('getPbsActivitySummary', () => {
  it('returns a null label and zero count for undefined pbs', () => {
    expect(getPbsActivitySummary(undefined)).toEqual({
      label: null,
      detail: null,
      activeTaskCount: 0,
    });
  });

  it('returns a null label when there are no jobs and no active tasks', () => {
    expect(getPbsActivitySummary({})).toEqual({
      label: null,
      detail: null,
      activeTaskCount: 0,
    });
  });

  it('reports "N jobs" with no detail when jobs exist but none are active', () => {
    const pbs: PBSPlatformData = {
      backupJobCount: 2,
      verifyJobCount: 1,
      backupJobs: [backupJob({ id: 'a', status: 'ok' })],
    };
    expect(getPbsActivitySummary(pbs)).toEqual({
      label: '3 jobs',
      detail: null,
      activeTaskCount: 0,
    });
  });

  it('omits the total detail when active count equals total job count', () => {
    const pbs: PBSPlatformData = {
      backupJobCount: 1,
      backupJobs: [backupJob({ id: 'only', status: 'running' })],
    };
    expect(getPbsActivitySummary(pbs)).toEqual({
      label: '1 active',
      detail: null,
      activeTaskCount: 1,
    });
  });

  it('includes the total detail when more jobs exist than are active', () => {
    const pbs: PBSPlatformData = {
      backupJobCount: 2,
      backupJobs: [
        backupJob({ id: 'run', status: 'running' }),
        backupJob({ id: 'idle', status: 'ok' }),
      ],
    };
    expect(getPbsActivitySummary(pbs)).toEqual({
      label: '1 active',
      detail: '2 total',
      activeTaskCount: 1,
    });
  });

  it('counts active tasks across every job family for the summary', () => {
    const pbs: PBSPlatformData = {
      backupJobCount: 1,
      syncJobCount: 1,
      verifyJobCount: 1,
      pruneJobCount: 1,
      garbageJobCount: 1,
      backupJobs: [backupJob({ id: 'b', status: 'running' })],
      syncJobs: [syncJob({ id: 's', status: 'queued' })],
      verifyJobs: [verifyJob({ id: 'v', status: 'starting' })],
      pruneJobs: [pruneJob({ id: 'p', status: 'in progress' })],
      garbageJobs: [garbageJob({ id: 'g', status: 'pending' })],
    };
    expect(getPbsActivitySummary(pbs)).toEqual({
      label: '5 active',
      detail: null,
      activeTaskCount: 5,
    });
  });
});

describe('formatPbsEvidenceFamily (via evidence label)', () => {
  it.each([
    ['backup', 'Backup'],
    ['sync', 'Sync'],
    ['verify', 'Verify'],
    ['prune', 'Prune'],
    ['garbage', 'Garbage collection'],
    ['BACKUP', 'Backup'],
    // NOTE: 'garbage-collection' normalizes to 'garbage collection', which does
    // NOT equal the exact token 'garbage', so it bypasses the canonical branch
    // and falls through to the generic title-case path — yielding 'Collection'
    // with a capital C instead of the canonical lowercase 'c'. See GLM_REPORT.md.
    ['garbage-collection', 'Garbage Collection'],
  ])('maps family %s to %s', (family, expected) => {
    const pbs: PBSPlatformData = { jobHealthEvidence: [evidence({ id: 'fam', family })] };
    expect(evidenceWithId(pbs, 'fam')?.label).toBe(`${expected} fam`);
  });

  it('title-cases an unrecognized family', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'fam', family: 're-index' })],
    };
    expect(evidenceWithId(pbs, 'fam')?.label).toBe('Re Index fam');
  });

  it('falls back to "Job" when the family is empty', () => {
    const pbs: PBSPlatformData = { jobHealthEvidence: [evidence({ id: 'fam', family: '' })] };
    expect(evidenceWithId(pbs, 'fam')?.label).toBe('Job fam');
  });

  it('drops the id suffix when the evidence id trims to empty', () => {
    const pbs: PBSPlatformData = { jobHealthEvidence: [evidence({ id: '   ', family: 'sync' })] };
    expect(buildPbsJobHealthEvidenceModel(pbs).entries[0]?.label).toBe('Sync');
  });
});

describe('formatPbsEvidenceSource (via evidence sourceLabel)', () => {
  it('returns "Observed backup task history" for backup family with task history', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'src', family: 'backup', evidenceSource: 'pbs-task-history' }),
      ],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Observed backup task history');
  });

  it('returns the generic "Observed task history" for non-backup task history', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'src', family: 'verify', evidenceScope: 'task-history' }),
      ],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Observed task history');
  });

  it('returns "Configured PBS job" for the configured-job scope', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'src', evidenceScope: 'configured-job' })],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Configured PBS job');
  });

  it('returns "Configured PBS job" when the source mentions job config', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'src', evidenceSource: 'pbs job config reader' })],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Configured PBS job');
  });

  it('returns "Partial PBS read" for the partial-read scope', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'src', evidenceScope: 'partial-read' })],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Partial PBS read');
  });

  it('title-cases a free-form source as the fallback', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'src', evidenceSource: 'pbs-datastore-scan' })],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Pbs Datastore Scan');
  });

  it('falls back to "PBS evidence" when source and scope are empty', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'src', evidenceSource: '', evidenceScope: '' })],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('PBS evidence');
  });

  it('prefers configured-job over partial-read when both signals appear', () => {
    // configured-job is checked before partial-read in source precedence.
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'src', evidenceScope: 'configured job', evidenceSource: 'partial read' }),
      ],
    };
    expect(evidenceWithId(pbs, 'src')?.sourceLabel).toBe('Configured PBS job');
  });
});

describe('buildPbsEvidenceContext (via evidence context)', () => {
  it('joins store, remote, namespace and worker-id when all are present', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({
          id: 'ctx',
          store: 'fast',
          remote: 'offsite',
          namespace: 'tenant-a',
          'worker-id': 'w1',
        }),
      ],
    };
    expect(evidenceWithId(pbs, 'ctx')?.context).toBe(
      'fast · Remote offsite · Namespace tenant-a · Worker w1',
    );
  });

  it('renders only the remote segment when store is absent', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'ctx', store: '', remote: 'offsite' })],
    };
    expect(evidenceWithId(pbs, 'ctx')?.context).toBe('Remote offsite');
  });

  it('returns null when every context segment is absent', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'ctx', store: '', remote: '', namespace: '', 'worker-id': '' }),
      ],
    };
    expect(evidenceWithId(pbs, 'ctx')?.context).toBeNull();
  });
});

describe('buildPbsEvidenceFreshnessLabel (via evidence freshnessLabel)', () => {
  it('prefers the ISO last-run-end time from freshness', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({
          id: 'fr',
          freshness: { observedAt: '2026-05-01T08:00:00Z', lastRunEndTime: '2026-04-20T21:30:00Z' },
          'last-run-endtime': 1776717000,
        }),
      ],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Last run 2026-04-20T21:30:00Z');
  });

  it('falls back to the unix last-run-endtime when the ISO time is absent', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'fr', 'last-run-endtime': 1776717000 }),
      ],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Last run 2026-04-20T20:30:00Z');
  });

  it('reports "Task ended" from the unix task-endtime when no last run is present', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'fr', 'task-endtime': 1700000000 })],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Task ended 2023-11-14T22:13:20Z');
  });

  it('reports "Next run" from the unix next-run when nothing earlier is present', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [evidence({ id: 'fr', 'next-run': 1800000000 })],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Next run 2027-01-15T08:00:00Z');
  });

  it('reports "Observed" from the freshness observedAt as the final fallback', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({ id: 'fr', freshness: { observedAt: '2026-05-01T08:00:00Z' } }),
      ],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Observed 2026-05-01T08:00:00Z');
  });

  it('returns null when no freshness signal of any kind is present', () => {
    const pbs: PBSPlatformData = { jobHealthEvidence: [evidence({ id: 'fr' })] };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBeNull();
  });

  it('treats a non-finite unix last-run-endtime as absent and falls through', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidence: [
        evidence({
          id: 'fr',
          'last-run-endtime': Number.NaN,
          'task-endtime': 1700000000,
        }),
      ],
    };
    expect(evidenceWithId(pbs, 'fr')?.freshnessLabel).toBe('Task ended 2023-11-14T22:13:20Z');
  });
});

describe('buildPbsJobHealthEvidenceModel aggregation', () => {
  it('returns a zero-evidence model for undefined pbs', () => {
    expect(buildPbsJobHealthEvidenceModel(undefined)).toEqual({
      evidenceCount: 0,
      visibleCount: 0,
      countLabel: null,
      entries: [],
    });
  });

  it('uses the greater of the declared count and the visible entries', () => {
    const pbs: PBSPlatformData = {
      jobHealthEvidenceCount: 5,
      jobHealthEvidence: [evidence({ id: 'a' }), evidence({ id: 'b' })],
    };
    expect(buildPbsJobHealthEvidenceModel(pbs)).toMatchObject({
      evidenceCount: 5,
      visibleCount: 2,
      countLabel: '5 evidence records',
    });
  });
});
