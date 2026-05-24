import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  DOCKER_TAB_SPECS,
  buildDockerPageModel,
  getDockerHostSystemBadge,
  hasDockerSwarmEvidence,
  resolveDockerPageTabId,
} from '../dockerPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  ...resource,
  name: resource.name ?? resource.id,
  displayName: resource.displayName ?? resource.id,
  platformId: resource.platformId ?? 'lab',
  platformType: resource.platformType ?? 'docker',
  sourceType: resource.sourceType ?? 'agent',
  status: resource.status ?? 'online',
  lastSeen: resource.lastSeen ?? 1_700_000_000_000,
});

describe('dockerPageModel', () => {
  it('buckets Docker hosts, containers, images, volumes, networks, nodes, tasks, secrets, configs, and Swarm services from canonical resources', () => {
    const model = buildDockerPageModel([
      makeResource({ id: 'docker-host-1', type: 'agent' }),
      makeResource({
        id: 'ctr-1',
        type: 'app-container',
        platformType: 'docker',
      }),
      makeResource({ id: 'svc-1', type: 'docker-service' }),
      makeResource({ id: 'image-1', type: 'docker-image' }),
      makeResource({ id: 'volume-1', type: 'docker-volume' }),
      makeResource({ id: 'network-1', type: 'docker-network' }),
      makeResource({ id: 'node-1', type: 'docker-swarm-node' }),
      makeResource({ id: 'task-1', type: 'docker-task' }),
      makeResource({ id: 'secret-1', type: 'docker-secret' }),
      makeResource({ id: 'config-1', type: 'docker-config' }),
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        platformType: 'proxmox-pve',
      }),
    ]);

    expect(model.hosts.map((resource) => resource.id)).toEqual(['docker-host-1']);
    expect(model.containers.map((resource) => resource.id)).toEqual(['ctr-1']);
    expect(model.services.map((resource) => resource.id)).toEqual(['svc-1']);
    expect(model.images.map((resource) => resource.id)).toEqual(['image-1']);
    expect(model.volumes.map((resource) => resource.id)).toEqual(['volume-1']);
    expect(model.networks.map((resource) => resource.id)).toEqual(['network-1']);
    expect(model.nodes.map((resource) => resource.id)).toEqual(['node-1']);
    expect(model.tasks.map((resource) => resource.id)).toEqual(['task-1']);
    expect(model.secrets.map((resource) => resource.id)).toEqual(['secret-1']);
    expect(model.configs.map((resource) => resource.id)).toEqual(['config-1']);
    expect(model.resources.map((resource) => resource.id).sort()).toEqual(
      [
        'config-1',
        'ctr-1',
        'docker-host-1',
        'image-1',
        'network-1',
        'node-1',
        'secret-1',
        'svc-1',
        'task-1',
        'volume-1',
      ].sort(),
    );
  });

  it('declares operator workflow tabs for Docker runtime inventory', () => {
    expect(DOCKER_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'containers',
      'images',
      'storage',
      'networks',
      'swarm',
    ]);
  });

  it('keeps legacy Docker object routes mapped to workflow tabs', () => {
    expect(resolveDockerPageTabId(undefined)).toBe('overview');
    expect(resolveDockerPageTabId('containers')).toBe('containers');
    expect(resolveDockerPageTabId('volumes')).toBe('storage');
    expect(resolveDockerPageTabId('storage')).toBe('storage');
    expect(resolveDockerPageTabId('services')).toBe('swarm');
    expect(resolveDockerPageTabId('tasks')).toBe('swarm');
    expect(resolveDockerPageTabId('swarm-nodes')).toBe('swarm');
    expect(resolveDockerPageTabId('secrets')).toBe('swarm');
    expect(resolveDockerPageTabId('configs')).toBe('swarm');
    expect(resolveDockerPageTabId('unknown')).toBe('overview');
  });

  it('excludes non-Docker hosts that share the agent type', () => {
    const model = buildDockerPageModel([
      makeResource({
        id: 'truenas-host',
        type: 'agent',
        platformType: 'truenas',
      }),
    ]);
    expect(model.hosts).toEqual([]);
    expect(model.resources).toEqual([]);
  });

  it('surfaces host-identity badges for Docker hosts', () => {
    const lxcHost = makeResource({
      id: 'proxmox-lxc-docker:pve:101',
      type: 'agent',
      name: 'frigate',
      displayName: 'Frigate host',
      docker: { hostSourceId: 'proxmox-lxc-docker:pve:101' },
    });
    const genericDockerHost = makeResource({
      id: 'docker-host-1',
      type: 'agent',
      name: 'plain-docker',
    });

    expect(getDockerHostSystemBadge(lxcHost)?.label).toBe('LXC');
    expect(getDockerHostSystemBadge(genericDockerHost)).toBeUndefined();
  });

  it('surfaces the host OS family as the system badge for plain Docker hosts', () => {
    const debianHost = makeResource({
      id: 'docker-host-debian',
      type: 'agent',
      name: 'edge-apps-01',
      docker: { os: 'Debian GNU/Linux 12 (bookworm)', kernelVersion: '6.8.12-1-amd64' },
    });
    const alpineHost = makeResource({
      id: 'docker-host-alpine',
      type: 'agent',
      name: 'edge-apps-02',
      docker: { os: 'Alpine Linux 3.19' },
    });
    const ubuntuHost = makeResource({
      id: 'docker-host-ubuntu',
      type: 'agent',
      name: 'ops-services-01',
      docker: { os: 'Ubuntu 24.04.1 LTS' },
    });

    expect(getDockerHostSystemBadge(debianHost)?.label).toBe('Debian');
    expect(getDockerHostSystemBadge(debianHost)?.title).toBe('Debian GNU/Linux 12 (bookworm)');
    expect(getDockerHostSystemBadge(alpineHost)?.label).toBe('Alpine');
    expect(getDockerHostSystemBadge(ubuntuHost)?.label).toBe('Ubuntu');
  });

  it('does not treat standalone inactive Docker Swarm metadata as Swarm evidence', () => {
    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'docker-host-1',
          type: 'agent',
          docker: {
            swarm: {
              nodeRole: 'worker',
              localState: 'inactive',
              scope: 'node',
            },
          },
        }),
      ),
    ).toBe(false);

    expect(
      hasDockerSwarmEvidence(
        makeResource({
          id: 'docker-host-1',
          type: 'agent',
          docker: {
            swarm: {
              nodeId: 'node-1',
              nodeRole: 'manager',
              localState: 'active',
            },
          },
        }),
      ),
    ).toBe(true);
  });
});
