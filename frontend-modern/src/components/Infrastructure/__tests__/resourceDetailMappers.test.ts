import { describe, expect, it } from 'vitest';
import {
  buildTemperatureRows,
  formatInteger,
  formatSensorName,
  formatSourceType,
  toAgentFromResource,
  toNodeFromProxmox,
} from '@/components/Infrastructure/resourceDetailMappers';
import type { Resource } from '@/types/resource';

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
