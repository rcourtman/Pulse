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
import type {
  DiagramGroup,
  DiagramNetwork,
  DiagramTile,
  ServiceDiagram,
} from './infrastructureMapServices';

/**
 * Lays the service diagram out in layers that read from the top down: the
 * Internet, the ways in, the networks, what runs (one box per purpose), then
 * storage and backups, then hardware. Connectors run between layers, never
 * between single tiles, so the picture stays readable at any size and every
 * line states something Pulse knows.
 *
 * Like the other map layouts it is a function of the model and a width.
 */

interface Metrics {
  tileW: number;
  tileH: number;
  icon: number;
  gap: number;
  pad: number;
  head: number;
  named: boolean;
}

const NAMED: Metrics = { tileW: 84, tileH: 78, icon: 34, gap: 8, pad: 12, head: 26, named: true };
const COMPACT: Metrics = {
  tileW: 42,
  tileH: 42,
  icon: 26,
  gap: 6,
  pad: 10,
  head: 26,
  named: false,
};

const GROUP_GAP = 20;
const LAYER_GAP = 46;
const ROW_GAP = 46;
const BUS_DROP = 18;
const GUTTER = 16;
const BAND_HEIGHT = 26;
const MAX_COLUMNS = 8;
const MAX_NETWORKS = 6;
const MIN_WIDTH = 300;
const NETWORK_TONES: readonly MapNetworkTone[] = ['net0', 'net1', 'net2', 'net3', 'net4', 'net5'];

const labelWidth = (text: string, size: number): number =>
  estimateTextWidth(text, { fontPx: size });

const hueOf = (text: string): number => {
  let hash = 0;
  for (let index = 0; index < text.length; index += 1) {
    hash = (hash * 31 + text.charCodeAt(index)) % 360;
  }
  return hash;
};

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

const sizeGroup = (group: DiagramGroup, metrics: Metrics, maxWidth: number): Sized => {
  const count = group.tiles.length;
  const pitch = metrics.tileW + metrics.gap;
  const fit = Math.max(1, Math.floor((maxWidth - metrics.pad * 2 + metrics.gap) / pitch));
  // A handful sits in one line. More than that makes a block a little wider
  // than it is tall, and a large group spreads wider still so that a big
  // estate does not turn into one long column of boxes.
  const spread = metrics.named ? (count > 12 ? 1.8 : 1.25) : 2.4;
  const wanted = count <= 5 ? count : Math.ceil(Math.sqrt(count * spread));
  const columns = Math.max(1, Math.min(count, fit, MAX_COLUMNS * (metrics.named ? 1 : 2), wanted));
  const rows = Math.ceil(count / columns);
  const tilesWidth = columns * pitch - metrics.gap;
  const titleWidth = Math.min(maxWidth, labelWidth(group.label, 11) + metrics.pad * 2);
  return {
    group,
    columns,
    w: Math.max(tilesWidth + metrics.pad * 2, titleWidth),
    h: metrics.head + rows * (metrics.tileH + metrics.gap) - metrics.gap + metrics.pad,
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
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w: metrics.tileW,
    h: metrics.tileH,
    r: 8,
    fill: 'card',
    stroke: 'border',
    ...(dashed ? { dashed } : {}),
  });
  const iconY = metrics.named ? y + 9 : y + (metrics.tileH - metrics.icon) / 2;
  if (tile.platform) {
    const size = metrics.icon - 8;
    out.nodes.push({
      kind: 'icon',
      x: cx - size / 2,
      y: iconY + 4,
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
      hue: hueOf(tile.name),
      ...(off ? { dim: true } : {}),
    });
  }
  if (metrics.named) {
    out.nodes.push({
      kind: 'text',
      x: cx,
      y: y + 57,
      // A tile that stands for several shows their count as its mark, so
      // its one line of text says what they are.
      text: fitMapText(
        tile.navigable || dashed ? tile.name : tile.caption,
        metrics.tileW - 8,
        10.5,
        600,
      ),
      size: 10.5,
      weight: 600,
      fill: off ? 'faint' : 'text',
      anchor: 'middle',
    });
    if (tile.navigable || dashed) {
      out.nodes.push({
        kind: 'text',
        x: cx,
        y: y + 70,
        text: fitMapText(tile.caption, metrics.tileW - 8, 9, 400),
        size: 9,
        weight: 400,
        fill: off ? 'faint' : 'muted',
        anchor: 'middle',
      });
    }
  }
  const tone = painter.toneFor(tile.segmentId);
  if (tone) out.nodes.push({ kind: 'rect', x: x + 5, y: y + 6, w: 3, h: 10, r: 1.5, fill: tone });
  if (!dashed) {
    out.nodes.push({
      kind: 'dot',
      x: x + metrics.tileW - 9,
      y: y + 9,
      r: 3.5,
      health: tile.health,
    });
  }
  if (dashed) return;
  out.hits.push({
    kind: 'hit',
    x,
    y,
    w: metrics.tileW,
    h: metrics.tileH,
    r: 8,
    hit: {
      id: tile.id,
      name: tile.name,
      meta: tile.navigable
        ? [mapHitMeta(tile.typeLabel, tile.status, tile.health), tile.caption]
            .filter(Boolean)
            .join(' · ')
        : `${tile.name.replace('+', '')} ${tile.caption}`,
      health: tile.health,
      cpu: tile.cpu,
      memory: tile.memory,
      address: tile.address,
      detail: tile.detail,
      navigable: tile.navigable,
    },
  });
};

