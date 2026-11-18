import { estimateTextWidth } from '@/utils/format';
import {
  drawPosterFooter,
  drawPosterHeader,
  fitMapText,
  mapHitMeta,
  type MapCursor,
  type MapLayoutOptions,
  type MapNetworkTone,
  type MapScene,
  type MapTone,
  type SceneNode,
} from './infrastructureMapLayout';
import {
  DIAGRAM_MAX_NETWORKS,
  type DiagramGroup,
  type DiagramLink,
  type DiagramNetwork,
  type DiagramTile,
  type ServiceDiagram,
} from './infrastructureMapServices';

/**
 * Lays the service diagram out in layers that read from the top down: the
 * Internet, the ways in, the networks, what runs (one box per purpose), then
 * storage and backups, then hardware. Connectors run between layers. A line
 * between two single tiles is drawn only for a connection a collected value
 * states, so every line on the picture says something Pulse knows.
 *
 * The boxes are drawn the way the rest of the product draws a section: a
 * bordered card with a small uppercase header strip and its count, in the
 * theme's own colours.
 */

interface Metrics {
  tileW: number;
  tileH: number;
  icon: number;
  gap: number;
  pad: number;
  named: boolean;
}

const NAMED: Metrics = { tileW: 92, tileH: 72, icon: 32, gap: 4, pad: 8, named: true };
const COMPACT: Metrics = { tileW: 40, tileH: 40, icon: 26, gap: 4, pad: 8, named: false };

const HEADER = 26;
const CARD_RADIUS = 6;
const GROUP_GAP = 16;
const LAYER_GAP = 32;
const ROW_GAP = 32;
const BUS_DROP = 18;
const GUTTER = 16;
const PILL_HEIGHT = 24;
const MAX_COLUMNS = 8;
// How far a link stops short of the icon it leaves or arrives at, and how far
// under a row of boxes it runs when it goes from one row to another.
const LINK_CLEARANCE = 2;
// Where each kind of link runs and what it is drawn in: how far above a row
// of tiles, how far to one side of the middle of a gap between columns, and
// how far under a row of boxes. A proxy's route is drawn like the arrow from
// the Internet that it continues. A shared network is drawn in the neutral
// tone, because it is an arrangement and not a path anyone asked for.
const LINK_LANES: Record<
  DiagramLink['kind'],
  { above: number; aside: number; under: number; tone: MapTone }
> = {
  route: { above: 1, aside: 1.5, under: 9, tone: 'accent' },
  network: { above: 5, aside: -1.5, under: 5, tone: 'muted' },
};
// A link passes the network pills in the space between two of them, which is
// as wide as the pills are set apart, or this far beyond the end of their row.
// Nearer than the clearance to a pill counts as behind it.
const PILL_SPACING = 8;
const PILL_ROUND = 5;
const PILL_CLEARANCE = 2;
const MIN_WIDTH = 300;
const NETWORK_TONES: readonly MapNetworkTone[] = ['net0', 'net1', 'net2', 'net3', 'net4', 'net5'];
const LOWER_LABEL: Record<string, string> = {
  storage: 'disks and volumes',
  backups: 'backups',
  hardware: 'runs on',
};
// Fitting to a screen: how much wider than the screen the drawing may be laid
// out before it is scaled down, in how many tries, and how small it may get.
const FIT_MAX_STRETCH = 3;
const FIT_STEPS = 24;
const FIT_MIN_SCALE = 0.3;

const labelWidth = (text: string, size: number): number =>
  estimateTextWidth(text, { fontPx: size });

// Two letters that stand for a name with no logo: the initials of its first
// two words, or its first two letters when it is one word.
const lettersOf = (name: string): string => {
  if (name.startsWith('+')) return name.slice(0, 4);
  const parts = name.split(/[^A-Za-z0-9]+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).replace(/^./, (first) => first.toUpperCase());
  return (parts[0][0] + parts[1][0]).toUpperCase();
};

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
  /** Where the title and count in the box's header strip end. */
  titleEnd?: number;
}
const centreX = (box: Box): number => box.x + box.w / 2;
const bottomOf = (box: Box): number => box.y + box.h;

interface Sized {
  group: DiagramGroup;
  columns: number;
  w: number;
  h: number;
}

const headerText = (group: DiagramGroup): { title: string; count: string } => ({
  title: group.label.toUpperCase(),
  count: group.count.toLocaleString(),
});

