import { describe, expect, it } from 'vitest';

import {
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
    ).toBe('519 queue total · 16 backlog');
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
});
