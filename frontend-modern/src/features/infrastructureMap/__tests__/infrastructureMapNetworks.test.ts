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

// A Proxmox guest as the server sends it: each device says which bridge and
// VLAN its configuration plugs it into, with or without an address.
const guest = (
  id: string,
  devices: Array<{ bridge?: string; vlan?: number; addresses?: string[]; name?: string }>,
  partial: Partial<Resource> = {},
): Resource =>
  resource({
    id,
    type: 'vm',
    name: id,
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    ...partial,
    proxmox: {
      clusterName: 'homelab',
      ...partial.proxmox,
      networkInterfaces: devices.map((device, index) => ({
        name: device.name ?? `net${index}`,
        addresses: device.addresses ?? [],
        ...(device.bridge ? { bridge: device.bridge } : {}),
        ...(device.vlan ? { vlan: device.vlan } : {}),
      })),
    },
  });

const VLAN30 = 'bridge:homelab:vmbr0:30';

describe('Proxmox bridges and VLANs', () => {
  it('places a guest that reports no address on the bridge and VLAN it is plugged into', () => {
    const { segments, attachments } = resolveMapNetworks([
      ...lab(),
      guest('cam-nvr', [{ bridge: 'vmbr0', vlan: 30 }]),
      guest('cam-relay', [{ bridge: 'vmbr0', vlan: 30 }], { type: 'system-container' }),
    ]);
    expect(segments.find((segment) => segment.id === VLAN30)).toMatchObject({
      label: 'vmbr0 · VLAN 30',
      kind: 'bridge',
      memberCount: 2,
    });
    expect(segments.find((segment) => segment.id === VLAN30)?.detail).toBeUndefined();
    expect(attachments.get('cam-nvr')).toEqual([{ segmentId: VLAN30, address: undefined }]);
    expect(attachments.get('cam-relay')).toEqual([{ segmentId: VLAN30, address: undefined }]);
  });

  it('keeps one guest on its VLAN instead of filing a lone address under Other', () => {
    const { segments, attachments } = resolveMapNetworks([
      guest('dmz-proxy', [{ bridge: 'vmbr0', vlan: 40, addresses: ['10.0.40.2'] }]),
    ]);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      id: 'range:10.0.40',
      label: 'vmbr0 · VLAN 40',
      kind: 'bridge',
      detail: '10.0.40.x',
      memberCount: 1,
    });
    expect(attachments.get('dmz-proxy')).toEqual([
      { segmentId: 'range:10.0.40', address: '10.0.40.2' },
    ]);
  });

  it('treats an untagged bridge and the LAN its guests answer on as one network', () => {
    const { segments, attachments } = resolveMapNetworks([
      ...lab(),
      guest('files', [{ bridge: 'vmbr0', addresses: ['192.168.10.60'], name: 'ens18' }]),
      guest('no-agent-on-lan', [{ bridge: 'vmbr0' }]),
    ]);
    const lan = segments.find((segment) => segment.id === LAN);
    // The node, the NAS-style machines placed by address and both guests.
    expect(lan).toMatchObject({
      label: 'vmbr0',
      kind: 'bridge',
      detail: '192.168.10.0/24',
      memberCount: 5,
    });
    expect(attachments.get('files')).toEqual([{ segmentId: LAN, address: '192.168.10.60' }]);
    expect(attachments.get('no-agent-on-lan')).toEqual([{ segmentId: LAN, address: undefined }]);
    expect(attachments.get('docker')).toEqual([{ segmentId: LAN, address: '192.168.10.20' }]);
    expect(segments.some((segment) => segment.id.startsWith('bridge:'))).toBe(false);
  });

  it('follows the bridge, not a stray address, and joins guests with and without one', () => {
    const { segments, attachments } = resolveMapNetworks([
      guest('iot-a', [{ bridge: 'vmbr0', vlan: 30, addresses: ['10.0.30.5'] }]),
      guest('iot-b', [{ bridge: 'vmbr0', vlan: 30, addresses: ['10.0.30.6'] }]),
      guest('iot-stray', [{ bridge: 'vmbr0', vlan: 30, addresses: ['192.168.77.9'] }]),
      guest('iot-silent', [{ bridge: 'vmbr0', vlan: 30 }]),
    ]);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      id: IOT,
      label: 'vmbr0 · VLAN 30',
      kind: 'bridge',
      detail: '10.0.30.x',
      memberCount: 4,
    });
    expect(attachments.get('iot-stray')).toEqual([{ segmentId: IOT, address: '192.168.77.9' }]);
    expect(attachments.get('iot-silent')).toEqual([{ segmentId: IOT, address: undefined }]);
  });

  it('reads a flat homelab as one bridge on one LAN', () => {
    // The shapes a real single-node estate sends: the node's own bridge with a
    // prefix and a VPN address without a usable one, containers that report an
    // address with no prefix, and VMs with no guest agent that report nothing.
    const { segments, attachments } = resolveMapNetworks([
      resource({
        id: 'node',
        type: 'agent',
        name: 'delly',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['192.168.0.5/24'] },
            { name: 'tailscale0', addresses: ['100.101.4.7/32'] },
          ],
        },
      }),
      guest('pihole', [{ bridge: 'vmbr0', addresses: ['192.168.0.53'], name: 'eth0' }], {
        type: 'system-container',
        parentId: 'node',
      }),
      guest('docker', [{ bridge: 'vmbr0' }], { parentId: 'node' }),
      guest('ollama', [{ bridge: 'vmbr0' }], { parentId: 'node' }),
    ]);
    expect(segments.map((segment) => [segment.label, segment.kind, segment.detail])).toEqual([
      ['vmbr0', 'bridge', '192.168.0.0/24'],
      ['Other addresses', 'other', undefined],
    ]);
    expect(segments[0].memberCount).toBe(4);
    expect(attachments.get('docker')).toEqual([
      { segmentId: 'net:192.168.0.0/24', address: undefined },
    ]);
    expect(attachments.get('node')?.[0]).toEqual({
      segmentId: 'net:192.168.0.0/24',
      address: '192.168.0.5',
    });
  });

  it('does not name a bridge after one of several ranges it carries', () => {
    const { segments, attachments } = resolveMapNetworks([
      guest('a', [{ bridge: 'vmbr0', vlan: 30, addresses: ['10.30.1.5'] }]),
      guest('b', [{ bridge: 'vmbr0', vlan: 30, addresses: ['10.30.2.5'] }]),
      guest('c', [{ bridge: 'vmbr0', vlan: 30, addresses: ['10.30.3.5'] }]),
    ]);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      id: VLAN30,
      label: 'vmbr0 · VLAN 30',
      kind: 'bridge',
      memberCount: 3,
    });
    expect(segments[0].detail).toBeUndefined();
    // Each guest still shows the address it answers on.
    expect(attachments.get('b')).toEqual([{ segmentId: VLAN30, address: '10.30.2.5' }]);
  });

  it('puts a guest with two devices on both networks', () => {
    const { attachments } = resolveMapNetworks([
      guest('router', [
        { bridge: 'vmbr0', addresses: ['192.168.10.1/24'] },
        { bridge: 'vmbr0', vlan: 30 },
        { addresses: ['100.64.0.3'], name: 'tailscale0' },
      ]),
    ]);
    expect(attachments.get('router')).toEqual([
      { segmentId: LAN, address: '192.168.10.1' },
      { segmentId: VLAN30, address: undefined },
    ]);
  });

  it('does not join the same bridge name on two unrelated nodes', () => {
    const { segments, attachments } = resolveMapNetworks([
      guest('a', [{ bridge: 'vmbr0', vlan: 30 }]),
      guest('b', [{ bridge: 'vmbr0', vlan: 30 }], {
        proxmox: { clusterName: '', instance: 'shed' },
      }),
    ]);
    expect(attachments.get('a')?.[0].segmentId).toBe(VLAN30);
    expect(attachments.get('b')?.[0].segmentId).toBe('bridge:shed:vmbr0:30');
    expect(segments.map((segment) => [segment.label, segment.detail])).toEqual([
      ['vmbr0 · VLAN 30', 'homelab'],
      ['vmbr0 · VLAN 30', 'shed'],
    ]);
  });

  it('names the subnet when two bridges carry it', () => {
    const { segments } = resolveMapNetworks([
      guest('a', [{ bridge: 'vmbr0', addresses: ['192.168.10.60/24'] }]),
      guest('b', [{ bridge: 'vmbr2', addresses: ['192.168.10.61/24'] }]),
    ]);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({
      id: LAN,
      label: '192.168.10.0/24',
      kind: 'bridge',
      detail: 'vmbr0, vmbr2',
    });
  });

  it('ignores a bridge on anything that is not a Proxmox guest, and a tag out of range', () => {
    const { segments, attachments } = resolveMapNetworks([
      resource({
        id: 'agent-host',
        type: 'agent',
        name: 'agent-host',
        platformType: 'agent',
        agent: { networkInterfaces: [{ name: 'eth0', addresses: [], bridge: 'vmbr0' }] },
      }),
      guest('odd-tag', [{ bridge: 'vmbr0', vlan: 5000 }]),
    ]);
    expect(attachments.has('agent-host')).toBe(false);
    expect(segments.map((segment) => segment.label)).toEqual(['vmbr0']);
  });

  it('marks a guest with the VLAN it is plugged into, address or not', () => {
    const resources = [
      ...lab(),
      guest('cam-nvr', [{ bridge: 'vmbr0', vlan: 30 }]),
      guest('cam-relay', [{ bridge: 'vmbr0', vlan: 30 }]),
      guest('cam-store', [{ bridge: 'vmbr0', vlan: 30 }]),
    ];
    const node = hostNamed(hostsOf(resources), 'pve');
    expect(node.sections[0].items.find((item) => item.name === 'cam-nvr')).toMatchObject({
      segmentIds: [VLAN30],
    });
    expect(node.sections[0].items.find((item) => item.name === 'cam-nvr')?.address).toBeUndefined();
  });

  it('hides bridge names, subnets and cluster names when names are hidden', () => {
    const anonymous = anonymizeInfrastructureMapModel(
      buildInfrastructureMapModel([
        ...lab(),
        guest('files', [{ bridge: 'vmbr0', addresses: ['192.168.10.60'] }]),
        guest('cam-nvr', [{ bridge: 'vmbr7', vlan: 30 }]),
        guest('cam-shed', [{ bridge: 'vmbr7', vlan: 30 }], {
          proxmox: { clusterName: '', instance: 'shed' },
        }),
      ]),
    );
    const text = JSON.stringify(anonymous, (key, value) =>
      key === 'id' || key.endsWith('egmentIds') || key === 'key' ? undefined : value,
    );
    for (const secret of ['vmbr', 'VLAN', '192.168.10', 'homelab', 'shed']) {
      expect(text).not.toContain(secret);
    }
    expect(anonymous.segments.every((segment) => segment.detail === undefined)).toBe(true);
  });
});
