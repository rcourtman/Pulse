import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import {
  DOCKER_TAB_SPECS,
  buildDockerIncidentRows,
  buildDockerNetworkAttachmentRows,
  buildDockerPageModel,
  compareDockerContainers,
  compareDockerServices,
  compareDockerSwarmNodes,
  compareDockerTasks,
  dockerServiceStack,
  filterDockerIncidents,
  filterDockerResources,
  getDockerHostSystemBadge,
  getDockerPageTabSpecs,
  hasDockerEngineStorageUsage,
  hasDockerSwarmEvidence,
  hasDockerSwarmInventory,
  mapDockerContainerStatus,
  mapDockerIncidentSeverity,
  mapDockerServiceStatus,
  mapDockerSwarmNodeStatus,
  mapDockerTaskStatus,
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

  it('admits an agent-scoped runtime host when its Docker facet has runtime evidence', () => {
    const model = buildDockerPageModel([
      makeResource({
        id: 'tower',
        type: 'agent',
        platformType: 'agent',
        platformScopes: ['agent'],
        platformData: { docker: { runtime: 'docker' } },
      }),
    ]);

    expect(model.hosts.map((resource) => resource.id)).toEqual(['tower']);
    expect(model.resources.map((resource) => resource.id)).toEqual(['tower']);
  });

  it('declares operator workflow tabs for Docker runtime inventory', () => {
    expect(DOCKER_TAB_SPECS.map((tab) => tab.id)).toEqual([
      'overview',
      'images',
      'storage',
      'networks',
      'swarm',
    ]);
  });

  it('keeps legacy Docker object routes mapped to workflow tabs', () => {
    expect(resolveDockerPageTabId(undefined)).toBe('overview');
    expect(resolveDockerPageTabId('containers')).toBe('overview');
    expect(resolveDockerPageTabId('volumes')).toBe('storage');
    expect(resolveDockerPageTabId('storage')).toBe('storage');
    expect(resolveDockerPageTabId('services')).toBe('swarm');
    expect(resolveDockerPageTabId('tasks')).toBe('swarm');
    expect(resolveDockerPageTabId('swarm-nodes')).toBe('swarm');
    expect(resolveDockerPageTabId('secrets')).toBe('swarm');
    expect(resolveDockerPageTabId('configs')).toBe('swarm');
    expect(resolveDockerPageTabId('unknown')).toBe('overview');
  });

  it('builds host-scoped Docker network attachment rows from relationships and legacy network names', () => {
    const network = makeResource({
      id: 'network-1',
      type: 'docker-network',
      name: 'frontend',
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
      },
    });
    const related = makeResource({
      id: 'container-1',
      type: 'app-container',
      name: 'api',
      displayName: 'api',
      status: 'running',
      relationships: [
        {
          sourceId: 'container-1',
          targetId: 'network-1',
          type: 'attached_to',
          confidence: 1,
          active: true,
          discoverer: 'docker_adapter',
          observedAt: '2026-06-03T09:00:00Z',
          lastSeenAt: '2026-06-03T09:00:00Z',
        },
      ],
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        image: 'repo/api:latest',
        containerState: 'running',
        health: 'healthy',
        networks: [{ name: 'frontend', ipv4: '10.88.0.12' }],
      },
    });
    const fallback = makeResource({
      id: 'container-2',
      type: 'app-container',
      name: 'worker',
      displayName: 'worker',
      status: 'running',
      docker: {
        hostname: 'edge-01',
        hostSourceId: 'docker-host-1',
        image: 'repo/worker:latest',
        containerState: 'running',
        networks: [{ name: 'frontend', ipv4: '10.88.0.13' }],
      },
    });
    const otherHost = makeResource({
      id: 'container-3',
      type: 'app-container',
      name: 'other-host-api',
      displayName: 'other-host-api',
      status: 'running',
      docker: {
        hostname: 'edge-02',
        hostSourceId: 'docker-host-2',
        networks: [{ name: 'frontend', ipv4: '10.99.0.12' }],
      },
    });

    const rows = buildDockerNetworkAttachmentRows(network, [network, related, fallback, otherHost]);

    expect(rows.map((row) => [row.name, row.address])).toEqual([
      ['api', '10.88.0.12'],
      ['worker', '10.88.0.13'],
    ]);
    expect(rows[0].searchText).toContain('repo/api:latest');
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

  it('derives visible Docker workflow tabs from canonical inventory evidence', () => {
    const hostOnlyModel = buildDockerPageModel([
      makeResource({
        id: 'docker-host-1',
        type: 'agent',
        docker: {
          runtime: 'docker',
          swarm: {
            nodeRole: 'worker',
            localState: 'inactive',
          },
        },
      }),
    ]);
    const runtimeInventoryModel = buildDockerPageModel([
      makeResource({ id: 'docker-host-1', type: 'agent' }),
      makeResource({ id: 'ctr-1', type: 'app-container' }),
      makeResource({ id: 'image-1', type: 'docker-image' }),
      makeResource({ id: 'volume-1', type: 'docker-volume' }),
      makeResource({ id: 'network-1', type: 'docker-network' }),
    ]);
    const swarmModel = buildDockerPageModel([
      makeResource({
        id: 'docker-host-1',
        type: 'agent',
        docker: {
          runtime: 'docker',
          swarm: {
            nodeId: 'node-1',
            nodeRole: 'manager',
            localState: 'active',
          },
        },
      }),
    ]);

    expect(hasDockerSwarmInventory(hostOnlyModel)).toBe(false);
    expect(getDockerPageTabSpecs(hostOnlyModel).map((tab) => tab.id)).toEqual(['overview']);
    expect(getDockerPageTabSpecs(runtimeInventoryModel).map((tab) => tab.id)).toEqual([
      'overview',
      'images',
      'storage',
      'networks',
    ]);
    expect(hasDockerSwarmInventory(swarmModel)).toBe(true);
    expect(getDockerPageTabSpecs(swarmModel).map((tab) => tab.id)).toEqual(['overview', 'swarm']);
  });

  it('shows the Storage workflow tab when engine disk usage exists without volume rows', () => {
    const model = buildDockerPageModel([
      makeResource({
        id: 'docker-host-storage',
        type: 'agent',
        docker: {
          runtime: 'docker',
          imagesUsage: {
            totalCount: 1,
            totalSizeBytes: 1024,
          },
        },
      }),
    ]);

    expect(getDockerPageTabSpecs(model).map((tab) => tab.id)).toEqual(['overview', 'storage']);
  });

  it('detects engine storage usage only from populated disk-usage buckets', () => {
    expect(
      hasDockerEngineStorageUsage(
        makeResource({
          id: 'docker-host-empty',
          type: 'agent',
          docker: {
            runtime: 'docker',
            imagesUsage: {
              totalCount: 0,
              totalSizeBytes: 0,
              reclaimableBytes: 0,
            },
          },
        }),
      ),
    ).toBe(false);

    expect(
      hasDockerEngineStorageUsage(
        makeResource({
          id: 'docker-host-storage',
          type: 'agent',
          docker: {
            runtime: 'docker',
            buildCacheUsage: {
              totalCount: 1,
              totalSizeBytes: 1024,
            },
          },
        }),
      ),
    ).toBe(true);
  });

  describe('mapDockerContainerStatus', () => {
    it('escalates dead, OOMKilled, and unhealthy containers to danger', () => {
      expect(
        mapDockerContainerStatus(
          makeResource({ id: 'c-dead', type: 'app-container', docker: { containerState: 'dead' } }),
        ).variant,
      ).toBe('danger');
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-oom',
            type: 'app-container',
            docker: { containerState: 'oomkilled' },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'OOMKilled' });
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-unhealthy',
            type: 'app-container',
            docker: { containerState: 'running', health: 'unhealthy' },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'Unhealthy' });
    });

    it('flags exited containers with non-zero exit codes as danger', () => {
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-exit-1',
            type: 'app-container',
            docker: { containerState: 'exited', exitCode: 137 },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'Exited (137)' });
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-exit-0',
            type: 'app-container',
            docker: { containerState: 'exited', exitCode: 0 },
          }),
        ),
      ).toEqual({ variant: 'muted', label: 'Exited' });
    });

    it('treats restarting and health=starting as warning', () => {
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-restart',
            type: 'app-container',
            docker: { containerState: 'restarting' },
          }),
        ).variant,
      ).toBe('warning');
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-starting',
            type: 'app-container',
            docker: { containerState: 'running', health: 'starting' },
          }),
        ).variant,
      ).toBe('warning');
    });

    it('returns success for healthy running containers', () => {
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-ok',
            type: 'app-container',
            docker: { containerState: 'running', health: 'healthy' },
          }),
        ),
      ).toEqual({ variant: 'success', label: 'Healthy' });
      expect(
        mapDockerContainerStatus(
          makeResource({
            id: 'c-no-health',
            type: 'app-container',
            docker: { containerState: 'running' },
          }),
        ),
      ).toEqual({ variant: 'success', label: 'Running' });
    });
  });

  describe('mapDockerServiceStatus', () => {
    it('classifies services by running vs desired tasks', () => {
      expect(
        mapDockerServiceStatus(
          makeResource({
            id: 'svc-ok',
            type: 'docker-service',
            docker: { desiredTasks: 3, runningTasks: 3 },
          }),
        ).variant,
      ).toBe('success');
      expect(
        mapDockerServiceStatus(
          makeResource({
            id: 'svc-partial',
            type: 'docker-service',
            docker: { desiredTasks: 3, runningTasks: 1 },
          }),
        ),
      ).toEqual({ variant: 'warning', label: '1 / 3 running' });
      expect(
        mapDockerServiceStatus(
          makeResource({
            id: 'svc-zero',
            type: 'docker-service',
            docker: { desiredTasks: 3, runningTasks: 0 },
          }),
        ),
      ).toEqual({ variant: 'danger', label: '0 / 3 running' });
      expect(
        mapDockerServiceStatus(
          makeResource({
            id: 'svc-scaled',
            type: 'docker-service',
            docker: { desiredTasks: 0, runningTasks: 0 },
          }),
        ).variant,
      ).toBe('muted');
    });

    it('flags paused rollbacks as warning even when replicas match', () => {
      expect(
        mapDockerServiceStatus(
          makeResource({
            id: 'svc-rollback',
            type: 'docker-service',
            docker: {
              desiredTasks: 3,
              runningTasks: 3,
              serviceUpdate: { state: 'paused' },
            },
          }),
        ),
      ).toEqual({ variant: 'warning', label: 'Rollback paused' });
    });
  });

  describe('mapDockerTaskStatus', () => {
    it('classifies tasks by current state', () => {
      expect(
        mapDockerTaskStatus(
          makeResource({ id: 't-run', type: 'docker-task', docker: { currentState: 'running' } }),
        ).variant,
      ).toBe('success');
      expect(
        mapDockerTaskStatus(
          makeResource({ id: 't-fail', type: 'docker-task', docker: { currentState: 'failed' } }),
        ).variant,
      ).toBe('danger');
      expect(
        mapDockerTaskStatus(
          makeResource({
            id: 't-prep',
            type: 'docker-task',
            docker: { currentState: 'preparing' },
          }),
        ).variant,
      ).toBe('warning');
      expect(
        mapDockerTaskStatus(
          makeResource({
            id: 't-shutdown',
            type: 'docker-task',
            docker: { currentState: 'shutdown', desiredState: 'shutdown' },
          }),
        ).variant,
      ).toBe('muted');
    });
  });

  describe('mapDockerSwarmNodeStatus', () => {
    it('flags unreachable managers as danger', () => {
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: 'n-mgr-down',
            type: 'docker-swarm-node',
            docker: { nodeRole: 'manager', managerReachability: 'unreachable' },
          }),
        ),
      ).toEqual({ variant: 'danger', label: 'Manager unreachable' });
    });

    it('treats drain as warning and pause as muted', () => {
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: 'n-drain',
            type: 'docker-swarm-node',
            docker: { availability: 'drain' },
          }),
        ).variant,
      ).toBe('warning');
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: 'n-pause',
            type: 'docker-swarm-node',
            docker: { availability: 'pause' },
          }),
        ).variant,
      ).toBe('muted');
    });

    it('marks leader nodes with a Leader label', () => {
      expect(
        mapDockerSwarmNodeStatus(
          makeResource({
            id: 'n-leader',
            type: 'docker-swarm-node',
            status: 'online',
            docker: { availability: 'active', leader: true },
          }),
        ),
      ).toEqual({ variant: 'success', label: 'Leader' });
    });
  });

  describe('rank comparators float attention rows above healthy rows', () => {
    it('orders containers danger → warning → muted → success', () => {
      const healthy = makeResource({
        id: 'happy',
        type: 'app-container',
        docker: { containerState: 'running', health: 'healthy' },
      });
      const restarting = makeResource({
        id: 'restarting',
        type: 'app-container',
        docker: { containerState: 'restarting' },
      });
      const dead = makeResource({
        id: 'dead',
        type: 'app-container',
        docker: { containerState: 'dead' },
      });
      const stopped = makeResource({
        id: 'stopped',
        type: 'app-container',
        docker: { containerState: 'exited', exitCode: 0 },
      });
      expect(
        [healthy, restarting, dead, stopped].sort(compareDockerContainers).map((r) => r.id),
      ).toEqual(['dead', 'restarting', 'stopped', 'happy']);
    });

    it('orders Swarm services by replica health', () => {
      const ok = makeResource({
        id: 'svc-ok',
        type: 'docker-service',
        docker: { desiredTasks: 2, runningTasks: 2 },
      });
      const partial = makeResource({
        id: 'svc-partial',
        type: 'docker-service',
        docker: { desiredTasks: 2, runningTasks: 1 },
      });
      const down = makeResource({
        id: 'svc-down',
        type: 'docker-service',
        docker: { desiredTasks: 2, runningTasks: 0 },
      });
      expect([ok, partial, down].sort(compareDockerServices).map((r) => r.id)).toEqual([
        'svc-down',
        'svc-partial',
        'svc-ok',
      ]);
    });

    it('orders tasks failed → preparing → running', () => {
      const run = makeResource({
        id: 't-run',
        type: 'docker-task',
        docker: { currentState: 'running' },
      });
      const prep = makeResource({
        id: 't-prep',
        type: 'docker-task',
        docker: { currentState: 'preparing' },
      });
      const fail = makeResource({
        id: 't-fail',
        type: 'docker-task',
        docker: { currentState: 'failed' },
      });
      expect([run, prep, fail].sort(compareDockerTasks).map((r) => r.id)).toEqual([
        't-fail',
        't-prep',
        't-run',
      ]);
    });

    it('orders Swarm nodes by manager reachability and availability', () => {
      const ok = makeResource({
        id: 'n-ok',
        type: 'docker-swarm-node',
        status: 'online',
        docker: { availability: 'active', managerReachability: 'reachable' },
      });
      const drain = makeResource({
        id: 'n-drain',
        type: 'docker-swarm-node',
        status: 'online',
        docker: { availability: 'drain' },
      });
      const down = makeResource({
        id: 'n-mgr-down',
        type: 'docker-swarm-node',
        docker: { nodeRole: 'manager', managerReachability: 'unreachable' },
      });
      expect([ok, drain, down].sort(compareDockerSwarmNodes).map((r) => r.id)).toEqual([
        'n-mgr-down',
        'n-drain',
        'n-ok',
      ]);
    });

    it('emits pre-sorted buckets from buildDockerPageModel', () => {
      const happy = makeResource({
        id: 'c-happy',
        type: 'app-container',
        docker: { containerState: 'running' },
      });
      const dead = makeResource({
        id: 'c-dead',
        type: 'app-container',
        docker: { containerState: 'dead' },
      });
      const svcOk = makeResource({
        id: 'svc-ok',
        type: 'docker-service',
        docker: { desiredTasks: 2, runningTasks: 2 },
      });
      const svcDown = makeResource({
        id: 'svc-down',
        type: 'docker-service',
        docker: { desiredTasks: 2, runningTasks: 0 },
      });
      const model = buildDockerPageModel([happy, dead, svcOk, svcDown]);
      expect(model.containers.map((r) => r.id)).toEqual(['c-dead', 'c-happy']);
      expect(model.services.map((r) => r.id)).toEqual(['svc-down', 'svc-ok']);
    });
  });

  describe('filterDockerResources', () => {
    const rows: Resource[] = [
      makeResource({ id: 'web', type: 'app-container', status: 'online' }),
      makeResource({
        id: 'edge-proxy',
        type: 'app-container',
        status: 'online',
        docker: {
          hostname: 'edge-01',
          image: 'traefik:v3.1',
          runtime: 'docker',
          runtimeVersion: '27.5.1',
          containerState: 'running',
          ports: [{ ip: '0.0.0.0', publicPort: 8080, privatePort: 80, protocol: 'tcp' }],
        },
      }),
      makeResource({
        id: 'svc-payments',
        type: 'docker-service',
        status: 'degraded',
        docker: {
          serviceName: 'payments-worker',
          swarm: { clusterName: 'prod-swarm', nodeRole: 'manager' },
        },
      }),
      makeResource({
        id: 'svc-search',
        type: 'docker-service',
        status: 'warning' as Resource['status'],
        docker: { serviceName: 'search-worker' },
      }),
      makeResource({
        id: 'redis-vol',
        type: 'docker-volume',
        status: 'online',
        docker: { volumeName: 'redis-data', driver: 'local' },
      }),
    ];

    it('matches docker.* fields the shared filter no longer carries', () => {
      expect(filterDockerResources(rows, 'edge-01', 'all').map((r) => r.id)).toEqual([
        'edge-proxy',
      ]);
      expect(filterDockerResources(rows, 'traefik', 'all').map((r) => r.id)).toEqual([
        'edge-proxy',
      ]);
      expect(filterDockerResources(rows, 'payments-worker', 'all').map((r) => r.id)).toEqual([
        'svc-payments',
      ]);
      expect(filterDockerResources(rows, 'prod-swarm', 'all').map((r) => r.id)).toEqual([
        'svc-payments',
      ]);
      expect(filterDockerResources(rows, 'redis-data', 'all').map((r) => r.id)).toEqual([
        'redis-vol',
      ]);
    });

    it('matches composite port tokens like host:public->private/proto', () => {
      expect(filterDockerResources(rows, '0.0.0.0:8080->80/tcp', 'all').map((r) => r.id)).toEqual([
        'edge-proxy',
      ]);
    });

    it('hides rows matching -term exclusions', () => {
      expect(filterDockerResources(rows, '-worker', 'all').map((r) => r.id)).toEqual([
        'web',
        'edge-proxy',
        'redis-vol',
      ]);
      // Exclusions search the same haystack as positive terms (image, swarm
      // cluster, volume name), and combine with a positive needle.
      expect(filterDockerResources(rows, '-traefik -redis', 'all').map((r) => r.id)).toEqual([
        'web',
        'svc-payments',
        'svc-search',
      ]);
      expect(filterDockerResources(rows, 'worker -payments', 'all').map((r) => r.id)).toEqual([
        'svc-search',
      ]);
    });

    it('matches container labels and Swarm stack names', () => {
      const labelled: Resource[] = [
        ...rows,
        makeResource({
          id: 'compose-db',
          type: 'app-container',
          status: 'online',
          docker: {
            containerState: 'running',
            labels: {
              'com.docker.compose.project': 'orion',
              'com.docker.compose.service': 'database',
            },
          },
        }),
        makeResource({
          id: 'svc-stacked',
          type: 'docker-service',
          status: 'online',
          docker: {
            serviceName: 'shop-api',
            stack: 'shopstack',
            labels: { 'com.docker.stack.namespace': 'legacy-stack' },
          },
        }),
        makeResource({
          id: 'podman-web',
          type: 'app-container',
          status: 'online',
          docker: {
            containerState: 'running',
            podman: {
              podName: 'edge-pod',
              podId: 'pod-123',
              infra: false,
              composeProject: 'orion',
              composeService: 'web',
              autoUpdatePolicy: 'registry',
              userNamespace: 'keep-id',
            },
          },
        }),
      ];

      expect(filterDockerResources(labelled, 'orion', 'all').map((r) => r.id)).toEqual([
        'compose-db',
        'podman-web',
      ]);
      expect(filterDockerResources(labelled, 'database', 'all').map((r) => r.id)).toEqual([
        'compose-db',
      ]);
      expect(filterDockerResources(labelled, 'shopstack', 'all').map((r) => r.id)).toEqual([
        'svc-stacked',
      ]);
      expect(dockerServiceStack(labelled.find((resource) => resource.id === 'svc-stacked')!)).toBe(
        'shopstack',
      );
      expect(filterDockerResources(labelled, 'legacy-stack', 'all').map((r) => r.id)).toEqual([
        'svc-stacked',
      ]);
      expect(filterDockerResources(labelled, 'pod:edge-pod', 'all').map((r) => r.id)).toEqual([
        'podman-web',
      ]);
      expect(filterDockerResources(labelled, 'compose:orion', 'all').map((r) => r.id)).toEqual([
        'podman-web',
      ]);
      expect(filterDockerResources(labelled, 'keep-id', 'all').map((r) => r.id)).toEqual([
        'podman-web',
      ]);
    });

    it('still combines status and search', () => {
      expect(filterDockerResources(rows, 'payments-worker', 'degraded').map((r) => r.id)).toEqual([
        'svc-payments',
      ]);
      expect(filterDockerResources(rows, 'payments-worker', 'online')).toEqual([]);
    });

    it('returns all rows for empty search and triad filters by status', () => {
      expect(
        filterDockerResources(rows, '', 'all')
          .map((r) => r.id)
          .sort(),
      ).toEqual(['edge-proxy', 'redis-vol', 'svc-payments', 'svc-search', 'web'].sort());
      expect(
        filterDockerResources(rows, '', 'degraded')
          .map((r) => r.id)
          .sort(),
      ).toEqual(['svc-payments', 'svc-search'].sort());
    });
  });

  describe('mapDockerIncidentSeverity', () => {
    it('buckets severity strings into critical / warning / info', () => {
      expect(mapDockerIncidentSeverity('critical')).toBe('critical');
      expect(mapDockerIncidentSeverity('FATAL')).toBe('critical');
      expect(mapDockerIncidentSeverity('error')).toBe('critical');
      expect(mapDockerIncidentSeverity('warning')).toBe('warning');
      expect(mapDockerIncidentSeverity('degraded')).toBe('warning');
      expect(mapDockerIncidentSeverity('info')).toBe('info');
      expect(mapDockerIncidentSeverity(undefined)).toBe('info');
      expect(mapDockerIncidentSeverity('whatever')).toBe('info');
    });
  });

  describe('buildDockerIncidentRows', () => {
    it('emits one row per ResourceIncident and rolls up severity sort order', () => {
      const rows = buildDockerIncidentRows([
        makeResource({
          id: 'host-edge',
          type: 'agent',
          name: 'edge-01',
          docker: { hostname: 'edge-01' },
          incidents: [
            { code: 'docker_host_down', severity: 'critical', summary: 'Engine unreachable' },
            { code: 'docker_image_update', severity: 'info', summary: 'Image update available' },
          ],
        }),
        makeResource({
          id: 'ctr-payments',
          type: 'app-container',
          name: 'payments-worker',
          docker: { hostname: 'edge-01' },
          incidents: [
            {
              code: 'docker_container_restarting',
              severity: 'warning',
              summary: 'Restarted 7 times in 5m',
            },
          ],
        }),
      ]);

      expect(rows.map((r) => ({ id: r.id, severity: r.severityBucket }))).toEqual([
        { id: 'host-edge:incident:docker_host_down:0', severity: 'critical' },
        { id: 'ctr-payments:incident:docker_container_restarting:0', severity: 'warning' },
        { id: 'host-edge:incident:docker_image_update:1', severity: 'info' },
      ]);
      expect(rows[0].summary).toBe('Engine unreachable');
      expect(rows[0].source).toBe('docker');
    });

    it('falls back to a rollup row when only incidentCount / incidentSummary are present', () => {
      const rows = buildDockerIncidentRows([
        makeResource({
          id: 'host-stale',
          type: 'agent',
          name: 'stale-01',
          incidentCount: 2,
          incidentSeverity: 'warning',
          incidentSummary: 'Two alerts firing',
          incidentLabel: 'Docker Alerts',
        }),
      ]);
      expect(rows).toHaveLength(1);
      expect(rows[0].id).toBe('host-stale:incident:rollup');
      expect(rows[0].severityBucket).toBe('warning');
      expect(rows[0].summary).toBe('Two alerts firing');
    });

    it('skips resources with no incident signal', () => {
      expect(
        buildDockerIncidentRows([makeResource({ id: 'clean', type: 'app-container' })]),
      ).toEqual([]);
    });

    it('surfaces incidents on the page model output', () => {
      const model = buildDockerPageModel([
        makeResource({
          id: 'host-edge',
          type: 'agent',
          incidents: [{ code: 'docker_host_down', severity: 'critical', summary: 'down' }],
        }),
        makeResource({ id: 'clean', type: 'app-container' }),
      ]);
      expect(model.incidents.map((r) => r.resourceId)).toEqual(['host-edge']);
    });
  });

  describe('filterDockerIncidents', () => {
    const incidents = buildDockerIncidentRows([
      makeResource({
        id: 'host-edge',
        type: 'agent',
        name: 'edge-01',
        docker: { hostname: 'edge-01' },
        incidents: [{ code: 'docker_host_down', severity: 'critical', summary: 'Engine down' }],
      }),
      makeResource({
        id: 'ctr-payments',
        type: 'app-container',
        name: 'payments-worker',
        docker: { hostname: 'edge-01' },
        incidents: [
          { code: 'docker_container_restarting', severity: 'warning', summary: 'Restart loop' },
        ],
      }),
    ]);

    it('filters by severity bucket', () => {
      expect(filterDockerIncidents(incidents, '', 'critical').map((r) => r.resourceId)).toEqual([
        'host-edge',
      ]);
      expect(filterDockerIncidents(incidents, '', 'warning').map((r) => r.resourceId)).toEqual([
        'ctr-payments',
      ]);
    });

    it('matches resource name, code, summary, and host', () => {
      expect(filterDockerIncidents(incidents, 'payments', 'all').map((r) => r.resourceId)).toEqual([
        'ctr-payments',
      ]);
      expect(filterDockerIncidents(incidents, 'host_down', 'all').map((r) => r.resourceId)).toEqual(
        ['host-edge'],
      );
      expect(
        filterDockerIncidents(incidents, 'engine down', 'all').map((r) => r.resourceId),
      ).toEqual(['host-edge']);
      expect(filterDockerIncidents(incidents, 'edge-01', 'all').length).toBe(2);
    });
  });
});
