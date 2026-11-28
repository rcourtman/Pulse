import { DOCKER_ICON_PATH } from '@/components/icons/DockerIcon';
import { KUBERNETES_ICON_PATH } from '@/components/icons/KubernetesIcon';
import { PROXMOX_ICON_PATH } from '@/components/icons/ProxmoxIcon';
import { TRUENAS_ICON_PATH } from '@/components/icons/TrueNASIcon';
import { resolveMapAppLogo } from './infrastructureMapCatalogue';
import type { MapGlyphKey, MapHit, MapScene, MapTone, SceneNode } from './infrastructureMapLayout';
import type { MapHealth, MapTrafficTier } from './infrastructureMapModel';

/**
 * What the page and the exported image share: the colours, one glyph per
 * platform, and the text of an SVG file built from a laid-out scene.
 */

export type MapPalette = Record<MapTone | MapHealth | 'none', string>;

// The theme as literal colours. Structure is the semantic tokens' values;
// status and accent are the pairs the shared status dot and links use
// (emerald, amber and red at 500 on light and 400 on dark, blue at 600 and
// 400). A saved image uses these when the page's own values cannot be read.
export const MAP_PALETTES: { light: MapPalette; dark: MapPalette } = {
  light: {
    none: 'none',
    base: '#f1f5f9',
    surface: '#ffffff',
    surfaceAlt: '#f1f5f9',
    border: '#e2e8f0',
    text: '#0f172a',
    muted: '#475569',
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
    off: '#94a3b8',
    stale: '#94a3b8',
    unknown: '#94a3b8',
  },
  dark: {
    none: 'none',
    base: '#0f172a',
    surface: '#1e293b',
    surfaceAlt: '#1e293b',
    border: '#334155',
    text: '#f1f5f9',
    muted: '#cbd5e1',
    accent: '#60a5fa',
    net0: '#60a5fa',
    net1: '#a78bfa',
    net2: '#f472b6',
    net3: '#22d3ee',
    net4: '#818cf8',
    net5: '#e879f9',
    netOther: '#94a3b8',
    ok: '#34d399',
    attention: '#fbbf24',
    critical: '#f87171',
    off: '#94a3b8',
    stale: '#94a3b8',
    unknown: '#94a3b8',
  },
};

// On the page the structure is drawn with references to the theme's own
// variables, so it follows the theme like any other surface. Each is written
// out whole, so a search for a token's variable finds the map. The stylesheet
// only defines variables for the semantic tokens, so status and accent
// colours stay the literal values above.
const pagePalette = (literal: MapPalette): MapPalette => ({
  ...literal,
  base: 'var(--color-bg-base)',
  surface: 'var(--color-bg-surface)',
  surfaceAlt: 'var(--color-bg-surface-alt)',
  border: 'var(--color-border-base)',
  text: 'var(--color-text-base)',
  muted: 'var(--color-text-muted)',
});

export const MAP_PAGE_PALETTES: { light: MapPalette; dark: MapPalette } = {
  light: pagePalette(MAP_PALETTES.light),
  dark: pagePalette(MAP_PALETTES.dark),
};

// The product's own sans stack, written out for a saved image. On the page
// the drawing inherits it.
export const MAP_FONT_FAMILY = 'ui-sans-serif, system-ui, sans-serif';

export interface MapGlyph {
  d: string;
  /** Brand marks are filled shapes; the generic glyphs are 2px outlines. */
  mode: 'fill' | 'stroke';
}

