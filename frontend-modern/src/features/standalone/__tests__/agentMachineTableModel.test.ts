import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
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

  it('includes sensor and active SMART disk temperatures in the machine temperature title', () => {
    const title = getAgentMachineTemperatureTitle(
      resource({
        agent: {
          sensors: {
            temperatureCelsius: { 'cpu.package': 61 },
            smart: [
              { device: '/dev/sda', model: 'Cold Standby', temperature: 33, standby: true },
              { device: '/dev/sdb', model: 'Archive HDD', temperature: 38 },
            ],
          },
        },
      }),
    );

    expect(title.split('\n')).toContain('cpu.package: 61°C');
    expect(title.split('\n')).toContain('Disk /dev/sdb Archive HDD: 38°C');
    expect(title).not.toContain('Cold Standby');
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
