import { describe, expect, it } from 'vitest';
import {
  buildDisk,
  buildMemory,
  buildTemperatureRows,
  toAgentDisks,
  toNodeFromProxmox,
  type AgentDiskInfo,
  type PlatformData,
} from '@/components/Infrastructure/resourceDetailMappers';
import type { Disk, HostSensorSummary, Memory as MemoryType } from '@/types/api';
import type { Resource, ResourceMetric } from '@/types/resource';

/**
 * Branch-coverage suite for the currently-uncovered functions in
 * resourceDetailMappers.ts. Several targets (asString, getPreferredHostLabel,
 * formatGPUStatsLabel, formatPowerWatts, buildTypedGPUTemperatureKeys) are
 * module-private, so they are driven exclusively through the exported entry
 * points (toNodeFromProxmox, buildTemperatureRows) and asserted on observable
 * return values — never re-implemented or imported directly.
 */

const baseProxmoxResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'resource:host:hash-1',
    type: 'agent',
    name: 'tower',
    displayName: 'Tower',
    platformId: 'tower',
    platformType: 'proxmox-pve',
    sourceType: 'hybrid',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    cpu: { current: 12 },
    memory: { current: 0.25, total: 1024, used: 256, free: 768 },
    disk: { current: 0.25, total: 2048, used: 512, free: 1536 },
    platformData: {
      proxmox: { nodeName: 'pve-node-1' },
      agent: { agentId: 'agent-canonical' },
    },
    ...overrides,
  }) as unknown as Resource;

describe('buildMemory', () => {
  it('prefers metric values over fallback values', () => {
    const metric: ResourceMetric = { current: 0.5, total: 100, used: 50, free: 50 };
    const fallback: Partial<MemoryType> = { total: 999, used: 1, free: 2, usage: 0.9 };

    expect(buildMemory(metric, fallback)).toEqual({ total: 100, used: 50, free: 50, usage: 0.5 });
  });

  it('falls back to the fallback memory when no metric is given', () => {
    const fallback: Partial<MemoryType> = { total: 200, used: 80, free: 120, usage: 0.4 };

    expect(buildMemory(undefined, fallback)).toEqual({
      total: 200,
      used: 80,
      free: 120,
      usage: 0.4,
    });
  });

  it('computes usage from total>0 and ignores fallback.usage', () => {
    const fallback: Partial<MemoryType> = { usage: 0.99 };

    expect(buildMemory({ current: 0, total: 100, used: 25 }, fallback)).toEqual({
      total: 100,
      used: 25,
      free: 75,
      usage: 0.25,
    });
  });

  it('returns fallback.usage when total is 0 and metric is absent', () => {
    const fallback: Partial<MemoryType> = { usage: 0.42 };

    expect(buildMemory(undefined, fallback).usage).toBe(0.42);
  });

  it('defaults every field to 0 with no metric and no fallback', () => {
    expect(buildMemory()).toEqual({ total: 0, used: 0, free: 0, usage: 0 });
  });

  it('computes free as max(total - used, 0) when free is missing everywhere', () => {
    expect(buildMemory({ current: 0.3, total: 100, used: 30 }).free).toBe(70);
  });

  it('clamps free to 0 when used exceeds total and free is missing', () => {
    expect(buildMemory({ current: 1, total: 10, used: 30 }).free).toBe(0);
  });

  it('uses metric.free over computed free even when fallback.free exists', () => {
    expect(
      buildMemory({ current: 0, total: 100, used: 10, free: 5 }, { free: 90 }).free,
    ).toBe(5);
  });
});