const drawGroup = (painter: Painter, sized: Sized, x: number, y: number): Box => {
  const { out, metrics } = painter;
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w: sized.w,
    h: sized.h,
    r: 10,
    fill: 'frame',
    stroke: 'border',
  });
  out.nodes.push({
    kind: 'text',
    x: x + sized.w / 2,
    y: y + 17,
    text: fitMapText(sized.group.label, sized.w - 12, 11, 600),
    size: 11,
    weight: 600,
    fill: 'muted',
    anchor: 'middle',
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
      y + metrics.head + Math.floor(index / sized.columns) * pitchY,
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
  const w = labelWidth(text, 10) + 12;
  labels.push({ kind: 'rect', x: x - w / 2, y: y - 8, w, h: 16, r: 4, fill: 'canvas' });
  labels.push({
    kind: 'text',
    x,
    y: y + 3.5,
    text,
    size: 10,
    weight: 400,
    fill: 'accent',
    anchor: 'middle',
  });
};

const networkText = (network: DiagramNetwork): string =>
  `${network.label} · ${network.memberCount.toLocaleString()} ${
    network.memberCount === 1 ? 'address' : 'addresses'
  }`;

// One pill per network, centred and wrapped. Returns the bottom of the band.
const drawNetworks = (
  out: MapCursor,
  networks: DiagramNetwork[],
  left: number,
  width: number,
  y: number,
  toneOf: (network: DiagramNetwork) => MapTone,
): number => {
  const shown = networks.slice(0, MAX_NETWORKS);
  const hidden = networks.length - shown.length;
  const single = networks.length === 1;
  const pills = shown.map((network) => {
    const text = fitMapText(
      single ? `Network · ${networkText(network)}` : networkText(network),
      width - 40,
      11,
      400,
    );
    return { text, tone: toneOf(network), w: labelWidth(text, 11) + 34 };
  });
  if (hidden > 0) {
    const text = `+${hidden} more`;
    pills.push({ text, tone: 'netOther', w: labelWidth(text, 11) + 34 });
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
    // A lone network is drawn as one bar across the layer, like the rail
    // everything below hangs from.
    const stretch = single ? Math.max(total, width) : total;
    let x = left + (width - stretch) / 2;
    for (const pill of entries) {
      const w = single ? stretch : pill.w;
      out.nodes.push({
        kind: 'rect',
        x,
        y: top,
        w,
        h: BAND_HEIGHT,
        r: BAND_HEIGHT / 2,
        fill: 'frame',
        stroke: pill.tone,
      });
      const textX = x + w / 2 + 7;
      out.nodes.push({
        kind: 'rect',
        x: textX - labelWidth(pill.text, 11) / 2 - 14,
        y: top + BAND_HEIGHT / 2 - 4,
        w: 8,
        h: 8,
        r: 4,
        fill: pill.tone,
      });
      out.nodes.push({
        kind: 'text',
        x: textX,
        y: top + 17,
        text: pill.text,
        size: 11,
        weight: 400,
        fill: 'text',
        anchor: 'middle',
      });
      x += w + 8;
    }
    top += BAND_HEIGHT + 6;
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

  const toneByNetwork = new Map<string, MapNetworkTone>();
  for (const network of diagram.networks) {
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

  if (diagram.internet) {
    const tile: DiagramTile = {
      id: 'internet',
      name: 'Internet',
      caption: 'not monitored',
      typeLabel: '',
      status: '',
      health: 'unknown',
      platform: 'internet',
      navigable: false,
    };
    drawTile(painter, tile, mid - metrics.tileW / 2, y, true);
    source = { x: mid, y: y + metrics.tileH };
    y += metrics.tileH + LAYER_GAP;
  }

  if (diagram.access) {
    const sized = sizeGroup(diagram.access, metrics, inner);
    const box = drawGroup(painter, sized, mid - sized.w / 2, y);
    if (source && diagram.internet) {
      line(
        lines,
        [
          [source.x, source.y],
          [mid, box.y],
        ],
        { arrow: true, dashed: true },
      );
      connectorLabel(labels, diagram.internet.label, mid, (source.y + box.y) / 2);
    }
    source = { x: mid, y: bottomOf(box) };
    y = bottomOf(box) + LAYER_GAP;
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
    if (source)
      line(
        lines,
        [
          [source.x, source.y],
          [mid, y],
        ],
        { arrow: true },
      );
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

  // Storage and backups share a layer.
  const lower = [diagram.storage, diagram.backups].filter(
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
        connectorLabel(
          labels,
          group.key === 'backups' ? 'backups' : 'disks and volumes',
          centreX(box),
          (collectY + box.y) / 2 + 3,
        );
      });
    }
    source = { x: centreX(lowerBoxes[0]), y: bottomOf(lowerBoxes[0]) };
    y = lowerBottom + LAYER_GAP;
  } else if (placed.length > 0) {
    source = null;
    y = appsBottom + LAYER_GAP;
  }

  if (diagram.hardware) {
    const sized = sizeGroup(diagram.hardware, metrics, inner);
    const box = drawGroup(painter, sized, mid - sized.w / 2, y);
    if (source && lower.length > 0) {
      const bendY = box.y - BUS_DROP;
      const points: Array<[number, number]> =
        Math.abs(source.x - mid) < 1
          ? [
              [source.x, source.y],
              [mid, box.y],
            ]
          : [
              [source.x, source.y],
              [source.x, bendY],
              [mid, bendY],
              [mid, box.y],
            ];
      line(lines, points, { arrow: true });
      connectorLabel(
        labels,
        'hosted on',
        (source.x + mid) / 2,
        Math.abs(source.x - mid) < 1 ? (source.y + box.y) / 2 : bendY,
      );
    } else if (placed.length > 0) {
      // No storage to pass through: what runs sits straight on the hardware.
      const collectY = appsBottom + BUS_DROP;
      const boxes = placed[placed.length - 1];
      for (const entry of boxes)
        line(lines, [
          [centreX(entry), bottomOf(entry)],
          [centreX(entry), collectY],
        ]);
      const xs = [...boxes.map(centreX), mid];
      line(lines, [
        [Math.min(...xs), collectY],
        [Math.max(...xs), collectY],
      ]);
      line(
        lines,
        [
          [mid, collectY],
          [mid, box.y],
        ],
        { arrow: true },
      );
      connectorLabel(labels, 'runs on', mid, (collectY + box.y) / 2 + 3);
    }
    y = bottomOf(box) + LAYER_GAP;
  }

  const height = drawPosterFooter(out, options.header, margin, width, Math.max(0, y - LAYER_GAP));
  return {
    width,
    height: Math.ceil(height),
    nodes: [...lines, ...out.nodes, ...labels, ...out.hits],
  };
}