const sizeGroup = (group: DiagramGroup, metrics: Metrics, maxWidth: number): Sized => {
  const count = group.tiles.length;
  const pitch = metrics.tileW + metrics.gap;
  const fit = Math.max(1, Math.floor((maxWidth - metrics.pad * 2 + metrics.gap) / pitch));
  // A handful sits in one line. More than that makes a block several times
  // wider than it is tall, because the screen it has to fit on is wide and
  // every extra row of tiles pushes the rest of the map down.
  const wanted = count <= 5 ? count : Math.ceil(Math.sqrt(count * (metrics.named ? 3 : 4)));
  const most = Math.max(1, Math.min(count, fit, MAX_COLUMNS * (metrics.named ? 1 : 2), wanted));
  const rows = Math.ceil(count / most);
  // The same number of rows with the tiles shared out evenly, so the last
  // row is not left with one tile.
  const columns = Math.ceil(count / rows);
  const tilesWidth = columns * pitch - metrics.gap;
  const { title, count: total } = headerText(group);
  const titleWidth = Math.min(maxWidth, labelWidth(title, 11) + labelWidth(total, 11) + 36);
  return {
    group,
    columns,
    w: Math.max(tilesWidth + metrics.pad * 2, titleWidth),
    h: HEADER + metrics.pad + rows * (metrics.tileH + metrics.gap) - metrics.gap + metrics.pad,
  };
};

/** Where a tile sits, which is what a link to it has to steer by. */
interface Anchor {
  /** The middle of the tile, which is the middle of its icon. */
  x: number;
  /** The top of its icon, where a link leaves or arrives. */
  iconTop: number;
  /** The top of the tile. The clear channel above its row runs just over it. */
  tileTop: number;
  /** Which row of boxes the tile is in, as an index into the painter's bands. */
  band: number;
  /** The box the tile is in. A tile drawn on its own has none. */
  box?: Box;
}

/** One row of boxes across the map. */
interface Band {
  bottom: number;
  boxes: Box[];
}

/** The network pills, drawn between two rows of boxes. */
interface PillRow {
  /** The row of boxes above the pills, as an index into the painter's bands. */
  after: number;
  top: number;
  bottom: number;
  /** Where each pill starts and ends across the map. */
  spans: Array<[number, number]>;
}

interface Painter {
  out: MapCursor;
  metrics: Metrics;
  toneFor: (segmentId: string | undefined) => MapNetworkTone | undefined;
  anchors: Map<string, Anchor>;
  /** The cards behind the tiles. Links are drawn over these and under the rest. */
  frames: SceneNode[];
  /** Every row of boxes drawn so far, top to bottom. */
  bands: Band[];
  /** The row of network pills, which a link passes between and never behind. */
  pills?: PillRow;
  /** The box being filled, while its tiles are drawn. */
  box?: Box;
}

const drawTile = (painter: Painter, tile: DiagramTile, x: number, y: number, dashed = false) => {
  const { out, metrics } = painter;
  const off = tile.health === 'off';
  const cx = x + metrics.tileW / 2;
  // What stands for several things, or for something Pulse does not monitor,
  // has one line of text and no page of its own.
  const real = tile.navigable;
  if (dashed) {
    out.nodes.push({
      kind: 'rect',
      x,
      y,
      w: metrics.tileW,
      h: metrics.tileH,
      r: CARD_RADIUS,
      fill: 'none',
      stroke: 'border',
      dashed: true,
    });
  }
  const iconY = metrics.named ? y + 7 : y + (metrics.tileH - metrics.icon) / 2;
  painter.anchors.set(tile.id, {
    x: cx,
    iconTop: iconY,
    tileTop: y,
    band: painter.bands.length - 1,
    ...(painter.box ? { box: painter.box } : {}),
  });
  if (tile.platform) {
    const size = metrics.icon - 10;
    out.nodes.push({
      kind: 'icon',
      x: cx - size / 2,
      y: iconY + 5,
      size,
      platform: tile.platform,
    });
  } else {
    out.nodes.push({
      kind: 'app',
      x: cx - metrics.icon / 2,
      y: iconY,
      size: metrics.icon,
      ...(tile.icon ? { icon: tile.icon } : {}),
      letters: lettersOf(tile.name),
      ...(off ? { dim: true } : {}),
    });
  }
  if (metrics.named) {
    out.nodes.push({
      kind: 'text',
      x: cx,
      y: y + 53,
      text: fitMapText(real || dashed ? tile.name : tile.caption, metrics.tileW - 6, 12, 500),
      size: 12,
      weight: 500,
      fill: off ? 'muted' : 'text',
      anchor: 'middle',
      ...(off ? { dim: true } : {}),
    });
    if (real || dashed) {
      out.nodes.push({
        kind: 'text',
        x: cx,
        y: y + 66,
        text: fitMapText(tile.caption, metrics.tileW - 6, 10, 400),
        size: 10,
        weight: 400,
        fill: 'muted',
        anchor: 'middle',
        ...(off ? { dim: true } : {}),
      });
    }
  }
  const tone = painter.toneFor(tile.segmentId);
  if (tone) out.nodes.push({ kind: 'rect', x: x + 4, y: y + 6, w: 3, h: 10, r: 1.5, fill: tone });
  // The Internet has nothing to say on hover. A gateway does: who uses it.
  if (dashed && !tile.links?.length) return;
  if (real) {
    // The status dot sits on the icon's corner, ringed in the card's colour,
    // the way a badge sits on an avatar.
    const dotX = cx + metrics.icon / 2 - 1;
    const dotY = iconY + 1;
    out.nodes.push({ kind: 'rect', x: dotX - 6, y: dotY - 6, w: 12, h: 12, r: 6, fill: 'surface' });
    out.nodes.push({ kind: 'dot', x: dotX, y: dotY, r: 4, health: tile.health });
  }
  out.hits.push({
    kind: 'hit',
    x,
    y,
    w: metrics.tileW,
    h: metrics.tileH,
    r: 4,
    hit: {
      id: tile.id,
      name: tile.name,
      meta: real
        ? [mapHitMeta(tile.typeLabel, tile.status, tile.health), tile.caption]
            .filter(Boolean)
            .join(' · ')
        : dashed
          ? tile.typeLabel
          : `${tile.name.replace('+', '')} ${tile.caption}`,
      health: tile.health,
      cpu: tile.cpu,
      memory: tile.memory,
      address: tile.address,
      detail: tile.detail,
      ...(tile.links?.length ? { links: tile.links } : {}),
      navigable: real,
    },
  });
};