describe('buildDisk', () => {
  it('prefers metric values and carries fallback identity fields', () => {
    const metric: ResourceMetric = { current: 0.5, total: 100, used: 50, free: 50 };
    const fallback: Partial<Disk> = {
      total: 9,
      used: 9,
      free: 9,
      mountpoint: '/',
      type: 'ext4',
      device: 'sda1',
    };

    expect(buildDisk(metric, fallback)).toEqual({
      total: 100,
      used: 50,
      free: 50,
      usage: 0.5,
      mountpoint: '/',
      type: 'ext4',
      device: 'sda1',
    });
  });

  it('falls back to fallback disk when no metric is given', () => {
    const fallback: Partial<Disk> = { total: 200, used: 80, free: 120, usage: 0.4 };

    expect(buildDisk(undefined, fallback)).toEqual({
      total: 200,
      used: 80,
      free: 120,
      usage: 0.4,
      mountpoint: undefined,
      type: undefined,
      device: undefined,
    });
  });

  it('computes usage from total>0 and ignores fallback.usage', () => {
    expect(buildDisk({ current: 0, total: 200, used: 50 }, { usage: 0.99 }).usage).toBe(0.25);
  });

  it('returns fallback.usage when total is 0 and metric is absent', () => {
    expect(buildDisk(undefined, { usage: 0.42 }).usage).toBe(0.42);
  });

  it('defaults every field to 0 with no metric and no fallback', () => {
    expect(buildDisk()).toEqual({
      total: 0,
      used: 0,
      free: 0,
      usage: 0,
      mountpoint: undefined,
      type: undefined,
      device: undefined,
    });
  });

  it('computes free as max(total - used, 0) when free is missing everywhere', () => {
    expect(buildDisk({ current: 0.3, total: 100, used: 30 }).free).toBe(70);
  });

  it('clamps free to 0 when used exceeds total and free is missing', () => {
    expect(buildDisk({ current: 1, total: 10, used: 30 }).free).toBe(0);
  });
});

describe('toAgentDisks', () => {
  it('returns undefined for undefined input', () => {
    expect(toAgentDisks(undefined)).toBeUndefined();
  });

  it('returns undefined for an empty array', () => {
    expect(toAgentDisks([])).toBeUndefined();
  });

  it('returns undefined for null input (defensive guard)', () => {
    expect(toAgentDisks(null as unknown as AgentDiskInfo[])).toBeUndefined();
  });

  it('maps a fully-populated disk', () => {
    const disks: AgentDiskInfo[] = [
      {
        device: 'sda1',
        mountpoint: '/',
        filesystem: 'ext4',
        type: 'ssd',
        total: 100,
        used: 25,
        free: 75,
      },
    ];

    expect(toAgentDisks(disks)).toEqual([
      {
        total: 100,
        used: 25,
        free: 75,
        usage: 0.25,
        mountpoint: '/',
        type: 'ext4',
        device: 'sda1',
      },
    ]);
  });

  it('computes usage = used/total when total > 0', () => {
    expect(toAgentDisks([{ total: 8, used: 2 }])?.[0].usage).toBe(0.25);
  });

  it('reports usage 0 when total is 0', () => {
    expect(toAgentDisks([{ total: 0, used: 5 }])?.[0].usage).toBe(0);
  });

  it('defaults total/used to 0 when missing', () => {
    expect(toAgentDisks([{}])?.[0]).toEqual({
      total: 0,
      used: 0,
      free: 0,
      usage: 0,
      mountpoint: undefined,
      type: undefined,
      device: undefined,
    });
  });

  it('computes free as max(total - used, 0) when free is missing', () => {
    expect(toAgentDisks([{ total: 100, used: 30 }])?.[0].free).toBe(70);
    expect(toAgentDisks([{ total: 10, used: 30 }])?.[0].free).toBe(0);
  });

  it('falls back to device for mountpoint when mountpoint is absent', () => {
    expect(toAgentDisks([{ device: 'nvme0n1' }])?.[0].mountpoint).toBe('nvme0n1');
  });

  it('prefers filesystem over type for the type field', () => {
    expect(toAgentDisks([{ filesystem: 'zfs', type: 'hdd' }])?.[0].type).toBe('zfs');
  });

  it('uses type when filesystem is absent', () => {
    expect(toAgentDisks([{ type: 'nvme' }])?.[0].type).toBe('nvme');
  });

  it('preserves order across multiple disks', () => {
    const result = toAgentDisks([{ device: 'a' }, { device: 'b' }]);
    expect(result?.map((d) => d.device)).toEqual(['a', 'b']);
  });
});

