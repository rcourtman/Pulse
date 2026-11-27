import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  MAP_LINK_SATURATED_PERCENT,
  MAP_TRAFFIC_ACTIVE_BYTES,
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
  computeMapLinkUtilizationPercent,
  resolveMapHealth,
  resolveMapTrafficTier,
  type MapBlock,
  type MapHost,
} from '../infrastructureMapModel';

const resource = (partial: Partial<Resource> & Pick<Resource, 'id' | 'type' | 'name'>): Resource =>
  ({
    displayName: partial.name,
    platformId: partial.id,
    platformType: 'generic',
    sourceType: 'api',
    status: 'online',
    lastSeen: 1,
    ...partial,
  }) as Resource;

const GIB = 1024 ** 3;

const estate = (): Resource[] => [
  resource({
    id: 'node-b',
    type: 'agent',
    name: 'pve-b',
    platformType: 'proxmox-pve',
    clusterId: 'homelab',
    proxmox: { isClusterMember: true, pveVersion: '8.2.4' },
    agent: { cpuCount: 8 },
    memory: { current: 50, total: 32 * GIB, used: 16 * GIB },
    cpu: { current: 12.4 },
    health: { verdict: 'ok', reasons: [] },
  }),
  resource({
    id: 'node-a',
    type: 'agent',
    name: 'pve-a',
    platformType: 'proxmox-pve',
    clusterId: 'homelab',
    proxmox: { isClusterMember: true },
    health: { verdict: 'ok', reasons: [] },
  }),
  resource({
    id: 'node-solo',
    type: 'agent',
    name: 'pve-solo',
    platformType: 'proxmox-pve',
    clusterId: 'pve-solo',
    proxmox: { isClusterMember: false },
  }),
  resource({
    id: 'vm-10',
    type: 'vm',
    name: 'nextcloud',
    platformType: 'proxmox-pve',
    parentId: 'node-a',
    status: 'running',
    health: { verdict: 'attention', reasons: [{ code: 'backup_stale' }] },
  }),
  resource({
    id: 'vm-2',
    type: 'vm',
    name: 'haos',
    platformType: 'proxmox-pve',
    parentId: 'node-a',
    status: 'running',
  }),
  resource({
    id: 'ct-1',
    type: 'system-container',
    name: 'pihole',
    platformType: 'proxmox-pve',
    parentId: 'node-a',
    status: 'stopped',
  }),
  resource({
    id: 'local-zfs',
    type: 'storage',
    name: 'local-zfs',
    platformType: 'proxmox-pve',
    parentId: 'node-a',
    disk: { current: 40, total: 100 * GIB, used: 40 * GIB },
  }),
  resource({
    id: 'nfs',
    type: 'storage',
    name: 'nas-nfs',
    platformType: 'proxmox-pve',
    proxmox: { instance: 'homelab' },
    disk: { current: 91, total: 200 * GIB, used: 182 * GIB },
  }),
  resource({ id: 'ceph', type: 'ceph', name: 'ceph', platformType: 'proxmox-pve' }),
  resource({
    id: 'docker-1',
    type: 'docker-host',
    name: 'apps',
    platformType: 'docker',
    docker: { runtime: 'podman', runtimeVersion: '5.1' },
  }),
  resource({
    id: 'c-1',
    type: 'app-container',
    name: 'jellyfin',
    platformType: 'docker',
    parentId: 'docker-1',
    status: 'running',
  }),
  resource({
    id: 'image-1',
    type: 'docker-image',
    name: 'jellyfin:latest',
    platformType: 'docker',
    parentId: 'docker-1',
  }),
  resource({ id: 'k8s', type: 'k8s-cluster', name: 'k3s', platformType: 'kubernetes' }),
  resource({
    id: 'k8s-node',
    type: 'agent',
    name: 'k3s-1',
    platformType: 'agent',
    parentId: 'k8s',
  }),
  resource({
    id: 'pod-1',
    type: 'pod',
    name: 'traefik-abc',
    platformType: 'kubernetes',
    parentId: 'k8s',
    status: 'running',
    kubernetes: { namespace: 'kube-system' },
  }),
  resource({ id: 'nas', type: 'agent', name: 'truenas', platformType: 'truenas' }),
  resource({
    id: 'tank',
    type: 'storage',
    name: 'tank',
    platformType: 'truenas',
    parentId: 'nas',
  }),
  resource({
    id: 'disk-1',
    type: 'physical_disk',
    name: 'WD Red',
    platformType: 'truenas',
    parentId: 'tank',
    status: 'unknown',
  }),
  resource({
    id: 'app-1',
    type: 'app-container',
    name: 'immich',
    platformType: 'truenas',
    parentId: 'nas',
    status: 'running',
  }),
  resource({ id: 'pbs', type: 'pbs', name: 'backups', platformType: 'proxmox-pbs' }),
  resource({
    id: 'ds',
    type: 'storage',
    name: 'main',
    platformType: 'proxmox-pbs',
    parentId: 'pbs',
  }),
  resource({
    id: 'check-1',
    type: 'network-endpoint',
    name: 'opnsense-edge',
    status: 'offline',
    availability: { targetKind: 'router' },
  }),
  resource({ id: 'mini', type: 'agent', name: 'mini-pc', platformType: 'agent' }),
];

const hostsOf = (blocks: MapBlock[]): MapHost[] =>
  blocks.flatMap((block) => (block.kind === 'group' ? block.group.hosts : [block.host]));

const hostNamed = (blocks: MapBlock[], name: string): MapHost => {
  const host = hostsOf(blocks).find((candidate) => candidate.name === name);
  if (!host) throw new Error(`no host named ${name}`);
  return host;
};

