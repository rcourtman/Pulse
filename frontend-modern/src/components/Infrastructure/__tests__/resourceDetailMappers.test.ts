import { describe, expect, it } from 'vitest';
import {
  buildCustomSensorRows,
  buildTemperatureRows,
  HOST_AGENT_STOPPED_REPORTING_REASON,
  formatInteger,
  formatSensorName,
  formatSourceType,
  toAgentFromResource,
  toNodeFromProxmox,
} from '@/components/Infrastructure/resourceDetailMappers';
import resourceDetailMappersSource from '@/components/Infrastructure/resourceDetailMappers.ts?raw';
import resourceDetailDiscoveryModelSource from '@/components/Infrastructure/resourceDetailDiscoveryModel.ts?raw';
import type { PhysicalDiskCollectionStatus, Resource } from '@/types/resource';

const createHybridHostResource = (): Resource =>
  ({
    id: 'resource:host:hash-1',
    type: 'agent',
    name: 'tower',
    displayName: 'Tower',
    platformId: 'tower',
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: Date.now(),
    cpu: { current: 15 },
    memory: { current: 0.25, total: 1024, used: 256, free: 768 },
    disk: { current: 0.25, total: 2048, used: 512, free: 1536 },
    platformData: {
      proxmox: {
        nodeName: 'pve-node-1',
      },
      agent: {
        agentId: 'agent-canonical',
        agentVersion: '1.2.3',
        hostname: 'tower.local',
        osName: 'Unraid',
        kernelVersion: '6.1.0',
      },
    },
  }) as unknown as Resource;