describe('toNodeFromProxmox', () => {
  it('returns null when platformData has no proxmox block', () => {
    expect(
      toNodeFromProxmox({ ...baseProxmoxResource(), platformData: { agent: {} } }),
    ).toBeNull();
  });

  it('returns null when platformData is absent entirely', () => {
    expect(toNodeFromProxmox({ ...baseProxmoxResource(), platformData: undefined })).toBeNull();
  });

  it('maps core fields from a populated proxmox resource', () => {
    const node = toNodeFromProxmox(baseProxmoxResource());

    expect(node).not.toBeNull();
    expect(node?.id).toBe('resource:host:hash-1');
    expect(node?.name).toBe('pve-node-1');
    expect(node?.host).toBe('pve-node-1');
    expect(node?.instance).toBe('tower');
    expect(node?.status).toBe('online');
    expect(node?.cpu).toBe(12);
    expect(node?.memory).toEqual({ total: 1024, used: 256, free: 768, usage: 0.25 });
    expect(node?.disk).toEqual({
      total: 2048,
      used: 512,
      free: 1536,
      usage: 0.25,
      mountpoint: undefined,
      type: undefined,
      device: undefined,
    });
  });

  it('formats lastSeen as an ISO string of the finite timestamp', () => {
    const node = toNodeFromProxmox(baseProxmoxResource({ lastSeen: 1_700_000_000_000 }));
    expect(node?.lastSeen).toBe(new Date(1_700_000_000_000).toISOString());
  });

  it('falls back to "now" when lastSeen is not finite', () => {
    const before = Date.now();
    const node = toNodeFromProxmox(
      baseProxmoxResource({ lastSeen: Number.NaN }),
    );
    const after = Date.now();

    expect(node?.lastSeen).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/);
    const seenMs = Date.parse(node!.lastSeen);
    expect(seenMs).toBeGreaterThanOrEqual(before - 1);
    expect(seenMs).toBeLessThanOrEqual(after + 1);
  });

  it('prefers resource.uptime over proxmox.uptime', () => {
    const node = toNodeFromProxmox(
      baseProxmoxResource({
        uptime: 111,
        platformData: { proxmox: { nodeName: 'n', uptime: 222 } },
      }),
    );
    expect(node?.uptime).toBe(111);
  });

  it('uses proxmox.uptime when resource.uptime is absent', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      uptime: undefined,
      platformData: { proxmox: { nodeName: 'n', uptime: 222 } },
    });
    expect(node?.uptime).toBe(222);
  });

  it('defaults uptime to 0 when neither source provides it', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      uptime: undefined,
      platformData: { proxmox: { nodeName: 'n' } },
    });
    expect(node?.uptime).toBe(0);
  });

  it('defaults kernelVersion and pveVersion to "Unknown"', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      platformData: { proxmox: { nodeName: 'n' } },
    });
    expect(node?.kernelVersion).toBe('Unknown');
    expect(node?.pveVersion).toBe('Unknown');
  });

  it('defaults cpuInfo to Unknown/0 when cpuInfo is absent', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      platformData: { proxmox: { nodeName: 'n' } },
    });
    expect(node?.cpuInfo).toEqual({ model: 'Unknown', cores: 0, sockets: 0, mhz: '0' });
  });

  it('reads cpuInfo fields when present', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      platformData: {
        proxmox: { nodeName: 'n', cpuInfo: { model: 'Xeon', cores: 8, sockets: 2 } },
      },
    });
    expect(node?.cpuInfo).toEqual({ model: 'Xeon', cores: 8, sockets: 2, mhz: '0' });
  });

  it('defaults cpu to 0 when resource.cpu is absent', () => {
    const node = toNodeFromProxmox({ ...baseProxmoxResource(), cpu: undefined });
    expect(node?.cpu).toBe(0);
  });

  it('uses resource.id for instance when platformId is absent', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      platformId: undefined,
      platformData: { proxmox: { nodeName: 'n' } },
    } as unknown as Resource);
    expect(node?.instance).toBe('resource:host:hash-1');
  });

  it('reports connectionHealth from resource.status', () => {
    const node = toNodeFromProxmox(baseProxmoxResource({ status: 'offline' }));
    expect(node?.connectionHealth).toBe('offline');
  });

  it('falls back connectionHealth to "unknown" when status is undefined', () => {
    const node = toNodeFromProxmox({
      ...baseProxmoxResource(),
      status: undefined,
      platformData: { proxmox: { nodeName: 'n' } },
    } as unknown as Resource);
    expect(node?.connectionHealth).toBe('unknown');
  });

  describe('asString (via linkedAgentId resolution)', () => {
    it('trims and returns a non-empty platformData.linkedAgentId', () => {
      const platformData = {
        proxmox: { nodeName: 'n' },
        linkedAgentId: '  primary-link  ',
      } as unknown as PlatformData;
      const node = toNodeFromProxmox({ ...baseProxmoxResource(), platformData });
      expect(node?.linkedAgentId).toBe('primary-link');
    });

    it('returns undefined for an empty-string linkedAgentId and falls back', () => {
      const platformData = {
        proxmox: { nodeName: 'n' },
        agent: { agentId: 'agent-canonical' },
        linkedAgentId: '',
      } as unknown as PlatformData;
      const node = toNodeFromProxmox({ ...baseProxmoxResource(), platformData });
      expect(node?.linkedAgentId).toBe('agent-canonical');
    });

    it('returns undefined for a whitespace-only linkedAgentId and falls back', () => {
      const platformData = {
        proxmox: { nodeName: 'n' },
        agent: { agentId: 'agent-canonical' },
        linkedAgentId: '   ',
      } as unknown as PlatformData;
      const node = toNodeFromProxmox({ ...baseProxmoxResource(), platformData });
      expect(node?.linkedAgentId).toBe('agent-canonical');
    });

    it('returns undefined for a non-string linkedAgentId and falls back', () => {
      const platformData = {
        proxmox: { nodeName: 'n' },
        agent: { agentId: 'agent-canonical' },
        linkedAgentId: 12345,
      } as unknown as PlatformData;
      const node = toNodeFromProxmox({ ...baseProxmoxResource(), platformData });
      expect(node?.linkedAgentId).toBe('agent-canonical');
    });
  });

  describe('getPreferredHostLabel (via name/host when nodeName is absent)', () => {
    it('uses identity.hostname when nodeName is absent (hostname branch)', () => {
      const node = toNodeFromProxmox({
        ...baseProxmoxResource(),
        platformData: { proxmox: {} },
        identity: { hostname: 'real-host' } as Resource['identity'],
      });
      expect(node?.name).toBe('real-host');
      expect(node?.host).toBe('real-host');
    });

    it('falls back to resource.name when no hostname source resolves', () => {
      const node = toNodeFromProxmox({
        ...baseProxmoxResource({ name: 'fallback-name' }),
        platformData: { proxmox: {} },
      });
      expect(node?.name).toBe('fallback-name');
    });
  });
});

