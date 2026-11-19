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
  /**
   * Lets a name take a second line. A saved image has no hover to read a
   * cut-off name from; the page does, and has to fit one screen.
   */
  wrapNames?: boolean;
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

/**
 * A name cut to fit one line that keeps a number it ends in. With names
 * hidden a tile reads `Container 121`, and the number is all that tells it
 * from the next one, so the word gives way and the number stays.
 */
export const fitMapName = (
  text: string,
  maxWidth: number,
  size: number,
  weight: 400 | 500 | 600,
): string => {
  const fitted = fitMapText(text, maxWidth, size, weight);
  if (fitted === text) return text;
  const number = /\s\d+$/.exec(text)?.[0];
  if (!number) return fitted;
  const word = fitMapText(
    text.slice(0, -number.length),
    maxWidth - textWidth(number, size, weight),
    size,
    weight,
  );
  return word ? `${word}${number}` : fitted;
};

/**
 * A name on one line when it fits and on two when it does not, broken after
 * a space, hyphen, underscore or dot, or where a capital starts a new word
 * inside it (`iSponsorBlock` + `TV`), whichever falls in reach. A word with
 * none of those is broken where the line runs out. What will not go on two
 * lines ends in an ellipsis.
 */
export const wrapMapText = (
  text: string,
  maxWidth: number,
  size: number,
  weight: 400 | 500 | 600,
): string[] => {
  if (maxWidth <= 0) return [''];
  if (textWidth(text, size, weight) <= maxWidth) return [text];
  const breaksBefore = (index: number): boolean =>
    /[\s\-_.]/.test(text[index - 1]) ||
    (/[a-z0-9]/.test(text[index - 1]) && /[A-Z]/.test(text[index]));
  // The longest opening that fits and ends at a break.
  let cut = 0;
  for (let index = 1; index < text.length; index += 1) {
    if (!breaksBefore(index)) continue;
    if (textWidth(text.slice(0, index).trimEnd(), size, weight) > maxWidth) break;
    cut = index;
  }
  if (cut === 0) {
    // One long word: as much of it as fits on the first line, leaving more
    // than a letter or two for the second.
    for (let index = 1; index < text.length; index += 1) {
      if (textWidth(text.slice(0, index), size, weight) > maxWidth) break;
      cut = index;
    }
    cut = Math.max(1, Math.min(cut, text.length - 3));
  }
  if (cut === 0) return [fitMapText(text, maxWidth, size, weight)];
  return [
    text.slice(0, cut).trimEnd(),
    fitMapText(text.slice(cut).trimStart(), maxWidth, size, weight),
  ];
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

// The states a status dot can show, in the order the page's legend lists them.
const POSTER_LEGEND: readonly MapHealth[] = ['ok', 'attention', 'critical', 'stale', 'off'];
const POSTER_LEGEND_GAP = 18;

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
  const baseline = bottom + 30;
  out.nodes.push({
    kind: 'text',
    x: width - margin,
    y: baseline,
    text: header.footer,
    size: 11,
    weight: 400,
    fill: 'muted',
    anchor: 'end',
    dim: true,
  });
  // The page explains its dots in a legend under the map. A saved image is
  // seen without the page, so it carries the same key.
  const footerLeft = width - margin - textWidth(header.footer, 11, 400);
  const legendWidth = POSTER_LEGEND.reduce(
    (total, health) => total + 12 + textWidth(HEALTH_WORD[health], 11, 400) + POSTER_LEGEND_GAP,
    -POSTER_LEGEND_GAP,
  );
  if (margin + legendWidth + POSTER_LEGEND_GAP <= footerLeft) {
    let x = margin;
    for (const health of POSTER_LEGEND) {
      out.nodes.push({ kind: 'dot', x: x + 4, y: baseline - 4, r: 4, health });
      out.nodes.push({
        kind: 'text',
        x: x + 12,
        y: baseline,
        text: HEALTH_WORD[health],
        size: 11,
        weight: 400,
        fill: 'muted',
      });
      x += 12 + textWidth(HEALTH_WORD[health], 11, 400) + POSTER_LEGEND_GAP;
    }
  }
  return baseline + margin - 8;
};
