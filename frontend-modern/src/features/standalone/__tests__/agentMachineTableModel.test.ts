import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineDiskPercent,
  getAgentMachineDiskIODetails,
  getAgentMachineNetworkInterfaceDetails,
  getAgentMachineRaidArrayDetails,
  getAgentMachineTemperatureCelsius,
  getAgentMachineTemperatureMetric,
  getAgentMachineTemperatureTitle,
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

describe('agentMachineTableModel', () => {
  it('normalizes agent disk I/O device counters for table inspection', () => {
    const details = getAgentMachineDiskIODetails(
      resource({
        agent: {
          diskIO: [
            {
              device: ' /dev/sda ',
              readBytes: 1024,
              writeBytes: 2048,
              readOps: 10,
              writeOps: 20,
              ioTimeMs: 30,
            },
            {
              device: '   ',
              readBytes: 0,
              writeBytes: -1,
            },
            { device: '' },
          ],
        },
      }),
    );

    expect(details).toEqual([
      {
        device: '/dev/sda',
        readBytes: 1024,
        writeBytes: 2048,
        readOps: 10,
        writeOps: 20,
        ioTimeMs: 30,
      },
      {
        device: 'disk-2',
        readBytes: 0,
      },
    ]);
  });

  it('normalizes agent RAID array details for table inspection', () => {
    const details = getAgentMachineRaidArrayDetails(
      resource({
        agent: {
          raid: [
            {
              device: ' /dev/md0 ',
              name: ' media ',
              level: ' raid6 ',
              state: ' clean ',
              totalDevices: 6,
              activeDevices: 6,
              workingDevices: 6,
              failedDevices: 0,
              spareDevices: 1,
              devices: [
                { device: ' /dev/sda ', state: ' active ', slot: 0 },
                { device: '', state: ' spare ', slot: 7 },
              ],
              rebuildPercent: 12.4,
              rebuildSpeed: ' 120 MB/s ',
            },
            {
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
            },
          ],
        },
      }),
    );

    expect(details).toEqual([
      {
        device: '/dev/md0',
        name: 'media',
        level: 'raid6',
        state: 'clean',
        totalDevices: 6,
        activeDevices: 6,
        workingDevices: 6,
        failedDevices: 0,
        spareDevices: 1,
        devices: [
          { device: '/dev/sda', state: 'active', slot: 0 },
          { device: 'disk-2', state: 'spare', slot: 7 },
        ],
        rebuildPercent: 12.4,
        rebuildSpeed: '120 MB/s',
      },
    ]);
  });

  it('normalizes agent network interface details for table inspection', () => {
    const details = getAgentMachineNetworkInterfaceDetails(
      resource({
        agent: {
          networkInterfaces: [
            {
              name: ' en0 ',
              mac: ' 10:20:30:40:50:60 ',
              addresses: [' 192.168.0.20 ', '192.168.0.20', '', ' fe80::1 '],
              rxBytes: 1024,
              txBytes: 2048,
              speedMbps: 1000,
            },
            {
              name: '   ',
              addresses: ['10.0.0.2'],
              txBytes: 512,
              speedMbps: 0,
            },
            { name: '' },
          ],
        },
      }),
    );

    expect(details).toEqual([
      {
        name: 'en0',
        mac: '10:20:30:40:50:60',
        addresses: ['192.168.0.20', 'fe80::1'],
        rxBytes: 1024,
        txBytes: 2048,
        speedMbps: 1000,
      },
      {
        name: 'eth1',
        addresses: ['10.0.0.2'],
        txBytes: 512,
      },
    ]);
  });

  it('falls back to active SMART disk temperatures when machine sensors are absent', () => {
    const machine = resource({
      agent: {
        sensors: {
          smart: [
            { device: '/dev/sda', model: 'Standby HDD', temperature: 62, standby: true },
            { device: '/dev/sdb', model: 'Archive HDD', temperature: 38 },
            { device: '/dev/nvme0', model: 'Fast SSD', temperature: 44 },
          ],
        },
      },
    });

    expect(getAgentMachineTemperatureCelsius(machine)).toBe(44);
    expect(getAgentMachineTemperatureMetric(machine)).toBe('diskTemperature');
  });

  it('prefers direct and sensor temperatures over SMART fallback temperatures', () => {
    const direct = resource({
      temperature: 55,
      agent: {
        sensors: {
          temperatureCelsius: { 'cpu.package': 61 },
          smart: [{ device: '/dev/nvme0', temperature: 72 }],
        },
      },
    });
    expect(getAgentMachineTemperatureCelsius(direct)).toBe(55);
    expect(getAgentMachineTemperatureMetric(direct)).toBe('temperature');

    const sensor = resource({
      agent: {
        sensors: {
          temperatureCelsius: { 'cpu.package': 61, 'cpu.core0': 64 },
          smart: [{ device: '/dev/nvme0', temperature: 72 }],
        },
      },
    });
    expect(getAgentMachineTemperatureCelsius(sensor)).toBe(64);
    expect(getAgentMachineTemperatureMetric(sensor)).toBe('temperature');
  });

  it('includes agent sensor, SMART, fan, and additional readings in the temperature title', () => {
    const title = getAgentMachineTemperatureTitle(
      resource({
        agent: {
          sensors: {
            temperatureCelsius: { 'cpu.package': 61 },
            fanRpm: { cpu_fan: 1_400 },
            additional: { nvme0: 42 },
            smart: [
              { device: '/dev/sda', model: 'Cold Standby', temperature: 33, standby: true },
              { device: '/dev/sdb', model: 'Archive HDD', temperature: 38 },
            ],
          },
        },
      }),
    );

    expect(title.split('\n')).toEqual([
      'Temperatures',
      'cpu.package: 61°C',
      'Disk Temperatures',
      'Disk /dev/sdb Archive HDD: 38°C',
      'Disk /dev/sda Cold Standby: standby',
      'Fan Speeds',
      'cpu_fan: 1400 RPM',
      'Other Sensors',
      'nvme0: 42°C',
    ]);
  });

  it('sorts machines by SMART fallback temperature when no direct temperature is present', () => {
    const sorted = sortAgentMachines(
      [
        resource({
          id: 'cool',
          name: 'Cool',
          agent: {
            sensors: {
              smart: [{ device: '/dev/sda', temperature: 34 }],
            },
          },
        }),
        resource({
          id: 'warm',
          name: 'Warm',
          agent: {
            sensors: {
              smart: [{ device: '/dev/sda', temperature: 47 }],
            },
          },
        }),
      ],
      'temp',
      'desc',
      () => '',
      () => '',
    );

    expect(sorted.map((machine) => machine.id)).toEqual(['warm', 'cool']);
  });

  it('uses the worst operational filesystem as the machine disk percent', () => {
    const machine = resource({
      disk: {
        total: 1000,
        used: 300,
        free: 700,
        current: 30,
      },
      agent: {
        disks: [
          {
            device: '/dev/sda2',
            mountpoint: '/',
            type: 'ext4',
            total: 1000,
            used: 420,
            free: 580,
          },
          {
            device: '/dev/sdb1',
            mountpoint: '/var/lib/postgresql',
            type: 'xfs',
            total: 1000,
            used: 920,
            free: 80,
          },
          {
            device: 'pmxcfs',
            mountpoint: '/etc/pve',
            type: 'fuse',
            total: 1000,
            used: 990,
            free: 10,
          },
          {
            device: 'D:',
            mountpoint: 'D:\\',
            type: 'NTFS',
            total: 1000,
            used: 510,
            free: 490,
          },
          {
            device: 'system-reserved',
            mountpoint: 'System Reserved',
            type: 'NTFS',
            total: 1000,
            used: 980,
            free: 20,
          },
        ],
      },
    });

    expect(getAgentMachineDiskPercent(machine)).toBe(92);
  });

  it('sorts machines by worst operational disk pressure', () => {
    const sorted = sortAgentMachines(
      [
        resource({
          id: 'aggregate-heavy',
          name: 'Aggregate Heavy',
          disk: { total: 10_000, used: 6_000, free: 4_000, current: 60 },
          agent: {
            disks: [
              {
                device: '/dev/sda1',
                mountpoint: '/',
                type: 'ext4',
                total: 10_000,
                used: 6_000,
                free: 4_000,
              },
            ],
          },
        }),
        resource({
          id: 'small-hot-volume',
          name: 'Small Hot Volume',
          disk: { total: 10_000, used: 2_000, free: 8_000, current: 20 },
          agent: {
            disks: [
              {
                device: '/dev/sda1',
                mountpoint: '/',
                type: 'ext4',
                total: 9_000,
                used: 1_000,
                free: 8_000,
              },
              {
                device: '/dev/sdb1',
                mountpoint: '/srv/app',
                type: 'xfs',
                total: 1_000,
                used: 910,
                free: 90,
              },
            ],
          },
        }),
      ],
      'disk',
      'desc',
      () => '',
      () => '',
    );

    expect(sorted.map((machine) => machine.id)).toEqual(['small-hot-volume', 'aggregate-heavy']);
  });

  it('matches machine-native search fields beyond the generic resource identity', () => {
    const machine = resource({
      id: 'richard-mac-mini',
      name: 'Richard Mac Mini',
      identity: { hostname: 'richard-mac-mini.local', ips: ['192.168.0.98'] },
      agent: {
        agentVersion: '6.0.0',
        hostname: 'richard-mac-mini.local',
        osName: 'macOS',
        osVersion: '15.5',
        kernelVersion: 'Darwin 24.5.0',
        architecture: 'arm64',
        networkInterfaces: [
          {
            name: 'en0',
            mac: '10:20:30:40:50:60',
            addresses: ['10.0.0.98'],
          },
        ],
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
            devices: [{ device: '/dev/disk3', state: 'active', slot: 0 }],
            rebuildPercent: 0,
          },
        ],
      },
    });
    const getSystemLabel = () => 'macOS 15.5';
    const getAgentLabel = () => '6.0.0';

    expect(matchesAgentMachineSearch(machine, 'macos', getSystemLabel, getAgentLabel)).toBe(true);
    expect(matchesAgentMachineSearch(machine, '10.0.0.98', getSystemLabel, getAgentLabel)).toBe(
      true,
    );
    expect(matchesAgentMachineSearch(machine, 'arm64', getSystemLabel, getAgentLabel)).toBe(true);
    expect(matchesAgentMachineSearch(machine, 'darwin', getSystemLabel, getAgentLabel)).toBe(true);
    expect(matchesAgentMachineSearch(machine, '/dev/disk3', getSystemLabel, getAgentLabel)).toBe(
      true,
    );
    expect(matchesAgentMachineSearch(machine, 'windows', getSystemLabel, getAgentLabel)).toBe(
      false,
    );
  });
});
