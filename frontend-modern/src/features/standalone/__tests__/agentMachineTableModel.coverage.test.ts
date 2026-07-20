import { describe, expect, it } from 'vitest';
import type { HostDiskSMART } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineDiskIODetails,
  getAgentMachineHottestSmartDiskType,
  getAgentMachineMemoryPercent,
  getAgentMachineRaidSummary,
  getAgentMachineTemperatureCelsius,
  getAgentMachineTemperatureDetailSections,
  timestampMillisFrom,
  type AgentMachineTemperatureDetailRow,
  type AgentMachineTemperatureDetailSection,
} from '../agentMachineTableModel';

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'machine-1',
    name: overrides.name ?? overrides.id ?? 'machine-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'machine-1',
    type: 'agent',
    platformId: 'agent',
    platformType: 'agent',
    sourceType: 'agent',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

const rowsFor = (
  sections: readonly AgentMachineTemperatureDetailSection[],
  heading: string,
): AgentMachineTemperatureDetailRow[] =>
  sections.find((entry) => entry.heading === heading)?.rows ?? [];

describe('agentMachineTableModel coverage', () => {
  describe('getAgentMachineMemoryPercent', () => {
    it('derives the percentage from total/used and ignores the current fallback', () => {
      expect(
        getAgentMachineMemoryPercent(
          resource({ memory: { current: 99, total: 16_000, used: 4_000 } }),
        ),
      ).toBe(25);
    });

    it('returns zero when used is explicitly zero rather than the current fallback', () => {
      expect(
        getAgentMachineMemoryPercent(resource({ memory: { current: 99, total: 16_000, used: 0 } })),
      ).toBe(0);
    });

    it('falls back to memory.current when total is non-positive or missing', () => {
      expect(
        getAgentMachineMemoryPercent(resource({ memory: { current: 30, total: 0, used: 5 } })),
      ).toBe(30);
      expect(getAgentMachineMemoryPercent(resource({ memory: { current: 45 } }))).toBe(45);
    });

    it('falls back to memory.current when total is positive but used is not a number', () => {
      expect(getAgentMachineMemoryPercent(resource({ memory: { current: 60, total: 1000 } }))).toBe(
        60,
      );
    });

    it('prefers memory.current over agent.memory.usage', () => {
      expect(
        getAgentMachineMemoryPercent(
          resource({ memory: { current: 33 }, agent: { memory: { usage: 88 } } }),
        ),
      ).toBe(33);
    });

    it('falls back to agent.memory.usage only when memory.current is absent', () => {
      expect(getAgentMachineMemoryPercent(resource({ agent: { memory: { usage: 42 } } }))).toBe(42);
    });

    it('returns undefined when no memory source is available', () => {
      expect(getAgentMachineMemoryPercent(resource({}))).toBeUndefined();
    });
  });

  describe('timestampMillisFrom', () => {
    it('reads valid Date instances and rejects invalid ones', () => {
      const valid = new Date('2023-01-01T00:00:00Z');
      expect(timestampMillisFrom(valid)).toBe(valid.getTime());
      expect(timestampMillisFrom(new Date('not-a-date'))).toBeUndefined();
    });

    it('parses valid date strings and rejects unparseable strings', () => {
      expect(timestampMillisFrom('2023-01-01T00:00:00Z')).toBe(Date.parse('2023-01-01T00:00:00Z'));
      expect(timestampMillisFrom('not-a-date')).toBeUndefined();
    });

    it('treats sub-10^10 numbers as seconds and >=10^10 as milliseconds', () => {
      expect(timestampMillisFrom(1_700_000_000)).toBe(1_700_000_000_000);
      expect(timestampMillisFrom(1_700_000_000_000)).toBe(1_700_000_000_000);
      // Boundary: exactly 10_000_000_000 is treated as milliseconds (not multiplied).
      expect(timestampMillisFrom(10_000_000_000)).toBe(10_000_000_000);
      expect(timestampMillisFrom(9_999_999_999)).toBe(9_999_999_999_000);
    });

    it('returns undefined for non-positive, non-finite, missing, or mistyped inputs', () => {
      expect(timestampMillisFrom(0)).toBeUndefined();
      expect(timestampMillisFrom(-5)).toBeUndefined();
      expect(timestampMillisFrom(Number.NaN)).toBeUndefined();
      expect(timestampMillisFrom(Number.POSITIVE_INFINITY)).toBeUndefined();
      expect(timestampMillisFrom(undefined)).toBeUndefined();
      expect(timestampMillisFrom(true as unknown as number)).toBeUndefined();
    });
  });

  describe('getAgentMachineHottestSmartDiskType', () => {
    it('returns the transport type of the hottest active SMART disk', () => {
      const machine = resource({
        agent: {
          sensors: {
            smart: [
              { device: '/dev/sda', type: 'sata', temperature: 40 },
              { device: '/dev/nvme0', type: 'nvme', temperature: 55 },
            ],
          },
        },
      });
      expect(getAgentMachineHottestSmartDiskType(machine)).toBe('nvme');
    });

    it('keeps the earliest disk when multiple share the hottest temperature', () => {
      const machine = resource({
        agent: {
          sensors: {
            smart: [
              { device: '/dev/sda', type: 'ssd', temperature: 50 },
              { device: '/dev/sdb', type: 'hdd', temperature: 50 },
            ],
          },
        },
      });
      expect(getAgentMachineHottestSmartDiskType(machine)).toBe('ssd');
    });

    it('returns undefined when the hottest active disk has no transport type', () => {
      const machine = resource({
        agent: { sensors: { smart: [{ device: '/dev/sda', temperature: 50 }] } },
      });
      expect(getAgentMachineHottestSmartDiskType(machine)).toBeUndefined();
    });

    it('ignores standby disks', () => {
      const machine = resource({
        agent: {
          sensors: {
            smart: [{ device: '/dev/sda', type: 'sata', temperature: 55, standby: true }],
          },
        },
      });
      expect(getAgentMachineHottestSmartDiskType(machine)).toBeUndefined();
    });

    it('ignores active disks whose temperature is zero', () => {
      const machine = resource({
        agent: { sensors: { smart: [{ device: '/dev/sda', type: 'sata', temperature: 0 }] } },
      });
      expect(getAgentMachineHottestSmartDiskType(machine)).toBeUndefined();
    });

    it('returns undefined when there are no SMART disks', () => {
      expect(getAgentMachineHottestSmartDiskType(resource({}))).toBeUndefined();
    });
  });

  describe('temperature readings and ordering (private helpers)', () => {
    it('keeps only positive, finite sensor temperatures', () => {
      const sections = getAgentMachineTemperatureDetailSections(
        resource({
          agent: {
            sensors: {
              temperatureCelsius: { cpu: 60, ambient: 0, neg: -3, broken: Number.NaN },
            },
          },
        }),
      );
      expect(rowsFor(sections, 'Temperatures')).toEqual([{ label: 'cpu', value: '60°C' }]);
    });

    it('sorts sensor readings worst-first with a numeric label tiebreak', () => {
      const sections = getAgentMachineTemperatureDetailSections(
        resource({
          agent: { sensors: { temperatureCelsius: { hot: 99, t10: 50, t2: 50 } } },
        }),
      );
      expect(rowsFor(sections, 'Temperatures')).toEqual([
        { label: 'hot', value: '99°C' },
        { label: 't2', value: '50°C' },
        { label: 't10', value: '50°C' },
      ]);
    });

    it('sorts fan readings by label with numeric awareness', () => {
      const sections = getAgentMachineTemperatureDetailSections(
        resource({ agent: { sensors: { fanRpm: { fan10: 1000, fan2: 1000, fan1: 1000 } } } }),
      );
      expect(rowsFor(sections, 'Fan Speeds')).toEqual([
        { label: 'fan1', value: '1000 RPM' },
        { label: 'fan2', value: '1000 RPM' },
        { label: 'fan10', value: '1000 RPM' },
      ]);
    });

    it('caps sections at six rows and appends a trailing overflow marker', () => {
      const sections = getAgentMachineTemperatureDetailSections(
        resource({
          agent: {
            sensors: {
              temperatureCelsius: { s1: 10, s2: 20, s3: 30, s4: 40, s5: 50, s6: 60, s7: 70 },
            },
          },
        }),
      );
      const rows = rowsFor(sections, 'Temperatures');
      expect(rows).toHaveLength(7);
      // Worst-first, so the least alarming (s1=10°C) is dropped behind the marker.
      expect(rows.slice(0, 6).map((row) => row.value)).toEqual([
        '70°C',
        '60°C',
        '50°C',
        '40°C',
        '30°C',
        '20°C',
      ]);
      expect(rows[6]).toEqual({ label: '+1 more', value: '', muted: true });
    });

    it('does not cap sections with six or fewer rows', () => {
      const sections = getAgentMachineTemperatureDetailSections(
        resource({
          agent: {
            sensors: { temperatureCelsius: { s1: 10, s2: 20, s3: 30, s4: 40, s5: 50, s6: 60 } },
          },
        }),
      );
      const rows = rowsFor(sections, 'Temperatures');
      expect(rows).toHaveLength(6);
      expect(rows.at(-1)).toEqual({ label: 's1', value: '10°C' });
    });

    it('builds SMART labels from device/model, drops non-standby disks lacking a temperature, and marks standby disks', () => {
      const smart = [
        { device: '/dev/sda', model: 'Foo', temperature: 40 },
        { device: '/dev/sdb', temperature: 41 },
        { temperature: 42 },
        { device: '/dev/sdc', standby: true, temperature: 0 },
        { device: '/dev/sdd', temperature: 0 },
      ] as unknown as HostDiskSMART[];
      const sections = getAgentMachineTemperatureDetailSections(
        resource({ agent: { sensors: { smart } } }),
      );
      // Active readings sorted worst-first; standby appended and sorted by label;
      // the device-less disk falls back to "disk" and the zero-temp non-standby
      // disk (/dev/sdd) is skipped entirely.
      expect(rowsFor(sections, 'Disk Temperatures')).toEqual([
        { label: 'Disk disk', value: '42°C' },
        { label: 'Disk /dev/sdb', value: '41°C' },
        { label: 'Disk /dev/sda Foo', value: '40°C' },
        { label: 'Disk /dev/sdc', value: 'standby', muted: true },
      ]);
    });

    it('excludes zero-temperature active SMART disks from the fallback temperature', () => {
      expect(
        getAgentMachineTemperatureCelsius(
          resource({ agent: { sensors: { smart: [{ device: '/dev/sda', temperature: 0 }] } } }),
        ),
      ).toBeUndefined();
    });
  });

  describe('getAgentMachineRaidSummary', () => {
    it('returns an empty string when there are no RAID arrays', () => {
      expect(getAgentMachineRaidSummary(resource({}))).toBe('');
    });

    it('reports degraded arrays and prioritizes failure over rebuilding', () => {
      const failed = resource({
        agent: {
          raid: [
            {
              device: '/dev/md0',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 1,
              workingDevices: 1,
              failedDevices: 1,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 0,
            },
          ],
        },
      });
      expect(getAgentMachineRaidSummary(failed)).toBe('1/1 degraded');

      // A single array that is both failed and rebuilding still reports degraded.
      const failedAndRebuilding = resource({
        agent: {
          raid: [
            {
              device: '/dev/md0',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 1,
              workingDevices: 1,
              failedDevices: 1,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 50,
            },
          ],
        },
      });
      expect(getAgentMachineRaidSummary(failedAndRebuilding)).toBe('1/1 degraded');
    });

    it('reports rebuilding arrays when none have failed', () => {
      const rebuilding = resource({
        agent: {
          raid: [
            {
              device: '/dev/md0',
              level: 'raid5',
              state: 'clean',
              totalDevices: 3,
              activeDevices: 3,
              workingDevices: 3,
              failedDevices: 0,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 50,
            },
          ],
        },
      });
      expect(getAgentMachineRaidSummary(rebuilding)).toBe('1/1 rebuilding');
    });

    it('summarizes arrays sharing a single state', () => {
      const uniform = resource({
        agent: {
          raid: [
            {
              device: '/dev/md0',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 2,
              workingDevices: 2,
              failedDevices: 0,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 0,
            },
            {
              device: '/dev/md1',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 2,
              workingDevices: 2,
              failedDevices: 0,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 0,
            },
          ],
        },
      });
      expect(getAgentMachineRaidSummary(uniform)).toBe('2 clean');
    });

    it('falls back to a count of arrays when states differ', () => {
      const mixed = resource({
        agent: {
          raid: [
            {
              device: '/dev/md0',
              level: 'raid1',
              state: 'clean',
              totalDevices: 2,
              activeDevices: 2,
              workingDevices: 2,
              failedDevices: 0,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 0,
            },
            {
              device: '/dev/md1',
              level: 'raid5',
              state: 'resync',
              totalDevices: 3,
              activeDevices: 3,
              workingDevices: 3,
              failedDevices: 0,
              spareDevices: 0,
              devices: [],
              rebuildPercent: 0,
            },
          ],
        },
      });
      expect(getAgentMachineRaidSummary(mixed)).toBe('2 arrays');
    });
  });

  describe('getAgentMachineDiskIODetails', () => {
    it('reads from the lower-case diskIo fallback and includes read/write time counters', () => {
      const details = getAgentMachineDiskIODetails(
        resource({
          agent: {
            diskIo: [{ device: ' /dev/sda ', readBytes: 100, readTimeMs: 5, writeTimeMs: 8 }],
          },
        }),
      );
      expect(details).toEqual([
        { device: '/dev/sda', readBytes: 100, readTimeMs: 5, writeTimeMs: 8 },
      ]);
    });

    it('prefers diskIO over the lower-case diskIo fallback when both are present', () => {
      const details = getAgentMachineDiskIODetails(
        resource({
          agent: {
            diskIO: [{ device: '/dev/from-diskIO' }],
            diskIo: [{ device: '/dev/from-diskIo' }],
          },
        }),
      );
      expect(details).toEqual([{ device: '/dev/from-diskIO' }]);
    });

    it('omits negative counters while keeping zero values', () => {
      const details = getAgentMachineDiskIODetails(
        resource({
          agent: {
            diskIO: [{ device: ' /dev/sda ', readTimeMs: -5, writeOps: 0, ioTimeMs: 3 }],
          },
        }),
      );
      expect(details).toEqual([{ device: '/dev/sda', writeOps: 0, ioTimeMs: 3 }]);
    });
  });
});
