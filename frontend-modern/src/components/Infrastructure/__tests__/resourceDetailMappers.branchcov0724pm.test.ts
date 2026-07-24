import { describe, expect, it } from 'vitest';
import {
  buildTemperatureRows,
  toAgentFromResource,
  toNodeFromProxmox,
  type AgentPlatformData,
} from '@/components/Infrastructure/resourceDetailMappers';
import type { HostSensorSummary } from '@/types/api';
import type { Resource } from '@/types/resource';

/**
 * Branch-coverage additions for resourceDetailMappers.ts targeting the arms
 * left uncovered by the existing happy-path and prior branchcov suites.
 *
 * Every assertion is made against observable return values of the exported
 * entry points (toAgentFromResource, toNodeFromProxmox, buildTemperatureRows).
 * The module-private helpers they delegate to (getPreferredHostLabel,
 * getPreferredResourceHostname, getPreferredInfrastructureDisplayName,
 * formatPowerSensorLabel, formatFanSensorLabel, formatGPUStatsValue,
 * formatFanRPM) are driven exclusively through those exports and never
 * re-implemented or imported directly.
 */

const baseResource = (overrides: Partial<Resource> = {}): Resource =>
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

describe('toAgentFromResource — uncovered arms', () => {
  it('returns null when neither an explicit agent nor platformData.agent exists', () => {
    // Drives the `if (!agent) return null` consequent (branch at line 289).
    const resource = baseResource({
      platformData: { proxmox: { nodeName: 'pve-node-1' } },
    });

    expect(toAgentFromResource(resource)).toBeNull();
  });

  it('returns null when platformData is absent entirely', () => {
    expect(toAgentFromResource(baseResource({ platformData: undefined }))).toBeNull();
  });

  it('reads proxmox.cpuInfo.cores when cpuInfo is present (non-short-circuit optional chain)', () => {
    // Drives the `cpuInfo?.cores` access arm (branch at line 291) by giving
    // proxmox a populated cpuInfo, and confirms the value flows through the
    // cpuCount `.find` predicate (`value > 0` evaluates true).
    const resource = baseResource({
      platformData: {
        proxmox: { cpuInfo: { cores: 4 } },
        agent: { agentId: 'agent-canonical' },
      },
    });

    expect(toAgentFromResource(resource)?.cpuCount).toBe(4);
  });

  it('treats a non-positive cpuCount as not-usable and falls through to undefined', () => {
    // cpuCount === 0 is a number so `typeof value === 'number'` is true, but
    // `value > 0` is false — drives the false arm of the `.find` predicate's
    // `&&` (branch at line 297). With no other positive candidate, cpuCount
    // resolves to undefined.
    const resource = baseResource({
      platformData: {
        proxmox: {},
        agent: { agentId: 'agent-canonical', cpuCount: 0 },
      },
    });

    expect(toAgentFromResource(resource)?.cpuCount).toBeUndefined();
  });

  it('accepts a positive cpuCount as the first usable candidate', () => {
    const resource = baseResource({
      platformData: {
        proxmox: { cpuInfo: { cores: 99 } },
        agent: { agentId: 'agent-canonical', cpuCount: 8 },
      },
    });

    // First array element wins over the proxmox fallback.
    expect(toAgentFromResource(resource)?.cpuCount).toBe(8);
  });

  it('falls back to getPreferredHostLabel when agent.hostname is absent', () => {
    // Drives the `agent.hostname ?? getPreferredHostLabel(resource)` RHS
    // (branch at line 299). resource.name remains set so the host label
    // resolves to "tower" via getPreferredResourceHostname.
    const resource = baseResource({
      platformData: {
        proxmox: {},
        agent: { agentId: 'agent-canonical' },
      },
    });

    expect(toAgentFromResource(resource)?.hostname).toBe('tower');
  });

  it('falls through hostname and displayName to resource.id when no identity source resolves', () => {
    // Drives BOTH `||` short-circuit RHS arms in getPreferredHostLabel
    // (branches at lines 188 and 189): no hostname, no displayName, no name,
    // no platformId, no identity — so getPreferredResourceHostname is falsy
    // (line 188 RHS evaluated) and getPreferredInfrastructureDisplayName also
    // bottoms out at the empty resource.id (line 189 RHS evaluated).
    //
    // NOTE: the line-189 arm is only reachable when resource.id is empty
    // (getPreferredInfrastructureDisplayName ultimately returns resource.id),
    // i.e. a degenerate identity-less resource. It is reported here, not faked.
    const resource = {
      id: '',
      type: 'agent',
      status: 'online',
      lastSeen: 1_700_000_000_000,
      platformData: { proxmox: {} },
    } as unknown as Resource;

    const node = toNodeFromProxmox(resource);
    expect(node?.name).toBe('');
    expect(node?.host).toBe('');
  });

  it('uses resource.id for the agent id when no actionable agent id resolves', () => {
    // Drives the `getActionableAgentIdFromResource(resource) || resource.id`
    // RHS (branch at line 300). The resource carries no agent id in any form
    // and no discoveryTarget.
    const resource = baseResource({
      platformData: { proxmox: { nodeName: 'pve-node-1' } },
    });
    const explicitAgent = { hostname: 'h' } as AgentPlatformData;

    expect(toAgentFromResource(resource, explicitAgent)?.id).toBe('resource:host:hash-1');
  });

  it('defaults osName to "Unknown" when the agent omits it', () => {
    const resource = baseResource();
    const explicitAgent = {} as AgentPlatformData;

    expect(toAgentFromResource(resource, explicitAgent)?.osName).toBe('Unknown');
  });

  it('defaults kernelVersion to "Unknown" when the agent omits it', () => {
    const resource = baseResource();
    const explicitAgent = {} as AgentPlatformData;

    expect(toAgentFromResource(resource, explicitAgent)?.kernelVersion).toBe('Unknown');
  });

  it('resolves getPreferredHostLabel via resource.displayName when every hostname source is empty', () => {
    // Drives the line-188 `||` RHS (getPreferredResourceHostname falsy) while
    // keeping getPreferredInfrastructureDisplayName truthy via displayName, so
    // the host label resolves to the displayName without touching resource.id.
    const resource = baseResource({
      name: undefined,
      platformId: undefined,
      identity: undefined,
      displayName: 'Display Only',
      platformData: { proxmox: {} },
    });
    const explicitAgent = {} as AgentPlatformData;

    expect(toAgentFromResource(resource, explicitAgent)?.hostname).toBe('Display Only');
  });
});

