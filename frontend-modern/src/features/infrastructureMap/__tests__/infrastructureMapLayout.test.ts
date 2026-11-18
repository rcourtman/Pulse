import { describe, expect, it } from 'vitest';
import { buildMapExportSvg, resolveMapExportWidth } from '../infrastructureMapExport';
import {
  MAP_NAMES_ITEM_LIMIT,
  fitMapText,
  layoutInfrastructureMap,
  resolveDefaultMapDensity,
  type MapScene,
  type SceneNode,
} from '../infrastructureMapLayout';
import type { InfrastructureMapModel, MapBlock, MapHost, MapItem } from '../infrastructureMapModel';
import { MAP_PALETTES, renderMapSceneBody, renderMapSceneToSvg } from '../infrastructureMapRender';

const item = (id: string, name = id): MapItem => ({
  id,
  name,
  typeLabel: 'VM',
  health: 'ok',
  status: 'running',
});

const host = (id: string, itemCount = 3, overrides: Partial<MapHost> = {}): MapHost => ({
  id,
  isResource: true,
  name: id,
  platform: 'proxmox',
  typeLabel: 'Proxmox node',
  subtitle: 'PVE 8.2.4',
  health: 'ok',
  status: 'online',
  cpu: 20,
  memory: 50,
  sections:
    itemCount > 0
      ? [
          {
            key: 'vms',
            label: 'VMs',
            items: Array.from({ length: itemCount }, (_, index) => item(`${id}-vm-${index}`)),
          },
        ]
      : [],
  pools: [],
  ownSegmentIds: [],
  guestSegmentIds: [],
  ...overrides,
});

const modelOf = (blocks: MapBlock[]): InfrastructureMapModel => ({
  blocks,
  segments: [],
  totals: [{ key: 'hosts', label: 'Hosts', singular: 'Host', count: blocks.length }],
  attentionCount: 0,
  isEmpty: blocks.length === 0,
});

const hostBlock = (value: MapHost): MapBlock => ({ kind: 'host', host: value });
const groupBlock = (name: string, hosts: MapHost[]): MapBlock => ({
  kind: 'group',
  group: {
    id: `group:${name}`,
    name,
    typeLabel: 'Proxmox cluster',
    platform: 'proxmox',
    hosts,
    pools: [],
  },
});

type Rect = Extract<SceneNode, { kind: 'rect' }>;
const cards = (scene: MapScene): Rect[] =>
  scene.nodes.filter((node): node is Rect => node.kind === 'rect' && node.fill === 'card');
const frames = (scene: MapScene): Rect[] =>
  scene.nodes.filter((node): node is Rect => node.kind === 'rect' && node.fill === 'frame');
const overlaps = (a: Rect, b: Rect): boolean =>
  a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

