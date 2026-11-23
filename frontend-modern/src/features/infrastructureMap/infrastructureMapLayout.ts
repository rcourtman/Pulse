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
export type MapGlyphKey = MapPlatform | 'internet' | 'gateway' | 'switch' | 'access-point';

export type MapInspectTarget =
  | { kind: 'group'; groupKey: string; initialFilter?: 'all' | 'attention' | 'off' }
  | { kind: 'network'; networkId: string }
  | { kind: 'all-networks' };

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
  /** Set on a thing nothing monitors: its tile opens a new check for this address. */
  checkAddress?: string;
  /** Set on overflow or summary tiles that open the interactive inspector drawer. */
  inspectTarget?: MapInspectTarget;
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
  | {
      /** Clickable header or network pill target that opens the inspector drawer. */
      kind: 'inspect';
      x: number;
      y: number;
      w: number;
      h: number;
      r: number;
      label: string;
      target: MapInspectTarget;
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
  /**
   * How wide a named tile is, when wider than the usual. The page uses it
   * where a screen leaves the map width to spare, so that a name which one
   * line of the usual tile would cut is written whole.
   */
  nameWidth?: number;
}

export interface MapCursor {
  nodes: SceneNode[];
  hits: SceneNode[];
  inspects?: SceneNode[];
}

/** Estates up to this many guests and containers read best with every name drawn. */
export const MAP_NAMES_ITEM_LIMIT = 220;

/** The keys that move a keyboard user from one tile to another. */
export type MapMoveKey = 'ArrowUp' | 'ArrowDown' | 'ArrowLeft' | 'ArrowRight' | 'Home' | 'End';
const MAP_MOVE_KEYS: ReadonlySet<string> = new Set([
  'ArrowUp',
  'ArrowDown',
  'ArrowLeft',
  'ArrowRight',
  'Home',
  'End',
]);
export const isMapMoveKey = (key: string): key is MapMoveKey => MAP_MOVE_KEYS.has(key);

/**
 * The tile a key moves to from tile `from`, both counted among the scene's
 * tiles in the order they are drawn, or null when the key moves nowhere.
 *
 * An arrow goes to the nearest tile in line that way on the drawing: along
 * the row for left and right, up or down the column for the other two. Up
 * and down, when no tile is in line (under the gap a short last row
 * leaves), it is the nearest one off to the side. Left and right, when the
 * row has no more, it carries on into the row after or before it, as a line
 * of text does, so the end of a row is never a dead end. Home and End are
 * the first and last tile drawn.
 */
export const nextMapHitIndex = (scene: MapScene, from: number, key: MapMoveKey): number | null => {
  const tiles = scene.nodes.filter(
    (node): node is Extract<SceneNode, { kind: 'hit' }> => node.kind === 'hit',
  );
  const at = tiles[from];
  if (!at) return null;
  const another = (index: number): number | null =>
    index !== from && index >= 0 && index < tiles.length ? index : null;
  if (key === 'Home') return another(0);
  if (key === 'End') return another(tiles.length - 1);
  const sideways = key === 'ArrowLeft' || key === 'ArrowRight';
  const onward = key === 'ArrowRight' || key === 'ArrowDown';
  // A tile as two spans: along the way the key travels, and across it.
  const spans = (tile: { x: number; y: number; w: number; h: number }) =>
    sideways
      ? { near: tile.x, far: tile.x + tile.w, low: tile.y, high: tile.y + tile.h }
      : { near: tile.y, far: tile.y + tile.h, low: tile.x, high: tile.x + tile.w };
  const here = spans(at);
  let best: number | null = null;
  let bestInLine = false;
  let bestCost = Infinity;
  tiles.forEach((tile, index) => {
    if (index === from) return;
    const there = spans(tile);
    // How far ahead it starts. A tile level with this one is not ahead.
    const ahead = onward ? there.near - here.far : here.near - there.far;
    if (ahead < 0) return;
    const aside = Math.max(0, there.low - here.high, here.low - there.high);
    const inLine = aside === 0;
    // Sideways, a tile on another row is that row's, reached by going on.
    if (sideways && !inLine) return;
    const offCentre = Math.abs((there.low + there.high - here.low - here.high) / 2);
    const cost = inLine ? ahead + offCentre / 2 : ahead + aside * 2;
    if ((inLine && !bestInLine) || (inLine === bestInLine && cost < bestCost)) {
      best = index;
      bestInLine = inLine;
      bestCost = cost;
    }
  });
  if (best !== null) return best;
  return sideways ? another(onward ? from + 1 : from - 1) : null;
};

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
 * none of those is broken where the line runs out, and so is a name whose
 * break comes too early to leave a second line that fits (`VM` + a long
 * word). What will not go on two lines ends in an ellipsis.
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
  // An emoji is two code units, and a line must not end between them.
  const splitsCharacter = (index: number): boolean =>
    /[\uD800-\uDBFF]/.test(text[index - 1]) && /[\uDC00-\uDFFF]/.test(text[index]);
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
      if (splitsCharacter(index)) continue;
      if (textWidth(text.slice(0, index), size, weight) > maxWidth) break;
      cut = index;
    }
    cut = Math.max(1, Math.min(cut, text.length - 3));
    if (splitsCharacter(cut)) cut -= 1;
  }
  if (cut === 0) return [fitMapText(text, maxWidth, size, weight)];
  if (textWidth(text.slice(cut).trimStart(), size, weight) > maxWidth) {
    // The break left more than a line. Filling the first line instead shows
    // the whole name when that is all it takes, by breaking inside a word
    // that is long enough to stand it: four letters or more each side, so
    // `observabi` + `lity` but never `Swa` + `rm` or `ing` + `est`.
    const wordEnds = (index: number): number => {
      let end = index;
      while (
        end < text.length &&
        !/[\s\-_.]/.test(text[end]) &&
        (end === index || !breaksBefore(end))
      ) {
        end += 1;
      }
      return end;
    };
    const wordStarts = (index: number): number => {
      let begin = index;
      while (begin > 0 && !breaksBefore(begin)) begin -= 1;
      return begin;
    };
    let filled = cut;
    for (let index = cut + 1; index < text.length; index += 1) {
      if (splitsCharacter(index)) continue;
      if (textWidth(text.slice(0, index).trimEnd(), size, weight) > maxWidth) break;
      // A break just before a hyphen or a dot is between words, not in one.
      const insideWord = !breaksBefore(index) && !/[\s\-_.]/.test(text[index]);
      if (insideWord && (index - wordStarts(index) < 4 || wordEnds(index) - index < 4)) {
        continue;
      }
      filled = index;
    }
    if (textWidth(text.slice(filled).trimStart(), size, weight) <= maxWidth) cut = filled;
  }
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
