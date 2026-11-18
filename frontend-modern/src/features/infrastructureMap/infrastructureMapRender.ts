import { DOCKER_ICON_PATH } from '@/components/icons/DockerIcon';
import { KUBERNETES_ICON_PATH } from '@/components/icons/KubernetesIcon';
import { PROXMOX_ICON_PATH } from '@/components/icons/ProxmoxIcon';
import { TRUENAS_ICON_PATH } from '@/components/icons/TrueNASIcon';
import type { MapScene, MapTone, SceneNode } from './infrastructureMapLayout';
import type { MapHealth, MapPlatform } from './infrastructureMapModel';

/**
 * What the page and the exported image share: one palette, one glyph per
 * platform, and the text of an SVG file built from a laid-out scene.
 */

export type MapPalette = Record<MapTone | MapHealth, string>;

export const MAP_PALETTES: { light: MapPalette; dark: MapPalette } = {
  light: {
    canvas: '#ffffff',
    frame: '#f8fafc',
    card: '#ffffff',
    border: '#e2e8f0',
    track: '#e2e8f0',
    text: '#0f172a',
    muted: '#475569',
    faint: '#94a3b8',
    accent: '#2563eb',
    net0: '#2563eb',
    net1: '#7c3aed',
    net2: '#db2777',
    net3: '#0891b2',
    net4: '#4f46e5',
    net5: '#c026d3',
    netOther: '#94a3b8',
    ok: '#10b981',
    attention: '#f59e0b',
    critical: '#ef4444',
    off: '#cbd5e1',
    stale: '#94a3b8',
    unknown: '#94a3b8',
  },
  dark: {
    canvas: '#1e293b',
    frame: '#172033',
    card: '#0f172a',
    border: '#334155',
    track: '#334155',
    text: '#f1f5f9',
    muted: '#94a3b8',
    faint: '#64748b',
    accent: '#60a5fa',
    net0: '#60a5fa',
    net1: '#a78bfa',
    net2: '#f472b6',
    net3: '#22d3ee',
    net4: '#818cf8',
    net5: '#e879f9',
    netOther: '#64748b',
    ok: '#34d399',
    attention: '#fbbf24',
    critical: '#f87171',
    off: '#475569',
    stale: '#64748b',
    unknown: '#64748b',
  },
};

export const MAP_FONT_FAMILY =
  "ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif";

export interface MapGlyph {
  d: string;
  /** Brand marks are filled shapes; the generic glyphs are 2px outlines. */
  mode: 'fill' | 'stroke';
}

// Every glyph sits in a 24-unit box. Brand marks are the ones the platform
// tabs use; the rest are plain outlines drawn for this map.
export const MAP_GLYPHS: Record<MapPlatform, MapGlyph> = {
  proxmox: { d: PROXMOX_ICON_PATH, mode: 'fill' },
  docker: { d: DOCKER_ICON_PATH, mode: 'fill' },
  kubernetes: { d: KUBERNETES_ICON_PATH, mode: 'fill' },
  truenas: { d: TRUENAS_ICON_PATH, mode: 'fill' },
  vmware: {
    d: 'M6 6h12v12H6zM9.5 9.5h5v5h-5zM9 2v4M15 2v4M9 18v4M15 18v4M2 9h4M2 15h4M18 9h4M18 15h4',
    mode: 'stroke',
  },
  machine: {
    d: 'M4 4h16a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2zM4 13h16a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2zM6 7.5h.01M6 16.5h.01',
    mode: 'stroke',
  },
  backup: {
    d: 'M3 4h18v4H3zM5 8v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8M10 12h4',
    mode: 'stroke',
  },
  mail: {
    d: 'M4 5h16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V7a2 2 0 0 1 2-2zM22 7l-10 6L2 7',
    mode: 'stroke',
  },
  storage: {
    d: 'M3 5c0-1.7 4-3 9-3s9 1.3 9 3-4 3-9 3-9-1.3-9-3zM3 5v14c0 1.7 4 3 9 3s9-1.3 9-3V5M3 12c0 1.7 4 3 9 3s9-1.3 9-3',
    mode: 'stroke',
  },
  checks: { d: 'M22 12h-4l-3 9L9 3l-3 9H2', mode: 'stroke' },
};

