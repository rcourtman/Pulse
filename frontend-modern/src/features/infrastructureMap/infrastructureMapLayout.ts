import { estimateTextWidth } from '@/utils/format';
import type { MapSegment } from './infrastructureMapNetworks';
import type {
  InfrastructureMapModel,
  MapBlock,
  MapGroup,
  MapHealth,
  MapHost,
  MapItem,
  MapPlatform,
  MapPool,
  MapSection,
} from './infrastructureMapModel';

/**
 * Turns the map model into a flat list of marks at fixed positions. The page
 * draws the list as SVG and the export writes the same list to a file, so
 * the image someone shares is the map they were looking at.
 *
 * The layout is a function of the model and a width, with no pan, zoom or
 * saved positions: cards flow into as many columns as fit and the map grows
 * downward like any other page.
 */

export type MapNetworkTone = 'net0' | 'net1' | 'net2' | 'net3' | 'net4' | 'net5' | 'netOther';

export type MapTone =
  | 'canvas'
  | 'frame'
  | 'card'
  | 'border'
  | 'track'
  | 'text'
  | 'muted'
  | 'faint'
  | 'accent'
  | MapNetworkTone;

/** How a card is drawn. `networkTone` turns on the per-row network colours. */
export interface MapDrawStyle {
  density: MapDensity;
  networkTone?: (segmentId: string) => MapNetworkTone;
}

export type MapDensity = 'names' | 'marks';

/** `networks` draws what is connected to what; `hosts` what runs where. */
export type MapView = 'networks' | 'hosts';

export interface MapHit {
  id: string;
  name: string;
  meta: string;
  health: MapHealth;
  cpu?: number;
  memory?: number;
  address?: string;
  detail?: string;
  /** Composed cards and clusters are not resources and open nothing. */
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
      fill: MapTone | MapHealth;
      stroke?: MapTone | MapHealth;
    }
  | {
      kind: 'text';
      x: number;
      y: number;
      text: string;
      size: number;
      weight: 400 | 600;
      fill: MapTone;
      anchor?: 'start' | 'end';
      caps?: boolean;
    }
  | { kind: 'dot'; x: number; y: number; r: number; health: MapHealth }
  | { kind: 'mark'; x: number; y: number; size: number; health: MapHealth }
  | { kind: 'icon'; x: number; y: number; size: number; platform: MapPlatform }
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

/** Estates up to this many named things read best with every name drawn. */
export const MAP_NAMES_ITEM_LIMIT = 220;

export const resolveDefaultMapDensity = (model: InfrastructureMapModel): MapDensity => {
  let named = 0;
  for (const block of model.blocks) {
    const hosts = block.kind === 'group' ? block.group.hosts : [block.host];
    for (const host of hosts) {
      for (const section of host.sections) if (!section.marksOnly) named += section.items.length;
    }
  }
  return named <= MAP_NAMES_ITEM_LIMIT ? 'names' : 'marks';
};

const GAP = 14;
const MIN_COLUMN = 272;
const MAX_COLUMNS = 6;
const PAD = 12;
const FRAME_PAD = 12;
const FRAME_HEADER = 30;
const HEADER_HEIGHT = 40;
const METRICS_HEIGHT = 24;
const SECTION_GAP = 10;
const SECTION_LABEL = 15;
const ROW_HEIGHT = 17;
const MARK = 9;
const MARK_GAP = 3;
const POOL_ROW = 17;
const MAX_POOLS = 6;

const textWidth = (text: string, size: number, weight: 400 | 600): number =>
  weight === 600
    ? estimateTextWidth(text, { fontPx: size })
    : estimateTextWidth('', { detail: text, fontPx: size });

