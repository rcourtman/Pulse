import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineDiskIODetails,
  getAgentMachineNetworkInterfaceDetails,
  getAgentMachineRaidArrayDetails,
  getAgentMachineTemperatureCelsius,
  getAgentMachineTemperatureTitle,
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
  });

  it('prefers direct and sensor temperatures over SMART fallback temperatures', () => {
    expect(
      getAgentMachineTemperatureCelsius(
        resource({
          temperature: 55,
          agent: {
            sensors: {
              temperatureCelsius: { 'cpu.package': 61 },
              smart: [{ device: '/dev/nvme0', temperature: 72 }],
            },
          },
        }),
      ),
    ).toBe(55);

    expect(
      getAgentMachineTemperatureCelsius(
        resource({
          agent: {
            sensors: {
              temperatureCelsius: { 'cpu.package': 61, 'cpu.core0': 64 },
              smart: [{ device: '/dev/nvme0', temperature: 72 }],
            },
          },
        }),
      ),
    ).toBe(64);
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
});
