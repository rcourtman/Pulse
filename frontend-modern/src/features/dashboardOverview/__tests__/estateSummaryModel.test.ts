import { describe, expect, it } from 'vitest';
import type { ConnectedInfrastructureItem } from '@/types/api';
import { buildDashboardEstateSummary } from '../estateSummaryModel';

const item = (
  overrides: Partial<ConnectedInfrastructureItem> = {},
): ConnectedInfrastructureItem => ({
  id: 'host-1',
  name: 'host-1',
  status: 'active',
  healthStatus: 'online',
  lastSeen: Date.parse('2026-04-23T10:00:00.000Z'),
  surfaces: [{ id: 'agent:host-1', kind: 'agent', label: 'Host telemetry' }],
  ...overrides,
});

describe('buildDashboardEstateSummary', () => {
  it('summarizes connected infrastructure as monitored systems, not platform tabs', () => {
    const summary = buildDashboardEstateSummary([
      item({
        id: 'pve-1',
        name: 'pve-1',
        surfaces: [
          { id: 'agent:pve-1', kind: 'agent', label: 'Host telemetry' },
          { id: 'proxmox:pve-1', kind: 'proxmox', label: 'Proxmox VE data' },
        ],
      }),
      item({
        id: 'nas-1',
        name: 'nas-1',
        healthStatus: 'degraded',
        surfaces: [{ id: 'truenas:nas-1', kind: 'truenas', label: 'TrueNAS API data' }],
      }),
      item({
        id: 'k8s-1',
        name: 'k8s-1',
        surfaces: [
          { id: 'kubernetes:k8s-1', kind: 'kubernetes', label: 'Kubernetes cluster data' },
        ],
      }),
    ]);

    expect(summary.totalSystems).toBe(3);
    expect(summary.activeSystems).toBe(3);
    expect(summary.healthySystems).toBe(2);
    expect(summary.degradedSystems).toBe(1);
    expect(summary.headline).toBe('1 system needs attention');
    expect(summary.surfaces.map((surface) => surface.label)).toEqual([
      'Agent',
      'Kubernetes',
      'Proxmox',
      'TrueNAS',
    ]);
  });

  it('uses the compact dashboard fallback when the connected projection has not arrived yet', () => {
    const summary = buildDashboardEstateSummary([], {
      total: 4,
      online: 3,
    });

    expect(summary.hasCanonicalProjection).toBe(false);
    expect(summary.totalSystems).toBe(4);
    expect(summary.healthySystems).toBe(3);
    expect(summary.unknownSystems).toBe(1);
    expect(summary.headline).toBe('4 resources reporting');
    expect(summary.detail).toBe('3 resources online while the system map syncs');
  });

  it('counts each active system needing attention once', () => {
    const summary = buildDashboardEstateSummary([
      item({ id: 'pve-1', healthStatus: 'degraded', isOutdatedBinary: true }),
      item({ id: 'ignored-1', status: 'ignored', isOutdatedBinary: true }),
    ]);

    expect(summary.totalSystems).toBe(2);
    expect(summary.degradedSystems).toBe(1);
    expect(summary.outdatedSystems).toBe(1);
    expect(summary.ignoredSystems).toBe(1);
    expect(summary.attentionSystems).toBe(1);
    expect(summary.headline).toBe('1 system needs attention');
  });
});