export const fitMapText = (
  text: string,
  maxWidth: number,
  size: number,
  weight: 400 | 600,
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

const hitMeta = (typeLabel: string, status: string, health: MapHealth): string => {
  const state = health === 'ok' || health === 'off' ? titleCase(status) : HEALTH_WORD[health];
  return [typeLabel, state || HEALTH_WORD[health]].filter(Boolean).join(' · ');
};

const itemHit = (item: MapItem): MapHit => ({
  id: item.id,
  name: item.name,
  meta: hitMeta(item.typeLabel, item.status, item.health),
  health: item.health,
  cpu: item.cpu,
  memory: item.memory,
  address: item.address,
  detail: item.detail,
  navigable: true,
});

const usageHealth = (percent: number): MapHealth =>
  percent >= 90 ? 'critical' : percent >= 80 ? 'attention' : 'ok';

interface Cursor {
  nodes: SceneNode[];
  hits: SceneNode[];
}

const drawBar = (
  out: Cursor,
  x: number,
  y: number,
  w: number,
  h: number,
  percent: number | undefined,
) => {
  out.nodes.push({ kind: 'rect', x, y, w, h, r: h / 2, fill: 'track' });
  if (percent === undefined) return;
  const filled = Math.max(h, (w * percent) / 100);
  out.nodes.push({ kind: 'rect', x, y, w: filled, h, r: h / 2, fill: usageHealth(percent) });
};

const drawPools = (out: Cursor, pools: MapPool[], x: number, y: number, w: number): number => {
  if (pools.length === 0) return y;
  const shown = pools.slice(0, MAX_POOLS);
  const nameWidth = Math.floor(w * 0.44);
  for (const pool of shown) {
    out.nodes.push({ kind: 'dot', x: x + 3, y: y + POOL_ROW / 2, r: 3, health: pool.health });
    out.nodes.push({
      kind: 'text',
      x: x + 11,
      y: y + 12,
      text: fitMapText(pool.name, nameWidth - 11, 11, 400),
      size: 11,
      weight: 400,
      fill: 'text',
    });
    const percentLabel = pool.usedPercent === undefined ? '' : `${Math.round(pool.usedPercent)}%`;
    out.nodes.push({
      kind: 'text',
      x: x + w,
      y: y + 12,
      text: percentLabel,
      size: 10,
      weight: 400,
      fill: 'muted',
      anchor: 'end',
    });
    drawBar(out, x + nameWidth + 6, y + 6, w - nameWidth - 6 - 34, 5, pool.usedPercent);
    out.hits.push({
      kind: 'hit',
      x: x - 3,
      y,
      w: w + 6,
      h: POOL_ROW,
      r: 4,
      hit: {
        id: pool.id,
        name: pool.name,
        meta: ['Storage', pool.sizeLabel].filter(Boolean).join(' · '),
        health: pool.health,
        navigable: true,
      },
    });
    y += POOL_ROW;
  }
  if (pools.length > shown.length) {
    out.nodes.push({
      kind: 'text',
      x: x + 11,
      y: y + 12,
      text: `+${pools.length - shown.length} more`,
      size: 10,
      weight: 400,
      fill: 'muted',
    });
    y += POOL_ROW;
  }
  return y;
};

const drawSectionLabel = (
  out: Cursor,
  label: string,
  count: number,
  x: number,
  y: number,
  note?: { text: string; right: number },
) => {
  if (note) {
    out.nodes.push({
      kind: 'text',
      x: note.right,
      y: y + 10,
      text: note.text,
      size: 9.5,
      weight: 400,
      fill: 'faint',
      anchor: 'end',
    });
  }
  const text = label.toUpperCase();
  out.nodes.push({
    kind: 'text',
    x,
    y: y + 10,
    text,
    size: 9.5,
    weight: 600,
    fill: 'muted',
    caps: true,
  });
  out.nodes.push({
    kind: 'text',
    x: x + textWidth(text, 9.5, 600) + text.length * 0.6 + 6,
    y: y + 10,
    text: String(count),
    size: 9.5,
    weight: 400,
    fill: 'faint',
  });
};

const drawSection = (
  out: Cursor,
  section: MapSection,
  style: MapDrawStyle,
  x: number,
  y: number,
  w: number,
): number => {
  const noteRoom = section.note ? textWidth(section.note, 9.5, 400) + 12 : 0;
  drawSectionLabel(
    out,
    fitMapText(section.label, w - noteRoom - 24, 9.5, 600),
    section.items.length,
    x,
    y,
    section.note ? { text: section.note, right: x + w } : undefined,
  );
  y += SECTION_LABEL;

  if (section.marksOnly || style.density === 'marks') {
    const perRow = Math.max(1, Math.floor((w + MARK_GAP) / (MARK + MARK_GAP)));
    section.items.forEach((item, index) => {
      const mx = x + (index % perRow) * (MARK + MARK_GAP);
      const my = y + Math.floor(index / perRow) * (MARK + MARK_GAP);
      out.nodes.push({ kind: 'mark', x: mx, y: my, size: MARK, health: item.health });
      out.hits.push({
        kind: 'hit',
        x: mx - MARK_GAP / 2,
        y: my - MARK_GAP / 2,
        w: MARK + MARK_GAP,
        h: MARK + MARK_GAP,
        r: 3,
        hit: itemHit(item),
      });
    });
    return y + Math.ceil(section.items.length / perRow) * (MARK + MARK_GAP) - MARK_GAP;
  }

  // With networks on, each row says where the item is: a bar in its
  // network's colour on the left, and its address or ports on the right.
  const tone = style.networkTone;
  const columns = !tone && w >= 236 ? 2 : 1;
  const cell = (w - (columns - 1) * 8) / columns;
  const rows = Math.ceil(section.items.length / columns);
  section.items.forEach((item, index) => {
    // Down each column, then across, so a sorted list still reads sorted.
    const cx = x + Math.floor(index / rows) * (cell + 8);
    const cy = y + (index % rows) * ROW_HEIGHT;
    let textX = cx + 11;
    let room = cell - 13;
    if (tone) {
      const segments = (item.segmentIds ?? []).slice(0, 2);
      segments.forEach((segmentId, pip) => {
        out.nodes.push({
          kind: 'rect',
          x: cx + pip * 4,
          y: cy + 3,
          w: 3,
          h: ROW_HEIGHT - 6,
          r: 1.5,
          fill: tone(segmentId),
        });
      });
      const dotX = cx + 13;
      out.nodes.push({ kind: 'dot', x: dotX, y: cy + ROW_HEIGHT / 2, r: 3, health: item.health });
      textX = dotX + 8;
      const side = item.detail ?? item.address ?? '';
      if (side) {
        out.nodes.push({
          kind: 'text',
          x: cx + cell,
          y: cy + 12,
          text: side,
          size: 10,
          weight: 400,
          fill: 'muted',
          anchor: 'end',
        });
      }
      room = cell - (textX - cx) - (side ? textWidth(side, 10, 400) + 8 : 0);
    } else {
      out.nodes.push({ kind: 'dot', x: cx + 3, y: cy + ROW_HEIGHT / 2, r: 3, health: item.health });
    }
    out.nodes.push({
      kind: 'text',
      x: textX,
      y: cy + 12,
      text: fitMapText(item.name, room, 11, 400),
      size: 11,
      weight: 400,
      fill: item.health === 'off' ? 'muted' : 'text',
    });
    out.hits.push({
      kind: 'hit',
      x: cx - 3,
      y: cy,
      w: cell + 4,
      h: ROW_HEIGHT,
      r: 4,
      hit: itemHit(item),
    });
  });
  return y + rows * ROW_HEIGHT;
};

const drawHost = (
  out: Cursor,
  host: MapHost,
  style: MapDrawStyle,
  x: number,
  y: number,
  w: number,
): number => {
  const top = y;
  const cardIndex = out.nodes.length;
  out.nodes.push({ kind: 'rect', x, y, w, h: 0, r: 8, fill: 'card', stroke: 'border' });

  const innerX = x + PAD;
  const innerW = w - PAD * 2;
  out.nodes.push({ kind: 'icon', x: innerX, y: y + 11, size: 16, platform: host.platform });
  if (host.isResource) {
    out.nodes.push({ kind: 'dot', x: x + w - PAD - 4, y: y + 19, r: 4, health: host.health });
  }
  out.nodes.push({
    kind: 'text',
    x: innerX + 23,
    y: y + 20,
    text: fitMapText(host.name, innerW - 23 - 14, 13, 600),
    size: 13,
    weight: 600,
    fill: 'text',
  });
  out.nodes.push({
    kind: 'text',
    x: innerX + 23,
    y: y + 34,
    text: fitMapText(
      [style.networkTone ? host.address : '', host.subtitle || host.typeLabel]
        .filter(Boolean)
        .join(' · '),
      innerW - 23,
      10.5,
      400,
    ),
    size: 10.5,
    weight: 400,
    fill: 'muted',
  });
  out.hits.push({
    kind: 'hit',
    x: x + 4,
    y: y + 4,
    w: w - 8,
    h: HEADER_HEIGHT - 2,
    r: 6,
    hit: {
      id: host.id,
      name: host.name,
      meta: hitMeta(host.typeLabel, host.status, host.health),
      health: host.health,
      cpu: host.cpu,
      memory: host.memory,
      address: host.address,
      navigable: host.isResource,
    },
  });
  y += HEADER_HEIGHT + 4;

  const metrics: Array<[string, number]> = [];
  if (host.cpu !== undefined) metrics.push(['CPU', host.cpu]);
  if (host.memory !== undefined) metrics.push(['MEM', host.memory]);
  if (host.disk !== undefined) metrics.push(['DISK', host.disk]);
  if (metrics.length > 0) {
    const slot = (innerW - (metrics.length - 1) * 10) / metrics.length;
    metrics.forEach(([label, percent], index) => {
      const mx = innerX + index * (slot + 10);
      out.nodes.push({
        kind: 'text',
        x: mx,
        y: y + 9,
        text: label,
        size: 9,
        weight: 600,
        fill: 'muted',
        caps: true,
      });
      out.nodes.push({
        kind: 'text',
        x: mx + slot,
        y: y + 9,
        text: `${Math.round(percent)}%`,
        size: 9.5,
        weight: 400,
        fill: 'text',
        anchor: 'end',
      });
      drawBar(out, mx, y + 14, slot, 4, percent);
    });
    y += METRICS_HEIGHT;
  }

  const sections =
    style.networkTone && style.density === 'names' && host.networkSections
      ? host.networkSections
      : host.sections;
  for (const section of sections) {
    if (section.items.length === 0) continue;
    y = drawSection(out, section, style, innerX, y + SECTION_GAP, innerW);
  }

  if (host.pools.length > 0) {
    y += SECTION_GAP;
    if (host.platform !== 'storage') {
      drawSectionLabel(
        out,
        host.platform === 'backup' ? 'Datastores' : 'Storage',
        host.pools.length,
        innerX,
        y,
      );
      y += SECTION_LABEL;
    }
    y = drawPools(out, host.pools, innerX, y, innerW);
  }

  y += PAD;
  const card = out.nodes[cardIndex];
  if (card.kind === 'rect') card.h = y - top;
  return y;
};

/** Places blocks into the shortest run of columns that holds them. */
const placeOnSkyline = (heights: number[], span: number): { column: number; y: number } => {
  let best = { column: 0, y: Number.POSITIVE_INFINITY };
  for (let column = 0; column + span <= heights.length; column += 1) {
    const y = Math.max(...heights.slice(column, column + span));
    if (y < best.y) best = { column, y };
  }
  return best;
};

const drawHostsInColumns = (
  out: Cursor,
  hosts: MapHost[],
  style: MapDrawStyle,
  x: number,
  y: number,
  w: number,
  columns: number,
): number => {
  const columnWidth = (w - (columns - 1) * GAP) / columns;
  const heights = new Array<number>(columns).fill(y);
  for (const host of hosts) {
    const { column, y: top } = placeOnSkyline(heights, 1);
    heights[column] =
      drawHost(out, host, style, x + column * (columnWidth + GAP), top, columnWidth) + GAP;
  }
  return Math.max(...heights) - GAP;
};

const drawGroup = (
  out: Cursor,
  group: MapGroup,
  style: MapDrawStyle,
  x: number,
  y: number,
  w: number,
  columns: number,
): number => {
  const top = y;
  const frameIndex = out.nodes.length;
  out.nodes.push({ kind: 'rect', x, y, w, h: 0, r: 10, fill: 'frame', stroke: 'border' });
  out.nodes.push({ kind: 'icon', x: x + FRAME_PAD, y: y + 10, size: 14, platform: group.platform });
  const typeText = group.typeLabel.toUpperCase();
  const countText = `${group.hosts.length} ${group.hosts.length === 1 ? 'host' : 'hosts'}`;
  out.nodes.push({
    kind: 'text',
    x: x + w - FRAME_PAD,
    y: y + 21,
    text: countText,
    size: 10.5,
    weight: 400,
    fill: 'muted',
    anchor: 'end',
  });
  const nameX = x + FRAME_PAD + 21;
  const nameRoom = w - FRAME_PAD * 2 - 21 - textWidth(countText, 10.5, 400) - 10;
  const name = fitMapText(group.name, nameRoom * 0.6, 13, 600);
  out.nodes.push({
    kind: 'text',
    x: nameX,
    y: y + 21,
    text: name,
    size: 13,
    weight: 600,
    fill: 'text',
  });
  out.nodes.push({
    kind: 'text',
    x: nameX + textWidth(name, 13, 600) + 8,
    y: y + 21,
    text: fitMapText(typeText, nameRoom * 0.4 - 8, 9.5, 600),
    size: 9.5,
    weight: 600,
    fill: 'muted',
    caps: true,
  });
  y += FRAME_HEADER + 4;

  const innerX = x + FRAME_PAD;
  const innerW = w - FRAME_PAD * 2;
  y = drawHostsInColumns(out, group.hosts, style, innerX, y, innerW, columns);

  if (group.pools.length > 0) {
    y += SECTION_GAP + 2;
    drawSectionLabel(out, 'Shared storage', group.pools.length, innerX, y);
    y += SECTION_LABEL;
    const columnWidth = (innerW - (columns - 1) * GAP) / columns;
    const shown = group.pools.slice(0, MAX_POOLS * columns);
    const perColumn = Math.ceil(shown.length / columns);
    let bottom = y;
    for (let column = 0; column < columns; column += 1) {
      const slice = shown.slice(column * perColumn, (column + 1) * perColumn);
      bottom = Math.max(
        bottom,
        drawPools(out, slice, innerX + column * (columnWidth + GAP), y, columnWidth),
      );
    }
    y = bottom;
    if (group.pools.length > shown.length) {
      out.nodes.push({
        kind: 'text',
        x: innerX + 11,
        y: y + 12,
        text: `+${group.pools.length - shown.length} more`,
        size: 10,
        weight: 400,
        fill: 'muted',
      });
      y += POOL_ROW;
    }
  }

  y += FRAME_PAD;
  const frame = out.nodes[frameIndex];
  if (frame.kind === 'rect') frame.h = y - top;
  return y;
};

type PosterHeader = MapLayoutOptions['header'];

const drawPosterHeader = (
  out: Cursor,
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

const drawPosterFooter = (
  out: Cursor,
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
    fill: 'faint',
    anchor: 'end',
  });
  return bottom + 30 + margin - 8;
};