// A section card: border, header strip, uppercase label and count, as a
// table section is drawn elsewhere in the product.
const drawGroup = (painter: Painter, sized: Sized, x: number, y: number): Box => {
  const { out, metrics, frames } = painter;
  frames.push({
    kind: 'rect',
    x,
    y,
    w: sized.w,
    h: sized.h,
    r: CARD_RADIUS,
    fill: 'surface',
    stroke: 'border',
  });
  // The strip is rounded with the card at the top and square where it meets
  // the body, so it is drawn as a rounded piece with its lower half squared.
  frames.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + 0.5,
    w: sized.w - 1,
    h: HEADER - 0.5,
    r: CARD_RADIUS - 0.5,
    fill: 'surfaceAlt',
  });
  frames.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + HEADER / 2,
    w: sized.w - 1,
    h: HEADER / 2,
    r: 0,
    fill: 'surfaceAlt',
  });
  frames.push({ kind: 'rect', x, y: y + HEADER, w: sized.w, h: 1, r: 0, fill: 'border' });
  const { title, count } = headerText(sized.group);
  const fitted = fitMapText(title, sized.w - 24 - labelWidth(count, 11) - 8, 11, 600);
  out.nodes.push({
    kind: 'text',
    x: x + 10,
    y: y + 17,
    text: fitted,
    size: 11,
    weight: 600,
    fill: 'muted',
    caps: true,
  });
  // Tracked-out capitals run wider than the plain estimate.
  const countX = x + 10 + labelWidth(fitted, 11) + fitted.length * 0.6 + 8;
  out.nodes.push({
    kind: 'text',
    x: countX,
    y: y + 17,
    text: count,
    size: 11,
    weight: 600,
    fill: 'text',
  });
  const pitchX = metrics.tileW + metrics.gap;
  const pitchY = metrics.tileH + metrics.gap;
  const tilesWidth = sized.columns * pitchX - metrics.gap;
  const left = x + (sized.w - tilesWidth) / 2;
  const box: Box = { x, y, w: sized.w, h: sized.h, titleEnd: countX + labelWidth(count, 11) };
  painter.box = box;
  sized.group.tiles.forEach((tile, index) => {
    drawTile(
      painter,
      tile,
      left + (index % sized.columns) * pitchX,
      y + HEADER + metrics.pad + Math.floor(index / sized.columns) * pitchY,
    );
  });
  painter.box = undefined;
  return box;
};

const fillRows = (sized: Sized[], width: number): Sized[][] => {
  const rows: Sized[][] = [];
  let row: Sized[] = [];
  let used = 0;
  for (const entry of sized) {
    const next = used === 0 ? entry.w : used + GROUP_GAP + entry.w;
    if (row.length > 0 && next > width) {
      rows.push(row);
      row = [entry];
      used = entry.w;
    } else {
      row.push(entry);
      used = next;
    }
  }
  if (row.length > 0) rows.push(row);
  return rows;
};

// Groups in reading order, as many to a row as fit. When they need several
// rows the rows are evened out, so the last one is not left with a single box.
const packRows = (sized: Sized[], width: number): Sized[][] => {
  const rows = fillRows(sized, width);
  if (rows.length < 2) return rows;
  let low = Math.max(...sized.map((entry) => entry.w));
  let high = width;
  while (high - low > 4) {
    const middle = (low + high) / 2;
    if (fillRows(sized, middle).length <= rows.length) high = middle;
    else low = middle;
  }
  return fillRows(sized, high);
};

const placeRow = (
  painter: Painter,
  row: Sized[],
  left: number,
  width: number,
  y: number,
): Box[] => {
  const total = row.reduce((sum, entry) => sum + entry.w, 0) + GROUP_GAP * (row.length - 1);
  let x = left + Math.max(0, (width - total) / 2);
  const band: Band = { bottom: y, boxes: [] };
  painter.bands.push(band);
  for (const entry of row) {
    const box = drawGroup(painter, entry, x, y);
    band.boxes.push(box);
    band.bottom = Math.max(band.bottom, bottomOf(box));
    x += entry.w + GROUP_GAP;
  }
  return band.boxes;
};

