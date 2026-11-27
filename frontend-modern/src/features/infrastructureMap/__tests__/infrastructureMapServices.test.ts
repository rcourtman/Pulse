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
import {
  buildServiceDiagram,
  collectActiveTrafficTiles,
  collectDiagramGroups,
  collectExposedEntries,
  collectNetworkTiles,
  compareDiagramTilesByTraffic,
  countDiagramResources,
  findDiagramGroup,
  formatMapLatency,
  formatMapLinkSpeed,
  type DiagramGroup,
  type DiagramTile,
} from '../infrastructureMapServices';

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

  it('names an app as itself and not as the one it replaces', () => {
    // Byparr stands in for FlareSolverr. With names hidden the tile is
    // labelled by its app, and the wrong app's name would be published.
    expect(resolveMapApp({ name: 'byparr' })?.label).toBe('Byparr');
    expect(resolveMapApp({ name: 'x', image: 'ghcr.io/thephaseless/byparr:latest' })?.label).toBe(
      'Byparr',
    );
    expect(resolveMapApp({ name: 'flaresolverr' })?.label).toBe('FlareSolverr');
    expect(resolveMapApp({ name: 'byparr' })?.category).toBe('media');
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
    expect(diagram.groups[0].tiles[0].caption).toBe('Proxmox 1');
    expect(names(diagram.groups[1])).toEqual(['Redis']);
    expect(diagram.hardware?.tiles.map((tile) => tile.name)).toEqual([
      'Docker 1',
      'Proxmox 1',
      'Proxmox 2',
    ]);
    // Ids are lookups and are never drawn.
    const drawn = JSON.stringify(diagram, (key, value) =>
      ['id', 'segmentId', 'segmentIds', 'fromId', 'toId'].includes(key) ? undefined : value,
    );
    expect(drawn).not.toMatch(/delly|minipc|Tower|rstack|ps4|authentik|192\.168/);
  });

  it('draws a check on network gear as that gear, with the ways in', () => {
    const check = (id: string, name: string, address: string, targetKind: string): Resource =>
      resource({
        id,
        type: 'network-endpoint',
        name,
        platformType: 'availability',
        availability: { address, targetKind },
      } as Parameters<typeof resource>[0]);
    const resources = [
      ...homelab(),
      // Named for nothing a switch is called: the type its owner chose decides.
      check('check-switch', 'usw-24-rack', '192.168.10.2', 'switch'),
      check('check-ap', 'Living room', '192.168.10.3', 'access_point'),
      check('check-fw', 'Edge', '192.168.10.1', 'router'),
      check('check-nas', 'old-nas', '192.168.10.9', 'machine'),
      // And a name alone decides nothing.
      check('check-site', 'Router admin page', '192.168.10.80', 'service'),
      check('check-ups', 'Rack UPS', '192.168.10.81', 'device'),
    ];
    const diagram = diagramOf(resources);

    const gear = (diagram.access?.tiles ?? []).filter((tile) => tile.id.startsWith('check-'));
    expect(gear.map((tile) => [tile.name, tile.caption, tile.platform])).toEqual([
      ['Edge', 'router', 'gateway'],
      ['Living room', 'access point', 'access-point'],
      ['usw-24-rack', 'switch', 'switch'],
    ]);
    // Each opens its check, and keeps its address for the hover.
    expect(gear.every((tile) => tile.navigable && tile.typeLabel === 'Check')).toBe(true);
    expect(gear.map((tile) => tile.address)).toEqual([
      '192.168.10.1',
      '192.168.10.3',
      '192.168.10.2',
    ]);
    // A check on anything else stays a check, whatever it is called. That
    // includes a machine: only a Pulse agent makes a machine.
    expect(names(diagram.groups.find((group) => group.key === 'checks'))).toEqual([
      'old-nas',
      'Rack UPS',
      'Router admin page',
    ]);
    expect(diagram.hardware?.tiles.some((tile) => tile.id === 'check-nas')).toBe(false);

    // With names hidden it is the same gear in the same box.
    const hidden = buildServiceDiagram(
      anonymizeInfrastructureMapModel(buildInfrastructureMapModel(resources)),
    );
    expect(
      (hidden.access?.tiles ?? [])
        .filter((tile) => tile.id.startsWith('check-'))
        .map((tile) => [tile.name, tile.caption]),
    ).toEqual([
      ['AP 1', 'access point'],
      ['Router 1', 'router'],
      ['Switch 1', 'switch'],
    ]);
    expect(names(hidden.groups.find((group) => group.key === 'checks'))).toEqual([
      'Check 1',
      'Check 2',
      'Check 3',
    ]);
  });

  it('counts what each box holds and gives probed endpoints a box of their own', () => {
    const diagram = diagramOf([
      ...homelab(),
      resource({
        id: 'check-site',
        type: 'network-endpoint',
        name: 'Status page',
        platformType: 'availability',
        availability: { address: '192.168.10.80' },
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
      name: 'Status page',
      caption: '192.168.10.80',
      platform: 'checks',
      navigable: true,
    });
    expect(diagram.storage?.count).toBe(1);
    expect(diagram.hardware?.count).toBe(3);
  });

  describe('connections', () => {
    const hex = (seed: string): string =>
      [...seed]
        .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
        .join('')
        .padEnd(64, '0')
        .slice(0, 64);
    const dockerHost = (agent: NonNullable<Resource['agent']> = {}): Resource =>
      resource({ id: 'tower', type: 'docker-host', name: 'Tower', platformType: 'docker', agent });
    const box = (
      name: string,
      docker: NonNullable<Resource['docker']> = {},
      extra: Partial<Resource> = {},
    ): Resource =>
      resource({
        id: `c-${name}`,
        type: 'app-container',
        name,
        platformType: 'docker',
        parentId: 'tower',
        status: 'running',
        docker: {
          hostSourceId: 'tower-daemon',
          containerId: hex(name),
          containerState: 'running',
          image: `example/${name}:latest`,
          networks: [{ name: 'bridge' }],
          ...docker,
        },
        ...extra,
      });
    const behindVpn = (name: string): Resource =>
      box(name, { networkMode: `container:${hex('gluetun')}`, networks: undefined });
    // The links between services, without each one's link to its machine.
    const between = (diagram: ReturnType<typeof buildServiceDiagram>) =>
      diagram.links.filter((link) => link.fromId !== 'tower');
    const tileOf = (diagram: ReturnType<typeof buildServiceDiagram>, id: string) =>
      [diagram.access, ...diagram.groups, diagram.backups, diagram.hardware]
        .flatMap((group) => group?.tiles ?? [])
        .find((tile) => tile.id === id);

    // A real Unraid host: a download client and an indexer inside a VPN
    // container's network, beside containers with networks of their own.
    const unraid = (): Resource[] => [
      dockerHost(),
      box('gluetun', { networkMode: 'bridge' }),
      behindVpn('qbittorrent'),
      behindVpn('prowlarr'),
      box('radarr', { networkMode: 'bridge' }),
    ];

    it('links a container to the one whose network it uses and says so on both', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(unraid()));

      expect(between(diagram)).toEqual([
        { fromId: 'c-prowlarr', toId: 'c-gluetun', label: 'via gluetun' },
        { fromId: 'c-qbittorrent', toId: 'c-gluetun', label: 'via gluetun' },
      ]);
      const client = tileOf(diagram, 'c-qbittorrent');
      expect(client?.caption).toBe('via gluetun');
      expect(client?.links).toEqual(['Uses the network of gluetun']);
      expect(tileOf(diagram, 'c-gluetun')?.links).toEqual(['Carries the network of 2 containers']);
      // A container with a network of its own keeps its host as caption.
      expect(tileOf(diagram, 'c-radarr')?.caption).toBe('Tower');
      expect(tileOf(diagram, 'c-radarr')?.links).toBeUndefined();
    });

    it('links nothing to a tile folded away', () => {
      const many = [
        dockerHost(),
        box('gluetun', { networkMode: 'bridge' }),
        ...Array.from({ length: 6 }, (_, index) => behindVpn(`zz-worker-${index}`)),
      ];
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(many), {
        maxTilesPerGroup: 3,
      });
      const drawn = new Set(
        [...diagram.groups, diagram.hardware].flatMap((group) =>
          (group?.tiles ?? []).map((tile) => tile.id),
        ),
      );
      expect(between(diagram).length).toBeGreaterThan(0);
      expect(between(diagram).length).toBeLessThan(6);
      for (const link of diagram.links) {
        expect(drawn.has(link.fromId)).toBe(true);
        expect(drawn.has(link.toId)).toBe(true);
      }
    });

    it('links a reverse proxy to what it routes and writes the hostname under the app', () => {
      const diagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          dockerHost(),
          box('traefik', { image: 'traefik:v3.1' }),
          box('jellyfin', {
            labels: {
              'traefik.enable': 'true',
              'traefik.http.routers.jellyfin.rule': 'Host(`media.example.test`)',
            },
          }),
        ]),
      );

      expect(between(diagram)).toEqual([
        { fromId: 'c-traefik', toId: 'c-jellyfin', label: 'media.example.test' },
      ]);
      // Where it answers is its caption, in place of the host it runs on.
      expect(tileOf(diagram, 'c-jellyfin')?.caption).toBe('media.example.test');
      expect(tileOf(diagram, 'c-jellyfin')?.links).toEqual([
        'Routed by traefik at media.example.test',
      ]);
      expect(tileOf(diagram, 'c-traefik')?.links).toEqual(['Routes 1 container']);
    });

    it('draws a gateway nothing monitored answers on as a tile of its own', () => {
      const diagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          // What a real Unraid host reports: its router is its resolver too.
          dockerHost({
            defaultGateways: [{ address: '192.168.0.1', interface: 'eth0' }],
            dnsServers: ['192.168.0.1', '1.1.1.1'],
          }),
          // A node whose resolver is elsewhere, as one on a tailnet is.
          resource({
            id: 'pve',
            type: 'agent',
            name: 'delly',
            platformType: 'proxmox-pve',
            agent: {
              defaultGateways: [{ address: '192.168.0.1', interface: 'vmbr0' }],
              dnsServers: ['100.100.100.100'],
            },
          }),
          box('radarr'),
        ]),
      );

      expect(diagram.gateways).toEqual([
        {
          id: 'gateway:0',
          name: '192.168.0.1',
          caption: 'default gateway',
          typeLabel: 'Default gateway, not monitored',
          status: '',
          health: 'unknown',
          platform: 'gateway',
          links: ['Used by delly, Tower', 'Resolves names for Tower'],
          navigable: false,
          // What its tile fills in on the form that adds a check for it.
          checkAddress: '192.168.0.1',
        },
      ]);
    });

    it('lets a check on the gateway take the place of the unmonitored tile', () => {
      const diagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          resource({
            id: 'pve',
            type: 'agent',
            name: 'delly',
            platformType: 'proxmox-pve',
            agent: { defaultGateways: [{ address: '192.168.0.1', interface: 'vmbr0' }] },
          }),
          resource({
            id: 'check-router',
            type: 'network-endpoint',
            name: 'Gateway',
            // What the map's own link to the form sets.
            availability: { address: '192.168.0.1', targetKind: 'router' },
          } as Parameters<typeof resource>[0]),
        ]),
      );

      // Monitored now, so nothing says it is not, and typed as a router, so
      // it is drawn as one where the dashed tile was.
      expect(diagram.gateways).toEqual([]);
      expect(diagram.access?.tiles.map((tile) => tile.id)).toEqual(['check-router']);
      expect(tileOf(diagram, 'check-router')).toMatchObject({
        name: 'Gateway',
        caption: 'router',
        platform: 'gateway',
        links: ['Default gateway of delly'],
      });
    });

    it('says it on the monitored thing when a gateway or resolver is one', () => {
      const pve = resource({
        id: 'pve',
        type: 'agent',
        name: 'delly',
        platformType: 'proxmox-pve',
        agent: {
          networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.0.5/24'] }],
          defaultGateways: [{ address: '192.168.0.1', interface: 'vmbr0' }],
          dnsServers: ['192.168.0.53', '100.100.100.100'],
        },
      });
      const diagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          pve,
          guest('opnsense', 'pve', { identity: { ips: ['192.168.0.1'] } }),
          guest('adguard', 'pve', { identity: { ips: ['192.168.0.53'] } }),
        ]),
      );

      expect(diagram.gateways).toEqual([]);
      expect(tileOf(diagram, 'ct-opnsense')?.links).toEqual(['Default gateway of delly']);
      expect(tileOf(diagram, 'ct-adguard')?.links).toEqual(['Resolves names for delly']);
    });

    it('draws nothing for a resolver that is not on the map', () => {
      const diagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          dockerHost({ dnsServers: ['100.100.100.100'] }),
          box('radarr'),
        ]),
      );
      expect(diagram.gateways).toEqual([]);
      expect(between(diagram)).toEqual([]);
      expect(tileOf(diagram, 'tower')?.links).toEqual(['Media 1']);
    });

    it('keeps the links and drops hostnames and addresses when names are hidden', () => {
      const model = anonymizeInfrastructureMapModel(
        buildInfrastructureMapModel([
          dockerHost({ defaultGateways: [{ address: '192.168.0.1' }] }),
          box('traefik', { image: 'traefik:v3.1' }),
          box('wiki-internal', {
            labels: { 'traefik.http.routers.wiki.rule': 'Host(`wiki.example.test`)' },
          }),
        ]),
      );
      const diagram = buildServiceDiagram(model);
      // Ids are opaque in a real estate. What can leak is anything written.
      const written = JSON.stringify([
        diagram.gateways,
        [diagram.access, ...diagram.groups, diagram.hardware].flatMap((group) =>
          (group?.tiles ?? []).map((tile) => [tile.name, tile.caption, tile.links, tile.address]),
        ),
      ]);

      expect(between(diagram)).toEqual([
        { fromId: 'c-traefik', toId: 'c-wiki-internal', label: 'routes' },
        { fromId: 'gateway:0', toId: 'tower', label: 'gateway' },
      ]);
      expect(diagram.gateways.map((tile) => tile.name)).toEqual(['Gateway']);
      expect(written).toContain('Routed by Traefik');
      expect(written).not.toContain('example.test');
      expect(written).not.toContain('192.168.0.1');
      expect(written).not.toContain('wiki-internal');
      expect(written).not.toContain('Tower');
    });
  });

  describe('what each thing is joined to', () => {
    const tiles = (diagram: ReturnType<typeof buildServiceDiagram>) =>
      [
        diagram.access,
        ...diagram.groups,
        diagram.storage,
        diagram.backups,
        diagram.hardware,
      ].flatMap((group) => group?.tiles ?? []);
    const tileOf = (diagram: ReturnType<typeof buildServiceDiagram>, id: string) =>
      tiles(diagram).find((tile) => tile.id === id);
    const joined = (diagram: ReturnType<typeof buildServiceDiagram>, id: string) =>
      diagram.links
        .flatMap((link) =>
          link.fromId === id ? [link.toId] : link.toId === id ? [link.fromId] : [],
        )
        .sort();
    const TB = 1_000_000_000_000;
    // A guest that mounts a filesystem from outside itself, as Proxmox
    // reports one: the host path it is bound from, at a mount point.
    const mounting = (
      name: string,
      parentId: string,
      disks: Array<{
        total: number;
        used: number;
        device?: string;
        mountpoint?: string;
        filesystem?: string;
      }>,
    ): Resource =>
      guest(name, parentId, {
        proxmox: {
          disks: [
            { mountpoint: '/', device: 'local-zfs:subvol-1-disk-0', total: 8e9, used: 2e9 },
            ...disks.map((disk, index) => ({
              mountpoint: disk.mountpoint ?? `/mnt/share${index}`,
              device: disk.device ?? `/mnt/${name}-${index}`,
              filesystem: disk.filesystem ?? `mp${index}`,
              total: disk.total,
              used: disk.used,
            })),
          ],
        },
      });
    const pool = (id: string, name: string, parentId: string, total: number, used: number) =>
      resource({
        id,
        type: 'storage',
        name,
        platformType: 'proxmox-pve',
        parentId,
        disk: { current: (used / total) * 100, total, used },
      });

    it('joins a machine to what runs on it and says what kind of things those are', () => {
      const diagram = diagramOf(homelab());
      expect(tileOf(diagram, 'pve')?.links?.[0]).toBe('Other 3 · Media 1 · AI 1');
      expect(tileOf(diagram, 'pve2')?.links?.[0]).toBe('Ways in 2 · Media 1');
      expect(joined(diagram, 'pve')).toEqual(
        ['ct-jellyfin', 'ct-ollama', 'ct-ps4-ctrl', 'pbs-store', 'zfs-a'].sort(),
      );
      // A thing folded into a `+N` tile is not drawn, so nothing points at it.
      expect(joined(diagram, 'ct-old-lab')).toEqual([]);
      expect(joined(diagram, 'ct-jellyfin')).toEqual(['pve']);
      // The pool on both hosts is joined to both.
      expect(joined(diagram, 'zfs-a')).toEqual(['pve', 'pve2']);
    });

    it('names the hosts a cluster backs up from, in place of the cluster', () => {
      const node = (id: string, name: string, nodeName = name) =>
        resource({
          id,
          type: 'agent',
          name,
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          proxmox: { isClusterMember: true, nodeName },
        });
      const target = (id: string, name: string, nodes?: string[]) =>
        resource({
          id,
          type: 'storage',
          name,
          platformType: 'proxmox-pve',
          proxmox: { instance: 'homelab' },
          storage: { type: 'pbs', content: 'backup', shared: true, ...(nodes ? { nodes } : {}) },
        });
      const diagram = diagramOf([
        node('n1', 'delly'),
        // Shown under a name of its own; Proxmox names it by its node name.
        node('n2', 'Media box', 'minipc'),
        guest('jellyfin', 'n1'),
        target('t1', 'pbs-delly', ['delly', 'minipc']),
        target('t2', 'pbs-offsite', ['delly']),
        target('t3', 'pbs-anywhere'),
        // A node Proxmox names that Pulse does not draw is left out.
        target('t4', 'pbs-old', ['retired']),
      ]);
      expect(
        diagram.backups?.tiles.map((tile) => [tile.name, tile.caption, tile.links ?? []]),
      ).toEqual([
        ['pbs-anywhere', 'homelab', []],
        ['pbs-delly', '2 hosts', ['Backup storage of delly, Media box']],
        ['pbs-offsite', 'delly', ['Backup storage of delly']],
        ['pbs-old', 'homelab', []],
      ]);
      expect(tileOf(diagram, 'n1')?.links).toContain('Backs up to pbs-delly, pbs-offsite');
      expect(tileOf(diagram, 'n2')?.links).toContain('Backs up to pbs-delly');
      expect(joined(diagram, 't1')).toEqual(['n1', 'n2']);
      expect(joined(diagram, 't3')).toEqual([]);
    });

    it('draws a container that carries others with them, and them after it', () => {
      const hex = (seed: string): string =>
        [...seed]
          .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
          .join('')
          .padEnd(64, '0')
          .slice(0, 64);
      const container = (name: string, networkMode = 'bridge'): Resource =>
        resource({
          id: `c-${name}`,
          type: 'app-container',
          name,
          platformType: 'docker',
          parentId: 'tower',
          status: 'running',
          docker: {
            hostSourceId: 'tower-daemon',
            containerId: hex(name),
            containerState: 'running',
            image: `example/${name}:latest`,
            networkMode,
            ...(networkMode === 'bridge' ? { networks: [{ name: 'bridge' }] } : {}),
          },
        });
      const vpn = `container:${hex('gluetun')}`;
      const unraid = [
        resource({ id: 'tower', type: 'docker-host', name: 'Tower', platformType: 'docker' }),
        container('gluetun'),
        container('iventoy'),
        container('jellyfin'),
        container('qbittorrent', vpn),
        container('prowlarr', vpn),
        container('radarr'),
      ];
      const diagram = diagramOf(unraid);
      // Gluetun is a network tool, but here it is there for two media apps.
      expect(names(diagram.groups.find((group) => group.key === 'media'))).toEqual([
        'gluetun',
        'prowlarr',
        'qbittorrent',
        'jellyfin',
        'radarr',
      ]);
      expect(names(diagram.groups.find((group) => group.key === 'network'))).toEqual(['iventoy']);
      // The machine counts it in the box it is drawn in.
      expect(tileOf(diagram, 'tower')?.links?.[0]).toBe('Media 5 · Network tools 1');

      // One that uses it from another box stays there; one switched off
      // keeps its place at the back, where a full box folds things away.
      const mixed = diagramOf([
        ...unraid,
        container('homeassistant', vpn),
        { ...container('sabnzbd', vpn), status: 'stopped' } as Resource,
      ]);
      expect(names(mixed.groups.find((group) => group.key === 'media'))).toEqual([
        'gluetun',
        'prowlarr',
        'qbittorrent',
        'jellyfin',
        'radarr',
        'sabnzbd',
      ]);
      expect(names(mixed.groups.find((group) => group.key === 'home'))).toEqual(['homeassistant']);

      // It stays in its own box while as much of what it carries is there.
      const split = diagramOf([
        ...unraid.slice(0, 2),
        container('iventoy', vpn),
        container('jellyfin', vpn),
      ]);
      expect(names(split.groups.find((group) => group.key === 'network'))).toContain('gluetun');
    });

    it('joins a guest to the pool its mount is on, by size and how full it is', () => {
      const diagram = diagramOf([
        ...homelab(),
        pool('array', 'Tower Array', 'pve2', 30 * TB, 23 * TB),
        pool('nvme', 'delly-nvme', 'pve', 1 * TB, 0.6 * TB),
        // Read by the guest a moment apart from the host: a few bytes off.
        mounting('bazarr', 'pve', [{ total: 30 * TB - 204_800, used: 23 * TB + 524_288 }]),
        mounting('sabnzbd', 'pve2', [
          { total: 30 * TB, used: 23 * TB },
          { total: 30 * TB, used: 23 * TB, mountpoint: '/tv' },
        ]),
        mounting('frigate', 'pve', [{ total: 1 * TB, used: 0.6 * TB + 12_500_000 }]),
      ]);
      expect(tileOf(diagram, 'ct-bazarr')?.links).toEqual(['Mounts Tower Array']);
      expect(tileOf(diagram, 'ct-sabnzbd')?.links).toEqual(['Mounts Tower Array']);
      expect(tileOf(diagram, 'array')?.links).toEqual(['Used by bazarr, sabnzbd']);
      expect(tileOf(diagram, 'nvme')?.links).toEqual(['Used by frigate']);
      expect(joined(diagram, 'array')).toEqual(['ct-bazarr', 'ct-sabnzbd', 'pve2']);
      expect(joined(diagram, 'ct-frigate')).toEqual(['nvme', 'pve']);
    });

    it('matches no mount it cannot tell apart, and joins guests that share one nothing holds', () => {
      const diagram = diagramOf([
        ...homelab(),
        pool('a', 'disk-a', 'pve', 2 * TB, 1 * TB),
        pool('b', 'disk-b', 'pve2', 2 * TB, 1 * TB),
        pool('empty', 'fresh', 'pve', 4 * TB, 0.001 * TB),
        // Two pools look the same: the mount is on one of them, not both.
        mounting('either', 'pve', [{ total: 2 * TB, used: 1 * TB }]),
        // Nearly empty filesystems of one size are not one filesystem.
        mounting('blank', 'pve', [{ total: 4 * TB, used: 0.001 * TB }]),
        // Somewhat fuller is a different filesystem.
        mounting('fuller', 'pve', [{ total: 2 * TB, used: 1.01 * TB }]),
        // A disk of the guest's own is never shared.
        mounting('own', 'pve', [{ total: 2 * TB, used: 1 * TB, device: '/dev/sdb1' }]),
        mounting('volume', 'pve', [{ total: 2 * TB, used: 1 * TB, device: 'tank:vm-9-disk-1' }]),
        // Two clones of one Windows template report the same disk, which is
        // a copy each, as the guest agent names it.
        mounting('clone-a', 'pve', [
          { total: 3 * TB, used: 1 * TB, device: '\\\\.\\PhysicalDrive2', filesystem: 'ntfs' },
        ]),
        mounting('clone-b', 'pve2', [
          { total: 3 * TB, used: 1 * TB, device: '\\\\.\\PhysicalDrive2', filesystem: 'ntfs' },
        ]),
        mounting('clone-c', 'pve2', [
          { total: 3 * TB, used: 1 * TB, device: 'scsi-1', filesystem: 'ext4' },
        ]),
        // Two guests share a share from somewhere Pulse does not watch.
        mounting('nfs-a', 'pve', [{ total: 9 * TB, used: 5 * TB, device: 'nas:/export/media' }]),
        mounting('nfs-b', 'pve2', [{ total: 9 * TB, used: 5 * TB, device: 'nas:/export/media' }]),
      ]);
      for (const id of [
        'ct-either',
        'ct-blank',
        'ct-fuller',
        'ct-own',
        'ct-volume',
        'ct-clone-a',
        'ct-clone-b',
      ]) {
        expect(tileOf(diagram, id)?.links).toBeUndefined();
      }
      expect(tileOf(diagram, 'ct-nfs-a')?.links).toEqual(['Shares a filesystem with nfs-b']);
      expect(tileOf(diagram, 'ct-nfs-b')?.links).toEqual(['Shares a filesystem with nfs-a']);
      expect(joined(diagram, 'ct-nfs-a')).toEqual(['ct-nfs-b', 'pve']);
      expect(tileOf(diagram, 'ct-clone-c')?.links).toBeUndefined();
    });

    it('matches each mount on its own', () => {
      const diagram = diagramOf([
        ...homelab(),
        pool('array', 'array', 'pve2', 30 * TB, 23 * TB),
        pool('nvme', 'nvme', 'pve', 1 * TB, 0.6 * TB),
        mounting('both', 'pve', [
          { total: 30 * TB, used: 23 * TB },
          { total: 1 * TB, used: 0.6 * TB },
          { total: 30 * TB, used: 23 * TB, mountpoint: '/again' },
        ]),
      ]);
      expect(tileOf(diagram, 'ct-both')?.links).toEqual(['Mounts array', 'Mounts nvme']);
      expect(joined(diagram, 'ct-both')).toEqual(['array', 'nvme', 'pve']);
    });

    it('says a share many guests mount on each, and joins only a few pair by pair', () => {
      const share = { total: 9 * TB, used: 5 * TB, device: 'nas:/export/media' };
      const many = Array.from({ length: 8 }, (_, index) =>
        mounting(`reader-${index}`, 'pve', [share]),
      );
      const diagram = diagramOf([...homelab(), ...many], 40);
      expect(tileOf(diagram, 'ct-reader-0')?.links).toEqual([
        'Shares a filesystem with reader-1, reader-2, reader-3 +4',
      ]);
      expect(joined(diagram, 'ct-reader-0')).toEqual(['pve']);
    });

    it('joins a guest to a pool its disk is on, as the platform reports it', () => {
      const diagram = diagramOf([
        ...homelab().map((entry) =>
          entry.id === 'zfs-a'
            ? ({
                ...entry,
                storage: {
                  topConsumers: [
                    {
                      resourceId: 'ct-jellyfin',
                      resourceType: 'system-container',
                      name: 'jellyfin',
                    },
                  ],
                },
              } as Resource)
            : entry,
        ),
      ]);
      expect(tileOf(diagram, 'ct-jellyfin')?.links).toEqual(['Data on local-zfs']);
      expect(tileOf(diagram, 'zfs-a')?.links).toEqual(['Used by jellyfin']);
    });

    it('reads a backup server as where others write, not where it backs up to', () => {
      const diagram = diagramOf([
        resource({ id: 'pbs', type: 'pbs', name: 'pbs', platformType: 'proxmox-pbs' }),
        resource({
          id: 'store',
          type: 'datastore',
          name: 'repo',
          platformType: 'proxmox-pbs',
          parentId: 'pbs',
          // A backup target's consumers are the guests it holds backups of.
          storage: {
            topConsumers: [{ resourceId: 'ct-ps4-ctrl', resourceType: 'vm', name: 'ps4-ctrl' }],
          },
        }),
        ...homelab(),
      ]);
      expect(tileOf(diagram, 'store')?.links).toBeUndefined();
      expect(tileOf(diagram, 'pbs')?.links).toBeUndefined();
      expect(tileOf(diagram, 'ct-ps4-ctrl')?.links).toBeUndefined();
      // It is still on the server, and rings it.
      expect(joined(diagram, 'store')).toEqual(['pbs']);
    });

    it('says which pool a backup target is kept on', () => {
      const diagram = diagramOf([
        resource({ id: 'tower', type: 'agent', name: 'Tower', platformType: 'agent' }),
        resource({
          id: 'nvme',
          type: 'storage',
          name: 'pbs_1tb_nvme',
          parentId: 'tower',
          disk: { current: 66, total: 965_562_335_232, used: 634_121_179_136 },
        }),
        resource({
          id: 'pve',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
        }),
        resource({
          id: 'target',
          type: 'storage',
          name: 'pbs-delly',
          platformType: 'proxmox-pve',
          parentId: 'pve',
          storage: { type: 'pbs' },
          disk: { current: 66, total: 965_478_449_152, used: 634_040_352_768 },
        }),
      ]);
      expect(tileOf(diagram, 'target')?.links).toEqual([
        'Backup storage of delly',
        'Stored on pbs_1tb_nvme',
      ]);
      expect(tileOf(diagram, 'nvme')?.links).toEqual(['Holds pbs-delly']);
    });
  });

  it('is empty for an empty estate', () => {
    const diagram = diagramOf([]);
    expect(diagram).toEqual({
      gateways: [],
      links: [],
      networks: [],
      groups: [],
      hostFrames: [],
      exposed: [],
      serviceCount: 0,
    });
  });

  describe('live network telemetry helpers and aggregation', () => {
    it('formats link speeds and probe latencies concisely', () => {
      expect(formatMapLinkSpeed(100)).toBe('100M');
      expect(formatMapLinkSpeed(1000)).toBe('1G');
      expect(formatMapLinkSpeed(2500)).toBe('2.5G');
      expect(formatMapLinkSpeed(10000)).toBe('10G');

      expect(formatMapLatency(0)).toBe('<1 ms');
      expect(formatMapLatency(0.4)).toBe('<1 ms');
      expect(formatMapLatency(2)).toBe('2 ms');
      expect(formatMapLatency(2.34)).toBe('2.3 ms');
      expect(formatMapLatency(12.6)).toBe('13 ms');
    });

    it('sorts tiles by combined live traffic descending and falls back to attention', () => {
      const tile = (name: string, partial: Partial<DiagramTile>): DiagramTile => ({
        id: name,
        name,
        caption: '',
        typeLabel: 'LXC',
        status: 'running',
        health: 'ok',
        navigable: true,
        ...partial,
      });
      const sorted = [
        tile('unmeasured-ok', {}),
        tile('unmeasured-crit', { health: 'critical' }),
        tile('idle-zero', { netTotalBytes: 0 }),
        tile('busy', { netTotalBytes: 4_000_000 }),
        tile('moderate', { netTotalBytes: 500_000 }),
      ].sort(compareDiagramTilesByTraffic);
      expect(sorted.map((entry) => entry.name)).toEqual([
        'busy',
        'moderate',
        'idle-zero',
        'unmeasured-crit',
        'unmeasured-ok',
      ]);
    });

    it('populates telemetry on tiles and appends latency to network gear captions', () => {
      const diagram = diagramOf([
        resource({
          id: 'pve',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          network: { rxBytes: 8_000_000, txBytes: 4_000_000 },
          agent: {
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['192.168.10.11/24'], speedMbps: 10000, linkUp: true },
              { name: 'enp2s0', addresses: [], speedMbps: 1000, linkUp: false },
            ],
          },
        }),
        guest('jellyfin', 'pve', {
          identity: { ips: ['192.168.10.50'] },
          network: { rxBytes: 2_000_000, txBytes: 500_000 },
          proxmox: {
            networkInterfaces: [
              { name: 'net0', addresses: ['192.168.10.50'], bridge: 'vmbr0', firewall: true },
            ],
          },
        }),
        resource({
          id: 'check-router',
          type: 'network-endpoint',
          name: 'Edge',
          platformType: 'availability',
          availability: {
            address: '192.168.10.1',
            targetKind: 'router',
            available: true,
            latencyMillis: 2,
          },
        }),
        resource({
          id: 'check-site',
          type: 'network-endpoint',
          name: 'Status',
          platformType: 'availability',
          availability: {
            address: '192.168.10.80',
            targetKind: 'service',
            available: true,
            latencyMillis: 14.4,
          },
        }),
      ]);

      const jellyfin = diagram.groups.find((group) => group.key === 'media')?.tiles[0];
      expect(jellyfin).toMatchObject({
        netRxBytes: 2_000_000,
        netTxBytes: 500_000,
        netTotalBytes: 2_500_000,
        trafficTier: 'medium',
        firewallEnabled: true,
      });

      const hostTile = diagram.hardware?.tiles.find((tile) => tile.id === 'pve');
      expect(hostTile).toMatchObject({
        netRxBytes: 8_000_000,
        netTxBytes: 4_000_000,
        netTotalBytes: 12_000_000,
        trafficTier: 'medium',
        linkSpeedMbps: 10000,
        linkDownCount: 1,
      });

      const router = diagram.access?.tiles.find((tile) => tile.id === 'check-router');
      expect(router).toMatchObject({
        caption: 'router · 2 ms',
        latencyMs: 2,
      });

      const check = diagram.groups.find((group) => group.key === 'checks')?.tiles[0];
      expect(check).toMatchObject({
        caption: '192.168.10.80',
        latencyMs: 14.4,
      });
    });

    it('aggregates network traffic without double-counting Proxmox hosts and guests, and falls back to hosts when workloads report none', () => {
      const diagram = diagramOf([
        // Network A: Proxmox host + two guests with rates -> sum guests only (no double-count).
        resource({
          id: 'pve',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          network: { rxBytes: 10_000_000, txBytes: 10_000_000 },
          agent: {
            networkInterfaces: [
              { name: 'eno1', addresses: ['192.168.10.11/24'], speedMbps: 10000, linkUp: true },
              { name: 'eno2', addresses: [], speedMbps: 1000, linkUp: false },
            ],
          },
        }),
        guest('jellyfin', 'pve', {
          identity: { ips: ['192.168.10.50'] },
          network: { rxBytes: 2_000_000, txBytes: 1_000_000 },
        }),
        guest('sonarr', 'pve', {
          identity: { ips: ['192.168.10.51'] },
          network: { rxBytes: 500_000, txBytes: 500_000 },
        }),
        // Network B: Docker host with rates, container with no rate -> use host rate.
        resource({
          id: 'tower',
          type: 'docker-host',
          name: 'Tower',
          platformType: 'docker',
          network: { rxBytes: 20_000_000, txBytes: 10_000_000 },
          agent: {
            networkInterfaces: [
              { name: 'eth0', addresses: ['10.0.50.10/24'], speedMbps: 2500, linkUp: true },
            ],
          },
        }),
        resource({
          id: 'c-radarr',
          type: 'app-container',
          name: 'radarr',
          platformType: 'docker',
          parentId: 'tower',
          status: 'running',
        }),
      ]);

      const lan = diagram.networks.find((network) => network.id === 'net:192.168.10.0/24');
      expect(lan).toMatchObject({
        netTotalBytes: 4_000_000,
        trafficTier: 'medium',
        linkSpeedMbps: 10000,
        linkDownCount: 1,
      });

      const dockerNet = diagram.networks.find((network) => network.id === 'net:10.0.50.0/24');
      expect(dockerNet).toMatchObject({
        netTotalBytes: 30_000_000,
        trafficTier: 'high',
        linkSpeedMbps: 2500,
      });
      expect(dockerNet?.linkDownCount).toBeUndefined();
    });

    it('propagates aggregated metrics when folding VLANs of one bridge without double-counting multi-VLAN guests', () => {
      const plugged = (
        name: string,
        vlans: number[],
        network?: { rxBytes: number; txBytes: number },
      ): Resource =>
        guest(name, 'pve', {
          ...(network ? { network } : {}),
          proxmox: {
            clusterName: 'homelab',
            networkInterfaces: vlans.map((vlan, index) => ({
              name: `net${index}`,
              addresses: [],
              bridge: 'vmbr0',
              ...(vlan ? { vlan } : {}),
            })),
          },
        });

      const diagram = diagramOf(
        [
          ...homelab(),
          plugged('svc-0', [0], { rxBytes: 600_000, txBytes: 0 }),
          plugged('svc-10', [10], { rxBytes: 400_000, txBytes: 0 }),
          plugged('svc-20', [20]),
          plugged('svc-30', [30]),
          plugged('svc-40', [40]),
          plugged('svc-50', [50]),
          plugged('svc-60', [60]),
          // Multi-VLAN guest on the same bridge should contribute its rate once.
          plugged('multi-homed', [10, 20], { rxBytes: 500_000, txBytes: 0 }),
        ],
        60,
      );

      const folded = diagram.networks.find((network) => network.id === 'bridge:homelab:vmbr0');
      expect(folded).toMatchObject({
        label: 'vmbr0',
        netTotalBytes: 1_500_000,
        trafficTier: 'medium',
      });
    });

    it('attaches planned network membership to network gear and plain checks so drawer lists match pill counts', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-1',
          type: 'agent',
          platformType: 'proxmox-pve',
          name: 'pve-1',
          status: 'online',
          agent: {
            networkInterfaces: [{ name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000 }],
          },
        }),
        resource({
          id: 'router-check',
          type: 'network-endpoint',
          name: 'Edge Router',
          status: 'online',
          availability: {
            targetKind: 'router',
            address: '10.30.0.1',
            available: true,
            latencyMillis: 2.3,
          },
        }),
        resource({
          id: 'printer-check',
          type: 'network-endpoint',
          name: 'Office Printer',
          status: 'online',
          availability: {
            targetKind: 'device',
            address: '10.30.0.50',
            available: true,
            latencyMillis: 4,
          },
        }),
      ];

      const diagram = diagramOf(resources);
      const net = diagram.networks.find((entry) => entry.label === '10.30.0.0/24');
      expect(net).toBeDefined();
      expect(net?.memberCount).toBe(3);
      const members = collectNetworkTiles(diagram, net!.id).map(({ tile }) => tile.id);
      expect(members.sort()).toEqual(['printer-check', 'pve-1', 'router-check']);
    });

    it('marks tiles with stated via or route captions so live traffic keeps connection labels intact', () => {
      const resources: Resource[] = [
        resource({ id: 'docker-1', type: 'docker-host', name: 'docker-1', status: 'online' }),
        resource({
          id: 'vpn',
          type: 'app-container',
          name: 'gluetun',
          parentId: 'docker-1',
          status: 'running',
          docker: { containerId: 'aaaaaaaaaaaaaaaa', image: 'qmcgaw/gluetun:latest' },
          network: { rxBytes: 2_000_000, txBytes: 500_000 },
        }),
        resource({
          id: 'dl',
          type: 'app-container',
          name: 'qbittorrent',
          parentId: 'docker-1',
          status: 'running',
          docker: {
            containerId: 'bbbbbbbbbbbbbbbb',
            image: 'lscr.io/linuxserver/qbittorrent',
            networkMode: 'container:aaaaaaaaaaaaaaaa',
          },
          network: { rxBytes: 5_000_000, txBytes: 1_000_000 },
        }),
      ];

      const diagram = diagramOf(resources);
      const media = diagram.groups.find((group) => group.key === 'media');
      const dlTile = media?.tiles.find((tile) => tile.id === 'dl');
      expect(dlTile?.caption).toBe('via gluetun');
      expect(dlTile?.hasConnectionCaption).toBe(true);
      expect(dlTile?.trafficTier).toBe('medium');
    });

    it('wires links for availability check targets, monitored gateways, resolvers, and unmonitored gateways', () => {
      const diagram = diagramOf([
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.10/24'] }],
            defaultGateways: [{ address: '192.168.10.1', interface: 'vmbr0' }],
            dnsServers: ['192.168.10.53'],
          },
        }),
        resource({
          id: 'pve-2',
          type: 'agent',
          name: 'minipc',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.11/24'] }],
            defaultGateways: [{ address: '10.99.0.1', interface: 'eth0' }],
            dnsServers: ['192.168.10.53'],
          },
        }),
        guest('adguard', 'pve-1', { identity: { ips: ['192.168.10.53'] } }),
        guest('jellyfin', 'pve-1', { identity: { ips: ['192.168.10.50'] } }),
        resource({
          id: 'check-router',
          type: 'network-endpoint',
          name: 'Edge Router',
          availability: {
            address: '192.168.10.1',
            targetKind: 'router',
            protocol: 'icmp',
            available: true,
            latencyMillis: 1.5,
          },
        }),
        resource({
          id: 'check-jf',
          type: 'network-endpoint',
          name: 'Jellyfin HTTPS',
          availability: {
            address: '192.168.10.50',
            targetKind: 'service',
            linkedResourceId: 'ct-jellyfin',
            protocol: 'https',
            applicationStatusCode: 200,
            available: true,
            latencyMillis: 12,
            certificate: {
              observedAt: '2026-11-27T00:00:00Z',
              notBefore: '2026-01-01T00:00:00Z',
              notAfter: '2027-01-01T00:00:00Z',
              fingerprintSha256: 'abc',
              trustStatus: 'expiring_soon' as unknown as 'trusted',
              chainValid: true,
              hostnameValid: true,
              selfSigned: false,
            },
          },
        }),
        resource({
          id: 'check-plain',
          type: 'network-endpoint',
          name: 'Minipc Check',
          availability: {
            address: '192.168.10.11',
            linkedResourceId: 'pve-2',
            protocol: 'tcp',
            port: 8006,
          },
        }),
      ]);

      const joined = (id: string) =>
        diagram.links
          .flatMap((link) =>
            link.fromId === id ? [link.toId] : link.toId === id ? [link.fromId] : [],
          )
          .sort();
      const allTiles = [
        diagram.access,
        ...diagram.groups,
        diagram.storage,
        diagram.backups,
        diagram.hardware,
      ].flatMap((group) => group?.tiles ?? []);
      const tileOf = (id: string) => allTiles.find((tile) => tile.id === id);

      // Check -> linked resource with bidirectional sentences and metadata propagation.
      expect(joined('check-jf')).toEqual(['ct-jellyfin']);
      expect(tileOf('check-jf')).toMatchObject({
        probeSummary: 'HTTPS 200',
        certWarning: 'Cert expiring soon',
        links: ['Checks jellyfin'],
      });
      expect(tileOf('ct-jellyfin')?.links).toEqual(['Checked by Jellyfin HTTPS · 12 ms']);
      expect(joined('check-plain')).toEqual(['pve-2']);
      expect(tileOf('pve-2')?.links).toContain('Checked by Minipc Check');

      // Monitored gateway -> machine, monitored resolver -> cross-host machine, unmonitored gateway -> machine.
      expect(joined('check-router')).toEqual(['pve-1']);
      expect(tileOf('check-router')?.probeSummary).toBe('ICMP');
      expect(joined('ct-adguard')).toEqual(['pve-1', 'pve-2']);
      expect(diagram.gateways.map((tile) => tile.id)).toEqual(['gateway:0']);
      expect(joined('gateway:0')).toEqual(['pve-2']);
    });

    it('propagates dockerNetworks, aggregates estateTrafficBytes without double-counting, and lists active traffic tiles', () => {
      const diagram = diagramOf([
        // Proxmox host reporting 20 MB/s with two reporting guests (3 MB/s + 1 MB/s = 4 MB/s).
        resource({
          id: 'pve',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          network: { rxBytes: 12_000_000, txBytes: 8_000_000 },
        }),
        guest('jellyfin', 'pve', { network: { rxBytes: 2_000_000, txBytes: 1_000_000 } }),
        guest('sonarr', 'pve', { network: { rxBytes: 600_000, txBytes: 400_000 } }),
        // Docker host reporting 30 MB/s whose container reports no traffic -> host contributes 30 MB/s.
        resource({
          id: 'tower',
          type: 'docker-host',
          name: 'Tower',
          platformType: 'docker',
          network: { rxBytes: 20_000_000, txBytes: 10_000_000 },
        }),
        resource({
          id: 'c-radarr',
          type: 'app-container',
          name: 'radarr',
          platformType: 'docker',
          parentId: 'tower',
          status: 'running',
          docker: { networks: [{ name: 'bridge' }, { name: 'media_net' }, { name: 'proxy_net' }] },
        }),
        // Standalone machine with no children reporting 5 MB/s -> contributes 5 MB/s.
        resource({
          id: 'nas',
          type: 'agent',
          name: 'truenas',
          platformType: 'truenas',
          network: { rxBytes: 4_000_000, txBytes: 1_000_000 },
        }),
        // Idle host with 0 B/s is excluded from active traffic tiles.
        resource({
          id: 'idle-box',
          type: 'agent',
          name: 'idle-box',
          platformType: 'agent',
          network: { rxBytes: 0, txBytes: 0 },
        }),
      ]);

      const radarr = diagram.groups
        .find((group) => group.key === 'media')
        ?.tiles.find((tile) => tile.id === 'c-radarr');
      expect(radarr?.dockerNetworks).toEqual(['media_net', 'proxy_net']);

      // Estate traffic = (3M + 1M from pve guests) + 30M (tower) + 5M (nas) = 39 MB/s (pve's 20M not double-counted).
      expect(diagram.estateTrafficBytes).toBe(39_000_000);

      // collectActiveTrafficTiles lists every unique navigable tile with netTotalBytes > 0, busiest first.
      const active = collectActiveTrafficTiles(diagram);
      expect(
        active.map(({ tile, groupLabel }) => [tile.id, groupLabel, tile.netTotalBytes]),
      ).toEqual([
        ['tower', 'Hardware', 30_000_000],
        ['pve', 'Hardware', 20_000_000],
        ['nas', 'Hardware', 5_000_000],
        ['ct-jellyfin', 'Media', 3_000_000],
        ['ct-sonarr', 'Media', 1_000_000],
      ]);

      // Quiet estate omits estateTrafficBytes and returns empty active list.
      const quiet = diagramOf([
        resource({
          id: 'quiet',
          type: 'agent',
          name: 'quiet',
          platformType: 'agent',
          network: { rxBytes: 0, txBytes: 0 },
        }),
      ]);
      expect(quiet.estateTrafficBytes).toBeUndefined();
      expect(collectActiveTrafficTiles(quiet)).toEqual([]);
    });

    it('filters the service diagram by activeTrafficOnly in exact 1-for-1 match with collectActiveTrafficTiles', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          network: { rxBytes: 100_000_000, txBytes: 10_000_000 },
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: ['10.10.0.10/24'], speedMbps: 1000 }],
            defaultGateways: [{ address: '10.10.0.1', interface: 'eno1' }],
          },
        }),
        guest('jellyfin', 'pve', {
          identity: { ips: ['10.10.0.50'] },
          network: { rxBytes: 2_000_000, txBytes: 500_000 },
        }),
        guest('sonarr', 'pve', {
          identity: { ips: ['10.10.0.51'] },
          network: { rxBytes: 0, txBytes: 0 },
        }),
        guest('radarr', 'pve', {
          identity: { ips: ['10.10.0.52'] },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const full = buildServiceDiagram(model);
      const expectedIds = collectActiveTrafficTiles(full)
        .map(({ tile }) => tile.id)
        .sort();
      expect(expectedIds).toEqual(['ct-jellyfin', 'pve']);

      const filtered = buildServiceDiagram(model, { filter: { activeTrafficOnly: true } });
      const visibleIds = [
        filtered.access,
        ...filtered.groups,
        filtered.storage,
        filtered.backups,
        filtered.hardware,
      ]
        .flatMap((group) => group?.allTiles ?? [])
        .map((tile) => tile.id)
        .sort();
      expect(visibleIds).toEqual(expectedIds);
      expect(filtered.gateways).toEqual([]);
      expect(filtered.hardware?.tiles[0].linkUtilizationPercent).toBe(80);
    });

    it('surfaces Kubernetes Ingress routes on service tiles and strips them when names are hidden', () => {
      const resources: Resource[] = [
        resource({
          id: 'k8s',
          type: 'k8s-cluster',
          name: 'homelab-k3s',
          platformType: 'kubernetes',
        }),
        resource({
          id: 'pod-grafana',
          type: 'pod',
          name: 'grafana-0',
          platformType: 'kubernetes',
          parentId: 'k8s',
          status: 'running',
          kubernetes: {
            namespace: 'monitoring',
            labels: { app: 'grafana' },
          },
        }),
        resource({
          id: 'svc-grafana',
          type: 'k8s-service',
          name: 'grafana-svc',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'monitoring',
            serviceType: 'NodePort',
            servicePorts: [{ port: 3000, nodePort: 30300 }],
            selector: { app: 'grafana' },
          },
        }),
        resource({
          id: 'ing-grafana',
          type: 'k8s-ingress',
          name: 'grafana-ingress',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'monitoring',
            serviceName: 'grafana-svc',
            hosts: ['metrics.example.test', 'dash.example.test'],
          },
        }),
      ];

      const diagram = diagramOf(resources);
      const tile = diagram.groups
        .flatMap((group) => group.tiles)
        .find((candidate) => candidate.id === 'pod-grafana')!;
      expect(tile).toMatchObject({
        caption: 'dash.example.test +1',
        hasConnectionCaption: true,
        detail: ':30300',
        links: ['Ingress: dash.example.test, metrics.example.test'],
      });

      // Anonymized diagram drops Ingress hostnames cleanly while retaining the numeric NodePort.
      const hidden = buildServiceDiagram(
        anonymizeInfrastructureMapModel(buildInfrastructureMapModel(resources)),
      );
      const hiddenTile = hidden.groups
        .flatMap((group) => group.tiles)
        .find((candidate) => candidate.id === 'pod-grafana')!;
      expect(hiddenTile.hasConnectionCaption).toBeUndefined();
      expect(hiddenTile.links).toBeUndefined();
      expect(hiddenTile.detail).toBe(':30300');
      expect(JSON.stringify(hiddenTile)).not.toContain('example.test');
    });
  });

  describe('host frames, Compose stacks, shared storage, and exposed entries', () => {
    const GIB = 1024 ** 3;
    const hex = (seed: string): string =>
      [...seed]
        .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
        .join('')
        .padEnd(64, '0')
        .slice(0, 64);

    const archEstate = (): Resource[] => [
      resource({
        id: 'pve-1',
        type: 'agent',
        name: 'pve-01',
        platformType: 'proxmox-pve',
        clusterId: 'lab-cluster',
        proxmox: {
          isClusterMember: true,
          cpus: 16,
          pveVersion: '8.3.0/abc',
          memory: { total: 64 * GIB },
        },
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'vm-notes',
        type: 'vm',
        name: 'notes-vm',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'running',
        proxmox: { vmid: 102 },
        identity: { ips: ['10.30.0.102'] },
        network: { rxBytes: 2_000_000, txBytes: 1_000_000 },
        health: { verdict: 'attention', reasons: [{ code: 'high_cpu' }] },
      }),
      guest('adguard', 'pve-1', {
        proxmox: { vmid: 105 },
        identity: { ips: ['10.30.0.53'] },
      }),
      resource({
        id: 'pve-1-zfs',
        type: 'storage',
        name: 'nvme-pool',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        disk: { current: 54, total: 2000 * GIB, used: 1080 * GIB },
        storage: {
          vdevLayout: 'mirror',
          type: 'zfspool',
          topConsumers: [{ resourceId: 'vm-notes', resourceType: 'vm', name: 'notes-vm' }],
        },
      }),
      resource({
        id: 'pve-1-pbs',
        type: 'storage',
        name: 'pbs-backup',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        disk: { current: 41, total: 4000 * GIB, used: 1640 * GIB },
        storage: { type: 'pbs', content: 'backup' },
      }),
      resource({
        id: 'cluster-nfs',
        type: 'storage',
        name: 'truenas-nfs',
        platformType: 'proxmox-pve',
        proxmox: { instance: 'lab-cluster' },
        disk: { current: 78, total: 10000 * GIB, used: 7800 * GIB },
        storage: { type: 'nfs', shared: true },
      }),
      resource({
        id: 'dock-1',
        type: 'docker-host',
        name: 'docker-prod',
        platformType: 'docker',
        docker: {
          cpus: 8,
          dockerVersion: '27.3.1',
          memory: { total: 32 * GIB },
        },
        agent: {
          networkInterfaces: [
            { name: 'eth0', addresses: ['10.30.0.20/24'], speedMbps: 2500, linkUp: true },
          ],
        },
        network: { rxBytes: 30_000_000, txBytes: 5_000_000 },
      }),
      resource({
        id: 'c-traefik',
        type: 'app-container',
        name: 'traefik',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('traefik'),
          containerState: 'running',
          image: 'traefik:v3.1',
          composeProject: 'edge-stack',
          networks: [{ name: 'proxy_net' }],
          ports: [{ publicPort: 443, privatePort: 443 }],
        },
      }),
      resource({
        id: 'c-sonarr',
        type: 'app-container',
        name: 'sonarr',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('sonarr'),
          containerState: 'running',
          image: 'lscr.io/linuxserver/sonarr:latest',
          composeProject: 'media-stack',
          networks: [{ name: 'proxy_net' }],
          ports: [{ publicPort: 8989, privatePort: 8989 }],
          labels: {
            'traefik.enable': 'true',
            'traefik.http.routers.sonarr.rule': 'Host(`tv.home.arpa`)',
          },
        },
      }),
      resource({
        id: 'c-radarr',
        type: 'app-container',
        name: 'radarr',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('radarr'),
          image: 'lscr.io/linuxserver/radarr:latest',
          composeProject: 'media-stack',
          ports: [{ publicPort: 7878, privatePort: 7878 }],
        },
      }),
      resource({
        id: 'c-grafana',
        type: 'app-container',
        name: 'grafana',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('grafana'),
          image: 'grafana/grafana:latest',
          composeProject: 'alpha-obs',
          ports: [{ publicPort: 3000, privatePort: 3000 }],
        },
      }),
      resource({
        id: 'c-prometheus',
        type: 'app-container',
        name: 'prometheus',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('prometheus'),
          image: 'prom/prometheus:latest',
          composeProject: 'alpha-obs',
        },
      }),
      resource({
        id: 'c-solo',
        type: 'app-container',
        name: 'portainer',
        platformType: 'docker',
        parentId: 'dock-1',
        status: 'running',
        docker: {
          hostSourceId: 'dock-1',
          containerId: hex('portainer'),
          image: 'portainer/portainer-ce:latest',
          composeProject: 'solo-stack',
          ports: [{ publicPort: 9000, privatePort: 9000 }],
        },
      }),
      resource({
        id: 'check-site',
        type: 'network-endpoint',
        name: 'Status page',
        platformType: 'availability',
        availability: {
          address: '10.30.0.80',
          protocol: 'https',
          applicationStatusCode: 200,
          available: true,
          latencyMillis: 8,
        },
      }),
    ];

    it('builds hostFrames with hardware specs, VMIDs, Compose stacks, local pools, sharedStorage, and checks', () => {
      const diagram = diagramOf(archEstate());

      expect(diagram.hostFrames.map((frame) => frame.id)).toEqual(['pve-1', 'dock-1']);

      const pveFrame = diagram.hostFrames[0];
      expect(pveFrame.clusterName).toBe('lab-cluster');
      expect(pveFrame.host).toMatchObject({
        name: 'pve-01',
        cpuCores: 16,
        memoryTotalBytes: 64 * GIB,
        versionLabel: 'PVE 8.3',
        linkSpeedMbps: 10000,
      });
      expect(pveFrame.linkSpeedMbps).toBe(10000);
      // Child workload traffic propagates onto the host frame trafficTier.
      expect(pveFrame.trafficTier).toBe('medium');
      // Access tile (adguard) lives in WAYS IN, never duplicated inside hostFrames.
      expect(pveFrame.looseTiles.map((tile) => tile.id)).toEqual(['vm-notes']);
      expect(pveFrame.looseTiles[0]).toMatchObject({
        vmid: 102,
        hostId: 'pve-1',
        hostName: 'pve-01',
      });
      expect(pveFrame.attentionCount).toBe(1);
      expect(pveFrame.totalCount).toBe(1);
      expect(pveFrame.overflowCount).toBe(0);
      // Local non-backup pool appears on the host frame, while backup pool stays in backups.
      expect(pveFrame.pools).toHaveLength(1);
      expect(pveFrame.pools[0]).toMatchObject({
        id: 'pve-1-zfs',
        name: 'nvme-pool',
        usedPercent: 54,
        vdevLayout: 'mirror',
        storageType: 'zfspool',
        navigable: true,
        links: ['Used by notes-vm'],
      });
      expect(pveFrame.pools[0].sizeLabel).toBeDefined();

      const dockFrame = diagram.hostFrames[1];
      expect(dockFrame.clusterName).toBeUndefined();
      expect(dockFrame.host).toMatchObject({
        name: 'docker-prod',
        cpuCores: 8,
        memoryTotalBytes: 32 * GIB,
        versionLabel: 'Docker 27.3',
      });
      expect(dockFrame.trafficTier).toBe('high');
      expect(dockFrame.linkSpeedMbps).toBe(2500);
      // Traefik is in WAYS IN, excluded from hostFrame workloads.
      expect(dockFrame.allTiles.some((tile) => tile.id === 'c-traefik')).toBe(false);
      // 2+ containers with the same composeProject form alphabetically sorted stacks.
      expect(
        dockFrame.stacks.map((stack) => [stack.label, stack.tiles.map((t) => t.name)]),
      ).toEqual([
        ['alpha-obs', ['grafana', 'prometheus']],
        ['media-stack', ['radarr', 'sonarr']],
      ]);
      // A single-container composeProject stays in looseTiles.
      expect(dockFrame.looseTiles.map((tile) => tile.name)).toEqual(['portainer']);
      expect(dockFrame.looseTiles[0].composeProject).toBe('solo-stack');

      // Shared storage excludes local pools and backup pools.
      expect(diagram.sharedStorage?.tiles.map((tile) => tile.id)).toEqual(['cluster-nfs']);
      expect(diagram.sharedStorage?.tiles[0]).toMatchObject({
        usedPercent: 78,
        storageType: 'nfs',
      });
      // Backups holds backup pools.
      expect(diagram.backups?.tiles.map((tile) => tile.id)).toEqual(['pve-1-pbs']);
      // Checks exposes plain non-gear checks directly.
      expect(diagram.checks?.tiles.map((tile) => tile.id)).toEqual(['check-site']);
    });

    it('caps hostFrames workloads via maxTilesPerGroup and resolves host:<id> via findDiagramGroup without double-counting', () => {
      const resources = archEstate();
      const diagram = diagramOf(resources, 2);
      const baseCount = countDiagramResources(diagram);

      const dockFrame = diagram.hostFrames.find((frame) => frame.id === 'dock-1')!;
      expect(dockFrame.totalCount).toBe(5);
      expect(dockFrame.overflowCount).toBe(3);
      expect(dockFrame.allTiles).toHaveLength(5);

      const hostGroup = findDiagramGroup(diagram, 'host:dock-1');
      expect(hostGroup).toMatchObject({
        key: 'host:dock-1',
        label: 'docker-prod',
        count: 5,
      });
      expect(hostGroup?.allTiles).toHaveLength(5);
      expect(findDiagramGroup(diagram, 'shared-storage')?.key).toBe('shared-storage');
      expect(findDiagramGroup(diagram, 'checks')?.key).toBe('checks');
      expect(findDiagramGroup(diagram, 'host:missing')).toBeUndefined();

      // collectDiagramGroups never double-counts host frames.
      expect(collectDiagramGroups(diagram).some((group) => group.key.startsWith('host:'))).toBe(
        false,
      );
      expect(countDiagramResources(diagram)).toBe(baseCount);
    });

    it('filters hostFrames by attentionOnly and query, keeping matching hosts or hosts with matching children', () => {
      const model = buildInfrastructureMapModel(archEstate());

      const attentionOnly = buildServiceDiagram(model, { filter: { attentionOnly: true } });
      expect(attentionOnly.hostFrames.map((frame) => frame.id)).toEqual(['pve-1']);
      expect(attentionOnly.hostFrames[0].allTiles.map((tile) => tile.id)).toEqual(['vm-notes']);

      const byStack = buildServiceDiagram(model, { filter: { query: 'media-stack' } });
      expect(byStack.hostFrames.map((frame) => frame.id)).toEqual(['dock-1']);
      expect(byStack.hostFrames[0].stacks.map((stack) => stack.label)).toEqual(['media-stack']);
      expect(byStack.hostFrames[0].looseTiles).toEqual([]);

      const byVmid = buildServiceDiagram(model, { filter: { query: '102' } });
      expect(byVmid.hostFrames.map((frame) => frame.id)).toEqual(['pve-1']);
      expect(byVmid.hostFrames[0].allTiles.map((tile) => tile.id)).toEqual(['vm-notes']);
    });

    it('collects and sorts exposed services from reverse proxies, K8s Ingresses, and published ports, and respects filters and Hide names', () => {
      const resources: Resource[] = [
        ...archEstate(),
        resource({ id: 'k8s', type: 'k8s-cluster', name: 'k3s', platformType: 'kubernetes' }),
        resource({
          id: 'pod-api',
          type: 'pod',
          name: 'api-0',
          platformType: 'kubernetes',
          parentId: 'k8s',
          status: 'running',
          kubernetes: { namespace: 'prod', labels: { app: 'api' } },
        }),
        resource({
          id: 'svc-api',
          type: 'k8s-service',
          name: 'api-svc',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'prod',
            serviceType: 'NodePort',
            servicePorts: [{ port: 8080, nodePort: 30808 }],
            selector: { app: 'api' },
          },
        }),
        resource({
          id: 'ing-api',
          type: 'k8s-ingress',
          name: 'api-ing',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'prod',
            serviceName: 'api-svc',
            hosts: ['API.Cluster.Local'],
          },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const diagram = buildServiceDiagram(model);
      const exposed = collectExposedEntries(diagram);

      // Proxied/ingress entries come first (alphabetically by name), then published-port-only entries.
      expect(
        exposed.map((entry) => ({
          id: entry.tile.id,
          hostnames: entry.hostnames,
          ports: entry.ports,
          proxyName: entry.proxyName,
          groupLabel: entry.groupLabel,
        })),
      ).toEqual([
        {
          id: 'pod-api',
          hostnames: ['api.cluster.local'],
          ports: ':30808',
          proxyName: undefined,
          groupLabel: 'Other',
        },
        {
          id: 'c-sonarr',
          hostnames: ['tv.home.arpa'],
          ports: ':8989',
          proxyName: 'traefik',
          groupLabel: 'Media',
        },
        {
          id: 'c-grafana',
          hostnames: [],
          ports: ':3000',
          proxyName: undefined,
          groupLabel: 'Monitoring',
        },
        {
          id: 'c-solo',
          hostnames: [],
          ports: ':9000',
          proxyName: undefined,
          groupLabel: 'Monitoring',
        },
        {
          id: 'c-radarr',
          hostnames: [],
          ports: ':7878',
          proxyName: undefined,
          groupLabel: 'Media',
        },
        {
          id: 'c-traefik',
          hostnames: [],
          ports: ':443',
          proxyName: undefined,
          groupLabel: 'Ways in',
        },
      ]);

      // Filtered diagram filters exposed entries to matching services.
      const filtered = buildServiceDiagram(model, { filter: { query: 'sonarr' } });
      expect(collectExposedEntries(filtered).map((entry) => entry.tile.id)).toEqual(['c-sonarr']);

      // Hide names strips Compose stacks, VMIDs, and exposed hostnames while retaining published ports and hardware specs.
      const hidden = buildServiceDiagram(anonymizeInfrastructureMapModel(model));
      const hiddenDock = hidden.hostFrames.find((frame) => frame.id === 'dock-1')!;
      const hiddenPve = hidden.hostFrames.find((frame) => frame.id === 'pve-1')!;
      expect(hiddenDock.stacks).toEqual([]);
      expect(hiddenDock.host.versionLabel).toBe('Docker 27.3');
      expect(hiddenPve.host.cpuCores).toBe(16);
      expect(hiddenPve.looseTiles[0].vmid).toBeUndefined();
      expect(hiddenPve.pools[0].vdevLayout).toBe('mirror');

      const hiddenSonarr = collectExposedEntries(hidden).find(
        (entry) => entry.tile.id === 'c-sonarr',
      )!;
      expect(hiddenSonarr.hostnames).toEqual([]);
      expect(hiddenSonarr.ports).toBe(':8989');
      expect(hiddenSonarr.proxyName).toBe('Traefik');

      const serialized = JSON.stringify([hidden.hostFrames, hidden.exposed]);
      expect(serialized).not.toContain('media-stack');
      expect(serialized).not.toContain('alpha-obs');
      expect(serialized).not.toContain('tv.home.arpa');
      expect(serialized).not.toContain('api.cluster.local');
    });

    it('attaches factual link labels across relationship types and preserves specificity over runs on', () => {
      const TB = 1_000_000_000_000;
      const resources: Resource[] = [
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.10/24'] }],
            defaultGateways: [{ address: '192.168.10.1', interface: 'vmbr0' }],
            dnsServers: ['192.168.10.53'],
          },
        }),
        resource({
          id: 'pve-2',
          type: 'agent',
          name: 'pve-02',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.11/24'] }],
            defaultGateways: [{ address: '10.99.0.1', interface: 'eth0' }],
            dnsServers: ['192.168.10.53'],
          },
        }),
        // Monitored gateway running on pve-1: 'gateway' must win over generic 'runs on'.
        guest('opnsense', 'pve-1', { identity: { ips: ['192.168.10.1'] } }),
        // Monitored DNS resolver running on pve-1: 'DNS' must win over generic 'runs on'.
        guest('adguard', 'pve-1', { identity: { ips: ['192.168.10.53'] } }),
        // Regular guest mounting a pool and targeted by a check.
        guest('jellyfin', 'pve-1', {
          identity: { ips: ['192.168.10.50'] },
          proxmox: {
            disks: [
              { mountpoint: '/', device: 'local-zfs:subvol-1-disk-0', total: 8e9, used: 2e9 },
              {
                mountpoint: '/media',
                device: '/mnt/media',
                filesystem: 'mp0',
                total: 10 * TB,
                used: 6 * TB,
              },
            ],
          },
        }),
        resource({
          id: 'pool-media',
          type: 'storage',
          name: 'media-array',
          platformType: 'proxmox-pve',
          parentId: 'pve-2',
          disk: { current: 60, total: 10 * TB, used: 6 * TB },
        }),
        resource({
          id: 'pool-local',
          type: 'storage',
          name: 'local-zfs',
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          storage: {
            topConsumers: [
              { resourceId: 'ct-adguard', resourceType: 'system-container', name: 'adguard' },
            ],
          },
        }),
        resource({
          id: 'pool-pbs',
          type: 'storage',
          name: 'pbs-store',
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          storage: { type: 'pbs', content: 'backup' },
        }),
        resource({
          id: 'check-jf',
          type: 'network-endpoint',
          name: 'Jellyfin HTTPS',
          availability: {
            address: '192.168.10.50',
            targetKind: 'service',
            linkedResourceId: 'ct-jellyfin',
          },
        }),
        resource({
          id: 'dock-1',
          type: 'docker-host',
          name: 'docker-prod',
          platformType: 'docker',
        }),
        resource({
          id: 'c-traefik',
          type: 'app-container',
          name: 'traefik',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: hex('traefik'),
            image: 'traefik:v3.1',
            networks: [{ name: 'proxy_net' }],
          },
        }),
        resource({
          id: 'c-gluetun',
          type: 'app-container',
          name: 'gluetun',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: hex('gluetun'),
            image: 'qmcgaw/gluetun:latest',
            networks: [{ name: 'proxy_net' }],
          },
        }),
        resource({
          id: 'c-sonarr',
          type: 'app-container',
          name: 'sonarr',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: hex('sonarr'),
            image: 'lscr.io/linuxserver/sonarr:latest',
            networkMode: `container:${hex('gluetun')}`,
            labels: {
              'traefik.enable': 'true',
              'traefik.http.routers.sonarr.rule': 'Host(`tv.example.test`)',
            },
          },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const diagram = buildServiceDiagram(model);
      const labelBetween = (a: string, b: string) =>
        diagram.links.find(
          (link) =>
            (link.fromId === a && link.toId === b) || (link.fromId === b && link.toId === a),
        )?.label;

      expect(labelBetween('c-traefik', 'c-sonarr')).toBe('tv.example.test');
      expect(labelBetween('c-sonarr', 'c-gluetun')).toBe('via gluetun');
      expect(labelBetween('check-jf', 'ct-jellyfin')).toBe('checks');
      expect(labelBetween('ct-opnsense', 'pve-1')).toBe('gateway');
      expect(labelBetween('gateway:0', 'pve-2')).toBe('gateway');
      expect(labelBetween('ct-adguard', 'pve-1')).toBe('DNS');
      expect(labelBetween('ct-adguard', 'pve-2')).toBe('DNS');
      expect(labelBetween('ct-jellyfin', 'pve-1')).toBe('runs on');
      expect(labelBetween('ct-jellyfin', 'pool-media')).toBe('mounts');
      expect(labelBetween('ct-adguard', 'pool-local')).toBe('storage');
      expect(labelBetween('pve-1', 'pool-local')).toBe('storage');
      expect(labelBetween('pve-1', 'pool-pbs')).toBe('backups');

      // Under Hide names, reverse proxy routes emit 'routes' without leaking hostnames.
      const hidden = buildServiceDiagram(anonymizeInfrastructureMapModel(model));
      const hiddenRoute = hidden.links.find(
        (link) => link.fromId === 'c-traefik' && link.toId === 'c-sonarr',
      );
      expect(hiddenRoute?.label).toBe('routes');
      expect(JSON.stringify(hidden.links)).not.toContain('example.test');
    });

    it('populates categoryLabel and isRouted on workload tiles for reverse proxy routes and Kubernetes Ingress', () => {
      const resources: Resource[] = [
        ...archEstate(),
        resource({ id: 'k8s', type: 'k8s-cluster', name: 'k3s', platformType: 'kubernetes' }),
        resource({
          id: 'pod-api',
          type: 'pod',
          name: 'api-0',
          platformType: 'kubernetes',
          parentId: 'k8s',
          status: 'running',
          kubernetes: { namespace: 'prod', labels: { app: 'api' } },
        }),
        resource({
          id: 'svc-api',
          type: 'k8s-service',
          name: 'api-svc',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'prod',
            serviceType: 'ClusterIP',
            selector: { app: 'api' },
          },
        }),
        resource({
          id: 'ing-api',
          type: 'k8s-ingress',
          name: 'api-ing',
          platformType: 'kubernetes',
          parentId: 'k8s',
          kubernetes: {
            namespace: 'prod',
            serviceName: 'api-svc',
            hosts: ['api.home.arpa'],
          },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const diagram = buildServiceDiagram(model);

      const dockFrame = diagram.hostFrames.find((frame) => frame.id === 'dock-1')!;
      const pveFrame = diagram.hostFrames.find((frame) => frame.id === 'pve-1')!;
      const k8sFrame = diagram.hostFrames.find((frame) => frame.id === 'k8s')!;

      const sonarr = dockFrame.allTiles.find((tile) => tile.id === 'c-sonarr')!;
      const radarr = dockFrame.allTiles.find((tile) => tile.id === 'c-radarr')!;
      const grafana = dockFrame.allTiles.find((tile) => tile.id === 'c-grafana')!;
      const notes = pveFrame.allTiles.find((tile) => tile.id === 'vm-notes')!;
      const podApi = k8sFrame.allTiles.find((tile) => tile.id === 'pod-api')!;

      expect(sonarr.categoryLabel).toBe('Media');
      expect(grafana.categoryLabel).toBe('Monitoring');
      expect(notes.categoryLabel).toBe('Other');
      expect(podApi.categoryLabel).toBe('Other');

      expect(sonarr.isRouted).toBe(true);
      expect(podApi.isRouted).toBe(true);
      expect(radarr.isRouted).toBeUndefined();
      expect(grafana.isRouted).toBeUndefined();

      // Reverse proxy isRouted remains true under Hide names while hostnames are stripped.
      const hidden = buildServiceDiagram(anonymizeInfrastructureMapModel(model));
      const hiddenSonarr = hidden.hostFrames
        .find((frame) => frame.id === 'dock-1')
        ?.allTiles.find((tile) => tile.id === 'c-sonarr');
      expect(hiddenSonarr?.categoryLabel).toBe('Media');
      expect(hiddenSonarr?.isRouted).toBe(true);
    });

    it('attaches clusterId, clusterTypeLabel, and clusterPools on multi-node clusters without duplicating them in sharedStorage', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
          clusterId: 'prod-cluster',
          proxmox: { isClusterMember: true, nodeName: 'pve-01' },
          health: { verdict: 'attention', reasons: [{ code: 'high_cpu' }] },
        }),
        resource({
          id: 'pve-2',
          type: 'agent',
          name: 'pve-02',
          platformType: 'proxmox-pve',
          clusterId: 'prod-cluster',
          proxmox: { isClusterMember: true, nodeName: 'pve-02' },
        }),
        guest('jellyfin', 'pve-1'),
        resource({
          id: 'cluster-ceph',
          type: 'storage',
          name: 'ceph-rbd',
          platformType: 'proxmox-pve',
          proxmox: { instance: 'prod-cluster' },
          disk: { current: 62, total: 8000 * GIB, used: 4960 * GIB },
          storage: {
            type: 'rbd',
            shared: true,
            nodes: ['pve-01', 'pve-02'],
            topConsumers: [
              { resourceId: 'ct-jellyfin', resourceType: 'system-container', name: 'jellyfin' },
            ],
          },
          health: { verdict: 'attention', reasons: [{ code: 'disk_high' }] },
        }),
        resource({
          id: 'cluster-pbs',
          type: 'storage',
          name: 'pbs-cluster',
          platformType: 'proxmox-pve',
          proxmox: { instance: 'prod-cluster' },
          storage: { type: 'pbs', content: 'backup', shared: true, nodes: ['pve-01', 'pve-02'] },
        }),
        resource({
          id: 'standalone-nfs',
          type: 'storage',
          name: 'unplaced-nfs',
          platformType: 'proxmox-pve',
          proxmox: { instance: 'other-cluster' },
          disk: { current: 45, total: 4000 * GIB, used: 1800 * GIB },
          storage: { type: 'nfs', shared: true },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const diagram = buildServiceDiagram(model);

      expect(diagram.hostFrames.map((frame) => frame.id)).toEqual(['pve-1', 'pve-2']);
      for (const frame of diagram.hostFrames) {
        expect(frame.clusterId).toBe('map:group:proxmox:prod-cluster');
        expect(frame.clusterName).toBe('prod-cluster');
        expect(frame.clusterTypeLabel).toBe('Proxmox cluster');
        // Non-backup cluster pools attach to the cluster's host frames; backup pools stay in backups.
        expect(frame.clusterPools?.map((tile) => tile.id)).toEqual(['cluster-ceph']);
        expect(frame.clusterPools?.[0]).toMatchObject({
          name: 'ceph-rbd',
          usedPercent: 62,
          storageType: 'rbd',
          links: ['Used by jellyfin'],
        });
      }

      // 2+ node cluster pool is NOT duplicated in sharedStorage, while unclustered shared storage stays there.
      expect(diagram.sharedStorage?.tiles.map((tile) => tile.id)).toEqual(['standalone-nfs']);
      expect(diagram.backups?.tiles.map((tile) => tile.id)).toEqual(['cluster-pbs']);

      // By-role storage group and collectDiagramGroups remain completely unaffected.
      expect(diagram.storage?.tiles.map((tile) => tile.id)).toEqual([
        'cluster-ceph',
        'standalone-nfs',
      ]);
      expect(collectDiagramGroups(diagram).map((group) => group.key)).toEqual([
        'media',
        'storage',
        'backups',
        'hardware',
      ]);

      // When a filter leaves fewer than 2 visible cluster nodes, cluster storage stays in sharedStorage so it never disappears.
      const filtered = buildServiceDiagram(model, { filter: { attentionOnly: true } });
      expect(filtered.hostFrames.map((frame) => frame.id)).toEqual(['pve-1']);
      expect(filtered.sharedStorage?.tiles.map((tile) => tile.id)).toEqual(['cluster-ceph']);

      // A 2-node cluster with no standalone shared pools omits sharedStorage entirely.
      const clusterOnly = diagramOf(resources.filter((entry) => entry.id !== 'standalone-nfs'));
      expect(clusterOnly.sharedStorage).toBeUndefined();
      expect(clusterOnly.hostFrames[0].clusterPools?.map((tile) => tile.id)).toEqual([
        'cluster-ceph',
      ]);
    });

    it('sets hasLinkSpeeds only when at least one drawn host frame, hardware tile, or network reports a positive link speed', () => {
      const withSpeed = diagramOf([
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: [], speedMbps: 10000, linkUp: true }],
          },
        }),
      ]);
      expect(withSpeed.hasLinkSpeeds).toBe(true);

      const withoutSpeed = diagramOf([
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
          agent: {
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['192.168.10.10/24'], speedMbps: 0, linkUp: true },
            ],
          },
        }),
      ]);
      expect(withoutSpeed.hasLinkSpeeds).toBeUndefined();

      expect(diagramOf(archEstate()).hasLinkSpeeds).toBe(true);
      expect(diagramOf(homelab()).hasLinkSpeeds).toBeUndefined();
    });

    it('propagates host temperature and physical disk metadata onto hostFrames and hardware tiles and matches diskSummary in search', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-nvme',
          platformType: 'proxmox-pve',
          proxmox: { temperature: 48.3 },
        }),
        resource({
          id: 'd-nvme-1',
          type: 'physical_disk',
          name: 'nvme0n1',
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          status: 'online',
          physicalDisk: { diskType: 'nvme', temperature: 42, health: 'PASSED' },
        }),
        resource({
          id: 'd-nvme-2',
          type: 'physical_disk',
          name: 'nvme1n1',
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          status: 'online',
          physicalDisk: { diskType: 'nvme', temperature: 45, health: 'PASSED' },
        }),
        resource({
          id: 'd-hdd-1',
          type: 'physical_disk',
          name: 'sda',
          platformType: 'proxmox-pve',
          parentId: 'pve-1',
          status: 'degraded',
          physicalDisk: { diskType: 'hdd', temperature: 38, health: 'FAILED' },
        }),
        resource({
          id: 'dock-1',
          type: 'docker-host',
          name: 'docker-ssd',
          platformType: 'docker',
          docker: { temperature: 55 },
        }),
        resource({
          id: 'd-ssd-1',
          type: 'physical_disk',
          name: 'sdb',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'online',
          physicalDisk: { diskType: 'ssd', temperature: 33, health: 'OK' },
        }),
      ];

      const model = buildInfrastructureMapModel(resources);
      const diagram = buildServiceDiagram(model);

      const pveFrameHost = diagram.hostFrames.find((frame) => frame.id === 'pve-1')?.host;
      const pveHardware = diagram.hardware?.tiles.find((tile) => tile.id === 'pve-1');
      const expectedPve = {
        temperatureC: 48,
        diskSummary: '2× NVMe · 1× HDD',
        diskMaxTempC: 45,
        unhealthyDiskCount: 1,
      };
      expect(pveFrameHost).toMatchObject(expectedPve);
      expect(pveHardware).toMatchObject(expectedPve);

      const dockFrameHost = diagram.hostFrames.find((frame) => frame.id === 'dock-1')?.host;
      const dockHardware = diagram.hardware?.tiles.find((tile) => tile.id === 'dock-1');
      const expectedDock = {
        temperatureC: 55,
        diskSummary: '1× SSD',
        diskMaxTempC: 33,
      };
      expect(dockFrameHost).toMatchObject(expectedDock);
      expect(dockHardware).toMatchObject(expectedDock);
      expect(dockFrameHost?.unhealthyDiskCount).toBeUndefined();

      // Query matching finds hosts by diskSummary tokens (e.g. "nvme", "hdd", "ssd").
      const nvmeSearch = buildServiceDiagram(model, { filter: { query: 'nvme' } });
      expect(nvmeSearch.hostFrames.map((frame) => frame.id)).toEqual(['pve-1']);
      expect(nvmeSearch.hardware?.tiles.map((tile) => tile.id)).toEqual(['pve-1']);

      const hddSearch = buildServiceDiagram(model, { filter: { query: 'hdd' } });
      expect(hddSearch.hostFrames.map((frame) => frame.id)).toEqual(['pve-1']);
      expect(hddSearch.hardware?.tiles.map((tile) => tile.id)).toEqual(['pve-1']);

      const ssdSearch = buildServiceDiagram(model, { filter: { query: 'ssd' } });
      expect(ssdSearch.hostFrames.map((frame) => frame.id)).toEqual(['dock-1']);
      expect(ssdSearch.hardware?.tiles.map((tile) => tile.id)).toEqual(['dock-1']);

      // Hide names preserves thermal and disk summaries on host tiles.
      const hidden = buildServiceDiagram(anonymizeInfrastructureMapModel(model));
      expect(hidden.hostFrames.find((frame) => frame.id === 'pve-1')?.host).toMatchObject(
        expectedPve,
      );
      expect(hidden.hardware?.tiles.find((tile) => tile.id === 'pve-1')).toMatchObject(expectedPve);
    });
  });
});
