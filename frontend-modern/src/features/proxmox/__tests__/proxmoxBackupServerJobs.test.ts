import { describe, expect, it } from 'vitest';

import type { PBSJobHealthEvidence } from '@/types/api';

import { describeDatastoreJobs, summarizeDatastoreJobs } from '../proxmoxBackupServerJobs';

const job = (overrides: Partial<PBSJobHealthEvidence>): PBSJobHealthEvidence => ({
  id: 'job',
  family: 'verify',
  store: 'main',
  enabled: true,
  confidence: 'config-last-run',
  posture: 'healthy',
  postureReason: 'last-run-success',
  ...overrides,
});

describe('summarizeDatastoreJobs', () => {
  it('says OK only when every reported job on the store last succeeded', () => {
    const summary = summarizeDatastoreJobs(
      [
        job({ id: 'garbage:main', family: 'garbage' }),
        job({ id: 'v-1', family: 'verify' }),
        job({ id: 's-1', family: 'sync', postureReason: 'running' }),
      ],
      'main',
    );
    expect(summary.state).toBe('ok');
    expect(summary.label).toBe('OK');
    expect(describeDatastoreJobs(summary)).toBe('GC OK · Verify OK · Sync running');
  });

  it('leads with the worst backend judgment and lists the rest in the detail', () => {
    const summary = summarizeDatastoreJobs(
      [
        job({ id: 'p-1', family: 'prune', posture: 'warning', postureReason: 'job-overdue' }),
        job({ id: 's-1', family: 'sync', posture: 'critical', postureReason: 'last-run-failed' }),
        job({ id: 'v-1', family: 'verify' }),
      ],
      'main',
    );
    expect(summary.state).toBe('critical');
    expect(summary.label).toBe('Sync failed');
    expect(describeDatastoreJobs(summary)).toBe('Sync failed · Prune late · Verify OK');
  });

  it('ignores other stores, disabled jobs and backup jobs', () => {
    const summary = summarizeDatastoreJobs(
      [
        job({ id: 's-other', family: 'sync', store: 'offsite', posture: 'critical' }),
        job({ id: 'v-off', family: 'verify', enabled: false, posture: 'critical' }),
        job({ id: 'b-1', family: 'backup', posture: 'critical' }),
      ],
      'main',
    );
    expect(summary).toEqual({ state: 'none', label: '—', entries: [] });
  });

  it('lets the newest run of a repeated job speak for it', () => {
    const summary = summarizeDatastoreJobs(
      [
        job({
          id: 'verify:v-1',
          posture: 'critical',
          postureReason: 'last-run-failed',
          'last-run-endtime': 1_000,
        }),
        job({ id: 'verify:v-1', 'last-run-endtime': 2_000 }),
      ],
      'main',
    );
    expect(summary.label).toBe('OK');
    expect(summary.entries).toHaveLength(1);
  });

  it('never reports a family as OK when its server-wide read failed', () => {
    const summary = summarizeDatastoreJobs(
      [
        job({ id: 'v-1', family: 'verify' }),
        job({
          id: 'garbage:partial',
          family: 'garbage',
          store: undefined,
          posture: 'unknown',
          error: 'permission denied',
        }),
      ],
      'main',
    );
    expect(summary.state).toBe('unknown');
    expect(summary.label).toBe('GC unknown');
  });

  it('names a job that has not run yet without calling it healthy', () => {
    const summary = summarizeDatastoreJobs(
      [job({ id: 'v-1', posture: 'unknown', postureReason: 'no-run-observed' })],
      'main',
    );
    expect(summary.state).toBe('unknown');
    expect(summary.label).toBe('Verify not run');
  });

  it('has nothing to say without a datastore or evidence', () => {
    expect(summarizeDatastoreJobs(undefined, 'main').label).toBe('—');
    expect(summarizeDatastoreJobs([job({})], undefined).label).toBe('—');
  });
});
