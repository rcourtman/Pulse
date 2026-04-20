import { describe, expect, it } from 'vitest';

import {
  buildPbsActiveTasks,
  getPbsActivitySummary,
  buildPbsVisibleJobBreakdown,
  buildPmgVisibleMailBreakdown,
  buildPmgVisibleQueueBreakdown,
  getPbsJobTotal,
  getPmgQueueBacklog,
  getServiceDetailsSummary,
} from '@/components/Infrastructure/resourceDetailDrawerServiceModel';

describe('resourceDetailDrawerServiceModel', () => {
  it('builds PBS summary and visible job breakdown from canonical counts', () => {
    const pbs = {
      datastoreCount: 2,
      backupJobCount: 3,
      syncJobCount: 0,
      verifyJobCount: 1,
      pruneJobCount: 0,
      garbageJobCount: 0,
    };

    expect(getPbsJobTotal(pbs)).toBe(4);
    expect(buildPbsVisibleJobBreakdown(pbs)).toEqual([
      { label: 'Backup', value: 3 },
      { label: 'Verify', value: 1 },
    ]);
    expect(
      getServiceDetailsSummary({
        resourceType: 'pbs',
        docker: undefined,
        pbs,
        pmg: undefined,
      }),
    ).toBe('2 datastores · 4 jobs');
  });

  it('derives active PBS tasks from the canonical raw job payloads', () => {
    const pbs = {
      datastoreCount: 2,
      backupJobCount: 2,
      syncJobCount: 1,
      verifyJobCount: 1,
      pruneJobCount: 0,
      garbageJobCount: 0,
      backupJobs: [
        {
          id: 'backup-nightly',
          store: 'fast',
          type: 'vm',
          vmid: '100',
          lastBackup: '',
          nextRun: '',
          status: 'running',
          error: '',
        },
        {
          id: 'backup-weekly',
          store: 'archive',
          type: 'ct',
          vmid: '200',
          lastBackup: '',
          nextRun: '',
          status: 'ok',
          error: '',
        },
      ],
      syncJobs: [
        {
          id: 'sync-remote',
          store: 'fast',
          remote: 'offsite',
          status: 'queued',
          lastSync: '',
          nextRun: '',
          error: '',
        },
      ],
      verifyJobs: [
        {
          id: 'verify-1',
          store: 'fast',
          status: 'ok',
          lastVerify: '',
          nextRun: '',
          error: '',
        },
      ],
    };

    expect(buildPbsActiveTasks(pbs)).toEqual([
      {
        id: 'backup-nightly',
        label: 'Backup backup-nightly',
        context: 'fast · VM 100',
        statusLabel: 'Running',
        error: undefined,
      },
      {
        id: 'sync-remote',
        label: 'Sync sync-remote',
        context: 'fast · Remote offsite',
        statusLabel: 'Queued',
        error: undefined,
      },
    ]);
    expect(getPbsActivitySummary(pbs)).toEqual({
      label: '2 active',
      detail: '4 total',
      activeTaskCount: 2,
    });
    expect(
      getServiceDetailsSummary({
        resourceType: 'pbs',
        docker: undefined,
        pbs,
        pmg: undefined,
      }),
    ).toBe('2 datastores · 2 active tasks');
  });

  it('keeps PMG backlog and breakdown visibility canonical', () => {
    const pmg = {
      queueTotal: 519,
      queueActive: 100,
      queueDeferred: 12,
      queueHold: 4,
      queueIncoming: 0,
      mailCountTotal: 1200,
      spamIn: 32,
      virusIn: 2,
    };

    expect(getPmgQueueBacklog(pmg)).toBe(16);
    expect(buildPmgVisibleQueueBreakdown(pmg)).toEqual([
      { label: 'Active', value: 100 },
      { label: 'Deferred', value: 12, warn: true },
      { label: 'Hold', value: 4, warn: true },
    ]);
    expect(buildPmgVisibleMailBreakdown(pmg)).toEqual([
      { label: 'Mail', value: 1200 },
      { label: 'Spam', value: 32 },
      { label: 'Virus', value: 2 },
    ]);
    expect(
      getServiceDetailsSummary({
        resourceType: 'pmg',
        docker: undefined,
        pbs: undefined,
        pmg,
      }),
    ).toBe('519 queued messages · 16 delayed messages');
  });

  it('keeps docker service summary on container and update counts', () => {
    expect(
      getServiceDetailsSummary({
        resourceType: 'docker-host',
        docker: {
          containerCount: 7,
          updatesAvailableCount: 3,
        },
        pbs: undefined,
        pmg: undefined,
      }),
    ).toBe('7 containers · 3 updates');
  });

  it('pluralizes singular service summary counts cleanly', () => {
    expect(
      getServiceDetailsSummary({
        resourceType: 'docker-host',
        docker: {
          containerCount: 1,
          updatesAvailableCount: 1,
        },
        pbs: undefined,
        pmg: undefined,
      }),
    ).toBe('1 container · 1 update');

    expect(
      getServiceDetailsSummary({
        resourceType: 'pbs',
        docker: undefined,
        pbs: {
          datastoreCount: 1,
          backupJobCount: 1,
          syncJobCount: 0,
          verifyJobCount: 0,
          pruneJobCount: 0,
          garbageJobCount: 0,
        },
        pmg: undefined,
      }),
    ).toBe('1 datastore · 1 job');
  });
});
