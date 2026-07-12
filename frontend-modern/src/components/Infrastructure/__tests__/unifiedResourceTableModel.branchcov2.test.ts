import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { IODistributionStats } from '@/components/Infrastructure/infrastructureSelectors';
import {
  getOutlierEmphasis,
  getPBSTableRow,
  getPMGTableRow,
} from '@/components/Infrastructure/unifiedResourceTableModel';

/**
 * Branch-coverage suite for the still-uncovered exported functions in
 * unifiedResourceTableModel.ts: getPBSTableRow, getPMGTableRow, getOutlierEmphasis.
 * Each test drives a specific branch (guard, ternary arm, optional-chain outcome)
 * and asserts on the concrete returned shape — never truthiness alone.
 */

const makeResource = (overrides: Partial<Resource> & { id: string }): Resource =>
  ({
    type: 'agent',
    name: `name-${overrides.id}`,
    displayName: `Display ${overrides.id}`,
    platformId: `platform-${overrides.id}`,
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    platformData: {},
    ...overrides,
  }) as unknown as Resource;

const makeStats = (overrides: Partial<IODistributionStats> = {}): IODistributionStats => ({
  median: 0,
  mad: 0,
  max: 100,
  p97: 90,
  p99: 95,
  count: 10,
  ...overrides,
});

describe('getPBSTableRow', () => {
  it('returns null when resource.type is not pbs (guard)', () => {
    expect(getPBSTableRow(makeResource({ id: 'a', type: 'pmg' }))).toBeNull();
    expect(getPBSTableRow(makeResource({ id: 'b', type: 'agent' }))).toBeNull();
  });

  it('nulls every derived field when platformData has no pbs block (optional-chain miss)', () => {
    const row = getPBSTableRow(
      makeResource({ id: 'pbs-empty', type: 'pbs', status: 'online', platformData: {} }),
    );
    expect(row).toEqual({
      datastores: null,
      jobs: null,
      activity: null,
      activityDetail: null,
      activeTaskCount: 0,
      health: null,
      tone: 'ok',
    });
  });

  it('surfaces a positive datastoreCount and jobs total, trims health, and derives tone from health', () => {
    const row = getPBSTableRow(
      makeResource({
        id: 'pbs-full',
        type: 'pbs',
        status: 'degraded',
        platformData: {
          pbs: { datastoreCount: 3, backupJobCount: 2, connectionHealth: '  Healthy  ' },
        },
      }),
    );
    expect(row).toEqual({
      datastores: 3,
      jobs: 2,
      activity: '2 jobs',
      activityDetail: null,
      activeTaskCount: 0,
      health: 'Healthy',
      tone: 'ok',
    });
  });

  it('nulls datastores/jobs for zero counts and maps whitespace health + offline status to a warning tone', () => {
    const row = getPBSTableRow(
      makeResource({
        id: 'pbs-zero',
        type: 'pbs',
        status: 'offline',
        platformData: { pbs: { datastoreCount: 0, connectionHealth: '   ' } },
      }),
    );
    expect(row).toEqual({
      datastores: null,
      jobs: null,
      activity: null,
      activityDetail: null,
      activeTaskCount: 0,
      health: null,
      tone: 'warning',
    });
  });
});

describe('getPMGTableRow', () => {
  it('returns null when resource.type is not pmg (guard)', () => {
    expect(getPMGTableRow(makeResource({ id: 'a', type: 'pbs' }))).toBeNull();
    expect(getPMGTableRow(makeResource({ id: 'b', type: 'agent' }))).toBeNull();
  });

  it('nulls every derived field when platformData has no pmg block (optional-chain miss)', () => {
    const row = getPMGTableRow(
      makeResource({ id: 'pmg-empty', type: 'pmg', status: 'online', platformData: {} }),
    );
    expect(row).toEqual({
      queue: null,
      deferred: null,
      hold: null,
      nodes: null,
      health: null,
      tone: 'ok',
    });
  });

  it('reports positive queue/deferred/hold/nodes and forces a warning tone when backlog > 0', () => {
    const row = getPMGTableRow(
      makeResource({
        id: 'pmg-full',
        type: 'pmg',
        status: 'online',
        platformData: {
          pmg: {
            queueTotal: 12,
            queueDeferred: 3,
            queueHold: 2,
            nodeCount: 4,
            connectionHealth: 'degraded',
          },
        },
      }),
    );
    expect(row).toEqual({
      queue: 12,
      deferred: 3,
      hold: 2,
      nodes: 4,
      health: 'degraded',
      tone: 'warning',
    });
  });

  it('nulls zero counts, trims whitespace health to null, and derives tone from status when backlog is 0', () => {
    const row = getPMGTableRow(
      makeResource({
        id: 'pmg-zero',
        type: 'pmg',
        status: 'warning',
        platformData: {
          pmg: {
            queueTotal: 5,
            queueDeferred: 0,
            queueHold: 0,
            nodeCount: 0,
            connectionHealth: '   ',
          },
        },
      }),
    );
    expect(row).toEqual({
      queue: 5,
      deferred: null,
      hold: null,
      nodes: null,
      health: null,
      tone: 'warning',
    });
  });

  it('nulls queue when queueTotal is explicitly 0 with a present pmg block', () => {
    const row = getPMGTableRow(
      makeResource({
        id: 'pmg-no-queue',
        type: 'pmg',
        status: 'online',
        platformData: { pmg: { queueTotal: 0, nodeCount: 1 } },
      }),
    );
    expect(row).toEqual({
      queue: null,
      deferred: null,
      hold: null,
      nodes: 1,
      health: null,
      tone: 'ok',
    });
  });
});