// Every glyph sits in a 24-unit box. Brand marks are the ones the platform
// tabs use; the rest are plain outlines drawn for this map.
export const MAP_GLYPHS: Record<MapGlyphKey, MapGlyph> = {
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
  internet: {
    d: 'M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2c2.8 3 4.2 6.3 4.2 10s-1.4 7-4.2 10c-2.8-3-4.2-6.3-4.2-10S9.2 5 12 2z',
    mode: 'stroke',
  },
  // A router: a box with two aerials and its status lights.
  gateway: {
    d: 'M4 13h16a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2zM7 13V6M17 13V6M6 16.5h.01M10 16.5h.01',
    mode: 'stroke',
  },
  // A switch: a low box with a row of ports.
  switch: {
    d: 'M3 8h18a1 1 0 0 1 1 1v6a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1zM6 12h.01M10 12h.01M14 12h.01M18 12h.01',
    mode: 'stroke',
  },
  // An access point: the radio waves over it.
  'access-point': {
    d: 'M12 20h.01M2 8.82a15 15 0 0 1 20 0M5 12.86a10 10 0 0 1 14 0M8.5 16.43a5 5 0 0 1 7 0',
    mode: 'stroke',
  },
};

const escapeXml = (value: string): string =>
  value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

const n = (value: number): string => String(Math.round(value * 100) / 100);

// A logo is drawn in white on its brand colour, or in near-black when the
// brand colour is too light for white to read on.
const readsDarkOn = (hex: string): boolean => {
  const channel = (start: number) => parseInt(hex.slice(start, start + 2), 16) / 255;
  return 0.2126 * channel(0) + 0.7152 * channel(2) + 0.0722 * channel(4) > 0.62;
};

// The tile of a thing with no logo: slate, the product's own colour, solid
// like a logo's tile so its initials read as a mark and not as a gap. The
// same in both themes, as brand colours are.
const MONOGRAM_TILE = '#475569';
const MONOGRAM_EDGE = '#64748b';
const MONOGRAM_LETTERS = '#f8fafc';

const monogramFontSize = (size: number, letters: string): number => {
  if (letters.length >= 4) return size * 0.34;
  if (letters.length === 3) return size * 0.39;
  return size * 0.45;
};

export const MAP_TILE_NODE_ATTRIBUTE = 'data-map-tile';
export const MAP_SPOTLIGHT_CLASS = 'pulse-map-spotlight';
export const MAP_SPOTLIGHT_LIT_CLASS = 'pulse-map-spotlight-lit';

const tileAttr = (node: SceneNode, live: boolean): string =>
  live && 'tileId' in node && node.tileId
    ? ` ${MAP_TILE_NODE_ATTRIBUTE}="${escapeXml(node.tileId)}"`
    : '';

// A known app is its logo on its brand colour. Anything else is its initials
// on a slate tile, so the logos are the only colour that is not the product's.
const appToSvg = (
  node: Extract<SceneNode, { kind: 'app' }>,
  palette: MapPalette,
  live: boolean,
): string => {
  const logo = resolveMapAppLogo(node.icon);
  const corner = n(node.size * 0.2);
  const square = `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.size)}" height="${n(node.size)}" rx="${corner}" fill="${logo ? `#${logo.hex}` : MONOGRAM_TILE}" stroke="${logo ? palette.border : MONOGRAM_EDGE}"/>`;
  const inset = node.size * 0.21;
  const mark = logo
    ? `<path transform="translate(${n(node.x + inset)} ${n(node.y + inset)}) scale(${n((node.size - inset * 2) / 24)})" d="${logo.d}" fill="${readsDarkOn(logo.hex) ? '#0f172a' : '#ffffff'}"/>`
    : `<text x="${n(node.x + node.size / 2)}" y="${n(node.y + node.size * 0.65)}" font-size="${n(monogramFontSize(node.size, node.letters))}" font-weight="700" text-anchor="middle" fill="${MONOGRAM_LETTERS}">${escapeXml(node.letters)}</text>`;
  const attr = tileAttr(node, live);
  if (node.dim && attr) return `<g${attr}><g opacity="0.45">${square}${mark}</g></g>`;
  if (node.dim) return `<g opacity="0.45">${square}${mark}</g>`;
  if (attr) return `<g${attr}>${square}${mark}</g>`;
  return square + mark;
};

const ARROW_LENGTH = 7;
const ARROW_HALF_WIDTH = 3.5;

/** Live throughput threshold (5 MB/s) at which active connectors emit a soft outer glow. */
export const MAP_FLOW_GLOW_MIN_BYTES = 5 * 1024 * 1024;