const blockSpan = (block: MapBlock, columns: number): number =>
  block.kind === 'host' ? 1 : Math.min(columns, Math.max(1, block.group.hosts.length));

export function layoutInfrastructureMap(
  model: InfrastructureMapModel,
  options: MapLayoutOptions,
): MapScene {
  const margin = options.header ? 28 : 0;
  const width = Math.max(MIN_COLUMN + margin * 2, Math.floor(options.width));
  const contentWidth = width - margin * 2;
  const columns = Math.min(
    MAX_COLUMNS,
    Math.max(1, Math.floor((contentWidth + GAP) / (MIN_COLUMN + GAP))),
  );
  const columnWidth = (contentWidth - (columns - 1) * GAP) / columns;
  const out: Cursor = { nodes: [], hits: [] };
  const style: MapDrawStyle = { density: options.density };

  const top = drawPosterHeader(out, options.header, margin, contentWidth);

  const heights = new Array<number>(columns).fill(top);
  const place = (block: MapBlock, column: number, y: number, span: number): number => {
    const x = margin + column * (columnWidth + GAP);
    const w = span * columnWidth + (span - 1) * GAP;
    const bottom =
      block.kind === 'host'
        ? drawHost(out, block.host, style, x, y, w)
        : drawGroup(out, block.group, style, x, y, w, span);
    for (let index = column; index < column + span; index += 1) heights[index] = bottom + GAP;
    return bottom;
  };
  const hostHeight = (host: MapHost): number =>
    drawHost({ nodes: [], hits: [] }, host, style, 0, 0, columnWidth);

  const pending = [...model.blocks];
  while (pending.length > 0) {
    const block = pending.shift()!;
    const span = blockSpan(block, columns);
    const { column, y } = placeOnSkyline(heights, span);
    if (span > 1) {
      // A wide block seals whatever is still open above it. Before it lands,
      // move single cards from later in the order up into those gaps.
      for (let index = column; index < column + span; index += 1) {
        for (;;) {
          const room = y - heights[index];
          const fit = pending.findIndex(
            (candidate) => candidate.kind === 'host' && hostHeight(candidate.host) + GAP <= room,
          );
          if (fit < 0) break;
          place(pending.splice(fit, 1)[0], index, heights[index], 1);
        }
      }
    }
    place(block, column, y, span);
  }

  const height = drawPosterFooter(
    out,
    options.header,
    margin,
    width,
    Math.max(top, Math.max(...heights) - GAP),
  );

  return { width, height: Math.ceil(height), nodes: [...out.nodes, ...out.hits] };
}

