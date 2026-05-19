import { describe, expect, it } from 'vitest';
import { createRoot } from 'solid-js';
import type { Resource } from '@/types/resource';
import {
  createPlatformTableFilterState,
  filterPlatformResources,
  type PlatformResourceStatusFilter,
} from '../sharedPlatformPage';

const makeResource = (
  partial: Partial<Resource> & Pick<Resource, 'id' | 'type' | 'status'>,
): Resource => ({
  name: partial.id,
  displayName: partial.id,
  platformId: 'lab',
  platformType: 'docker',
  sourceType: 'agent',
  sources: ['docker'],
  lastSeen: 1_700_000_000_000,
  ...partial,
});

describe('filterPlatformResources', () => {
  const resources: Resource[] = [
    makeResource({ id: 'host-alpha', type: 'agent', status: 'online' }),
    makeResource({ id: 'host-bravo', type: 'agent', status: 'running' }),
    makeResource({ id: 'host-charlie', type: 'agent', status: 'degraded' }),
    makeResource({ id: 'host-delta', type: 'agent', status: 'offline' }),
    makeResource({ id: 'host-echo', type: 'agent', status: 'stopped' }),
    makeResource({ id: 'host-foxtrot', type: 'agent', status: 'paused' }),
    makeResource({
      id: 'host-with-tag',
      type: 'agent',
      status: 'online',
      tags: ['prod', 'gpu'],
    }),
  ];

  it('keeps all rows when no filters apply', () => {
    expect(filterPlatformResources(resources, '', 'all')).toHaveLength(resources.length);
  });

  it('collapses online/running into the online status chip', () => {
    const filtered = filterPlatformResources(resources, '', 'online');
    expect(filtered.map((r) => r.id).sort()).toEqual(
      ['host-alpha', 'host-bravo', 'host-with-tag'].sort(),
    );
  });

  it('collapses degraded/paused into the degraded chip', () => {
    const filtered = filterPlatformResources(resources, '', 'degraded');
    expect(filtered.map((r) => r.id).sort()).toEqual(['host-charlie', 'host-foxtrot'].sort());
  });

  it('collapses offline/stopped into the offline chip', () => {
    const filtered = filterPlatformResources(resources, '', 'offline');
    expect(filtered.map((r) => r.id).sort()).toEqual(['host-delta', 'host-echo'].sort());
  });

  it('searches against id, display name, parent, and tags case-insensitively', () => {
    expect(filterPlatformResources(resources, 'ALPHA', 'all').map((r) => r.id)).toEqual([
      'host-alpha',
    ]);
    expect(filterPlatformResources(resources, 'gpu', 'all').map((r) => r.id)).toEqual([
      'host-with-tag',
    ]);
  });

  it('searches platform-native metadata used by bespoke tables', () => {
    const nativeRows: Resource[] = [
      makeResource({
        id: 'docker-host',
        type: 'agent',
        status: 'online',
        docker: { runtimeVersion: '24.0.7', swarm: { nodeRole: 'manager' } },
      }),
      makeResource({
        id: 'k8s-deploy',
        type: 'k8s-deployment',
        status: 'online',
        kubernetes: {
          clusterName: 'prod-cluster',
          namespace: 'payments',
          containerRuntimeVersion: 'containerd://1.7',
        },
      }),
    ];

    expect(filterPlatformResources(nativeRows, 'manager', 'all').map((r) => r.id)).toEqual([
      'docker-host',
    ]);
    expect(filterPlatformResources(nativeRows, 'payments', 'all').map((r) => r.id)).toEqual([
      'k8s-deploy',
    ]);
  });

  it('combines search and status filters', () => {
    const filtered = filterPlatformResources(resources, 'host', 'degraded');
    expect(filtered.map((r) => r.id).sort()).toEqual(['host-charlie', 'host-foxtrot'].sort());
  });

  it('centralizes provider table filter state and row counts', () => {
    createRoot((dispose) => {
      try {
        const state = createPlatformTableFilterState({
          resources: () => resources,
          initialStatus: 'all' as PlatformResourceStatusFilter,
          filter: filterPlatformResources,
        });

        expect(state.total()).toBe(resources.length);
        expect(state.visible()).toBe(resources.length);

        state.setSearch('gpu');
        expect(state.filtered().map((r) => r.id)).toEqual(['host-with-tag']);
        expect(state.visible()).toBe(1);

        state.setSearch('host');
        state.setStatus('offline');
        expect(
          state
            .filtered()
            .map((r) => r.id)
            .sort(),
        ).toEqual(['host-delta', 'host-echo']);
      } finally {
        dispose();
      }
    });
  });
});