export type MapFlowIntensity = 'trickle' | 'stream' | 'heavy' | 'massive';

export interface MapFlowVisualProfile {
  intensity: MapFlowIntensity;
  flowWidth: number;
  flowOpacity: number;
  dashArray: string;
  dashDuration: number;
  glowExtraWidth: number;
  glowOpacity: number;
  packetCount: number;
  packetRadius: number;
  packetSpeedPxPerSec: number;
}

/**
 * Resolves line width, glow, dash cadence, and traveling packet parameters from
 * live bytes/sec so light background chatter stays quiet while heavy and massive
 * transfers visibly thicken, brighten, and race along the path.
 */
export const resolveFlowVisualProfile = (
  bytesPerSec?: number,
  fallbackTier?: MapTrafficTier,
): MapFlowVisualProfile => {
  const bytes =
    bytesPerSec !== undefined && Number.isFinite(bytesPerSec) && bytesPerSec > 0
      ? bytesPerSec
      : fallbackTier === 'high'
        ? 30 * 1024 * 1024
        : fallbackTier === 'medium'
          ? 3 * 1024 * 1024
          : 50 * 1024;

  if (bytes >= 50 * 1024 * 1024) {
    return {
      intensity: 'massive',
      flowWidth: 4.4,
      flowOpacity: 1,
      dashArray: '10 9',
      dashDuration: 0.55,
      glowExtraWidth: 6.5,
      glowOpacity: 0.34,
      packetCount: 3,
      packetRadius: 3.9,
      packetSpeedPxPerSec: 255,
    };
  }
  if (bytes >= 10 * 1024 * 1024) {
    return {
      intensity: 'heavy',
      flowWidth: 3.4,
      flowOpacity: 0.94,
      dashArray: '8 10',
      dashDuration: 0.92,
      glowExtraWidth: 4.8,
      glowOpacity: 0.26,
      packetCount: 2,
      packetRadius: 3.2,
      packetSpeedPxPerSec: 165,
    };
  }
  if (bytes >= 512 * 1024) {
    const t = Math.min(
      1,
      (Math.log10(bytes) - Math.log10(512 * 1024)) /
        (Math.log10(10 * 1024 * 1024) - Math.log10(512 * 1024)),
    );
    return {
      intensity: 'stream',
      flowWidth: Number((2.2 + t * 0.6).toFixed(2)),
      flowOpacity: Number((0.72 + t * 0.14).toFixed(2)),
      dashArray: '6 11',
      dashDuration: Number((2.1 - t * 0.65).toFixed(2)),
      glowExtraWidth: bytes >= MAP_FLOW_GLOW_MIN_BYTES ? 3.4 : 0,
      glowOpacity: bytes >= MAP_FLOW_GLOW_MIN_BYTES ? 0.2 : 0,
      packetCount: bytes >= 2 * 1024 * 1024 ? 2 : 1,
      packetRadius: Number((2.3 + t * 0.5).toFixed(2)),
      packetSpeedPxPerSec: Math.round(62 + t * 48),
    };
  }
  return {
    intensity: 'trickle',
    flowWidth: 1.5,
    flowOpacity: 0.42,
    dashArray: '3 14',
    dashDuration: 3.6,
    glowExtraWidth: 0,
    glowOpacity: 0,
    packetCount: 0,
    packetRadius: 0,
    packetSpeedPxPerSec: 0,
  };
};

/** Maps live throughput (bytes/sec) smoothly onto a CSS dash animation duration in seconds. */
export const flowDurationForBytes = (bytesPerSec?: number): number | undefined => {
  if (bytesPerSec === undefined || !Number.isFinite(bytesPerSec) || bytesPerSec < 10_000) {
    return undefined;
  }
  const normalized = Math.max(0, Math.min(1, (Math.log10(bytesPerSec) - 4) / 4));
  return Math.round((3.4 - normalized * 2.95) * 100) / 100;
};

