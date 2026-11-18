import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { layoutServiceDiagram } from '../infrastructureMapDiagram';
import { buildMapExportSvg } from '../infrastructureMapExport';
import type { MapDensity, MapScene, SceneNode } from '../infrastructureMapLayout';
import { buildInfrastructureMapModel } from '../infrastructureMapModel';
import {
  MAP_PAGE_PALETTES,
  MAP_PALETTES,
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
    for (const label of ['tunnel', 'disks and volumes', 'backups', 'hosted on']) {
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
    for (const absent of ['Internet', 'WAYS IN', 'STORAGE', 'BACKUPS', 'hosted on']) {
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

  it('draws with the theme’s own variables on the page and with literal colours in a file', () => {
    const scene = sceneOf(estate(), 1400);
    const page = renderMapSceneBody(scene, MAP_PAGE_PALETTES.dark, () => null);
    expect(page).toContain('fill="var(--color-bg-surface)" stroke="var(--color-border-base)"');
    expect(page).toContain('fill="var(--color-text-muted)"');
    const file = renderMapSceneToSvg(scene, MAP_PALETTES.dark);
    expect(file).not.toContain('var(');
    expect(file).toContain('fill="#1e293b" stroke="#334155"');
    // An unknown app is its initials on the page's own background.
    expect(file).toMatch(/fill="#0f172a" stroke="#334155"\/><text[^>]*>PC<\/text>/);
  });

  it('heads each box with how many things it holds, folded ones included', () => {
    const drawn = texts(sceneOf(estate(), 1400));
    const at = (title: string) => drawn[drawn.indexOf(title) + 1];
    expect(at('MEDIA')).toBe('3');
    expect(at('OTHER')).toBe('2');
    expect(at('HARDWARE')).toBe('1');
    expect(drawn).toContain('switched off');
  });
});