describe('buildInfrastructureMapModel', () => {
  it('draws nothing for an empty estate', () => {
    const model = buildInfrastructureMapModel([]);
    expect(model.isEmpty).toBe(true);
    expect(model.blocks).toEqual([]);
    expect(model.totals).toEqual([]);
  });

  it('frames cluster members together and leaves a standalone node on its own', () => {
    const model = buildInfrastructureMapModel(estate());
    const groups = model.blocks.flatMap((block) => (block.kind === 'group' ? [block.group] : []));
    expect(groups).toHaveLength(1);
    expect(groups[0]).toMatchObject({ name: 'homelab', typeLabel: 'Proxmox cluster' });
    expect(groups[0].hosts.map((host) => host.name)).toEqual(['pve-a', 'pve-b']);

    const loose = model.blocks.flatMap((block) => (block.kind === 'host' ? [block.host.name] : []));
    expect(loose).toContain('pve-solo');
  });

  it('orders blocks by platform, clusters before single hosts', () => {
    const model = buildInfrastructureMapModel(estate());
    const names = model.blocks.map((block) =>
      block.kind === 'group' ? block.group.name : block.host.name,
    );
    expect(names).toEqual([
      'homelab',
      'pve-solo',
      'k3s',
      'apps',
      'truenas',
      'backups',
      'mini-pc',
      'Shared storage',
      'Availability checks',
    ]);
  });

  it('nests guests under the node the registry parents them to', () => {
    const node = hostNamed(buildInfrastructureMapModel(estate()).blocks, 'pve-a');
    expect(
      node.sections.map((section) => [section.label, section.items.map((i) => i.name)]),
    ).toEqual([
      ['VMs', ['haos', 'nextcloud']],
      ['LXCs', ['pihole']],
    ]);
    expect(node.pools.map((pool) => pool.name)).toEqual(['local-zfs']);
    expect(node.pools[0]).toMatchObject({ usedPercent: 40, sizeLabel: '100 GB' });
  });

  it('describes a host by what kind of thing it is and how it is doing', () => {
    const blocks = buildInfrastructureMapModel(estate()).blocks;
    expect(hostNamed(blocks, 'pve-b')).toMatchObject({ typeLabel: 'Proxmox node', health: 'ok' });
    expect(hostNamed(blocks, 'pve-b').cpu).toBeCloseTo(12.4);
    expect(hostNamed(blocks, 'apps').typeLabel).toBe('Docker host');
    expect(hostNamed(blocks, 'backups').typeLabel).toBe('Backup server');
  });

  it('keeps only what a reader would call a workload inside a Docker host', () => {
    const docker = hostNamed(buildInfrastructureMapModel(estate()).blocks, 'apps');
    expect(docker.sections).toHaveLength(1);
    expect(docker.sections[0]).toMatchObject({ label: 'Containers' });
    expect(docker.sections[0].items.map((item) => item.name)).toEqual(['jellyfin']);
  });

  it('puts namespaced pods inside their cluster and draws no node as a host of its own', () => {
    const blocks = buildInfrastructureMapModel(estate()).blocks;
    const cluster = hostNamed(blocks, 'k3s');
    expect(cluster.sections.map((section) => section.label)).toEqual(['kube-system']);
    expect(hostsOf(blocks).map((host) => host.name)).not.toContain('k3s-1');
  });

  it('gives a TrueNAS system its apps and pools and leaves single disks out', () => {
    const nas = hostNamed(buildInfrastructureMapModel(estate()).blocks, 'truenas');
    expect(nas.sections.map((section) => section.label)).toEqual(['Apps']);
    expect(nas.pools.map((pool) => pool.name)).toEqual(['tank']);
  });

  it('attaches shared storage to the cluster that reports it and guesses nothing else', () => {
    const model = buildInfrastructureMapModel(estate());
    const group = model.blocks.flatMap((block) => (block.kind === 'group' ? [block.group] : []))[0];
    expect(group.pools.map((pool) => pool.name)).toEqual(['nas-nfs']);
    const shared = hostNamed(model.blocks, 'Shared storage');
    expect(shared.isResource).toBe(false);
    expect(shared.pools.map((pool) => pool.name)).toEqual(['ceph']);
  });

  it('keeps availability checks together, apart from any host', () => {
    const checks = hostNamed(buildInfrastructureMapModel(estate()).blocks, 'Availability checks');
    expect(checks.isResource).toBe(false);
    expect(checks.health).toBe('critical');
    expect(checks.sections[0].items.map((item) => item.name)).toEqual(['opnsense-edge']);
    // Its owner typed it a router, which is what the diagram draws it as.
    expect(checks.sections[0].items[0].checkKind).toBe('router');
  });

  it('counts what it drew and what needs attention', () => {
    const model = buildInfrastructureMapModel(estate());
    expect(Object.fromEntries(model.totals.map((total) => [total.label, total.count]))).toEqual({
      Hosts: 8,
      VMs: 2,
      LXCs: 1,
      Containers: 2,
      Pods: 1,
      Storage: 5,
      Checks: 1,
    });
    expect(model.totals.find((total) => total.key === 'vms')).toMatchObject({ singular: 'VM' });
    // The stale-backup VM and the failed check.
    expect(model.attentionCount).toBe(2);
  });
});

describe('resolveMapHealth', () => {
  it('trusts the registry verdict over the raw status', () => {
    expect(
      resolveMapHealth(
        resource({
          id: 'a',
          type: 'vm',
          name: 'a',
          status: 'running',
          health: { verdict: 'critical', reasons: [] },
        }),
      ),
    ).toBe('critical');
  });

  it('reads a stopped guest as off, not as a failure', () => {
    expect(resolveMapHealth(resource({ id: 'a', type: 'vm', name: 'a', status: 'stopped' }))).toBe(
      'off',
    );
    expect(resolveMapHealth(resource({ id: 'a', type: 'vm', name: 'a', status: 'offline' }))).toBe(
      'critical',
    );
    expect(resolveMapHealth(resource({ id: 'a', type: 'vm', name: 'a', status: 'degraded' }))).toBe(
      'attention',
    );
  });
});

describe('anonymizeInfrastructureMapModel', () => {
  it('replaces every reported name and keeps structure, health and facts', () => {
    const model = buildInfrastructureMapModel(estate());
    const anonymous = anonymizeInfrastructureMapModel(model);
    // Ids and section keys are lookups; neither is ever drawn. What app a
    // thing is stays on purpose, so it is left out of the search for names.
    const text = JSON.stringify(anonymous, (key, value) =>
      key === 'id' || key === 'key' || key === 'app' || key === 'category' ? undefined : value,
    );
    for (const name of [
      'homelab',
      'pve-a',
      'nextcloud',
      'pihole',
      'jellyfin',
      'kube-system',
      'traefik-abc',
      'tank',
      'nas-nfs',
      'opnsense',
      'mini-pc',
    ]) {
      expect(text).not.toContain(name);
    }

    const node = hostsOf(anonymous.blocks)[0];
    expect(node.name).toBe('Proxmox 1');
    // A guest the catalogue knows is named for its app, in the app's own
    // spelling. Anything else is numbered.
    expect(node.sections[0].items.map((item) => item.name)).toEqual([
      'Home Assistant',
      'Nextcloud',
    ]);
    expect(
      hostsOf(
        anonymizeInfrastructureMapModel(
          buildInfrastructureMapModel([
            ...estate(),
            resource({
              id: 'vm-99',
              type: 'vm',
              name: 'accounts-box',
              platformType: 'proxmox-pve',
              parentId: 'node-solo',
              status: 'running',
            }),
          ]),
        ).blocks,
      )
        .flatMap((host) => host.sections)
        .flatMap((section) => section.items)
        .map((item) => item.name),
    ).toContain('VM 1');
    expect(node.sections[0].items[1].health).toBe('attention');
    expect(anonymous.totals).toEqual(model.totals);
    // A check on network gear keeps what it is on and loses what it was called.
    expect(hostNamed(anonymous.blocks, 'Availability checks').sections[0].items[0]).toMatchObject({
      name: 'Router 1',
      checkKind: 'router',
    });
  });

  it('gives pools that share a name one stand-in, so they stay one tile on the map', () => {
    const local = (node: string): Resource =>
      resource({
        id: `local-${node}`,
        type: 'storage',
        name: 'local',
        platformType: 'proxmox-pve',
        parentId: node,
      });
    const anonymous = anonymizeInfrastructureMapModel(
      buildInfrastructureMapModel([
        ...estate(),
        local('node-a'),
        local('node-b'),
        resource({
          id: 'fast',
          type: 'storage',
          name: 'fast-nvme',
          platformType: 'proxmox-pve',
          parentId: 'node-a',
        }),
      ]),
    );
    const pools = hostsOf(anonymous.blocks).flatMap((host) => host.pools);
    const standIn = (id: string) => pools.find((pool) => pool.id === id)?.name;
    expect(standIn('local-node-a')).toMatch(/^Pool \d+$/);
    expect(standIn('local-node-b')).toBe(standIn('local-node-a'));
    expect(standIn('fast')).toMatch(/^Pool \d+$/);
    expect(standIn('fast')).not.toBe(standIn('local-node-a'));
    expect(JSON.stringify(pools)).not.toContain('fast-nvme');
  });

  it('preserves numeric and boolean network telemetry while removing names and addresses', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-1',
        type: 'agent',
        name: 'secret-pve',
        platformType: 'proxmox-pve',
        network: { rxBytes: 5_000_000, txBytes: 2_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'enp1s0', addresses: ['10.20.30.10/24'], speedMbps: 10000, linkUp: true },
            { name: 'enp2s0', addresses: [], speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      resource({
        id: 'vm-1',
        type: 'vm',
        name: 'secret-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'running',
        network: { rxBytes: 1_500_000, txBytes: 500_000 },
        proxmox: {
          networkInterfaces: [
            { name: 'net0', addresses: ['10.20.30.50'], bridge: 'vmbr0', firewall: true },
          ],
        },
      }),
      resource({
        id: 'check-gw',
        type: 'network-endpoint',
        name: 'secret-router',
        status: 'online',
        availability: {
          address: '10.20.30.1',
          targetKind: 'router',
          available: true,
          latencyMillis: 1.8,
        },
      }),
    ]);

    const anonymous = anonymizeInfrastructureMapModel(model);
    const host = hostsOf(anonymous.blocks).find((entry) => entry.id === 'pve-1')!;
    expect(host).toMatchObject({
      name: 'Proxmox 1',
      netRxBytes: 5_000_000,
      netTxBytes: 2_000_000,
      linkSpeedMbps: 10000,
      linkDownCount: 1,
    });
    expect(host.address).toBeUndefined();

    const vm = host.sections[0].items[0];
    expect(vm).toMatchObject({
      name: 'VM 1',
      netRxBytes: 1_500_000,
      netTxBytes: 500_000,
      firewallEnabled: true,
    });
    expect(vm.address).toBeUndefined();

    const router = hostNamed(anonymous.blocks, 'Availability checks').sections[0].items[0];
    expect(router).toMatchObject({
      name: 'Router 1',
      latencyMs: 1.8,
    });
    expect(router.address).toBeUndefined();
  });
});