describe('layoutInfrastructureMap', () => {
  it('flows cards into as many columns as the width holds', () => {
    const model = modelOf(['a', 'b', 'c', 'd'].map((id) => hostBlock(host(id))));
    const wide = layoutInfrastructureMap(model, { width: 1200, density: 'names' });
    expect(new Set(cards(wide).map((card) => card.x)).size).toBe(4);
    expect(new Set(cards(wide).map((card) => card.y)).size).toBe(1);

    const phone = layoutInfrastructureMap(model, { width: 360, density: 'names' });
    expect(new Set(cards(phone).map((card) => card.x)).size).toBe(1);
    expect(phone.width).toBe(360);
    expect(phone.height).toBeGreaterThan(wide.height * 3);
  });

  it('never overlaps two cards and keeps everything inside the scene', () => {
    const model = modelOf([
      groupBlock('one', [host('a', 9), host('b', 2), host('c', 30)]),
      groupBlock('two', [host('d'), host('e'), host('f'), host('g'), host('h', 14)]),
      ...['i', 'j', 'k', 'l', 'm'].map((id, index) => hostBlock(host(id, index * 7))),
    ]);
    for (const width of [320, 700, 1180, 1880]) {
      const scene = layoutInfrastructureMap(model, { width, density: 'names' });
      const drawn = cards(scene);
      expect(drawn).toHaveLength(13);
      drawn.forEach((card, index) => {
        expect(card.x).toBeGreaterThanOrEqual(0);
        expect(card.x + card.w).toBeLessThanOrEqual(scene.width + 0.01);
        expect(card.y + card.h).toBeLessThanOrEqual(scene.height);
        for (const other of drawn.slice(index + 1)) expect(overlaps(card, other)).toBe(false);
      });
      const [first, second] = frames(scene);
      expect(overlaps(first, second)).toBe(false);
    }
  });

  it('moves a later single card up beside a narrow cluster before a full-width one seals the gap', () => {
    const model = modelOf([
      groupBlock('three', [host('a'), host('b'), host('c')]),
      groupBlock('four', [host('d'), host('e'), host('f'), host('g')]),
      hostBlock(host('solo', 1)),
    ]);
    const scene = layoutInfrastructureMap(model, { width: 1200, density: 'names' });
    const [three, four] = frames(scene);
    // Drawn fourth: after the first cluster's three cards, before the second's.
    const solo = cards(scene)[3];
    expect(solo.y).toBe(three.y);
    expect(solo.x).toBeGreaterThan(three.x + three.w);
    expect(four.y).toBeGreaterThan(three.y + three.h);
    expect(four.w).toBeGreaterThan(three.w);
  });

  it('draws names or one mark per item, and one target for each either way', () => {
    const model = modelOf([hostBlock(host('a', 40))]);
    const names = layoutInfrastructureMap(model, { width: 320, density: 'names' });
    const marks = layoutInfrastructureMap(model, { width: 320, density: 'marks' });
    const texts = (scene: MapScene) =>
      scene.nodes.filter((node) => node.kind === 'text' && node.text.startsWith('a-vm-'));
    const hitIds = (scene: MapScene) =>
      scene.nodes.flatMap((node) => (node.kind === 'hit' ? [node.hit.id] : []));

    expect(texts(names)).toHaveLength(40);
    expect(texts(marks)).toHaveLength(0);
    expect(marks.nodes.filter((node) => node.kind === 'mark')).toHaveLength(40);
    expect(marks.height).toBeLessThan(names.height / 2);
    expect(hitIds(names)).toEqual(hitIds(marks));
    expect(hitIds(names)).toHaveLength(41);
  });

  it('draws disks as marks even when names are on', () => {
    const disks = host('nas', 0, {
      sections: [
        { key: 'disks', label: 'Disks', marksOnly: true, items: [item('d1'), item('d2')] },
      ],
    });
    const scene = layoutInfrastructureMap(modelOf([hostBlock(disks)]), {
      width: 320,
      density: 'names',
    });
    expect(scene.nodes.filter((node) => node.kind === 'mark')).toHaveLength(2);
  });

  it('describes each target for hover, and opens only real resources', () => {
    const composed = host('map:checks', 1, { isResource: false, name: 'Availability checks' });
    const scene = layoutInfrastructureMap(modelOf([hostBlock(host('a', 1)), hostBlock(composed)]), {
      width: 700,
      density: 'names',
    });
    const hits = scene.nodes.flatMap((node) => (node.kind === 'hit' ? [node.hit] : []));
    expect(hits.find((hit) => hit.id === 'a')).toMatchObject({
      meta: 'Proxmox node · Online',
      navigable: true,
      cpu: 20,
    });
    expect(hits.find((hit) => hit.id === 'a-vm-0')).toMatchObject({ meta: 'VM · Running' });
    expect(hits.find((hit) => hit.id === 'map:checks')).toMatchObject({ navigable: false });
  });

  it('adds a title and footer only when asked, for the exported image', () => {
    const model = modelOf([hostBlock(host('a'))]);
    const plain = layoutInfrastructureMap(model, { width: 600, density: 'names' });
    const poster = layoutInfrastructureMap(model, {
      width: 600,
      density: 'names',
      header: { title: 'Infrastructure map', facts: ['1 Host'], footer: 'Drawn by Pulse' },
    });
    const textsOf = (scene: MapScene) =>
      scene.nodes.flatMap((node) => (node.kind === 'text' ? [node.text] : []));
    expect(textsOf(plain)).not.toContain('Drawn by Pulse');
    expect(textsOf(poster)).toEqual(
      expect.arrayContaining(['Infrastructure map', 'Drawn by Pulse']),
    );
    expect(poster.height).toBeGreaterThan(plain.height);
  });
});