const escapeXml = (value: string): string =>
  value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const n = (value: number): string => String(Math.round(value * 100) / 100);

const nodeToSvg = (node: SceneNode, palette: MapPalette): string => {
  switch (node.kind) {
    case 'rect':
      return `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette[node.fill]}"${
        node.stroke ? ` stroke="${palette[node.stroke]}"` : ''
      }/>`;
    case 'text':
      return `<text x="${n(node.x)}" y="${n(node.y)}" font-size="${n(node.size)}" font-weight="${node.weight}" fill="${palette[node.fill]}"${
        node.anchor === 'end' ? ' text-anchor="end"' : ''
      }${node.caps ? ' letter-spacing="0.6"' : ''}>${escapeXml(node.text)}</text>`;
    case 'dot':
      return node.health === 'off'
        ? `<circle cx="${n(node.x)}" cy="${n(node.y)}" r="${n(node.r - 0.75)}" fill="none" stroke="${palette.off}" stroke-width="1.5"/>`
        : `<circle cx="${n(node.x)}" cy="${n(node.y)}" r="${n(node.r)}" fill="${palette[node.health]}"/>`;
    case 'mark':
      return node.health === 'off'
        ? `<rect x="${n(node.x + 0.75)}" y="${n(node.y + 0.75)}" width="${n(node.size - 1.5)}" height="${n(node.size - 1.5)}" rx="1.5" fill="none" stroke="${palette.off}" stroke-width="1.5"/>`
        : `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.size)}" height="${n(node.size)}" rx="2" fill="${palette[node.health]}"/>`;
    case 'icon': {
      const glyph = MAP_GLYPHS[node.platform];
      const paint =
        glyph.mode === 'fill'
          ? `fill="${palette.muted}"`
          : `fill="none" stroke="${palette.muted}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"`;
      return `<path transform="translate(${n(node.x)} ${n(node.y)}) scale(${n(node.size / 24)})" d="${glyph.d}" ${paint}/>`;
    }
    case 'hit':
      return '';
  }
};

export const MAP_HIT_ATTRIBUTE = 'data-map-hit';

// The hover and focus treatment of everything on the map that answers the
// pointer. Kept as one literal so the stylesheet build can see the classes.
const HIT_CLASS =
  'cursor-pointer fill-transparent stroke-transparent outline-none transition-colors hover:fill-sky-500/15 focus-visible:fill-sky-500/15 focus-visible:stroke-sky-500';

/**
 * The inside of the on-page map: the drawn marks, then one target per
 * resource on top. A target the page can open is a link to that resource's
 * own page; the rest still answer hover and focus with their details.
 */
export function renderMapSceneBody(
  scene: MapScene,
  palette: MapPalette,
  hrefFor: (hitIndex: number) => string | null,
): string {
  let hitIndex = 0;
  return scene.nodes
    .map((node) => {
      if (node.kind !== 'hit') return nodeToSvg(node, palette);
      const index = hitIndex;
      hitIndex += 1;
      const label = escapeXml(`${node.hit.name}, ${node.hit.meta}`);
      const rect = `x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" class="${HIT_CLASS}"`;
      const href = node.hit.navigable ? hrefFor(index) : null;
      return href
        ? `<a href="${escapeXml(href)}" ${MAP_HIT_ATTRIBUTE}="${index}" aria-label="${label}"><rect ${rect}/></a>`
        : `<rect ${rect} ${MAP_HIT_ATTRIBUTE}="${index}" tabindex="0" role="img" aria-label="${label}"/>`;
    })
    .join('');
}

/** A complete, self-contained SVG document for a scene. */
export function renderMapSceneToSvg(scene: MapScene, palette: MapPalette): string {
  const body = scene.nodes.map((node) => nodeToSvg(node, palette)).join('');
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${scene.width}" height="${scene.height}" ` +
    `viewBox="0 0 ${scene.width} ${scene.height}" font-family="${escapeXml(MAP_FONT_FAMILY)}">` +
    `<rect width="${scene.width}" height="${scene.height}" fill="${palette.canvas}"/>${body}</svg>`
  );
}
