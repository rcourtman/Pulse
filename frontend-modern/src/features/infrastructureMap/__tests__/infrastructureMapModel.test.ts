import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  MAP_TRAFFIC_ACTIVE_BYTES,
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
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
});
