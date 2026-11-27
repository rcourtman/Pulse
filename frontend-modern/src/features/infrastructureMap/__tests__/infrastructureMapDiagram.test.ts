import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  fitServiceDiagram,
  flowServiceDiagram,
  layoutServiceDiagram,
} from '../infrastructureMapDiagram';
import { buildMapExportSvg } from '../infrastructureMapExport';
import {
  elevatedLoadTone,
  fitCardTitle,
  formatLiveActivitySubtitle,
  hostCountLabel,
  isSpikingServiceTile,
  layoutHostDiagram,
  proportionalStorageTrackWidth,
  strokeWidthForLinkSpeed,
  tileActivityTier,
} from '../infrastructureMapHostDiagram';
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
  applyMapSpotlight,
  flowDurationForBytes,
  MAP_FLOW_GLOW_MIN_BYTES,
  MAP_MAX_LINK_ARCS,
  MAP_PAGE_PALETTES,
  MAP_PALETTES,
  MAP_SPOTLIGHT_CLASS,
  MAP_SPOTLIGHT_LIT_CLASS,
  MAP_TILE_NODE_ATTRIBUTE,
  renderMapLinkRings,
  renderMapNetworkRings,
  renderMapSceneBody,
  renderMapSceneToSvg,
  resolveFlowVisualProfile,
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
      expect(rings.match(/stroke-width="1\.5"\/>/g)).toHaveLength(2);
      expect(rings).toContain(`x="${to.x}" y="${to.y}"`);
      expect(rings).toContain(`x="${machine.x}" y="${machine.y}"`);
      expect(rings).toContain(`stroke="${palette.accent}"`);
      expect(rings).toContain('fill="none"');
      expect(renderMapLinkRings(scene, to.hit, palette)).toContain(`x="${from.x}" y="${from.y}"`);
      // A container with no connection of its own rings only its machine.
      const alone = hitOf(scene, 'c-sonarr');
      if (!alone) throw new Error('the tile should be drawn');
      const lone = renderMapLinkRings(scene, alone.hit, palette);
      expect(lone.match(/stroke-width="1\.5"\/>/g)).toHaveLength(1);
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
      const roleScene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });
      const hostScene = layoutHostDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });

      // Logical By-role connectors glow/thicken bidirectionally without inventing one-way arrows.
      const roleBody = renderMapSceneBody(roleScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(roleBody).toContain('class="pulse-map-active pulse-map-active-');
      expect(roleBody).not.toContain('class="pulse-map-flow ');
      expect(roleBody).not.toContain('pulse-map-packet');

      // Measured host uplinks in By-host emit directional flow overlays and active tile classes.
      const liveBody = renderMapSceneBody(hostScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(liveBody).toContain('class="pulse-map-flow pulse-map-flow-');
      expect(liveBody).toMatch(/stroke-dasharray="\d+ \d+"/);
      expect(liveBody).toContain('stroke-linecap="round"');
      expect(liveBody).toContain('class="pulse-map-active pulse-map-active-');

      // Direct renderMapSceneToSvg on a live scene stays completely static with zero animation classes.
      const staticSvgFromLive = renderMapSceneToSvg(hostScene, MAP_PALETTES.dark);
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
      expect(texts).toContain('↓ 1.9M/s');
      expect(texts).not.toContain('↓ 7.6M/s');
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

      // Static relationship arcs: all 21 linked tiles get tinted rings, curved halos and static arcs capped at MAP_MAX_LINK_ARCS (16), never animated.
      const overlay = renderMapLinkRings(scene, hostHit, palette);
      expect(overlay.match(/<rect /g)).toHaveLength(21);
      expect(overlay).toContain(`fill="${palette.accent}" fill-opacity="0.06"`);
      expect(overlay.match(/class="pulse-map-link-halo"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(overlay).toContain(
        `stroke="${palette.surface}" stroke-width="4" stroke-opacity="0.72" class="pulse-map-link-halo"`,
      );
      expect(overlay.match(/class="pulse-map-link-arc"/g)).toHaveLength(MAP_MAX_LINK_ARCS);
      expect(overlay.indexOf('pulse-map-link-halo')).toBeLessThan(
        overlay.indexOf('pulse-map-link-arc'),
      );
      expect(overlay).toMatch(/d="M[\d.]+ [\d.]+ Q[\d.]+ [\d.]+ [\d.]+ [\d.]+"/);
      expect(overlay.match(/<circle /g)).toHaveLength(MAP_MAX_LINK_ARCS + 1);
      expect(overlay).not.toContain('pulse-map-link-flow');

      // Unmonitored gateway hover draws 1 ring, 1 halo, 1 static curved arc, and 1 factual relationship pill.
      const gatewayOverlay = renderMapLinkRings(scene, gatewayHit, palette);
      expect(gatewayOverlay.match(/<rect /g)).toHaveLength(2);
      expect(gatewayOverlay.match(/class="pulse-map-link-label"/g)).toHaveLength(1);
      expect(gatewayOverlay).toContain('>gateway</text>');
      expect(gatewayOverlay.match(/class="pulse-map-link-halo"/g)).toHaveLength(1);
      expect(gatewayOverlay.match(/class="pulse-map-link-arc"/g)).toHaveLength(1);
      expect(gatewayOverlay).not.toContain('pulse-map-link-flow');

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

  describe('layoutHostDiagram and host-framed architectural mode', () => {
    const richEstate = (): Resource[] => [
      resource({
        id: 'pve-1',
        type: 'agent',
        name: 'pve-core',
        platformType: 'proxmox-pve',
        clusterId: 'homelab',
        status: 'online',
        memory: { current: 50, total: 64 * 1024 ** 3, used: 32 * 1024 ** 3 },
        network: { rxBytes: 40_000_000, txBytes: 10_000_000 },
        proxmox: {
          clusterName: 'homelab',
          cpus: 16,
          pveVersion: '8.3.2/abc',
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
        agent: {
          defaultGateways: [{ address: '10.30.0.1', interface: 'vmbr0' }],
          networkInterfaces: [
            { name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'ct-102',
        type: 'system-container',
        name: 'jellyfin',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'running',
        identity: { ips: ['10.30.0.102'] },
        network: { rxBytes: 12_000_000, txBytes: 3_000_000 },
        proxmox: { vmid: 102 },
      }),
      resource({
        id: 'vm-105',
        type: 'vm',
        name: 'homeassistant',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'running',
        identity: { ips: ['10.40.0.105'] },
        proxmox: { vmid: 105 },
      }),
      resource({
        id: 'ct-tunnel',
        type: 'system-container',
        name: 'cloudflared',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'running',
        identity: { ips: ['10.30.0.20'] },
        proxmox: { vmid: 100 },
      }),
      resource({
        id: 'pool-nvme',
        type: 'storage',
        name: 'nvme-pool',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'online',
        disk: { current: 82, total: 4 * 1024 ** 4, used: Math.round(3.28 * 1024 ** 4) },
        storage: { type: 'zfspool', vdevLayout: 'mirror' },
      }),
      resource({
        id: 'pool-critical',
        type: 'storage',
        name: 'archive-pool',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'online',
        disk: { current: 94, total: 8 * 1024 ** 4, used: Math.round(7.52 * 1024 ** 4) },
        storage: { type: 'zfspool', vdevLayout: 'raidz1' },
      }),
      resource({
        id: 'docker-1',
        type: 'docker-host',
        name: 'dock-app',
        platformType: 'docker',
        status: 'online',
        docker: {
          cpus: 8,
          dockerVersion: '27.3.1',
          memory: { total: 32 * 1024 ** 3, used: 12 * 1024 ** 3 },
          networkInterfaces: [
            { name: 'eno1', addresses: ['10.40.0.30/24'], speedMbps: 2500, linkUp: true },
          ],
        },
      }),
      resource({
        id: 'c-sonarr',
        type: 'app-container',
        name: 'sonarr',
        platformType: 'docker',
        parentId: 'docker-1',
        status: 'running',
        identity: { ips: ['10.30.0.31'] },
        docker: {
          composeProject: 'media-stack',
          image: 'lscr.io/linuxserver/sonarr:latest',
          ports: [{ publicPort: 8989, privatePort: 8989, protocol: 'tcp' }],
        },
      }),
      resource({
        id: 'c-radarr',
        type: 'app-container',
        name: 'radarr',
        platformType: 'docker',
        parentId: 'docker-1',
        status: 'running',
        identity: { ips: ['10.30.0.32'] },
        docker: {
          composeProject: 'media-stack',
          image: 'lscr.io/linuxserver/radarr:latest',
          ports: [{ publicPort: 7878, privatePort: 7878, protocol: 'tcp' }],
        },
      }),
      resource({
        id: 'c-grafana',
        type: 'app-container',
        name: 'grafana',
        platformType: 'docker',
        parentId: 'docker-1',
        status: 'running',
        identity: { ips: ['10.40.0.33'] },
        docker: {
          image: 'grafana/grafana:latest',
        },
      }),
      resource({
        id: 'pbs-store',
        type: 'storage',
        name: 'pbs-offsite',
        platformType: 'proxmox-pve',
        parentId: 'pve-1',
        status: 'online',
        disk: { current: 45, total: 10 * 1024 ** 4, used: Math.round(4.5 * 1024 ** 4) },
        storage: { type: 'pbs' },
      }),
      resource({
        id: 'chk-web',
        type: 'network-endpoint',
        name: 'Public site',
        status: 'online',
        availability: {
          protocol: 'https',
          applicationStatusCode: 200,
          available: true,
          latencyMillis: 24,
        },
      }),
    ];

    it('maps link speeds to connector stroke widths and renders strokeWidth and rect opacity in SVG', () => {
      expect(strokeWidthForLinkSpeed(10000)).toBe(2.75);
      expect(strokeWidthForLinkSpeed(25000)).toBe(2.75);
      expect(strokeWidthForLinkSpeed(2500)).toBe(2.25);
      expect(strokeWidthForLinkSpeed(1000)).toBe(1.75);
      expect(strokeWidthForLinkSpeed(100)).toBe(1.25);
      expect(strokeWidthForLinkSpeed(undefined)).toBe(1.5);

      const miniScene: MapScene = {
        width: 200,
        height: 100,
        nodes: [
          {
            kind: 'rect',
            x: 10,
            y: 10,
            w: 80,
            h: 40,
            r: 6,
            fill: 'surfaceAlt',
            opacity: 0.45,
            stroke: 'border',
          },
          {
            kind: 'line',
            points: [
              [10, 60],
              [120, 60],
            ],
            tone: 'accent',
            strokeWidth: 2.75,
            activity: 'high',
          },
        ],
      };

      const staticSvg = renderMapSceneToSvg(miniScene, MAP_PALETTES.dark);
      expect(staticSvg).toContain('fill-opacity="0.45"');
      expect(staticSvg).toContain('stroke-width="2.75"');
      expect(staticSvg).not.toContain('pulse-map-flow');

      const liveBody = renderMapSceneBody(miniScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(liveBody).toContain('fill-opacity="0.45"');
      expect(liveBody).toContain('stroke-width="2.75"');
      expect(liveBody).toContain('stroke-width="3.4"');
    });

    it('renders host frames with hardware specs, VMID, last octet, VLAN stripes, Compose stacks, storage bars, and speed drops', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(richEstate()));
      const scene = layoutHostDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });
      const drawn = texts(scene);

      // Host headers with hardware specs, cluster name, and live speed
      expect(drawn).toContain('pve-core');
      expect(drawn).toContain('· homelab');
      expect(
        drawn.some(
          (text) => text.includes('16c') && text.includes('64 GB') && text.includes('PVE 8.3'),
        ),
      ).toBe(true);
      expect(drawn).toContain('dock-app');
      expect(
        drawn.some(
          (text) => text.includes('8c') && text.includes('32 GB') && text.includes('Docker 27.3'),
        ),
      ).toBe(true);

      // Horizontal service cards show VMID badges and last IPv4 octets
      expect(drawn).toContain('CT 102');
      expect(drawn).toContain('.102');
      expect(drawn).toContain('VM 105');
      expect(drawn).toContain('.105');

      // Left VLAN accent stripes (3.5px wide) drawn when multiple networks exist
      const vlanStripes = ofKind(scene, 'rect').filter(
        (node) => node.w === 3.5 && node.h === 36 && node.fill.startsWith('net'),
      );
      expect(vlanStripes.length).toBeGreaterThanOrEqual(3);

      // Host header single-network left accent bar (3px wide, 52px header minus 8px)
      const hostBars = ofKind(scene, 'rect').filter(
        (node) => node.w === 3 && node.h === 44 && node.fill.startsWith('net'),
      );
      expect(hostBars.length).toBeGreaterThanOrEqual(1);

      // Compose stack sub-box drawn with uppercase label and container count
      expect(drawn).toContain('MEDIA-STACK');
      expect(drawn).toContain('sonarr');
      expect(drawn).toContain('radarr');

      // Inline storage pool usage bars with track, severity fills, and percentages
      expect(drawn).toContain('STORAGE');
      expect(drawn).toContain('nvme-pool');
      expect(drawn).toContain('82%');
      expect(drawn).toContain('archive-pool');
      expect(drawn).toContain('94%');
      const barFills = ofKind(scene, 'rect').filter(
        (node) =>
          node.h === 6 &&
          (node.fill === 'attention' || node.fill === 'critical' || node.fill === 'ok'),
      );
      expect(barFills.some((node) => node.fill === 'attention')).toBe(true);
      expect(barFills.some((node) => node.fill === 'critical')).toBe(true);
      expect(barFills.some((node) => node.fill === 'ok')).toBe(true);

      // Link speed pills and stroke widths on host drops + network pills
      expect(drawn).toContain('10G');
      expect(drawn).toContain('2.5G');
      expect(drawn.some((text) => text.includes('10.30.0.0/24') && text.includes('10G'))).toBe(
        true,
      );
      const lines = ofKind(scene, 'line');
      expect(lines.some((node) => node.strokeWidth === 2.75)).toBe(true);
      expect(lines.some((node) => node.strokeWidth === 2.25)).toBe(true);

      // Bottom layer renders backups and standalone checks with probe summary + latency
      expect(drawn).toContain('BACKUPS');
      expect(drawn).toContain('CHECKS');
      expect(drawn).toContain('backups');
      expect(drawn).toContain('checks');
      expect(drawn.some((text) => text.includes('HTTPS 200') && text.includes('24 ms'))).toBe(true);

      // Host header hit carries enriched metadata and wired links to guests and pools
      const hostHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pve-1')?.hit;
      expect(hostHit).toMatchObject({
        cpuCores: 16,
        versionLabel: 'PVE 8.3',
        linkSpeedMbps: 10000,
        navigable: true,
      });
      expect(hostHit?.linked).toContain('ct-102');
      expect(hostHit?.linked).toContain('pool-nvme');

      const ctHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'ct-102')?.hit;
      expect(ctHit?.vmid).toBe(102);

      const poolHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pool-nvme')?.hit;
      expect(poolHit).toMatchObject({
        usedPercent: expect.closeTo(82, 5),
        vdevLayout: 'mirror',
      });
    });

    it('draws compact 40x40 icon tiles inside host frames in marks density and handles overflow bars', () => {
      const manyGuests = Array.from({ length: 10 }, (_, index) =>
        guest(`svc-${index}`, {
          parentId: 'pve-1',
          identity: { ips: [`10.30.0.${50 + index}`] },
          proxmox: { vmid: 200 + index },
        }),
      );
      const resources = [
        resource({
          id: 'pve-1',
          type: 'agent',
          name: 'pve-big',
          platformType: 'proxmox-pve',
          proxmox: { cpus: 32, pveVersion: '8.3.1' },
        }),
        ...manyGuests,
      ];
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources), {
        maxTilesPerGroup: 6,
      });

      const namedScene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const compactScene = layoutHostDiagram(diagram, { width: 1400, density: 'marks' });

      expect(compactScene.height).toBeLessThan(namedScene.height);
      expect(texts(compactScene)).toContain('pve-big');
      expect(texts(compactScene)).not.toContain('svc-0');
      expect(
        ofKind(compactScene, 'hit').filter((node) => node.w === 40 && node.h === 40).length,
      ).toBe(6);

      // Overflow bar opens the host frame drawer group (`host:pve-1`)
      const overflowHit = ofKind(namedScene, 'hit').find(
        (node) =>
          node.hit.inspectTarget?.kind === 'group' &&
          node.hit.inspectTarget.groupKey === 'host:pve-1',
      );
      expect(overflowHit).toBeDefined();
      expect(texts(namedScene)).toContain('+4 more');
    });

    it('strips names, IPv4 octets, VMIDs, and Compose stack names under Hide names', () => {
      const hidden = anonymizeInfrastructureMapModel(buildInfrastructureMapModel(richEstate()));
      const diagram = buildServiceDiagram(hidden);
      const scene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const drawn = texts(scene);
      const joined = drawn.join(' ');

      expect(joined).not.toContain('pve-core');
      expect(joined).not.toContain('dock-app');
      expect(joined).not.toContain('media-stack');
      expect(joined).not.toContain('MEDIA-STACK');
      expect(joined).not.toContain('CT 102');
      expect(joined).not.toContain('VM 105');
      expect(drawn.some((text) => /^\.\d{1,3}$/.test(text))).toBe(false);
    });

    it('renders cleanly for checks-only estates with no monitored hosts and stays inside bounds at all widths', () => {
      const checksOnly: Resource[] = [
        resource({
          id: 'chk-1',
          type: 'network-endpoint',
          name: 'API health',
          status: 'online',
          identity: { ips: ['10.20.0.5'] },
          availability: {
            protocol: 'https',
            applicationStatusCode: 200,
            available: true,
            latencyMillis: 12,
          },
        }),
        resource({
          id: 'chk-2',
          type: 'network-endpoint',
          name: 'DNS check',
          status: 'online',
          identity: { ips: ['10.20.0.6'] },
          availability: { protocol: 'tcp', available: true, latencyMillis: 5 },
        }),
      ];
      const checksDiagram = buildServiceDiagram(buildInfrastructureMapModel(checksOnly));
      const checksScene = layoutHostDiagram(checksDiagram, { width: 900, density: 'names' });
      expect(texts(checksScene)).toContain('CHECKS');
      expect(texts(checksScene)).toContain('API health');

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(richEstate()));
      for (const width of [320, 375, 768, 1024, 1400, 1900]) {
        for (const density of ['names', 'marks'] as const) {
          const scene = layoutHostDiagram(diagram, { width, density, liveTraffic: true });
          expect(scene.width).toBeLessThanOrEqual(Math.max(width, 300));
          for (const node of scene.nodes) {
            if (node.kind === 'rect' || node.kind === 'hit') {
              expect(node.x, `${width} ${density}`).toBeGreaterThanOrEqual(0);
              expect(node.x + node.w, `${width} ${density}`).toBeLessThanOrEqual(scene.width + 0.5);
              expect(node.y + node.h, `${width} ${density}`).toBeLessThanOrEqual(
                scene.height + 0.5,
              );
            }
            if (node.kind === 'line') {
              for (const [x, y] of node.points) {
                expect(x, `${width} ${density}`).toBeGreaterThanOrEqual(0);
                expect(x, `${width} ${density}`).toBeLessThanOrEqual(scene.width);
                expect(y, `${width} ${density}`).toBeLessThanOrEqual(scene.height + 0.5);
              }
            }
          }
        }
      }
    });

    it('delegates layoutMode hosts through layoutServiceDiagram, fitServiceDiagram, flowServiceDiagram, and buildMapExportSvg', () => {
      const model = buildInfrastructureMapModel(richEstate());
      const diagram = buildServiceDiagram(model);

      const viaDelegate = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        layoutMode: 'hosts',
      });
      expect(texts(viaDelegate)).toContain('CT 102');
      expect(texts(viaDelegate)).toContain('MEDIA-STACK');

      const fitted = fitServiceDiagram(diagram, { width: 1400, height: 900 }, 'names', {
        layoutMode: 'hosts',
        liveTraffic: true,
      });
      expect(texts(fitted.scene)).toContain('pve-core');
      expect(texts(fitted.scene)).toContain('MEDIA-STACK');

      const flowed = flowServiceDiagram(diagram, 375, 'names', {
        layoutMode: 'hosts',
        liveTraffic: false,
      });
      expect(texts(flowed.scene)).toContain('pve-core');

      const exported = buildMapExportSvg(model, {
        title: 'Infrastructure map',
        density: 'names',
        palette: MAP_PALETTES.dark,
        layoutMode: 'hosts',
        now: new Date('2026-11-27T12:00:00Z'),
      });
      expect(exported.svg).toContain('Infrastructure map');
      expect(exported.svg).toContain('MEDIA-STACK');
      expect(exported.svg).toContain('CT 102');
      expect(exported.svg).toContain('stroke-width="2.75"');
      expect(exported.svg).not.toContain('class=');

      // Roles mode also applies link-speed stroke widths to hardware and network drops
      const rolesScene = layoutServiceDiagram(diagram, {
        width: 1400,
        density: 'names',
        layoutMode: 'roles',
      });
      expect(ofKind(rolesScene, 'line').some((node) => node.strokeWidth === 2.75)).toBe(true);
    });

    it('wraps 2+ clustered hosts in a dashed cluster container with shared storage footer and keeps bounds across widths', () => {
      const clusteredEstate: Resource[] = [
        resource({
          id: 'pve-a',
          type: 'agent',
          name: 'pve-a',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          status: 'online',
          proxmox: {
            isClusterMember: true,
            nodeName: 'pve-a',
            clusterName: 'homelab',
            cpus: 16,
            pveVersion: '8.3.1',
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['10.30.0.11/24'], speedMbps: 10000, linkUp: true },
            ],
          },
        }),
        resource({
          id: 'pve-b',
          type: 'agent',
          name: 'pve-b',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          status: 'online',
          proxmox: {
            isClusterMember: true,
            nodeName: 'pve-b',
            clusterName: 'homelab',
            cpus: 16,
            pveVersion: '8.3.1',
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['10.30.0.12/24'], speedMbps: 10000, linkUp: true },
            ],
          },
        }),
        guest('jellyfin', {
          id: 'ct-a1',
          parentId: 'pve-a',
          identity: { ips: ['10.30.0.51'] },
        }),
        guest('grafana', {
          id: 'ct-b1',
          parentId: 'pve-b',
          identity: { ips: ['10.30.0.52'] },
        }),
        resource({
          id: 'ceph-pool',
          type: 'storage',
          name: 'ceph-rbd',
          platformType: 'proxmox-pve',
          proxmox: { instance: 'homelab' },
          status: 'online',
          disk: { current: 62, total: 12 * 1024 ** 4, used: Math.round(7.44 * 1024 ** 4) },
          storage: { type: 'rbd', shared: true, nodes: ['pve-a', 'pve-b'] },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(clusteredEstate));
      const desktop = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const drawn = texts(desktop);

      // Dashed cluster container with 0.35 opacity and radius 10
      const clusterRect = ofKind(desktop, 'rect').find(
        (node) => node.dashed && node.r === 10 && node.opacity === 0.35,
      );
      expect(clusterRect).toBeDefined();

      // Uppercase cluster header naming cluster, type, and node count
      expect(
        drawn.some((text) => text.startsWith('HOMELAB · ') && text.endsWith(' · 2 NODES')),
      ).toBe(true);

      // Cluster-shared storage rendered inside the cluster footer, not duplicated as a separate bottom group
      expect(drawn).toContain('SHARED STORAGE');
      expect(drawn).toContain('ceph-rbd');
      expect(drawn).toContain('62%');

      // Child host frames sit side-by-side inside the cluster on desktop
      const hitA = ofKind(desktop, 'hit').find((node) => node.hit.id === 'pve-a')!;
      const hitB = ofKind(desktop, 'hit').find((node) => node.hit.id === 'pve-b')!;
      expect(hitA.y).toBe(hitB.y);
      expect(hitA.x).toBeGreaterThan(clusterRect!.x);
      expect(hitB.x + hitB.w).toBeLessThan(clusterRect!.x + clusterRect!.w);

      // Wraps cleanly inside bounds on mobile and desktop viewports
      for (const width of [320, 375, 768, 1400]) {
        const scene = layoutHostDiagram(diagram, { width, density: 'names' });
        expect(scene.width).toBeLessThanOrEqual(Math.max(width, 300));
        for (const node of scene.nodes) {
          if (node.kind === 'rect' || node.kind === 'hit') {
            expect(node.x, `${width}`).toBeGreaterThanOrEqual(0);
            expect(node.x + node.w, `${width}`).toBeLessThanOrEqual(scene.width + 0.5);
            expect(node.y + node.h, `${width}`).toBeLessThanOrEqual(scene.height + 0.5);
          }
          if (node.kind === 'line') {
            for (const [x, y] of node.points) {
              expect(x, `${width}`).toBeGreaterThanOrEqual(0);
              expect(x, `${width}`).toBeLessThanOrEqual(scene.width);
              expect(y, `${width}`).toBeLessThanOrEqual(scene.height + 0.5);
            }
          }
        }
      }
    });

    it('renders 2-row host header with CPU/MEM micro-meters, BUSY and DOWN pills, non-overlapping inspect count pill, and VLAN/saturated drop connectors', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-fast',
          type: 'agent',
          name: 'pve-fast',
          platformType: 'proxmox-pve',
          status: 'online',
          cpu: { current: 28 },
          memory: { current: 58, total: 64 * 1024 ** 3, used: Math.round(37 * 1024 ** 3) },
          network: { rxBytes: 25_000_000, txBytes: 5_000_000 },
          proxmox: { cpus: 16, pveVersion: '8.3.1' },
          agent: {
            networkInterfaces: [
              { name: 'eno1', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
            ],
          },
        }),
        guest('jellyfin', {
          id: 'ct-jf',
          parentId: 'pve-fast',
          identity: { ips: ['10.30.0.50'] },
        }),
        resource({
          id: 'pve-busy',
          type: 'agent',
          name: 'pve-busy',
          platformType: 'proxmox-pve',
          status: 'online',
          cpu: { current: 82 },
          memory: { current: 93, total: 32 * 1024 ** 3, used: Math.round(29.76 * 1024 ** 3) },
          // 110 MB/s on a 1000 Mbps link = 88% full-duplex peak utilization
          network: { rxBytes: 110_000_000, txBytes: 4_000_000 },
          proxmox: { cpus: 8, pveVersion: '8.3.1' },
          agent: {
            networkInterfaces: [
              { name: 'eno1', addresses: ['10.40.0.10/24'], speedMbps: 1000, linkUp: true },
              { name: 'eno2', addresses: [], speedMbps: 1000, linkUp: false },
            ],
          },
        }),
        guest('grafana', {
          id: 'ct-gf',
          parentId: 'pve-busy',
          identity: { ips: ['10.40.0.50'] },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const scene = layoutHostDiagram(diagram, {
        width: 1400,
        density: 'names',
        liveTraffic: true,
      });
      const drawn = texts(scene);

      // Row 2 CPU & MEM micro-meters (5px tall bars with severity colours)
      expect(drawn).toContain('CPU');
      expect(drawn).toContain('28%');
      expect(drawn).toContain('MEM');
      expect(drawn).toContain('58%');
      expect(drawn).toContain('82%');
      expect(drawn).toContain('93%');
      const meterFills = ofKind(scene, 'rect').filter(
        (node) => node.h === 5 && node.fill !== 'border',
      );
      expect(meterFills.some((node) => node.fill === 'ok')).toBe(true);
      expect(meterFills.some((node) => node.fill === 'attention')).toBe(true);
      expect(meterFills.some((node) => node.fill === 'critical')).toBe(true);

      // Row 1 inline amber BUSY 88% and 1 DOWN pills
      expect(drawn).toContain('BUSY 88%');
      expect(drawn).toContain('1 DOWN');
      const amberPills = ofKind(scene, 'rect').filter(
        (node) => node.fill === 'attention' && node.opacity === 0.18 && node.stroke === 'attention',
      );
      expect(amberPills).toHaveLength(2);

      // Non-overlapping host inspect count pill (`host:pve-fast` and `host:pve-busy`)
      const fastHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pve-fast')!;
      const fastInspect = ofKind(scene, 'inspect').find(
        (node) => node.target.kind === 'group' && node.target.groupKey === 'host:pve-fast',
      )!;
      expect(fastInspect).toBeDefined();
      expect(fastInspect.label).toBe('Inspect pve-fast (1)');
      expect(fastHit.x + fastHit.w).toBeLessThan(fastInspect.x);

      // VLAN-coloured drop for pve-fast (`net0`, 10G -> 2.75px) and amber saturated drop for pve-busy (`attention`, 1G -> 1.75px)
      const lines = ofKind(scene, 'line');
      const vlanDrop = lines.find(
        (node) => node.arrow && node.strokeWidth === 2.75 && node.tone.startsWith('net'),
      );
      expect(vlanDrop).toBeDefined();

      const saturatedDrop = lines.find(
        (node) => node.arrow && node.strokeWidth === 1.75 && node.tone === 'attention',
      );
      expect(saturatedDrop).toBeDefined();
      expect(drawn).toContain('1G · 88%');

      // Narrow host header (< 250px) falls back to compact meter text without overflowing
      const narrowDiagram = buildServiceDiagram(
        buildInfrastructureMapModel([
          resource({
            id: 'tiny-host',
            type: 'agent',
            name: 'tiny',
            platformType: 'proxmox-pve',
            status: 'online',
            cpu: { current: 28 },
            memory: { current: 58 },
          }),
        ]),
      );
      const narrowScene = layoutHostDiagram(narrowDiagram, { width: 240, density: 'names' });
      expect(texts(narrowScene)).toContain('CPU 28% · MEM 58%');
    });

    it('groups 5+ loose host workloads across 2+ categories under uppercase sub-headers and keeps <=4 flat', () => {
      const busyHost = [
        resource({
          id: 'pve-cat',
          type: 'agent',
          name: 'pve-cat',
          platformType: 'proxmox-pve',
          status: 'online',
        }),
        guest('jellyfin', { parentId: 'pve-cat' }),
        guest('sonarr', { parentId: 'pve-cat' }),
        guest('grafana', { parentId: 'pve-cat' }),
        guest('prometheus', { parentId: 'pve-cat' }),
        guest('postgres', { parentId: 'pve-cat' }),
      ];
      const busyScene = layoutHostDiagram(
        buildServiceDiagram(buildInfrastructureMapModel(busyHost)),
        { width: 1400, density: 'names' },
      );
      const busySubheaders = ofKind(busyScene, 'text')
        .filter((node) => node.caps && node.size === 10 && node.weight === 600)
        .map((node) => node.text);
      expect(busySubheaders).toContain('MEDIA');
      expect(busySubheaders).toContain('MONITORING');
      expect(busySubheaders).toContain('DATABASES');

      // <= 4 loose workloads stay flat with no category sub-headers
      const smallHost = [
        resource({
          id: 'pve-small',
          type: 'agent',
          name: 'pve-small',
          platformType: 'proxmox-pve',
          status: 'online',
        }),
        guest('jellyfin', { parentId: 'pve-small' }),
        guest('grafana', { parentId: 'pve-small' }),
        guest('postgres', { parentId: 'pve-small' }),
        guest('gitea', { parentId: 'pve-small' }),
      ];
      const smallScene = layoutHostDiagram(
        buildServiceDiagram(buildInfrastructureMapModel(smallHost)),
        { width: 1400, density: 'names' },
      );
      const smallSubheaders = ofKind(smallScene, 'text')
        .filter((node) => node.caps && node.size === 10 && node.weight === 600)
        .map((node) => node.text);
      expect(smallSubheaders).not.toContain('MEDIA');
      expect(smallSubheaders).not.toContain('MONITORING');
      expect(smallSubheaders).not.toContain('DATABASES');
    });

    it('appends exposed port detail on horizontal cards when space allows', () => {
      const resources: Resource[] = [
        resource({
          id: 'dock-1',
          type: 'docker-host',
          name: 'dock-1',
          platformType: 'docker',
          status: 'online',
        }),
        resource({
          id: 'c-proxy',
          type: 'app-container',
          name: 'traefik',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: 'a'.repeat(64),
            containerState: 'running',
            image: 'traefik:latest',
            networks: [{ name: 'proxy' }],
          },
        }),
        resource({
          id: 'c-notes',
          type: 'app-container',
          name: 'notes',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: 'b'.repeat(64),
            containerState: 'running',
            image: 'example/notes:latest',
            networks: [{ name: 'proxy' }],
            labels: {
              'traefik.enable': 'true',
              'traefik.http.routers.notes.rule': 'Host(`notes.home.arpa`)',
            },
            ports: [{ publicPort: 3000, privatePort: 3000, protocol: 'tcp' }],
          },
        }),
      ];

      const scene = layoutHostDiagram(buildServiceDiagram(buildInfrastructureMapModel(resources)), {
        width: 1400,
        density: 'names',
      });
      expect(texts(scene)).toContain('notes.home.arpa · :3000');
    });

    it('renders relationship-labelled hover arcs when <= 6 arcs, clamps long labels, and omits them when > 6', () => {
      const resources: Resource[] = [
        resource({
          id: 'dock-1',
          type: 'docker-host',
          name: 'dock-1',
          platformType: 'docker',
          status: 'online',
        }),
        resource({
          id: 'c-proxy',
          type: 'app-container',
          name: 'traefik',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: 'c'.repeat(64),
            containerState: 'running',
            image: 'traefik:latest',
            networks: [{ name: 'proxy' }],
          },
        }),
        resource({
          id: 'c-app',
          type: 'app-container',
          name: 'memos',
          platformType: 'docker',
          parentId: 'dock-1',
          status: 'running',
          docker: {
            hostSourceId: 'dock-1',
            containerId: 'd'.repeat(64),
            containerState: 'running',
            image: 'neosmemo/memos:latest',
            networks: [{ name: 'proxy' }],
            labels: {
              'traefik.enable': 'true',
              'traefik.http.routers.memos.rule':
                'Host(`very-long-service-hostname.internal.example.org`)',
            },
          },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const hostScene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const proxyHit = ofKind(hostScene, 'hit').find((node) => node.hit.id === 'c-proxy')!.hit;
      expect(proxyHit.linkLabels?.['c-app']).toBe(
        'very-long-service-hostname.internal.example.org',
      );

      const overlay = renderMapLinkRings(hostScene, proxyHit, MAP_PAGE_PALETTES.dark);
      expect(overlay).toContain('class="pulse-map-link-label"');
      // Long label is clamped to <= 24 chars with ellipsis
      expect(overlay).toContain('>very-long-service-hostn…</text>');
      // Label pills render after arcs and dots so arcs never cross over pills
      expect(overlay.lastIndexOf('pulse-map-link-arc')).toBeLessThan(
        overlay.indexOf('pulse-map-link-label'),
      );
      expect(overlay.lastIndexOf('<circle ')).toBeLessThan(overlay.indexOf('pulse-map-link-label'));

      // When > 6 targets are linked, relationship pills are omitted to prevent clutter
      const crowdedResources: Resource[] = [
        resource({ id: 'pve-hub', type: 'agent', name: 'pve-hub', platformType: 'proxmox-pve' }),
        ...Array.from({ length: 8 }, (_, index) => guest(`svc-${index}`, { parentId: 'pve-hub' })),
      ];
      const crowdedScene = layoutHostDiagram(
        buildServiceDiagram(buildInfrastructureMapModel(crowdedResources)),
        { width: 1400, density: 'names' },
      );
      const hubHit = ofKind(crowdedScene, 'hit').find((node) => node.hit.id === 'pve-hub')!.hit;
      expect(hubHit.linked?.length).toBeGreaterThan(6);
      const crowdedOverlay = renderMapLinkRings(crowdedScene, hubHit, MAP_PAGE_PALETTES.dark);
      expect(crowdedOverlay).not.toContain('pulse-map-link-label');
    });

    it('draws compact inline link-speed key in poster footer when showLinkSpeedKey is true and space permits', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(richEstate()));
      const withKey = layoutHostDiagram(diagram, {
        width: 1480,
        density: 'names',
        header: {
          title: 'Infrastructure map',
          facts: ['2 hosts'],
          footer: 'Drawn by Pulse · 2026-11-27',
          showLinkSpeedKey: true,
        },
      });
      const withKeyTexts = texts(withKey);
      expect(withKeyTexts).toContain('Link speed');
      expect(
        ofKind(withKey, 'rect').some((node) => node.h === 2.75 && node.fill === 'accent'),
      ).toBe(true);
      expect(
        ofKind(withKey, 'rect').some((node) => node.h === 1.75 && node.fill === 'accent'),
      ).toBe(true);

      // Omitted when showLinkSpeedKey is false/unset
      const withoutKey = layoutHostDiagram(diagram, {
        width: 1480,
        density: 'names',
        header: {
          title: 'Infrastructure map',
          facts: ['2 hosts'],
          footer: 'Drawn by Pulse · 2026-11-27',
        },
      });
      expect(texts(withoutKey)).not.toContain('Link speed');
    });

    it('tags live tile nodes for spotlight dimming while keeping static exports clean', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(richEstate()));
      const hostScene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const liveBody = renderMapSceneBody(hostScene, MAP_PAGE_PALETTES.dark, () => null);
      const staticSvg = renderMapSceneToSvg(hostScene, MAP_PALETTES.dark);

      expect(liveBody).toContain(`${MAP_TILE_NODE_ATTRIBUTE}="pve-1"`);
      expect(liveBody).toContain(`${MAP_TILE_NODE_ATTRIBUTE}="ct-102"`);
      expect(staticSvg).not.toContain(`${MAP_TILE_NODE_ATTRIBUTE}=`);

      const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      svg.innerHTML = liveBody;
      applyMapSpotlight(svg, new Set(['pve-1', 'ct-102']));
      expect(svg.classList.contains(MAP_SPOTLIGHT_CLASS)).toBe(true);
      expect(
        svg
          .querySelector(`[${MAP_TILE_NODE_ATTRIBUTE}="pve-1"]`)
          ?.classList.contains(MAP_SPOTLIGHT_LIT_CLASS),
      ).toBe(true);
      expect(
        svg
          .querySelector(`[${MAP_TILE_NODE_ATTRIBUTE}="docker-1"]`)
          ?.classList.contains(MAP_SPOTLIGHT_LIT_CLASS),
      ).toBe(false);

      applyMapSpotlight(svg, null);
      expect(svg.classList.contains(MAP_SPOTLIGHT_CLASS)).toBe(false);
      expect(svg.querySelectorAll(`.${MAP_SPOTLIGHT_LIT_CLASS}`)).toHaveLength(0);
    });

    it('renders zero-truncation host specs alongside meters, host thermals, physical disk strips, and equal-height cluster rows', () => {
      const resources: Resource[] = [
        resource({
          id: 'pve-a',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          cpu: { current: 28 },
          memory: { current: 58, total: 64 * 1024 ** 3, used: 37 * 1024 ** 3 },
          proxmox: { cpus: 16, pveVersion: '8.3.2', temperature: 51 },
        }),
        guest('jellyfin', { id: 'g-1', parentId: 'pve-a' }),
        guest('grafana', { id: 'g-2', parentId: 'pve-a' }),
        guest('plex', { id: 'g-3', parentId: 'pve-a' }),
        resource({
          id: 'disk-1',
          type: 'physical_disk',
          name: 'nvme0n1',
          parentId: 'pve-a',
          physicalDisk: { diskType: 'nvme', temperature: 41, health: 'PASSED' },
        }),
        resource({
          id: 'disk-2',
          type: 'physical_disk',
          name: 'sda',
          parentId: 'pve-a',
          health: { verdict: 'attention', reasons: [] },
          physicalDisk: { diskType: 'hdd', temperature: 56, health: 'WARN' },
        }),
        resource({
          id: 'pve-b',
          type: 'agent',
          name: 'pve-02',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          cpu: { current: 18 },
          memory: { current: 42, total: 64 * 1024 ** 3, used: 27 * 1024 ** 3 },
          proxmox: { cpus: 12, pveVersion: '8.3.2' },
        }),
        guest('nextcloud', { id: 'g-4', parentId: 'pve-b' }),
        resource({
          id: 'nas-box',
          type: 'agent',
          name: 'nas-01',
          platformType: 'truenas',
          cpu: { current: 31 },
          memory: { current: 66, total: 32 * 1024 ** 3, used: 21 * 1024 ** 3 },
          truenas: { version: '24.10.1' },
          agent: { cpuCount: 8 },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const scene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const drawn = texts(scene);

      // Zero-truncation version labels on Row 2 alongside live CPU/MEM meters
      expect(drawn.some((text) => text.includes('PVE 8.3') && !text.includes('…'))).toBe(true);
      expect(drawn.some((text) => text.includes('TrueNAS 24.10') && !text.includes('…'))).toBe(
        true,
      );
      expect(drawn.some((text) => text.includes('51°C'))).toBe(true);

      // Physical disk summary strip on pve-01
      expect(drawn).toContain('DISKS');
      expect(drawn.some((text) => text.includes('1× NVMe · 1× HDD'))).toBe(true);
      expect(drawn.some((text) => text.includes('56°C'))).toBe(true);
      expect(drawn.some((text) => text.includes('1 WARN'))).toBe(true);

      // Equal-height sibling host frames inside cluster row (both share the same frame height)
      const hostFrameRects = ofKind(scene, 'rect').filter(
        (node) => node.r === 8 && node.fill === 'surface' && node.stroke === 'border',
      );
      const pveARect = hostFrameRects[0];
      const pveBRect = hostFrameRects[1];
      expect(pveARect).toBeDefined();
      expect(pveBRect).toBeDefined();
      expect(pveARect.h).toBe(pveBRect.h);
    });

    it('formats platform-aware host workload count labels and falls back to compact counts when narrow or in marks mode', () => {
      expect(hostCountLabel('proxmox', 1)).toBe('1 guest');
      expect(hostCountLabel('proxmox', 3)).toBe('3 guests');
      expect(hostCountLabel('vmware', 1)).toBe('1 guest');
      expect(hostCountLabel('vmware', 4)).toBe('4 guests');
      expect(hostCountLabel('docker', 1)).toBe('1 container');
      expect(hostCountLabel('docker', 5)).toBe('5 containers');
      expect(hostCountLabel('kubernetes', 1)).toBe('1 pod');
      expect(hostCountLabel('kubernetes', 7)).toBe('7 pods');
      expect(hostCountLabel('truenas', 1)).toBe('1 app');
      expect(hostCountLabel('truenas', 2)).toBe('2 apps');
      expect(hostCountLabel('machine', 1)).toBe('1 item');
      expect(hostCountLabel(undefined, 3)).toBe('3 items');

      const multiPlatformResources: Resource[] = [
        resource({
          id: 'pve-host',
          type: 'agent',
          name: 'pve-01',
          platformType: 'proxmox-pve',
        }),
        guest('jellyfin', { id: 'g-pve-1', parentId: 'pve-host' }),
        guest('grafana', { id: 'g-pve-2', parentId: 'pve-host' }),
        resource({
          id: 'esx-host',
          type: 'agent',
          name: 'esxi-01',
          platformType: 'vmware-vsphere',
        }),
        resource({
          id: 'vm-esx-1',
          type: 'vm',
          name: 'app-vm',
          platformType: 'vmware-vsphere',
          parentId: 'esx-host',
          status: 'running',
        }),
        resource({
          id: 'dock-host',
          type: 'docker-host',
          name: 'dock-01',
          platformType: 'docker',
        }),
        resource({
          id: 'c-dock-1',
          type: 'app-container',
          name: 'sonarr',
          platformType: 'docker',
          parentId: 'dock-host',
          status: 'running',
        }),
        resource({
          id: 'c-dock-2',
          type: 'app-container',
          name: 'radarr',
          platformType: 'docker',
          parentId: 'dock-host',
          status: 'running',
        }),
        resource({
          id: 'k8s-host',
          type: 'k8s-cluster',
          name: 'prod-k8s',
          platformType: 'kubernetes',
        }),
        resource({
          id: 'pod-1',
          type: 'pod',
          name: 'api-0',
          platformType: 'kubernetes',
          parentId: 'k8s-host',
          status: 'running',
        }),
        resource({
          id: 'pod-2',
          type: 'pod',
          name: 'web-0',
          platformType: 'kubernetes',
          parentId: 'k8s-host',
          status: 'running',
        }),
        resource({
          id: 'nas-host',
          type: 'agent',
          name: 'truenas-01',
          platformType: 'truenas',
        }),
        resource({
          id: 'app-nas-1',
          type: 'app-container',
          name: 'immich',
          platformType: 'truenas',
          parentId: 'nas-host',
          status: 'running',
        }),
        resource({
          id: 'app-nas-2',
          type: 'app-container',
          name: 'syncthing',
          platformType: 'truenas',
          parentId: 'nas-host',
          status: 'running',
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(multiPlatformResources));
      const namedScene = layoutHostDiagram(diagram, { width: 1600, density: 'names' });
      const namedTexts = texts(namedScene);

      expect(namedTexts).toContain('2 guests');
      expect(namedTexts).toContain('1 guest');
      expect(namedTexts).toContain('2 containers');
      expect(namedTexts).toContain('2 pods');
      expect(namedTexts).toContain('2 apps');
      expect(namedTexts.some((text) => /\bitems?\b/.test(text))).toBe(false);

      // Inspect aria-label format stays Inspect <host> (<count>)
      const inspects = ofKind(namedScene, 'inspect');
      expect(inspects.some((node) => node.label === 'Inspect pve-01 (2)')).toBe(true);
      expect(inspects.some((node) => node.label === 'Inspect dock-01 (2)')).toBe(true);
      expect(inspects.some((node) => node.label === 'Inspect prod-k8s (2)')).toBe(true);
      expect(inspects.some((node) => node.label === 'Inspect truenas-01 (2)')).toBe(true);

      // Compact marks density falls back to numeric counts
      const marksScene = layoutHostDiagram(diagram, { width: 1600, density: 'marks' });
      const marksTexts = texts(marksScene);
      expect(marksTexts).not.toContain('2 guests');
      expect(marksTexts).not.toContain('2 containers');
      expect(marksTexts).not.toContain('2 pods');
      expect(marksTexts).not.toContain('2 apps');
      expect(marksTexts).toContain('2');
    });

    it('renders inline REBUILD, DEGRADED, FAULTED, and ERR storage bar badges with matching bar tones and bounds', () => {
      const baseResources: Resource[] = [
        resource({
          id: 'nas-1',
          type: 'agent',
          name: 'nas-core',
          platformType: 'truenas',
          status: 'online',
        }),
        resource({
          id: 'app-1',
          type: 'app-container',
          name: 'immich',
          platformType: 'truenas',
          parentId: 'nas-1',
          status: 'running',
        }),
        resource({
          id: 'pool-rebuild',
          type: 'storage',
          name: 'fast',
          platformType: 'truenas',
          parentId: 'nas-1',
          status: 'online',
          disk: { current: 42, total: 4 * 1024 ** 4, used: Math.round(1.68 * 1024 ** 4) },
          storage: { type: 'zfspool', vdevLayout: 'mirror' },
        }),
        resource({
          id: 'pool-degraded',
          type: 'storage',
          name: 'tank',
          platformType: 'truenas',
          parentId: 'nas-1',
          status: 'online',
          disk: { current: 55, total: 16 * 1024 ** 4, used: Math.round(8.8 * 1024 ** 4) },
          storage: { type: 'zfspool', vdevLayout: 'raidz2' },
        }),
        resource({
          id: 'pool-faulted',
          type: 'storage',
          name: 'cold',
          platformType: 'truenas',
          parentId: 'nas-1',
          status: 'online',
          disk: { current: 30, total: 8 * 1024 ** 4, used: Math.round(2.4 * 1024 ** 4) },
          storage: { type: 'zfspool', vdevLayout: 'raidz1' },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(baseResources));
      const frame = diagram.hostFrames.find((entry) => entry.id === 'nas-1')!;
      const rebuildPool = frame.pools.find((pool) => pool.id === 'pool-rebuild')!;
      const degradedPool = frame.pools.find((pool) => pool.id === 'pool-degraded')!;
      const faultedPool = frame.pools.find((pool) => pool.id === 'pool-faulted')!;

      rebuildPool.rebuildInProgress = true;
      rebuildPool.zfsState = 'DEGRADED';

      degradedPool.zfsState = 'DEGRADED';
      degradedPool.zfsErrorCount = 3;

      faultedPool.zfsState = 'FAULTED';
      faultedPool.zfsErrorCount = 12;

      const scene = layoutHostDiagram(diagram, { width: 1400, density: 'names' });
      const drawn = texts(scene);

      // REBUILD wins over zfsState when rebuildInProgress is true
      expect(drawn).toContain('REBUILD');
      expect(drawn).toContain('DEGRADED');
      expect(drawn).toContain('FAULTED');
      expect(drawn).toContain('3 ERR');
      expect(drawn).toContain('12 ERR');

      // Pill rect and text tones: amber (attention) for REBUILD and DEGRADED, critical for FAULTED
      const rebuildPill = ofKind(scene, 'rect').find(
        (node) =>
          node.tileId === 'pool-rebuild' &&
          node.fill === 'attention' &&
          node.opacity === 0.18 &&
          node.stroke === 'attention',
      );
      expect(rebuildPill).toBeDefined();
      const rebuildText = ofKind(scene, 'text').find(
        (node) => node.tileId === 'pool-rebuild' && node.text === 'REBUILD',
      );
      expect(rebuildText).toMatchObject({ size: 9, weight: 600, fill: 'attention' });

      const degradedPill = ofKind(scene, 'rect').find(
        (node) =>
          node.tileId === 'pool-degraded' &&
          node.fill === 'attention' &&
          node.opacity === 0.18 &&
          node.stroke === 'attention',
      );
      expect(degradedPill).toBeDefined();

      const faultedPill = ofKind(scene, 'rect').find(
        (node) =>
          node.tileId === 'pool-faulted' &&
          node.fill === 'critical' &&
          node.opacity === 0.18 &&
          node.stroke === 'critical',
      );
      expect(faultedPill).toBeDefined();

      // Bar fill tones are elevated to match anomaly severity even when usage is below 75%
      const rebuildBar = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'pool-rebuild' && node.h === 6 && node.fill !== 'border',
      );
      const degradedBar = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'pool-degraded' && node.h === 6 && node.fill !== 'border',
      );
      const faultedBar = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'pool-faulted' && node.h === 6 && node.fill !== 'border',
      );
      expect(rebuildBar?.fill).toBe('attention');
      expect(degradedBar?.fill).toBe('attention');
      expect(faultedBar?.fill).toBe('critical');

      // Narrow mobile width stays strictly within scene bounds
      const narrowScene = layoutHostDiagram(diagram, { width: 320, density: 'names' });
      for (const node of ofKind(narrowScene, 'rect')) {
        expect(node.x).toBeGreaterThanOrEqual(0);
        expect(node.x + node.w).toBeLessThanOrEqual(narrowScene.width + 0.5);
      }
    });

    it('forwards uptimeSeconds, pendingUpdates, lastBackupAt, zfsState, rebuildInProgress, and zfsErrorCount onto MapHit in host and roles layouts', () => {
      const diagram = buildServiceDiagram(buildInfrastructureMapModel(richEstate()));
      const hostFrame = diagram.hostFrames.find((entry) => entry.id === 'pve-1')!;
      hostFrame.host.uptimeSeconds = 1_209_600;
      hostFrame.host.pendingUpdates = 4;

      const ctTile = hostFrame.allTiles.find((tile) => tile.id === 'ct-102')!;
      ctTile.uptimeSeconds = 86_400;
      ctTile.pendingUpdates = 2;
      ctTile.lastBackupAt = '2026-11-27T04:00:00Z';

      const poolTile = hostFrame.pools.find((pool) => pool.id === 'pool-nvme')!;
      poolTile.zfsState = 'DEGRADED';
      poolTile.rebuildInProgress = true;
      poolTile.zfsErrorCount = 5;

      // Also mirror onto role-diagram groups so drawTile forwarding is verified
      const hwTile = diagram.hardware?.tiles.find((tile) => tile.id === 'pve-1');
      if (hwTile) {
        hwTile.uptimeSeconds = 1_209_600;
        hwTile.pendingUpdates = 4;
      }
      const mediaTile = diagram.groups
        .flatMap((group) => group.tiles)
        .find((tile) => tile.id === 'ct-102');
      if (mediaTile) {
        mediaTile.uptimeSeconds = 86_400;
        mediaTile.pendingUpdates = 2;
        mediaTile.lastBackupAt = '2026-11-27T04:00:00Z';
      }
      const storageTile = diagram.storage?.tiles.find((tile) => tile.id === 'pool-nvme');
      if (storageTile) {
        storageTile.zfsState = 'DEGRADED';
        storageTile.rebuildInProgress = true;
        storageTile.zfsErrorCount = 5;
      }

      for (const scene of [
        layoutHostDiagram(diagram, { width: 1400, density: 'names' }),
        layoutServiceDiagram(diagram, { width: 1400, density: 'names', layoutMode: 'roles' }),
      ]) {
        const hostHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pve-1')?.hit;
        expect(hostHit).toMatchObject({
          uptimeSeconds: 1_209_600,
          pendingUpdates: 4,
        });

        const ctHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'ct-102')?.hit;
        expect(ctHit).toMatchObject({
          uptimeSeconds: 86_400,
          pendingUpdates: 2,
          lastBackupAt: '2026-11-27T04:00:00Z',
        });

        const poolHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pool-nvme')?.hit;
        expect(poolHit).toMatchObject({
          zfsState: 'DEGRADED',
          rebuildInProgress: true,
          zfsErrorCount: 5,
        });
      }
    });

    it('preserves distinguishing suffixes on overflowing card titles, keeps pool capacity units intact, aligns cluster storage footers, connects beside-gateways, and derives workload-aware counts on machine hosts', () => {
      // 1. Suffix-preserving truncation keeps distinguishing suffixes readable
      const a = fitCardTitle('pulse-backup-lab-1', 110, 12, 600);
      const b = fitCardTitle('pulse-backup-lab-2', 110, 12, 600);
      expect(a).toContain('…');
      expect(a.endsWith('-1')).toBe(true);
      expect(b.endsWith('-2')).toBe(true);
      expect(a).not.toBe(b);

      // 2. Machine host with mixed containers and VMs gets "workloads", all-container gets "containers"
      const towerResources: Resource[] = [
        resource({
          id: 'tower',
          type: 'agent',
          name: 'Tower',
          platformType: 'generic',
          status: 'online',
          agent: {
            osName: 'Unraid',
            networkInterfaces: [
              { name: 'br0', addresses: ['192.168.0.10/24'], speedMbps: 1000, linkUp: true },
            ],
            defaultGateways: [{ address: '192.168.0.1', interface: 'br0' }],
          },
        }),
        resource({
          id: 'c-1',
          type: 'app-container',
          name: 'sonarr',
          platformType: 'docker',
          parentId: 'tower',
          status: 'running',
        }),
        resource({
          id: 'vm-1',
          type: 'vm',
          name: 'pulse-dev',
          platformType: 'generic',
          parentId: 'tower',
          status: 'running',
        }),
        resource({
          id: 'adguard',
          type: 'system-container',
          name: 'adguard',
          platformType: 'proxmox-pve',
          parentId: 'pve-tall',
          status: 'running',
          proxmox: {
            networkInterfaces: [{ name: 'eth0', bridge: 'vmbr0', addresses: ['192.168.0.53/24'] }],
          },
        }),
        resource({
          id: 'pve-tall',
          type: 'agent',
          name: 'delly',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          status: 'online',
          proxmox: {
            clusterName: 'homelab',
            isClusterMember: true,
            nodeName: 'delly',
            pveVersion: '8.3.1',
          },
          agent: {
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['192.168.0.5/24'], speedMbps: 1000, linkUp: true },
            ],
            defaultGateways: [{ address: '192.168.0.1', interface: 'vmbr0' }],
          },
        }),
        ...Array.from({ length: 8 }, (_, idx) =>
          resource({
            id: `tall-vm-${idx}`,
            type: 'vm',
            name: `pulse-backup-lab-${idx + 1}`,
            platformType: 'proxmox-pve',
            parentId: 'pve-tall',
            status: 'running',
          }),
        ),
        resource({
          id: 'pve-short',
          type: 'agent',
          name: 'delly2',
          platformType: 'proxmox-pve',
          clusterId: 'homelab',
          status: 'online',
          proxmox: {
            clusterName: 'homelab',
            isClusterMember: true,
            nodeName: 'delly2',
            pveVersion: '8.3.1',
          },
          agent: {
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['192.168.0.6/24'], speedMbps: 1000, linkUp: true },
            ],
          },
        }),
        resource({
          id: 'short-vm',
          type: 'vm',
          name: 'tiny-guest',
          platformType: 'proxmox-pve',
          parentId: 'pve-short',
          status: 'running',
        }),
        resource({
          id: 'pool-tall',
          type: 'storage',
          name: 'local-zfs',
          platformType: 'proxmox-pve',
          parentId: 'pve-tall',
          disk: { total: 442 * 1024 ** 3, used: 238 * 1024 ** 3, current: 54 },
        }),
        resource({
          id: 'pool-short',
          type: 'storage',
          name: 'local-zfs',
          platformType: 'proxmox-pve',
          parentId: 'pve-short',
          disk: { total: 442 * 1024 ** 3, used: 238 * 1024 ** 3, current: 54 },
        }),
        resource({
          id: 'pool-short-2',
          type: 'storage',
          name: 'frigate-storage',
          platformType: 'proxmox-pve',
          parentId: 'pve-short',
          disk: { total: 12 * 1024 ** 3, used: 8 * 1024 ** 3, current: 64 },
        }),
        resource({
          id: 'disk-tall',
          type: 'physical_disk',
          name: 'nvme0',
          platformType: 'proxmox-pve',
          parentId: 'pve-tall',
          physicalDisk: { diskType: 'nvme', temperature: 45, health: 'PASSED' },
        }),
        resource({
          id: 'disk-short',
          type: 'physical_disk',
          name: 'nvme1',
          platformType: 'proxmox-pve',
          parentId: 'pve-short',
          physicalDisk: { diskType: 'nvme', temperature: 48, health: 'PASSED' },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(towerResources));
      const towerFrame = diagram.hostFrames.find((entry) => entry.id === 'tower')!;
      expect(hostCountLabel('machine', 2, towerFrame)).toBe('2 workloads');

      const scene = layoutHostDiagram(diagram, { width: 1440, density: 'names' });
      const drawnTexts = texts(scene);
      expect(drawnTexts).toContain('2 workloads');
      // Capacity strings never get chopped into "442…"
      expect(drawnTexts.some((t) => /^442…$/.test(t))).toBe(false);
      // Beside-gateway connects via dashed elbow line
      const dashedLines = ofKind(scene, 'line').filter(
        (line) => line.dashed && line.points.length === 3,
      );
      expect(dashedLines.length).toBeGreaterThanOrEqual(1);
      // Both cluster siblings align their bottom STORAGE pool bars cleanly above DISKS
      const tallPoolHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pool-tall')!;
      const shortPoolHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'pool-short')!;
      expect(Math.abs(shortPoolHit.y - tallPoolHit.y)).toBeLessThanOrEqual(40);
    });

    it('scales connector flow speed and glow by throughput, highlights spiking services, draws elevated-only load heat strips, and log-scales storage tracks', () => {
      // 1. Continuous flow duration decreases monotonically with throughput
      expect(flowDurationForBytes(5_000)).toBeUndefined();
      const slowDur = flowDurationForBytes(20_000)!;
      const medDur = flowDurationForBytes(2_000_000)!;
      const fastDur = flowDurationForBytes(50_000_000)!;
      expect(slowDur).toBeGreaterThan(medDur);
      expect(medDur).toBeGreaterThan(fastDur);
      expect(fastDur).toBeGreaterThanOrEqual(0.45);

      // 2. Elevated-only load tone stays quiet below 75% / 75C
      expect(elevatedLoadTone(40, 65, 55)).toBeUndefined();
      expect(elevatedLoadTone(78, 40, 50)).toBe('attention');
      expect(elevatedLoadTone(20, 86, 50)).toBe('attention');
      expect(elevatedLoadTone(20, 30, 76)).toBe('attention');
      expect(elevatedLoadTone(92, 40, 50)).toBe('critical');
      expect(elevatedLoadTone(20, 30, 88)).toBe('critical');

      // 3. Log-scaled storage track grows monotonically with capacity within bounds
      const tinyTrack = proportionalStorageTrackWidth(5 * 1024 ** 3, 32, 72);
      const midTrack = proportionalStorageTrackWidth(500 * 1024 ** 3, 32, 72);
      const hugeTrack = proportionalStorageTrackWidth(37 * 1024 ** 4, 32, 72);
      expect(tinyTrack).toBe(32);
      expect(midTrack).toBeGreaterThan(tinyTrack);
      expect(hugeTrack).toBeGreaterThan(midTrack);
      expect(hugeTrack).toBeLessThanOrEqual(72);

      // 4. End-to-end live scene vs static export verification
      const resources: Resource[] = [
        resource({
          id: 'pve-hot',
          type: 'agent',
          name: 'pve-hot',
          platformType: 'proxmox-pve',
          status: 'online',
          cpu: { current: 22 },
          memory: { current: 86, total: 64 * 1024 ** 3, used: 55 * 1024 ** 3 },
          network: { rxBytes: 24 * 1024 * 1024, txBytes: 2 * 1024 * 1024 },
          agent: {
            networkInterfaces: [
              { name: 'vmbr0', addresses: ['10.30.0.10/24'], speedMbps: 10000, linkUp: true },
            ],
          },
        }),
        resource({
          id: 'vm-spike',
          type: 'vm',
          name: 'jellyfin',
          platformType: 'proxmox-pve',
          parentId: 'pve-hot',
          status: 'running',
          cpu: { current: 94 },
          memory: { current: 45 },
          network: { rxBytes: 22 * 1024 * 1024, txBytes: 1 * 1024 * 1024 },
        }),
        resource({
          id: 'vm-calm',
          type: 'vm',
          name: 'grafana',
          platformType: 'proxmox-pve',
          parentId: 'pve-hot',
          status: 'running',
          cpu: { current: 8 },
          memory: { current: 24 },
          network: { rxBytes: 80 * 1024, txBytes: 20 * 1024 },
        }),
        resource({
          id: 'pool-small',
          type: 'storage',
          name: 'local',
          platformType: 'proxmox-pve',
          parentId: 'pve-hot',
          disk: { total: 5 * 1024 ** 3, used: 1 * 1024 ** 3, current: 20 },
        }),
        resource({
          id: 'pool-big',
          type: 'storage',
          name: 'tank',
          platformType: 'proxmox-pve',
          parentId: 'pve-hot',
          disk: { total: 37 * 1024 ** 4, used: 20 * 1024 ** 4, current: 54 },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const spikeTile = diagram.hostFrames[0].allTiles.find((t) => t.id === 'vm-spike')!;
      const calmTile = diagram.hostFrames[0].allTiles.find((t) => t.id === 'vm-calm')!;
      expect(isSpikingServiceTile(spikeTile, spikeTile.netTotalBytes ?? 0, true)).toBe(true);
      expect(isSpikingServiceTile(calmTile, spikeTile.netTotalBytes ?? 0, true)).toBe(false);
      expect(isSpikingServiceTile(spikeTile, spikeTile.netTotalBytes ?? 0, false)).toBe(false);

      const scene = layoutHostDiagram(diagram, {
        width: 1440,
        density: 'names',
        liveTraffic: true,
      });

      // Spiking tile gets spike highlight + wider 2px bottom traffic strip than calm tile
      const spikeRect = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'vm-spike' && node.spike === true,
      );
      expect(spikeRect).toBeDefined();
      const spikeTrafficBar = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'vm-spike' && node.h === 2 && node.fill === 'accent',
      );
      const calmTrafficBar = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'vm-calm' && node.h === 2 && node.fill === 'accent',
      );
      expect(spikeTrafficBar).toBeDefined();
      expect(calmTrafficBar).toBeDefined();
      expect(spikeTrafficBar!.w).toBeGreaterThan(calmTrafficBar!.w);

      // Elevated-only load heat bars appear on hot host (86% MEM -> attention) and hot VM (94% CPU -> critical), never on calm VM
      expect(
        ofKind(scene, 'rect').some(
          (node) => node.tileId === 'pve-hot' && node.h === 2 && node.fill === 'attention',
        ),
      ).toBe(true);
      expect(
        ofKind(scene, 'rect').some(
          (node) => node.tileId === 'vm-spike' && node.h === 2 && node.fill === 'critical',
        ),
      ).toBe(true);
      expect(
        ofKind(scene, 'rect').some(
          (node) =>
            node.tileId === 'vm-calm' &&
            node.h === 2 &&
            (node.fill === 'attention' || node.fill === 'critical'),
        ),
      ).toBe(false);

      // Big pool track is wider than small pool track
      const smallTrackRect = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'pool-small' && node.h === 6 && node.fill === 'border',
      )!;
      const bigTrackRect = ofKind(scene, 'rect').find(
        (node) => node.tileId === 'pool-big' && node.h === 6 && node.fill === 'border',
      )!;
      expect(bigTrackRect.w).toBeGreaterThan(smallTrackRect.w);

      // Live SVG emits continuous animation-duration, flow glow (>= 5 MB/s), and pulse-map-spike; static export omits all live motion/glow
      const heavyLine = ofKind(scene, 'line').find(
        (line) => (line.flowBytes ?? 0) >= MAP_FLOW_GLOW_MIN_BYTES,
      );
      expect(heavyLine).toBeDefined();

      const liveSvg = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
      expect(liveSvg).toContain('pulse-map-flow-glow');
      expect(liveSvg).toContain('animation-duration:');
      expect(liveSvg).toContain('pulse-map-spike');

      const staticSvg = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
      expect(staticSvg).not.toContain('pulse-map-flow-glow');
      expect(staticSvg).not.toContain('pulse-map-spike');
      expect(staticSvg).not.toContain('animation-duration:');
    });

    it('renders host GPU badges and workload hardware accelerator pills on cards and hits', () => {
      const model = buildInfrastructureMapModel([
        resource({
          id: 'host-gpu',
          type: 'agent',
          name: 'gpu-box',
          platformType: 'generic',
          agent: {
            sensors: {
              temperatureCelsius: { cpu: 49 },
              gpu: [
                {
                  id: 'gpu0',
                  name: 'NVIDIA GeForce RTX 4090',
                  utilizationPercent: 55,
                  temperatureCelsius: 64,
                },
              ],
            },
          },
        }),
        resource({
          id: 'c-frigate',
          type: 'app-container',
          name: 'frigate',
          parentId: 'host-gpu',
          platformType: 'docker',
          docker: {
            image: 'ghcr.io/blakeblackshear/frigate:stable',
            accelerators: ['TPU', 'iGPU'],
          },
        }),
      ]);
      const diagram = buildServiceDiagram(model);
      const scene = layoutHostDiagram(diagram, {
        width: 1280,
        density: 'names',
        liveTraffic: false,
      });
      const texts = ofKind(scene, 'text').map((node) => node.text);
      expect(texts).toContain('GPU 55%');
      expect(texts).toContain('iGPU +1');
      const hostHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'host-gpu')?.hit;
      expect(hostHit?.gpuSummary).toBe('RTX 4090');
      const frigateHit = ofKind(scene, 'hit').find((node) => node.hit.id === 'c-frigate')?.hit;
      expect(frigateHit?.accelerators).toEqual(['iGPU', 'TPU']);
    });

    it('wires diskIO into directional tile readouts, spike halos, and ambient at-rest flow arcs', () => {
      const poolTotal = 20 * 1024 ** 4;
      const poolUsed = Math.round(poolTotal * 0.42);
      const resources: Resource[] = [
        resource({
          id: 'host-tower',
          type: 'agent',
          name: 'Tower',
          platformType: 'unraid',
          status: 'online',
        }),
        resource({
          id: 'pool-array',
          type: 'storage',
          name: 'Tower Array',
          platformType: 'unraid',
          parentId: 'host-tower',
          status: 'online',
          disk: { current: 42, total: poolTotal, used: poolUsed },
          storage: { path: '/mnt/user' },
        }),
        resource({
          id: 'c-jellyfin',
          type: 'app-container',
          name: 'jellyfin',
          platformType: 'docker',
          parentId: 'host-tower',
          status: 'running',
          network: { rxBytes: 40 * 1024, txBytes: 2.4 * 1024 * 1024 },
          diskIO: { readRate: 14 * 1024 * 1024, writeRate: 64 * 1024 },
          docker: {
            image: 'jellyfin/jellyfin:latest',
            mounts: [{ type: 'bind', source: '/mnt/user/media', destination: '/media' }],
          },
        }),
        resource({
          id: 'c-idle',
          type: 'app-container',
          name: 'uptime-kuma',
          platformType: 'docker',
          parentId: 'host-tower',
          status: 'running',
          network: { rxBytes: 4 * 1024, txBytes: 4 * 1024 },
          diskIO: { readRate: 8 * 1024, writeRate: 8 * 1024 },
          docker: { image: 'louislam/uptime-kuma:latest' },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const jellyfinTile = diagram.hostFrames[0].allTiles.find((t) => t.id === 'c-jellyfin')!;
      const idleTile = diagram.hostFrames[0].allTiles.find((t) => t.id === 'c-idle')!;

      expect(jellyfinTile.diskReadBytes).toBe(14 * 1024 * 1024);
      expect(tileActivityTier(jellyfinTile)).toBe('medium');
      expect(tileActivityTier(idleTile)).toBeUndefined();
      expect(
        isSpikingServiceTile(
          jellyfinTile,
          jellyfinTile.netTotalBytes ?? 0,
          true,
          jellyfinTile.diskTotalBytes ?? 0,
        ),
      ).toBe(true);
      expect(
        isSpikingServiceTile(
          idleTile,
          jellyfinTile.netTotalBytes ?? 0,
          true,
          jellyfinTile.diskTotalBytes ?? 0,
        ),
      ).toBe(false);

      const subtitle = formatLiveActivitySubtitle(jellyfinTile, 180);
      expect(subtitle).toContain('↑');
      expect(subtitle).toContain('Read');

      const scene = layoutHostDiagram(diagram, {
        width: 1440,
        density: 'names',
        liveTraffic: true,
      });

      const liveSvg = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
      expect(liveSvg).toContain('pulse-map-spike-halo');

      // Balanced net & disk subtitles always retain /s units.
      const balancedNet = formatLiveActivitySubtitle(
        {
          ...jellyfinTile,
          netRxBytes: 1.2 * 1024 * 1024,
          netTxBytes: 2.4 * 1024 * 1024,
          netTotalBytes: 3.6 * 1024 * 1024,
          trafficTier: 'medium',
          diskReadBytes: undefined,
          diskWriteBytes: undefined,
          diskTotalBytes: undefined,
          diskTier: undefined,
        },
        180,
      );
      expect(balancedNet).toContain('/s');
      expect(balancedNet).toMatch(/↓.*\/s.*↑.*\/s/);

      const balancedDisk = formatLiveActivitySubtitle(
        {
          ...jellyfinTile,
          netRxBytes: undefined,
          netTxBytes: undefined,
          netTotalBytes: undefined,
          trafficTier: undefined,
          diskReadBytes: 4 * 1024 * 1024,
          diskWriteBytes: 3 * 1024 * 1024,
          diskTotalBytes: 7 * 1024 * 1024,
          diskTier: 'medium',
        },
        180,
      );
      expect(balancedDisk).toMatch(/R .*\/s.*W .*\/s/);

      // Static export keeps packets and spike halos out
      const staticSvg = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
      expect(staticSvg).not.toContain('pulse-map-spike-halo');
      expect(staticSvg).not.toContain('pulse-map-packet');
      expect(staticSvg).not.toContain('animateMotion');

      // Flow visual profile scales strictly from trickle -> stream -> heavy -> massive
      const trickle = resolveFlowVisualProfile(64 * 1024);
      const stream = resolveFlowVisualProfile(2.4 * 1024 * 1024);
      const heavy = resolveFlowVisualProfile(18 * 1024 * 1024);
      const massive = resolveFlowVisualProfile(95 * 1024 * 1024);
      expect(trickle.intensity).toBe('trickle');
      expect(trickle.packetCount).toBe(0);
      expect(stream.intensity).toBe('stream');
      expect(heavy.intensity).toBe('heavy');
      expect(massive.intensity).toBe('massive');
      expect(massive.flowWidth).toBeGreaterThan(heavy.flowWidth);
      expect(heavy.flowWidth).toBeGreaterThan(stream.flowWidth);
      expect(stream.flowWidth).toBeGreaterThan(trickle.flowWidth);
      expect(massive.packetRadius).toBeGreaterThan(heavy.packetRadius);
      expect(heavy.packetRadius).toBeGreaterThan(stream.packetRadius);
      expect(massive.packetSpeedPxPerSec).toBeGreaterThan(heavy.packetSpeedPxPerSec);
      expect(heavy.packetSpeedPxPerSec).toBeGreaterThan(stream.packetSpeedPxPerSec);
    });

    it('orients host bus connectors by measured upload/download direction and emits one animateMotion per packet group', () => {
      const resources: Resource[] = [
        resource({
          id: 'host-down',
          type: 'agent',
          name: 'downloader',
          platformType: 'proxmox-pve',
          status: 'online',
          network: { rxBytes: 60 * 1024 * 1024, txBytes: 1 * 1024 * 1024 },
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: ['10.20.0.10/24'], speedMbps: 10000 }],
          },
        }),
        resource({
          id: 'host-up',
          type: 'agent',
          name: 'uploader',
          platformType: 'proxmox-pve',
          status: 'online',
          network: { rxBytes: 1 * 1024 * 1024, txBytes: 40 * 1024 * 1024 },
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: ['10.20.0.11/24'], speedMbps: 10000 }],
          },
        }),
        resource({
          id: 'host-even',
          type: 'agent',
          name: 'balanced',
          platformType: 'proxmox-pve',
          status: 'online',
          network: { rxBytes: 15 * 1024 * 1024, txBytes: 15 * 1024 * 1024 },
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: ['10.20.0.12/24'], speedMbps: 10000 }],
          },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));
      const scene = layoutHostDiagram(diagram, {
        width: 1440,
        density: 'names',
        liveTraffic: true,
      });

      const verticalDrops = ofKind(scene, 'line').filter(
        (node) =>
          node.points.length === 2 &&
          node.points[0][0] === node.points[1][0] &&
          node.flowBytes !== undefined &&
          node.points[0][1] !== node.points[1][1],
      );
      const downDrop = verticalDrops.find(
        (node) => !node.bidirectional && node.points[1][1] > node.points[0][1],
      );
      const upDrop = verticalDrops.find(
        (node) => !node.bidirectional && node.points[1][1] < node.points[0][1],
      );
      const evenDrop = verticalDrops.find((node) => node.bidirectional && node.arrow !== true);

      expect(downDrop).toBeDefined();
      expect(upDrop).toBeDefined();
      expect(evenDrop).toBeDefined();

      // Each packet group carries exactly one <animateMotion> instead of duplicating it across 3 circles.
      const svg = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
      const packetGroups = (svg.match(/class="pulse-map-packet /g) ?? []).length;
      const motionTags = (svg.match(/<animateMotion /g) ?? []).length;
      expect(packetGroups).toBeGreaterThan(0);
      expect(motionTags).toBe(packetGroups);
    });

    it('never invents host uplink, Ways in, or By-role directional arrows from guest-only or TX-only traffic', () => {
      const resources: Resource[] = [
        // Host reports 0 B/s on its physical uplink while its guest uploads 40 MB/s (e.g., intra-host traffic).
        resource({
          id: 'pve-idle-nic',
          type: 'agent',
          name: 'pve-idle-nic',
          platformType: 'proxmox-pve',
          status: 'online',
          network: { rxBytes: 0, txBytes: 0 },
          agent: {
            networkInterfaces: [{ name: 'eno1', addresses: ['10.40.0.10/24'], speedMbps: 10000 }],
          },
        }),
        resource({
          id: 'c-traefik',
          type: 'app-container',
          name: 'traefik',
          platformType: 'docker',
          parentId: 'pve-idle-nic',
          status: 'running',
          network: { rxBytes: 0, txBytes: 20 * 1024 * 1024 },
          docker: { image: 'traefik:latest' },
        }),
        resource({
          id: 'vm-jellyfin',
          type: 'vm',
          name: 'jellyfin',
          platformType: 'proxmox-pve',
          parentId: 'pve-idle-nic',
          status: 'running',
          network: { rxBytes: 0, txBytes: 40 * 1024 * 1024 },
          identity: { ips: ['10.40.0.50'] },
        }),
      ];

      const diagram = buildServiceDiagram(buildInfrastructureMapModel(resources));

      // By host: zero directional flow or packets on either the idle host uplink or the Ways in connector.
      const hostScene = layoutHostDiagram(diagram, {
        width: 1440,
        density: 'names',
        liveTraffic: true,
      });
      const directionalHostLines = ofKind(hostScene, 'line').filter(
        (node) => node.activity !== undefined && !node.bidirectional,
      );
      expect(directionalHostLines).toEqual([]);
      const hostSvg = renderMapSceneBody(hostScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(hostSvg).not.toContain('pulse-map-packet');
      expect(hostSvg).not.toContain('pulse-map-flow ');

      // By role: zero directional flow or downward arrows into Media for a TX-only workload.
      const roleScene = layoutServiceDiagram(diagram, {
        width: 1440,
        density: 'names',
        liveTraffic: true,
        layoutMode: 'roles',
      });
      const directionalRoleLines = ofKind(roleScene, 'line').filter(
        (node) => node.activity !== undefined && !node.bidirectional,
      );
      expect(directionalRoleLines).toEqual([]);
      const roleSvg = renderMapSceneBody(roleScene, MAP_PAGE_PALETTES.dark, () => null);
      expect(roleSvg).not.toContain('pulse-map-packet');
      expect(roleSvg).not.toContain('pulse-map-flow ');
    });
  });
});