const NET_GUTTER = 26;
const NET_CHANNEL = 4;
const BUS_LANE = 20;
const BUS_HEIGHT = 4;
const BAND_GAP = 26;

export const mapNetworkTone = (segment: MapSegment): MapNetworkTone =>
  segment.colorIndex < 0 ? 'netOther' : (`net${segment.colorIndex % 6}` as MapNetworkTone);

const SEGMENT_KIND_NOTE: Record<MapSegment['kind'], string> = {
  subnet: 'subnet',
  range: 'address range',
  portgroup: 'vSphere port group',
  other: 'no shared range',
};

/**
 * The connection view. Every network is a horizontal line, drawn once. A
 * machine hangs under the network its own address is on, and a line runs
 * from it to each other network it or its guests are on, so what joins two
 * networks, and which networks nothing joins, can be read off the picture.
 */
export function layoutInfrastructureNetworkMap(
  model: InfrastructureMapModel,
  options: MapLayoutOptions,
): MapScene {
  const margin = options.header ? 28 : 0;
  const width = Math.max(MIN_COLUMN + NET_GUTTER + margin * 2, Math.floor(options.width));
  const contentWidth = width - margin * 2;
  const columns = Math.min(
    MAX_COLUMNS,
    Math.max(1, Math.floor(contentWidth / (MIN_COLUMN + NET_GUTTER))),
  );
  const columnWidth = contentWidth / columns - NET_GUTTER;
  const columnX = (column: number) => margin + NET_GUTTER + column * (columnWidth + NET_GUTTER);

  const tones = new Map(model.segments.map((segment) => [segment.id, mapNetworkTone(segment)]));
  const toneOf = (segmentId: string): MapNetworkTone => tones.get(segmentId) ?? 'netOther';
  const style: MapDrawStyle = { density: options.density, networkTone: toneOf };

  const hosts = model.blocks.flatMap((block) =>
    block.kind === 'group' ? block.group.hosts : [block.host],
  );
  const homeOf = (host: MapHost): string | undefined =>
    host.ownSegmentIds[0] ?? host.guestSegmentIds[0];

  const out: Cursor = { nodes: [], hits: [] };
  const buses: SceneNode[] = [];
  const labels: SceneNode[] = [];
  const links: SceneNode[] = [];
  const busY = new Map<string, number>();
  const ties: Array<{ column: number; segmentId: string; y: number; cardX: number }> = [];

  let y = drawPosterHeader(out, options.header, margin, contentWidth);

  const drawCards = (members: MapHost[], top: number): number => {
    if (members.length === 0) return top;
    const heights = new Array<number>(columns).fill(top);
    for (const host of members) {
      const { column, y: cardTop } = placeOnSkyline(heights, 1);
      const x = columnX(column);
      heights[column] = drawHost(out, host, style, x, cardTop, columnWidth) + GAP;
      const home = homeOf(host);
      if (!home) continue;
      const others = [...host.ownSegmentIds, ...host.guestSegmentIds].filter((id) => id !== home);
      [home, ...others].slice(0, 5).forEach((segmentId, index) => {
        if (segmentId) ties.push({ column, segmentId, y: cardTop + 14 + index * 6, cardX: x });
      });
    }
    return Math.max(...heights) - GAP;
  };

  // The networks, stacked as lines across the top. Each carries its own
  // name; everything below ties up to the lines it is on.
  const labelX = margin + NET_GUTTER + 10;
  for (const segment of model.segments) {
    const tone = toneOf(segment.id);
    const middle = y + BUS_LANE / 2;
    buses.push({
      kind: 'rect',
      x: margin,
      y: middle - BUS_HEIGHT / 2,
      w: contentWidth,
      h: BUS_HEIGHT,
      r: BUS_HEIGHT / 2,
      fill: tone,
    });
    busY.set(segment.id, middle);
    const count = `${segment.memberCount} ${segment.memberCount === 1 ? 'address' : 'addresses'}`;
    const note = `${SEGMENT_KIND_NOTE[segment.kind]} · ${count}`;
    const noteWidth = textWidth(note, 10, 400);
    const label = fitMapText(
      segment.label,
      Math.min(contentWidth - 60, 520) - noteWidth,
      11.5,
      600,
    );
    const labelWidth = textWidth(label, 11.5, 600);
    // The name interrupts its line, so the line reads as belonging to it.
    labels.push({
      kind: 'rect',
      x: labelX - 6,
      y: y + 1,
      w: labelWidth + noteWidth + 20,
      h: BUS_LANE - 2,
      r: 3,
      fill: 'canvas',
    });
    labels.push({
      kind: 'text',
      x: labelX,
      y: middle + 4,
      text: label,
      size: 11.5,
      weight: 600,
      fill: 'text',
    });
    labels.push({
      kind: 'text',
      x: labelX + labelWidth + 8,
      y: middle + 4,
      text: note,
      size: 10,
      weight: 400,
      fill: 'muted',
    });
    y += BUS_LANE;
  }
  if (model.segments.length > 0) y += 14;

  const order = new Map(model.segments.map((segment, index) => [segment.id, index]));
  const placed = hosts
    .filter((host) => homeOf(host))
    .sort((a, b) => order.get(homeOf(a)!)! - order.get(homeOf(b)!)!);
  const unplaced = hosts.filter((host) => !homeOf(host));
  y = drawCards([...placed, ...unplaced], y) + BAND_GAP;

  // One vertical line per network per column, in the gutter left of the
  // cards, from the network's line to the furthest card it reaches.
  const channels = new Map<number, string[]>();
  const runs = new Map<
    string,
    { column: number; segmentId: string; ys: number[]; cardX: number }
  >();
  for (const tie of ties) {
    if (!busY.has(tie.segmentId)) continue;
    const order = channels.get(tie.column) ?? [];
    if (!order.includes(tie.segmentId)) order.push(tie.segmentId);
    channels.set(tie.column, order);
    const key = `${tie.column}|${tie.segmentId}`;
    const run = runs.get(key) ?? {
      column: tie.column,
      segmentId: tie.segmentId,
      ys: [],
      cardX: tie.cardX,
    };
    run.ys.push(tie.y);
    runs.set(key, run);
  }
  for (const run of runs.values()) {
    const tone = toneOf(run.segmentId);
    const channel = Math.min(4, channels.get(run.column)!.indexOf(run.segmentId));
    const x = run.cardX - 6 - channel * NET_CHANNEL;
    const bus = busY.get(run.segmentId)!;
    const from = Math.min(bus, ...run.ys);
    const to = Math.max(bus, ...run.ys);
    links.push({ kind: 'rect', x: x - 1, y: from, w: 2, h: to - from + 1, r: 1, fill: tone });
    links.push({ kind: 'rect', x: x - 3.5, y: bus - 3.5, w: 7, h: 7, r: 3.5, fill: tone });
    for (const tieY of run.ys) {
      links.push({
        kind: 'rect',
        x: x - 1,
        y: tieY - 1,
        w: run.cardX - x + 1,
        h: 2,
        r: 1,
        fill: tone,
      });
    }
  }

  const height = drawPosterFooter(out, options.header, margin, width, Math.max(0, y - BAND_GAP));
  return {
    width,
    height: Math.ceil(height),
    nodes: [...buses, ...links, ...labels, ...out.nodes, ...out.hits],
  };
}