const line = (
  lines: SceneNode[],
  points: Array<[number, number]>,
  options: { arrow?: boolean; dashed?: boolean } = {},
) => {
  lines.push({
    kind: 'line',
    points,
    tone: 'accent',
    ...(options.arrow ? { arrow: true } : {}),
    ...(options.dashed ? { dashed: true } : {}),
  });
};

const connectorLabel = (labels: SceneNode[], text: string, x: number, y: number) => {
  const w = labelWidth(text, 11) + 12;
  labels.push({ kind: 'rect', x: x - w / 2, y: y - 8, w, h: 16, r: 4, fill: 'surface' });
  labels.push({
    kind: 'text',
    x,
    y: y + 4,
    text,
    size: 11,
    weight: 400,
    fill: 'muted',
    anchor: 'middle',
  });
};

// A Proxmox bridge counts what is plugged into it, since a guest there may
// report no address at all. Its detail is the subnet it carries, or whose
// bridge it is when two clusters both have a `vmbr0`.
const networkText = (network: DiagramNetwork): string =>
  network.kind === 'bridge'
    ? [network.label, network.detail, `${network.memberCount.toLocaleString()} connected`]
        .filter(Boolean)
        .join(' · ')
    : `${network.label} · ${network.memberCount.toLocaleString()} ${
        network.memberCount === 1 ? 'address' : 'addresses'
      }`;

// One pill per network, centred and wrapped. Returns the bottom of the row.
const drawNetworks = (
  out: MapCursor,
  networks: DiagramNetwork[],
  left: number,
  width: number,
  y: number,
  toneOf: (network: DiagramNetwork) => MapTone,
): number => {
  const shown = networks.slice(0, DIAGRAM_MAX_NETWORKS);
  const hidden = networks.length - shown.length;
  const pills = shown.map((network) => {
    const text = fitMapText(networkText(network), width - 44, 12, 400);
    return { text, tone: toneOf(network), w: labelWidth(text, 12) + 36 };
  });
  if (hidden > 0) {
    const text = `+${hidden} more`;
    pills.push({ text, tone: 'netOther', w: labelWidth(text, 12) + 36 });
  }
  const rows: Array<typeof pills> = [];
  let row: typeof pills = [];
  let used = 0;
  for (const pill of pills) {
    const next = used === 0 ? pill.w : used + 8 + pill.w;
    if (row.length > 0 && next > width) {
      rows.push(row);
      row = [pill];
      used = pill.w;
    } else {
      row.push(pill);
      used = next;
    }
  }
  if (row.length > 0) rows.push(row);
  let top = y;
  for (const entries of rows) {
    const total = entries.reduce((sum, pill) => sum + pill.w, 0) + 8 * (entries.length - 1);
    let x = left + (width - total) / 2;
    for (const pill of entries) {
      out.nodes.push({
        kind: 'rect',
        x,
        y: top,
        w: pill.w,
        h: PILL_HEIGHT,
        r: PILL_HEIGHT / 2,
        fill: 'surfaceAlt',
        stroke: 'border',
      });
      out.nodes.push({
        kind: 'rect',
        x: x + 10,
        y: top + PILL_HEIGHT / 2 - 4,
        w: 8,
        h: 8,
        r: 4,
        fill: pill.tone,
      });
      out.nodes.push({
        kind: 'text',
        x: x + 24,
        y: top + 16,
        text: pill.text,
        size: 12,
        weight: 400,
        fill: 'text',
      });
      x += pill.w + 8;
    }
    top += PILL_HEIGHT + 6;
  }
  return top - 6;
};

// Where a link can pass the network pills from top to bottom: between two
// pills, or round either end of their row. Pills on several rows count as one
// row here, so a way is clear of every one of them.
const waysPastPills = (pills: PillRow): number[] => {
  const merged: Array<[number, number]> = [];
  for (const [start, end] of [...pills.spans].sort((a, b) => a[0] - b[0])) {
    const last = merged[merged.length - 1];
    if (last && start <= last[1]) last[1] = Math.max(last[1], end);
    else merged.push([start, end]);
  }
  if (merged.length === 0) return [];
  const ways = [merged[0][0] - PILL_ROUND];
  merged.forEach(([, end], index) => {
    const next = merged[index + 1];
    if (!next) ways.push(end + PILL_ROUND);
    else if (next[0] - end >= PILL_SPACING) ways.push((end + next[0]) / 2);
  });
  return ways;
};

// The same path without a point drawn twice or a run that doubles back on
// itself, which a step aside can leave where it meets the run it joins.
const withoutDoubling = (points: Array<[number, number]>): Array<[number, number]> => {
  const path: Array<[number, number]> = [];
  for (const point of points) {
    path.push(point);
    for (;;) {
      const [a, b, c] = [path[path.length - 3], path[path.length - 2], path[path.length - 1]];
      if (b && b[0] === c[0] && b[1] === c[1]) path.pop();
      else if (a && b && ((a[0] === b[0] && b[0] === c[0]) || (a[1] === b[1] && b[1] === c[1])))
        path.splice(path.length - 2, 1);
      else break;
    }
  }
  return path;
};

