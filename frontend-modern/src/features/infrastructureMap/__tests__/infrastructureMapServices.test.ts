import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { MAP_APP_ICONS } from '../infrastructureMapAppIcons';
import {
  MAP_CATALOGUE_APPS,
  resolveMapApp,
  resolveMapAppLogo,
  resolveMapCategory,
  resolveMapDeviceRole,
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
      key === 'id' || key === 'segmentId' ? undefined : value,
    );
    expect(drawn).not.toMatch(/delly|minipc|Tower|rstack|ps4|authentik|192\.168/);
  });

  it('knows network gear from what its owner called the check', () => {
    for (const name of ['Router', 'OPNsense', 'pfsense-fw', 'FRITZ!Box 7590', 'UDM Pro']) {
      expect(resolveMapDeviceRole(name)).toBe('router');
    }
    for (const name of ['Core switch', 'switch-24-rack', 'USW_Switch_8']) {
      expect(resolveMapDeviceRole(name)).toBe('switch');
    }
    for (const name of ['AP living room', 'ap-garage', 'Wi-Fi upstairs', 'Access Point 2']) {
      expect(resolveMapDeviceRole(name)).toBe('access-point');
    }
    // A wireless router is the router. A word inside another word is not it.
    expect(resolveMapDeviceRole('Router AP')).toBe('router');
    for (const name of ['Status page', 'API gateway', 'Nintendo Switch', 'apt-cache', 'Grafana']) {
      expect(resolveMapDeviceRole(name)).toBeUndefined();
    }
  });

  it('draws a check on network gear as that gear, with the ways in', () => {
    const check = (id: string, name: string, address: string): Resource =>
      resource({
        id,
        type: 'network-endpoint',
        name,
        platformType: 'availability',
        availability: { address },
      } as Parameters<typeof resource>[0]);
    const resources = [
      ...homelab(),
      check('check-switch', 'Core switch', '192.168.10.2'),
      check('check-ap', 'AP living room', '192.168.10.3'),
      check('check-fw', 'OPNsense', '192.168.10.1'),
      check('check-site', 'Status page', 'status.example.test'),
    ];
    const diagram = diagramOf(resources);

    const gear = (diagram.access?.tiles ?? []).filter((tile) => tile.id.startsWith('check-'));
    expect(gear.map((tile) => [tile.name, tile.caption, tile.platform])).toEqual([
      ['AP living room', 'access point', 'access-point'],
      ['Core switch', 'switch', 'switch'],
      ['OPNsense', 'router', 'gateway'],
    ]);
    // Each opens its check, and keeps its address for the hover.
    expect(gear.every((tile) => tile.navigable && tile.typeLabel === 'Check')).toBe(true);
    expect(gear.map((tile) => tile.address)).toEqual([
      '192.168.10.3',
      '192.168.10.2',
      '192.168.10.1',
    ]);
    // A check on anything else stays a check.
    expect(names(diagram.groups.find((group) => group.key === 'checks'))).toEqual(['Status page']);

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
    expect(names(hidden.groups.find((group) => group.key === 'checks'))).toEqual(['Check 1']);
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

      expect(diagram.links).toEqual([
        { fromId: 'c-prowlarr', toId: 'c-gluetun' },
        { fromId: 'c-qbittorrent', toId: 'c-gluetun' },
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
      const drawn = new Set(diagram.groups.flatMap((group) => group.tiles.map((tile) => tile.id)));
      expect(diagram.links.length).toBeGreaterThan(0);
      expect(diagram.links.length).toBeLessThan(6);
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

      expect(diagram.links).toEqual([{ fromId: 'c-traefik', toId: 'c-jellyfin' }]);
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
            availability: { address: '192.168.0.1' },
          } as Parameters<typeof resource>[0]),
        ]),
      );

      // Monitored now, so nothing says it is not. A check on a machine's
      // default gateway is on the router whatever it is called, so it is
      // drawn as one, where the dashed tile was.
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
      expect(diagram.links).toEqual([]);
      expect(tileOf(diagram, 'tower')?.links).toBeUndefined();
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

      expect(diagram.links).toEqual([{ fromId: 'c-traefik', toId: 'c-wiki-internal' }]);
      expect(diagram.gateways.map((tile) => tile.name)).toEqual(['Gateway']);
      expect(written).toContain('Routed by Traefik');
      expect(written).not.toContain('example.test');
      expect(written).not.toContain('192.168.0.1');
      expect(written).not.toContain('wiki-internal');
      expect(written).not.toContain('Tower');
    });
  });

  it('is empty for an empty estate', () => {
    const diagram = diagramOf([]);
    expect(diagram).toEqual({
      gateways: [],
      links: [],
      networks: [],
      groups: [],
      serviceCount: 0,
    });
  });
});
