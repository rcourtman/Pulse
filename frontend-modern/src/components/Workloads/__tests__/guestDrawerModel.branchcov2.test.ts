import { describe, expect, it } from 'vitest';

import type { AggregatedMetricPoint } from '@/api/charts';
import type { WorkloadGuest } from '@/types/workloads';

import {
  buildGuestDrawerHistoryPath,
  getGuestDrawerAgentLabel,
  getGuestDrawerAgentTitle,
  getGuestDrawerBackupPresentation,
  getGuestDrawerHistoryFallbackMetrics,
  getGuestDrawerHistoryRangeBounds,
  getGuestDrawerHistoryScale,
  getGuestDrawerHistoryTarget,
  getGuestDrawerHistoryValueLabel,
  getGuestDrawerMemoryRows,
  getGuestDrawerNetworkInterfaces,
  hasGuestDrawerFilesystemDetails,
  hasGuestDrawerOsInfo,
  isGuestDrawerVM,
  normalizeGuestDrawerHistoryPoints,
  normalizeGuestDrawerTags,
} from '../guestDrawerModel';

const makeGuest = (overrides?: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-0',
    vmid: 100,
    name: 'workload-0',
    node: 'pve',
    instance: 'cluster-a',
    status: 'running',
    type: 'qemu',
    cpu: 0.5,
    cpus: 2,
    memory: { total: 4096, used: 1024, free: 3072, usage: 0.25 },
    disk: { total: 102400, used: 10240, free: 92160, usage: 0.1 },
    networkIn: 100,
    networkOut: 200,
    diskRead: 10,
    diskWrite: 5,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    workloadType: 'vm',
    ...overrides,
  }) as WorkloadGuest;

// AggregatedMetricPoint declares min/max as required numbers, but the runtime
// contract (and the code under test) tolerates missing/non-finite min/max, so
// build points with optional min/max and cast to satisfy the declared type.
const pt = (timestamp: number, value: number, min?: number, max?: number): AggregatedMetricPoint =>
  ({ timestamp, value, min, max }) as AggregatedMetricPoint;