// A link runs at right angles like every other connector, and only where
// nothing else is drawn, so it never passes through a tile it does not join:
// it leaves the top of one icon, runs level in the clear strip above that row
// of tiles, goes up or down in the gap between two columns, and comes down
// onto the other icon. From one row of boxes to another it crosses in the
// space between the rows, and passes a row in between by the gap between two
// of its boxes. Past the network pills it goes between two of them, or round
// the end of their row: behind a pill it would read as joined to that network.
// Links that share a strip run together as one line.
const drawLinks = (painter: Painter, links: DiagramLink[], out: SceneNode[]) => {
  const { metrics, bands } = painter;
  const beside = metrics.tileW / 2 + metrics.gap / 2;
  const pills = painter.pills && painter.pills.after >= 0 ? painter.pills : undefined;
  const pillWays = pills ? waysPastPills(pills) : [];
  const behindPill = (x: number): boolean =>
    pills !== undefined &&
    pills.spans.some(([start, end]) => x > start - PILL_CLEARANCE && x < end + PILL_CLEARANCE);
  // Where a row of boxes can be passed from top to bottom: either side of
  // it, or between two of its boxes. The one nearest to where the link is
  // heading.
  const passage = (band: Band, towards: number): number => {
    // Round the outside it keeps close to the row, clear of the rails the
    // layer connectors use down the edges of the map.
    const boxes = [...band.boxes].sort((a, b) => a.x - b.x);
    const ways = [boxes[0].x - GROUP_GAP / 4];
    boxes.forEach((box, index) => {
      const next = boxes[index + 1];
      ways.push(next ? (box.x + box.w + next.x) / 2 : box.x + box.w + GROUP_GAP / 4);
    });
    return ways.reduce((best, way) =>
      Math.abs(way - towards) < Math.abs(best - towards) ? way : best,
    );
  };

  for (const link of links) {
    const from = painter.anchors.get(link.fromId);
    const to = painter.anchors.get(link.toId);
    if (!from || !to || from.band < 0 || to.band < 0) continue;
    // Each kind keeps to its own lane, a few pixels from the other, so a
    // proxy's routes and a shared network never run as one line.
    const lane = LINK_LANES[link.kind];
    const strip = (anchor: Anchor): number => anchor.tileTop - lane.above;
    const gap = (anchor: Anchor, towards: number): number =>
      anchor.x + (towards > anchor.x ? 1 : -1) * beside + lane.aside;
    // The space under a row of boxes, clear of the connector that feeds the
    // next row.
    const corridor = (band: number): number => bands[band].bottom + lane.under;
    // Into a box from another row it comes down beside the tile. Where that
    // would run behind the box's title, it comes down just inside the box's
    // left edge instead and runs along the strip to the tile.
    const entry = (anchor: Anchor, towards: number): number => {
      const direct = gap(anchor, towards);
      const { box } = anchor;
      if (!box || direct < box.x + metrics.pad || direct > (box.titleEnd ?? box.x) + 4) {
        return direct;
      }
      return box.x + metrics.pad / 2 + lane.aside;
    };
    const points: Array<[number, number]> = [
      [from.x, from.iconTop - LINK_CLEARANCE],
      [from.x, strip(from)],
    ];
    // The upright part sits at the end that links share: beside the proxy
    // that routes several containers, beside the container whose network
    // several use. Those links then run together instead of side by side.
    const fanOut = link.kind === 'route';
    if (from.band === to.band) {
      if (strip(from) !== strip(to)) {
        const x = fanOut ? gap(from, to.x) : gap(to, from.x);
        points.push([x, strip(from)], [x, strip(to)]);
      }
    } else {
      const down = to.band > from.band;
      const step = down ? 1 : -1;
      // A proxy's routes go down beside it. Containers that use another's
      // network leave their row of boxes by the way nearest that one, so
      // every link from the row to the same place leaves it together.
      let x = fanOut ? gap(from, to.x) : passage(bands[from.band], to.x) + lane.aside;
      const arrive = entry(to, x);
      let y = corridor(down ? from.band : from.band - 1);
      points.push([x, strip(from)], [x, y]);
      for (let band = from.band + step; band !== to.band; band += step) {
        x = passage(bands[band], arrive) + lane.aside;
        points.push([x, y]);
        y = corridor(down ? band : band - 1);
        points.push([x, y]);
      }
      points.push([arrive, y], [arrive, strip(to)]);
    }
    points.push([to.x, strip(to)], [to.x, to.iconTop - LINK_CLEARANCE]);
    // An upright that would cross the pills behind one of them steps aside
    // first: level above the pills to the nearest way past them, through, and
    // level again below them, in the link's own lane on each side.
    const routed: Array<[number, number]> = [];
    let stepped = false;
    points.forEach((point, index) => {
      routed.push(point);
      const next = points[index + 1];
      if (!pills || !next || next[0] !== point[0]) return;
      const x = point[0];
      const top = Math.min(point[1], next[1]);
      const bottom = Math.max(point[1], next[1]);
      if (top >= pills.top || bottom <= pills.bottom || !behindPill(x)) return;
      const above = Math.max(top, corridor(pills.after));
      const below = Math.min(bottom, pills.bottom + lane.under);
      const down = next[1] > point[1];
      // The level run this upright joins above the pills ends at the other
      // end of the link. The way nearest that end is the one links share.
      const joined = down ? points[index - 1] : points[index + 2];
      const far = joined && joined[1] === above ? joined[0] : x;
      const way = pillWays
        .map((candidate) => candidate + lane.aside)
        .reduce((best, candidate) => {
          const cost = (at: number) => Math.abs(at - far) + Math.abs(at - x);
          if (cost(candidate) !== cost(best))
            return cost(candidate) < cost(best) ? candidate : best;
          return Math.abs(candidate - far) < Math.abs(best - far) ? candidate : best;
        });
      const detour: Array<[number, number]> = [
        [x, above],
        [way, above],
        [way, below],
        [x, below],
      ];
      routed.push(...(down ? detour : detour.reverse()));
      stepped = true;
    });
    out.push({
      kind: 'line',
      points: stepped ? withoutDoubling(routed) : points,
      tone: lane.tone,
      arrow: true,
      ...(link.idle ? { dashed: true } : {}),
    });
  }
};