describe('resolveMapTrafficTier', () => {
  it('classifies live bytes per second into truthful tiers', () => {
    expect(MAP_TRAFFIC_ACTIVE_BYTES).toBe(10 * 1024);
    expect(resolveMapTrafficTier(undefined)).toBeUndefined();
    expect(resolveMapTrafficTier(Number.NaN)).toBeUndefined();
    expect(resolveMapTrafficTier(Number.POSITIVE_INFINITY)).toBeUndefined();
    expect(resolveMapTrafficTier(-5)).toBeUndefined();
    expect(resolveMapTrafficTier(0)).toBeUndefined();
    expect(resolveMapTrafficTier(MAP_TRAFFIC_ACTIVE_BYTES - 1)).toBeUndefined();
    expect(resolveMapTrafficTier(MAP_TRAFFIC_ACTIVE_BYTES)).toBe('low');
    expect(resolveMapTrafficTier(1024 * 1024 - 1)).toBe('low');
    expect(resolveMapTrafficTier(1024 * 1024)).toBe('medium');
    expect(resolveMapTrafficTier(25 * 1024 * 1024 - 1)).toBe('medium');
    expect(resolveMapTrafficTier(25 * 1024 * 1024)).toBe('high');
    expect(resolveMapTrafficTier(100 * 1024 * 1024)).toBe('high');
  });
});