describe('getOutlierEmphasis', () => {
  describe('guard branch (!isFinite(value) || value <= 0 || stats.max <= 0)', () => {
    it('mutes a non-finite value (NaN)', () => {
      expect(getOutlierEmphasis(Number.NaN, makeStats({ max: 100 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('mutes a non-finite value (Infinity)', () => {
      expect(getOutlierEmphasis(Number.POSITIVE_INFINITY, makeStats({ max: 100 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('mutes a value of exactly 0', () => {
      expect(getOutlierEmphasis(0, makeStats({ max: 100 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('mutes a negative value', () => {
      expect(getOutlierEmphasis(-5, makeStats({ max: 100 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('mutes when stats.max is 0', () => {
      expect(getOutlierEmphasis(50, makeStats({ max: 0 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('mutes when stats.max is negative', () => {
      expect(getOutlierEmphasis(50, makeStats({ max: -1 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });
  });

  describe('small-sample branch (stats.count < 4)', () => {
    it('emphasizes when value/max ratio >= 0.995 (exact boundary)', () => {
      expect(getOutlierEmphasis(99.5, makeStats({ max: 100, count: 3 }))).toEqual({
        className: 'text-base-content font-medium',
        showOutlierHint: true,
      });
    });

    it('emphasizes a ratio of 1.0', () => {
      expect(getOutlierEmphasis(100, makeStats({ max: 100, count: 3 }))).toEqual({
        className: 'text-base-content font-medium',
        showOutlierHint: true,
      });
    });

    it('mutes when value/max ratio < 0.995', () => {
      expect(getOutlierEmphasis(80, makeStats({ max: 100, count: 3 }))).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });
  });

  describe('mad-positive branch (stats.count >= 4, stats.mad > 0)', () => {
    it('returns semibold for a severe outlier (modifiedZ >= 6.5 and value >= p99)', () => {
      // modifiedZ = 0.6745 * (200 - 50) / 2 = 50.5875 (>= 6.5); value 200 >= p99 180
      const stats = makeStats({ median: 50, mad: 2, max: 200, p97: 150, p99: 180, count: 10 });
      expect(getOutlierEmphasis(200, stats)).toEqual({
        className: 'text-base-content font-semibold',
        showOutlierHint: true,
      });
    });

    it('returns medium when modifiedZ is in [5.5, 6.5) and value >= p97 but < p99', () => {
      // modifiedZ = 0.6745 * (90 - 0) / 10 = 6.0705 (in [5.5, 6.5)); value 90 >= p97 80, < p99 200
      const stats = makeStats({ median: 0, mad: 10, max: 300, p97: 80, p99: 200, count: 10 });
      expect(getOutlierEmphasis(90, stats)).toEqual({
        className: 'text-base-content font-medium',
        showOutlierHint: true,
      });
    });

    it('mutes a non-outlier whose modifiedZ is below 5.5', () => {
      // modifiedZ = 0.6745 * (55 - 50) / 10 = 0.337 (< 5.5)
      const stats = makeStats({ median: 50, mad: 10, max: 200, p97: 80, p99: 180, count: 10 });
      expect(getOutlierEmphasis(55, stats)).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('falls through to medium when modifiedZ >= 6.5 but value < p99 (skips severe, hits moderate)', () => {
      // modifiedZ = 0.6745 * (70 - 50) / 2 = 6.745 (>= 6.5); value 70 >= p99 80 is FALSE;
      // value 70 >= p97 60 is TRUE -> moderate path.
      const stats = makeStats({ median: 50, mad: 2, max: 200, p97: 60, p99: 80, count: 10 });
      expect(getOutlierEmphasis(70, stats)).toEqual({
        className: 'text-base-content font-medium',
        showOutlierHint: true,
      });
    });
  });

  describe('mad-zero fallback branch (stats.count >= 4, stats.mad <= 0)', () => {
    it('returns semibold when value >= p99', () => {
      const stats = makeStats({ mad: 0, max: 300, p97: 120, p99: 150, count: 10 });
      expect(getOutlierEmphasis(200, stats)).toEqual({
        className: 'text-base-content font-semibold',
        showOutlierHint: true,
      });
    });

    it('returns medium when value >= p97 but < p99', () => {
      const stats = makeStats({ mad: 0, max: 300, p97: 120, p99: 150, count: 10 });
      expect(getOutlierEmphasis(130, stats)).toEqual({
        className: 'text-base-content font-medium',
        showOutlierHint: true,
      });
    });

    it('mutes a positive value below p97', () => {
      const stats = makeStats({ mad: 0, max: 300, p97: 120, p99: 150, count: 10 });
      expect(getOutlierEmphasis(50, stats)).toEqual({
        className: 'text-muted',
        showOutlierHint: false,
      });
    });

    it('honors an explicitly negative mad identically to mad === 0', () => {
      // Defensive: mad <= 0 takes the fallback path; -1 must behave like 0.
      const stats = makeStats({ mad: -1, max: 300, p97: 120, p99: 150, count: 10 });
      expect(getOutlierEmphasis(200, stats)).toEqual({
        className: 'text-base-content font-semibold',
        showOutlierHint: true,
      });
    });
  });
});
