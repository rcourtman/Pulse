import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  layoutInfrastructureMap,
  layoutInfrastructureNetworkMap,
  type MapScene,
  type SceneNode,
} from '../infrastructureMapLayout';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
  type MapHost,
} from '../infrastructureMapModel';
import {
  OTHER_SEGMENT_ID,
  parseMapAddress,
  resolveMapNetworks,
} from '../infrastructureMapNetworks';

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

const LAN = 'net:192.168.10.0/24';
const IOT = 'range:10.0.30';

const lab = (): Resource[] => [
  resource({
    id: 'pve',
    type: 'agent',
    name: 'pve',
    platformType: 'proxmox-pve',
    agent: {
      networkInterfaces: [
        { name: 'lo', addresses: ['127.0.0.1/8'] },
        { name: 'vmbr0', addresses: ['192.168.10.11/24', 'fe80::1/64'] },
      ],
    },
  }),
  resource({
    id: 'vm-lan',
    type: 'vm',
    name: 'nextcloud',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    identity: { ips: ['192.168.10.50'] },
  }),
  resource({
    id: 'vm-iot',
    type: 'vm',
    name: 'haos',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    proxmox: { networkInterfaces: [{ name: 'eth0', addresses: ['10.0.30.5'] }] },
    identity: { ips: ['10.0.30.5'] },
  }),
  resource({
    id: 'ct-iot',
    type: 'system-container',
    name: 'zigbee',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    identity: { ips: ['10.0.30.9'] },
  }),
  resource({
    id: 'vm-alone',
    type: 'vm',
    name: 'lab-vm',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    identity: { ips: ['10.9.9.9'] },
  }),
  resource({
    id: 'vm-silent',
    type: 'vm',
    name: 'no-agent',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
  }),
  resource({
    id: 'docker',
    type: 'docker-host',
    name: 'apps',
    platformType: 'docker',
    // Sent on the wire; the shared type does not list interfaces yet.
    docker: {
      networkInterfaces: [
        { name: 'eth0', addresses: ['192.168.10.20/24'] },
        { name: 'docker0', addresses: ['172.17.0.1/16'] },
      ],
    } as Resource['docker'],
    identity: { ips: ['192.168.10.20', '172.20.0.1'] },
  }),
  resource({
    id: 'net-proxy',
    type: 'docker-network',
    name: 'proxy',
    platformType: 'docker',
    parentId: 'docker',
    docker: { driver: 'bridge', subnets: [{ subnet: '172.20.0.0/24', gateway: '172.20.0.1' }] },
  }),
  resource({
    id: 'c-traefik',
    type: 'app-container',
    name: 'traefik',
    platformType: 'docker',
    parentId: 'docker',
    status: 'running',
    docker: {
      networks: [{ name: 'proxy' }, { name: 'media' }],
      ports: [
        { privatePort: 443, publicPort: 443, protocol: 'tcp' },
        { privatePort: 80, publicPort: 80, protocol: 'tcp' },
        { privatePort: 443, publicPort: 443, protocol: 'udp' },
      ],
    },
  }),
  resource({
    id: 'c-jellyfin',
    type: 'app-container',
    name: 'jellyfin',
    platformType: 'docker',
    parentId: 'docker',
    status: 'running',
    docker: { networks: [{ name: 'media' }] },
  }),
  resource({ id: 'bare', type: 'agent', name: 'bare', platformType: 'agent' }),
];

describe('parseMapAddress', () => {
  it('reads IPv4 with or without a prefix and ignores what places nothing', () => {
    expect(parseMapAddress('192.168.10.5/24')).toMatchObject({ ip: '192.168.10.5', prefix: 24 });
    expect(parseMapAddress(' 10.0.0.9 ')).toMatchObject({ ip: '10.0.0.9', prefix: undefined });
    // A /32 names one machine, not a network other machines can share.
    expect(parseMapAddress('10.0.0.9/32')).toMatchObject({ ip: '10.0.0.9', prefix: undefined });
    for (const value of [
      '127.0.0.1/8',
      '169.254.3.4',
      '0.0.0.0',
      'fe80::1/64',
      'nas.lan',
      '',
      '300.1.1.1',
    ]) {
      expect(parseMapAddress(value)).toBeNull();
    }
  });
});

