import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { MAP_APP_ICONS } from '../infrastructureMapAppIcons';
import {
  MAP_CATALOGUE_APPS,
  resolveMapApp,
  resolveMapAppLogo,
  resolveMapCategory,
} from '../infrastructureMapCatalogue';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
} from '../infrastructureMapModel';
import { buildServiceDiagram, type DiagramGroup } from '../infrastructureMapServices';

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

const guest = (name: string, parentId: string, extra: Partial<Resource> = {}): Resource =>
  resource({
    id: `ct-${name}`,
    type: 'system-container',
    name,
    platformType: 'proxmox-pve',
    parentId,
    status: 'running',
    ...extra,
  });

const homelab = (): Resource[] => [
  resource({
    id: 'pve',
    type: 'agent',
    name: 'delly',
    platformType: 'proxmox-pve',
    agent: { networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.11/24'] }] },
  }),
  resource({ id: 'pve2', type: 'agent', name: 'minipc', platformType: 'proxmox-pve' }),
  guest('jellyfin', 'pve', { identity: { ips: ['192.168.10.50'] } }),
  guest('adguard', 'pve2'),
  guest('cloudflared', 'pve2'),
  guest('ps4-ctrl', 'pve'),
  guest('rstack-curator', 'pve2', { tags: ['192.168.0.188', 'media'] }),
  guest('old-lab', 'pve', { status: 'stopped' }),
  guest('older-lab', 'pve', { status: 'stopped' }),
  guest('ollama', 'pve', { status: 'stopped' }),
  resource({
    id: 'zfs-a',
    type: 'storage',
    name: 'local-zfs',
    platformType: 'proxmox-pve',
    parentId: 'pve',
  }),
  resource({
    id: 'zfs-b',
    type: 'storage',
    name: 'local-zfs',
    platformType: 'proxmox-pve',
    parentId: 'pve2',
  }),
  resource({
    id: 'pbs-store',
    type: 'storage',
    name: 'pbs-delly',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    storage: { type: 'pbs', content: 'backup' },
  }),
  resource({ id: 'tower', type: 'docker-host', name: 'Tower', platformType: 'docker' }),
  resource({
    id: 'c-cache',
    type: 'app-container',
    name: 'authentik-redis',
    platformType: 'docker',
    parentId: 'tower',
    status: 'running',
    docker: { image: 'redis:7-alpine' },
  }),
];

const diagramOf = (resources: Resource[], maxTilesPerGroup?: number) =>
  buildServiceDiagram(buildInfrastructureMapModel(resources), { maxTilesPerGroup });

const names = (group: DiagramGroup | undefined): string[] =>
  (group?.tiles ?? []).map((tile) => tile.name);

describe('resolveMapApp', () => {
  it('reads the app from the image before the name', () => {
    expect(resolveMapApp({ name: 'authentik-redis', image: 'redis:7-alpine' })?.label).toBe(
      'Redis',
    );
    expect(resolveMapApp({ name: 'vw', image: 'vaultwarden/server:latest' })?.label).toBe(
      'Vaultwarden',
    );
    expect(
      resolveMapApp({ name: 'tv', image: 'lscr.io/linuxserver/sonarr:latest@sha256:abc' })?.label,
    ).toBe('Sonarr');
    expect(resolveMapApp({ name: 'x', image: 'registry.lan:5000/library/grafana' })?.label).toBe(
      'Grafana',
    );
  });

  it('takes the longest spelling a name matches, on whole words only', () => {
    expect(resolveMapApp({ name: 'nginx-proxy-manager' })?.label).toBe('Nginx Proxy Manager');
    expect(resolveMapApp({ name: 'zigbee2mqtt' })?.label).toBe('Zigbee2MQTT');
    expect(resolveMapApp({ name: 'home_assistant' })?.label).toBe('Home Assistant');
    expect(resolveMapApp({ name: 'my-grafana-01' })?.label).toBe('Grafana');
    expect(resolveMapApp({ name: 'influxdb-telegraf' })?.label).toBe('InfluxDB');
    expect(resolveMapApp({ name: 'plexiglass' })).toBeUndefined();
    expect(resolveMapApp({ name: 'ps4-ctrl' })).toBeUndefined();
  });

  it('only reads an everyday word as an app when nothing but filler follows it', () => {
    expect(resolveMapApp({ name: 'pulse' })?.label).toBe('Pulse');
    expect(resolveMapApp({ name: 'pulse-prod' })?.label).toBe('Pulse');
    expect(resolveMapApp({ name: 'homepage-docker' })?.label).toBe('Homepage');
    expect(resolveMapApp({ name: 'pulse-backup-lab-2439' })).toBeUndefined();
    expect(resolveMapApp({ name: 'blog-ghost-writer' })).toBeUndefined();
    expect(resolveMapApp({ name: 'x', image: 'ghcr.io/gethomepage/homepage:latest' })?.label).toBe(
      'Homepage',
    );
  });

  it('gives every app one group and a logo only where one exists', () => {
    expect(MAP_CATALOGUE_APPS.length).toBeGreaterThan(120);
    for (const app of MAP_CATALOGUE_APPS) {
      if (app.icon) expect(resolveMapAppLogo(app.icon), app.id).toBeDefined();
    }
    expect(resolveMapApp({ name: 'jellyfin' })).toMatchObject({
      category: 'media',
      icon: 'jellyfin',
    });
    expect(resolveMapApp({ name: 'cloudflared' })).toMatchObject({
      category: 'access',
      role: 'tunnel',
      icon: 'cloudflare',
    });
    expect(resolveMapApp({ name: 'gluetun' })?.icon).toBeUndefined();
  });

  it('carries only path data and a colour for each logo', () => {
    for (const [key, logo] of Object.entries(MAP_APP_ICONS)) {
      expect(logo.hex, key).toMatch(/^[0-9A-F]{6}$/);
      expect(logo.d, key).toMatch(/^[MmLlHhVvCcSsQqTtAaZz0-9\s,.\-eE]+$/);
    }
  });
});

describe('resolveMapCategory', () => {
  it('uses the app, then a tag that names a purpose, then the last group', () => {
    expect(resolveMapCategory(resolveMapApp({ name: 'jdownloader' }), ['vpn', 'monitoring'])).toBe(
      'media',
    );
    expect(resolveMapCategory(undefined, ['192.168.0.188', 'Media'])).toBe('media');
    expect(resolveMapCategory(undefined, ['home-automation'])).toBe('home');
    expect(resolveMapCategory(undefined, ['community-script'])).toBe('other');
    expect(resolveMapCategory(undefined, undefined)).toBe('other');
  });
});

describe('buildServiceDiagram', () => {
  it('boxes what runs by purpose and names the host under each thing', () => {
    const diagram = diagramOf(homelab());
    expect(diagram.groups.map((group) => group.key)).toEqual(['media', 'data', 'ai', 'other']);
    expect(names(diagram.groups[0])).toEqual(['jellyfin', 'rstack-curator']);
    expect(diagram.groups[0].tiles[0]).toMatchObject({
      caption: 'delly',
      icon: 'jellyfin',
      health: 'ok',
      navigable: true,
      typeLabel: 'Jellyfin · LXC',
    });
    expect(diagram.groups[1].tiles[0]).toMatchObject({ name: 'authentik-redis', caption: 'Tower' });
    expect(diagram.serviceCount).toBe(9);
  });

  it('keeps a stopped app it can name and folds away stopped things it cannot', () => {
    const diagram = diagramOf(homelab());
    expect(diagram.groups[2].tiles[0]).toMatchObject({ name: 'ollama', health: 'off' });
    const other = diagram.groups[3];
    expect(names(other)).toEqual(['ps4-ctrl', '+2']);
    expect(other.tiles[1]).toMatchObject({ caption: 'switched off', navigable: false });
  });

  it('draws the Internet only while something that reaches it is running', () => {
    const diagram = diagramOf(homelab());
    expect(diagram.internet).toEqual({ label: 'tunnel' });
    expect(diagram.access?.tiles.map((tile) => [tile.name, tile.caption])).toEqual([
      ['adguard', 'DNS'],
      ['cloudflared', 'tunnel'],
    ]);

    const stopped = homelab().map((entry) =>
      entry.name === 'cloudflared' ? ({ ...entry, status: 'stopped' } as Resource) : entry,
    );
    expect(diagramOf(stopped).internet).toBeUndefined();
    const dnsOnly = homelab().filter((entry) => entry.name !== 'cloudflared');
    expect(diagramOf(dnsOnly).internet).toBeUndefined();
    expect(names(diagramOf(dnsOnly).access)).toEqual(['adguard']);
  });

  it('draws one tile for a pool several hosts have and sets backup targets apart', () => {
    const diagram = diagramOf(homelab());
    expect(diagram.storage?.tiles).toHaveLength(1);
    expect(diagram.storage?.tiles[0]).toMatchObject({
      name: 'local-zfs',
      caption: '2 hosts',
      platform: 'storage',
    });
    expect(diagram.backups?.tiles[0]).toMatchObject({
      name: 'pbs-delly',
      caption: 'delly',
      platform: 'backup',
    });
  });

  it('lists the hardware with how much runs on each', () => {
    const diagram = diagramOf(homelab());
    expect(diagram.hardware?.tiles.map((tile) => [tile.name, tile.caption])).toEqual([
      ['delly', '5 services'],
      ['minipc', '3 services'],
      ['Tower', '1 service'],
    ]);
    expect(diagram.networks).toEqual([
      {
        id: 'net:192.168.10.0/24',
        label: '192.168.10.0/24',
        kind: 'subnet',
        memberCount: 2,
        colorIndex: 0,
      },
    ]);
  });

  describe('more networks than the diagram names', () => {
    const plugged = (
      name: string,
      scope: string,
      vlans: number[],
      extra: Partial<Resource> = {},
    ): Resource =>
      guest(name, 'pve', {
        ...extra,
        proxmox: {
          clusterName: scope,
          networkInterfaces: vlans.map((vlan, index) => ({
            name: `net${index}`,
            addresses: [],
            bridge: 'vmbr0',
            ...(vlan ? { vlan } : {}),
          })),
        },
      });
    const tileNamed = (diagram: ReturnType<typeof diagramOf>, name: string) =>
      diagram.groups.flatMap((group) => group.tiles).find((tile) => tile.name === name);

    it('names the VLANs of one bridge together and counts each guest once', () => {
      const vlans = [0, 10, 20, 30, 40, 50, 60];
      const diagram = diagramOf(
        [
          ...homelab(),
          ...vlans.map((vlan) => plugged(`svc-${vlan}`, 'homelab', [vlan])),
          // On two VLANs of the bridge, and still one thing connected to it.
          plugged('multi-homed', 'homelab', [10, 20]),
        ],
        60,
      );
      expect(diagram.networks).toEqual([
        {
          id: 'bridge:homelab:vmbr0',
          label: 'vmbr0',
          kind: 'bridge',
          detail: 'VLANs 10, 20, 30, 40 +2',
          memberCount: 8,
          colorIndex: 0,
        },
        {
          id: 'net:192.168.10.0/24',
          label: '192.168.10.0/24',
          kind: 'subnet',
          memberCount: 2,
          colorIndex: 1,
        },
      ]);
      // The tile takes the bridge's colour, and its hover keeps the exact VLAN.
      expect(tileNamed(diagram, 'svc-30')).toMatchObject({
        segmentId: 'bridge:homelab:vmbr0',
        address: 'on vmbr0 · VLAN 30',
      });
      expect(tileNamed(diagram, 'multi-homed')?.address).toBe(
        'on vmbr0 · VLAN 10 and vmbr0 · VLAN 20',
      );
      expect(tileNamed(diagram, 'jellyfin')?.address).toBe('192.168.10.50');
    });

    it('says whose bridge it is when two clusters fold to the same name', () => {
      const diagram = diagramOf(
        [
          ...homelab(),
          ...[10, 20, 30, 40].map((vlan) => plugged(`east-${vlan}`, 'east', [vlan])),
          ...[10, 20, 30].map((vlan) => plugged(`west-${vlan}`, 'west', [vlan])),
        ],
        60,
      );
      expect(
        diagram.networks.map(({ label, detail, memberCount }) => [label, detail, memberCount]),
      ).toEqual([
        ['vmbr0', 'east', 4],
        ['vmbr0', 'west', 3],
        ['192.168.10.0/24', undefined, 2],
      ]);
    });

    it('leaves a bridge with one network under its own name', () => {
      const diagram = diagramOf(
        [
          ...homelab(),
          ...[10, 20, 30, 40, 50].map((vlan) => plugged(`east-${vlan}`, 'east', [vlan])),
          plugged('lone', 'shed', [70]),
        ],
        60,
      );
      expect(diagram.networks.map((network) => network.label)).toEqual([
        'vmbr0',
        '192.168.10.0/24',
        'vmbr0 · VLAN 70',
      ]);
      expect(tileNamed(diagram, 'lone')?.segmentId).toBe('bridge:shed:vmbr0:70');
    });

    it('does not fold while every network can be named', () => {
      const diagram = diagramOf([
        ...homelab(),
        ...[10, 20, 30, 40, 50].map((vlan) => plugged(`svc-${vlan}`, 'homelab', [vlan])),
      ]);
      expect(diagram.networks).toHaveLength(6);
      expect(diagram.networks.filter((network) => network.kind === 'bridge')).toHaveLength(5);
      expect(tileNamed(diagram, 'svc-30')?.segmentId).toBe('bridge:homelab:vmbr0:30');
    });

    it('folds nothing once names are hidden, and names no bridge', () => {
      const hidden = buildServiceDiagram(
        anonymizeInfrastructureMapModel(
          buildInfrastructureMapModel([
            ...homelab(),
            ...[0, 10, 20, 30, 40, 50, 60].map((vlan) => plugged(`svc-${vlan}`, 'homelab', [vlan])),
          ]),
        ),
      );
      expect(hidden.networks).toHaveLength(8);
      const shown = hidden.networks.map(({ label, detail }) => [label, detail]);
      expect(JSON.stringify(shown)).not.toMatch(/vmbr|VLAN|homelab|192\.168/);
    });
  });

  it('carries a Proxmox bridge as a network of its own kind', () => {
    const onVlan = (name: string) =>
      guest(name, 'pve', {
        proxmox: {
          clusterName: 'homelab',
          networkInterfaces: [{ name: 'eth0', addresses: [], bridge: 'vmbr0', vlan: 30 }],
        },
      });
    const diagram = diagramOf([...homelab(), onVlan('frigate'), onVlan('esphome')]);
    expect(diagram.networks.find((network) => network.kind === 'bridge')).toMatchObject({
      id: 'bridge:homelab:vmbr0:30',
      label: 'vmbr0 · VLAN 30',
      memberCount: 2,
    });
    // A bridge that carries a known subnet says which.
    const onLan = guest('paperless', 'pve', {
      proxmox: {
        clusterName: 'homelab',
        networkInterfaces: [{ name: 'eth0', addresses: ['192.168.10.60'], bridge: 'vmbr0' }],
      },
    });
    expect(diagramOf([...homelab(), onLan]).networks).toEqual([
      {
        id: 'net:192.168.10.0/24',
        label: 'vmbr0',
        kind: 'bridge',
        detail: '192.168.10.0/24',
        memberCount: 3,
        colorIndex: 0,
      },
    ]);
  });

  it('folds the tail of a large group into one tile', () => {
    const many = [
      ...homelab(),
      ...Array.from({ length: 6 }, (_, index) => guest(`sonarr-${index}`, 'pve')),
    ];
    const media = diagramOf(many, 4).groups[0];
    expect(names(media)).toEqual(['jellyfin', 'rstack-curator', 'sonarr-0', '+5']);
    expect(media.tiles[3]).toMatchObject({ caption: 'more', navigable: false });
  });

  it('keeps what an app is and hides what it was called when names are hidden', () => {
    const model = anonymizeInfrastructureMapModel(buildInfrastructureMapModel(homelab()));
    const diagram = buildServiceDiagram(model);
    expect(names(diagram.groups[0])).toEqual(['Jellyfin', 'LXC 4']);
    expect(diagram.groups[0].tiles[0].caption).toBe('Proxmox node 1');
    expect(names(diagram.groups[1])).toEqual(['Redis']);
    expect(diagram.hardware?.tiles.map((tile) => tile.name)).toEqual([
      'Docker host 1',
      'Proxmox node 1',
      'Proxmox node 2',
    ]);
    // Ids are lookups and are never drawn.
    const drawn = JSON.stringify(diagram, (key, value) =>
      key === 'id' || key === 'segmentId' ? undefined : value,
    );
    expect(drawn).not.toMatch(/delly|minipc|Tower|rstack|ps4|authentik|192\.168/);
  });

  it('counts what each box holds and gives probed endpoints a box of their own', () => {
    const diagram = diagramOf([
      ...homelab(),
      resource({
        id: 'check-router',
        type: 'network-endpoint',
        name: 'Router',
        platformType: 'availability',
        availability: { address: '192.168.10.1' },
      }),
    ]);
    expect(diagram.groups.map((group) => [group.key, group.count])).toEqual([
      ['media', 2],
      ['data', 1],
      ['ai', 1],
      // One running thing and the two switched off that fold into one tile.
      ['other', 3],
      ['checks', 1],
    ]);
    expect(diagram.groups.at(-1)).toMatchObject({ label: 'Checks' });
    expect(diagram.groups.at(-1)?.tiles[0]).toMatchObject({
      name: 'Router',
      platform: 'checks',
      navigable: true,
    });
    expect(diagram.storage?.count).toBe(1);
    expect(diagram.hardware?.count).toBe(3);
  });

  it('is empty for an empty estate', () => {
    const diagram = diagramOf([]);
    expect(diagram).toEqual({ networks: [], groups: [], serviceCount: 0 });
  });
});