const renderTravelingPackets = (
  d: string,
  pathLength: number,
  colour: string,
  coreColour: string,
  profile: MapFlowVisualProfile,
): string => {
  if (profile.packetCount <= 0 || profile.packetSpeedPxPerSec <= 0 || pathLength < 24) {
    return '';
  }
  // Cap packet density on short paths so dots stay cleanly separated.
  const maxForLength = Math.max(1, Math.floor(pathLength / 55));
  const count = Math.min(profile.packetCount, maxForLength);
  const dur = Math.max(0.35, Math.min(5.5, pathLength / profile.packetSpeedPxPerSec));
  const r = profile.packetRadius;
  const haloR = r + 1.8;
  const coreR = Math.max(1.1, r * 0.46);
  const parts: string[] = [];
  for (let i = 0; i < count; i += 1) {
    const begin = -((i / count) * dur);
    parts.push(
      `<g class="pulse-map-packet pulse-map-packet-${profile.intensity}">` +
        `<animateMotion path="${d}" dur="${n(dur)}s" begin="${n(begin)}s" repeatCount="indefinite"/>` +
        `<circle r="${n(haloR)}" fill="${colour}" fill-opacity="0.28"/>` +
        `<circle r="${n(r)}" fill="${colour}" fill-opacity="0.96"/>` +
        `<circle r="${n(coreR)}" fill="${coreColour}" fill-opacity="0.92"/>` +
        `</g>`,
    );
  }
  return parts.join('');
};

const polylineLength = (points: Array<[number, number]>): number => {
  let total = 0;
  for (let i = 1; i < points.length; i += 1) {
    total += Math.hypot(points[i][0] - points[i - 1][0], points[i][1] - points[i - 1][1]);
  }
  return total;
};

const ACTIVITY_FILL_OPACITY: Record<MapTrafficTier, string> = {
  low: '0.07',
  medium: '0.1',
  high: '0.14',
};

const ACTIVITY_STROKE_OPACITY: Record<MapTrafficTier, string> = {
  low: '0.35',
  medium: '0.55',
  high: '0.8',
};

const lineToSvg = (
  node: Extract<SceneNode, { kind: 'line' }>,
  palette: MapPalette,
  live: boolean,
): string => {
  if (node.points.length < 2) return '';
  const colour = palette[node.tone];
  const d = node.points
    .map(([x, y], index) => `${index === 0 ? 'M' : 'L'}${n(x)} ${n(y)}`)
    .join('');
  const strokeWidth = node.strokeWidth ?? 1.5;
  const profile =
    live && node.activity && !node.dashed
      ? resolveFlowVisualProfile(node.flowBytes, node.activity)
      : undefined;
  const flowWidth = profile ? Math.max(strokeWidth + 0.25, profile.flowWidth) : strokeWidth;
  const duration = flowDurationForBytes(node.flowBytes);
  const glow =
    profile && profile.glowExtraWidth > 0
      ? `<path d="${d}" fill="none" stroke="${colour}" stroke-width="${n(flowWidth + profile.glowExtraWidth)}" stroke-opacity="${n(profile.glowOpacity)}" stroke-linecap="round" stroke-linejoin="round" class="pulse-map-flow-glow"/>`
      : '';
  const pathWidth =
    profile && node.bidirectional
      ? Math.max(strokeWidth, Math.min(flowWidth, strokeWidth + 1.5))
      : strokeWidth;
  const path = `<path d="${d}" fill="none" stroke="${colour}" stroke-width="${n(pathWidth)}" stroke-linejoin="round"${
    node.dashed ? ' stroke-dasharray="5 4"' : ''
  }/>`;
  const flow =
    profile && node.activity && !node.bidirectional
      ? `<path d="${d}" fill="none" stroke="${colour}" stroke-width="${n(flowWidth)}" stroke-opacity="${n(profile.flowOpacity)}" stroke-dasharray="${profile.dashArray}" stroke-linecap="round" stroke-linejoin="round" class="pulse-map-flow pulse-map-flow-${node.activity} pulse-map-flow-${profile.intensity}" style="animation-duration:${n(duration ?? profile.dashDuration)}s"/>`
      : '';
  const packets =
    profile && !node.bidirectional
      ? renderTravelingPackets(d, polylineLength(node.points), colour, palette.surface, profile)
      : '';
  if (!node.arrow) return glow + path + flow + packets;
  const [tipX, tipY] = node.points[node.points.length - 1];
  const [fromX, fromY] = node.points[node.points.length - 2];
  const length = Math.hypot(tipX - fromX, tipY - fromY) || 1;
  const ux = (tipX - fromX) / length;
  const uy = (tipY - fromY) / length;
  const baseX = tipX - ux * ARROW_LENGTH;
  const baseY = tipY - uy * ARROW_LENGTH;
  const head = `<path d="M${n(tipX)} ${n(tipY)}L${n(baseX - uy * ARROW_HALF_WIDTH)} ${n(baseY + ux * ARROW_HALF_WIDTH)}L${n(baseX + uy * ARROW_HALF_WIDTH)} ${n(baseY - ux * ARROW_HALF_WIDTH)}z" fill="${colour}"/>`;
  return glow + path + flow + packets + head;
};

