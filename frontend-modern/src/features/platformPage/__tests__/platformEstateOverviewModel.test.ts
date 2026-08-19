import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildPlatformEstateMetrics,
  buildPlatformOperationalSpotlights,
  deserializePlatformEstateOverviewVisibility,
  formatPlatformEstateMetricValue,
  isPlatformEstateAttentionResource,
  type PlatformEstateOverviewPlatform,
} from '../platformEstateOverviewModel';

const makeResource = (
  id: string,
  type: Resource['type'],
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id,
    type,
    name: id,
    displayName: id,
    platformId: 'estate',
    platformType: 'generic',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('platformEstateOverviewModel', () => {
  it('derives a large Proxmox estate from the canonical scoped resource set', () => {
    const resources = [
      makeResource('pve-a', 'agent', { proxmox: { clusterName: 'production' } }),
      makeResource('pve-b', 'agent', { proxmox: { clusterName: 'production' } }),
      makeResource('pve-lab', 'agent'),
      makeResource('vm-101', 'vm', { status: 'running' }),
      makeResource('ct-201', 'system-container', { status: 'stopped' }),
      makeResource('oci-301', 'oci-container', { status: 'warning' }),
    ];

    expect(buildPlatformEstateMetrics('proxmox', resources)).toEqual([
      { id: 'nodes', label: 'Proxmox nodes', value: 3 },
      { id: 'workloads', label: 'VMs and containers', value: 3 },
      { id: 'topology', label: 'Clusters + standalone', value: '1 + 1' },
      { id: 'attention', label: 'Need attention', value: 1 },
    ]);
  });

  it.each<PlatformEstateOverviewPlatform>([
    'proxmox',
    'docker',
    'kubernetes',
    'truenas',
    'vmware',
    'standalone',
  ])('keeps %s on the same four-metric overview contract', (platform) => {
    const metrics = buildPlatformEstateMetrics(platform, []);

    expect(metrics).toHaveLength(4);
    expect(metrics.map((item) => item.id)).toContain('attention');
  });

  it('counts actionable evidence without treating an intentionally stopped workload as a fault', () => {
    expect(
      isPlatformEstateAttentionResource(makeResource('stopped', 'vm', { status: 'stopped' })),
    ).toBe(false);
    expect(
      isPlatformEstateAttentionResource(
        makeResource('pressure', 'storage', { disk: { current: 91 } }),
      ),
    ).toBe(true);
    expect(
      isPlatformEstateAttentionResource(
        makeResource('alerting', 'agent', {
          alerts: [
            {
              id: 'alert-1',
              type: 'temperature',
              level: 'critical',
              message: 'Temperature threshold exceeded',
              value: 95,
              threshold: 90,
              startTime: 1_700_000_000_000,
            },
          ],
        }),
      ),
    ).toBe(true);
  });

  it('prioritizes danger, caps the list, and routes spotlights to platform detail areas', () => {
    const resources = [
      makeResource('warning-pod', 'pod', { status: 'offline' }),
      makeResource('offline-node', 'k8s-node', { status: 'offline' }),
      makeResource('degraded-claim', 'k8s-persistent-volume-claim', { status: 'degraded' }),
      makeResource('warning-event', 'k8s-event', { status: 'warning' }),
    ];

    const spotlights = buildPlatformOperationalSpotlights('kubernetes', resources, 3);

    expect(spotlights).toHaveLength(3);
    expect(spotlights[0]).toMatchObject({
      resourceId: 'offline-node',
      tone: 'danger',
      href: '/kubernetes/nodes',
    });
    expect(spotlights.map((item) => item.href)).toContain('/kubernetes/storage');
    expect(spotlights.map((item) => item.href)).toContain('/kubernetes/workloads');
  });

  it('uses one tolerant global visibility preference', () => {
    expect(deserializePlatformEstateOverviewVisibility('false')).toBe(false);
    expect(deserializePlatformEstateOverviewVisibility('true')).toBe(true);
    expect(deserializePlatformEstateOverviewVisibility('legacy-value')).toBe(true);
  });

  it('keeps a very large estate projection bounded to four metrics and three spotlights', () => {
    const resources = Array.from({ length: 10_000 }, (_, index) =>
      makeResource(`node-${index}`, 'agent', {
        status: index % 5 === 0 ? 'offline' : 'online',
        proxmox: { clusterName: `cluster-${index % 40}` },
      }),
    );

    expect(buildPlatformEstateMetrics('proxmox', resources)).toHaveLength(4);
    expect(buildPlatformOperationalSpotlights('proxmox', resources)).toHaveLength(3);
    expect(formatPlatformEstateMetricValue(resources.length).length).toBeGreaterThan(5);
  });
});