describe('resourceDetailMappers', () => {
  describe('formatInteger', () => {
    it('returns dash for undefined', () => {
      expect(formatInteger(undefined)).toBe('—');
    });

    it('returns dash for null', () => {
      expect(formatInteger(undefined)).toBe('—');
    });

    it('returns dash for NaN', () => {
      expect(formatInteger(NaN)).toBe('—');
    });

    it('formats integer with commas', () => {
      expect(formatInteger(1000)).toBe('1,000');
      expect(formatInteger(1000000)).toBe('1,000,000');
    });

    it('rounds decimal values', () => {
      expect(formatInteger(1000.7)).toBe('1,001');
      expect(formatInteger(1000.3)).toBe('1,000');
    });

    it('handles zero', () => {
      expect(formatInteger(0)).toBe('0');
    });

    it('handles negative numbers', () => {
      expect(formatInteger(-1000)).toBe('-1,000');
    });
  });

  describe('formatSourceType', () => {
    it('returns Hybrid for hybrid', () => {
      expect(formatSourceType('hybrid')).toBe('Hybrid');
    });

    it('returns Agent for agent', () => {
      expect(formatSourceType('agent')).toBe('Agent');
    });

    it('returns API for api', () => {
      expect(formatSourceType('api')).toBe('API');
    });

    it('returns unknown source type as-is', () => {
      expect(formatSourceType('unknown-source' as any)).toBe('unknown-source');
    });
  });

  describe('formatSensorName', () => {
    it('strips sensor prefixes and title-cases the remainder with the shared helper', () => {
      expect(formatSensorName('fan1_cpu_temp')).toBe('Cpu Temp');
      expect(formatSensorName('disk_0_temp')).toBe('0 Temp');
      expect(formatSensorName('')).toBe('');
    });
  });

  describe('buildTemperatureRows', () => {
    it('surfaces macOS thermal pressure without inventing Celsius temperatures', () => {
      const rows = buildTemperatureRows({
        thermalState: {
          source: 'pmset',
          pressure: 'constrained',
          limitsPercent: {
            cpu_speed_limit: 80,
            scheduler_limit: 100,
          },
        },
      });

      expect(rows).toEqual([
        {
          label: 'Thermal pressure',
          value: 'Constrained',
          valueTitle: 'Constrained via pmset',
        },
        {
          label: 'Speed Limit',
          value: '80%',
          valueTitle: 'Speed Limit 80%',
        },
      ]);
    });

    it('surfaces typed GPU utilization and memory readings', () => {
      const rows = buildTemperatureRows({
        temperatureCelsius: {
          gpu_nvidia_0: 63,
          cpu_package: 41,
        },
        gpu: [
          {
            id: '0',
            name: 'NVIDIA RTX A6000',
            temperatureCelsius: 63,
            utilizationPercent: 0,
            memoryUsedBytes: 2 * 1024 * 1024 * 1024,
            memoryTotalBytes: 48 * 1024 * 1024 * 1024,
          },
        ],
      });

      expect(rows).toEqual([
        {
          label: 'GPU 0',
          value: 'NVIDIA RTX A6000 · 63°C · 0% · 2.00 GB / 48.0 GB',
          valueTitle: 'NVIDIA RTX A6000 · 63°C · 0% · 2.00 GB / 48.0 GB',
        },
        {
          label: 'Package',
          value: '41°C',
          valueTitle: '41.0°C',
        },
      ]);
    });

    it('surfaces host power, fan, and additional sensor rows', () => {
      const rows = buildTemperatureRows({
        temperatureCelsius: {
          cpu_package: 41,
        },
        additional: {
          vrm_temp: 55.6,
        },
        fanRpm: {
          chassis_fan: 1199.6,
        },
        powerWatts: {
          cpu_package: 82.4,
          dram: 13.2,
        },
      });

      expect(rows).toEqual([
        {
          label: 'Package',
          value: '41°C',
          valueTitle: '41.0°C',
        },
        {
          label: 'VRM Temp',
          value: '56°C',
          valueTitle: '55.6°C',
        },
        {
          label: 'Chassis Fan',
          value: '1,200 RPM',
          valueTitle: 'Chassis Fan 1,200 RPM',
        },
        {
          label: 'CPU Package Power',
          value: '82.4 W',
          valueTitle: 'CPU Package Power 82.4 W',
        },
        {
          label: 'DRAM Power',
          value: '13.2 W',
          valueTitle: 'DRAM Power 13.2 W',
        },
      ]);
    });

    it('marks a silent agent disk temperature as last known and drops disks without one', () => {
      const rows = buildTemperatureRows({
        smart: [
          {
            device: 'sda',
            temperature: 71,
            // A host agent past its reporting lease keeps its last reading.
            collection: {
              temperature: {
                state: 'unavailable',
                source: 'host_agent',
                reason: 'host agent stopped reporting',
              },
            },
          },
          {
            device: 'sdb',
            temperature: 38,
            collection: { temperature: { state: 'available', source: 'smartctl' } },
          },
          {
            device: 'sdc',
            temperature: 0,
            collection: {
              temperature: {
                state: 'unsupported',
                source: 'smartctl',
                reason: 'device did not expose a temperature reading',
              },
            },
          },
        ],
      });

      expect(rows).toEqual([
        {
          label: 'Disk sda',
          value: '71°C (last known)',
          valueTitle: 'Last known reading, not current: host agent stopped reporting',
        },
        { label: 'Disk sdb', value: '38°C', valueTitle: '38.0°C' },
      ]);
    });

    it("marks a silent agent's sensor readings last known and leaves disks to their own state", () => {
      const rows = buildTemperatureRows(
        {
          thermalState: { pressure: 'serious' },
          temperatureCelsius: { 'cpu.package': 45 },
          fanRpm: { chassis_fan: 1200 },
          smart: [
            {
              device: 'sda',
              temperature: 38,
              collection: { temperature: { state: 'available', source: 'smartctl' } },
            },
          ],
        },
        { lastKnownReason: HOST_AGENT_STOPPED_REPORTING_REASON },
      );

      const lastKnown = 'Last known reading, not current: host agent stopped reporting';
      expect(rows).toEqual([
        { label: 'Thermal pressure', value: 'Serious (last known)', valueTitle: lastKnown },
        { label: 'Cpu.package', value: '45°C (last known)', valueTitle: lastKnown },
        { label: 'Chassis Fan', value: '1,200 RPM (last known)', valueTitle: lastKnown },
        // A disk row follows its own collection state, which the backend
        // withdraws when the lease expires.
        { label: 'Disk sda', value: '38°C', valueTitle: '38.0°C' },
      ]);
      expect(buildTemperatureRows({ temperatureCelsius: { 'cpu.package': 45 } })).toEqual([
        { label: 'Cpu.package', value: '45°C', valueTitle: '45.0°C' },
      ]);
    });

    it('lists a SMART row only for a positive reading in any state, never a standby disk', () => {
      const states: (PhysicalDiskCollectionStatus | undefined)[] = [
        undefined,
        { temperature: { state: 'available', source: 'smartctl' } },
        { temperature: { state: 'unavailable', source: 'smartctl' } },
      ];
      for (const collection of states) {
        for (const temperature of [0, -3]) {
          expect(
            buildTemperatureRows({ smart: [{ device: 'sda', temperature, collection }] }),
          ).toEqual([]);
        }
      }

      expect(
        buildTemperatureRows({
          smart: [
            {
              device: 'sda',
              temperature: 44,
              standby: true,
              collection: { temperature: { state: 'unavailable', source: 'smartctl' } },
            },
            {
              device: 'sdb',
              temperature: 39,
              collection: { temperature: { state: 'missing', source: 'smartctl' } },
            },
          ],
        }),
      ).toEqual([
        {
          label: 'Disk sdb',
          value: '39°C (last known)',
          valueTitle: 'Last known reading, not current',
        },
      ]);
    });
  });

  describe('buildCustomSensorRows', () => {
    it('formats units, status, and stale last-good values without treating them as temperatures', () => {
      expect(
        buildCustomSensorRows({
          custom: [
            {
              id: 'queue_depth',
              name: 'Queue depth',
              unit: 'items',
              value: 12.25,
              status: 'warning',
              observedAt: '2026-07-30T19:00:00Z',
            },
            {
              id: 'cache_hit_rate',
              name: 'Cache hit rate',
              unit: '%',
              value: 98.5,
              status: 'error',
              observedAt: '2026-07-30T18:55:00Z',
              error: 'probe timed out',
              stale: true,
            },
          ],
        }),
      ).toEqual([
        {
          label: 'Cache hit rate',
          value: '98.5% (stale)',
          valueTitle: '98.5% (stale) · Error · probe timed out',
        },
        {
          label: 'Queue depth',
          value: '12.25 items',
          valueTitle: '12.25 items · Warning',
        },
      ]);
    });

    it('shows unavailable for a failed sensor with no last-good value', () => {
      expect(
        buildCustomSensorRows({
          custom: [
            {
              id: 'failed',
              name: 'Failed probe',
              status: 'error',
              observedAt: '2026-07-30T19:00:00Z',
              error: 'invalid output',
            },
          ],
        }),
      ).toEqual([
        {
          label: 'Failed probe',
          value: 'Unavailable',
          valueTitle: 'Unavailable · Error · invalid output',
        },
      ]);
    });

    it('groups boolean and timestamp metrics and formats their typed values', () => {
      expect(
        buildCustomSensorRows({
          custom: [
            {
              id: 'backup_age',
              name: 'Last file backup',
              group: 'Main server',
              subgroup: 'Backup',
              kind: 'timestamp',
              value: 7_200,
              status: 'warning',
              observedAt: '2026-07-30T20:00:00Z',
              eventAt: '2026-07-30T18:00:00Z',
            },
            {
              id: 'dns_online',
              name: 'DNS online',
              group: 'Main server',
              subgroup: 'Services',
              kind: 'boolean',
              value: 0,
              status: 'critical',
              observedAt: '2026-07-30T20:00:00Z',
            },
          ],
        }),
      ).toEqual([
        {
          label: 'Main server / Backup / Last file backup',
          value: '2h ago',
          valueTitle: '2h ago · Warning',
        },
        {
          label: 'Main server / Services / DNS online',
          value: 'Offline',
          valueTitle: 'Offline · Critical',
        },
      ]);
    });

    it('formats an online boolean metric with an OK status', () => {
      expect(
        buildCustomSensorRows({
          custom: [
            {
              id: 'service_online',
              name: 'Checkout',
              group: 'Main server',
              subgroup: 'Services',
              kind: 'boolean',
              value: 1,
              status: 'ok',
              observedAt: '2026-07-30T20:00:00Z',
            },
          ],
        }),
      ).toEqual([
        {
          label: 'Main server / Services / Checkout',
          value: 'Online',
          valueTitle: 'Online · OK',
        },
      ]);
    });
  });

  describe('detail row projection boundary', () => {
    it('formats temperature, GPU, power, fan and custom rows from the selected resource payload only', () => {
      expect(resourceDetailMappersSource).not.toContain('fetch(');
      expect(resourceDetailMappersSource).not.toContain('nvidia-smi');
      expect(resourceDetailMappersSource).not.toContain('powercap');
      expect(resourceDetailMappersSource).not.toContain('export const toDiscoveryConfig');
      expect(resourceDetailDiscoveryModelSource).toContain('export const toDiscoveryConfig');
    });
  });

  describe('toNodeFromProxmox', () => {
    it('preserves canonical linkedAgentId for hybrid hosts', () => {
      const node = toNodeFromProxmox(createHybridHostResource());

      expect(node?.linkedAgentId).toBe('agent-canonical');
    });
  });

  describe('toAgentFromResource', () => {
    it('uses the canonical actionable agent id instead of the hashed resource id', () => {
      const agent = toAgentFromResource(createHybridHostResource());

      expect(agent?.id).toBe('agent-canonical');
      expect(agent?.id).not.toBe('resource:host:hash-1');
    });

    it('reports no uptime for a silent agent, whose last report is not how long it has been up', () => {
      const base = createHybridHostResource();
      const reporting = { ...base, uptime: 2_500_000 } as Resource;
      expect(toAgentFromResource(reporting)?.uptimeSeconds).toBe(2_500_000);

      const silent = { ...reporting, agent: { stale: true } } as Resource;
      expect(toAgentFromResource(silent)?.uptimeSeconds).toBe(0);
    });

    it('preserves the local infrastructure display name for governed resources', () => {
      const agent = toAgentFromResource({
        ...createHybridHostResource(),
        name: 'secret-host',
        displayName: 'Tower',
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname', 'alias'] },
        },
        aiSafeSummary: 'restricted host summary safe for remote AI consumption',
      });

      expect(agent?.displayName).toBe('Tower');
    });
  });
});