describe('live network telemetry extraction', () => {
  it('extracts truthful rates, link speed, link-down count, firewall, and check latency', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-live',
        type: 'agent',
        name: 'pve-live',
        platformType: 'proxmox-pve',
        status: 'online',
        network: { rxBytes: 0, txBytes: 12_500 },
        agent: {
          networkInterfaces: [
            // Host-local interfaces are ignored for speed and link-down counts.
            { name: 'lo', speedMbps: 40000, linkUp: false },
            { name: 'docker0', speedMbps: 25000, linkUp: false },
            { name: 'podman0', speedMbps: 25000, linkUp: false },
            { name: 'br-0123456789ab', speedMbps: 25000, linkUp: false },
            { name: 'veth9a8b', speedMbps: 10000, linkUp: false },
            { name: 'virbr0', speedMbps: 10000, linkUp: false },
            { name: 'cni0', speedMbps: 10000, linkUp: false },
            { name: 'flannel.1', speedMbps: 10000, linkUp: false },
            { name: 'cali1234', speedMbps: 10000, linkUp: false },
            { name: 'kube-ipvs0', speedMbps: 10000, linkUp: false },
            // Real NICs are inspected.
            { name: 'eno1', speedMbps: 2500, linkUp: true, rxBytes: 999_999_999 },
            { name: 'eno2', speedMbps: 1000, linkUp: false },
          ],
        },
        docker: {
          networkInterfaces: [
            // Duplicate NIC name across facets should not double-count link-down.
            { name: 'eno2', speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      resource({
        id: 'pve-offline',
        type: 'agent',
        name: 'pve-offline',
        platformType: 'proxmox-pve',
        status: 'offline',
        agent: {
          networkInterfaces: [
            // Cumulative counters here are never treated as live rates.
            {
              name: 'eno1',
              speedMbps: 1000,
              linkUp: false,
              rxBytes: 800_000_000,
              txBytes: 900_000,
            },
          ],
        },
      }),
      resource({
        id: 'vm-fw',
        type: 'vm',
        name: 'fw-guest',
        platformType: 'proxmox-pve',
        parentId: 'pve-live',
        status: 'running',
        network: { rxBytes: -10, txBytes: Number.NaN },
        proxmox: {
          networkInterfaces: [
            { name: 'net0', bridge: 'vmbr0', firewall: true },
            { name: 'net1', bridge: 'vmbr1', firewall: false },
          ],
        },
      }),
      resource({
        id: 'check-up',
        type: 'network-endpoint',
        name: 'core-router',
        status: 'online',
        availability: { targetKind: 'router', available: true, latencyMillis: 2.4 },
      }),
      resource({
        id: 'check-down',
        type: 'network-endpoint',
        name: 'down-router',
        status: 'offline',
        availability: { targetKind: 'router', available: false, latencyMillis: 2.4 },
      }),
    ]);

    const liveHost = hostNamed(model.blocks, 'pve-live');
    expect(liveHost).toMatchObject({
      netRxBytes: 0,
      netTxBytes: 12_500,
      linkSpeedMbps: 2500,
      linkDownCount: 1,
    });

    const offlineHost = hostNamed(model.blocks, 'pve-offline');
    expect(offlineHost.netRxBytes).toBeUndefined();
    expect(offlineHost.netTxBytes).toBeUndefined();
    expect(offlineHost.linkSpeedMbps).toBe(1000);
    expect(offlineHost.linkDownCount).toBeUndefined();

    const vm = liveHost.sections[0].items[0];
    expect(vm.netRxBytes).toBeUndefined();
    expect(vm.netTxBytes).toBeUndefined();
    expect(vm.firewallEnabled).toBe(true);

    const checkItems = hostNamed(model.blocks, 'Availability checks').sections[0].items;
    expect(checkItems.find((item) => item.id === 'check-up')?.latencyMs).toBe(2.4);
    expect(checkItems.find((item) => item.id === 'check-down')?.latencyMs).toBeUndefined();
  });

  it('extracts user-defined Docker networks, check link targets, probe summaries, and certificate warnings', () => {
    const soonIso = new Date(Date.now() + 5 * 86_400_000).toISOString();
    const model = buildInfrastructureMapModel([
      resource({ id: 'dock-1', type: 'docker-host', name: 'dock-1', platformType: 'docker' }),
      resource({
        id: 'c-user-nets',
        type: 'app-container',
        name: 'jellyfin',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          networks: [
            { name: 'bridge' },
            { name: 'host' },
            { name: 'none' },
            { name: 'ingress' },
            { name: 'docker_gwbridge' },
            { name: 'media_net' },
            { name: 'proxy_net' },
            { name: 'media_net' },
          ],
        },
      }),
      resource({
        id: 'c-default-only',
        type: 'app-container',
        name: 'redis',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: { networks: [{ name: 'bridge' }] },
      }),
      resource({
        id: 'chk-https',
        type: 'network-endpoint',
        name: 'App HTTPS',
        availability: {
          linkedResourceId: '  c-user-nets  ',
          protocol: 'https',
          applicationStatusCode: 200,
          certificate: {
            observedAt: '2026-11-27T00:00:00Z',
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: '2027-01-01T00:00:00Z',
            fingerprintSha256: 'abc',
            trustStatus: 'expired',
            chainValid: false,
            hostnameValid: true,
            selfSigned: false,
          },
        },
      }),
      resource({
        id: 'chk-http-err',
        type: 'network-endpoint',
        name: 'API HTTP',
        availability: {
          protocol: 'http',
          applicationStatusCode: 503,
          certificate: {
            observedAt: '2026-11-27T00:00:00Z',
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: '2027-01-01T00:00:00Z',
            fingerprintSha256: 'abc',
            trustStatus: 'untrusted',
            chainValid: false,
            hostnameValid: true,
            selfSigned: false,
          },
        },
      }),
      resource({
        id: 'chk-hostname-bad',
        type: 'network-endpoint',
        name: 'Bad Hostname',
        availability: {
          protocol: 'https',
          certificate: {
            observedAt: '2026-11-27T00:00:00Z',
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: '2027-01-01T00:00:00Z',
            fingerprintSha256: 'abc',
            trustStatus: 'trusted',
            chainValid: true,
            hostnameValid: false,
            selfSigned: false,
          },
        },
      }),
      resource({
        id: 'chk-expiring',
        type: 'network-endpoint',
        name: 'Expiring Cert',
        availability: {
          protocol: 'https',
          certificateMonitoring: true,
          certificateExpiryWarningDays: 30,
          certificate: {
            observedAt: '2026-11-27T00:00:00Z',
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: soonIso,
            fingerprintSha256: 'abc',
            trustStatus: 'trusted',
            chainValid: true,
            hostnameValid: true,
            selfSigned: false,
          },
        },
      }),
      resource({
        id: 'chk-tcp',
        type: 'network-endpoint',
        name: 'DB Port',
        availability: { protocol: 'tcp', port: 443 },
      }),
      resource({
        id: 'chk-icmp',
        type: 'network-endpoint',
        name: 'Router Ping',
        availability: { protocol: 'icmp' },
      }),
      resource({
        id: 'chk-push',
        type: 'network-endpoint',
        name: 'Cron Heartbeat',
        availability: { protocol: 'push' },
      }),
    ]);

    const dock = hostNamed(model.blocks, 'dock-1');
    const jf = dock.sections[0].items.find((item) => item.id === 'c-user-nets');
    const redis = dock.sections[0].items.find((item) => item.id === 'c-default-only');
    expect(jf?.dockerNetworks).toEqual(['media_net', 'proxy_net']);
    expect(redis?.dockerNetworks).toBeUndefined();

    const checks = hostNamed(model.blocks, 'Availability checks').sections[0].items;
    const byId = (id: string) => checks.find((item) => item.id === id);
    expect(byId('chk-https')).toMatchObject({
      linkedResourceId: 'c-user-nets',
      probeSummary: 'HTTPS 200',
      certWarning: 'Cert expired',
    });
    expect(byId('chk-http-err')).toMatchObject({
      probeSummary: 'HTTP 503',
      certWarning: 'Cert untrusted',
    });
    expect(byId('chk-hostname-bad')).toMatchObject({
      probeSummary: 'HTTPS',
      certWarning: 'Cert invalid',
    });
    expect(byId('chk-expiring')).toMatchObject({
      probeSummary: 'HTTPS',
      certWarning: 'Cert expiring soon',
    });
    expect(byId('chk-tcp')?.probeSummary).toBe('TCP :443');
    expect(byId('chk-icmp')?.probeSummary).toBe('ICMP');
    expect(byId('chk-push')?.probeSummary).toBe('Push');

    // Anonymization strips user-defined Docker networks while preserving opaque link IDs and factual probe/cert state.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonDock = hostsOf(anonymous.blocks).find((host) => host.id === 'dock-1')!;
    const anonJf = anonDock.sections[0].items.find((item) => item.id === 'c-user-nets');
    expect(anonJf?.dockerNetworks).toBeUndefined();
    expect(JSON.stringify(anonymous)).not.toContain('media_net');
    expect(JSON.stringify(anonymous)).not.toContain('proxy_net');

    const anonCheck = hostNamed(anonymous.blocks, 'Availability checks').sections[0].items.find(
      (item) => item.id === 'chk-https',
    );
    expect(anonCheck).toMatchObject({
      linkedResourceId: 'c-user-nets',
      probeSummary: 'HTTPS 200',
      certWarning: 'Cert expired',
    });
  });

  it('computes full-duplex link utilization, omitting sub-1% noise and clamping at 100%', () => {
    expect(MAP_LINK_SATURATED_PERCENT).toBe(80);
    expect(computeMapLinkUtilizationPercent(undefined, undefined, 1000)).toBeUndefined();
    expect(computeMapLinkUtilizationPercent(50_000_000, 0, undefined)).toBeUndefined();
    expect(computeMapLinkUtilizationPercent(50_000_000, 0, 0)).toBeUndefined();
    expect(computeMapLinkUtilizationPercent(50_000_000, 0, -1000)).toBeUndefined();
    expect(
      computeMapLinkUtilizationPercent(Number.NaN, Number.POSITIVE_INFINITY, 1000),
    ).toBeUndefined();
    expect(computeMapLinkUtilizationPercent(0, 0, 1000)).toBeUndefined();
    // Sub-1% on 10G (12 MB/s = 0.96%) and 1G (1 MB/s = 0.8%) stays omitted.
    expect(computeMapLinkUtilizationPercent(12_000_000, 5_000_000, 10000)).toBeUndefined();
    expect(computeMapLinkUtilizationPercent(1_000_000, 0, 1000)).toBeUndefined();
    // Exact 1% boundary on 10G: 12.5 MB/s.
    expect(computeMapLinkUtilizationPercent(12_500_000, 0, 10000)).toBe(1);
    // Full-duplex peak uses max(rx, tx), never rx + tx.
    expect(computeMapLinkUtilizationPercent(50_000_000, 50_000_000, 1000)).toBe(40);
    // Saturated threshold at 80% (100 MB/s on 1G).
    expect(computeMapLinkUtilizationPercent(100_000_000, 10_000_000, 1000)).toBe(80);
    // Clamps above line rate to 100%.
    expect(computeMapLinkUtilizationPercent(0, 150_000_000, 1000)).toBe(100);

    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-busy',
        type: 'agent',
        name: 'busy-host',
        platformType: 'proxmox-pve',
        network: { rxBytes: 105_000_000, txBytes: 20_000_000 },
        agent: {
          networkInterfaces: [{ name: 'eno1', speedMbps: 1000, linkUp: true }],
        },
      }),
      resource({
        id: 'vm-busy',
        type: 'vm',
        name: 'busy-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-busy',
        status: 'running',
        network: { rxBytes: 25_000_000, txBytes: 62_500_000 },
        proxmox: {
          networkInterfaces: [{ name: 'net0', speedMbps: 1000, linkUp: true }],
        },
      }),
    ]);

    const host = hostNamed(model.blocks, 'busy-host');
    expect(host.linkUtilizationPercent).toBe(84);
    expect(host.sections[0].items[0].linkUtilizationPercent).toBe(50);

    // Anonymization preserves numeric utilization percentage.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonHost = hostsOf(anonymous.blocks).find((entry) => entry.id === 'pve-busy')!;
    expect(anonHost.linkUtilizationPercent).toBe(84);
    expect(anonHost.sections[0].items[0].linkUtilizationPercent).toBe(50);
  });

  it('extracts published ports from Docker containers, Swarm endpointPorts, and TrueNAS apps', () => {
    const model = buildInfrastructureMapModel([
      resource({ id: 'dock-1', type: 'docker-host', name: 'dock-1', platformType: 'docker' }),
      resource({
        id: 'c-swarm',
        type: 'app-container',
        name: 'gateway-svc',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          ports: [{ publicPort: 8080, privatePort: 80 }],
          endpointPorts: [
            { publishedPort: 443, targetPort: 443, protocol: 'tcp' },
            { publishedPort: 8080, targetPort: 8080, protocol: 'tcp' },
          ],
        },
      }),
      resource({ id: 'nas-1', type: 'agent', name: 'truenas-box', platformType: 'truenas' }),
      resource({
        id: 'tn-app',
        type: 'app-container',
        name: 'immich',
        platformType: 'truenas',
        parentId: 'nas-1',
        status: 'running',
        truenas: {
          app: {
            usedPorts: [
              { containerPort: 2283, hostPorts: [{ hostPort: 30041, hostIp: '0.0.0.0' }] },
              { containerPort: 80, hostPorts: [{ hostPort: 8081 }] },
            ],
            containers: [
              {
                serviceName: 'server',
                portConfig: [
                  { containerPort: 443, hostPorts: [{ hostPort: 8443 }, { hostPort: 30041 }] },
                  { containerPort: 9090, hostPorts: [{ hostPort: 9090 }] },
                ],
              },
            ],
          },
        },
      }),
    ]);

    const dock = hostNamed(model.blocks, 'dock-1');
    expect(dock.sections[0].items[0].detail).toBe(':443, 8080');

    const nas = hostNamed(model.blocks, 'truenas-box');
    expect(nas.sections[0].items[0].detail).toBe(':8081, 8443, 9090 +1');

    // Numeric published ports stay preserved when names are hidden.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonNas = hostsOf(anonymous.blocks).find((entry) => entry.id === 'nas-1')!;
    expect(anonNas.sections[0].items[0].detail).toBe(':8081, 8443, 9090 +1');
  });

  it('matches Kubernetes Services and Ingresses factually by cluster, namespace, and selector, and strips Ingress hosts when anonymized', () => {
    const model = buildInfrastructureMapModel([
      resource({ id: 'k8s-a', type: 'k8s-cluster', name: 'prod-k8s', platformType: 'kubernetes' }),
      resource({
        id: 'k8s-b',
        type: 'k8s-cluster',
        name: 'staging-k8s',
        platformType: 'kubernetes',
      }),
      resource({
        id: 'pod-web',
        type: 'pod',
        name: 'web-7c9d',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        status: 'running',
        kubernetes: {
          namespace: 'apps',
          labels: { app: 'web', tier: 'frontend' },
        },
      }),
      resource({
        id: 'pod-partial',
        type: 'pod',
        name: 'web-worker-1',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        status: 'running',
        kubernetes: {
          namespace: 'apps',
          labels: { app: 'web' },
        },
      }),
      resource({
        id: 'pod-default-ns',
        type: 'pod',
        name: 'metrics-0',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        status: 'running',
        kubernetes: {
          labels: { app: 'metrics' },
        },
      }),
      resource({
        id: 'pod-other-cluster',
        type: 'pod',
        name: 'web-staging',
        platformType: 'kubernetes',
        parentId: 'k8s-b',
        status: 'running',
        kubernetes: {
          namespace: 'apps',
          labels: { app: 'web', tier: 'frontend' },
        },
      }),
      // LoadBalancer service matching pod-web (externalIps must never leak into detail).
      resource({
        id: 'svc-web-lb',
        type: 'k8s-service',
        name: 'web-svc',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        kubernetes: {
          namespace: 'apps',
          serviceType: 'LoadBalancer',
          externalIps: ['203.0.113.42'],
          servicePorts: [
            { port: 443, targetPort: '8443', nodePort: 31443 },
            { port: 80, targetPort: '8080', nodePort: 31080 },
          ],
          selector: { app: 'web', tier: 'frontend' },
        },
      }),
      // ClusterIP service matching pod-web (no published ports, but matched by Ingress).
      resource({
        id: 'svc-web-internal',
        type: 'k8s-service',
        name: 'web-internal',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        kubernetes: {
          namespace: 'apps',
          serviceType: 'ClusterIP',
          servicePorts: [{ port: 9000, targetPort: '9000' }],
          selector: { app: 'web', tier: 'frontend' },
        },
      }),
      // NodePort service in default namespace matching pod-default-ns.
      resource({
        id: 'svc-metrics-np',
        type: 'k8s-service',
        name: 'metrics-np',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        kubernetes: {
          serviceType: 'NodePort',
          servicePorts: [{ port: 9090, nodePort: 30090 }],
          selector: { app: 'metrics' },
        },
      }),
      // Ingress routing to web-internal in same cluster + namespace.
      resource({
        id: 'ing-web',
        type: 'k8s-ingress',
        name: 'web-ingress',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        kubernetes: {
          namespace: 'apps',
          serviceName: 'web-internal',
          hosts: ['WWW.Shop.Example.Test', 'shop.example.test', 'shop.example.test', '*'],
        },
      }),
      // Ingress in another namespace must NOT match pod-web.
      resource({
        id: 'ing-wrong-ns',
        type: 'k8s-ingress',
        name: 'other-ingress',
        platformType: 'kubernetes',
        parentId: 'k8s-a',
        kubernetes: {
          namespace: 'other',
          serviceName: 'web-internal',
          hosts: ['wrong-ns.example.test'],
        },
      }),
    ]);

    const prod = hostNamed(model.blocks, 'prod-k8s');
    const appsSection = prod.sections.find((section) => section.label === 'apps')!;
    const defaultSection = prod.sections.find((section) => section.label === 'default')!;
    const webPod = appsSection.items.find((item) => item.id === 'pod-web')!;
    const partialPod = appsSection.items.find((item) => item.id === 'pod-partial')!;
    const metricsPod = defaultSection.items.find((item) => item.id === 'pod-default-ns')!;

    expect(webPod.detail).toBe(':80, 443');
    expect(webPod.detail).not.toContain('203.0.113.42');
    expect(webPod.ingressHosts).toEqual(['shop.example.test', 'www.shop.example.test']);
    expect(partialPod.detail).toBeUndefined();
    expect(partialPod.ingressHosts).toBeUndefined();
    expect(metricsPod.detail).toBe(':30090');
    expect(metricsPod.ingressHosts).toBeUndefined();

    const staging = hostNamed(model.blocks, 'staging-k8s');
    const stagingPod = staging.sections[0].items[0];
    expect(stagingPod.detail).toBeUndefined();
    expect(stagingPod.ingressHosts).toBeUndefined();

    // Anonymization strips Ingress hostnames while preserving numeric Service ports.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonProd = hostsOf(anonymous.blocks).find((host) => host.id === 'k8s-a')!;
    const anonWebPod = anonProd.sections
      .flatMap((section) => section.items)
      .find((item) => item.id === 'pod-web')!;
    expect(anonWebPod.ingressHosts).toBeUndefined();
    expect(anonWebPod.detail).toBe(':80, 443');
    expect(JSON.stringify(anonymous)).not.toContain('shop.example.test');
    expect(JSON.stringify(anonymous)).not.toContain('203.0.113.42');
  });

  it('extracts host hardware specs, version labels, VMIDs, Compose projects, and pool metadata, and strips names under Hide names', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-node',
        type: 'agent',
        name: 'rack-pve',
        platformType: 'proxmox-pve',
        proxmox: {
          cpus: 16,
          pveVersion: 'pve-manager/8.3.1/c1689ccb1065a83b',
          memory: { total: 64 * GIB },
        },
        agent: { cpuCount: 8, memory: { total: 32 * GIB } },
      }),
      resource({
        id: 'vm-102',
        type: 'vm',
        name: 'gitlab-runner',
        platformType: 'proxmox-pve',
        parentId: 'pve-node',
        status: 'running',
        proxmox: { vmid: 102 },
      }),
      resource({
        id: 'ct-bad-vmid',
        type: 'system-container',
        name: 'scratch',
        platformType: 'proxmox-pve',
        parentId: 'pve-node',
        status: 'running',
        proxmox: { vmid: 0 },
      }),
      resource({
        id: 'nvme-pool',
        type: 'storage',
        name: 'nvme-pool',
        platformType: 'proxmox-pve',
        parentId: 'pve-node',
        disk: { current: 62, total: 4000 * GIB, used: 2480 * GIB },
        storage: { vdevLayout: 'mirror', type: 'zfspool' },
      }),
      resource({
        id: 'dock-host',
        type: 'docker-host',
        name: 'docker-box',
        platformType: 'docker',
        docker: {
          cpus: 12,
          dockerVersion: '27.3.1',
          memory: { total: 32 * GIB },
        },
      }),
      resource({
        id: 'c-media-1',
        type: 'app-container',
        name: 'sonarr',
        platformType: 'docker',
        parentId: 'dock-host',
        status: 'running',
        docker: { composeProject: '  media-stack  ' },
      }),
      resource({
        id: 'c-blank-project',
        type: 'app-container',
        name: 'redis',
        platformType: 'docker',
        parentId: 'dock-host',
        status: 'running',
        docker: { composeProject: '   ' },
      }),
      resource({
        id: 'pbs-box',
        type: 'pbs',
        name: 'pbs-main',
        platformType: 'proxmox-pbs',
        pbs: { version: '3.2.7' },
        memory: { current: 25, total: 16 * GIB, used: 4 * GIB },
      }),
      resource({
        id: 'linux-box',
        type: 'agent',
        name: 'edge-linux',
        platformType: 'agent',
        agent: {
          cpuCount: 4,
          osName: 'Debian GNU/Linux',
          osVersion: '12 (bookworm)',
          memory: { total: 8 * GIB },
        },
      }),
      resource({
        id: 'long-os-box',
        type: 'agent',
        name: 'enterprise-box',
        platformType: 'agent',
        agent: {
          osName: 'Red Hat Enterprise Linux',
          osVersion: '9.4',
        },
      }),
    ]);

    const pve = hostNamed(model.blocks, 'rack-pve');
    expect(pve).toMatchObject({
      cpuCores: 16,
      memoryTotalBytes: 64 * GIB,
      versionLabel: 'PVE 8.3',
    });
    expect(pve.sections[0].items[0].vmid).toBe(102);
    expect(pve.sections[1].items[0].vmid).toBeUndefined();
    expect(pve.pools[0]).toMatchObject({
      vdevLayout: 'mirror',
      storageType: 'zfspool',
    });

    const dock = hostNamed(model.blocks, 'docker-box');
    expect(dock).toMatchObject({
      cpuCores: 12,
      memoryTotalBytes: 32 * GIB,
      versionLabel: 'Docker 27.3',
    });
    expect(dock.sections[0].items.find((item) => item.id === 'c-media-1')?.composeProject).toBe(
      'media-stack',
    );
    expect(
      dock.sections[0].items.find((item) => item.id === 'c-blank-project')?.composeProject,
    ).toBeUndefined();

    expect(hostNamed(model.blocks, 'pbs-main')).toMatchObject({
      memoryTotalBytes: 16 * GIB,
      versionLabel: 'PBS 3.2',
    });
    expect(hostNamed(model.blocks, 'edge-linux')).toMatchObject({
      cpuCores: 4,
      memoryTotalBytes: 8 * GIB,
      versionLabel: 'Debian 12',
    });
    const longOsLabel = hostNamed(model.blocks, 'enterprise-box').versionLabel;
    expect(longOsLabel).toBeDefined();
    expect(longOsLabel!.length).toBeLessThanOrEqual(18);

    // Hide names strips composeProject and vmid while preserving hardware specs, versions, and pool layouts.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonPve = hostsOf(anonymous.blocks).find((host) => host.id === 'pve-node')!;
    const anonDock = hostsOf(anonymous.blocks).find((host) => host.id === 'dock-host')!;
    expect(anonPve).toMatchObject({
      cpuCores: 16,
      memoryTotalBytes: 64 * GIB,
      versionLabel: 'PVE 8.3',
    });
    expect(anonPve.sections[0].items[0].vmid).toBeUndefined();
    expect(anonPve.pools[0]).toMatchObject({
      vdevLayout: 'mirror',
      storageType: 'zfspool',
    });
    expect(
      anonDock.sections[0].items.find((item) => item.id === 'c-media-1')?.composeProject,
    ).toBeUndefined();
    expect(JSON.stringify(anonymous)).not.toContain('media-stack');
  });

  it('extracts host temperature across Proxmox, Docker, universal, and agent sensors while ignoring invalid values', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-temp',
        type: 'agent',
        name: 'pve-hot',
        platformType: 'proxmox-pve',
        proxmox: { temperature: 48.4 },
      }),
      resource({
        id: 'dock-temp',
        type: 'docker-host',
        name: 'docker-hot',
        platformType: 'docker',
        docker: { temperature: 61.6 },
      }),
      resource({
        id: 'agent-temp',
        type: 'agent',
        name: 'linux-sensors',
        platformType: 'agent',
        proxmox: { temperature: 0 },
        agent: {
          sensors: {
            temperatureCelsius: {
              cpu_package: 54.2,
              core_0: 57.6,
              cold: 0,
              sub_one: 0.5,
              wild: 151,
              broken: Number.NaN,
            },
          },
        },
      }),
      resource({
        id: 'universal-temp',
        type: 'agent',
        name: 'universal-box',
        platformType: 'agent',
        temperature: 43.3,
      }),
      resource({
        id: 'invalid-temp',
        type: 'agent',
        name: 'bad-sensors',
        platformType: 'proxmox-pve',
        proxmox: { temperature: Number.POSITIVE_INFINITY },
        temperature: -4,
        agent: {
          sensors: {
            temperatureCelsius: { zero: 0, negative: -10, over: 160, nan: Number.NaN },
          },
        },
      }),
    ]);

    expect(hostNamed(model.blocks, 'pve-hot').temperatureC).toBe(48);
    expect(hostNamed(model.blocks, 'docker-hot').temperatureC).toBe(62);
    expect(hostNamed(model.blocks, 'linux-sensors').temperatureC).toBe(58);
    expect(hostNamed(model.blocks, 'universal-box').temperatureC).toBe(43);
    expect(hostNamed(model.blocks, 'bad-sensors').temperatureC).toBeUndefined();
  });

  it('summarizes child physical disks, max disk temperature, and unhealthy count without altering totals or anonymization safety', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-1',
        type: 'agent',
        name: 'secret-pve',
        platformType: 'proxmox-pve',
        proxmox: { temperature: 51.2 },
      }),
      resource({
        id: 'disk-nvme-1',
        type: 'physical_disk',
        name: 'Samsung 990 Pro',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'online',
        physicalDisk: {
          diskType: 'NVMe',
          model: 'Samsung-990-PRO-Secret',
          serial: 'SN-SECRET-001',
          temperature: 46.6,
          health: 'PASSED',
        },
      }),
      resource({
        id: 'disk-nvme-2',
        type: 'physical_disk',
        name: 'WD Black',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'online',
        physicalDisk: { diskType: 'nvme', temperature: 121, health: 'OK' },
      }),
      ...Array.from({ length: 4 }, (_, index) =>
        resource({
          id: `disk-hdd-${index}`,
          type: 'physical_disk',
          name: `Seagate Exos ${index}`,
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          status: index === 0 ? 'degraded' : 'online',
          physicalDisk: {
            diskType: index % 2 === 0 ? 'HDD' : 'hdd',
            temperature: index === 1 ? 39.2 : 0,
            health: index === 2 ? 'PREFAIL' : 'PASSED',
          },
        }),
      ),
      resource({
        id: 'nas-1',
        type: 'agent',
        name: 'generic-nas',
        platformType: 'truenas',
      }),
      resource({
        id: 'pool-1',
        type: 'storage',
        name: 'tank',
        platformType: 'truenas',
        parentId: 'nas-1',
      }),
      resource({
        id: 'generic-1',
        type: 'physical_disk',
        name: 'sda',
        platformType: 'truenas',
        parentId: 'pool-1',
        status: 'online',
        physicalDisk: { diskType: 'sata', temperature: 34, health: 'PASSED' },
      }),
      resource({
        id: 'generic-2',
        type: 'physical_disk',
        name: 'sdb',
        platformType: 'truenas',
        parentId: 'pool-1',
        status: 'online',
        physicalDisk: { diskType: 'sas', temperature: 36.4, health: 'warning' },
      }),
      resource({
        id: 'generic-3',
        type: 'physical_disk',
        name: 'sdc',
        platformType: 'truenas',
        parentId: 'nas-1',
        status: 'online',
        physicalDisk: { temperature: Number.NaN },
      }),
      resource({
        id: 'mixed-host',
        type: 'agent',
        name: 'mixed-box',
        platformType: 'agent',
      }),
      resource({
        id: 'mix-ssd',
        type: 'physical_disk',
        name: 'ssd-0',
        parentId: 'mixed-host',
        physicalDisk: { diskType: 'ssd', health: 'OK' },
      }),
      resource({
        id: 'mix-other',
        type: 'physical_disk',
        name: 'raw-0',
        parentId: 'mixed-host',
        physicalDisk: { diskType: 'usb', health: 'OK' },
      }),
    ]);

    const pve = hostNamed(model.blocks, 'secret-pve');
    expect(pve).toMatchObject({
      temperatureC: 51,
      diskSummary: '2× NVMe · 4× HDD',
      diskMaxTempC: 47,
      unhealthyDiskCount: 2,
    });
    expect(pve.sections).toEqual([]);

    const nas = hostNamed(model.blocks, 'generic-nas');
    expect(nas).toMatchObject({
      diskSummary: '3 disks',
      diskMaxTempC: 36,
      unhealthyDiskCount: 1,
    });

    const mixed = hostNamed(model.blocks, 'mixed-box');
    expect(mixed.diskSummary).toBe('1× SSD · 1× Disk');
    expect(mixed.diskMaxTempC).toBeUndefined();
    expect(mixed.unhealthyDiskCount).toBeUndefined();

    // Physical disks do not perturb model.totals or model.attentionCount.
    expect(Object.fromEntries(model.totals.map((total) => [total.label, total.count]))).toEqual({
      Hosts: 3,
      Storage: 1,
    });
    expect(model.attentionCount).toBe(0);

    // Anonymization preserves generic thermal and disk summaries while revealing no names or serials.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonPve = hostsOf(anonymous.blocks).find((host) => host.id === 'pve-1')!;
    const anonNas = hostsOf(anonymous.blocks).find((host) => host.id === 'nas-1')!;
    expect(anonPve).toMatchObject({
      name: 'Proxmox 1',
      temperatureC: 51,
      diskSummary: '2× NVMe · 4× HDD',
      diskMaxTempC: 47,
      unhealthyDiskCount: 2,
    });
    expect(anonNas).toMatchObject({
      name: 'TrueNAS 1',
      diskSummary: '3 disks',
      diskMaxTempC: 36,
      unhealthyDiskCount: 1,
    });
    const serialized = JSON.stringify(anonymous);
    expect(serialized).not.toContain('secret-pve');
    expect(serialized).not.toContain('Samsung-990-PRO-Secret');
    expect(serialized).not.toContain('SN-SECRET-001');
  });

  it('extracts uptime, pending updates, backup recency, and ZFS pool anomalies while preserving them under Hide names', () => {
    const backupIso = '2026-11-26T03:30:00Z';
    const model = buildInfrastructureMapModel([
      resource({
        id: 'pve-up',
        type: 'agent',
        name: 'secret-pve-1',
        platformType: 'proxmox-pve',
        status: 'online',
        proxmox: { uptime: 1_209_600.6, pendingUpdates: 7 },
      }),
      resource({
        id: 'pve-down',
        type: 'agent',
        name: 'pve-offline',
        platformType: 'proxmox-pve',
        status: 'offline',
        proxmox: { uptime: 86_400, pendingUpdates: 0 },
      }),
      resource({
        id: 'dock-up',
        type: 'docker-host',
        name: 'secret-docker',
        platformType: 'docker',
        status: 'online',
        docker: { uptimeSeconds: 7_200.4, updatesAvailableCount: 4 },
      }),
      resource({
        id: 'agent-up',
        type: 'agent',
        name: 'linux-agent',
        platformType: 'agent',
        status: 'online',
        agent: {
          uptimeSeconds: 172_800.2,
          packageUpdates: { supported: true, pendingCount: 12 },
        },
      }),
      resource({
        id: 'agent-unsupported',
        type: 'agent',
        name: 'unsupported-agent',
        platformType: 'agent',
        status: 'online',
        agent: {
          uptimeSeconds: -10,
          packageUpdates: { supported: false, pendingCount: 9 },
        },
      }),
      resource({
        id: 'pbs-up',
        type: 'pbs',
        name: 'pbs-vault',
        platformType: 'proxmox-pbs',
        status: 'online',
        pbs: { uptimeSeconds: 43_200 },
      }),
      resource({
        id: 'vm-running',
        type: 'vm',
        name: 'secret-db',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        status: 'running',
        proxmox: {
          uptime: 345_600.4,
          pendingUpdates: 3,
          lastBackup: `  ${backupIso}  `,
        },
      }),
      resource({
        id: 'vm-stopped',
        type: 'vm',
        name: 'stopped-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        status: 'stopped',
        proxmox: {
          uptime: 9_999,
          pendingUpdates: 1.5,
          lastBackup: 'not-a-valid-date',
        },
      }),
      resource({
        id: 'vm-critical',
        type: 'vm',
        name: 'critical-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        status: 'running',
        health: { verdict: 'critical', reasons: [{ code: 'unreachable' }] },
        proxmox: { uptime: 12_345, lastBackup: '   ' },
      }),
      resource({
        id: 'c-update',
        type: 'app-container',
        name: 'grafana',
        platformType: 'docker',
        parentId: 'dock-up',
        status: 'running',
        docker: {
          uptimeSeconds: 14_400.7,
          updateStatus: { updateAvailable: true },
        },
      }),
      resource({
        id: 'pool-healthy',
        type: 'storage',
        name: 'tank-ok',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        storage: {
          type: 'zfspool',
          zfsPoolState: 'online',
          syncAction: 'scrub',
          zfsReadErrors: 0,
          zfsWriteErrors: 0,
          zfsChecksumErrors: 0,
        },
      }),
      resource({
        id: 'pool-degraded',
        type: 'storage',
        name: 'secret-tank-deg',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        storage: {
          type: 'zfspool',
          zfsPoolState: ' degraded ',
          rebuildInProgress: true,
          zfsReadErrors: 1,
          zfsWriteErrors: 2,
          zfsChecksumErrors: 4,
        },
      }),
      resource({
        id: 'pool-array-recover',
        type: 'storage',
        name: 'array-rebuild',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        storage: {
          arrayState: 'faulted',
          syncAction: 'Resilvering',
          zfsReadErrors: -5,
          zfsWriteErrors: Number.NaN,
          zfsChecksumErrors: 3,
        },
      }),
      resource({
        id: 'pool-array-healthy',
        type: 'storage',
        name: 'array-started',
        platformType: 'proxmox-pve',
        parentId: 'pve-up',
        storage: {
          arrayState: 'Started',
          syncAction: 'idle',
        },
      }),
    ]);

    const pveUp = hostNamed(model.blocks, 'secret-pve-1');
    expect(pveUp.uptimeSeconds).toBe(1_209_601);
    expect(pveUp.pendingUpdates).toBe(7);

    const pveDown = hostNamed(model.blocks, 'pve-offline');
    expect(pveDown.uptimeSeconds).toBeUndefined();
    expect(pveDown.pendingUpdates).toBeUndefined();

    const dockUp = hostNamed(model.blocks, 'secret-docker');
    expect(dockUp.uptimeSeconds).toBe(7_200);
    expect(dockUp.pendingUpdates).toBe(4);

    const agentUp = hostNamed(model.blocks, 'linux-agent');
    expect(agentUp.uptimeSeconds).toBe(172_800);
    expect(agentUp.pendingUpdates).toBe(12);

    const agentUnsupported = hostNamed(model.blocks, 'unsupported-agent');
    expect(agentUnsupported.uptimeSeconds).toBeUndefined();
    expect(agentUnsupported.pendingUpdates).toBeUndefined();

    expect(hostNamed(model.blocks, 'pbs-vault').uptimeSeconds).toBe(43_200);

    const vmRunning = pveUp.sections[0].items.find((item) => item.id === 'vm-running')!;
    expect(vmRunning).toMatchObject({
      uptimeSeconds: 345_600,
      pendingUpdates: 3,
      lastBackupAt: backupIso,
    });

    const vmStopped = pveUp.sections[0].items.find((item) => item.id === 'vm-stopped')!;
    expect(vmStopped.uptimeSeconds).toBeUndefined();
    expect(vmStopped.pendingUpdates).toBeUndefined();
    expect(vmStopped.lastBackupAt).toBeUndefined();

    const vmCritical = pveUp.sections[0].items.find((item) => item.id === 'vm-critical')!;
    expect(vmCritical.uptimeSeconds).toBeUndefined();
    expect(vmCritical.lastBackupAt).toBeUndefined();

    const container = dockUp.sections[0].items.find((item) => item.id === 'c-update')!;
    expect(container.uptimeSeconds).toBe(14_401);
    expect(container.pendingUpdates).toBe(1);

    const poolById = (id: string) => pveUp.pools.find((pool) => pool.id === id)!;
    expect(poolById('pool-healthy').zfsState).toBeUndefined();
    expect(poolById('pool-healthy').rebuildInProgress).toBeUndefined();
    expect(poolById('pool-healthy').zfsErrorCount).toBeUndefined();

    expect(poolById('pool-array-healthy').zfsState).toBeUndefined();
    expect(poolById('pool-array-healthy').rebuildInProgress).toBeUndefined();

    expect(poolById('pool-degraded')).toMatchObject({
      zfsState: 'DEGRADED',
      rebuildInProgress: true,
      zfsErrorCount: 7,
    });

    expect(poolById('pool-array-recover')).toMatchObject({
      zfsState: 'FAULTED',
      rebuildInProgress: true,
      zfsErrorCount: 3,
    });

    // Anonymization preserves all operational telemetry while stripping names.
    const anonymous = anonymizeInfrastructureMapModel(model);
    const anonPve = hostsOf(anonymous.blocks).find((host) => host.id === 'pve-up')!;
    expect(anonPve).toMatchObject({
      uptimeSeconds: 1_209_601,
      pendingUpdates: 7,
    });
    expect(anonPve.sections[0].items.find((item) => item.id === 'vm-running')).toMatchObject({
      uptimeSeconds: 345_600,
      pendingUpdates: 3,
      lastBackupAt: backupIso,
    });
    expect(anonPve.pools.find((pool) => pool.id === 'pool-degraded')).toMatchObject({
      zfsState: 'DEGRADED',
      rebuildInProgress: true,
      zfsErrorCount: 7,
    });
    const serialized = JSON.stringify(anonymous);
    expect(serialized).not.toContain('secret-pve-1');
    expect(serialized).not.toContain('secret-db');
    expect(serialized).not.toContain('secret-tank-deg');
  });

  it('normalizes raw darwin OS names to macOS in host version labels', () => {
    const model = buildInfrastructureMapModel([
      resource({
        id: 'mac-mini',
        type: 'agent',
        name: 'mac-mini.local',
        platformType: 'generic',
        agent: { osName: 'darwin', osVersion: '27.0.1' },
      }),
    ]);
    expect(hostNamed(model.blocks, 'mac-mini.local').versionLabel).toBe('macOS');
  });
});
