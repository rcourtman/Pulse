import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildStandalonePageModel,
  buildStandalonePostureSummary,
  sortStandaloneResourcesByAttention,
} from '../standalonePageModel';

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'agent-1',
    name: overrides.name ?? overrides.id ?? 'agent-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'agent-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'agent-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('standalonePageModel', () => {
  it('summarizes normal, attention, unknown, and freshness posture from canonical status', () => {
    const summary = buildStandalonePostureSummary(
      [
        resource({ id: 'online', status: 'online', lastSeen: 1_700_000_000_000 }),
        resource({ id: 'warning', status: 'degraded', lastSeen: 1_700_000_010_000 }),
        resource({ id: 'offline', status: 'offline', lastSeen: 1_700_000_020_000 }),
        resource({ id: 'unknown', status: 'unknown', lastSeen: 1_700_000_005_000 }),
      ],
      1_700_000_030_000,
    );

    expect(summary).toEqual({
      attention: 3,
      critical: 1,
      latestUpdateAt: 1_700_000_020_000,
      normal: 1,
      total: 4,
      unknown: 0,
      warning: 2,
    });
  });

  it('treats an online agent that stopped reporting as attention', () => {
    const summary = buildStandalonePostureSummary(
      [resource({ id: 'stale-agent', status: 'online', lastSeen: 1_700_000_000_000 })],
      1_700_000_600_001,
    );

    expect(summary.attention).toBe(1);
    expect(summary.warning).toBe(1);
    expect(summary.normal).toBe(0);
  });

  it('orders attention before unknown and healthy resources', () => {
    const ordered = sortStandaloneResourcesByAttention(
      [
        resource({ id: 'healthy', displayName: 'Healthy', status: 'online' }),
        resource({ id: 'unknown-b', displayName: 'Unknown B', status: 'unknown' }),
        resource({ id: 'warning', displayName: 'Warning', status: 'degraded' }),
        resource({ id: 'offline', displayName: 'Offline', status: 'offline' }),
        resource({ id: 'unknown-a', displayName: 'Unknown A', status: 'unknown' }),
      ],
      1_700_000_030_000,
    );

    expect(ordered.map((item) => item.id)).toEqual([
      'offline',
      'unknown-a',
      'unknown-b',
      'warning',
      'healthy',
    ]);
  });

  it('projects standalone agent machine resources without admitting provider-owned host rows', () => {
    const model = buildStandalonePageModel([
      resource({ id: 'mac-mini', platformType: 'agent', type: 'agent', sources: ['agent'] }),
      resource({
        id: 'linux-docker-host',
        platformType: 'agent',
        platformScopes: ['agent', 'docker'],
        type: 'agent',
        sourceType: 'hybrid',
        sources: ['agent', 'docker'],
      }),
      resource({
        id: 'pve-node',
        platformType: 'proxmox-pve',
        type: 'agent',
        sourceType: 'hybrid',
        sources: ['proxmox', 'agent'],
      }),
      resource({
        id: 'esxi-host',
        platformType: 'vmware-vsphere',
        platformScopes: ['agent', 'vmware-vsphere'],
        type: 'agent',
        sourceType: 'api',
        sources: ['vmware'],
        agent: { agentId: 'vc-host-101', platform: 'vmware-vsphere' },
      }),
      resource({ id: 'k8s-pod', platformType: 'agent', type: 'pod', sources: ['agent'] }),
      resource({
        id: 'docker-host',
        platformType: 'docker',
        type: 'docker-host',
        sources: ['agent'],
      }),
    ]);

    expect(model.resources.map((item) => item.id)).toEqual(['mac-mini', 'linux-docker-host']);
  });

  it('keeps legacy source-less agent platform rows visible', () => {
    const model = buildStandalonePageModel([
      resource({ id: 'legacy-agent', platformType: 'agent', type: 'agent', sources: undefined }),
    ]);

    expect(model.resources.map((item) => item.id)).toEqual(['legacy-agent']);
    expect(model.machines.map((item) => item.id)).toEqual(['legacy-agent']);
    expect(model.availabilityChecks).toEqual([]);
  });

  it('keeps agentless availability checks out of standalone machines', () => {
    const model = buildStandalonePageModel([
      resource({ id: 'mac-mini', platformType: 'agent', type: 'agent', sources: ['agent'] }),
      resource({
        id: 'router-ping',
        platformType: 'availability',
        type: 'network-endpoint',
        sources: ['availability'],
        availability: { targetKind: 'machine' },
      }),
      resource({
        id: 'endpoint-1',
        platformType: 'availability',
        type: 'network-endpoint',
        sources: ['availability'],
        availability: { targetKind: 'service' },
      }),
    ]);

    expect(model.machines.map((item) => item.id)).toEqual(['mac-mini']);
    expect(model.availabilityChecks.map((item) => item.id)).toEqual(['router-ping', 'endpoint-1']);
    expect(model.resources.map((item) => item.id)).toEqual(['mac-mini']);
  });

  it('does not duplicate an attached resource in the availability-check inventory', () => {
    const model = buildStandalonePageModel([
      resource({
        id: 'agent:docker-trust',
        platformType: 'docker',
        type: 'agent',
        sources: ['agent', 'docker', 'availability'],
        availability: {
          targetId: 'tower-api',
          correlationState: 'attached',
          available: true,
        },
      }),
      resource({
        id: 'availability:orphan',
        platformType: 'availability',
        type: 'network-endpoint',
        sources: ['availability'],
        availability: {
          targetId: 'orphan',
          correlationState: 'standalone',
          available: true,
        },
      }),
    ]);

    expect(model.availabilityChecks.map((item) => item.id)).toEqual(['availability:orphan']);
  });

  it('treats stale or unobserved availability evidence as attention', () => {
    const now = Date.parse('2026-07-19T04:00:00Z');
    const summary = buildStandalonePostureSummary(
      [
        resource({
          id: 'availability:stale',
          platformType: 'availability',
          type: 'network-endpoint',
          availability: {
            targetId: 'stale',
            available: true,
            lastChecked: '2026-07-19T03:50:00Z',
            pollIntervalSeconds: 60,
          },
        }),
        resource({
          id: 'availability:unobserved',
          platformType: 'availability',
          type: 'network-endpoint',
          availability: { targetId: 'unobserved' },
        }),
      ],
      now,
    );

    expect(summary.attention).toBe(2);
    expect(summary.warning).toBe(2);
    expect(summary.normal).toBe(0);
  });
});