describe('guestDrawerModel (branch coverage)', () => {
  describe('isGuestDrawerVM', () => {
    it('returns true for a qemu/vm guest', () => {
      expect(isGuestDrawerVM(makeGuest({ type: 'qemu', workloadType: 'vm' }))).toBe(true);
    });

    it('returns false for a system-container (lxc) guest', () => {
      expect(isGuestDrawerVM(makeGuest({ type: 'lxc', workloadType: 'system-container' }))).toBe(
        false,
      );
    });

    it('returns false for an app-container', () => {
      expect(
        isGuestDrawerVM(
          makeGuest({ type: 'app-container', workloadType: 'app-container', id: 'c1' }),
        ),
      ).toBe(false);
    });
  });

  describe('getGuestDrawerHistoryFallbackMetrics', () => {
    it('scales the canonical workload cpu ratio by 100 and returns finite metric values', () => {
      const guest = makeGuest({
        cpu: 1.0,
        memory: { total: 100, used: 40, free: 60, usage: 0.4 },
        disk: { total: 100, used: 30, free: 70, usage: 0.3 },
        networkIn: 100,
        networkOut: 200,
        diskRead: 10,
        diskWrite: 5,
      });
      expect(getGuestDrawerHistoryFallbackMetrics(guest)).toStrictEqual({
        cpu: 100,
        memory: 0.4,
        disk: 0.3,
        netin: 100,
        netout: 200,
        diskread: 10,
        diskwrite: 5,
      });
    });

    it('uses the same ratio contract above 100 percent', () => {
      expect(getGuestDrawerHistoryFallbackMetrics(makeGuest({ cpu: 2.0 })).cpu).toBe(200);
    });

    it('treats the cpu boundary of exactly 1.5 as a ratio (150)', () => {
      expect(getGuestDrawerHistoryFallbackMetrics(makeGuest({ cpu: 1.5 })).cpu).toBe(150);
    });

    it('drops cpu when it is a non-finite number', () => {
      expect(
        getGuestDrawerHistoryFallbackMetrics(makeGuest({ cpu: Number.NaN })).cpu,
      ).toBeUndefined();
    });

    it('drops cpu when it is not a number (typeof guard)', () => {
      expect(
        getGuestDrawerHistoryFallbackMetrics(makeGuest({ cpu: 'busy' as unknown as number })).cpu,
      ).toBeUndefined();
    });

    it('drops memory/disk when those objects are absent (optional-chain arm)', () => {
      const result = getGuestDrawerHistoryFallbackMetrics(
        makeGuest({
          memory: undefined as unknown as WorkloadGuest['memory'],
          disk: undefined as unknown as WorkloadGuest['disk'],
        }),
      );
      expect(result.memory).toBeUndefined();
      expect(result.disk).toBeUndefined();
    });

    it('does not synthesize a history point from unavailable live memory', () => {
      const result = getGuestDrawerHistoryFallbackMetrics(
        makeGuest({
          memory: {
            total: 8192,
            used: 0,
            free: 0,
            usage: 0,
            usageUnavailable: true,
          },
        }),
      );
      expect(result.memory).toBeUndefined();
    });

    it('drops a non-finite network value via the finite() guard', () => {
      const result = getGuestDrawerHistoryFallbackMetrics(
        makeGuest({ networkIn: Number.POSITIVE_INFINITY }),
      );
      expect(result.netin).toBeUndefined();
      expect(result.netout).toBe(200);
    });
  });

  describe('normalizeGuestDrawerHistoryPoints (and internal clampHistoryPointValue)', () => {
    it('returns an empty array for undefined points', () => {
      expect(normalizeGuestDrawerHistoryPoints(undefined, '%')).toEqual([]);
    });

    it('filters non-finite timestamp/value points, clamps % values, and sorts by timestamp', () => {
      const points = [
        pt(3, 150, 200, -10), // value->100, min->100, max->0 (% clamp)
        pt(1, 50), // min/max undefined -> fall back to clamped value (50)
        pt(2, Number.NaN), // filtered: non-finite value
        pt(Number.NaN, 40), // filtered: non-finite timestamp
        pt(4, 60, Number.NaN, 'x' as unknown as number), // min NaN -> value; max non-number -> value
      ];
      expect(normalizeGuestDrawerHistoryPoints(points, '%')).toEqual([
        { timestamp: 1, value: 50, min: 50, max: 50 },
        { timestamp: 3, value: 100, min: 100, max: 0 },
        { timestamp: 4, value: 60, min: 60, max: 60 },
      ]);
    });

    it('clamps non-% values to >=0 without the 100 cap', () => {
      const points = [pt(1, -50, -20, 250)];
      expect(normalizeGuestDrawerHistoryPoints(points, 'B/s')).toEqual([
        { timestamp: 1, value: 0, min: 0, max: 250 },
      ]);
    });
  });

  describe('getGuestDrawerHistoryScale', () => {
    it('returns a fixed 0-100 scale for percent units', () => {
      expect(getGuestDrawerHistoryScale([{ points: [pt(1, 150, 0, 999)] }], '%')).toStrictEqual({
        minValue: 0,
        maxValue: 100,
      });
    });

    describe("unit 'C'", () => {
      it('falls back to 0-100 when no finite points are present', () => {
        expect(getGuestDrawerHistoryScale([], 'C')).toStrictEqual({ minValue: 0, maxValue: 100 });
        expect(
          getGuestDrawerHistoryScale(
            [{ points: [pt(1, Number.NaN, Number.NaN, Number.NaN)] }],
            'C',
          ),
        ).toStrictEqual({ minValue: 0, maxValue: 100 });
      });

      it('expands a single distinct value (min===max) symmetrically around it', () => {
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 50, 50, 50)] }], 'C')).toStrictEqual({
          minValue: 45,
          maxValue: 55,
        });
      });

      it('clamps the symmetric lower bound to 0 when the value is near zero', () => {
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 3, 3, 3)] }], 'C')).toStrictEqual({
          minValue: 0,
          maxValue: 8,
        });
      });

      it('applies 15%% padding around distinct min/max using finite min/max fields', () => {
        // min=50, max=80 -> padding=max(2, 30*0.15=4.5)=4.5 -> {45.5, 84.5}
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 55, 50, 80)] }], 'C')).toStrictEqual({
          minValue: 45.5,
          maxValue: 84.5,
        });
      });

      it('falls back to point.value for low/high when min/max are absent', () => {
        // value=40 only -> min===max=40 -> {35, 45}
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 40)] }], 'C')).toStrictEqual({
          minValue: 35,
          maxValue: 45,
        });
      });
    });

    describe('default unit (e.g. B/s)', () => {
      it('returns {0,1} when there are no finite points', () => {
        expect(getGuestDrawerHistoryScale([], 'B/s')).toStrictEqual({ minValue: 0, maxValue: 1 });
        expect(
          getGuestDrawerHistoryScale(
            [
              {
                points: [pt(1, 100, undefined, Number.POSITIVE_INFINITY), pt(2, Number.NaN)],
              },
            ],
            'B/s',
          ),
        ).toStrictEqual({ minValue: 0, maxValue: 1 });
      });

      it('scales to 1.15x of the largest finite max (prefers max over value)', () => {
        // max=200 -> maxValue = max(1, 200*1.15) (IEEE-754 -> 229.99999999999997)
        const expectedMax = Math.max(1, 200 * 1.15);
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 100, 0, 200)] }], 'B/s')).toStrictEqual(
          { minValue: 0, maxValue: expectedMax },
        );
      });

      it('falls back to point.value when max is absent', () => {
        // value=150 -> maxValue = max(1, 150*1.15) = 172.5
        expect(getGuestDrawerHistoryScale([{ points: [pt(1, 150)] }], 'B/s')).toStrictEqual({
          minValue: 0,
          maxValue: 172.5,
        });
      });
    });
  });

  describe('buildGuestDrawerHistoryPath', () => {
    it('returns an empty string for fewer than two points', () => {
      expect(buildGuestDrawerHistoryPath([], { minValue: 0, maxValue: 100 }, 0, 100)).toBe('');
      expect(buildGuestDrawerHistoryPath([pt(1, 50)], { minValue: 0, maxValue: 100 }, 0, 100)).toBe(
        '',
      );
    });

    it('builds an M.. L.. path with default geometry across the full value range', () => {
      const path = buildGuestDrawerHistoryPath(
        [pt(0, 0), pt(100, 100)],
        { minValue: 0, maxValue: 100 },
        0,
        100,
      );
      expect(path).toBe('M34.00,74.00 L352.00,8.00');
    });

    it('clamps values above/below the scale to the plot edges', () => {
      const path = buildGuestDrawerHistoryPath(
        [pt(0, 150), pt(100, -20)],
        { minValue: 0, maxValue: 100 },
        0,
        100,
      );
      expect(path).toBe('M34.00,8.00 L352.00,74.00');
    });

    it('honors custom width/height', () => {
      // width=200 -> plotWidth=158; height=100 -> plotHeight=74
      const path = buildGuestDrawerHistoryPath(
        [pt(0, 0), pt(100, 100)],
        { minValue: 0, maxValue: 100 },
        0,
        100,
        200,
        100,
      );
      expect(path).toBe('M34.00,82.00 L192.00,8.00');
    });

    it('uses Math.max(1,...) for degenerate time/value spans', () => {
      // startTime===endTime -> timeSpan=1; minValue===maxValue -> valueSpan=1
      const path = buildGuestDrawerHistoryPath(
        [pt(5, 5), pt(5, 5)],
        { minValue: 5, maxValue: 5 },
        5,
        5,
      );
      expect(path).toBe('M34.00,74.00 L34.00,74.00');
    });
  });

  describe('getGuestDrawerHistoryValueLabel', () => {
    it("returns '-' when there are no points", () => {
      expect(getGuestDrawerHistoryValueLabel([], '%')).toBe('-');
    });

    it('formats the latest point value for each unit arm', () => {
      expect(getGuestDrawerHistoryValueLabel([pt(1, 10), pt(2, 50)], '%')).toBe('50.0%');
      expect(getGuestDrawerHistoryValueLabel([pt(1, 1024)], 'B/s')).toBe('1.00 KB/s');
      expect(getGuestDrawerHistoryValueLabel([pt(1, 1024)], '')).toBe('1.00 KB');
      expect(getGuestDrawerHistoryValueLabel([pt(1, 22.7)], 'C')).toBe('23°C');
      expect(getGuestDrawerHistoryValueLabel([pt(1, 3)], 'ops')).toBe('3 ops');
      expect(getGuestDrawerHistoryValueLabel([pt(1, 3.5)], 'ops')).toBe('3.5 ops');
    });
  });

  describe('getGuestDrawerHistoryRangeBounds', () => {
    it('returns null when every group has no points', () => {
      expect(getGuestDrawerHistoryRangeBounds([{ points: [] }, { points: [] }])).toBeNull();
    });

    it('returns the combined min/max timestamps across groups', () => {
      expect(
        getGuestDrawerHistoryRangeBounds([
          { points: [pt(10, 1), pt(30, 2)] },
          { points: [pt(5, 3), pt(20, 4)] },
        ]),
      ).toStrictEqual({ startTime: 5, endTime: 30 });
    });
  });

  describe('getGuestDrawerHistoryTarget', () => {
    it('returns null when the canonical id trims to empty', () => {
      expect(
        getGuestDrawerHistoryTarget(
          makeGuest({ id: '   ', type: 'app-container', workloadType: 'app-container' }),
        ),
      ).toBeNull();
    });

    it('maps a vm to the canonical node-scoped id', () => {
      expect(
        getGuestDrawerHistoryTarget(
          makeGuest({ type: 'qemu', workloadType: 'vm', instance: 'c', node: 'pve', vmid: 101 }),
        ),
      ).toStrictEqual({ resourceType: 'vm', resourceId: 'c:pve:101' });
    });

    it('maps a system-container to the canonical node-scoped id', () => {
      expect(
        getGuestDrawerHistoryTarget(
          makeGuest({
            type: 'lxc',
            workloadType: 'system-container',
            instance: 'c',
            node: 'pve',
            vmid: 102,
          }),
        ),
      ).toStrictEqual({ resourceType: 'system-container', resourceId: 'c:pve:102' });
    });

    it('maps an app-container to its plain id', () => {
      expect(
        getGuestDrawerHistoryTarget(
          makeGuest({ id: 'app-1', type: 'app-container', workloadType: 'app-container' }),
        ),
      ).toStrictEqual({ resourceType: 'app-container', resourceId: 'app-1' });
    });

    it('maps a pod to its plain id', () => {
      expect(
        getGuestDrawerHistoryTarget(makeGuest({ id: 'pod-1', type: 'pod', workloadType: 'pod' })),
      ).toStrictEqual({ resourceType: 'pod', resourceId: 'pod-1' });
    });
  });

  describe('hasGuestDrawerOsInfo', () => {
    it('returns false when both os fields are absent', () => {
      expect(hasGuestDrawerOsInfo(makeGuest())).toBe(false);
    });

    it('returns true when osName is a non-empty string', () => {
      expect(hasGuestDrawerOsInfo(makeGuest({ osName: 'Ubuntu' }))).toBe(true);
    });

    it('returns true when only osVersion is set', () => {
      expect(hasGuestDrawerOsInfo(makeGuest({ osVersion: '22.04' }))).toBe(true);
    });

    it('returns false for an empty-string osName (length 0)', () => {
      expect(hasGuestDrawerOsInfo(makeGuest({ osName: '' }))).toBe(false);
    });
  });

  describe('getGuestDrawerAgentLabel', () => {
    it("returns '' when agentVersion is missing or blank", () => {
      expect(getGuestDrawerAgentLabel(makeGuest())).toBe('');
      expect(getGuestDrawerAgentLabel(makeGuest({ agentVersion: '   ' }))).toBe('');
    });

    it('prefixes the version with QEMU for a vm', () => {
      expect(getGuestDrawerAgentLabel(makeGuest({ type: 'qemu', agentVersion: '1.0' }))).toBe(
        'QEMU 1.0',
      );
    });

    it('returns the bare version for a non-vm', () => {
      expect(
        getGuestDrawerAgentLabel(
          makeGuest({ type: 'lxc', workloadType: 'system-container', agentVersion: '1.0' }),
        ),
      ).toBe('1.0');
    });
  });

  describe('getGuestDrawerAgentTitle', () => {
    it("returns '' when agentVersion is missing", () => {
      expect(getGuestDrawerAgentTitle(makeGuest())).toBe('');
    });

    it('builds the full QEMU guest-agent title for a vm', () => {
      expect(getGuestDrawerAgentTitle(makeGuest({ type: 'qemu', agentVersion: '1.0' }))).toBe(
        'QEMU guest agent 1.0',
      );
    });

    it('returns the bare version for a non-vm', () => {
      expect(
        getGuestDrawerAgentTitle(
          makeGuest({ type: 'lxc', workloadType: 'system-container', agentVersion: '1.0' }),
        ),
      ).toBe('1.0');
    });
  });

  describe('getGuestDrawerMemoryRows', () => {
    it('returns an empty array when memory is absent', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({ memory: undefined as unknown as WorkloadGuest['memory'] }),
        ),
      ).toEqual([]);
    });

    it('returns an empty array when total is not positive and no balloon/swap', () => {
      expect(
        getGuestDrawerMemoryRows(makeGuest({ memory: { total: 0, used: 0, free: 0, usage: 0 } })),
      ).toEqual([]);
    });

    it('shows unavailable usage and known capacity without showing zero usage', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: {
              total: 8192,
              used: 0,
              free: 0,
              usage: 0,
              usageUnavailable: true,
            },
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: 'Unavailable' },
        { label: 'Total', value: '8.00 KB' },
      ]);
    });

    it('emits only Usage and Total when total>0 and no cache/free/balloon/swap', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: { total: 4096, used: 1024, usage: 0.25 } as unknown as WorkloadGuest['memory'],
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
      ]);
    });

    it('appends Reclaimable cache (>0) and Free (when free is a number)', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: { total: 4096, used: 1024, free: 3072, usage: 0.25, cache: 2048 },
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
        { label: 'Reclaimable cache', value: '2.00 KB' },
        { label: 'Free', value: '3.00 KB' },
      ]);
    });

    it('omits the Free row when free is not a number', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: { total: 4096, used: 1024, usage: 0.25, free: 'x' as unknown as number },
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
      ]);
    });

    it('appends a Balloon row only when balloon>0 and differs from total', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: {
              total: 4096,
              used: 1024,
              usage: 0.25,
              balloon: 2048,
            } as unknown as WorkloadGuest['memory'],
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
        { label: 'Balloon', value: '2.00 KB' },
      ]);
      // balloon === total -> row suppressed
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: {
              total: 4096,
              used: 1024,
              usage: 0.25,
              balloon: 4096,
            } as unknown as WorkloadGuest['memory'],
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
      ]);
    });

    it('appends a Swap row and defaults swapUsed to 0 when absent', () => {
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: {
              total: 4096,
              used: 1024,
              usage: 0.25,
              swapTotal: 1024,
              swapUsed: 512,
            } as unknown as WorkloadGuest['memory'],
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
        { label: 'Swap', value: '512 B / 1.00 KB' },
      ]);
      expect(
        getGuestDrawerMemoryRows(
          makeGuest({
            memory: {
              total: 4096,
              used: 1024,
              usage: 0.25,
              swapTotal: 1024,
            } as unknown as WorkloadGuest['memory'],
          }),
        ),
      ).toEqual([
        { label: 'Usage', value: '25% · 1.00 KB' },
        { label: 'Total', value: '4.00 KB' },
        { label: 'Swap', value: '0 B / 1.00 KB' },
      ]);
    });
  });

  describe('hasGuestDrawerFilesystemDetails', () => {
    it('returns false when disks is absent', () => {
      expect(hasGuestDrawerFilesystemDetails(makeGuest({ disks: undefined }))).toBe(false);
    });

    it('returns false for an empty disks array', () => {
      expect(hasGuestDrawerFilesystemDetails(makeGuest({ disks: [] }))).toBe(false);
    });

    it('returns true when at least one disk is present', () => {
      expect(
        hasGuestDrawerFilesystemDetails(
          makeGuest({ disks: [{ total: 1, used: 0, free: 1, usage: 0 }] }),
        ),
      ).toBe(true);
    });
  });

  describe('getGuestDrawerNetworkInterfaces', () => {
    it('returns an empty array when absent (|| fallback)', () => {
      expect(getGuestDrawerNetworkInterfaces(makeGuest({ networkInterfaces: undefined }))).toEqual(
        [],
      );
    });

    it('returns the same array reference when present', () => {
      const ifaces = [{ name: 'eth0', mac: 'aa:bb' }];
      expect(getGuestDrawerNetworkInterfaces(makeGuest({ networkInterfaces: ifaces }))).toBe(
        ifaces,
      );
    });
  });

  describe('normalizeGuestDrawerTags', () => {
    it('trims and filters an array of tags', () => {
      expect(normalizeGuestDrawerTags(['  a  ', '', 'b'])).toEqual(['a', 'b']);
      expect(normalizeGuestDrawerTags([])).toEqual([]);
    });

    it('splits, trims, and filters a comma-separated string', () => {
      expect(normalizeGuestDrawerTags('  a , , b ')).toEqual(['a', 'b']);
      expect(normalizeGuestDrawerTags('')).toEqual([]);
    });

    it('returns an empty array for null', () => {
      expect(normalizeGuestDrawerTags(null)).toEqual([]);
    });
  });

  describe('getGuestDrawerBackupPresentation', () => {
    const now = new Date(2024, 5, 15, 12, 0, 0); // 2024-06-15T12:00:00 local
    const dayMs = 1000 * 60 * 60 * 24;
    const backupNDaysAgo = (days: number): Date => new Date(now.getTime() - days * dayMs);

    const expectPresentation = (days: number) => {
      const lastBackup = backupNDaysAgo(days);
      const result = getGuestDrawerBackupPresentation(lastBackup, now);
      expect(result.dateLabel).toBe(new Date(lastBackup).toLocaleDateString());
      return result;
    };

    it('labels day 0 as Today and uses the green (fresh) class', () => {
      expect(expectPresentation(0)).toStrictEqual({
        ageClass: 'text-green-600 dark:text-green-400',
        ageLabel: 'Today',
        dateLabel: new Date(backupNDaysAgo(0)).toLocaleDateString(),
      });
    });

    it('labels day 1 as Yesterday', () => {
      const result = expectPresentation(1);
      expect(result.ageLabel).toBe('Yesterday');
      expect(result.ageClass).toBe('text-green-600 dark:text-green-400');
    });

    it('uses the green class up to and including day 7 (isOld = daysSince > 7)', () => {
      expect(expectPresentation(7).ageClass).toBe('text-green-600 dark:text-green-400');
      expect(expectPresentation(7).ageLabel).toBe('7d ago');
    });

    it('switches to the amber class at day 8', () => {
      const result = expectPresentation(8);
      expect(result.ageClass).toBe('text-amber-600 dark:text-amber-400');
      expect(result.ageLabel).toBe('8d ago');
    });

    it('keeps the amber class at day 30 (isCritical = daysSince > 30)', () => {
      expect(expectPresentation(30).ageClass).toBe('text-amber-600 dark:text-amber-400');
      expect(expectPresentation(30).ageLabel).toBe('30d ago');
    });

    it('switches to the red (critical) class at day 31', () => {
      const result = expectPresentation(31);
      expect(result.ageClass).toBe('text-red-600 dark:text-red-400');
      expect(result.ageLabel).toBe('31d ago');
    });

    it('accepts a numeric (ms) lastBackup value', () => {
      const lastBackup = backupNDaysAgo(10).getTime();
      const result = getGuestDrawerBackupPresentation(lastBackup, now);
      expect(result.ageLabel).toBe('10d ago');
      expect(result.ageClass).toBe('text-amber-600 dark:text-amber-400');
      expect(result.dateLabel).toBe(new Date(lastBackup).toLocaleDateString());
    });
  });
});