const nodeToSvg = (node: SceneNode, palette: MapPalette, live: boolean): string => {
  const attr = tileAttr(node, live);
  switch (node.kind) {
    case 'rect':
      if (node.activity) {
        if (!live) {
          if (node.fill === 'none' && !node.stroke) return '';
          return `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette[node.fill]}"${
            node.stroke ? ` stroke="${palette[node.stroke]}"` : ''
          }${node.dashed ? ' stroke-dasharray="4 3"' : ''}/>`;
        }
        const fillOpacity = node.spike ? '0.22' : ACTIVITY_FILL_OPACITY[node.activity];
        const strokeOpacity = node.spike ? '0.96' : ACTIVITY_STROKE_OPACITY[node.activity];
        const strokeW = node.spike ? '2' : '1.25';
        const spikeClass = node.spike ? ' pulse-map-spike' : '';
        const halo = node.spike
          ? `<rect x="${n(node.x - 1.5)}" y="${n(node.y - 1.5)}" width="${n(node.w + 3)}" height="${n(node.h + 3)}" rx="${n(node.r + 1.5)}" fill="none" stroke="${palette.accent}" stroke-opacity="0.34" stroke-width="3" class="pulse-map-spike-halo"/>`
          : '';
        return `${halo}<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette.accent}" fill-opacity="${fillOpacity}" stroke="${palette.accent}" stroke-opacity="${strokeOpacity}" stroke-width="${strokeW}" class="pulse-map-active pulse-map-active-${node.activity}${spikeClass}"${attr}/>`;
      }
      return `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette[node.fill]}"${
        node.opacity !== undefined ? ` fill-opacity="${n(node.opacity)}"` : ''
      }${node.stroke ? ` stroke="${palette[node.stroke]}"` : ''}${
        node.dashed ? ' stroke-dasharray="4 3"' : ''
      }${attr}/>`;
    case 'text':
      return `<text x="${n(node.x)}" y="${n(node.y)}" font-size="${n(node.size)}" font-weight="${node.weight}" fill="${palette[node.fill]}"${
        node.anchor === 'end'
          ? ' text-anchor="end"'
          : node.anchor === 'middle'
            ? ' text-anchor="middle"'
            : ''
      }${node.caps ? ' letter-spacing="0.6"' : ''}${node.dim ? ' opacity="0.6"' : ''}${attr}>${escapeXml(node.text)}</text>`;
    case 'app':
      return appToSvg(node, palette, live);
    case 'line':
      return lineToSvg(node, palette, live);
    case 'dot':
      return node.health === 'off'
        ? `<circle cx="${n(node.x)}" cy="${n(node.y)}" r="${n(node.r - 0.75)}" fill="none" stroke="${palette.off}" stroke-width="1.5"${attr}/>`
        : `<circle cx="${n(node.x)}" cy="${n(node.y)}" r="${n(node.r)}" fill="${palette[node.health]}"${attr}/>`;
    case 'icon': {
      const glyph = MAP_GLYPHS[node.platform];
      const paint =
        glyph.mode === 'fill'
          ? `fill="${palette.muted}"`
          : `fill="none" stroke="${palette.muted}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"`;
      return `<path transform="translate(${n(node.x)} ${n(node.y)}) scale(${n(node.size / 24)})" d="${glyph.d}" ${paint}${attr}/>`;
    }
    case 'inspect':
    case 'hit':
      return '';
  }
};