export function layoutServiceDiagram(diagram: ServiceDiagram, options: MapLayoutOptions): MapScene {
  const metrics = options.density === 'marks' ? COMPACT : NAMED;
  const margin = options.header ? 28 : 0;
  const width = Math.max(MIN_WIDTH + margin * 2, Math.floor(options.width));
  const left = margin + GUTTER;
  const inner = width - (margin + GUTTER) * 2;
  const out: MapCursor = { nodes: [], hits: [] };
  const lines: SceneNode[] = [];
  const links: SceneNode[] = [];
  const labels: SceneNode[] = [];
  const mid = width / 2;

  // Only a network with a pill has a colour. A tile on any other network is
  // left unmarked, so a recycled colour never claims two networks are one.
  const toneByNetwork = new Map<string, MapNetworkTone>();
  for (const network of diagram.networks.slice(0, DIAGRAM_MAX_NETWORKS)) {
    toneByNetwork.set(
      network.id,
      network.colorIndex < 0
        ? 'netOther'
        : NETWORK_TONES[network.colorIndex % NETWORK_TONES.length],
    );
  }
  const painter: Painter = {
    out,
    metrics,
    // With one network every tile is on it, so the mark would say nothing.
    toneFor: (segmentId) =>
      diagram.networks.length > 1 && segmentId ? toneByNetwork.get(segmentId) : undefined,
    anchors: new Map(),
    frames: [],
    bands: [],
  };

  let y = drawPosterHeader(out, options.header, margin, width - margin * 2);
  /** Where the next layer's connector starts: the bottom of the layer above. */
  let source: { x: number; y: number } | null = null;

  // A gateway that machines name in their route tables. No line is drawn to
  // it: which machines use it is said on its hover, and a line from each
  // would cross the whole map.
  const gatewayPitch = metrics.tileW + metrics.gap;
  const drawGateways = (tiles: DiagramTile[], x: number, top: number) => {
    tiles.forEach((gateway, index) => {
      drawTile(painter, gateway, x + index * gatewayPitch, top, true);
    });
  };
  // As many gateways as a row of the given width holds, centred on the map.
  const drawGatewayRow = (top: number) => {
    const shown = diagram.gateways.slice(
      0,
      Math.max(1, Math.floor((inner + metrics.gap) / gatewayPitch)),
    );
    drawGateways(shown, mid - (shown.length * gatewayPitch - metrics.gap) / 2, top);
  };

  if (diagram.access) {
    const sized = sizeGroup(diagram.access, metrics, inner);
    const boxX = mid - sized.w / 2;
    const internet: DiagramTile = {
      id: 'internet',
      name: 'Internet',
      caption: 'not monitored',
      typeLabel: '',
      status: '',
      health: 'unknown',
      platform: 'internet',
      navigable: false,
    };
    const label = diagram.internet?.label ?? '';
    // The Internet sits beside the ways in when there is room, so the map
    // spends its height on what runs. Gateways take the other side.
    const reach = Math.max(72, labelWidth(label, 11) + 28);
    const beside = boxX - left >= metrics.tileW + reach;
    const internetAbove = diagram.internet !== undefined && !beside;
    const roomRight = Math.floor(
      (left + inner - (boxX + sized.w) - GROUP_GAP + metrics.gap) / gatewayPitch,
    );
    const gatewaysAbove = diagram.gateways.length > 0 && diagram.gateways.length > roomRight;
    if (internetAbove || gatewaysAbove) {
      const tileBottom = y + metrics.tileH;
      if (internetAbove) drawTile(painter, internet, mid - metrics.tileW / 2, y, true);
      if (gatewaysAbove && internetAbove) {
        // The Internet keeps the middle, over its arrow. Gateways follow it.
        const x = mid + metrics.tileW / 2 + metrics.gap;
        const room = Math.max(0, Math.floor((left + inner - x + metrics.gap) / gatewayPitch));
        drawGateways(diagram.gateways.slice(0, room), x, y);
      } else if (gatewaysAbove) {
        drawGatewayRow(y);
      }
      y = tileBottom + LAYER_GAP;
      if (internetAbove) {
        line(
          lines,
          [
            [mid, tileBottom],
            [mid, y],
          ],
          { arrow: true, dashed: true },
        );
        connectorLabel(labels, label, mid, (tileBottom + y) / 2);
      }
    }
    const band: Band = { bottom: y, boxes: [] };
    painter.bands.push(band);
    const box = drawGroup(painter, sized, boxX, y);
    band.boxes.push(box);
    band.bottom = bottomOf(box);
    if (diagram.internet && beside) {
      const tileX = boxX - reach - metrics.tileW;
      const lineY = box.y + box.h / 2;
      drawTile(painter, internet, tileX, lineY - metrics.tileH / 2, true);
      line(
        lines,
        [
          [tileX + metrics.tileW, lineY],
          [boxX, lineY],
        ],
        { arrow: true, dashed: true },
      );
      connectorLabel(labels, label, tileX + metrics.tileW + reach / 2, lineY - 12);
    }
    if (!gatewaysAbove) {
      drawGateways(
        diagram.gateways,
        boxX + sized.w + GROUP_GAP,
        box.y + box.h / 2 - metrics.tileH / 2,
      );
    }
    source = { x: mid, y: bottomOf(box) };
    y = bottomOf(box) + LAYER_GAP;
  } else if (diagram.gateways.length > 0) {
    // No ways in to sit beside: the gateways are the top of the map.
    drawGatewayRow(y);
    y += metrics.tileH + LAYER_GAP;
  }

  if (diagram.networks.length > 0) {
    const firstPillNode = out.nodes.length;
    const bandBottom = drawNetworks(
      out,
      diagram.networks,
      left,
      inner,
      y,
      (network) => toneByNetwork.get(network.id) ?? 'netOther',
    );
    // The outline of each pill is the one rectangle of a pill's height.
    painter.pills = {
      after: painter.bands.length - 1,
      top: y,
      bottom: bandBottom,
      spans: out.nodes
        .slice(firstPillNode)
        .flatMap((node): Array<[number, number]> =>
          node.kind === 'rect' && node.h === PILL_HEIGHT ? [[node.x, node.x + node.w]] : [],
        ),
    };
    if (source) {
      line(
        lines,
        [
          [source.x, source.y],
          [mid, y],
        ],
        { arrow: true },
      );
    }
    source = { x: mid, y: bandBottom };
    y = bandBottom + LAYER_GAP;
  }

  // What runs: one box per purpose, wrapped into rows.
  const rows = packRows(
    diagram.groups.map((group) => sizeGroup(group, metrics, inner)),
    inner,
  );
  const placed: Box[][] = [];
  let rowTop = y;
  for (const row of rows) {
    const boxes = placeRow(painter, row, left, inner, rowTop);
    placed.push(boxes);
    rowTop = Math.max(...boxes.map(bottomOf)) + ROW_GAP;
  }
  const single = placed.length === 1;
  const leftRail = margin + GUTTER / 2;
  const rightRail = width - margin - GUTTER / 2;

  if (source && placed.length > 0) {
    if (single) {
      const boxes = placed[0];
      const busY = boxes[0].y - BUS_DROP;
      line(lines, [
        [source.x, source.y],
        [source.x, busY],
      ]);
      const xs = [...boxes.map(centreX), source.x];
      line(lines, [
        [Math.min(...xs), busY],
        [Math.max(...xs), busY],
      ]);
      for (const box of boxes)
        line(
          lines,
          [
            [centreX(box), busY],
            [centreX(box), box.y],
          ],
          { arrow: true },
        );
    } else {
      // Several rows: a bus above each row, joined by a rail down the left edge.
      const firstBus = placed[0][0].y - BUS_DROP;
      const lastBus = placed[placed.length - 1][0].y - BUS_DROP;
      line(lines, [
        [source.x, source.y],
        [source.x, firstBus],
      ]);
      line(lines, [
        [leftRail, firstBus],
        [leftRail, lastBus],
      ]);
      for (const boxes of placed) {
        const busY = boxes[0].y - BUS_DROP;
        const reach = busY === firstBus ? source.x : 0;
        line(lines, [
          [leftRail, busY],
          [Math.max(reach, ...boxes.map(centreX)), busY],
        ]);
        for (const box of boxes)
          line(
            lines,
            [
              [centreX(box), busY],
              [centreX(box), box.y],
            ],
            { arrow: true },
          );
      }
    }
  }

  const appsBottom = placed.length > 0 ? rowTop - ROW_GAP : y - LAYER_GAP;
  y = placed.length > 0 ? appsBottom + LAYER_GAP + 14 : y;

  // What everything sits on shares the bottom layer: storage, backups and
  // hardware, each reached by a connector that says what it carries.
  const lower = [diagram.storage, diagram.backups, diagram.hardware].filter(
    (group): group is DiagramGroup => group !== undefined,
  );
  let lowerBoxes: Box[] = [];
  if (lower.length > 0) {
    const sized = lower.map((group) => sizeGroup(group, metrics, inner));
    const lowerRows = packRows(sized, inner);
    let top = y;
    for (const row of lowerRows) {
      const boxes = placeRow(painter, row, left, inner, top);
      lowerBoxes = [...lowerBoxes, ...boxes];
      top = Math.max(...boxes.map(bottomOf)) + ROW_GAP;
    }
    const lowerBottom = top - ROW_GAP;
    const targets = lowerBoxes.filter((box) => box.y === y);
    if (placed.length > 0 && targets.length > 0) {
      const collectY = appsBottom + BUS_DROP;
      if (single) {
        const boxes = placed[0];
        for (const box of boxes)
          line(lines, [
            [centreX(box), bottomOf(box)],
            [centreX(box), collectY],
          ]);
        const xs = [...boxes.map(centreX), ...targets.map(centreX)];
        line(lines, [
          [Math.min(...xs), collectY],
          [Math.max(...xs), collectY],
        ]);
      } else {
        // Each row drains from its last box to a rail down the right edge.
        let railTop = collectY;
        for (const boxes of placed) {
          const last = boxes[boxes.length - 1];
          const tickY = last.y + last.h / 2;
          line(lines, [
            [last.x + last.w, tickY],
            [rightRail, tickY],
          ]);
          railTop = Math.min(railTop, tickY);
        }
        line(lines, [
          [rightRail, railTop],
          [rightRail, collectY],
        ]);
        line(lines, [
          [Math.min(...targets.map(centreX)), collectY],
          [rightRail, collectY],
        ]);
      }
      targets.forEach((box, index) => {
        const group = lower[index];
        line(
          lines,
          [
            [centreX(box), collectY],
            [centreX(box), box.y],
          ],
          { arrow: true },
        );
        connectorLabel(labels, LOWER_LABEL[group.key], centreX(box), (collectY + box.y) / 2 + 3);
      });
      // A bottom layer too wide for one row wraps. The boxes on its later
      // rows are reached down the right edge, so none is left unconnected.
      const wrapped = lowerBoxes.slice(targets.length);
      if (wrapped.length > 0) {
        const reach = Math.max(...wrapped.map((box) => box.y)) - BUS_DROP;
        line(lines, [
          [Math.max(...targets.map(centreX)), collectY],
          [rightRail, collectY],
          [rightRail, reach],
        ]);
        wrapped.forEach((box, index) => {
          const busY = box.y - BUS_DROP;
          line(
            lines,
            [
              [rightRail, busY],
              [centreX(box), busY],
              [centreX(box), box.y],
            ],
            { arrow: true },
          );
          connectorLabel(
            labels,
            LOWER_LABEL[lower[targets.length + index].key],
            centreX(box),
            busY,
          );
        });
      }
    }
    y = lowerBottom + LAYER_GAP;
  } else if (placed.length > 0) {
    y = appsBottom + LAYER_GAP;
  }

  drawLinks(painter, diagram.links, links);

  const height = drawPosterFooter(out, options.header, margin, width, Math.max(0, y - LAYER_GAP));
  return {
    width,
    height: Math.ceil(height),
    // Layer connectors run behind the cards. A link joins two tiles inside
    // them, so it is drawn over the cards and under everything on them.
    nodes: [...lines, ...painter.frames, ...links, ...out.nodes, ...labels, ...out.hits],
  };
}