describe('buildTypedGPUTemperatureKeys (observed via buildTemperatureRows)', () => {
  it('excludes a typed gpu_nvidia_<id> temp entry when the GPU has a finite temperature', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '0', temperatureCelsius: 63 }],
      temperatureCelsius: { gpu_nvidia_0: 63, cpu_package: 41 },
    });

    const labels = rows.map((r) => r.label);
    expect(labels).not.toContain('Nvidia 0'); // gpu_nvidia_0 excluded from sensor temps
    expect(labels).toContain('Package');
  });

  it('keeps a gpu_nvidia_<id> temp entry when the GPU lacks a finite temperature', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '0' }],
      temperatureCelsius: { gpu_nvidia_0: 70 },
    });

    // No GPU stats row (nothing to render) but the raw temp row remains.
    expect(rows.map((r) => r.label)).toContain('Nvidia 0');
    expect(rows[0].value).toBe('70°C');
  });

  it('keeps a gpu_nvidia_<id> temp entry when GPU temperature is non-finite', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '0', temperatureCelsius: Number.NaN }],
      temperatureCelsius: { gpu_nvidia_0: 70 },
    });

    expect(rows.map((r) => r.label)).toContain('Nvidia 0');
  });

  it('does not register a typed key for a GPU with an empty id', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '', temperatureCelsius: 50 }],
      temperatureCelsius: { gpu_nvidia_0: 70 },
    });

    // Empty id => no typed key => the temp entry survives.
    expect(rows.map((r) => r.label)).toContain('Nvidia 0');
  });

  it('does not register a typed key for a GPU with a whitespace-only id', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '   ', temperatureCelsius: 50 }],
      temperatureCelsius: { gpu_nvidia_1: 70 },
    });

    expect(rows.map((r) => r.label)).toContain('Nvidia 1');
  });
});

describe('formatGPUStatsLabel (observed via buildTemperatureRows GPU rows)', () => {
  it('renders "GPU <id>" when the GPU has a trimmed id', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '2', name: 'RTX 4090' }],
    });
    expect(rows.map((r) => r.label)).toContain('GPU 2');
  });

  it('falls back to "GPU <index+1>" when id is undefined', () => {
    const rows = buildTemperatureRows({
      gpu: [{ name: 'Integrated' }],
    });
    expect(rows.map((r) => r.label)).toContain('GPU 1');
  });

  it('falls back to "GPU <index+1>" when id is whitespace-only', () => {
    const rows = buildTemperatureRows({
      gpu: [{ id: '   ', name: 'Integrated' }, { id: '5', name: 'Discrete' }],
    });
    const labels = rows.map((r) => r.label);
    expect(labels).toEqual(['GPU 1', 'GPU 5']);
  });
});

