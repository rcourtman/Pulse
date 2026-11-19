import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { fitServiceDiagram, layoutServiceDiagram } from '../infrastructureMapDiagram';
import { buildMapExportSvg } from '../infrastructureMapExport';
import {
  fitMapName,
  wrapMapText,
  type MapDensity,
  type MapScene,
  type SceneNode,
} from '../infrastructureMapLayout';
import {
  anonymizeInfrastructureMapModel,
  buildInfrastructureMapModel,
} from '../infrastructureMapModel';
import {
  MAP_PAGE_PALETTES,
  MAP_PALETTES,
  renderMapLinkRings,
  renderMapSceneBody,
  renderMapSceneToSvg,
} from '../infrastructureMapRender';
import { buildServiceDiagram } from '../infrastructureMapServices';

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

const guest = (name: string, extra: Partial<Resource> = {}): Resource =>
  resource({
    id: `ct-${name}`,
    type: 'system-container',
    name,
    platformType: 'proxmox-pve',
    parentId: 'pve',
    status: 'running',
    ...extra,
  });

const SERVICES = [
  'jellyfin',
  'sonarr',
  'radarr',
  'homeassistant',
  'zigbee2mqtt',
  'grafana',
  'nextcloud',
  'postgres',
  'gitea',
  'ps4-ctrl',
];

const estate = (extra: Resource[] = []): Resource[] => [
  resource({
    id: 'pve',
    type: 'agent',
    name: 'delly',
    platformType: 'proxmox-pve',
    agent: { networkInterfaces: [{ name: 'vmbr0', addresses: ['192.168.10.11/24'] }] },
  }),
  guest('cloudflared'),
  guest('adguard'),
  ...SERVICES.map((name, index) =>
    guest(name, { identity: { ips: [`192.168.10.${50 + index}`] } }),
  ),
  guest('old-lab', { status: 'stopped' }),
  resource({
    id: 'zfs',
    type: 'storage',
    name: 'local-zfs',
    platformType: 'proxmox-pve',
    parentId: 'pve',
  }),
  resource({
    id: 'pbs-store',
    type: 'storage',
    name: 'pbs-delly',
    platformType: 'proxmox-pve',
    parentId: 'pve',
    storage: { type: 'pbs' },
  }),
  ...extra,
];

const sceneOf = (resources: Resource[], width: number, density: MapDensity = 'names'): MapScene =>
  layoutServiceDiagram(buildServiceDiagram(buildInfrastructureMapModel(resources)), {
    width,
    density,
  });

const ofKind = <K extends SceneNode['kind']>(scene: MapScene, kind: K) =>
  scene.nodes.filter((node): node is Extract<SceneNode, { kind: K }> => node.kind === kind);
const texts = (scene: MapScene): string[] => ofKind(scene, 'text').map((node) => node.text);

describe('wrapMapText', () => {
  const wrap = (text: string) => wrapMapText(text, 98, 12, 500);

  it('leaves a name that fits on one line', () => {
    expect(wrap('jellyfin')).toEqual(['jellyfin']);
  });

  it('breaks after a separator, keeping the separator on the first line', () => {
    expect(wrap('Proxmox Backup Server')).toEqual(['Proxmox', 'Backup Server']);
    const [first, second] = wrap('paperless-ngx-archive');
    expect(first.endsWith('-')).toBe(true);
    expect(first + second).toBe('paperless-ngx-archive');
  });

  it('breaks where a capital starts a new word inside a name', () => {
    expect(wrap('iSponsorBlockTV')).toEqual(['iSponsorBlock', 'TV']);
  });

  it('breaks inside a word only when it has no break in reach, and never leaves a stub', () => {
    const [first, second] = wrap('averyveryverylongsinglewordname');
    expect(first.length).toBeGreaterThan(8);
    expect('averyveryverylongsinglewordname'.startsWith(first)).toBe(true);
    expect(second).toBeTruthy();
    // A word one letter too long for the line gives up three, not one.
    const [head, tail] = wrap('mmmmmmmmmm');
    expect(head + tail).toBe('mmmmmmmmmm');
    expect(tail.length).toBeGreaterThanOrEqual(3);
  });

  it('ends in an ellipsis only when two lines are not enough', () => {
    const lines = wrap('one-extremely-long-name-that-cannot-possibly-fit-on-two-lines-of-a-tile');
    expect(lines).toHaveLength(2);
    expect(lines[1].endsWith('…')).toBe(true);
    expect(wrap('paperless-ngx-archive').join('')).not.toContain('…');
  });
});