describe('buildTemperatureRows — uncovered sensor-label arms', () => {
  const findRow = (sensors: HostSensorSummary, label: string) =>
    buildTemperatureRows(sensors).find((r) => r.label === label);

  it('does not append " Power" when the sensor name already carries a power keyword', () => {
    // Drives the consequent of the power-keyword ternary in
    // formatPowerSensorLabel (branch at line 342). "cpu_power" title-cases to
    // "CPU Power", which already matches \b(power)\b, so the label is returned
    // verbatim instead of becoming "CPU Power Power".
    expect(findRow({ powerWatts: { cpu_power: 82.4 } }, 'CPU Power')?.value).toBe('82.4 W');
    expect(
      buildTemperatureRows({ powerWatts: { cpu_power: 82.4 } }).some(
        (r) => r.label === 'CPU Power Power',
      ),
    ).toBe(false);
  });

  it('does not append " Power" for a "watts" keyword either', () => {
    expect(findRow({ powerWatts: { psu_watts: 1500 } }, 'Psu Watts')?.value).toBe('1,500 W');
  });

  it('appends " Fan" when the fan sensor name lacks the fan keyword', () => {
    // Drives the alternate of the fan-keyword ternary in formatFanSensorLabel
    // (branch at line 347). "blower" has no \bfan\b match, so the suffix is
    // added, yielding "Blower Fan".
    expect(findRow({ fanRpm: { blower: 1000 } }, 'Blower Fan')?.value).toBe('1,000 RPM');
  });

  it('defaults GPU memoryUsedBytes to 0 when it is absent while memoryTotalBytes is valid', () => {
    // Drives the alternate of the `used` ternary inside formatGPUStatsValue
    // (branch at line 371): memoryTotalBytes is a positive finite number but
    // memoryUsedBytes is undefined, so `used` falls back to 0 and is formatted
    // as "0 B".
    const rows = buildTemperatureRows({
      gpu: [
        {
          id: '0',
          name: 'Card',
          memoryTotalBytes: 1024 * 1024 * 1024,
        },
      ],
    } as HostSensorSummary);

    const gpuRow = rows.find((r) => r.label === 'GPU 0');
    expect(gpuRow?.value).toBe('Card · 0 B / 1.00 GB');
  });

  it('defaults GPU memoryUsedBytes to 0 when it is a non-finite value', () => {
    const rows = buildTemperatureRows({
      gpu: [
        {
          id: '0',
          name: 'Card',
          memoryTotalBytes: 1024 * 1024 * 1024,
          memoryUsedBytes: Number.NaN,
        },
      ],
    } as HostSensorSummary);

    expect(rows.find((r) => r.label === 'GPU 0')?.value).toBe('Card · 0 B / 1.00 GB');
  });

  it('drops a fan row whose RPM is non-finite (formatFanRPM returns empty and is filtered)', () => {
    // Drives the `if (!Number.isFinite(value)) return ''` consequent in
    // formatFanRPM (branch at line 385); the empty value is then removed by
    // the downstream `.filter(([, value]) => value)`.
    expect(buildTemperatureRows({ fanRpm: { dead: Number.NaN } })).toEqual([]);
    expect(buildTemperatureRows({ fanRpm: { dead: Number.POSITIVE_INFINITY } })).toEqual([]);
  });
});
