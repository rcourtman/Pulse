import { estimateTextWidth } from '@/utils/format';
import type { InfrastructureMapModel, MapHealth, MapPlatform } from './infrastructureMapModel';

/**
 * What a laid-out map is made of: a flat list of marks at fixed positions.
 * The page draws the list as SVG and the export writes the same list to a
 * file, so the image someone shares is the map they were looking at.
 *
 * A layout is a function of the model and a width, with no pan, zoom or saved
 * positions: the map flows into the space it is given and grows downward like
 * any other page.
 */

export type MapNetworkTone = 'net0' | 'net1' | 'net2' | 'net3' | 'net4' | 'net5' | 'netOther';

/**
 * The colours a mark can ask for. Each is one of the product's semantic
 * design tokens (`surface`, `border`, `muted` and so on), so the map takes
 * its colours from the theme like every other page.
 */
export type MapTone =
  'base' | 'surface' | 'surfaceAlt' | 'border' | 'text' | 'muted' | 'accent' | MapNetworkTone;

export type MapDensity = 'names' | 'marks';

/** Everything the map has an outline glyph for: each platform, the Internet and a gateway. */
export type MapGlyphKey = MapPlatform | 'internet' | 'gateway';

export interface MapHit {
  id: string;
  name: string;
  meta: string;
  health: MapHealth;
  cpu?: number;
  memory?: number;
  address?: string;
  detail?: string;
  /** What the thing's connections say about it, one sentence each. */
  links?: string[];
  /** The tiles those connections join it to, by id. Only ones that are drawn. */
  linked?: string[];
  /** A tile that stands for several things is not a resource and opens nothing. */
  navigable: boolean;
}

export type SceneNode =
  | {
      kind: 'rect';
      x: number;
      y: number;
      w: number;
      h: number;
      r: number;
      fill: MapTone | MapHealth | 'none';
      stroke?: MapTone | MapHealth;
      dashed?: boolean;
    }
  | {
      kind: 'text';
      x: number;
      y: number;
      text: string;
      size: number;
      weight: 400 | 500 | 600;
      fill: MapTone;
      anchor?: 'start' | 'middle' | 'end';
      /** Uppercase section labels are tracked out, as table headers are. */
      caps?: boolean;
      dim?: boolean;
    }
  | { kind: 'dot'; x: number; y: number; r: number; health: MapHealth }
  | { kind: 'icon'; x: number; y: number; size: number; platform: MapGlyphKey }
  | {
      /** An app's logo on its brand colour, or its initials when it has none. */
      kind: 'app';
      x: number;
      y: number;
      size: number;
      icon?: string;
      letters: string;
      dim?: boolean;
    }
  | {
      /** A connector through the given corners, with an arrowhead at the end. */
      kind: 'line';
      points: Array<[number, number]>;
      tone: MapTone;
      dashed?: boolean;
      arrow?: boolean;
    }
  | { kind: 'hit'; x: number; y: number; w: number; h: number; r: number; hit: MapHit };

export interface MapScene {
  width: number;
  height: number;
  nodes: SceneNode[];
}

export interface MapLayoutOptions {
  width: number;
  density: MapDensity;
  /** Drawn above the map in an exported image; the page has its own toolbar. */
  header?: { title: string; facts: string[]; footer: string };
}

export interface MapCursor {
  nodes: SceneNode[];
  hits: SceneNode[];
}

/** Estates up to this many guests and containers read best with every name drawn. */
export const MAP_NAMES_ITEM_LIMIT = 220;

export const resolveDefaultMapDensity = (model: InfrastructureMapModel): MapDensity => {
  let named = 0;
  for (const block of model.blocks) {
    const hosts = block.kind === 'group' ? block.group.hosts : [block.host];
    for (const host of hosts) {
      for (const section of host.sections) named += section.items.length;
    }
  }
  return named <= MAP_NAMES_ITEM_LIMIT ? 'names' : 'marks';
};

const textWidth = (text: string, size: number, weight: 400 | 500 | 600): number =>
  weight === 400
    ? estimateTextWidth('', { detail: text, fontPx: size })
    : estimateTextWidth(text, { fontPx: size });

export const fitMapText = (
  text: string,
  maxWidth: number,
  size: number,
  weight: 400 | 500 | 600,
): string => {
  if (maxWidth <= 0) return '';
  if (textWidth(text, size, weight) <= maxWidth) return text;
  let low = 0;
  let high = text.length;
  while (low < high) {
    const mid = Math.ceil((low + high) / 2);
    if (textWidth(`${text.slice(0, mid).trimEnd()}…`, size, weight) <= maxWidth) low = mid;
    else high = mid - 1;
  }
  return low > 0 ? `${text.slice(0, low).trimEnd()}…` : '';
};

const HEALTH_WORD: Record<MapHealth, string> = {
  ok: 'Healthy',
  attention: 'Needs attention',
  critical: 'Critical',
  off: 'Off',
  stale: 'Not reporting',
  unknown: 'Unknown',
};

const titleCase = (value: string): string =>
  value ? value.charAt(0).toUpperCase() + value.slice(1) : value;

/** What a tile says about itself on hover: what it is, then how it is doing. */
export const mapHitMeta = (typeLabel: string, status: string, health: MapHealth): string => {
  const state = health === 'ok' || health === 'off' ? titleCase(status) : HEALTH_WORD[health];
  return [typeLabel, state || HEALTH_WORD[health]].filter(Boolean).join(' · ');
};

type PosterHeader = MapLayoutOptions['header'];

export const drawPosterHeader = (
  out: MapCursor,
  header: PosterHeader,
  margin: number,
  contentWidth: number,
): number => {
  if (!header) return margin;
  out.nodes.push({
    kind: 'text',
    x: margin,
    y: margin + 22,
    text: fitMapText(header.title, contentWidth, 24, 600),
    size: 24,
    weight: 600,
    fill: 'text',
  });
  out.nodes.push({
    kind: 'text',
    x: margin,
    y: margin + 44,
    text: fitMapText(header.facts.join('   ·   '), contentWidth, 12, 400),
    size: 12,
    weight: 400,
    fill: 'muted',
  });
  return margin + 64;
};

export const drawPosterFooter = (
  out: MapCursor,
  header: PosterHeader,
  margin: number,
  width: number,
  bottom: number,
): number => {
  if (!header) return bottom;
  out.nodes.push({
    kind: 'text',
    x: width - margin,
    y: bottom + 30,
    text: header.footer,
    size: 11,
    weight: 400,
    fill: 'muted',
    anchor: 'end',
    dim: true,
  });
  return bottom + 30 + margin - 8;
};
