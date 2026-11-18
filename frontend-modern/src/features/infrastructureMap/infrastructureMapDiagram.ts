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
  type DiagramNetwork,
  type DiagramTile,
  type ServiceDiagram,
} from './infrastructureMapServices';

/**
 * Lays the service diagram out in layers that read from the top down: the
 * Internet, the ways in, the networks, what runs (one box per purpose), then
 * storage and backups, then hardware. Connectors run between layers, never
 * between single tiles, so the picture stays readable at any size and every
 * line states something Pulse knows. A connection between two tiles is said
 * under the tile and on its hover, and each tile carries which tiles it is
 * joined to, so the page can mark them while one is pointed at.
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

interface Painter {
  out: MapCursor;
  metrics: Metrics;
  toneFor: (segmentId: string | undefined) => MapNetworkTone | undefined;
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
  // A dashed tile is a place, not a thing: only one with something to say
  // about its connections can be pointed at.
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
      navigable: real,
      ...(tile.links?.length ? { links: tile.links } : {}),
    },
  });
};

// A section card: border, header strip, uppercase label and count, as a
// table section is drawn elsewhere in the product.
const drawGroup = (painter: Painter, sized: Sized, x: number, y: number): Box => {
  const { out, metrics } = painter;
  out.nodes.push({
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
  out.nodes.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + 0.5,
    w: sized.w - 1,
    h: HEADER - 0.5,
    r: CARD_RADIUS - 0.5,
    fill: 'surfaceAlt',
  });
  out.nodes.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + HEADER / 2,
    w: sized.w - 1,
    h: HEADER / 2,
    r: 0,
    fill: 'surfaceAlt',
  });
  out.nodes.push({ kind: 'rect', x, y: y + HEADER, w: sized.w, h: 1, r: 0, fill: 'border' });
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
  out.nodes.push({
    kind: 'text',
    // Tracked-out capitals run wider than the plain estimate.
    x: x + 10 + labelWidth(fitted, 11) + fitted.length * 0.6 + 8,
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
  sized.group.tiles.forEach((tile, index) => {
    drawTile(
      painter,
      tile,
      left + (index % sized.columns) * pitchX,
      y + HEADER + metrics.pad + Math.floor(index / sized.columns) * pitchY,
    );
  });
  return { x, y, w: sized.w, h: sized.h };
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
  return row.map((entry) => {
    const box = drawGroup(painter, entry, x, y);
    x += entry.w + GROUP_GAP;
    return box;
  });
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

export function layoutServiceDiagram(diagram: ServiceDiagram, options: MapLayoutOptions): MapScene {
  const metrics = options.density === 'marks' ? COMPACT : NAMED;
  const margin = options.header ? 28 : 0;
  const width = Math.max(MIN_WIDTH + margin * 2, Math.floor(options.width));
  const left = margin + GUTTER;
  const inner = width - (margin + GUTTER) * 2;
  const out: MapCursor = { nodes: [], hits: [] };
  const lines: SceneNode[] = [];
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
    const box = drawGroup(painter, sized, boxX, y);
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
    const bandBottom = drawNetworks(
      out,
      diagram.networks,
      left,
      inner,
      y,
      (network) => toneByNetwork.get(network.id) ?? 'netOther',
    );
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

  // No line is drawn for a link between two tiles. Each end is told the
  // other, and only when both are drawn: an end folded into a `+N` tile has
  // no place to point at.
  const drawn = new Map(
    out.hits.flatMap((node) => (node.kind === 'hit' ? [[node.hit.id, node.hit] as const] : [])),
  );
  for (const link of diagram.links) {
    const from = drawn.get(link.fromId);
    const to = drawn.get(link.toId);
    if (!from || !to) continue;
    for (const [end, other] of [
      [from, to],
      [to, from],
    ] as const) {
      if (!end.linked?.includes(other.id)) end.linked = [...(end.linked ?? []), other.id];
    }
  }

  const height = drawPosterFooter(out, options.header, margin, width, Math.max(0, y - LAYER_GAP));
  return {
    width,
    height: Math.ceil(height),
    nodes: [...lines, ...out.nodes, ...labels, ...out.hits],
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
