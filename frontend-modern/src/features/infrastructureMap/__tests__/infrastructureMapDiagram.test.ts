import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  fitServiceDiagram,
  flowServiceDiagram,
  layoutServiceDiagram,
} from '../infrastructureMapDiagram';
import { buildMapExportSvg } from '../infrastructureMapExport';
import {
  fitMapName,
  nextMapHitIndex,
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
  MAP_MAX_LINK_ARCS,
  MAP_PAGE_PALETTES,
  MAP_PALETTES,
  renderMapLinkRings,
  renderMapNetworkRings,
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

  it('fills the first line when an early break would cut a name two lines can hold', () => {
    const lines = wrap('VM abcdefghijklmnopqrst');
    expect(lines).toHaveLength(2);
    expect(lines.join('')).toBe('VM abcdefghijklmnopqrst');
    // A name two lines cannot hold keeps its break at the word.
    const [first, second] = wrap(`VM ${'abcdefghij'.repeat(6)}`);
    expect(first).toBe('VM');
    expect(second.endsWith('…')).toBe(true);
  });

  it('breaks inside a word only where that leaves enough of it each side to read', () => {
    // Every name, at every width a tile gives it: where both lines together
    // are the whole name and the break falls inside a word, at least four
    // letters of that word are on each line. `Frontend Swa` over `rm service`
    // and `Document ing` over `est readiness` were what it did before.
    const names = [
      'Frontend Swarm service',
      'Document ingest readiness',
      'Workshop door controller',
      'ops-observability-01',
      'VM abcdefghijklmnopqrst',
      'Solar inverter web panel',
    ];
    const inWord = (character: string | undefined) => /[A-Za-z0-9]/.test(character ?? '');
    let inside = 0;
    for (const name of names) {
      for (let width = 60; width <= 140; width += 1) {
        const lines = wrapMapText(name, width, 12, 500);
        if (lines.length !== 2 || lines[1].endsWith('…')) continue;
        const at = lines[0].length;
        // Only a break with a letter on both sides is inside a word.
        if (lines[0] + lines[1] !== name) continue;
        if (!inWord(name[at - 1]) || !inWord(name[at])) continue;
        if (/[a-z0-9]/.test(name[at - 1]) && /[A-Z]/.test(name[at])) continue;
        inside += 1;
        const before = /[A-Za-z0-9]+$/.exec(lines[0])?.[0] ?? '';
        const after = /^[A-Za-z0-9]+/.exec(lines[1])?.[0] ?? '';
        const where = `${name} at ${width}: ${JSON.stringify(lines)}`;
        expect(before.length, where).toBeGreaterThanOrEqual(4);
        expect(after.length, where).toBeGreaterThanOrEqual(4);
      }
    }
    // The long words do still break inside, so the check above saw something.
    expect(inside).toBeGreaterThan(0);
    // And the whole name is still shown where a long word allows it, or a
    // break just before a hyphen does.
    expect(wrap('ops-observability-01').join('')).toBe('ops-observability-01');
    expect(wrap('Frontend-Node-Workshop').join('')).toBe('Frontend-Node-Workshop');
  });

  it('never ends a line between the two halves of an emoji', () => {
    const whole = (line: string) =>
      !/[\uD800-\uDBFF]$/.test(line) && !/^[\uDC00-\uDFFF]/.test(line);
    for (const name of ['VM mmmmm😀abcdef', 'mmmmmmmmm😀mmmmm', '😀😀😀😀😀😀😀😀😀😀😀😀']) {
      for (let width = 20; width <= 120; width += 1) {
        for (const line of wrapMapText(name, width, 12, 500)) {
          expect(whole(line), `${name} at ${width}: ${JSON.stringify(line)}`).toBe(true);
        }
      }
    }
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

describe('nextMapHitIndex, the arrow keys', () => {
  const tilesAt = (places: Array<[number, number]>): MapScene =>
    ({
      width: 400,
      height: 400,
      nodes: places.map(([x, y], index) => ({
        kind: 'hit',
        x,
        y,
        w: 40,
        h: 40,
        r: 6,
        hit: { id: `t${index}`, name: `t${index}`, meta: '' },
      })),
    }) as unknown as MapScene;
  // Two rows of three, and a row of one under the middle:
  //   0 1 2
  //   3 4 5
  //     6
  const scene = tilesAt([
    [0, 0],
    [44, 0],
    [88, 0],
    [0, 44],
    [44, 44],
    [88, 44],
    [44, 88],
  ]);

  it('goes to the tile that lies the way the arrow points', () => {
    expect(nextMapHitIndex(scene, 0, 'ArrowRight')).toBe(1);
    expect(nextMapHitIndex(scene, 1, 'ArrowLeft')).toBe(0);
    expect(nextMapHitIndex(scene, 1, 'ArrowDown')).toBe(4);
    expect(nextMapHitIndex(scene, 4, 'ArrowUp')).toBe(1);
    expect(nextMapHitIndex(scene, 4, 'ArrowDown')).toBe(6);
    expect(nextMapHitIndex(scene, 6, 'ArrowUp')).toBe(4);
  });

  it('carries a row on into the next one, and back, when nothing lies to the side', () => {
    expect(nextMapHitIndex(scene, 2, 'ArrowRight')).toBe(3);
    expect(nextMapHitIndex(scene, 3, 'ArrowLeft')).toBe(2);
    expect(nextMapHitIndex(scene, 5, 'ArrowRight')).toBe(6);
    // The first and last tile are the ends.
    expect(nextMapHitIndex(scene, 0, 'ArrowLeft')).toBeNull();
    expect(nextMapHitIndex(scene, 6, 'ArrowRight')).toBeNull();
  });

  it('takes the nearest tile off to one side when none is in line, and stops at an edge', () => {
    expect(nextMapHitIndex(scene, 3, 'ArrowDown')).toBe(6);
    expect(nextMapHitIndex(scene, 5, 'ArrowDown')).toBe(6);
    expect(nextMapHitIndex(scene, 6, 'ArrowDown')).toBeNull();
    expect(nextMapHitIndex(scene, 1, 'ArrowUp')).toBeNull();
  });

  it('goes along the row to a tile in line, past a nearer one on another row', () => {
    const apart = tilesAt([
      [0, 0],
      [44, 60],
      [200, 10],
    ]);
    expect(nextMapHitIndex(apart, 0, 'ArrowRight')).toBe(2);
    expect(nextMapHitIndex(apart, 2, 'ArrowLeft')).toBe(0);
    // Down, a tile in line is preferred to a nearer one off to the side.
    const stacked = tilesAt([
      [0, 0],
      [60, 44],
      [10, 200],
    ]);
    expect(nextMapHitIndex(stacked, 0, 'ArrowDown')).toBe(2);
  });

  it('goes to the first and last tile on Home and End, and nowhere from a tile that is not there', () => {
    expect(nextMapHitIndex(scene, 4, 'Home')).toBe(0);
    expect(nextMapHitIndex(scene, 4, 'End')).toBe(6);
    expect(nextMapHitIndex(scene, 0, 'Home')).toBeNull();
    expect(nextMapHitIndex(scene, 6, 'End')).toBeNull();
    expect(nextMapHitIndex(scene, 99, 'ArrowRight')).toBeNull();
    expect(nextMapHitIndex(tilesAt([[0, 0]]), 0, 'End')).toBeNull();
  });

  it('reaches every tile of a drawn map with the arrows alone, each press going the way it says', () => {
    const many = Array.from({ length: 40 }, (_, index) => guest(`svc-${index}`));
    const diagram = buildServiceDiagram(buildInfrastructureMapModel(estate(many)), {
      maxTilesPerGroup: 80,
    });
    const densities = ['marks', 'names'] as const;
    const scenes = [
      ...[320, 374].flatMap((width) =>
        densities.map((density) => flowServiceDiagram(diagram, width, density).scene),
      ),
      ...[768, 1024, 1440].flatMap((width) =>
        densities.map((density) => layoutServiceDiagram(diagram, { width, density })),
      ),
    ];
    for (const drawn of scenes) {
      const tiles = drawn.nodes.filter(
        (node): node is Extract<SceneNode, { kind: 'hit' }> => node.kind === 'hit',
      );
      expect(tiles.length).toBeGreaterThan(40);
      const seen = new Set([0]);
      const queue = [0];
      while (queue.length > 0) {
        const from = queue.shift()!;
        for (const key of ['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'] as const) {
          const to = nextMapHitIndex(drawn, from, key);
          if (to === null) continue;
          const a = tiles[from];
          const b = tiles[to];
          const where = `${drawn.width} ${key} ${a.hit.name} to ${b.hit.name}`;
          if (key === 'ArrowDown') expect(b.y, where).toBeGreaterThanOrEqual(a.y + a.h);
          if (key === 'ArrowUp') expect(b.y + b.h, where).toBeLessThanOrEqual(a.y);
          if (key === 'ArrowRight') expect(b.x >= a.x + a.w || to === from + 1, where).toBe(true);
          if (key === 'ArrowLeft') expect(b.x + b.w <= a.x || to === from - 1, where).toBe(true);
          if (!seen.has(to)) {
            seen.add(to);
            queue.push(to);
          }
        }
      }
      const missed = tiles.filter((_, index) => !seen.has(index)).map((tile) => tile.hit.name);
      expect(missed, `${drawn.width}`).toEqual([]);
    }
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
    // The lines that start or end on one of these tiles: none ever should.
    const linesAt = (scene: MapScene, ids: string[]) => {
      const tiles = ofKind(scene, 'hit').filter((node) => ids.includes(node.hit.id));
      expect(tiles).toHaveLength(ids.length);
      return ofKind(scene, 'line').filter((line) =>
        [line.points[0], line.points[line.points.length - 1]].some(([x, y]) =>
          tiles.some(
            (tile) => x >= tile.x && x <= tile.x + tile.w && y >= tile.y && y <= tile.y + tile.h,
          ),
        ),
      );
    };

    it('draws no line between two tiles, and tells each end the other', () => {
      const plain = dockerEstate().map((resource) =>
        resource.id === 'c-qbittorrent'
          ? { ...resource, docker: { ...resource.docker, networkMode: 'bridge' } }
          : resource,
      );
      const scene = sceneOf(dockerEstate(), 1400);
      // Gluetun moves into the box of what it carries, which can take a box
      // and its connector away, never add one.
      expect(lineCount(scene)).toBeLessThanOrEqual(lineCount(sceneOf(plain, 1400)));
      expect(linesAt(scene, ['c-gluetun', 'c-qbittorrent', 'c-sonarr', 'tower'])).toEqual([]);
      const from = hitOf(scene, 'c-qbittorrent');
      const to = hitOf(scene, 'c-gluetun');
      // Each is joined to the machine it runs on, too.
      expect(from?.hit.linked).toEqual(['tower', 'c-gluetun']);
      expect(to?.hit.linked).toEqual(['tower', 'c-qbittorrent']);
      expect(from?.hit.links).toEqual(['Uses the network of gluetun']);
      expect(to?.hit.links).toEqual(['Carries the network of 1 container']);
      // What it uses is written under its name, where the host would be.
      expect(texts(scene)).toContain('via gluetun');
      expect(hitOf(scene, 'c-sonarr')?.hit.linked).toEqual(['tower']);
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
          expect(lineCount(scene)).toBeLessThanOrEqual(
            lineCount(sceneOf(unrouted, width, density)),
          );
          expect(
            linesAt(scene, [
              'c-traefik',
              'c-gluetun',
              'c-prowlarr',
              'tower',
              ...apps.map((name) => `c-${name}`),
            ]),
          ).toEqual([]);
          expect([...(hitOf(scene, 'c-traefik')?.hit.linked ?? [])].sort()).toEqual(
            [...apps.map((name) => `c-${name}`), 'tower'].sort(),
          );
          for (const name of apps) {
            expect(hitOf(scene, `c-${name}`)?.hit.linked).toEqual(['tower', 'c-traefik']);
          }
          expect([...(hitOf(scene, 'c-gluetun')?.hit.linked ?? [])].sort()).toEqual([
            'c-prowlarr',
            'c-qbittorrent',
            'tower',
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
      const machine = hitOf(scene, 'tower');
      if (!machine) throw new Error('the machine should be drawn');
      const rings = renderMapLinkRings(scene, from.hit, palette);
      expect(rings.match(/<rect /g)).toHaveLength(2);
      expect(rings).toContain(`x="${to.x}" y="${to.y}"`);
      expect(rings).toContain(`x="${machine.x}" y="${machine.y}"`);
      expect(rings).toContain(`stroke="${palette.accent}"`);
      expect(rings).toContain('fill="none"');
      expect(renderMapLinkRings(scene, to.hit, palette)).toContain(`x="${from.x}" y="${from.y}"`);
      // A container with no connection of its own rings only its machine.
      const alone = hitOf(scene, 'c-sonarr');
      if (!alone) throw new Error('the tile should be drawn');
      const lone = renderMapLinkRings(scene, alone.hit, palette);
      expect(lone.match(/<rect /g)).toHaveLength(1);
      expect(lone).toContain(`x="${machine.x}" y="${machine.y}"`);
      expect(renderMapLinkRings(scene, to.hit, palette)).not.toBe('');
      // The page has one empty layer for them, which the pointer passes through.
      const body = renderMapSceneBody(scene, palette, () => '/x');
      expect(body.match(/data-map-rings/g)).toHaveLength(1);
      // A link takes the keyboard focus itself, so the ring on its rect is
      // styled through the link. Safari draws no focus of its own here.
      const link = body.match(/<a [^>]*>\s*<rect [^>]*>/)?.[0] ?? '';
      expect(link).toContain('class="group outline-none"');
      expect(link).toContain('group-focus-visible:stroke-blue-500');
      expect(link).toContain('[vector-effect:non-scaling-stroke]');
      // No stroke at rest, which would take the pointer past the tile's edge.
      expect(link).not.toContain('stroke-transparent');
      expect(body.endsWith('<g data-map-rings="" pointer-events="none"></g>')).toBe(true);
      // A saved image has no pointer, so it has no rings: the captions say it.
      const saved = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
      expect(saved).not.toContain('data-map-rings');
      expect(saved).toContain('via gluetun');
    });

    it('links only a machine to what is on it, for an estate that states nothing else', () => {
      const scene = sceneOf(estate(), 1400);
      for (const node of ofKind(scene, 'hit')) {
        if (node.hit.id === 'pve') {
          expect(node.hit.links).toEqual([
            'Media 3 · Ways in 2 · Home automation 2 · Other 2 · Apps 1 · Monitoring 1 · Databases 1 · Development 1',
            'Backs up to pbs-delly',
          ]);
          continue;
        }
        if (node.hit.id === 'pbs-store') {
          expect(node.hit.links).toEqual(['Backup storage of delly']);
        } else {
          expect(node.hit.links).toBeUndefined();
        }
        // A folded tile stands for several things and is joined to nothing.
        expect(node.hit.linked).toEqual(node.hit.navigable ? ['pve'] : undefined);
      }
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
      expect(hitOf(scene, 'c-wiki')?.hit.linked).toEqual(['tower', 'c-traefik']);
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
      // Unmonitored gateway tiles link to the drawn machines that use them.
      expect(gateway?.hit.linked).toEqual(['tower']);
      expect(texts(scene)).toContain('192.168.0.1');
      expect(texts(scene)).toContain('default gateway');
      expect(
        ofKind(scene, 'rect').some(
          (node) => node.dashed && node.x === gateway?.x && node.y === gateway?.y,
        ),
      ).toBe(true);
      // It has no page of its own. Where the page gives it the form that adds a
      // check it is a link and says so. Where the page gives it nothing it is a
      // plain tile and promises nothing.
      const offered = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => '/x');
      expect(offered).toMatch(
        /<a href="\/x"[^>]*aria-label="192\.168\.0\.1, Default gateway, not monitored, add a check">/,
      );
      const withheld = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
      expect(withheld).toMatch(
        /<rect [^>]*role="img" aria-label="192\.168\.0\.1, Default gateway, not monitored"\/>/,
      );
      expect(withheld).not.toContain('add a check');
    });

    it('draws every gateway nothing monitors, on as many rows as a narrow window needs', () => {
      // Three machines, each behind its own router: the most the map draws.
      const routed = (extra: Resource[] = []): Resource[] => [
        ...[1, 2, 3].map((n) =>
          resource({
            id: `host-${n}`,
            type: 'docker-host',
            name: `Host ${n}`,
            platformType: 'docker',
            agent: { defaultGateways: [{ address: `192.168.${n}.1`, interface: 'eth0' }] },
          }),
        ),
        ...extra,
      ];
      const gatewaysOf = (scene: MapScene) =>
        ofKind(scene, 'hit').filter((node) => node.hit.checkAddress !== undefined);
      for (const estateOf of [routed, () => routed(waysIn())]) {
        for (const width of [320, 374, 768, 1440]) {
          for (const density of ['names', 'marks'] as const) {
            const scene = sceneOf(estateOf(), width, density);
            const where = `${width} ${density}`;
            const gateways = gatewaysOf(scene);
            expect(gateways.map((node) => node.hit.checkAddress).sort(), where).toEqual([
              '192.168.1.1',
              '192.168.2.1',
              '192.168.3.1',
            ]);
            // Inside the drawing, and no tile on top of another.
            const hits = ofKind(scene, 'hit');
            for (const node of gateways) {
              expect(node.x, where).toBeGreaterThanOrEqual(0);
              expect(node.x + node.w, where).toBeLessThanOrEqual(scene.width);
              for (const other of hits) {
                if (other === node) continue;
                const apart =
                  node.x + node.w <= other.x ||
                  other.x + other.w <= node.x ||
                  node.y + node.h <= other.y ||
                  other.y + other.h <= node.y;
                expect(apart, `${where} ${node.hit.name} over ${other.hit.name}`).toBe(true);
              }
            }
          }
        }
      }
    });

    it('gives the gateways a second row on a narrow phone with names on, clear of the Internet', () => {
      const routed: Resource[] = [
        ...[1, 2, 3].map((n) =>
          resource({
            id: `host-${n}`,
            type: 'docker-host',
            name: `Host ${n}`,
            platformType: 'docker',
            agent: { defaultGateways: [{ address: `192.168.${n}.1`, interface: 'eth0' }] },
          }),
        ),
        ...waysIn(),
      ];
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(routed));
      const rowsAt = (width: number) => {
        const { scene, scale } = flowServiceDiagram(diagram, width, 'names');
        expect(scale, `${width}`).toBe(1);
        const hits = ofKind(scene, 'hit');
        const gateways = hits.filter((node) => node.hit.checkAddress !== undefined);
        expect(gateways, `${width}`).toHaveLength(3);
        for (const node of gateways) {
          expect(node.x, `${width}`).toBeGreaterThanOrEqual(0);
          expect(node.x + node.w, `${width}`).toBeLessThanOrEqual(scene.width);
          for (const other of hits) {
            if (other === node) continue;
            const apart =
              node.x + node.w <= other.x ||
              other.x + other.w <= node.x ||
              node.y + node.h <= other.y ||
              other.y + other.h <= node.y;
            expect(apart, `${width} ${node.hit.name} over ${other.hit.name}`).toBe(true);
          }
        }
        // The Internet tile is no link, so it is found by its label: it and
        // its tile sit under the last gateway row, not among the gateways.
        const internet = ofKind(scene, 'text').find((node) => node.text === 'Internet');
        expect(internet, `${width}`).toBeDefined();
        const lastRow = Math.max(...gateways.map((node) => node.y + node.h));
        // Its tile is the smallest box drawn round the label.
        const [tile] = ofKind(scene, 'rect')
          .filter(
            (node) =>
              node.x <= internet!.x &&
              internet!.x <= node.x + node.w &&
              node.y <= internet!.y &&
              internet!.y <= node.y + node.h,
          )
          .sort((a, b) => a.w * a.h - b.w * b.h);
        expect(tile, `${width}`).toBeDefined();
        expect(tile.h, `${width}`).toBeLessThan(120);
        expect(tile.y, `${width}`).toBeGreaterThanOrEqual(lastRow);
        return new Set(gateways.map((node) => node.y)).size;
      };
      // 320 holds two wrapped-name tiles a row, 374 holds three.
      expect(rowsAt(320)).toBe(2);
      expect(rowsAt(374)).toBe(1);
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

    describe('names too long for one line of the usual tile', () => {
      const LONG = ['paperless-ngx-archive', 'home-assistant-core', 'uptime-kuma-status-page'];
      const longNamed = buildServiceDiagram(
        buildInfrastructureMapModel(estate(LONG.map((name) => guest(name)))),
      );
      // Sixty of them in one box: a map with no room to give.
      const crowded = buildServiceDiagram(
        buildInfrastructureMapModel(
          estate(Array.from({ length: 60 }, (_, index) => guest(`${LONG[index % 3]}-${index}`))),
        ),
        { maxTilesPerGroup: 80 },
      );
      const cut = (scene: MapScene) => texts(scene).filter((text) => text.endsWith('…'));
      const tileOf = (scene: MapScene) => {
        const [first] = ofKind(scene, 'hit');
        return { w: first.w, h: first.h };
      };
      const USUAL = { w: 92, h: 72 };
      // The fit as it was before names were given room: one line, the usual
      // tile, the widest drawing of the widths tried.
      const oneLineFit = (
        of: ReturnType<typeof buildServiceDiagram>,
        space: { width: number; height: number },
      ) => {
        let best: { scene: MapScene; scale: number } | null = null;
        for (let step = 0; step <= 24; step += 1) {
          const scene = layoutServiceDiagram(of, {
            width: space.width * (1 + (step / 24) * 2),
            density: 'names',
          });
          const scale = Math.min(1, space.width / scene.width, space.height / scene.height);
          if (!best || scale > best.scale + 0.001) best = { scene, scale };
          if (scale >= 1) break;
        }
        return { scene: best!.scene, scale: Math.max(0.3, best!.scale) };
      };

      it('are written whole where the screen has the room, and the map is drawn no smaller', () => {
        const space = { width: 1800, height: 1400 };
        const before = oneLineFit(longNamed, space);
        expect(tileOf(before.scene)).toEqual(USUAL);
        expect(cut(before.scene).length).toBeGreaterThan(0);

        const after = fitServiceDiagram(longNamed, space, 'names');
        expect(after.scale).toBe(before.scale);
        expect(cut(after.scene)).toEqual([]);
        const tile = tileOf(after.scene);
        expect(tile.w > USUAL.w || tile.h > USUAL.h).toBe(true);
        expect(texts(after.scene).join(' ')).toContain('archive');
      });

      it('take the width a short screen leaves, at no cost, where that is enough', () => {
        // Two long names that a wider tile holds on one line.
        const wide = buildServiceDiagram(
          buildInfrastructureMapModel(
            estate([guest('home-assistant-core'), guest('paperless-ngx-web')]),
          ),
        );
        const full = layoutServiceDiagram(wide, { width: 2400, density: 'names' });
        const space = { width: 2400, height: full.height };
        const before = oneLineFit(wide, space);
        expect(before.scale).toBe(1);
        expect(cut(before.scene).length).toBeGreaterThan(0);

        const after = fitServiceDiagram(wide, space, 'names');
        expect(after.scale).toBe(1);
        expect(tileOf(after.scene).h).toBe(USUAL.h);
        expect(tileOf(after.scene).w).toBeGreaterThan(USUAL.w);
        expect(cut(after.scene)).toEqual([]);
      });

      it('cost the map a tenth of its size at most, and never any of it off the screen', () => {
        for (const of of [longNamed, crowded]) {
          for (const space of [
            { width: 1800, height: 1400 },
            { width: 1776, height: 800 },
            { width: 1322, height: 620 },
            { width: 1000, height: 380 },
            { width: 700, height: 500 },
          ]) {
            const where = JSON.stringify(space);
            const before = oneLineFit(of, space);
            const after = fitServiceDiagram(of, space, 'names');
            expect(after.scale, where).toBeGreaterThanOrEqual(before.scale * 0.9 - 0.001);
            expect(cut(after.scene).length, where).toBeLessThanOrEqual(cut(before.scene).length);
            if (after.scale > 0.3) {
              expect(after.scene.width * after.scale, where).toBeLessThanOrEqual(space.width + 0.5);
              expect(after.scene.height * after.scale, where).toBeLessThanOrEqual(
                space.height + 0.5,
              );
            }
          }
        }
      });

      // The names on a drawing that are cut short: the 12px medium line.
      const cutNames = (scene: MapScene) =>
        ofKind(scene, 'text')
          .filter((node) => node.size === 12 && node.weight === 500 && node.text.includes('…'))
          .map((node) => node.text);

      it('give up some of the size for whole names where only a second line will do', () => {
        // Too long for the widest tile on one line, and a space the one-line
        // layout fills exactly: a second line makes the map taller.
        const twoLiners = buildServiceDiagram(
          buildInfrastructureMapModel([
            resource({ id: 'pve', type: 'agent', name: 'delly', platformType: 'proxmox-pve' }),
            guest('home-assistant-core-eu'),
            guest('paperless-ngx-archive-eu'),
            guest('jellyfin'),
          ]),
        );
        const full = layoutServiceDiagram(twoLiners, { width: 2400, density: 'names' });
        const space = { width: 2400, height: full.height };
        const before = oneLineFit(twoLiners, space);
        expect(before.scale).toBe(1);
        expect(cutNames(before.scene).length).toBe(2);

        const after = fitServiceDiagram(twoLiners, space, 'names');
        expect(cutNames(after.scene)).toEqual([]);
        expect(after.scale).toBeLessThan(1);
        expect(after.scale).toBeGreaterThanOrEqual(0.9);
        expect(after.scene.height * after.scale).toBeLessThanOrEqual(space.height + 0.5);
      });

      it('never take a readable map below the size at which the page stops choosing names', () => {
        for (const of of [longNamed, crowded]) {
          for (const width of [1200, 1776]) {
            for (let height = 300; height <= 700; height += 50) {
              const space = { width, height };
              const before = oneLineFit(of, space);
              const after = fitServiceDiagram(of, space, 'names');
              const where = JSON.stringify(space);
              if (before.scale >= 0.62) {
                expect(after.scale, where).toBeGreaterThanOrEqual(0.62);
                expect(after.scale, where).toBeGreaterThanOrEqual(before.scale * 0.9);
              } else {
                // Already too small for names to be the point: nothing is given up.
                expect(after.scale, where).toBeGreaterThanOrEqual(before.scale - 0.001);
              }
            }
          }
        }
      });

      it('count a name cut short that kept its number', () => {
        const numbered = buildServiceDiagram(
          buildInfrastructureMapModel([
            resource({ id: 'pve', type: 'agent', name: 'delly', platformType: 'proxmox-pve' }),
            guest('paperless-ngx-archive 123'),
          ]),
        );
        const space = { width: 1250, height: 800 };
        expect(cutNames(oneLineFit(numbered, space).scene)).toHaveLength(1);
        const after = fitServiceDiagram(numbered, space, 'names');
        expect(cutNames(after.scene)).toEqual([]);
        expect(texts(after.scene).join(' ')).toContain('123');
      });

      it('put whole names before whole captions', () => {
        // The widest one-line tile keeps the long caption whole and cuts the
        // long name; two lines keep the name.
        const captioned = buildServiceDiagram(
          buildInfrastructureMapModel([
            resource({
              id: 'pve',
              type: 'agent',
              name: 'stats-primary.example.com',
              platformType: 'proxmox-pve',
            }),
            guest('production-database-service'),
            guest('jellyfin'),
            guest('sonarr'),
          ]),
        );
        const after = fitServiceDiagram(captioned, { width: 1250, height: 800 }, 'names');
        expect(cutNames(after.scene)).toEqual([]);
      });

      it('leave a map alone whose only cut labels are not names', () => {
        const captionOnly = buildServiceDiagram(
          buildInfrastructureMapModel([
            resource({
              id: 'pve',
              type: 'agent',
              name: 'status.production.authentication-and-database-workers.example.com',
              platformType: 'proxmox-pve',
            }),
            ...['jellyfin', 'sonarr', 'radarr'].map((name) => guest(name)),
          ]),
        );
        const space = { width: 1200, height: 800 };
        const before = oneLineFit(captionOnly, space);
        if (cutNames(before.scene).length === 0) {
          const after = fitServiceDiagram(captionOnly, space, 'names');
          expect(tileOf(after.scene)).toEqual(USUAL);
        }
      });

      it('stay as they were on a map already at the smallest it is drawn', () => {
        const { scene, scale } = fitServiceDiagram(longNamed, { width: 320, height: 60 }, 'names');
        expect(scale).toBe(0.3);
        expect(tileOf(scene)).toEqual(USUAL);
      });

      it('leave a map whose labels all fit as it was, however much room there is', () => {
        const space = { width: 1800, height: 1400 };
        const shortNamed = buildServiceDiagram(
          buildInfrastructureMapModel([
            resource({ id: 'pve', type: 'agent', name: 'delly', platformType: 'proxmox-pve' }),
            ...['jellyfin', 'sonarr', 'radarr', 'grafana', 'gitea'].map((name) => guest(name)),
          ]),
        );
        const before = oneLineFit(shortNamed, space);
        expect(cut(before.scene)).toEqual([]);
        const after = fitServiceDiagram(shortNamed, space, 'names');
        expect(tileOf(after.scene)).toEqual(USUAL);
        expect(after.scene.height).toBe(before.scene.height);
      });

      it('change nothing where the map draws icons alone', () => {
        const { scene } = fitServiceDiagram(longNamed, { width: 1800, height: 1400 }, 'marks');
        expect(tileOf(scene)).toEqual({ w: 40, h: 40 });
      });
    });
  });

  describe('flowServiceDiagram, the map on a phone', () => {
    const diagram = buildServiceDiagram(buildInfrastructureMapModel(estate()));
    // The width a 390px phone leaves the drawing.
    const PHONE = 374;
    const tileWidths = (scene: ReturnType<typeof layoutServiceDiagram>) =>
      ofKind(scene, 'hit').map((node) => node.w);

    it("draws at full size at the phone's own width, however long that makes it", () => {
      for (const density of ['marks', 'names'] as const) {
        const { scene, scale } = flowServiceDiagram(diagram, PHONE, density);
        expect(scale, density).toBe(1);
        expect(scene.width, density).toBe(PHONE);
        // Fitted to one screen the same map is shrunk to a third.
        const fitted = fitServiceDiagram(diagram, { width: PHONE, height: 414 }, density);
        expect(fitted.scale, density).toBeLessThan(0.7);
        expect(scene.height, density).toBeGreaterThan(414);
      }
    });

    it('keeps every tile big enough to tap and inside the width', () => {
      const { scene } = flowServiceDiagram(diagram, PHONE, 'marks');
      const hits = ofKind(scene, 'hit');
      expect(hits.length).toBeGreaterThan(5);
      expect(Math.min(...tileWidths(scene))).toBeGreaterThanOrEqual(40);
      for (const node of hits) {
        expect(node.x).toBeGreaterThanOrEqual(0);
        expect(node.x + node.w).toBeLessThanOrEqual(PHONE);
      }
    });

    it('writes names out on two lines, since a phone cannot hover a cut one', () => {
      const flowed = flowServiceDiagram(diagram, PHONE, 'names').scene;
      const plain = layoutServiceDiagram(diagram, { width: PHONE, density: 'names' });
      const wrapped = layoutServiceDiagram(diagram, {
        width: PHONE,
        density: 'names',
        wrapNames: true,
      });
      expect(Math.max(...tileWidths(flowed))).toBe(Math.max(...tileWidths(wrapped)));
      expect(Math.max(...tileWidths(flowed))).toBeGreaterThan(Math.max(...tileWidths(plain)));
    });

    it('cuts a network label that is wider than the phone, and keeps its pill inside', () => {
      const long = `vmbr${'1'.repeat(60)}`;
      const onLong = (name: string) =>
        guest(name, {
          proxmox: {
            clusterName: 'homelab',
            networkInterfaces: [{ name: 'eth0', addresses: [], bridge: long, vlan: 30 }],
          },
        });
      const wide = buildServiceDiagram(
        buildInfrastructureMapModel(estate([onLong('frigate'), onLong('esphome')])),
      );
      for (const width of [320, PHONE]) {
        for (const density of ['marks', 'names'] as const) {
          const { scene, scale } = flowServiceDiagram(wide, width, density);
          const where = `${width} ${density}`;
          expect(scale, where).toBe(1);
          expect(
            texts(scene).some((text) => text.startsWith('vmbr1111') && text.endsWith('…')),
            where,
          ).toBe(true);
          for (const node of ofKind(scene, 'rect')) {
            expect(node.x, where).toBeGreaterThanOrEqual(0);
            expect(node.x + node.w, where).toBeLessThanOrEqual(scene.width);
          }
          // A pill is the one box 24 tall with round ends, and none is
          // wider than the row the pills share, the drawing less its gutters.
          const pills = ofKind(scene, 'rect').filter((node) => node.h === 24 && node.r === 12);
          expect(pills.length, where).toBeGreaterThan(0);
          for (const pill of pills) {
            expect(pill.w, where).toBeLessThanOrEqual(scene.width - 32);
          }
        }
      }
    });

    it('says a fold of a thousand or more as thousands, not as its first digits', () => {
      const many = Array.from({ length: 1300 }, (_, index) => guest(`svc-${index}`));
      const { scene } = flowServiceDiagram(
        buildServiceDiagram(buildInfrastructureMapModel(estate(many))),
        PHONE,
        'marks',
      );
      const folds = ofKind(scene, 'app')
        .map((node) => node.letters)
        .filter((letters) => letters.startsWith('+'));
      expect(folds).toContain('+1k');
      expect(folds.some((letters) => /^\+\d{3}$/.test(letters))).toBe(false);
    });

    it('reads no count into a name that only starts with a plus sign', () => {
      const letters = (name: string) =>
        ofKind(
          flowServiceDiagram(
            buildServiceDiagram(buildInfrastructureMapModel(estate([guest(name)]))),
            PHONE,
            'marks',
          ).scene,
          'app',
        ).map((node) => node.letters);
      expect(letters('+1e3')).toContain('+1e3');
      expect(letters('+Infinity')).toContain('+Inf');
    });

    it('scales down only when the window is narrower than the narrowest layout', () => {
      const { scene, scale } = flowServiceDiagram(diagram, 240, 'marks');
      expect(scene.width).toBeGreaterThan(240);
      expect(scale).toBeLessThan(1);
      expect(scene.width * scale).toBeLessThanOrEqual(240.5);
    });
  });

  describe('live network traffic visualization', () => {
    const liveEstate = (): Resource[] => [
      resource({
        id: 'pve',
        type: 'agent',
        name: 'delly',
        platformType: 'proxmox-pve',
        network: { rxBytes: 12_000_000, txBytes: 4_000_000 },
        agent: {
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['192.168.10.11/24'], speedMbps: 10000, linkUp: true },
            { name: 'enp2s0', addresses: [], speedMbps: 1000, linkUp: false },
          ],
        },
      }),
      guest('cloudflared', {
        identity: { ips: ['192.168.10.20'] },
        network: { rxBytes: 30_000_000, txBytes: 5_000_000 },
      }),
      guest('jellyfin', {
        identity: { ips: ['192.168.10.50'] },
        network: { rxBytes: 5_000_000, txBytes: 2_000_000 },
        proxmox: {
          networkInterfaces: [
            { name: 'net0', addresses: ['192.168.10.50'], bridge: 'vmbr0', firewall: true },
          ],
        },
      }),
      // Idle guest on a second quiet subnet.
      guest('grafana', {
        identity: { ips: ['10.0.40.10'] },
        network: { rxBytes: 512, txBytes: 256 },
      }),
      guest('prometheus', {
        identity: { ips: ['10.0.40.11'] },
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
        id: 'zfs',
        type: 'storage',
        name: 'local-zfs',
        platformType: 'proxmox-pve',
        parentId: 'pve',
      }),
    ];

    it('highlights active tiles, active network pills, and active connectors while keeping idle branches and Internet static', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(liveEstate()));
      const liveScene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });

      // Hits carry truthful telemetry fields.
      const jellyfinHit = ofKind(liveScene, 'hit').find((node) => node.hit.id === 'ct-jellyfin');
      expect(jellyfinHit?.hit).toMatchObject({
        netRxBytes: 5_000_000,
        netTxBytes: 2_000_000,
        firewallEnabled: true,
      });
      const hostHit = ofKind(liveScene, 'hit').find((node) => node.hit.id === 'pve');
      expect(hostHit?.hit).toMatchObject({
        netRxBytes: 12_000_000,
        netTxBytes: 4_000_000,
        linkSpeedMbps: 10000,
        linkDownCount: 1,
      });
      const routerHit = ofKind(liveScene, 'hit').find((node) => node.hit.id === 'check-router');
      expect(routerHit?.hit.latencyMs).toBe(2);

      // Active tiles show compact speed in accent on the caption line; idle tiles keep host caption.
      const drawnTexts = ofKind(liveScene, 'text');
      expect(drawnTexts.some((node) => node.fill === 'accent' && node.text.endsWith('/s'))).toBe(
        true,
      );
      expect(drawnTexts.some((node) => node.fill === 'muted' && node.text === 'delly')).toBe(true);

      // Active network pill shows speed and activity highlight; quiet subnet pill does not.
      expect(
        drawnTexts.some((node) => node.text.startsWith('vmbr0') && node.text.endsWith('/s')),
      ).toBe(true);
      expect(drawnTexts.some((node) => node.text === '10.0.40.x · 2 addresses')).toBe(true);
      const pillHighlights = ofKind(liveScene, 'rect').filter(
        (node) => node.activity !== undefined && node.h === 24 && node.r === 12,
      );
      expect(pillHighlights).toHaveLength(1);
      expect(pillHighlights[0].activity).toBe('high');

      // Active tiles get activity highlight rects in both names and marks density.
      const activeTileRects = ofKind(liveScene, 'rect').filter(
        (node) => node.activity !== undefined && node.h > 24,
      );
      expect(activeTileRects.map((node) => node.activity).sort()).toEqual([
        'high',
        'medium',
        'medium',
      ]);

      const compactLive = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'marks',
        liveTraffic: true,
      });
      expect(
        ofKind(compactLive, 'rect').filter((node) => node.activity !== undefined && node.h === 40),
      ).toHaveLength(3);

      // Connectors: active branches animate, idle branches and dashed Internet line never animate.
      const lines = ofKind(liveScene, 'line');
      const dashedLines = lines.filter((node) => node.dashed);
      expect(dashedLines).toHaveLength(1);
      expect(dashedLines[0].activity).toBeUndefined();
      expect(lines.some((node) => node.activity !== undefined)).toBe(true);
      expect(lines.some((node) => !node.dashed && node.activity === undefined)).toBe(true);

      // fitServiceDiagram and flowServiceDiagram forward liveTraffic cleanly.
      const fitted = fitServiceDiagram(diagram, { width: 1400, height: 900 }, 'names', {
        liveTraffic: true,
      });
      expect(ofKind(fitted.scene, 'line').some((node) => node.activity !== undefined)).toBe(true);

      const flowed = flowServiceDiagram(diagram, 374, 'names', { liveTraffic: true });
      expect(ofKind(flowed.scene, 'line').some((node) => node.activity !== undefined)).toBe(true);

      // Static mode (liveTraffic: false) omits all activity rects and active connector flags.
      const staticScene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: false,
      });
      expect(ofKind(staticScene, 'rect').filter((node) => node.activity !== undefined)).toEqual([]);
      expect(ofKind(staticScene, 'line').filter((node) => node.activity !== undefined)).toEqual([]);
    });

    it('emits flow overlays and active tile classes on the live page, and never in exported SVG', () => {
      const model = buildInfrastructureMapModel(liveEstate());
      const diagram = buildServiceDiagram(model);
      const liveScene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });

      const liveBody = renderMapSceneBody(liveScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(liveBody).toContain('class="pulse-map-flow pulse-map-flow-');
      expect(liveBody).toContain('stroke-dasharray="5 10"');
      expect(liveBody).toContain('stroke-linecap="round"');
      expect(liveBody).toContain('class="pulse-map-active pulse-map-active-');

      // Direct renderMapSceneToSvg on a live scene stays completely static with zero animation classes.
      const staticSvgFromLive = renderMapSceneToSvg(liveScene, MAP_PALETTES.dark);
      expect(staticSvgFromLive).not.toContain('pulse-map-flow');
      expect(staticSvgFromLive).not.toContain('pulse-map-active');
      expect(staticSvgFromLive).not.toContain('class=');

      // Exported poster lays out with liveTraffic: false and emits no animation classes.
      const exported = buildMapExportSvg(model, {
        title: 'Infrastructure map',
        density: 'names',
        palette: MAP_PALETTES.light,
        now: new Date('2026-11-26T12:00:00Z'),
      });
      expect(exported.svg).not.toContain('pulse-map-flow');
      expect(exported.svg).not.toContain('pulse-map-active');
      expect(exported.svg).not.toContain('class=');
    });

    it('keeps stated connection captions intact in live traffic mode while retaining the activity halo', () => {
      const resources: Resource[] = [
        resource({ id: 'docker-1', type: 'docker-host', name: 'docker-1', status: 'online' }),
        resource({
          id: 'vpn',
          type: 'app-container',
          name: 'gluetun',
          parentId: 'docker-1',
          status: 'running',
          docker: { containerId: 'aaaaaaaaaaaaaaaa', image: 'qmcgaw/gluetun:latest' },
          network: { rxBytes: 2_000_000, txBytes: 0 },
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
          network: { rxBytes: 8_000_000, txBytes: 0 },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const scene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });
      const texts = scene.nodes
        .filter((node): node is Extract<typeof node, { kind: 'text' }> => node.kind === 'text')
        .map((node) => node.text);
      expect(texts).toContain('via gluetun');
      expect(texts).toContain('1.9M/s');
      expect(texts).not.toContain('7.6M/s');
      const activeRects = scene.nodes.filter(
        (node) => node.kind === 'rect' && node.activity !== undefined,
      );
      expect(activeRects.length).toBeGreaterThanOrEqual(2);
    });

    it('propagates dockerNetworks, probeSummary, and certWarning onto hit targets', () => {
      const resources: Resource[] = [
        resource({ id: 'docker-1', type: 'docker-host', name: 'docker-1', status: 'online' }),
        resource({
          id: 'c-app',
          type: 'app-container',
          name: 'jellyfin',
          parentId: 'docker-1',
          status: 'running',
          docker: { networks: [{ name: 'bridge' }, { name: 'proxy_net' }] },
        }),
        resource({
          id: 'chk-app',
          type: 'network-endpoint',
          name: 'Jellyfin HTTPS',
          availability: {
            linkedResourceId: 'c-app',
            protocol: 'https',
            applicationStatusCode: 200,
            available: true,
            latencyMillis: 14,
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
      ];

      const scene = sceneOf(resources, 1400);
      const appHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'c-app')?.hit;
      const chkHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'chk-app')?.hit;
      expect(appHit?.dockerNetworks).toEqual(['proxy_net']);
      expect(appHit?.linked).toEqual(['docker-1', 'chk-app']);
      expect(chkHit).toMatchObject({
        probeSummary: 'HTTPS 200',
        certWarning: 'Cert expired',
        linked: ['c-app'],
      });
    });

    it('renders curved hover arcs with optional liveTraffic flow overlay and caps arcs at MAP_MAX_LINK_ARCS', () => {
      const manyContainers = Array.from({ length: 20 }, (_, index) =>
        resource({
          id: `c-svc-${index}`,
          type: 'app-container',
          name: `svc-${index}`,
          platformType: 'docker',
          parentId: 'docker-1',
          status: 'running',
        }),
      );
      const resources: Resource[] = [
        resource({
          id: 'docker-1',
          type: 'docker-host',
          name: 'Tower',
          platformType: 'docker',
          agent: { defaultGateways: [{ address: '192.168.0.1', interface: 'eth0' }] },
        }),
        ...manyContainers,
      ];
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources), {
        maxTilesPerGroup: 30,
      });
      const scene = layoutServiceDiagram(diagram, { width: 1400, density: 'names' });
      const hostHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'docker-1')!.hit;
      const gatewayHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'gateway:0')!.hit;
      const palette = MAP_PAGE_PALETTES.dark;

      // 20 containers + 1 unmonitored gateway = 21 linked tiles.
      expect(hostHit.linked).toHaveLength(21);
      expect(gatewayHit.linked).toEqual(['docker-1']);

      // Static mode: all 21 linked tiles get tinted rings, curved halos and arcs capped at MAP_MAX_LINK_ARCS (16), no flow overlay.
      const staticOverlay = renderMapLinkRings(scene, hostHit, palette);
      expect(staticOverlay.match(/<rect /g)).toHaveLength(21);
      expect(staticOverlay).toContain(`fill="${palette.accent}" fill-opacity="0.06"`);
      expect(staticOverlay.match(/class="pulse-map-link-halo"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(staticOverlay).toContain(
        `stroke="${palette.surface}" stroke-width="4" stroke-opacity="0.72" class="pulse-map-link-halo"`,
      );
      expect(staticOverlay.match(/class="pulse-map-link-arc"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(staticOverlay.indexOf('pulse-map-link-halo')).toBeLessThan(
        staticOverlay.indexOf('pulse-map-link-arc'),
      );
      expect(staticOverlay).toMatch(/d="M[\d.]+ [\d.]+ Q[\d.]+ [\d.]+ [\d.]+ [\d.]+"/);
      expect(staticOverlay.match(/<circle /g)).toHaveLength(MAP_MAX_LINK_ARCS + 1);
      expect(staticOverlay).not.toContain('pulse-map-link-flow');

      // Live traffic mode: emits matching animated flow paths alongside base arcs and halos.
      const liveOverlay = renderMapLinkRings(scene, hostHit, palette, { liveTraffic: true });
      expect(liveOverlay.match(/class="pulse-map-link-halo"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(liveOverlay.match(/class="pulse-map-link-arc"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(liveOverlay.match(/class="pulse-map-link-flow"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(liveOverlay.indexOf('pulse-map-link-halo')).toBeLessThan(
        liveOverlay.indexOf('pulse-map-link-arc'),
      );
      expect(liveOverlay).toContain('stroke-dasharray="4 10"');

      // Unmonitored gateway hover draws 1 ring, 1 halo, and 1 curved arc to its host.
      const gatewayOverlay = renderMapLinkRings(scene, gatewayHit, palette, { liveTraffic: true });
      expect(gatewayOverlay.match(/<rect /g)).toHaveLength(1);
      expect(gatewayOverlay.match(/class="pulse-map-link-halo"/g)).toHaveLength(1);
      expect(gatewayOverlay.match(/class="pulse-map-link-arc"/g)).toHaveLength(1);
      expect(gatewayOverlay.match(/class="pulse-map-link-flow"/g)).toHaveLength(1);

      // Exported SVG never contains hover rings, halos, or arcs.
      const exported = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
      expect(exported).not.toContain('pulse-map-link-halo');
      expect(exported).not.toContain('pulse-map-link-arc');
      expect(exported).not.toContain('pulse-map-link-flow');
    });

    it('propagates segmentIds and linkUtilizationPercent onto MapHit and renders interactive network rings', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-net',
          type: 'agent',
          name: 'pve-net',
          platformType: 'proxmox-pve',
          network: { rxBytes: 112_500_000, txBytes: 5_000_000 },
          agent: {
            networkInterfaces: [
              { name: 'eno1', addresses: ['10.30.0.10/24'], speedMbps: 1000, linkUp: true },
            ],
          },
        }),
        guest('jellyfin', {
          parentId: 'pve-net',
          identity: { ips: ['10.30.0.50', '10.40.0.50'] },
        }),
        guest('grafana', {
          parentId: 'pve-net',
          identity: { ips: ['10.40.0.60'] },
        }),
      ];
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const scene = layoutServiceDiagram(diagram, { width: 1400, density: 'names' });
      const net30 = diagram.networks.find((entry) => entry.label === '10.30.0.0/24')!;
      const net40 = diagram.networks.find((entry) => entry.label === '10.40.0.x')!;

      const hostHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pve-net')!.hit;
      const jellyfinHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'ct-jellyfin')!.hit;
      const grafanaHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'ct-grafana')!.hit;

      expect(hostHit.linkUtilizationPercent).toBe(90);
      expect(hostHit.segmentIds).toEqual([net30.id]);
      expect(jellyfinHit.segmentIds?.sort()).toEqual([net30.id, net40.id].sort());
      expect(grafanaHit.segmentIds).toEqual([net40.id]);

      // Network hover rings highlight every member tile of that network (including multi-homed tiles).
      const rings30 = renderMapNetworkRings(scene, net30.id, MAP_PAGE_PALETTES.dark.net0);
      expect(rings30.match(/class="pulse-map-network-ring"/g)).toHaveLength(2);
      expect(rings30).toContain(
        `fill="${MAP_PAGE_PALETTES.dark.net0}" fill-opacity="0.08" stroke="${MAP_PAGE_PALETTES.dark.net0}" stroke-width="2" class="pulse-map-network-ring"`,
      );

      const rings40 = renderMapNetworkRings(scene, net40.id, MAP_PAGE_PALETTES.dark.net1);
      expect(rings40.match(/class="pulse-map-network-ring"/g)).toHaveLength(2);

      // Unknown or empty network id emits no rings.
      expect(renderMapNetworkRings(scene, 'net:missing', '#60a5fa')).toBe('');
      expect(renderMapNetworkRings(scene, '', '#60a5fa')).toBe('');

      // Static SVG exports never contain interactive network rings.
      expect(renderMapSceneToSvg(scene, MAP_PALETTES.dark)).not.toContain('pulse-map-network-ring');
    });
  });
});