describe('fitMapText', () => {
  it('leaves short text alone and ends cut text with an ellipsis', () => {
    expect(fitMapText('pve1', 200, 11, 400)).toBe('pve1');
    const cut = fitMapText('a-very-long-container-name-that-cannot-fit', 60, 11, 400);
    expect(cut.endsWith('…')).toBe(true);
    expect(cut.length).toBeLessThan(20);
    expect(fitMapText('anything', 0, 11, 400)).toBe('');
  });
});

describe('resolveDefaultMapDensity', () => {
  it('writes names for a lab and switches to marks for a large estate', () => {
    expect(resolveDefaultMapDensity(modelOf([hostBlock(host('a', MAP_NAMES_ITEM_LIMIT))]))).toBe(
      'names',
    );
    expect(
      resolveDefaultMapDensity(modelOf([hostBlock(host('a', MAP_NAMES_ITEM_LIMIT + 1))])),
    ).toBe('marks');
  });
});

describe('rendering', () => {
  const model = modelOf([
    hostBlock(host('a', 1, { name: 'pve <one> & "two"' })),
    hostBlock(host('map:checks', 0, { isResource: false })),
  ]);
  const scene = layoutInfrastructureMap(model, { width: 700, density: 'names' });

  it('writes a self-contained SVG with reported names escaped', () => {
    const svg = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
    expect(svg.startsWith('<svg xmlns="http://www.w3.org/2000/svg"')).toBe(true);
    expect(svg).toContain('pve &lt;one&gt; &amp; &quot;two&quot;');
    expect(svg).not.toContain('<one>');
    expect(svg).toContain(`fill="${MAP_PALETTES.dark.canvas}"`);
    expect(svg).not.toContain('<a ');
    expect(svg).not.toContain('class=');
  });

  it('links each resource on the page to its own page and nothing else', () => {
    const body = renderMapSceneBody(scene, MAP_PALETTES.light, (index) => `/open?i=${index}&x=1`);
    expect(body.match(/<a href=/g)).toHaveLength(2);
    expect(body).toContain('href="/open?i=0&amp;x=1"');
    expect(body).toContain(
      'aria-label="pve &lt;one&gt; &amp; &quot;two&quot;, Proxmox node · Online"',
    );
    // The composed card is described but opens nothing.
    expect(body.match(/tabindex="0"/g)).toHaveLength(1);
  });

  it('sizes the exported poster to the estate and titles it', () => {
    const small = modelOf([hostBlock(host('a'))]);
    const large = modelOf(Array.from({ length: 40 }, (_, index) => hostBlock(host(`h${index}`))));
    expect(resolveMapExportWidth(small)).toBeLessThan(resolveMapExportWidth(large));

    const exported = buildMapExportSvg(small, {
      title: 'Infrastructure map',
      view: 'hosts',
      density: 'names',
      palette: MAP_PALETTES.dark,
      now: new Date('2026-11-18T10:00:00Z'),
    });
    expect(exported.svg).toContain('Infrastructure map');
    expect(exported.svg).toContain('>1 Host<');
    expect(exported.svg).not.toContain('1 Hosts');
    expect(exported.svg).toContain('Drawn by Pulse · 2026-11-18');
    expect(exported.svg).toContain(`width="${exported.width}"`);
  });
});
