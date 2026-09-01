import { describe, expect, it } from 'vitest';
import type { Resource, ResourceHealthVerdict } from '@/types/resource';
import {
  HOME_HEALTHY_GROUP_LIMIT,
  buildHomeAttentionTiles,
  buildHomePosture,
  buildHomeResourceGroups,
  getHomeResourceHref,
  getHomeVerdictTone,
} from '../homePageModel';

const resource = (
  id: string,
  verdict: ResourceHealthVerdict,
  platformType: Resource['platformType'] = 'proxmox-pve',
): Resource => ({
  id,
  type: 'vm',
  name: id,
  displayName: id,
  platformId: 'platform-1',
  platformType,
  sourceType: 'api',
  status: verdict === 'off' ? 'stopped' : 'online',
  lastSeen: Date.now(),
  health: { verdict, reasons: verdict === 'ok' ? [] : [{ code: `${verdict}_reason` }] },
});

describe('homePageModel', () => {
  it('counts every canonical verdict without treating powered-off workloads as failures', () => {
    const posture = buildHomePosture([
      resource('ok', 'ok'),
      resource('critical', 'critical'),
      resource('attention', 'attention'),
      resource('stale', 'stale'),
      resource('off', 'off'),
      resource('unknown', 'unknown'),
    ]);
    expect(posture).toMatchObject({
      total: 6,
      ok: 1,
      critical: 1,
      attention: 1,
      stale: 1,
      off: 1,
      unknown: 1,
      needsAttention: 2,
    });
  });

  it('puts critical resources before attention resources and sorts names stably', () => {
    const tiles = buildHomeAttentionTiles([
      resource('z-warning', 'attention'),
      resource('b-critical', 'critical'),
      resource('a-critical', 'critical'),
    ]);
    expect(tiles.map((tile) => tile.name)).toEqual(['a-critical', 'b-critical', 'z-warning']);
  });

  it('never hides stale or unknown resources behind the healthy group cap', () => {
    const resources = Array.from({ length: HOME_HEALTHY_GROUP_LIMIT + 5 }, (_, index) =>
      resource(`healthy-${index}`, 'ok'),
    );
    resources.push(resource('stale-first', 'stale'), resource('unknown-second', 'unknown'));
    const [group] = buildHomeResourceGroups(resources);
    expect(group?.tiles.slice(0, 2).map((tile) => tile.name)).toEqual([
      'stale-first',
      'unknown-second',
    ]);
    expect(group?.hiddenCount).toBe(5);
    expect(group?.hiddenTiles).toHaveLength(5);
  });

  it('groups resources in product navigation order and removes attention duplicates', () => {
    const groups = buildHomeResourceGroups([
      resource('vsphere', 'ok', 'vmware-vsphere'),
      resource('docker', 'ok', 'docker'),
      resource('urgent', 'critical', 'proxmox-pve'),
    ]);
    expect(groups.map((group) => group.key)).toEqual(['docker', 'vmware']);
  });

  it('uses the shared status tones for every canonical verdict', () => {
    expect(
      ['ok', 'attention', 'critical', 'stale', 'off', 'unknown'].map((verdict) =>
        getHomeVerdictTone(verdict as ResourceHealthVerdict),
      ),
    ).toEqual(['success', 'warning', 'danger', 'muted', 'muted', 'muted']);
  });

  it('links workloads to valid existing platform surfaces', () => {
    expect(getHomeResourceHref(resource('pve-vm', 'ok'))).toBe('/proxmox/overview?resource=pve-vm');
    expect(
      getHomeResourceHref({
        ...resource('docker-container', 'ok', 'docker'),
        type: 'app-container',
      }),
    ).toBe('/docker/overview?q=docker-container');
    expect(getHomeResourceHref(resource('vsphere-vm', 'ok', 'vmware-vsphere'))).toBe(
      '/vmware/overview?resource=vsphere-vm',
    );
  });

  it('keeps Home investigation links scoped to the resource workflow', () => {
    expect(
      getHomeResourceHref({
        ...resource('docker-container:api', 'attention', 'docker'),
        type: 'app-container',
        docker: { hostname: 'edge host' },
      }),
    ).toBe('/docker/overview?host=edge+host&q=docker-container%3Aapi');

    expect(
      getHomeResourceHref({
        ...resource('k8s:pod:api', 'critical', 'kubernetes'),
        type: 'pod',
        kubernetes: { clusterId: 'cluster-1', namespace: 'payments' },
      }),
    ).toBe('/kubernetes/workloads?cluster=cluster-1&namespace=payments&q=k8s%3Apod%3Aapi');

    expect(
      getHomeResourceHref({
        ...resource('truenas-disk', 'stale', 'truenas'),
        type: 'physical_disk',
      }),
    ).toBe('/truenas/storage');

    expect(
      getHomeResourceHref({
        ...resource('k8s:service:api', 'ok', 'generic'),
        type: 'k8s-service',
        sources: [],
      }),
    ).toBe('/kubernetes/services?q=k8s%3Aservice%3Aapi');

    expect(
      getHomeResourceHref({
        ...resource('machine:edge', 'ok', 'agent'),
        type: 'agent',
        sources: [],
      }),
    ).toBe('/standalone/machines?q=machine%3Aedge');
  });
});
