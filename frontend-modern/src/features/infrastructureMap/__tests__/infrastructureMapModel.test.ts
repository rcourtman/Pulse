import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
  resolveMapHealth,
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
    name: 'Router',
    status: 'offline',
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
    expect(checks.sections[0].items.map((item) => item.name)).toEqual(['Router']);
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
      'Router',
      'mini-pc',
    ]) {
      expect(text).not.toContain(name);
    }

    const node = hostsOf(anonymous.blocks)[0];
    expect(node.name).toBe('Proxmox node 1');
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
    expect(hostNamed(anonymous.blocks, 'Availability checks').sections[0].items[0].name).toBe(
      'Check 1',
    );
  });
});
