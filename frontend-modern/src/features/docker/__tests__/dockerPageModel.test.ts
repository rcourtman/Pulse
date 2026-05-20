import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  buildDockerContainerDefaultHiddenColumnIds,
  buildDockerPageModel,
  buildDockerWorkloadGroupLabelBadges,
  filterDockerHosts,
  filterDockerServices,
  getDockerHostSystemBadge,
  hasDockerSwarmEvidence,
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

const makeDockerHost = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'agent:docker-01',
  name: 'docker-01',
  displayName: 'Docker 01',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  type: 'agent',
  lastSeen: 1_700_000_000_000,
  docker: {
    runtime: 'docker',
    hostname: 'docker-01',
  },
  agent: {
    hostname: 'docker-01',
  },
  ...overrides,
});

const makeDockerService = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-service:svc-01',
  name: 'api',
  displayName: 'api',
  platformId: 'homelab',
  platformType: 'docker',
  sourceType: 'api',
  status: 'online',
  type: 'docker-service',
  lastSeen: 1_700_000_000_000,
  docker: {
    hostname: 'docker-01',
    hostSourceId: 'agent:docker-01',
    image: 'ghcr.io/pulse/api:latest',
    mode: 'replicated',
  },
  ...overrides,
});

describe('dockerPageModel', () => {
  it('buckets Docker hosts, containers, and Swarm services from canonical resources', () => {
    const model = buildDockerPageModel([
      makeResource({ id: 'docker-host-1', type: 'agent' }),
      makeResource({
        id: 'ctr-1',
        type: 'app-container',
        platformType: 'docker',
      }),
      makeResource({ id: 'svc-1', type: 'docker-service' }),
      makeResource({
        id: 'pve-node-1',
        type: 'agent',
        platformType: 'proxmox-pve',
      }),
    ]);

    expect(model.hosts.map((resource) => resource.id)).toEqual(['docker-host-1']);
    expect(model.containers.map((resource) => resource.id)).toEqual(['ctr-1']);
    expect(model.services.map((resource) => resource.id)).toEqual(['svc-1']);
    expect(model.resources.map((resource) => resource.id).sort()).toEqual(
      ['ctr-1', 'docker-host-1', 'svc-1'].sort(),
    );
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

  it('hides Docker container I/O columns by default when the snapshot has no I/O telemetry', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({ id: 'ctr-1', type: 'app-container' }),
      ]),
    ).toEqual(['disk', 'tags', 'netIo', 'diskIo']);
  });

  it('keeps Docker container I/O columns default-visible once telemetry exists', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-1',
          type: 'app-container',
          network: { rxBytes: 0, txBytes: 0 },
          diskIO: { readRate: 0, writeRate: 0 },
        }),
      ]),
    ).toEqual(['disk', 'tags']);
  });

  it('decides Docker container network and disk I/O defaults independently', () => {
    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-1',
          type: 'app-container',
          network: { rxBytes: 128, txBytes: 64 },
        }),
      ]),
    ).toEqual(['disk', 'tags', 'diskIo']);

    expect(
      buildDockerContainerDefaultHiddenColumnIds([
        makeResource({
          id: 'ctr-2',
          type: 'app-container',
          diskIO: { readRate: 128, writeRate: 64 },
        }),
      ]),
    ).toEqual(['disk', 'tags', 'netIo']);
  });

  it('builds host-identity badges for Docker workload group rows', () => {
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

    const badges = buildDockerWorkloadGroupLabelBadges([lxcHost, genericDockerHost]);
    expect(badges['app-container:frigate']?.label).toBe('LXC');
    expect(badges['app-container:Frigate host']?.label).toBe('LXC');
    expect(badges['app-container:proxmox-lxc-docker:pve:101']?.label).toBe('LXC');
    expect(badges['app-container:plain-docker']).toBeUndefined();
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

  it('filters Docker hosts by runtime, status, search, and selected host scope', () => {
    const hosts = [
      makeDockerHost(),
      makeDockerHost({
        id: 'agent:podman-01',
        name: 'podman-01',
        displayName: 'Podman 01',
        status: 'offline',
        docker: {
          runtime: 'podman',
          hostname: 'podman-01',
        },
        agent: {
          hostname: 'podman-01',
        },
      }),
    ];

    expect(filterDockerHosts(hosts, { containerRuntime: 'podman' }).map((host) => host.id)).toEqual(
      ['agent:podman-01'],
    );
    expect(filterDockerHosts(hosts, { statusMode: 'stopped' }).map((host) => host.id)).toEqual([
      'agent:podman-01',
    ]);
    expect(filterDockerHosts(hosts, { searchTerm: 'docker 01' }).map((host) => host.id)).toEqual([
      'agent:docker-01',
    ]);
    expect(filterDockerHosts(hosts, { searchTerm: 'podman' }).map((host) => host.id)).toEqual([
      'agent:podman-01',
    ]);
    expect(
      filterDockerHosts(hosts, { selectedHostScope: 'agent:podman-01' }).map((host) => host.id),
    ).toEqual(['agent:podman-01']);
  });

  it('filters Docker services with the shared page filters and hides them for Podman runtime scope', () => {
    const services = [
      makeDockerService(),
      makeDockerService({
        id: 'docker-service:svc-02',
        name: 'worker',
        status: 'degraded',
        docker: {
          hostname: 'docker-02',
          hostSourceId: 'agent:docker-02',
          image: 'ghcr.io/pulse/worker:latest',
          mode: 'global',
        },
      }),
    ];

    expect(filterDockerServices(services, { containerRuntime: 'podman' })).toEqual([]);
    expect(
      filterDockerServices(services, { statusMode: 'degraded' }).map((service) => service.id),
    ).toEqual(['docker-service:svc-02']);
    expect(
      filterDockerServices(services, { selectedHostScope: 'agent:docker-01' }).map(
        (service) => service.id,
      ),
    ).toEqual(['docker-service:svc-01']);
    expect(
      filterDockerServices(services, { searchTerm: 'worker' }).map((service) => service.id),
    ).toEqual(['docker-service:svc-02']);
  });
});
