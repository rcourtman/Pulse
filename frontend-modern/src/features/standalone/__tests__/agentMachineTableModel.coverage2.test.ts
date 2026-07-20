import { describe, expect, it } from 'vitest';
import type { HostDiskSMART, HostRAIDArray, HostRAIDDevice } from '@/types/api';
import type { Resource, ResourceDiskIO, ResourceMetric, ResourceNetwork } from '@/types/resource';
import {
  getAgentMachineDiskIOTotal,
  getAgentMachineDiskPercent,
  getAgentMachineIpValues,
  getAgentMachineNetworkTotal,
  getAgentMachineRaidArrayDetails,
  getAgentMachineRaidSummary,
  getAgentMachineTemperatureMetric,
  getAgentMachineTemperatureTitle,
  getAgentMachineThermalPressurePresentation,
  getNextAgentMachineSortState,
  matchesAgentMachineSearch,
  sortAgentMachines,
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

const raidArray = (overrides: Partial<HostRAIDArray> = {}): HostRAIDArray => ({
  device: '',
  level: '',
  state: '',
  totalDevices: 0,
  activeDevices: 0,
  workingDevices: 0,
  failedDevices: 0,
  spareDevices: 0,
  devices: [],
  rebuildPercent: 0,
  ...overrides,
});

const raidDevice = (overrides: Partial<HostRAIDDevice> = {}): HostRAIDDevice => ({
  device: '',
  state: '',
  slot: 0,
  ...overrides,
});

describe('agentMachineTableModel coverage2', () => {
  describe('sortAgentMachines', () => {
    const noopLabel = () => '';

    it('sorts by cpu ascending and pushes machines with missing metrics to the bottom', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'z-no-cpu', name: 'Zulu' }),
          resource({ id: 'high', name: 'High', cpu: { current: 90 } }),
          resource({ id: 'low', name: 'Low', cpu: { current: 10 } }),
        ],
        'cpu',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['low', 'high', 'z-no-cpu']);
    });

    it('sorts by memory descending', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'a', memory: { current: 30 } }),
          resource({ id: 'b', memory: { current: 80 } }),
        ],
        'memory',
        'desc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['b', 'a']);
    });

    it('sorts by network total descending', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'small', network: { rxBytes: 10, txBytes: 20 } }),
          resource({ id: 'big', network: { rxBytes: 1000, txBytes: 2000 } }),
        ],
        'network',
        'desc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['big', 'small']);
    });

    it('sorts by disk I/O total ascending', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'fast', diskIO: { readRate: 10, writeRate: 5 } }),
          resource({ id: 'slow', diskIO: { readRate: 100, writeRate: 50 } }),
        ],
        'diskio',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['fast', 'slow']);
    });

    it('sorts by uptime, falling back to agent.uptimeSeconds and pushing missing to the bottom', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'short', uptime: 100 }),
          resource({ id: 'long', agent: { uptimeSeconds: 99999 } }),
          resource({ id: 'missing' }),
        ],
        'uptime',
        'desc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['long', 'short', 'missing']);
    });

    it('sorts by lastSeen using timestamp normalization', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'old', lastSeen: 1_000 }),
          resource({ id: 'new', lastSeen: 2_000_000_000_000 }),
        ],
        'lastSeen',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['old', 'new']);
    });

    it('sorts by system label ascending and pushes empty labels to the bottom', () => {
      const getSystem = (m: Resource): string =>
        m.id === 'b' ? 'Ubuntu' : m.id === 'a' ? 'Debian' : '';
      const sorted = sortAgentMachines(
        [resource({ id: 'b' }), resource({ id: 'a' }), resource({ id: 'c' })],
        'system',
        'asc',
        getSystem,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['a', 'b', 'c']);
    });

    it('sorts by agent label descending and keeps empty labels at the bottom', () => {
      const getAgent = (m: Resource): string => (m.id === 'x' ? '6.0' : m.id === 'y' ? '5.0' : '');
      const sorted = sortAgentMachines(
        [resource({ id: 'y' }), resource({ id: 'z' }), resource({ id: 'x' })],
        'agent',
        'desc',
        noopLabel,
        getAgent,
      );
      expect(sorted.map((m) => m.id)).toEqual(['x', 'y', 'z']);
    });

    it('sorts by primary IP address ascending and pushes machines without IPs last', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'high', identity: { ips: ['10.0.0.99'] } }),
          resource({ id: 'low', identity: { ips: ['10.0.0.10'] } }),
          resource({ id: 'none' }),
        ],
        'ip',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['low', 'high', 'none']);
    });

    it('sorts by RAID summary text', () => {
      const sorted = sortAgentMachines(
        [
          resource({
            id: 'degraded',
            agent: { raid: [raidArray({ device: '/dev/md0', state: 'clean', failedDevices: 1 })] },
          }),
          resource({
            id: 'clean',
            agent: { raid: [raidArray({ device: '/dev/md0', state: 'clean' })] },
          }),
        ],
        'raid',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['clean', 'degraded']);
    });

    it('sorts by architecture', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'arm', agent: { architecture: 'arm64' } }),
          resource({ id: 'x86', agent: { architecture: 'x86_64' } }),
        ],
        'arch',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['arm', 'x86']);
    });

    it('sorts by kernel version', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'newer', agent: { kernelVersion: '6.6' } }),
          resource({ id: 'older', agent: { kernelVersion: '5.4' } }),
        ],
        'kernel',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['older', 'newer']);
    });

    it('sorts by machine name by default', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'gamma', name: 'Gamma' }),
          resource({ id: 'alpha', name: 'Alpha' }),
          resource({ id: 'beta', name: 'Beta' }),
        ],
        'name',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['alpha', 'beta', 'gamma']);
    });

    it('uses the display-name tiebreaker when primary sort values are equal', () => {
      const sorted = sortAgentMachines(
        [
          resource({ id: 'z', name: 'Zulu', cpu: { current: 50 } }),
          resource({ id: 'a', name: 'Alpha', cpu: { current: 50 } }),
        ],
        'cpu',
        'asc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['a', 'z']);
    });

    it('falls back to the name tiebreaker when both system labels are empty', () => {
      const sorted = sortAgentMachines(
        [resource({ id: 'z', name: 'Zulu' }), resource({ id: 'a', name: 'Alpha' })],
        'system',
        'desc',
        noopLabel,
        noopLabel,
      );
      expect(sorted.map((m) => m.id)).toEqual(['a', 'z']);
    });
  });

  describe('getNextAgentMachineSortState', () => {
    it('toggles direction when the same key is clicked again', () => {
      expect(getNextAgentMachineSortState('cpu', 'asc', 'cpu')).toEqual({
        key: 'cpu',
        direction: 'desc',
      });
      expect(getNextAgentMachineSortState('name', 'desc', 'name')).toEqual({
        key: 'name',
        direction: 'asc',
      });
    });

    it('defaults metric keys to desc when switching to a new key', () => {
      expect(getNextAgentMachineSortState('name', 'asc', 'memory')).toEqual({
        key: 'memory',
        direction: 'desc',
      });
    });

    it('defaults text keys to asc when switching to a new key', () => {
      expect(getNextAgentMachineSortState('cpu', 'desc', 'name')).toEqual({
        key: 'name',
        direction: 'asc',
      });
    });
  });

  describe('getAgentMachineNetworkTotal', () => {
    it('sums rx and tx bytes', () => {
      expect(
        getAgentMachineNetworkTotal(resource({ network: { rxBytes: 100, txBytes: 200 } })),
      ).toBe(300);
    });

    it('returns rx only when tx is absent', () => {
      expect(
        getAgentMachineNetworkTotal(
          resource({ network: { rxBytes: 100 } as unknown as ResourceNetwork }),
        ),
      ).toBe(100);
    });

    it('returns tx only when rx is absent', () => {
      expect(
        getAgentMachineNetworkTotal(
          resource({ network: { txBytes: 200 } as unknown as ResourceNetwork }),
        ),
      ).toBe(200);
    });

    it('returns undefined when both rx and tx are absent', () => {
      expect(getAgentMachineNetworkTotal(resource({}))).toBeUndefined();
    });
  });

  describe('getAgentMachineDiskIOTotal', () => {
    it('sums read and write rates', () => {
      expect(
        getAgentMachineDiskIOTotal(resource({ diskIO: { readRate: 500, writeRate: 300 } })),
      ).toBe(800);
    });

    it('returns read only when write is absent', () => {
      expect(
        getAgentMachineDiskIOTotal(
          resource({ diskIO: { readRate: 500 } as unknown as ResourceDiskIO }),
        ),
      ).toBe(500);
    });

    it('returns write only when read is absent', () => {
      expect(
        getAgentMachineDiskIOTotal(
          resource({ diskIO: { writeRate: 300 } as unknown as ResourceDiskIO }),
        ),
      ).toBe(300);
    });

    it('returns undefined when both rates are absent', () => {
      expect(getAgentMachineDiskIOTotal(resource({}))).toBeUndefined();
    });
  });

  describe('getAgentMachineDiskPercent', () => {
    it('prefers agent disk percentages over machine-level disk metrics', () => {
      const machine = resource({
        disk: { total: 1000, used: 900, current: 90 },
        agent: {
          disks: [{ device: '/dev/sda1', mountpoint: '/', type: 'ext4', total: 1000, used: 500 }],
        },
      });
      expect(getAgentMachineDiskPercent(machine)).toBe(50);
    });

    it('returns zero rather than falling back when agent disks exist but are all empty', () => {
      const machine = resource({
        disk: { total: 1000, used: 900, current: 90 },
        agent: {
          disks: [{ device: '/dev/sda1', mountpoint: '/', type: 'ext4', total: 0, used: 0 }],
        },
      });
      expect(getAgentMachineDiskPercent(machine)).toBe(0);
    });

    it('falls back to machine disk total/used when no operational agent disks exist', () => {
      expect(
        getAgentMachineDiskPercent(
          resource({ disk: { total: 500, used: 125 } as unknown as ResourceMetric }),
        ),
      ).toBe(25);
    });

    it('falls back to disk.current when total is non-positive', () => {
      expect(
        getAgentMachineDiskPercent(resource({ disk: { total: 0, used: 100, current: 42 } })),
      ).toBe(42);
    });

    it('falls back to disk.current when used is missing', () => {
      expect(getAgentMachineDiskPercent(resource({ disk: { total: 500, current: 33 } }))).toBe(33);
    });

    it('returns undefined when no disk data is available', () => {
      expect(getAgentMachineDiskPercent(resource({}))).toBeUndefined();
    });
  });

  describe('getAgentMachineThermalPressurePresentation', () => {
    it('returns undefined when no pressure is set', () => {
      expect(getAgentMachineThermalPressurePresentation(resource({}))).toBeUndefined();
    });

    it('returns undefined for whitespace-only pressure', () => {
      expect(
        getAgentMachineThermalPressurePresentation(
          resource({ agent: { sensors: { thermalState: { pressure: '  ' } } } }),
        ),
      ).toBeUndefined();
    });

    it('presents nominal pressure with emerald styling', () => {
      expect(
        getAgentMachineThermalPressurePresentation(
          resource({ agent: { sensors: { thermalState: { pressure: 'nominal' } } } }),
        ),
      ).toEqual({
        label: 'Nominal',
        className: 'text-emerald-600 dark:text-emerald-400',
        title: 'Thermal pressure nominal',
      });
    });

    it('presents constrained pressure with amber styling', () => {
      expect(
        getAgentMachineThermalPressurePresentation(
          resource({ agent: { sensors: { thermalState: { pressure: 'constrained' } } } }),
        ),
      ).toEqual({
        label: 'Constrained',
        className: 'text-amber-600 dark:text-amber-400',
        title: 'Thermal pressure constrained',
      });
    });

    it('presents unrecognised pressure values with muted styling', () => {
      expect(
        getAgentMachineThermalPressurePresentation(
          resource({ agent: { sensors: { thermalState: { pressure: 'critical' } } } }),
        ),
      ).toEqual({
        label: 'Unknown',
        className: 'text-muted',
        title: 'Thermal pressure unknown',
      });
    });

    it('includes the source and filtered, sorted limits in the title', () => {
      const result = getAgentMachineThermalPressurePresentation(
        resource({
          agent: {
            sensors: {
              thermalState: {
                pressure: 'nominal',
                source: 'throttling',
                limitsPercent: {
                  cpu_package: 80,
                  full: 100,
                  disk_array: 90,
                  gpu_hotspot: 95,
                },
              },
            },
          },
        }),
      );
      expect(result?.title).toBe(
        'Thermal pressure nominal via throttling; limits: cpu package 80%, disk array 90%, gpu hotspot 95%',
      );
    });
  });

  describe('getAgentMachineRaidArrayDetails', () => {
    it('returns an empty array when no raid data exists', () => {
      expect(getAgentMachineRaidArrayDetails(resource({}))).toEqual([]);
    });

    it('skips arrays whose identifying fields are all empty', () => {
      const details = getAgentMachineRaidArrayDetails(
        resource({
          agent: {
            raid: [
              raidArray({
                device: '',
                name: '',
                level: '',
                state: '',
                devices: [],
                rebuildPercent: 0,
              }),
            ],
          },
        }),
      );
      expect(details).toEqual([]);
    });

    it('falls back device label to array name then to array-N index', () => {
      const details = getAgentMachineRaidArrayDetails(
        resource({
          agent: {
            raid: [
              raidArray({ device: '', name: ' storage ', state: 'active' }),
              raidArray({ device: '', name: '', state: 'clean' }),
            ],
          },
        }),
      );
      expect(details.map((d) => d.device)).toEqual(['storage', 'array-2']);
      expect(details.map((d) => d.level)).toEqual(['unknown', 'unknown']);
    });

    it('skips empty raid devices, falls back slot to index, and rounds fractional slots', () => {
      const details = getAgentMachineRaidArrayDetails(
        resource({
          agent: {
            raid: [
              raidArray({
                device: '/dev/md0',
                state: 'clean',
                devices: [
                  raidDevice({ device: '', state: '', slot: undefined as unknown as number }),
                  raidDevice({ device: '', state: '', slot: 3 }),
                  raidDevice({ device: '/dev/sda', state: 'active', slot: 5.7 }),
                ],
              }),
            ],
          },
        }),
      );
      expect(details[0]?.devices).toEqual([
        { device: 'disk-2', state: 'unknown', slot: 3 },
        { device: '/dev/sda', state: 'active', slot: 6 },
      ]);
    });
  });

  describe('getAgentMachineRaidSummary', () => {
    it('summarises a single array by its state', () => {
      const machine = resource({
        agent: { raid: [raidArray({ device: '/dev/md0', state: 'active' })] },
      });
      expect(getAgentMachineRaidSummary(machine)).toBe('1 active');
    });

    it('falls back to an array count when all arrays normalise to unknown state', () => {
      const machine = resource({
        agent: {
          raid: [raidArray({ device: '/dev/md0' }), raidArray({ device: '/dev/md1' })],
        },
      });
      expect(getAgentMachineRaidSummary(machine)).toBe('2 unknown');
    });
  });

  describe('getAgentMachineTemperatureMetric', () => {
    it('defaults to temperature when no temperature data is available', () => {
      expect(getAgentMachineTemperatureMetric(resource({}))).toBe('temperature');
    });
  });

  describe('getAgentMachineIpValues', () => {
    it('merges identity ips and interface addresses with deduplication and trimming', () => {
      const machine = resource({
        identity: { ips: ['192.168.0.1', ' 10.0.0.1 '] },
        agent: {
          networkInterfaces: [{ name: 'eth0', addresses: ['192.168.0.1', 'fe80::1', ''] }],
        },
      });
      expect(getAgentMachineIpValues(machine)).toEqual(['192.168.0.1', '10.0.0.1', 'fe80::1']);
    });

    it('returns an empty array when no ip sources exist', () => {
      expect(getAgentMachineIpValues(resource({}))).toEqual([]);
    });

    it('handles undefined addresses arrays gracefully', () => {
      expect(
        getAgentMachineIpValues(resource({ agent: { networkInterfaces: [{ name: 'eth0' }] } })),
      ).toEqual([]);
    });
  });

  describe('matchesAgentMachineSearch', () => {
    const noop = () => '';

    it('returns true for empty or whitespace-only search', () => {
      const machine = resource({ name: 'Host' });
      expect(matchesAgentMachineSearch(machine, '', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, '   ', noop, noop)).toBe(true);
    });

    it('matches sensor label names from the search haystack', () => {
      const machine = resource({
        agent: {
          sensors: {
            temperatureCelsius: { 'core-0': 55 },
            fanRpm: { 'cpu-fan': 1200 },
            additional: { 'nvme-controller': 40 },
          },
        },
      });
      expect(matchesAgentMachineSearch(machine, 'core-0', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'cpu-fan', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'nvme-controller', noop, noop)).toBe(true);
    });

    it('matches SMART disk identifiers and the standby flag', () => {
      const machine = resource({
        agent: {
          sensors: {
            smart: [
              {
                device: '/dev/sda',
                model: 'Samsung',
                serial: 'SN123',
                standby: true,
              } as unknown as HostDiskSMART,
            ],
          },
        },
      });
      expect(matchesAgentMachineSearch(machine, 'samsung', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'SN123', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, 'standby', noop, noop)).toBe(true);
    });

    it('matches numeric agent fields like cpuCount and uptimeSeconds', () => {
      const machine = resource({ agent: { cpuCount: 16, uptimeSeconds: 86400 } });
      expect(matchesAgentMachineSearch(machine, '16', noop, noop)).toBe(true);
      expect(matchesAgentMachineSearch(machine, '86400', noop, noop)).toBe(true);
    });

    it('matches RAID summary text in the search haystack', () => {
      const machine = resource({
        agent: { raid: [raidArray({ device: '/dev/md0', state: 'clean', failedDevices: 1 })] },
      });
      expect(matchesAgentMachineSearch(machine, 'degraded', noop, noop)).toBe(true);
    });

    it('returns false when the needle appears in no field', () => {
      expect(
        matchesAgentMachineSearch(resource({ name: 'Alpha' }), 'zzz-not-found', noop, noop),
      ).toBe(false);
    });
  });

  describe('flattenTemperatureSections (via getAgentMachineTemperatureTitle)', () => {
    it('renders overflow rows as a bare label without a colon when capped', () => {
      const title = getAgentMachineTemperatureTitle(
        resource({
          agent: {
            sensors: {
              temperatureCelsius: { s1: 10, s2: 20, s3: 30, s4: 40, s5: 50, s6: 60, s7: 70 },
            },
          },
        }),
      );
      const lines = title.split('\n');
      expect(lines).toContain('+1 more');
      expect(lines).toContain('s7: 70°C');
    });
  });
});