describe('resolveMapNetworks', () => {
  it('names a subnet from a reported prefix and joins prefix-less addresses inside it', () => {
    const { segments, attachments } = resolveMapNetworks(lab());
    expect(segments[0]).toMatchObject({
      id: LAN,
      label: '192.168.10.0/24',
      kind: 'subnet',
      memberCount: 3,
      colorIndex: 0,
    });
    expect(attachments.get('vm-lan')).toEqual([{ segmentId: LAN, address: '192.168.10.50' }]);
    expect(attachments.get('pve')).toEqual([{ segmentId: LAN, address: '192.168.10.11' }]);
  });

  it('groups unproven addresses by range and labels the group as a range', () => {
    const { segments, attachments } = resolveMapNetworks(lab());
    expect(segments[1]).toMatchObject({
      id: IOT,
      label: '10.0.30.x',
      kind: 'range',
      memberCount: 2,
    });
    expect(attachments.get('vm-iot')).toEqual([{ segmentId: IOT, address: '10.0.30.5' }]);
  });

  it('does not make a network out of an address nothing else shares', () => {
    const { segments, attachments } = resolveMapNetworks(lab());
    expect(attachments.get('vm-alone')).toEqual([
      { segmentId: OTHER_SEGMENT_ID, address: '10.9.9.9' },
    ]);
    expect(segments.at(-1)).toMatchObject({ id: OTHER_SEGMENT_ID, kind: 'other', colorIndex: -1 });
    expect(segments).toHaveLength(3);
  });

  it('ignores a host’s own container bridges and places no container or silent guest', () => {
    const { attachments } = resolveMapNetworks(lab());
    expect(attachments.get('docker')).toEqual([{ segmentId: LAN, address: '192.168.10.20' }]);
    expect(attachments.has('c-traefik')).toBe(false);
    expect(attachments.has('vm-silent')).toBe(false);
    expect(attachments.has('bare')).toBe(false);
  });

  it('keeps the LAN when a macvlan or ipvlan container network shares its subnet', () => {
    for (const driver of ['macvlan', 'ipvlan']) {
      const { segments, attachments } = resolveMapNetworks([
        ...lab(),
        resource({
          id: `net-${driver}`,
          type: 'docker-network',
          name: 'eth0',
          platformType: 'docker',
          parentId: 'docker',
          docker: { driver, subnets: [{ subnet: '192.168.10.0/24', gateway: '192.168.10.1' }] },
        }),
      ]);
      expect(segments.find((segment) => segment.id === LAN)).toMatchObject({ memberCount: 3 });
      expect(attachments.get('pve')).toEqual([{ segmentId: LAN, address: '192.168.10.11' }]);
      expect(attachments.get('vm-lan')).toEqual([{ segmentId: LAN, address: '192.168.10.50' }]);
    }
  });

  it('drops a bridge address that the identity list repeats without its interface', () => {
    const { attachments } = resolveMapNetworks([
      ...lab(),
      resource({
        id: 'ct-docker',
        type: 'system-container',
        name: 'paperless',
        platformType: 'proxmox-pve',
        parentId: 'pve',
        status: 'running',
        proxmox: {
          networkInterfaces: [
            { name: 'br-be5db79246da', addresses: ['172.18.0.1'] },
            { name: 'podman0', addresses: ['10.88.0.1'] },
            { name: 'eth0', addresses: ['192.168.10.76'] },
          ],
        },
        identity: { ips: ['192.168.10.76', '172.18.0.1', '10.88.0.1'] },
      }),
    ]);
    expect(attachments.get('ct-docker')).toEqual([{ segmentId: LAN, address: '192.168.10.76' }]);
  });

  it('places a vSphere VM on the port group its adapter is plugged into', () => {
    const vm = resource({
      id: 'vsphere-vm',
      type: 'vm',
      name: 'web',
      platformType: 'vmware-vsphere',
      vmware: {
        connectionId: 'vc1',
        guestIpAddresses: ['192.168.10.77'],
        networkAdapters: [{ networkName: 'DMZ' }, { networkName: 'DMZ' }],
      },
    });
    const { segments, attachments } = resolveMapNetworks([...lab(), vm]);
    expect(attachments.get('vsphere-vm')).toEqual([
      { segmentId: 'pg:vc1:DMZ', address: '192.168.10.77' },
    ]);
    expect(segments.find((segment) => segment.id === 'pg:vc1:DMZ')).toMatchObject({
      label: 'DMZ',
      kind: 'portgroup',
    });
    // The port group stands in for the address; the LAN gains no member.
    expect(segments.find((segment) => segment.id === LAN)?.memberCount).toBe(3);
  });
});