export const MAP_HIT_ATTRIBUTE = 'data-map-hit';
/** Clickable group header or network pill target that opens the map inspector drawer. */
export const MAP_INSPECT_ATTRIBUTE = 'data-map-inspect';
/** The layer the page fills with rings while a tile is pointed at or focused. */
export const MAP_RINGS_ATTRIBUTE = 'data-map-rings';

// The hover and focus treatment of everything on the map that answers the
// pointer. Kept as one literal so the stylesheet build can see the classes.
// A tile that is a link takes the focus on the link, so its rect is styled
// through the link (`group`). The ring keeps its width however small the map
// is drawn. Without it Safari shows a keyboard user no focus at all. There is
// no stroke at rest: even a transparent one takes the pointer, a pixel past
// the tile's edge, and neighbours are little more than a pixel apart.
const HIT_BASE_CLASS =
  'fill-transparent stroke-2 [vector-effect:non-scaling-stroke] outline-none transition-colors hover:fill-slate-400/15 focus-visible:fill-slate-400/15 focus-visible:stroke-blue-500 group-focus-visible:fill-slate-400/15 group-focus-visible:stroke-blue-500';
const HIT_CLASS = `cursor-pointer ${HIT_BASE_CLASS}`;
const HIT_STATIC_CLASS = `cursor-default ${HIT_BASE_CLASS}`;
const INSPECT_CLASS =
  'cursor-pointer fill-transparent outline-none transition-colors hover:fill-slate-400/12';
const LINK_CLASS = 'group outline-none';

/**
 * The inside of the on-page map: the drawn marks, then one target per
 * resource on top. A target the page can open is a link to that resource's
 * own page; overflow tiles, group headers and network pills open the
 * inspector drawer; the rest still answer hover and focus with their details.
 */
export function renderMapSceneBody(
  scene: MapScene,
  palette: MapPalette,
  hrefFor: (hitIndex: number) => string | null,
): string {
  let hitIndex = 0;
  let inspectIndex = 0;
  return scene.nodes
    .map((node) => {
      if (node.kind === 'inspect') {
        const index = inspectIndex;
        inspectIndex += 1;
        const label = escapeXml(node.label);
        return `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" pointer-events="all" class="${INSPECT_CLASS}" ${MAP_INSPECT_ATTRIBUTE}="${index}" role="button" aria-label="${label}"><title>${label}</title></rect>`;
      }
      if (node.kind !== 'hit') return nodeToSvg(node, palette, true);
      const index = hitIndex;
      hitIndex += 1;
      const href = node.hit.navigable || node.hit.checkAddress ? hrefFor(index) : null;
      // Said only where the tile does lead to the form that adds one.
      const adds = Boolean(node.hit.checkAddress) && href !== null;
      const label = escapeXml(`${node.hit.name}, ${node.hit.meta}${adds ? ', add a check' : ''}`);
      const interactive = Boolean(href || node.hit.inspectTarget);
      const rect = `x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" pointer-events="all" class="${interactive ? HIT_CLASS : HIT_STATIC_CLASS}"`;
      const hitTileAttr = ` ${MAP_TILE_NODE_ATTRIBUTE}="${escapeXml(node.hit.id)}"`;
      if (href) {
        return `<a href="${escapeXml(href)}" class="${LINK_CLASS}" ${MAP_HIT_ATTRIBUTE}="${index}"${hitTileAttr} aria-label="${label}"><rect ${rect}/></a>`;
      }
      const role = node.hit.inspectTarget ? 'button' : 'img';
      return `<rect ${rect} ${MAP_HIT_ATTRIBUTE}="${index}"${hitTileAttr} tabindex="0" role="${role}" aria-label="${label}"/>`;
    })
    .concat(`<g ${MAP_RINGS_ATTRIBUTE}="" pointer-events="none"></g>`)
    .join('');
}

