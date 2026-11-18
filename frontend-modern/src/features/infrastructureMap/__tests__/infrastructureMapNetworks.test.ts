import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
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
  it('records where a host is and which network each of its guests is on', () => {
    const node = hostNamed(hostsOf(lab()), 'pve');
    expect(node.address).toBe('192.168.10.11');
    expect(node.ownSegmentIds).toEqual([LAN]);
    const vms = node.sections[0].items;
    expect(vms.find((item) => item.name === 'haos')).toMatchObject({
      address: '10.0.30.5',
      segmentIds: [IOT],
    });
    expect(vms.find((item) => item.name === 'no-agent')?.segmentIds).toBeUndefined();
    expect(hostNamed(hostsOf(lab()), 'bare').ownSegmentIds).toEqual([]);
  });

  it('lists a Docker host’s containers with the ports each publishes', () => {
    const docker = hostNamed(hostsOf(lab()), 'apps');
    expect(docker.sections[0].items.map((item) => item.name)).toEqual(['jellyfin', 'traefik']);
    expect(docker.sections[0].items[1].detail).toBe(':80, 443');
    expect(docker.sections[0].items[0].detail).toBeUndefined();
  });

  it('hides addresses and network names with the rest when names are hidden', () => {
    const anonymous = anonymizeInfrastructureMapModel(buildInfrastructureMapModel(lab()));
    // What app a container is stays on purpose, in the app's own spelling.
    const text = JSON.stringify(anonymous, (key, value) =>
      key === 'id' ||
      key.endsWith('egmentIds') ||
      key === 'key' ||
      key === 'app' ||
      key === 'category'
        ? undefined
        : value,
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
  });
});