const hostNamed = (hosts: MapHost[], name: string): MapHost => {
  const host = hosts.find((candidate) => candidate.name === name);
  if (!host) throw new Error(`no host named ${name}`);
  return host;
};
const hostsOf = (resources: Resource[]): MapHost[] =>
  buildInfrastructureMapModel(resources).blocks.flatMap((block) =>
    block.kind === 'group' ? block.group.hosts : [block.host],
  );

describe('networks on the map model', () => {
  it('records where a host is and which networks only its guests are on', () => {
    const node = hostNamed(hostsOf(lab()), 'pve');
    expect(node.address).toBe('192.168.10.11');
    expect(node.ownSegmentIds).toEqual([LAN]);
    expect(node.guestSegmentIds).toEqual([IOT, OTHER_SEGMENT_ID]);
    const vms = node.sections[0].items;
    expect(vms.find((item) => item.name === 'haos')).toMatchObject({
      address: '10.0.30.5',
      segmentIds: [IOT],
    });
    expect(vms.find((item) => item.name === 'no-agent')?.segmentIds).toBeUndefined();
    expect(hostNamed(hostsOf(lab()), 'bare').ownSegmentIds).toEqual([]);
  });

  it('groups a Docker host’s containers by the container network they share', () => {
    const docker = hostNamed(hostsOf(lab()), 'apps');
    expect(docker.sections[0].items.map((item) => item.name)).toEqual(['jellyfin', 'traefik']);
    expect(
      docker.networkSections?.map((section) => [
        section.label,
        section.note,
        section.items.map((item) => item.name),
      ]),
    ).toEqual([
      ['media', undefined, ['jellyfin', 'traefik']],
      ['proxy', '172.20.0.0/24', ['traefik']],
    ]);
    expect(docker.sections[0].items[1].detail).toBe(':80, 443');
    expect(docker.sections[0].items[0].detail).toBeUndefined();
  });

  it('lists what a Kubernetes cluster exposes outside itself', () => {
    const cluster = hostNamed(
      hostsOf([
        resource({ id: 'k8s', type: 'k8s-cluster', name: 'k3s', platformType: 'kubernetes' }),
        resource({
          id: 'svc-in',
          type: 'k8s-service',
          name: 'internal',
          parentId: 'k8s',
          kubernetes: { serviceType: 'ClusterIP', servicePorts: [{ port: 80 }] },
        }),
        resource({
          id: 'svc-out',
          type: 'k8s-service',
          name: 'web',
          parentId: 'k8s',
          kubernetes: { serviceType: 'NodePort', servicePorts: [{ port: 443, nodePort: 30443 }] },
        }),
      ]),
      'k3s',
    );
    expect(cluster.sections).toHaveLength(1);
    expect(cluster.sections[0]).toMatchObject({ label: 'Exposed' });
    expect(cluster.sections[0].items).toMatchObject([{ name: 'web', detail: ':30443' }]);
  });

  it('hides addresses and network names with the rest when names are hidden', () => {
    const anonymous = anonymizeInfrastructureMapModel(buildInfrastructureMapModel(lab()));
    const text = JSON.stringify(anonymous, (key, value) =>
      key === 'id' || key.endsWith('egmentIds') || key === 'key' ? undefined : value,
    );
    for (const secret of [
      '192.168.10',
      '10.0.30',
      '10.9.9.9',
      '172.20',
      'proxy',
      'media',
      'traefik',
    ]) {
      expect(text).not.toContain(secret);
    }
    expect(anonymous.segments.map((segment) => segment.label)).toEqual([
      'Network 1',
      'Network 2',
      'Other addresses',
    ]);
    const docker = anonymous.blocks.flatMap((block) =>
      block.kind === 'host' ? [block.host] : [],
    )[1];
    // One container keeps one stand-in name under both of its networks.
    const names = docker.networkSections!.flatMap((section) =>
      section.items.map((item) => item.name),
    );
    expect(new Set(names).size).toBe(2);
    expect(names).toHaveLength(3);
  });
});