/**
 * Toggles spotlight dimming on an interactive map SVG without re-rendering markup.
 * Passing `null` or an empty set restores every tile to full brightness.
 */
export function applyMapSpotlight(
  svg: SVGSVGElement | undefined,
  highlightedIds: ReadonlySet<string> | null,
): void {
  if (!svg) return;
  const active = highlightedIds !== null && highlightedIds.size > 0;
  svg.classList.toggle(MAP_SPOTLIGHT_CLASS, active);
  const tagged = svg.querySelectorAll(`[${MAP_TILE_NODE_ATTRIBUTE}]`);
  for (const el of tagged) {
    const id = el.getAttribute(MAP_TILE_NODE_ATTRIBUTE);
    el.classList.toggle(MAP_SPOTLIGHT_LIT_CLASS, Boolean(active && id && highlightedIds.has(id)));
  }
}

/** Maximum number of curved connection arcs drawn when hovering or focusing one tile. */
export const MAP_MAX_LINK_ARCS = 16;

type HitSceneNode = Extract<SceneNode, { kind: 'hit' }>;

/**
 * A ring round every tile that a connection joins to the given one, paired with
 * static curved arcs from the hovered or focused tile to its linked peers
 * (capped at MAP_MAX_LINK_ARCS so busy hosts stay legible). Configured
 * relationships are always drawn as static lines because per-edge throughput is
 * not measured separately from each resource's own counters. Shown only
 * interactively while a tile is pointed at or focused, never in static exports.
 */