describe('formatPowerWatts (observed via buildTemperatureRows power rows)', () => {
  const findPower = (sensors: HostSensorSummary, label: string) =>
    buildTemperatureRows(sensors).find((r) => r.label === label);

  it('formats a sub-100 value with one decimal', () => {
    expect(findPower({ powerWatts: { cpu: 82.4 } }, 'CPU Power')?.value).toBe('82.4 W');
  });

  it('formats a value >= 100 with locale grouping and no decimal', () => {
    expect(findPower({ powerWatts: { cpu: 1500 } }, 'CPU Power')?.value).toBe('1,500 W');
  });

  it('treats exactly 100 as the >= 100 branch', () => {
    expect(findPower({ powerWatts: { cpu: 100 } }, 'CPU Power')?.value).toBe('100 W');
  });

  it('drops the row for a non-finite value (empty string filters out)', () => {
    expect(findPower({ powerWatts: { cpu: Number.NaN } }, 'CPU Power')).toBeUndefined();
    expect(findPower({ powerWatts: { cpu: Number.POSITIVE_INFINITY } }, 'CPU Power')).toBeUndefined();
  });
});

describe('buildTemperatureRows (remaining uncovered branches)', () => {
  it('returns an empty array when sensors is undefined', () => {
    expect(buildTemperatureRows(undefined)).toEqual([]);
  });

  it('returns an empty array when sensors has no populated sections', () => {
    expect(buildTemperatureRows({})).toEqual([]);
  });

  it('omits the thermal row when thermalState exists but has no pressure', () => {
    const rows = buildTemperatureRows({ thermalState: { source: 'pmset' } });
    expect(rows).toEqual([]);
  });

  it('emits a thermal row whose valueTitle omits "via <source>" when source is absent', () => {
    const rows = buildTemperatureRows({ thermalState: { pressure: 'nominal' } });
    expect(rows).toEqual([
      { label: 'Thermal pressure', value: 'Nominal', valueTitle: 'Nominal' },
    ]);
  });

  it('filters out limitsPercent entries >= 100 and non-finite entries', () => {
    const rows = buildTemperatureRows({
      thermalState: {
        limitsPercent: { cpu_speed_limit: 100, scheduler_limit: Number.NaN, thermal_limit: 80 },
      },
    });
    expect(rows.map((r) => r.label)).toEqual(['Limit']);
  });

  it('skips a GPU row whose rendered value is empty', () => {
    const rows = buildTemperatureRows({ gpu: [{ id: '0' }] });
    expect(rows).toEqual([]);
  });

  it('filters out non-finite temperatureCelsius entries', () => {
    const rows = buildTemperatureRows({
      temperatureCelsius: { cpu_package: 41, gpu_die: Number.NaN },
    });
    expect(rows.map((r) => r.label)).toEqual(['Package']);
  });

  it('filters out non-finite additional entries', () => {
    const rows = buildTemperatureRows({
      additional: { vrm_temp: 55.6, psu_temp: Number.NaN },
    });
    expect(rows.map((r) => r.label)).toEqual(['VRM Temp']);
  });

  describe('smart disks', () => {
    it('renders a healthy, active disk row', () => {
      const rows = buildTemperatureRows({
        smart: [{ device: 'sda', temperature: 42 }],
      });
      expect(rows).toEqual([
        { label: 'Disk sda', value: '42°C', valueTitle: '42.0°C' },
      ]);
    });

    it('skips standby disks', () => {
      const rows = buildTemperatureRows({
        smart: [{ device: 'sdb', temperature: 42, standby: true }],
      });
      expect(rows).toEqual([]);
    });

    it('skips disks with a non-finite temperature', () => {
      const rows = buildTemperatureRows({
        smart: [{ device: 'sdc', temperature: Number.NaN }],
      });
      expect(rows).toEqual([]);
    });

    it('sorts multiple disks by device name', () => {
      const rows = buildTemperatureRows({
        smart: [
          { device: 'sdc', temperature: 30 },
          { device: 'sda', temperature: 40 },
          { device: 'sdb', temperature: 50 },
        ],
      });
      expect(rows.map((r) => r.label)).toEqual(['Disk sda', 'Disk sdb', 'Disk sdc']);
    });
  });
});