type Rect = Extract<SceneNode, { kind: 'rect' }>;
const rects = (scene: MapScene, fill: string): Rect[] =>
  scene.nodes.filter((node): node is Rect => node.kind === 'rect' && node.fill === fill);
const texts = (scene: MapScene): string[] =>
  scene.nodes.flatMap((node) => (node.kind === 'text' ? [node.text] : []));

describe('layoutInfrastructureNetworkMap', () => {
  const model = buildInfrastructureMapModel(lab());
  const scene = layoutInfrastructureNetworkMap(model, { width: 1200, density: 'names' });

  it('draws one full-width line per network, named, above the machines', () => {
    const cards = rects(scene, 'card');
    const firstCardTop = Math.min(...cards.map((card) => card.y));
    for (const tone of ['net0', 'net1', 'netOther']) {
      const line = rects(scene, tone).find((rect) => rect.w === scene.width);
      expect(line).toBeDefined();
      expect(line!.y).toBeLessThan(firstCardTop);
    }
    expect(texts(scene)).toEqual(
      expect.arrayContaining(['192.168.10.0/24', '10.0.30.x', 'Other addresses']),
    );
    expect(texts(scene)).toContain('address range · 2 addresses');
  });

  it('ties a machine to its own network and to each network its guests are on', () => {
    const cards = rects(scene, 'card');
    const node = cards[0];
    const tiesInto = (card: Rect, tone: string) =>
      rects(scene, tone).filter(
        (rect) => rect.h === 2 && Math.abs(rect.x + rect.w - card.x) < 0.01,
      );
    expect(tiesInto(node, 'net0')).toHaveLength(1);
    expect(tiesInto(node, 'net1')).toHaveLength(1);
    expect(tiesInto(node, 'netOther')).toHaveLength(1);
    const dockerCard = cards[1];
    expect(tiesInto(dockerCard, 'net0')).toHaveLength(1);
    expect(tiesInto(dockerCard, 'net1')).toHaveLength(0);
    // The machine with no address is drawn, tied to nothing.
    const bare = cards[2];
    for (const tone of ['net0', 'net1', 'netOther']) expect(tiesInto(bare, tone)).toHaveLength(0);
  });

  it('marks each guest with its network and writes its address beside it', () => {
    expect(texts(scene)).toEqual(
      expect.arrayContaining(['10.0.30.5', '192.168.10.50', ':80, 443']),
    );
    expect(texts(scene)).toContain('192.168.10.11 · Proxmox VE');
    // Section headings for the Docker host are its container networks.
    expect(texts(scene)).toEqual(expect.arrayContaining(['MEDIA', 'PROXY', '172.20.0.0/24']));
    const hosts = layoutInfrastructureMap(model, { width: 1200, density: 'names' });
    expect(texts(hosts)).not.toContain('10.0.30.5');
    expect(texts(hosts)).toContain('CONTAINERS');
  });

  it('keeps cards apart and inside the scene at every width', () => {
    for (const width of [320, 760, 1900]) {
      const sized = layoutInfrastructureNetworkMap(model, { width, density: 'names' });
      const cards = rects(sized, 'card');
      expect(cards).toHaveLength(3);
      cards.forEach((card, index) => {
        expect(card.x).toBeGreaterThan(0);
        expect(card.x + card.w).toBeLessThanOrEqual(sized.width + 0.01);
        expect(card.y + card.h).toBeLessThanOrEqual(sized.height);
        for (const other of cards.slice(index + 1)) {
          const apart =
            card.x + card.w <= other.x ||
            other.x + other.w <= card.x ||
            card.y + card.h <= other.y ||
            other.y + other.h <= card.y;
          expect(apart).toBe(true);
        }
      });
    }
  });

  it('draws every machine and no lines when nothing reports an address', () => {
    const silent = buildInfrastructureMapModel([
      resource({ id: 'bare', type: 'agent', name: 'bare', platformType: 'agent' }),
    ]);
    expect(silent.segments).toEqual([]);
    const empty = layoutInfrastructureNetworkMap(silent, { width: 800, density: 'names' });
    expect(rects(empty, 'card')).toHaveLength(1);
    expect(rects(empty, 'net0')).toHaveLength(0);
  });
});