export function renderMapLinkRings(scene: MapScene, of: MapHit, palette: MapPalette): string {
  const linked = new Set(of.linked ?? []);
  if (linked.size === 0) return '';
  let source: HitSceneNode | undefined;
  const targets: HitSceneNode[] = [];
  for (const node of scene.nodes) {
    if (node.kind !== 'hit') continue;
    if (node.hit.id === of.id) source = node;
    if (linked.has(node.hit.id)) targets.push(node);
  }
  if (targets.length === 0) return '';

  const halos: string[] = [];
  const arcs: string[] = [];
  const dots: string[] = [];
  const labelPills: string[] = [];
  if (source) {
    const sx = source.x + source.w / 2;
    const sy = source.y + source.h / 2;
    const drawnTargets = targets.slice(0, MAP_MAX_LINK_ARCS);
    drawnTargets.forEach((target, index) => {
      const tx = target.x + target.w / 2;
      const ty = target.y + target.h / 2;
      const dx = tx - sx;
      const dy = ty - sy;
      const dist = Math.hypot(dx, dy);
      if (dist < 1) return;
      const mx = (sx + tx) / 2;
      const my = (sy + ty) / 2;
      let nx = -dy / dist;
      let ny = dx / dist;
      if (Math.abs(dy) < 20) {
        if (ny > 0) {
          nx = -nx;
          ny = -ny;
        }
      } else if (dx < 0 ? nx > 0 : dx > 0 ? nx < 0 : index % 2 === 1) {
        nx = -nx;
        ny = -ny;
      }
      const bend = Math.min(48, Math.max(18, dist * 0.16));
      const cx = mx + nx * bend;
      const cy = my + ny * bend;
      const d = `M${n(sx)} ${n(sy)} Q${n(cx)} ${n(cy)} ${n(tx)} ${n(ty)}`;
      halos.push(
        `<path d="${d}" fill="none" stroke="${palette.surface}" stroke-width="4" stroke-opacity="0.72" class="pulse-map-link-halo"/>`,
      );
      arcs.push(
        `<path d="${d}" fill="none" stroke="${palette.accent}" stroke-width="1.5" stroke-opacity="0.65" class="pulse-map-link-arc"/>`,
      );
      dots.push(
        `<circle cx="${n(tx)}" cy="${n(ty)}" r="2.5" fill="${palette.accent}" fill-opacity="0.85"/>`,
      );
      const rawLabel = of.linkLabels?.[target.hit.id]?.trim();
      if (drawnTargets.length <= 6 && rawLabel) {
        const text = rawLabel.length > 24 ? `${rawLabel.slice(0, 23).trimEnd()}…` : rawLabel;
        const pillW = Math.max(36, text.length * 5.6 + 12);
        const pillH = 15;
        const bx = 0.25 * sx + 0.5 * cx + 0.25 * tx;
        const by = 0.25 * sy + 0.5 * cy + 0.25 * ty;
        labelPills.push(
          `<g class="pulse-map-link-label">` +
            `<rect x="${n(bx - pillW / 2)}" y="${n(by - pillH / 2)}" width="${n(pillW)}" height="${pillH}" rx="4" fill="${palette.surface}" stroke="${palette.accent}" stroke-opacity="0.55" stroke-width="1"/>` +
            `<text x="${n(bx)}" y="${n(by + 3.2)}" font-size="9.5" font-weight="500" text-anchor="middle" fill="${palette.text}">${escapeXml(text)}</text>` +
            `</g>`,
        );
      }
    });
    if (dots.length > 0) {
      dots.unshift(
        `<circle cx="${n(sx)}" cy="${n(sy)}" r="2.5" fill="${palette.accent}" fill-opacity="0.85"/>`,
      );
    }
  }

  const rings = targets
    .map(
      (node) =>
        `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette.accent}" fill-opacity="0.06" stroke="${palette.accent}" stroke-width="1.5"/>`,
    )
    .join('');

  return `${halos.join('')}${arcs.join('')}${dots.join('')}${rings}${labelPills.join('')}`;
}

/**
 * A ring round every tile attached to the given planned network. Shown only
 * interactively while a network pill is pointed at or focused, never in static
 * exports.
 */
export function renderMapNetworkRings(scene: MapScene, networkId: string, color: string): string {
  if (!networkId) return '';
  const paint = escapeXml(color);
  return scene.nodes
    .filter(
      (node): node is HitSceneNode =>
        node.kind === 'hit' && Boolean(node.hit.segmentIds?.includes(networkId)),
    )
    .map(
      (node) =>
        `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${paint}" fill-opacity="0.08" stroke="${paint}" stroke-width="2" class="pulse-map-network-ring"/>`,
    )
    .join('');
}

/**
 * The tile a tap chose, shaded and outlined. A pointer shows which tile its
 * card is about by resting on it. A finger has gone by the time the card is
 * read, so the tile is marked instead.
 */
export function renderMapTappedTile(scene: MapScene, of: MapHit, palette: MapPalette): string {
  const node = scene.nodes.find(
    (candidate) => candidate.kind === 'hit' && candidate.hit.id === of.id,
  );
  return node?.kind === 'hit'
    ? `<rect x="${n(node.x)}" y="${n(node.y)}" width="${n(node.w)}" height="${n(node.h)}" rx="${n(node.r)}" fill="${palette.muted}" fill-opacity="0.2" stroke="${palette.text}" stroke-width="1.5"/>`
    : '';
}

/** A complete, self-contained SVG document for a scene. */
export function renderMapSceneToSvg(scene: MapScene, palette: MapPalette): string {
  const body = scene.nodes.map((node) => nodeToSvg(node, palette, false)).join('');
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${scene.width}" height="${scene.height}" ` +
    `viewBox="0 0 ${scene.width} ${scene.height}" font-family="${escapeXml(MAP_FONT_FAMILY)}">` +
    `<rect width="${scene.width}" height="${scene.height}" fill="${palette.surface}"/>${body}</svg>`
  );
}