describe('fitMapName', () => {
  // The room for a name on a tile of the page.
  const fit = (text: string) => fitMapName(text, 86, 12, 500);

  it('leaves a name that fits as it is', () => {
    expect(fit('Proxmox 12')).toBe('Proxmox 12');
    expect(fit('jellyfin')).toBe('jellyfin');
  });

  it('cuts the word and keeps the number a name ends in', () => {
    expect(fit('Container 121')).toMatch(/^Cont\w*… 121$/);
    expect(fit('Container 122')).not.toBe(fit('Container 121'));
    // The caption under a tile, which names the machine or cluster it is on.
    expect(fitMapName('Proxmox cluster 12', 86, 10, 400)).toMatch(/^Proxmox \w*… 12$/);
  });

  it('cuts a name with no number at its end as before', () => {
    const cut = fit('paperless-ngx-archive');
    expect(cut.startsWith('paperless-')).toBe(true);
    expect(cut.endsWith('…')).toBe(true);
  });
});

describe('layoutServiceDiagram', () => {
  it('draws a tile for everything that runs, under a box for each purpose', () => {
    const scene = sceneOf(estate(), 1400);
    const drawn = texts(scene);
    // Each box is headed the way a table section is: its name in capitals.
    for (const title of [
      'WAYS IN',
      'MEDIA',
      'HOME AUTOMATION',
      'APPS',
      'MONITORING',
      'DATABASES',
      'DEVELOPMENT',
      'OTHER',
      'STORAGE',
      'BACKUPS',
      'HARDWARE',
    ]) {
      expect(drawn).toContain(title);
    }
    // Twelve services and the fold of the one switched off.
    expect(ofKind(scene, 'app')).toHaveLength(13);
    expect(ofKind(scene, 'app').filter((node) => node.icon)).toHaveLength(11);
    expect(ofKind(scene, 'app').find((node) => !node.icon && !node.dim)).toMatchObject({
      letters: 'PC',
    });
    expect(drawn).toContain('jellyfin');
    expect(drawn).toContain('delly');
  });

  it('joins the layers with arrows and says what each one carries', () => {
    const scene = sceneOf(estate(), 1400);
    const drawn = texts(scene);
    for (const label of ['tunnel', 'disks and volumes', 'backups', 'runs on']) {
      expect(drawn).toContain(label);
    }
    const lines = ofKind(scene, 'line');
    expect(lines.filter((node) => node.arrow).length).toBeGreaterThanOrEqual(11);
    // Only the Internet, which Pulse does not monitor, hangs on a dashed line.
    expect(lines.filter((node) => node.dashed)).toHaveLength(1);
    expect(drawn).toContain('Internet');
    expect(drawn).toContain('192.168.10.0/24 · 11 addresses');
  });

  it('opens a resource from its tile and nothing from a fold or the Internet', () => {
    const hits = ofKind(sceneOf(estate(), 1400), 'hit').map((node) => node.hit);
    expect(hits.find((hit) => hit.name === 'jellyfin')).toMatchObject({
      id: 'ct-jellyfin',
      navigable: true,
      meta: 'Jellyfin · LXC · Running · delly',
      address: '192.168.10.50',
    });
    expect(hits.find((hit) => hit.name === '+1')).toMatchObject({
      navigable: false,
      meta: '1 switched off',
    });
    expect(hits.find((hit) => hit.name === 'Internet')).toBeUndefined();
    expect(hits.find((hit) => hit.name === 'delly')).toMatchObject({ id: 'pve', navigable: true });
    expect(hits.find((hit) => hit.name === 'local-zfs')).toMatchObject({ id: 'zfs' });
  });

  it('keeps every mark inside the drawing at any width', () => {
    for (const width of [320, 375, 768, 1024, 1400, 1900]) {
      for (const density of ['names', 'marks'] as const) {
        const scene = sceneOf(estate(), width, density);
        expect(scene.width).toBeLessThanOrEqual(Math.max(width, 300));
        for (const node of scene.nodes) {
          if (node.kind === 'rect' || node.kind === 'hit') {
            expect(node.x, `${width} ${density}`).toBeGreaterThanOrEqual(0);
            expect(node.x + node.w, `${width} ${density}`).toBeLessThanOrEqual(scene.width + 0.5);
            expect(node.y + node.h).toBeLessThanOrEqual(scene.height + 0.5);
          }
          if (node.kind === 'line') {
            for (const [x, y] of node.points) {
              expect(x, `${width} ${density}`).toBeGreaterThanOrEqual(0);
              expect(x, `${width} ${density}`).toBeLessThanOrEqual(scene.width);
              expect(y).toBeLessThanOrEqual(scene.height);
            }
          }
        }
      }
    }
  });

  it('stacks the boxes on a phone and sets them side by side on a wide screen', () => {
    const boxes = (scene: MapScene) =>
      ofKind(scene, 'rect').filter(
        (node) => node.fill === 'surface' && node.stroke === 'border' && node.r === 6,
      );
    const wide = boxes(sceneOf(estate(), 1900));
    const phone = boxes(sceneOf(estate(), 375));
    expect(wide).toHaveLength(phone.length);
    expect(new Set(wide.map((box) => box.y)).size).toBeLessThan(
      new Set(phone.map((box) => box.y)).size,
    );
    expect(sceneOf(estate(), 375).height).toBeGreaterThan(sceneOf(estate(), 1900).height);
  });

  it('draws small tiles without names when asked for the compact map', () => {
    const named = sceneOf(estate(), 1400);
    const compact = sceneOf(estate(), 1400, 'marks');
    expect(texts(compact)).not.toContain('jellyfin');
    expect(texts(compact)).toContain('MEDIA');
    expect(ofKind(compact, 'app')).toHaveLength(ofKind(named, 'app').length);
    expect(compact.height).toBeLessThan(named.height);
  });

  it('marks each tile with its network only when there is more than one', () => {
    const pips = (scene: MapScene) =>
      ofKind(scene, 'rect').filter((node) => node.w === 3 && node.h === 10);
    expect(pips(sceneOf(estate(), 1400))).toHaveLength(0);
    const second = [
      guest('frigate', { identity: { ips: ['10.0.30.5'] } }),
      guest('esphome', { identity: { ips: ['10.0.30.6'] } }),
    ];
    const scene = sceneOf(estate(second), 1400);
    expect(pips(scene).length).toBeGreaterThan(10);
    expect(new Set(pips(scene).map((node) => node.fill))).toEqual(new Set(['net0', 'net1']));
    expect(texts(scene)).toContain('10.0.30.x · 2 addresses');
  });

  it('marks no tile with the colour of a network that has no pill', () => {
    const pips = (scene: MapScene) =>
      ofKind(scene, 'rect').filter((node) => node.w === 3 && node.h === 10);
    // Two guests on each of several ranges: every range is a network.
    const onRanges = (count: number) =>
      Array.from({ length: count }, (_, index) => [
        guest(`a-${index}`, { identity: { ips: [`10.0.${31 + index}.5`] } }),
        guest(`b-${index}`, { identity: { ips: [`10.0.${31 + index}.6`] } }),
      ]).flat();
    const layout = (extra: Resource[]) =>
      layoutServiceDiagram(
        buildServiceDiagram(buildInfrastructureMapModel(estate(extra)), { maxTilesPerGroup: 80 }),
        { width: 1400, density: 'names' },
      );
    // Six networks: the LAN and five ranges, each with a pill and a colour.
    const named = layout(onRanges(5));
    expect(texts(named).some((text) => /^\+\d+ more$/.test(text))).toBe(false);
    // Eight networks: the two smallest get no pill, and their four tiles no mark.
    const crowded = layout(onRanges(7));
    expect(texts(crowded)).toContain('+2 more');
    expect(pips(crowded)).toHaveLength(pips(named).length);
    expect(new Set(pips(crowded).map((node) => node.fill))).toEqual(
      new Set(['net0', 'net1', 'net2', 'net3', 'net4', 'net5']),
    );
  });

  it('counts what is plugged into a Proxmox bridge, address or not', () => {
    const onVlan = (name: string) =>
      guest(name, {
        proxmox: {
          clusterName: 'homelab',
          networkInterfaces: [{ name: 'eth0', addresses: [], bridge: 'vmbr0', vlan: 30 }],
        },
      });
    const scene = sceneOf(estate([onVlan('frigate'), onVlan('esphome')]), 1400);
    expect(texts(scene)).toContain('vmbr0 · VLAN 30 · 2 connected');

    // Two unrelated nodes with the same bridge name are told apart, and a
    // bridge that carries a known subnet names it.
    const elsewhere = guest('nvr', {
      proxmox: {
        instance: 'shed',
        networkInterfaces: [{ name: 'eth0', addresses: [], bridge: 'vmbr0', vlan: 30 }],
      },
    });
    const onLan = guest('paperless', {
      proxmox: {
        clusterName: 'homelab',
        networkInterfaces: [{ name: 'eth0', addresses: ['192.168.10.90'], bridge: 'vmbr0' }],
      },
    });
    const drawn = texts(sceneOf(estate([onVlan('frigate'), elsewhere, onLan]), 1400));
    expect(drawn).toContain('vmbr0 · VLAN 30 · homelab · 1 connected');
    expect(drawn).toContain('vmbr0 · VLAN 30 · shed · 1 connected');
    expect(drawn.some((text) => /^vmbr0 · 192\.168\.10\.0\/24 · \d+ connected$/.test(text))).toBe(
      true,
    );
  });

  it('draws only the layers the estate has', () => {
    const bare = sceneOf(
      [
        resource({ id: 'tower', type: 'docker-host', name: 'Tower', platformType: 'docker' }),
        resource({
          id: 'c1',
          type: 'app-container',
          name: 'sonarr',
          platformType: 'docker',
          parentId: 'tower',
          status: 'running',
        }),
      ],
      1200,
    );
    const drawn = texts(bare);
    expect(drawn).toContain('MEDIA');
    expect(drawn).toContain('runs on');
    for (const absent of ['Internet', 'WAYS IN', 'STORAGE', 'BACKUPS', 'disks and volumes']) {
      expect(drawn).not.toContain(absent);
    }
    expect(sceneOf([], 1200).nodes).toEqual([]);
  });

  it('renders logos, initials and arrowheads into the image', () => {
    const svg = renderMapSceneToSvg(sceneOf(estate(), 1400), MAP_PALETTES.dark);
    expect(svg).toContain('fill="#00A4DC"');
    expect(svg).toContain('>PC</text>');
    expect(svg).toContain('stroke-dasharray="5 4"');
    expect(svg).toContain('text-anchor="middle"');
    expect(svg).not.toContain('class=');
    expect(svg.match(/<path d="M[\d. ]+L[\d. ]+L[\d. ]+z"/g)?.length).toBeGreaterThanOrEqual(11);
  });

  it('exports the same diagram as a poster', () => {
    const { svg, width } = buildMapExportSvg(buildInfrastructureMapModel(estate()), {
      title: 'Infrastructure map',
      density: 'names',
      palette: MAP_PALETTES.light,
      now: new Date('2026-11-18T12:00:00Z'),
    });
    expect(width).toBe(1480);
    expect(svg).toContain('Infrastructure map');
    expect(svg).toContain('Drawn by Pulse · 2026-11-18');
    expect(svg).toContain('HOME AUTOMATION');
    expect(svg).toContain('fill="#00A4DC"');
  });

  const poster = (resources: Resource[], density: MapDensity = 'names') =>
    buildMapExportSvg(buildInfrastructureMapModel(resources), {
      title: 'Infrastructure map',
      density,
      palette: MAP_PALETTES.dark,
      now: new Date('2026-11-18T12:00:00Z'),
    });

  it('writes a long name out on two lines in the saved image, where the page cuts it short', () => {
    const resources = estate([guest('tailscale-router'), guest('paperless-ngx-archive')]);
    // The page keeps one line and leaves the rest to the hover.
    const onPage = texts(sceneOf(resources, 1400));
    expect(onPage).not.toContain('tailscale-router');
    expect(onPage).not.toContain('paperless-ngx-archive');
    expect(onPage.some((text) => text.startsWith('tailscale-') && text.endsWith('…'))).toBe(true);
    // Nobody can hover a saved image. Its wider tile takes the first name
    // whole and the second on two lines, and cuts neither.
    const { svg } = poster(resources);
    expect(svg).toMatch(/>tailscale-router<\/text>/);
    const lines = [...svg.matchAll(/>(paperless-[^<]*)<\/text><text[^>]*>([^<]+)<\/text>/g)][0];
    expect(lines?.[1].endsWith('-')).toBe(true);
    expect(`${lines?.[1]}${lines?.[2]}`).toBe('paperless-ngx-archive');
    expect(svg).not.toMatch(/>(tailscale|paperless)[^<]*…<\/text>/);
    // A name that fits stays on one line.
    expect(svg).toMatch(/>jellyfin<\/text>/);
  });

  it('writes every hidden name out whole on the page, number included', () => {
    const machine = (
      id: string,
      type: Resource['type'],
      platformType: Resource['platformType'] = 'generic',
    ): Resource => resource({ id, type, name: `secret-${id}`, platformType });
    const resources = [
      // Enough Proxmox nodes for the numbers to reach two digits.
      ...Array.from({ length: 12 }, (_, index) => machine(`pve-${index}`, 'agent', 'proxmox-pve')),
      machine('esx', 'agent', 'vmware-vsphere'),
      machine('nas', 'agent', 'truenas'),
      machine('box', 'agent'),
      machine('dock', 'docker-host'),
      machine('k8s', 'k8s-cluster'),
      machine('pbs', 'pbs'),
      machine('pmg', 'pmg'),
      resource({ id: 'jail', type: 'jail', name: 'secret-jail', parentId: 'nas' }),
    ];
    const hidden = anonymizeInfrastructureMapModel(buildInfrastructureMapModel(resources));
    const onPage = texts(
      layoutServiceDiagram(buildServiceDiagram(hidden), { width: 1400, density: 'names' }),
    );
    for (const name of [
      'Proxmox 1',
      'Proxmox 12',
      'vSphere 1',
      'TrueNAS 1',
      'Machine 1',
      'Docker 1',
      'Cluster 1',
      'Backup 1',
      'Mail 1',
      'Container 1',
    ]) {
      expect(onPage).toContain(name);
    }
    // A cut stand-in carries an ellipsis, before its number or in place of it.
    const standIn = /^(Proxmox|vSphere|TrueNAS|Machine|Docker|Cluster|Backup|Mail|Container)\b/;
    expect(onPage.filter((text) => standIn.test(text) && text.includes('…'))).toEqual([]);
    expect(onPage.join(' ')).not.toContain('secret-');
  });

  it('gives the saved image the key to its status dots', () => {
    const { svg } = poster(estate());
    for (const word of ['Healthy', 'Needs attention', 'Critical', 'Not reporting', 'Off']) {
      expect(svg).toContain(`>${word}</text>`);
    }
    // The page has its own legend under the drawing and draws none in it.
    expect(texts(sceneOf(estate(), 1400))).not.toContain('Needs attention');
  });

  it('makes the saved image wider when that keeps its layers from wrapping', () => {
    const pool = (index: number, backup = false): Resource =>
      resource({
        id: `${backup ? 'backup' : 'pool'}-${index}`,
        type: 'storage',
        name: `${backup ? 'offsite' : 'tank'}-${index}`,
        platformType: 'proxmox-pve',
        parentId: 'pve',
        ...(backup ? { storage: { type: 'pbs' } } : {}),
      });
    const many = estate([
      ...Array.from({ length: 12 }, (_, index) => pool(index)),
      ...Array.from({ length: 10 }, (_, index) => pool(index, true)),
    ]);
    const model = buildInfrastructureMapModel(many);
    const narrow = layoutServiceDiagram(buildServiceDiagram(model), {
      width: 1480,
      density: 'names',
      header: { title: 'Infrastructure map', facts: [], footer: '' },
      wrapNames: true,
    });
    const saved = poster(many);
    expect(saved.width).toBeGreaterThan(1480);
    expect(saved.height).toBeLessThan(narrow.height * 0.93);
    // A small estate gains nothing from the width and keeps the narrow image.
    expect(poster(estate()).width).toBe(1480);
  });

  it('draws with the theme’s own variables on the page and with literal colours in a file', () => {
    const scene = sceneOf(estate(), 1400);
    const page = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
    expect(page).toContain('fill="var(--color-bg-surface)" stroke="var(--color-border-base)"');
    expect(page).toContain('fill="var(--color-text-muted)"');
    const file = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
    expect(file).not.toContain('var(');
    expect(file).toContain('fill="#1e293b" stroke="#334155"');
  });

  it('draws a thing with no logo as bright initials on a slate tile, in every theme', () => {
    const scene = sceneOf(estate(), 1400);
    const monogram = /fill="#475569" stroke="#64748b"\/><text[^>]*fill="#f8fafc"[^>]*>PC<\/text>/;
    for (const palette of [MAP_PALETTES.dark, MAP_PALETTES.light]) {
      expect(renderMapSceneToSvg(scene, palette)).toMatch(monogram);
    }
    // On the page too: the tile is a fixed colour like a brand's, not a theme variable.
    for (const palette of [MAP_PAGE_PALETTES.dark, MAP_PAGE_PALETTES.light]) {
      expect(renderMapSceneBody(scene, palette, () => null)).toMatch(monogram);
    }
  });

  it('sets the Internet beside the ways in when there is room and above them when not', () => {
    const dashedTile = (scene: MapScene) =>
      ofKind(scene, 'rect').find((node) => node.dashed && node.fill === 'none');
    const card = (scene: MapScene) =>
      ofKind(scene, 'rect').find((node) => node.fill === 'surface' && node.stroke === 'border');
    const wide = sceneOf(estate(), 1400);
    expect(dashedTile(wide)!.x + dashedTile(wide)!.w).toBeLessThan(card(wide)!.x);
    expect(dashedTile(wide)!.y).toBeGreaterThanOrEqual(card(wide)!.y);
    const phone = sceneOf(estate(), 375);
    expect(dashedTile(phone)!.y + dashedTile(phone)!.h).toBeLessThan(card(phone)!.y);
  });

  it('puts storage, backups and hardware on one bottom row when they fit', () => {
    const scene = sceneOf(estate(), 1400);
    const drawn = ofKind(scene, 'text');
    const yOf = (title: string) => drawn.find((node) => node.text === title)!.y;
    expect(yOf('STORAGE')).toBe(yOf('BACKUPS'));
    expect(yOf('BACKUPS')).toBe(yOf('HARDWARE'));
    expect(yOf('HARDWARE')).toBeGreaterThan(yOf('MEDIA'));
  });

  it('heads each box with how many things it holds, folded ones included', () => {
    const drawn = texts(sceneOf(estate(), 1400));
    const at = (title: string) => drawn[drawn.indexOf(title) + 1];
    expect(at('MEDIA')).toBe('3');
    expect(at('OTHER')).toBe('2');
    expect(at('HARDWARE')).toBe('1');
    expect(drawn).toContain('switched off');
  });

  describe('connections between tiles', () => {
    const hex = (seed: string): string =>
      [...seed]
        .map((char) => char.charCodeAt(0).toString(16).padStart(2, '0'))
        .join('')
        .padEnd(64, '0')
        .slice(0, 64);
    const box = (name: string, docker: NonNullable<Resource['docker']> = {}): Resource =>
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
      });
    const dockerEstate = (extra: Resource[] = []): Resource[] => [
      resource({
        id: 'tower',
        type: 'docker-host',
        name: 'Tower',
        platformType: 'docker',
        agent: { defaultGateways: [{ address: '192.168.0.1', interface: 'eth0' }] },
      }),
      box('gluetun', { networkMode: 'bridge' }),
      box('qbittorrent', { networkMode: `container:${hex('gluetun')}`, networks: undefined }),
      box('sonarr', { networkMode: 'bridge' }),
      ...extra,
    ];
    // A second host with a tunnel on it, so the estate has a way in.
    const waysIn = (): Resource[] => [
      resource({ id: 'pve', type: 'agent', name: 'delly', platformType: 'proxmox-pve' }),
      guest('cloudflared'),
    ];
    const hitOf = (scene: MapScene, id: string) =>
      ofKind(scene, 'hit').find((node) => node.hit.id === id);
    // The connectors between layers are the only lines on the map. A stated
    // connection between two tiles adds none.
    const lineCount = (scene: MapScene) => ofKind(scene, 'line').length;

    it('draws no line between two tiles, and tells each end the other', () => {
      const plain = dockerEstate().map((resource) =>
        resource.id === 'c-qbittorrent'
          ? { ...resource, docker: { ...resource.docker, networkMode: 'bridge' } }
          : resource,
      );
      const scene = sceneOf(dockerEstate(), 1400);
      expect(lineCount(scene)).toBe(lineCount(sceneOf(plain, 1400)));
      const from = hitOf(scene, 'c-qbittorrent');
      const to = hitOf(scene, 'c-gluetun');
      expect(from?.hit.linked).toEqual(['c-gluetun']);
      expect(to?.hit.linked).toEqual(['c-qbittorrent']);
      expect(from?.hit.links).toEqual(['Uses the network of gluetun']);
      expect(to?.hit.links).toEqual(['Carries the network of 1 container']);
      // What it uses is written under its name, where the host would be.
      expect(texts(scene)).toContain('via gluetun');
      expect(hitOf(scene, 'c-sonarr')?.hit.linked).toBeUndefined();
    });

    it('tells a proxy every app it routes, at any width, and adds no line for them', () => {
      const routed = (name: string) =>
        box(name, {
          labels: { [`traefik.http.routers.${name}.rule`]: `Host(\`${name}.example.test\`)` },
        });
      const apps = ['jellyfin', 'grafana', 'nextcloud', 'gitea', 'vaultwarden', 'wiki'];
      const busy = dockerEstate([
        ...waysIn(),
        box('traefik', { image: 'traefik:v3.1' }),
        box('prowlarr', { networkMode: `container:${hex('gluetun')}`, networks: undefined }),
        ...apps.map(routed),
      ]);
      const unrouted = busy.map((resource) =>
        resource.docker?.labels || resource.docker?.networkMode?.startsWith('container:')
          ? {
              ...resource,
              docker: { ...resource.docker, labels: undefined, networkMode: 'bridge' },
            }
          : resource,
      );
      for (const width of [340, 390, 700, 1024, 1440, 2200]) {
        for (const density of ['names', 'marks'] as const) {
          const scene = sceneOf(busy, width, density);
          expect(lineCount(scene)).toBe(lineCount(sceneOf(unrouted, width, density)));
          expect([...(hitOf(scene, 'c-traefik')?.hit.linked ?? [])].sort()).toEqual(
            apps.map((name) => `c-${name}`).sort(),
          );
          for (const name of apps) {
            expect(hitOf(scene, `c-${name}`)?.hit.linked).toEqual(['c-traefik']);
          }
          expect([...(hitOf(scene, 'c-gluetun')?.hit.linked ?? [])].sort()).toEqual([
            'c-prowlarr',
            'c-qbittorrent',
          ]);
          // Every tile a link names is one that is drawn.
          const drawn = new Set(ofKind(scene, 'hit').map((node) => node.hit.id));
          for (const node of ofKind(scene, 'hit')) {
            for (const id of node.hit.linked ?? []) expect(drawn.has(id)).toBe(true);
          }
        }
      }
    });

    it('rings the tiles joined to the one pointed at, and nothing else', () => {
      const scene = sceneOf(dockerEstate(), 1400);
      const palette = MAP_PAGE_PALETTES.dark;
      const from = hitOf(scene, 'c-qbittorrent');
      const to = hitOf(scene, 'c-gluetun');
      if (!from || !to) throw new Error('both tiles should be drawn');
      const rings = renderMapLinkRings(scene, from.hit, palette);
      expect(rings.match(/<rect /g)).toHaveLength(1);
      expect(rings).toContain(`x="${to.x}" y="${to.y}"`);
      expect(rings).toContain(`stroke="${palette.accent}"`);
      expect(rings).toContain('fill="none"');
      expect(renderMapLinkRings(scene, to.hit, palette)).toContain(`x="${from.x}" y="${from.y}"`);
      const alone = hitOf(scene, 'c-sonarr');
      if (!alone) throw new Error('the tile should be drawn');
      expect(renderMapLinkRings(scene, alone.hit, palette)).toBe('');
      // The page has one empty layer for them, which the pointer passes through.
      const body = renderMapSceneBody(scene, palette, () => '/x');
      expect(body.match(/data-map-rings/g)).toHaveLength(1);
      expect(body.endsWith('<g data-map-rings="" pointer-events="none"></g>')).toBe(true);
      // A saved image has no pointer, so it has no rings: the captions say it.
      const saved = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
      expect(saved).not.toContain('data-map-rings');
      expect(saved).toContain('via gluetun');
    });

    it('links nothing for an estate that states none', () => {
      const scene = sceneOf(estate(), 1400);
      expect(
        ofKind(scene, 'hit').every(
          (node) => node.hit.links === undefined && node.hit.linked === undefined,
        ),
      ).toBe(true);
    });

    it('writes where a routed app answers under its name', () => {
      const scene = sceneOf(
        dockerEstate([
          box('traefik', { image: 'traefik:v3.1', networks: [{ name: 'bridge' }] }),
          box('wiki', {
            networks: [{ name: 'bridge' }],
            labels: { 'traefik.http.routers.wiki.rule': 'Host(`wiki.example.test`)' },
          }),
        ]),
        1400,
      );
      expect(texts(scene).some((text) => text.startsWith('wiki.examp'))).toBe(true);
      expect(hitOf(scene, 'c-wiki')?.hit.links).toEqual(['Routed by traefik at wiki.example.test']);
      expect(hitOf(scene, 'c-traefik')?.hit.links).toEqual(['Routes 1 container']);
      expect(hitOf(scene, 'c-wiki')?.hit.linked).toEqual(['c-traefik']);
    });

    it('draws a gateway known only by address as a dashed tile that says who uses it', () => {
      const scene = sceneOf(dockerEstate(waysIn()), 1400);
      const gateway = ofKind(scene, 'hit').find((node) => node.hit.id === 'gateway:0');
      expect(gateway?.hit).toMatchObject({
        name: '192.168.0.1',
        meta: 'Default gateway, not monitored',
        navigable: false,
        links: ['Used by Tower'],
      });
      // No tile is ringed for it: the machines that use it are named on its hover.
      expect(gateway?.hit.linked).toBeUndefined();
      expect(texts(scene)).toContain('192.168.0.1');
      expect(texts(scene)).toContain('default gateway');
      expect(
        ofKind(scene, 'rect').some(
          (node) => node.dashed && node.x === gateway?.x && node.y === gateway?.y,
        ),
      ).toBe(true);
      // Nothing opens from it: Pulse does not monitor it.
      expect(renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => '/x')).not.toContain(
        'aria-label="192.168.0.1, Default gateway, not monitored"><rect',
      );
    });

    it('still draws the gateway when the estate has no ways in', () => {
      const scene = sceneOf(dockerEstate(), 1400);
      expect(texts(scene)).toContain('192.168.0.1');
      expect(texts(scene)).not.toContain('Internet');
    });

    it('keeps the gateway inside the drawing at any width', () => {
      for (const width of [340, 390, 768, 1024, 1440, 2200]) {
        for (const density of ['names', 'marks'] as const) {
          const scene = sceneOf(dockerEstate(waysIn()), width, density);
          for (const node of ofKind(scene, 'hit')) {
            expect(node.x).toBeGreaterThanOrEqual(0);
            expect(node.x + node.w).toBeLessThanOrEqual(scene.width);
          }
          // The gateway is on the map at every width, not only where there
          // is room beside the ways in.
          expect(hitOf(scene, 'gateway:0')).toBeDefined();
          expect(hitOf(scene, 'c-qbittorrent')?.hit.links).toEqual(['Uses the network of gluetun']);
        }
      }
    });
  });

  describe('fitServiceDiagram', () => {
    const diagram = buildServiceDiagram(buildInfrastructureMapModel(estate()));

    it('draws at full size when the space holds the map', () => {
      const { scene, scale } = fitServiceDiagram(diagram, { width: 1800, height: 1200 }, 'names');
      expect(scale).toBe(1);
      expect(scene.width).toBe(1800);
      expect(scene.height).toBeLessThanOrEqual(1200);
    });

    it('shrinks the map so that all of it fits a space too short for it', () => {
      for (const space of [
        { width: 1322, height: 420 },
        { width: 1000, height: 380 },
        { width: 700, height: 500 },
      ]) {
        const { scene, scale } = fitServiceDiagram(diagram, space, 'names');
        expect(scale, JSON.stringify(space)).toBeLessThan(1);
        expect(scale).toBeGreaterThan(0.3);
        expect(scene.width * scale).toBeLessThanOrEqual(space.width + 0.5);
        expect(scene.height * scale).toBeLessThanOrEqual(space.height + 0.5);
      }
    });

    it('lays the map out wider than the space when that leaves it drawn larger', () => {
      const space = { width: 1000, height: 380 };
      const { scene, scale } = fitServiceDiagram(diagram, space, 'names');
      expect(scene.width).toBeGreaterThan(space.width);
      // Laid out at the space's own width it would be taller and so smaller.
      const plain = layoutServiceDiagram(diagram, { width: space.width, density: 'names' });
      expect(scale).toBeGreaterThan(space.height / plain.height);
    });

    it('fits more of the map in when it draws icons only', () => {
      const space = { width: 1000, height: 380 };
      const named = fitServiceDiagram(diagram, space, 'names');
      const compact = fitServiceDiagram(diagram, space, 'marks');
      expect(compact.scale).toBeGreaterThan(named.scale);
    });

    it('stops shrinking at a floor, below which the page scrolls instead', () => {
      expect(fitServiceDiagram(diagram, { width: 320, height: 60 }, 'names').scale).toBe(0.3);
    });
  });
});