export interface FittedDiagram {
  scene: MapScene;
  /** How much the scene is shrunk to be drawn, from 1 (not at all) down. */
  scale: number;
}

/**
 * The diagram laid out to be seen whole in the space given, with nothing
 * below the fold. A map read by scrolling is not a map, so when the drawing
 * is taller than the space it is laid out wider, which puts more boxes on a
 * row and makes it shorter, and then scaled down to fit. Of the widths tried,
 * the one that leaves the drawing largest wins.
 */
export function fitServiceDiagram(
  diagram: ServiceDiagram,
  space: { width: number; height: number },
  density: MapLayoutOptions['density'],
): FittedDiagram {
  let best: FittedDiagram | null = null;
  for (let step = 0; step <= FIT_STEPS; step += 1) {
    const width = space.width * (1 + (step / FIT_STEPS) * (FIT_MAX_STRETCH - 1));
    const scene = layoutServiceDiagram(diagram, { width, density });
    const scale =
      scene.height > 0 ? Math.min(1, space.width / scene.width, space.height / scene.height) : 1;
    if (!best || scale > best.scale + 0.001) best = { scene, scale };
    if (scale >= 1) break;
  }
  const fitted = best as FittedDiagram;
  return { scene: fitted.scene, scale: Math.max(FIT_MIN_SCALE, fitted.scale) };
}
