import { estimateTextWidth, formatBytes, formatCompactSpeed } from '@/utils/format';
import {
  drawPosterFooter,
  drawPosterHeader,
  fitMapName,
  fitMapText,
  mapHitMeta,
  type MapCursor,
  type MapHit,
  type MapInspectTarget,
  type MapLayoutOptions,
  type MapNetworkTone,
  type MapScene,
  type MapTone,
  type SceneNode,
} from './infrastructureMapLayout';
import {
  MAP_LINK_SATURATED_PERCENT,
  type MapHealth,
  type MapTrafficTier,
} from './infrastructureMapModel';
import {
  DIAGRAM_MAX_NETWORKS,
  formatMapLatency,
  formatMapLinkSpeed,
  type DiagramGroup,
  type DiagramHostFrame,
  type DiagramHostStack,
  type DiagramNetwork,
  type DiagramTile,
  type ServiceDiagram,
} from './infrastructureMapServices';

/**
 * Architectural host-framed layout for the Infrastructure Map.
 *
 * Reads top to bottom in four layers:
 *   1. Internet, ways in, and default gateways
 *   2. Named networks strip with link speeds and live throughput
 *   3. Architectural host frames containing Compose stacks, horizontal service
 *      cards (or compact icon tiles), and inline storage pool usage bars
 *   4. Bottom shared row for shared storage, backups, and standalone checks
 *
 * Every drawn mark uses Pulse's semantic theme tokens so the same scene renders
 * cleanly on the interactive page and in static SVG/PNG exports.
 */

const CARD_RADIUS = 8;
const INNER_RADIUS = 6;
const HOST_HEADER_H_NAMES = 52;
const HOST_HEADER_H_MARKS = 40;
const CLUSTER_HEADER_H = 24;
const CLUSTER_PAD = 10;
const CLUSTER_RADIUS = 10;
const CLUSTER_GAP = 12;
const CLUSTER_WRAP_GAP = 22;
const CATEGORY_SUBHEADER_H = 16;
const CATEGORY_GROUP_GAP = 8;
const GROUP_HEADER_H = 26;
const ROW_H = 44;
const ROW_GAP = 6;
const COL_GAP = 8;
const CARD_PAD = 10;
const STACK_HEADER_H = 22;
const STACK_PAD_X = 6;
const STACK_PAD_BOTTOM = 6;
const STACK_GAP = 8;
const POOL_BAR_H = 30;
const POOL_GAP = 6;
const OVERFLOW_BAR_H = 26;

const COMPACT_SIZE = 40;
const COMPACT_ICON = 29;
const COMPACT_GAP = 4;

const GROUP_GAP = 16;
const LAYER_GAP = 40;
const FRAME_ROW_GAP = 40;
const BUS_DROP = 28;
const GUTTER = 16;
const PILL_HEIGHT = 24;
const MIN_WIDTH = 300;

const NETWORK_TONES: readonly MapNetworkTone[] = ['net0', 'net1', 'net2', 'net3', 'net4', 'net5'];

const TRAFFIC_TIER_RANK: Record<MapTrafficTier, number> = {
  low: 1,
  medium: 2,
  high: 3,
};

/** Maps a reported interface speed (Mbps) to a connector stroke width. */
export const strokeWidthForLinkSpeed = (speedMbps?: number): number => {
  if (speedMbps === undefined || !(speedMbps > 0)) return 1.5;
  if (speedMbps >= 10000) return 2.75;
  if (speedMbps >= 2500) return 2.25;
  if (speedMbps >= 1000) return 1.75;
  return 1.25;
};

const textWidth = (text: string, size: number, weight: 400 | 500 | 600 = 600): number =>
  weight === 400
    ? estimateTextWidth('', { detail: text, fontPx: size })
    : estimateTextWidth(text, { fontPx: size });

const lettersOf = (name: string): string => {
  if (name.startsWith('+')) {
    const folded = /^\+\d+$/.test(name) ? Number(name.slice(1)) : 0;
    return folded >= 1000 ? `+${Math.floor(folded / 1000)}k` : name.slice(0, 4);
  }
  const parts = name.split(/[^A-Za-z0-9]+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).replace(/^./, (first) => first.toUpperCase());
  return (parts[0][0] + parts[1][0]).toUpperCase();
};

const IPV4_RE = /\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b/;

const extractLastOctet = (address?: string): string | undefined => {
  if (!address) return undefined;
  const match = IPV4_RE.exec(address);
  return match ? `.${match[4]}` : undefined;
};

const rightTopBadge = (tile: DiagramTile, inStack: boolean): string | undefined => {
  const baseType = tile.typeLabel.split(' · ').pop()?.trim() ?? '';
  if (tile.vmid !== undefined) {
    return `${baseType === 'VM' ? 'VM' : 'CT'} ${tile.vmid}`;
  }
  if (baseType === 'VM') return 'VM';
  if (baseType === 'LXC' || baseType === 'Jail') return 'CT';
  if (baseType === 'Pod') return 'Pod';
  if (baseType === 'Container' && !inStack && !/^Container\s+\d+$/.test(tile.name)) {
    return 'App';
  }
  return undefined;
};

const maxTrafficTier = (
  ...tiers: Array<MapTrafficTier | undefined>
): MapTrafficTier | undefined => {
  let best: MapTrafficTier | undefined;
  let bestRank = 0;
  for (const tier of tiers) {
    if (!tier) continue;
    const rank = TRAFFIC_TIER_RANK[tier];
    if (rank > bestRank) {
      best = tier;
      bestRank = rank;
    }
  }
  return best;
};

const groupTrafficTier = (group: DiagramGroup | undefined): MapTrafficTier | undefined =>
  group
    ? maxTrafficTier(
        ...group.allTiles.map((tile) => tile.trafficTier),
        ...group.tiles.map((tile) => tile.trafficTier),
      )
    : undefined;

const frameTrafficTier = (frame: DiagramHostFrame): MapTrafficTier | undefined =>
  maxTrafficTier(
    frame.trafficTier,
    frame.host.trafficTier,
    ...frame.allTiles.map((tile) => tile.trafficTier),
    ...frame.pools.map((pool) => pool.trafficTier),
  );

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

const centreX = (box: Box): number => box.x + box.w / 2;
const bottomOf = (box: Box): number => box.y + box.h;

interface HostPainter {
  out: MapCursor;
  density: MapLayoutOptions['density'];
  toneFor: (segmentId: string | undefined) => MapNetworkTone | undefined;
  liveTraffic: boolean;
  networkCount: number;
}

const buildHit = (
  tile: DiagramTile,
  options: {
    dashed?: boolean;
    groupKey?: string;
    inspectTarget?: MapInspectTarget;
  } = {},
): MapHit => {
  const real = tile.navigable;
  const dashed = Boolean(options.dashed);
  const inspectTarget: MapInspectTarget | undefined =
    options.inspectTarget ??
    (!real && !dashed && options.groupKey
      ? tile.id === 'other:off'
        ? { kind: 'group', groupKey: 'other', initialFilter: 'off' }
        : tile.id.endsWith(':more')
          ? { kind: 'group', groupKey: options.groupKey }
          : undefined
      : undefined);
  const segmentIds = tile.segmentIds?.length
    ? tile.segmentIds
    : tile.segmentId
      ? [tile.segmentId]
      : undefined;
  return {
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
    ...(tile.vmid !== undefined ? { vmid: tile.vmid } : {}),
    ...(tile.composeProject ? { composeProject: tile.composeProject } : {}),
    ...(tile.cpuCores !== undefined ? { cpuCores: tile.cpuCores } : {}),
    ...(tile.memoryTotalBytes !== undefined ? { memoryTotalBytes: tile.memoryTotalBytes } : {}),
    ...(tile.versionLabel ? { versionLabel: tile.versionLabel } : {}),
    ...(tile.usedPercent !== undefined ? { usedPercent: tile.usedPercent } : {}),
    ...(tile.sizeLabel ? { sizeLabel: tile.sizeLabel } : {}),
    ...(tile.vdevLayout ? { vdevLayout: tile.vdevLayout } : {}),
    ...(tile.netRxBytes !== undefined ? { netRxBytes: tile.netRxBytes } : {}),
    ...(tile.netTxBytes !== undefined ? { netTxBytes: tile.netTxBytes } : {}),
    ...(tile.latencyMs !== undefined ? { latencyMs: tile.latencyMs } : {}),
    ...(tile.linkSpeedMbps !== undefined ? { linkSpeedMbps: tile.linkSpeedMbps } : {}),
    ...(tile.linkUtilizationPercent !== undefined
      ? { linkUtilizationPercent: tile.linkUtilizationPercent }
      : {}),
    ...(tile.linkDownCount !== undefined ? { linkDownCount: tile.linkDownCount } : {}),
    ...(tile.firewallEnabled ? { firewallEnabled: true } : {}),
    ...(tile.dockerNetworks?.length ? { dockerNetworks: tile.dockerNetworks } : {}),
    ...(tile.probeSummary ? { probeSummary: tile.probeSummary } : {}),
    ...(tile.certWarning ? { certWarning: tile.certWarning } : {}),
    ...(segmentIds ? { segmentIds } : {}),
    address: tile.address,
    detail: tile.detail,
    navigable: real,
    ...(tile.links?.length ? { links: tile.links } : {}),
    ...(tile.checkAddress ? { checkAddress: tile.checkAddress } : {}),
    ...(inspectTarget ? { inspectTarget } : {}),
  };
};

const drawHorizontalTile = (
  painter: HostPainter,
  tile: DiagramTile,
  x: number,
  y: number,
  w: number,
  options: { dashed?: boolean; inStack?: boolean; groupKey?: string } = {},
) => {
  const { out } = painter;
  const off = tile.health === 'off';
  const dashed = Boolean(options.dashed);
  const inStack = Boolean(options.inStack);

  if (dashed) {
    out.nodes.push({
      kind: 'rect',
      x,
      y,
      w,
      h: ROW_H,
      r: INNER_RADIUS,
      fill: 'none',
      stroke: 'border',
      dashed: true,
    });
  } else {
    out.nodes.push({
      kind: 'rect',
      x,
      y,
      w,
      h: ROW_H,
      r: INNER_RADIUS,
      fill: 'surfaceAlt',
      opacity: 0.45,
      stroke: 'border',
    });
    if (painter.liveTraffic && tile.trafficTier) {
      out.nodes.push({
        kind: 'rect',
        x,
        y,
        w,
        h: ROW_H,
        r: INNER_RADIUS,
        fill: 'none',
        activity: tile.trafficTier,
      });
    }
  }

  const tone = painter.toneFor(tile.segmentId);
  if (tone && !dashed) {
    out.nodes.push({
      kind: 'rect',
      x: x + 3,
      y: y + 4,
      w: 3.5,
      h: ROW_H - 8,
      r: 1.75,
      fill: tone,
    });
  }

  const iconSize = 26;
  const iconX = x + (tone && !dashed ? 10 : 8);
  const iconY = y + (ROW_H - iconSize) / 2;

  if (tile.platform) {
    const glyphSize = 20;
    out.nodes.push({
      kind: 'icon',
      x: iconX + (iconSize - glyphSize) / 2,
      y: iconY + (iconSize - glyphSize) / 2,
      size: glyphSize,
      platform: tile.platform,
    });
  } else {
    out.nodes.push({
      kind: 'app',
      x: iconX,
      y: iconY,
      size: iconSize,
      ...(tile.icon ? { icon: tile.icon } : {}),
      letters: lettersOf(tile.name),
      ...(off ? { dim: true } : {}),
    });
  }

  if (tile.navigable) {
    const dotX = iconX + iconSize - 1;
    const dotY = iconY + 1;
    out.nodes.push({ kind: 'rect', x: dotX - 5, y: dotY - 5, w: 10, h: 10, r: 5, fill: 'surface' });
    out.nodes.push({ kind: 'dot', x: dotX, y: dotY, r: 3.5, health: tile.health });
  }

  const topBadge = dashed ? undefined : rightTopBadge(tile, inStack);
  const bottomBadge = dashed ? undefined : extractLastOctet(tile.address);
  const rightX = x + w - 8;
  const rightColW = Math.max(
    topBadge ? textWidth(topBadge, 10, 500) : 0,
    bottomBadge ? textWidth(bottomBadge, 10, 400) : 0,
  );

  if (topBadge) {
    out.nodes.push({
      kind: 'text',
      x: rightX,
      y: y + 18,
      text: topBadge,
      size: 10,
      weight: 500,
      fill: 'muted',
      anchor: 'end',
      ...(off ? { dim: true } : {}),
    });
  }
  if (bottomBadge) {
    out.nodes.push({
      kind: 'text',
      x: rightX,
      y: y + 32,
      text: bottomBadge,
      size: 10,
      weight: 400,
      fill: 'muted',
      anchor: 'end',
      ...(off ? { dim: true } : {}),
    });
  }

  const textX = iconX + iconSize + 8;
  const textMaxW = Math.max(36, rightX - textX - (rightColW > 0 ? rightColW + 6 : 0));

  const liveSpeedText =
    painter.liveTraffic && tile.trafficTier && tile.netTotalBytes !== undefined
      ? `${formatCompactSpeed(tile.netTotalBytes)}/s`
      : undefined;

  let subText = '';
  let subAccent = false;
  if (tile.hasConnectionCaption) {
    if (liveSpeedText) {
      const combined = `${tile.caption} · ${liveSpeedText}`;
      if (textWidth(combined, 10, 500) <= textMaxW) {
        subText = combined;
        subAccent = true;
      } else {
        subText = fitMapName(tile.caption, textMaxW, 10, 400);
      }
    } else {
      subText = fitMapName(tile.caption, textMaxW, 10, 400);
    }
  } else if (liveSpeedText) {
    subText = fitMapText(liveSpeedText, textMaxW, 10, 500);
    subAccent = true;
  } else if (tile.probeSummary) {
    const checkSummary = [
      tile.probeSummary,
      tile.latencyMs !== undefined ? formatMapLatency(tile.latencyMs) : '',
      tile.certWarning ?? '',
    ]
      .filter(Boolean)
      .join(' · ');
    subText = fitMapText(checkSummary, textMaxW, 10, 400);
  } else {
    const fallbackCaption =
      tile.detail ||
      (tile.caption && tile.caption !== tile.hostName ? tile.caption : '') ||
      tile.dockerNetworks?.[0] ||
      '';
    subText = fallbackCaption ? fitMapName(fallbackCaption, textMaxW, 10, 400) : '';
  }

  if (tile.detail && subText && !subText.includes(tile.detail)) {
    const withPort = `${subText} · ${tile.detail}`;
    if (textWidth(withPort, 10, subAccent ? 500 : 400) <= textMaxW) {
      subText = withPort;
    }
  }

  out.nodes.push({
    kind: 'text',
    x: textX,
    y: subText ? y + 18 : y + 26,
    text: fitMapName(tile.name, textMaxW, 12, 600),
    size: 12,
    weight: 600,
    fill: off ? 'muted' : 'text',
    ...(off ? { dim: true } : {}),
  });

  if (subText) {
    out.nodes.push({
      kind: 'text',
      x: textX,
      y: y + 32,
      text: subText,
      size: 10,
      weight: subAccent ? 500 : 400,
      fill: subAccent ? 'accent' : 'muted',
      ...(off && !subAccent ? { dim: true } : {}),
    });
  }

  if (dashed && !tile.links?.length && !tile.checkAddress) return;

  out.hits.push({
    kind: 'hit',
    x,
    y,
    w,
    h: ROW_H,
    r: INNER_RADIUS,
    hit: buildHit(tile, { dashed, groupKey: options.groupKey }),
  });
};

const drawCompactTile = (
  painter: HostPainter,
  tile: DiagramTile,
  x: number,
  y: number,
  options: { dashed?: boolean; groupKey?: string } = {},
) => {
  const { out } = painter;
  const off = tile.health === 'off';
  const dashed = Boolean(options.dashed);
  const cx = x + COMPACT_SIZE / 2;
  const iconY = y + (COMPACT_SIZE - COMPACT_ICON) / 2;

  if (dashed) {
    out.nodes.push({
      kind: 'rect',
      x,
      y,
      w: COMPACT_SIZE,
      h: COMPACT_SIZE,
      r: INNER_RADIUS,
      fill: 'none',
      stroke: 'border',
      dashed: true,
    });
  } else if (painter.liveTraffic && tile.trafficTier) {
    out.nodes.push({
      kind: 'rect',
      x,
      y,
      w: COMPACT_SIZE,
      h: COMPACT_SIZE,
      r: INNER_RADIUS,
      fill: 'none',
      activity: tile.trafficTier,
    });
  }

  if (tile.platform) {
    const size = COMPACT_ICON - 10;
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
      x: cx - COMPACT_ICON / 2,
      y: iconY,
      size: COMPACT_ICON,
      ...(tile.icon ? { icon: tile.icon } : {}),
      letters: lettersOf(tile.name),
      ...(off ? { dim: true } : {}),
    });
  }

  const tone = painter.toneFor(tile.segmentId);
  if (tone && !dashed) {
    out.nodes.push({
      kind: 'rect',
      x: x + 1.5,
      y: y + 6,
      w: 3,
      h: 10,
      r: 1.5,
      fill: tone,
    });
  }

  if (dashed && !tile.links?.length && !tile.checkAddress) return;

  if (tile.navigable) {
    const dotX = cx + COMPACT_ICON / 2 - 1;
    const dotY = iconY + 1;
    out.nodes.push({ kind: 'rect', x: dotX - 6, y: dotY - 6, w: 12, h: 12, r: 6, fill: 'surface' });
    out.nodes.push({ kind: 'dot', x: dotX, y: dotY, r: 4, health: tile.health });
  }

  out.hits.push({
    kind: 'hit',
    x,
    y,
    w: COMPACT_SIZE,
    h: COMPACT_SIZE,
    r: 4,
    hit: buildHit(tile, { dashed, groupKey: options.groupKey }),
  });
};

const drawOverflowBar = (
  out: MapCursor,
  x: number,
  y: number,
  w: number,
  count: number,
  groupKey: string,
  label?: string,
) => {
  const text = label ?? `+${count.toLocaleString()} more`;
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w,
    h: OVERFLOW_BAR_H,
    r: INNER_RADIUS,
    fill: 'surfaceAlt',
    opacity: 0.45,
    stroke: 'border',
    dashed: true,
  });
  out.nodes.push({
    kind: 'text',
    x: x + w / 2,
    y: y + 17,
    text: fitMapText(text, w - 16, 11, 500),
    size: 11,
    weight: 500,
    fill: 'muted',
    anchor: 'middle',
  });
  out.hits.push({
    kind: 'hit',
    x,
    y,
    w,
    h: OVERFLOW_BAR_H,
    r: INNER_RADIUS,
    hit: {
      id: `${groupKey}:more`,
      name: `+${count}`,
      meta: `${count.toLocaleString()} more`,
      health: 'off',
      navigable: false,
      inspectTarget: { kind: 'group', groupKey },
    },
  });
};

const drawStorageBar = (
  painter: HostPainter,
  tile: DiagramTile,
  x: number,
  y: number,
  w: number,
  groupKey?: string,
) => {
  const { out } = painter;
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w,
    h: POOL_BAR_H,
    r: 5,
    fill: 'surfaceAlt',
    opacity: 0.45,
    stroke: 'border',
  });

  out.nodes.push({
    kind: 'icon',
    x: x + 8,
    y: y + 8,
    size: 14,
    platform: tile.platform ?? 'storage',
  });

  const hasPercent = tile.usedPercent !== undefined && Number.isFinite(tile.usedPercent);
  let rightReserved = 10;

  if (hasPercent) {
    const pct = Math.max(0, Math.min(100, tile.usedPercent!));
    const pctText = `${Math.round(pct)}%`;
    const pctW = Math.max(24, textWidth(pctText, 10, 500));
    const trackW = w >= 208 ? 68 : Math.max(36, Math.min(68, w - 132));
    const trackX = x + w - 8 - pctW - 6 - trackW;
    const trackY = y + 12;

    out.nodes.push({
      kind: 'rect',
      x: trackX,
      y: trackY,
      w: trackW,
      h: 6,
      r: 3,
      fill: 'border',
    });

    if (pct > 0) {
      const fillW = Math.min(trackW, Math.max(3, (trackW * pct) / 100));
      const barTone: MapHealth =
        pct >= 90 || tile.health === 'critical'
          ? 'critical'
          : pct >= 75 || tile.health === 'attention'
            ? 'attention'
            : 'ok';
      out.nodes.push({
        kind: 'rect',
        x: trackX,
        y: trackY,
        w: fillW,
        h: 6,
        r: 3,
        fill: barTone,
      });
    }

    out.nodes.push({
      kind: 'text',
      x: x + w - 8,
      y: y + 19,
      text: pctText,
      size: 10,
      weight: 500,
      fill: 'muted',
      anchor: 'end',
    });

    rightReserved = x + w - trackX + 8;
  } else {
    out.nodes.push({
      kind: 'dot',
      x: x + w - 12,
      y: y + 15,
      r: 3.5,
      health: tile.health,
    });
    rightReserved = 24;
  }

  const textStart = x + 28;
  const textBudget = Math.max(40, w - 28 - rightReserved);
  const nameText = fitMapName(tile.name, textBudget, 11, 600);
  out.nodes.push({
    kind: 'text',
    x: textStart,
    y: y + 19,
    text: nameText,
    size: 11,
    weight: 600,
    fill: tile.health === 'off' ? 'muted' : 'text',
    ...(tile.health === 'off' ? { dim: true } : {}),
  });

  const metaParts = [tile.vdevLayout, tile.sizeLabel].filter(Boolean).join(' · ');
  if (metaParts) {
    const nameUsedW = textWidth(nameText, 11, 600);
    const remW = textBudget - nameUsedW - 6;
    if (remW >= 28) {
      out.nodes.push({
        kind: 'text',
        x: textStart + nameUsedW + 6,
        y: y + 19,
        text: fitMapText(metaParts, remW, 10, 400),
        size: 10,
        weight: 400,
        fill: 'muted',
      });
    }
  }

  out.hits.push({
    kind: 'hit',
    x,
    y,
    w,
    h: POOL_BAR_H,
    r: 5,
    hit: buildHit(tile, { groupKey }),
  });
};

const hostSpecParts = (frame: DiagramHostFrame): string => {
  const parts: string[] = [];
  if (frame.host.cpuCores !== undefined && frame.host.cpuCores > 0) {
    parts.push(`${frame.host.cpuCores}c`);
  }
  if (frame.host.memoryTotalBytes !== undefined && frame.host.memoryTotalBytes > 0) {
    parts.push(formatBytes(frame.host.memoryTotalBytes, 0));
  }
  const hasMeters = frame.host.cpu !== undefined || frame.host.memory !== undefined;
  const speed = frame.linkSpeedMbps ?? frame.host.linkSpeedMbps;
  if (!hasMeters && speed !== undefined && speed > 0) {
    parts.push(formatMapLinkSpeed(speed));
  }
  if (frame.host.versionLabel) {
    parts.push(frame.host.versionLabel);
  }
  return parts.join(' · ');
};

const hostHeaderH = (density: MapLayoutOptions['density']): number =>
  density === 'marks' ? HOST_HEADER_H_MARKS : HOST_HEADER_H_NAMES;

const hostSingleNetworkTone = (
  frame: DiagramHostFrame,
  networkCount: number,
  toneFor: (segmentId: string | undefined) => MapNetworkTone | undefined,
): MapNetworkTone | undefined => {
  if (networkCount <= 1) return undefined;
  const hostSegIds = frame.host.segmentIds?.length
    ? frame.host.segmentIds
    : frame.host.segmentId
      ? [frame.host.segmentId]
      : [];
  return hostSegIds.length === 1 ? toneFor(hostSegIds[0]) : undefined;
};

const meterTone = (pct: number): MapHealth =>
  pct >= 90 ? 'critical' : pct >= 75 ? 'attention' : 'ok';

interface LooseCategoryGroup {
  label: string;
  tiles: DiagramTile[];
}

const groupLooseTilesByCategory = (
  looseTiles: DiagramTile[],
  density: MapLayoutOptions['density'],
): LooseCategoryGroup[] | null => {
  if (density !== 'names' || looseTiles.length < 5) return null;
  const byCategory = new Map<string, DiagramTile[]>();
  for (const tile of looseTiles) {
    const label = tile.categoryLabel?.trim() || 'Other';
    const bucket = byCategory.get(label);
    if (bucket) bucket.push(tile);
    else byCategory.set(label, [tile]);
  }
  if (byCategory.size < 2) return null;
  return [...byCategory.entries()]
    .sort(([a], [b]) => {
      if (a === 'Other' && b !== 'Other') return 1;
      if (b === 'Other' && a !== 'Other') return -1;
      return a.localeCompare(b);
    })
    .map(([label, tiles]) => ({ label, tiles }));
};

interface PlacedStack {
  stack: DiagramHostStack;
  span: number;
  cols: number;
  w: number;
  h: number;
}

interface SizedHostFrame {
  frame: DiagramHostFrame;
  w: number;
  h: number;
  columns: number;
  colW: number;
  stackRows: PlacedStack[][];
  looseCategories: LooseCategoryGroup[] | null;
  poolColumns: number;
}

const packIntoRows = <T extends { w: number }>(
  items: T[],
  maxWidth: number,
  gap: number,
): T[][] => {
  const fill = (limit: number): T[][] => {
    const rows: T[][] = [];
    let row: T[] = [];
    let used = 0;
    for (const item of items) {
      const next = used === 0 ? item.w : used + gap + item.w;
      if (row.length > 0 && next > limit) {
        rows.push(row);
        row = [item];
        used = item.w;
      } else {
        row.push(item);
        used = next;
      }
    }
    if (row.length > 0) rows.push(row);
    return rows;
  };
  const rows = fill(maxWidth);
  if (rows.length < 2) return rows;
  let low = Math.max(...items.map((item) => item.w));
  let high = maxWidth;
  while (high - low > 4) {
    const middle = (low + high) / 2;
    if (fill(middle).length <= rows.length) high = middle;
    else low = middle;
  }
  return fill(high);
};

const sizeHostFrame = (
  frame: DiagramHostFrame,
  density: MapLayoutOptions['density'],
  inner: number,
  liveTraffic: boolean,
  inCluster = false,
): SizedHostFrame => {
  const headerH = hostHeaderH(density);
  const stackTileCount = frame.stacks.reduce((sum, stack) => sum + stack.tiles.length, 0);
  const displayedWorkloads = stackTileCount + frame.looseTiles.length;
  const specRaw = hostSpecParts(frame);
  const rateRaw =
    liveTraffic && frame.host.trafficTier && frame.host.netTotalBytes !== undefined
      ? `${formatCompactSpeed(frame.host.netTotalBytes)}/s`
      : '';
  const saturated =
    frame.host.linkUtilizationPercent !== undefined &&
    frame.host.linkUtilizationPercent >= MAP_LINK_SATURATED_PERCENT;
  const busyText = saturated ? `BUSY ${Math.round(frame.host.linkUtilizationPercent!)}%` : '';
  const downText =
    frame.host.linkDownCount !== undefined && frame.host.linkDownCount > 0
      ? `${frame.host.linkDownCount} DOWN`
      : '';
  const countLabel =
    frame.totalCount > 0 ? `${frame.totalCount} ${frame.totalCount === 1 ? 'item' : 'items'}` : '';

  const row1DesiredW =
    42 +
    textWidth(frame.host.name, 13, 600) +
    18 +
    (!inCluster && frame.clusterName ? textWidth(`· ${frame.clusterName}`, 10.5, 400) + 10 : 0) +
    (countLabel ? Math.max(22, textWidth(countLabel, 10, 500) + 12) + 8 : 0) +
    (rateRaw ? textWidth(rateRaw, 11, 600) + 8 : 0) +
    (busyText ? textWidth(busyText, 9.5, 600) + 18 : 0) +
    (downText ? textWidth(downText, 9.5, 600) + 18 : 0);

  const meterCount =
    Number(frame.host.cpu !== undefined && Number.isFinite(frame.host.cpu)) +
    Number(frame.host.memory !== undefined && Number.isFinite(frame.host.memory));
  const row2DesiredW =
    density === 'names'
      ? 24 + (specRaw ? textWidth(specRaw, 10, 400) + 14 : 0) + meterCount * 78
      : 0;
  const headerDesiredW = Math.max(row1DesiredW, row2DesiredW);

  if (density === 'marks') {
    const pitch = COMPACT_SIZE + COMPACT_GAP;
    const maxCols = Math.max(1, Math.floor((inner - CARD_PAD * 2 + COMPACT_GAP) / pitch));
    const wanted =
      displayedWorkloads <= 6
        ? Math.max(4, displayedWorkloads)
        : Math.ceil(Math.sqrt(displayedWorkloads * 2.5));
    const columns = Math.max(1, Math.min(maxCols, wanted));
    const minCardW = Math.min(inner, Math.max(228, Math.min(340, headerDesiredW)));
    const bodyW = Math.max(columns * pitch - COMPACT_GAP, minCardW - CARD_PAD * 2);
    const cardW = Math.min(inner, bodyW + CARD_PAD * 2);
    const actualBodyW = cardW - CARD_PAD * 2;
    const actualCols = Math.max(1, Math.floor((actualBodyW + COMPACT_GAP) / pitch));

    const placedStacks: PlacedStack[] = frame.stacks.map((stack) => {
      const innerMaxCols = Math.max(
        1,
        Math.floor((actualBodyW - STACK_PAD_X * 2 + COMPACT_GAP) / pitch),
      );
      const stackCols = Math.max(1, Math.min(innerMaxCols, stack.tiles.length));
      const rows = Math.ceil(stack.tiles.length / stackCols);
      const titleW = textWidth(stack.label.toUpperCase(), 10, 600) + 32;
      const stackW = Math.min(
        actualBodyW,
        Math.max(stackCols * pitch - COMPACT_GAP + STACK_PAD_X * 2, titleW),
      );
      const stackH =
        STACK_HEADER_H +
        rows * COMPACT_SIZE +
        Math.max(0, rows - 1) * COMPACT_GAP +
        STACK_PAD_BOTTOM;
      return { stack, span: 1, cols: stackCols, w: stackW, h: stackH };
    });

    const stackRows = packIntoRows(placedStacks, actualBodyW, STACK_GAP);
    let h = headerH;
    const hasWorkloads = displayedWorkloads > 0 || frame.overflowCount > 0;
    if (hasWorkloads) {
      h += CARD_PAD;
      if (stackRows.length > 0) {
        for (const row of stackRows) {
          h += Math.max(...row.map((entry) => entry.h)) + STACK_GAP;
        }
        if (frame.looseTiles.length === 0 && frame.overflowCount === 0) h -= STACK_GAP;
      }
      if (frame.looseTiles.length > 0) {
        const looseRows = Math.ceil(frame.looseTiles.length / actualCols);
        h += looseRows * COMPACT_SIZE + Math.max(0, looseRows - 1) * COMPACT_GAP;
        if (frame.overflowCount > 0) h += STACK_GAP;
      }
      if (frame.overflowCount > 0) {
        h += OVERFLOW_BAR_H;
      }
      h += CARD_PAD;
    }

    const poolColumns = frame.pools.length >= 2 && actualBodyW >= 440 ? 2 : 1;
    if (frame.pools.length > 0) {
      if (hasWorkloads) h += 1; // hairline divider
      h += 8 + 15; // top gap + STORAGE sub-label
      const poolRows = Math.ceil(frame.pools.length / poolColumns);
      h += poolRows * POOL_BAR_H + Math.max(0, poolRows - 1) * POOL_GAP + CARD_PAD;
    }

    return {
      frame,
      w: cardW,
      h,
      columns: actualCols,
      colW: COMPACT_SIZE,
      stackRows,
      looseCategories: null,
      poolColumns,
    };
  }

  // Named mode: Goblinmine-inspired horizontal service cards
  const baseRowW = inner < 520 ? Math.min(216, Math.max(180, inner - CARD_PAD * 2)) : 228;
  const maxFitCols = Math.max(
    1,
    Math.floor((inner - CARD_PAD * 2 + COL_GAP) / (baseRowW + COL_GAP)),
  );
  const desiredCols = displayedWorkloads >= 15 ? 3 : displayedWorkloads >= 5 ? 2 : 1;
  const columns = Math.max(1, Math.min(maxFitCols, desiredCols));
  const rawBodyW = columns * baseRowW + (columns - 1) * COL_GAP;
  const minWForHeader =
    columns === 1
      ? Math.min(inner, Math.min(460, Math.max(baseRowW + CARD_PAD * 2, headerDesiredW)))
      : 0;
  const cardW = Math.min(inner, Math.max(rawBodyW + CARD_PAD * 2, minWForHeader));
  const bodyW = cardW - CARD_PAD * 2;
  const colW = (bodyW - (columns - 1) * COL_GAP) / columns;
  const looseCategories = groupLooseTilesByCategory(frame.looseTiles, density);

  const stackRows: PlacedStack[][] = [];
  if (frame.stacks.length > 0) {
    let currentRow: PlacedStack[] = [];
    let usedSpan = 0;
    for (const stack of frame.stacks) {
      const span =
        columns === 1
          ? 1
          : stack.tiles.length > 6
            ? Math.min(columns, Math.ceil(stack.tiles.length / 5))
            : 1;
      const stackW = span * colW + (span - 1) * COL_GAP;
      const rows = Math.ceil(stack.tiles.length / span);
      const stackH =
        STACK_HEADER_H + rows * ROW_H + Math.max(0, rows - 1) * ROW_GAP + STACK_PAD_BOTTOM;
      const entry: PlacedStack = { stack, span, cols: span, w: stackW, h: stackH };
      if (currentRow.length > 0 && usedSpan + span > columns) {
        stackRows.push(currentRow);
        currentRow = [entry];
        usedSpan = span;
      } else {
        currentRow.push(entry);
        usedSpan += span;
      }
    }
    if (currentRow.length > 0) stackRows.push(currentRow);
  }

  let h = headerH;
  const hasWorkloads = displayedWorkloads > 0 || frame.overflowCount > 0;
  if (hasWorkloads) {
    h += CARD_PAD;
    if (stackRows.length > 0) {
      for (const row of stackRows) {
        h += Math.max(...row.map((entry) => entry.h)) + STACK_GAP;
      }
      if (frame.looseTiles.length === 0 && frame.overflowCount === 0) h -= STACK_GAP;
    }
    if (looseCategories) {
      looseCategories.forEach((category, idx) => {
        const rows = Math.ceil(category.tiles.length / columns);
        h += CATEGORY_SUBHEADER_H + rows * ROW_H + Math.max(0, rows - 1) * ROW_GAP;
        if (idx < looseCategories.length - 1) h += CATEGORY_GROUP_GAP;
      });
      if (frame.overflowCount > 0) h += STACK_GAP;
    } else if (frame.looseTiles.length > 0) {
      const looseRows = Math.ceil(frame.looseTiles.length / columns);
      h += looseRows * ROW_H + Math.max(0, looseRows - 1) * ROW_GAP;
      if (frame.overflowCount > 0) h += STACK_GAP;
    }
    if (frame.overflowCount > 0) {
      h += OVERFLOW_BAR_H;
    }
    h += CARD_PAD;
  }

  const poolColumns = columns >= 2 && frame.pools.length >= 2 && bodyW >= 440 ? 2 : 1;
  if (frame.pools.length > 0) {
    if (hasWorkloads) h += 1;
    h += 8 + 15;
    const poolRows = Math.ceil(frame.pools.length / poolColumns);
    h += poolRows * POOL_BAR_H + Math.max(0, poolRows - 1) * POOL_GAP + CARD_PAD;
  }

  return {
    frame,
    w: cardW,
    h,
    columns,
    colW,
    stackRows,
    looseCategories,
    poolColumns,
  };
};

const drawHostFrame = (
  painter: HostPainter,
  sized: SizedHostFrame,
  x: number,
  y: number,
  inCluster = false,
): Box => {
  const { out } = painter;
  const { frame, w, h } = sized;
  const headerH = hostHeaderH(painter.density);
  const isNamed = painter.density === 'names';
  const hasBody = h > headerH;

  // Outer architectural host card
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w,
    h,
    r: CARD_RADIUS,
    fill: 'surface',
    stroke: 'border',
  });

  // Header strip fill
  out.nodes.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + 0.5,
    w: w - 1,
    h: headerH - (hasBody ? 0.5 : 1),
    r: CARD_RADIUS - 0.5,
    fill: 'surfaceAlt',
  });
  if (hasBody) {
    out.nodes.push({
      kind: 'rect',
      x: x + 0.5,
      y: y + headerH / 2,
      w: w - 1,
      h: headerH / 2,
      r: 0,
      fill: 'surfaceAlt',
    });
    out.nodes.push({
      kind: 'rect',
      x,
      y: y + headerH,
      w,
      h: 1,
      r: 0,
      fill: 'border',
    });
  }

  // Left vertical network accent bar if host sits on a single named network
  const hostTone = hostSingleNetworkTone(frame, painter.networkCount, painter.toneFor);
  if (hostTone) {
    out.nodes.push({
      kind: 'rect',
      x: x + 1,
      y: y + 4,
      w: 3,
      h: headerH - 8,
      r: 1.5,
      fill: hostTone,
    });
  }

  const row1IconY = isNamed ? y + 6 : y + 12;
  const row1TextY = isNamed ? y + 17 : y + 24;
  const row1DotY = isNamed ? y + 13 : y + 20;
  const row1PillY = isNamed ? y + 5 : y + 11.5;

  const iconX = x + (hostTone ? 11 : 10);
  out.nodes.push({
    kind: 'icon',
    x: iconX,
    y: row1IconY,
    size: 16,
    platform: frame.host.platform ?? 'machine',
  });

  const leftStart = iconX + 22;
  let rightCursor = x + w - 8;
  const minNameReserve = Math.min(textWidth(frame.host.name, 13, 600), 56) + 16;

  // 1. Clickable workload count pill (always placed at far right; excluded from host header hit)
  let hitW = w;
  if (frame.totalCount > 0) {
    const countLabel =
      w >= 290
        ? `${frame.totalCount} ${frame.totalCount === 1 ? 'item' : 'items'}`
        : String(frame.totalCount);
    const pillW = Math.max(22, textWidth(countLabel, 10, 500) + 12);
    const pillH = 17;
    const pillX = rightCursor - pillW;
    if (pillX > leftStart + 44) {
      out.nodes.push({
        kind: 'rect',
        x: pillX,
        y: row1PillY,
        w: pillW,
        h: pillH,
        r: 4,
        fill: 'surface',
        stroke: 'border',
      });
      out.nodes.push({
        kind: 'text',
        x: pillX + pillW / 2,
        y: row1PillY + 12,
        text: countLabel,
        size: 10,
        weight: 500,
        fill: 'muted',
        anchor: 'middle',
      });
      out.inspects?.push({
        kind: 'inspect',
        x: pillX,
        y: row1PillY,
        w: pillW,
        h: pillH,
        r: 4,
        label: `Inspect ${frame.host.name} (${frame.totalCount})`,
        target: { kind: 'group', groupKey: `host:${frame.id}` },
      });
      hitW = Math.max(24, pillX - x - 2);
      rightCursor = pillX - 6;
    }
  }

  // 2. Live throughput readout
  const liveRate =
    painter.liveTraffic && frame.host.trafficTier && frame.host.netTotalBytes !== undefined
      ? `${formatCompactSpeed(frame.host.netTotalBytes)}/s`
      : undefined;
  if (liveRate) {
    const rateW = textWidth(liveRate, 11, 600);
    if (rightCursor - rateW > leftStart + minNameReserve) {
      out.nodes.push({
        kind: 'text',
        x: rightCursor,
        y: row1TextY,
        text: liveRate,
        size: 11,
        weight: 600,
        fill: 'accent',
        anchor: 'end',
      });
      rightCursor -= rateW + 6;
    }
  }

  // 3. Inline amber BUSY pill when link utilization >= MAP_LINK_SATURATED_PERCENT
  if (
    frame.host.linkUtilizationPercent !== undefined &&
    frame.host.linkUtilizationPercent >= MAP_LINK_SATURATED_PERCENT
  ) {
    const busyText = `BUSY ${Math.round(frame.host.linkUtilizationPercent)}%`;
    const busyW = textWidth(busyText, 9.5, 600) + 10;
    const busyH = 16;
    if (rightCursor - busyW > leftStart + minNameReserve) {
      const bx = rightCursor - busyW;
      out.nodes.push({
        kind: 'rect',
        x: bx,
        y: row1PillY,
        w: busyW,
        h: busyH,
        r: 4,
        fill: 'attention',
        opacity: 0.18,
        stroke: 'attention',
      });
      out.nodes.push({
        kind: 'text',
        x: bx + busyW / 2,
        y: row1PillY + 11.5,
        text: busyText,
        size: 9.5,
        weight: 600,
        fill: 'attention',
        anchor: 'middle',
      });
      rightCursor = bx - 6;
    }
  }

  // 4. Inline amber link-down pill when any physical interface is down
  if (frame.host.linkDownCount !== undefined && frame.host.linkDownCount > 0) {
    const downText = `${frame.host.linkDownCount} DOWN`;
    const downW = textWidth(downText, 9.5, 600) + 10;
    const downH = 16;
    if (rightCursor - downW > leftStart + minNameReserve) {
      const dx = rightCursor - downW;
      out.nodes.push({
        kind: 'rect',
        x: dx,
        y: row1PillY,
        w: downW,
        h: downH,
        r: 4,
        fill: 'attention',
        opacity: 0.18,
        stroke: 'attention',
      });
      out.nodes.push({
        kind: 'text',
        x: dx + downW / 2,
        y: row1PillY + 11.5,
        text: downText,
        size: 9.5,
        weight: 600,
        fill: 'attention',
        anchor: 'middle',
      });
      rightCursor = dx - 6;
    }
  }

  // Row 1 left: host name, status dot, and standalone cluster suffix
  const leftRoom = Math.max(40, rightCursor - leftStart);
  const fittedHostName = fitMapName(frame.host.name, Math.max(32, leftRoom - 14), 13, 600);
  out.nodes.push({
    kind: 'text',
    x: leftStart,
    y: row1TextY,
    text: fittedHostName,
    size: 13,
    weight: 600,
    fill: frame.host.health === 'off' ? 'muted' : 'text',
    ...(frame.host.health === 'off' ? { dim: true } : {}),
  });

  const actualNameW = textWidth(fittedHostName, 13, 600);
  const dotX = leftStart + actualNameW + 8;
  out.nodes.push({
    kind: 'dot',
    x: dotX,
    y: row1DotY,
    r: 3.5,
    health: frame.host.health,
  });

  if (!inCluster && frame.clusterName) {
    const clusterStart = dotX + 9;
    const clusterRoom = rightCursor - clusterStart - 4;
    if (clusterRoom >= 32) {
      out.nodes.push({
        kind: 'text',
        x: clusterStart,
        y: row1TextY,
        text: fitMapText(`· ${frame.clusterName}`, clusterRoom, 10.5, 400),
        size: 10.5,
        weight: 400,
        fill: 'muted',
      });
    }
  }

  // Row 2 (names mode): hardware specs on the left, live CPU & MEM micro-meters on the right
  if (isNamed) {
    const row2Y = y + 39;
    const row2Left = x + (hostTone ? 11 : 10);
    let row2Right = x + w - 8;

    const meters: Array<{ label: string; pct: number }> = [];
    if (frame.host.cpu !== undefined && Number.isFinite(frame.host.cpu)) {
      meters.push({ label: 'CPU', pct: Math.max(0, Math.min(100, Math.round(frame.host.cpu))) });
    }
    if (frame.host.memory !== undefined && Number.isFinite(frame.host.memory)) {
      meters.push({ label: 'MEM', pct: Math.max(0, Math.min(100, Math.round(frame.host.memory))) });
    }

    if (meters.length > 0) {
      if (w >= 250) {
        for (let idx = meters.length - 1; idx >= 0; idx -= 1) {
          const meter = meters[idx];
          const pctText = `${meter.pct}%`;
          const pctW = Math.max(22, textWidth(pctText, 9.5, 400));
          const barW = 32;
          const barH = 5;
          const lblW = textWidth(meter.label, 9, 500);
          const barX = row2Right - pctW - 4 - barW;
          const barY = row2Y - 5.5;
          if (barX - 4 - lblW < row2Left + 16) break;

          out.nodes.push({
            kind: 'text',
            x: row2Right,
            y: row2Y,
            text: pctText,
            size: 9.5,
            weight: 400,
            fill: 'muted',
            anchor: 'end',
          });
          out.nodes.push({
            kind: 'rect',
            x: barX,
            y: barY,
            w: barW,
            h: barH,
            r: 2.5,
            fill: 'border',
          });
          if (meter.pct > 0) {
            const fillW = Math.min(barW, Math.max(2.5, (barW * meter.pct) / 100));
            out.nodes.push({
              kind: 'rect',
              x: barX,
              y: barY,
              w: fillW,
              h: barH,
              r: 2.5,
              fill: meterTone(meter.pct),
            });
          }
          out.nodes.push({
            kind: 'text',
            x: barX - 4,
            y: row2Y,
            text: meter.label,
            size: 9,
            weight: 500,
            fill: 'muted',
            anchor: 'end',
          });
          row2Right = barX - 4 - lblW - 8;
        }
      } else {
        const compactMeters = meters.map((m) => `${m.label} ${m.pct}%`).join(' · ');
        const fittedMeters = fitMapText(compactMeters, Math.max(40, w - 24), 9.5, 400);
        if (fittedMeters) {
          out.nodes.push({
            kind: 'text',
            x: row2Right,
            y: row2Y,
            text: fittedMeters,
            size: 9.5,
            weight: 400,
            fill: 'muted',
            anchor: 'end',
          });
          row2Right -= textWidth(fittedMeters, 9.5, 400) + 8;
        }
      }
    }

    const specRaw = hostSpecParts(frame);
    const specBudget = row2Right - row2Left;
    if (specRaw && specBudget >= 28) {
      const specFitted = fitMapText(specRaw, specBudget, 10, 400);
      if (specFitted) {
        out.nodes.push({
          kind: 'text',
          x: row2Left,
          y: row2Y,
          text: specFitted,
          size: 10,
          weight: 400,
          fill: 'muted',
        });
      }
    }
  }

  // Clickable & hoverable host header hit (excludes right-hand count pill so inspect receives clicks)
  out.hits.push({
    kind: 'hit',
    x,
    y,
    w: hitW,
    h: headerH,
    r: CARD_RADIUS,
    hit: buildHit(frame.host),
  });

  const bodyX = x + CARD_PAD;
  const bodyW = w - CARD_PAD * 2;
  let curY = y + headerH;
  const groupKey = `host:${frame.id}`;
  const hasWorkloads =
    frame.stacks.length > 0 || frame.looseTiles.length > 0 || frame.overflowCount > 0;

  if (hasWorkloads) {
    curY += CARD_PAD;

    if (painter.density === 'marks') {
      const pitch = COMPACT_SIZE + COMPACT_GAP;
      for (const stackRow of sized.stackRows) {
        const rowH = Math.max(...stackRow.map((entry) => entry.h));
        let sx = bodyX;
        for (const entry of stackRow) {
          out.nodes.push({
            kind: 'rect',
            x: sx,
            y: curY,
            w: entry.w,
            h: entry.h,
            r: INNER_RADIUS,
            fill: 'surface',
            stroke: 'border',
          });
          const countText = String(entry.stack.tiles.length);
          const countW = textWidth(countText, 10, 600);
          out.nodes.push({
            kind: 'text',
            x: sx + 7,
            y: curY + 14.5,
            text: fitMapText(entry.stack.label.toUpperCase(), entry.w - countW - 22, 10, 600),
            size: 10,
            weight: 600,
            fill: 'muted',
            caps: true,
          });
          out.nodes.push({
            kind: 'text',
            x: sx + entry.w - 7,
            y: curY + 14.5,
            text: countText,
            size: 10,
            weight: 600,
            fill: 'muted',
            anchor: 'end',
          });
          entry.stack.tiles.forEach((tile, index) => {
            drawCompactTile(
              painter,
              tile,
              sx + STACK_PAD_X + (index % entry.cols) * pitch,
              curY + STACK_HEADER_H + Math.floor(index / entry.cols) * pitch,
              { groupKey },
            );
          });
          sx += entry.w + STACK_GAP;
        }
        curY += rowH + STACK_GAP;
      }
      if (
        sized.stackRows.length > 0 &&
        frame.looseTiles.length === 0 &&
        frame.overflowCount === 0
      ) {
        curY -= STACK_GAP;
      }

      if (frame.looseTiles.length > 0) {
        const looseRows = Math.ceil(frame.looseTiles.length / sized.columns);
        frame.looseTiles.forEach((tile, index) => {
          drawCompactTile(
            painter,
            tile,
            bodyX + (index % sized.columns) * pitch,
            curY + Math.floor(index / sized.columns) * pitch,
            { groupKey },
          );
        });
        curY += looseRows * COMPACT_SIZE + Math.max(0, looseRows - 1) * COMPACT_GAP;
        if (frame.overflowCount > 0) curY += STACK_GAP;
      }
    } else {
      // Named mode: Compose stack sub-boxes + categorized or flat loose horizontal service rows
      for (const stackRow of sized.stackRows) {
        const rowH = Math.max(...stackRow.map((entry) => entry.h));
        let sx = bodyX;
        for (const entry of stackRow) {
          out.nodes.push({
            kind: 'rect',
            x: sx,
            y: curY,
            w: entry.w,
            h: entry.h,
            r: INNER_RADIUS,
            fill: 'surface',
            stroke: 'border',
          });
          const countText = String(entry.stack.tiles.length);
          const countW = textWidth(countText, 10, 600);
          out.nodes.push({
            kind: 'text',
            x: sx + 8,
            y: curY + 15,
            text: fitMapText(entry.stack.label.toUpperCase(), entry.w - countW - 24, 10, 600),
            size: 10,
            weight: 600,
            fill: 'muted',
            caps: true,
          });
          out.nodes.push({
            kind: 'text',
            x: sx + entry.w - 8,
            y: curY + 15,
            text: countText,
            size: 10,
            weight: 600,
            fill: 'muted',
            anchor: 'end',
          });
          const innerTileW = (entry.w - STACK_PAD_X * 2 - (entry.cols - 1) * COL_GAP) / entry.cols;
          entry.stack.tiles.forEach((tile, index) => {
            const col = index % entry.cols;
            const row = Math.floor(index / entry.cols);
            drawHorizontalTile(
              painter,
              tile,
              sx + STACK_PAD_X + col * (innerTileW + COL_GAP),
              curY + STACK_HEADER_H + row * (ROW_H + ROW_GAP),
              innerTileW,
              { inStack: true, groupKey },
            );
          });
          sx += entry.w + COL_GAP;
        }
        curY += rowH + STACK_GAP;
      }
      if (
        sized.stackRows.length > 0 &&
        frame.looseTiles.length === 0 &&
        frame.overflowCount === 0
      ) {
        curY -= STACK_GAP;
      }

      if (sized.looseCategories) {
        sized.looseCategories.forEach((category, catIdx) => {
          out.nodes.push({
            kind: 'text',
            x: bodyX,
            y: curY + 11,
            text: fitMapText(category.label.toUpperCase(), bodyW, 10, 600),
            size: 10,
            weight: 600,
            fill: 'muted',
            caps: true,
          });
          curY += CATEGORY_SUBHEADER_H;
          const rows = Math.ceil(category.tiles.length / sized.columns);
          category.tiles.forEach((tile, index) => {
            const col = index % sized.columns;
            const row = Math.floor(index / sized.columns);
            drawHorizontalTile(
              painter,
              tile,
              bodyX + col * (sized.colW + COL_GAP),
              curY + row * (ROW_H + ROW_GAP),
              sized.colW,
              { groupKey },
            );
          });
          curY += rows * ROW_H + Math.max(0, rows - 1) * ROW_GAP;
          if (catIdx < sized.looseCategories!.length - 1) curY += CATEGORY_GROUP_GAP;
        });
        if (frame.overflowCount > 0) curY += STACK_GAP;
      } else if (frame.looseTiles.length > 0) {
        const looseRows = Math.ceil(frame.looseTiles.length / sized.columns);
        frame.looseTiles.forEach((tile, index) => {
          const col = index % sized.columns;
          const row = Math.floor(index / sized.columns);
          drawHorizontalTile(
            painter,
            tile,
            bodyX + col * (sized.colW + COL_GAP),
            curY + row * (ROW_H + ROW_GAP),
            sized.colW,
            { groupKey },
          );
        });
        curY += looseRows * ROW_H + Math.max(0, looseRows - 1) * ROW_GAP;
        if (frame.overflowCount > 0) curY += STACK_GAP;
      }
    }

    if (frame.overflowCount > 0) {
      drawOverflowBar(out, bodyX, curY, bodyW, frame.overflowCount, groupKey);
      curY += OVERFLOW_BAR_H;
    }

    curY += CARD_PAD;
  }

  // Inline storage / ZFS pool usage bar strip
  if (frame.pools.length > 0) {
    if (hasWorkloads) {
      out.nodes.push({ kind: 'rect', x, y: curY, w, h: 1, r: 0, fill: 'border' });
      curY += 1;
    }
    curY += 8;
    out.nodes.push({
      kind: 'text',
      x: bodyX,
      y: curY + 9,
      text: 'STORAGE',
      size: 10,
      weight: 600,
      fill: 'muted',
      caps: true,
    });
    curY += 15;
    const poolColW = (bodyW - (sized.poolColumns - 1) * COL_GAP) / sized.poolColumns;
    frame.pools.forEach((pool, index) => {
      const col = index % sized.poolColumns;
      const row = Math.floor(index / sized.poolColumns);
      drawStorageBar(
        painter,
        pool,
        bodyX + col * (poolColW + COL_GAP),
        curY + row * (POOL_BAR_H + POOL_GAP),
        poolColW,
        groupKey,
      );
    });
  }

  return { x, y, w, h };
};

type LayoutUnit =
  | { kind: 'host'; frame: DiagramHostFrame }
  | {
      kind: 'cluster';
      clusterId: string;
      clusterName: string;
      clusterTypeLabel: string;
      frames: DiagramHostFrame[];
      clusterPools: DiagramTile[];
    };

type SizedLayoutUnit =
  | { kind: 'host'; sized: SizedHostFrame; w: number; h: number }
  | {
      kind: 'cluster';
      clusterId: string;
      clusterName: string;
      clusterTypeLabel: string;
      frames: DiagramHostFrame[];
      clusterPools: DiagramTile[];
      childRows: SizedHostFrame[][];
      poolColumns: number;
      bodyW: number;
      w: number;
      h: number;
    };

interface HostDropTarget {
  box: Box;
  frame: DiagramHostFrame;
  tier?: MapTrafficTier;
  clusterLeftRailX?: number;
}

const partitionHostUnits = (hostFrames: DiagramHostFrame[]): LayoutUnit[] => {
  const counts = new Map<string, number>();
  for (const frame of hostFrames) {
    if (frame.clusterId) {
      counts.set(frame.clusterId, (counts.get(frame.clusterId) ?? 0) + 1);
    }
  }
  const seenClusters = new Set<string>();
  const units: LayoutUnit[] = [];
  for (const frame of hostFrames) {
    const cid = frame.clusterId;
    if (cid && (counts.get(cid) ?? 0) >= 2) {
      if (seenClusters.has(cid)) continue;
      seenClusters.add(cid);
      const members = hostFrames.filter((candidate) => candidate.clusterId === cid);
      const poolById = new Map<string, DiagramTile>();
      for (const member of members) {
        for (const pool of member.clusterPools ?? []) {
          if (!poolById.has(pool.id)) poolById.set(pool.id, pool);
        }
      }
      const primary = members[0];
      units.push({
        kind: 'cluster',
        clusterId: cid,
        clusterName: primary.clusterName || cid,
        clusterTypeLabel: primary.clusterTypeLabel || 'CLUSTER',
        frames: members,
        clusterPools: [...poolById.values()],
      });
    } else {
      units.push({ kind: 'host', frame });
    }
  }
  return units;
};

const sizeLayoutUnit = (
  unit: LayoutUnit,
  density: MapLayoutOptions['density'],
  inner: number,
  liveTraffic: boolean,
): SizedLayoutUnit => {
  if (unit.kind === 'host') {
    const sized = sizeHostFrame(unit.frame, density, inner, liveTraffic, false);
    return { kind: 'host', sized, w: sized.w, h: sized.h };
  }
  const clusterInner = Math.max(220, inner - CLUSTER_PAD * 2);
  const sizedChildren = unit.frames.map((frame) =>
    sizeHostFrame(frame, density, clusterInner, liveTraffic, true),
  );
  const childRows = packIntoRows(sizedChildren, clusterInner, CLUSTER_GAP);
  const maxRowW = Math.max(
    0,
    ...childRows.map(
      (row) => row.reduce((sum, entry) => sum + entry.w, 0) + CLUSTER_GAP * (row.length - 1),
    ),
  );
  const headerTitle = `${unit.clusterName.toUpperCase()} · ${unit.clusterTypeLabel.toUpperCase()} · ${unit.frames.length} NODES`;
  const headerDesiredW = Math.min(
    inner,
    textWidth(headerTitle, 10.5, 600) + headerTitle.length * 0.6 + 28,
  );
  const bodyW = Math.min(
    clusterInner,
    Math.max(maxRowW, Math.min(clusterInner, headerDesiredW - CLUSTER_PAD * 2)),
  );
  const w = Math.min(inner, bodyW + CLUSTER_PAD * 2);
  let h = CLUSTER_HEADER_H + CLUSTER_PAD;
  childRows.forEach((row, idx) => {
    h += Math.max(...row.map((entry) => entry.h));
    if (idx < childRows.length - 1) h += CLUSTER_WRAP_GAP;
  });
  const poolColumns = unit.clusterPools.length >= 2 && bodyW >= 440 ? 2 : 1;
  if (unit.clusterPools.length > 0) {
    h += CLUSTER_PAD + 1 + 8 + 15;
    const poolRows = Math.ceil(unit.clusterPools.length / poolColumns);
    h += poolRows * POOL_BAR_H + Math.max(0, poolRows - 1) * POOL_GAP + CLUSTER_PAD;
  } else {
    h += CLUSTER_PAD;
  }
  return {
    kind: 'cluster',
    clusterId: unit.clusterId,
    clusterName: unit.clusterName,
    clusterTypeLabel: unit.clusterTypeLabel,
    frames: unit.frames,
    clusterPools: unit.clusterPools,
    childRows,
    poolColumns,
    bodyW,
    w,
    h,
  };
};

const drawClusterUnit = (
  painter: HostPainter,
  sized: Extract<SizedLayoutUnit, { kind: 'cluster' }>,
  x: number,
  y: number,
  activeTier: (tier: MapTrafficTier | undefined) => MapTrafficTier | undefined,
): { box: Box; drops: HostDropTarget[]; tier?: MapTrafficTier } => {
  const { out } = painter;
  const { w, h, bodyW, childRows, clusterPools, poolColumns } = sized;

  // Subtle architectural dashed cluster container
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w,
    h,
    r: CLUSTER_RADIUS,
    fill: 'surface',
    opacity: 0.35,
    stroke: 'border',
    dashed: true,
  });

  // Left-aligned uppercase cluster header with clean backing so vertical drops never cut text
  const rawTitle = `${sized.clusterName.toUpperCase()} · ${sized.clusterTypeLabel.toUpperCase()} · ${sized.frames.length} NODES`;
  const fittedTitle = fitMapText(rawTitle, w - 24, 10.5, 600);
  const titleW = textWidth(fittedTitle, 10.5, 600) + fittedTitle.length * 0.6;
  out.nodes.push({
    kind: 'rect',
    x: x + 8,
    y: y + 4,
    w: Math.min(w - 16, titleW + 8),
    h: 16,
    r: 4,
    fill: 'surface',
  });
  out.nodes.push({
    kind: 'text',
    x: x + 12,
    y: y + 15.5,
    text: fittedTitle,
    size: 10.5,
    weight: 600,
    fill: 'muted',
    caps: true,
  });

  const drops: HostDropTarget[] = [];
  const tiers: Array<MapTrafficTier | undefined> = [];
  let curY = y + CLUSTER_HEADER_H + CLUSTER_PAD;

  childRows.forEach((row, rowIdx) => {
    const rowW = row.reduce((sum, entry) => sum + entry.w, 0) + CLUSTER_GAP * (row.length - 1);
    let cx = x + CLUSTER_PAD + Math.max(0, (bodyW - rowW) / 2);
    let rowH = 0;
    for (const entry of row) {
      const childBox = drawHostFrame(painter, entry, cx, curY, true);
      const childTier = activeTier(frameTrafficTier(entry.frame));
      tiers.push(childTier);
      drops.push({
        box: childBox,
        frame: entry.frame,
        tier: childTier,
        ...(rowIdx > 0 ? { clusterLeftRailX: x + 5 } : {}),
      });
      rowH = Math.max(rowH, childBox.h);
      cx += entry.w + CLUSTER_GAP;
    }
    curY += rowH;
    if (rowIdx < childRows.length - 1) curY += CLUSTER_WRAP_GAP;
  });

  if (clusterPools.length > 0) {
    curY += CLUSTER_PAD;
    out.nodes.push({ kind: 'rect', x, y: curY, w, h: 1, r: 0, fill: 'border' });
    curY += 1 + 8;
    const poolX = x + CLUSTER_PAD;
    out.nodes.push({
      kind: 'text',
      x: poolX,
      y: curY + 9,
      text: 'SHARED STORAGE',
      size: 10,
      weight: 600,
      fill: 'muted',
      caps: true,
    });
    curY += 15;
    const poolColW = (bodyW - (poolColumns - 1) * COL_GAP) / poolColumns;
    clusterPools.forEach((pool, index) => {
      const col = index % poolColumns;
      const row = Math.floor(index / poolColumns);
      drawStorageBar(
        painter,
        pool,
        poolX + col * (poolColW + COL_GAP),
        curY + row * (POOL_BAR_H + POOL_GAP),
        poolColW,
        'shared-storage',
      );
      tiers.push(activeTier(pool.trafficTier));
    });
  }

  return {
    box: { x, y, w, h },
    drops,
    tier: maxTrafficTier(...tiers),
  };
};

interface SizedGroupCard {
  group: DiagramGroup;
  mode: 'horizontal' | 'compact' | 'storage';
  columns: number;
  itemW: number;
  w: number;
  h: number;
  regularTiles: DiagramTile[];
  overflowTile?: DiagramTile;
}

const sizeGroupCard = (
  group: DiagramGroup,
  mode: 'horizontal' | 'compact' | 'storage',
  inner: number,
  maxColsCap = 3,
): SizedGroupCard => {
  const overflowTile = group.tiles.find((tile) => !tile.navigable && tile.id.endsWith(':more'));
  const regularTiles = overflowTile
    ? group.tiles.filter((tile) => tile !== overflowTile)
    : group.tiles;
  const titleText = group.label.toUpperCase();
  const countText = group.count.toLocaleString();
  const headerMinW = Math.min(
    inner,
    textWidth(titleText, 11, 600) + textWidth(countText, 11, 600) + 42,
  );

  if (mode === 'compact') {
    const pitch = COMPACT_SIZE + COMPACT_GAP;
    const maxCols = Math.max(1, Math.floor((inner - CARD_PAD * 2 + COMPACT_GAP) / pitch));
    const count = group.tiles.length;
    const wanted = count <= 5 ? count : Math.ceil(Math.sqrt(count * 3));
    const columns = Math.max(1, Math.min(count, maxCols, wanted));
    const rows = Math.ceil(count / columns);
    const w = Math.min(inner, Math.max(columns * pitch - COMPACT_GAP + CARD_PAD * 2, headerMinW));
    const h =
      GROUP_HEADER_H +
      CARD_PAD +
      rows * COMPACT_SIZE +
      Math.max(0, rows - 1) * COMPACT_GAP +
      CARD_PAD;
    return { group, mode, columns, itemW: COMPACT_SIZE, w, h, regularTiles: group.tiles };
  }

  if (mode === 'storage') {
    const baseBarW = inner < 520 ? Math.min(240, inner - CARD_PAD * 2) : 256;
    const maxCols = Math.max(
      1,
      Math.floor((inner - CARD_PAD * 2 + COL_GAP) / (baseBarW + COL_GAP)),
    );
    const count = regularTiles.length;
    const columns = Math.max(1, Math.min(maxCols, maxColsCap, count >= 4 ? 2 : 1));
    const rows = Math.ceil(Math.max(1, count) / columns);
    const w = Math.min(
      inner,
      Math.max(columns * baseBarW + (columns - 1) * COL_GAP + CARD_PAD * 2, headerMinW),
    );
    const bodyW = w - CARD_PAD * 2;
    const itemW = (bodyW - (columns - 1) * COL_GAP) / columns;
    let h =
      GROUP_HEADER_H + CARD_PAD + rows * POOL_BAR_H + Math.max(0, rows - 1) * POOL_GAP + CARD_PAD;
    if (overflowTile) h += POOL_GAP + OVERFLOW_BAR_H;
    return { group, mode, columns, itemW, w, h, regularTiles, overflowTile };
  }

  // Horizontal cards (WAYS IN or CHECKS)
  const preferredRowW = group.key === 'checks' ? 260 : 228;
  const baseRowW = inner < 520 ? Math.min(216, Math.max(180, inner - CARD_PAD * 2)) : preferredRowW;
  const maxCols = Math.max(1, Math.floor((inner - CARD_PAD * 2 + COL_GAP) / (baseRowW + COL_GAP)));
  const count = regularTiles.length;
  const desiredCols = count <= 3 ? count : count <= 8 ? 2 : 3;
  const columns = Math.max(1, Math.min(maxCols, maxColsCap, Math.max(1, desiredCols)));
  const rows = Math.ceil(Math.max(1, count) / columns);
  const w = Math.min(
    inner,
    Math.max(columns * baseRowW + (columns - 1) * COL_GAP + CARD_PAD * 2, headerMinW),
  );
  const bodyW = w - CARD_PAD * 2;
  const itemW = (bodyW - (columns - 1) * COL_GAP) / columns;
  let h = GROUP_HEADER_H + CARD_PAD + rows * ROW_H + Math.max(0, rows - 1) * ROW_GAP + CARD_PAD;
  if (overflowTile) h += ROW_GAP + OVERFLOW_BAR_H;
  return { group, mode, columns, itemW, w, h, regularTiles, overflowTile };
};

const drawGroupCard = (painter: HostPainter, sized: SizedGroupCard, x: number, y: number): Box => {
  const { out } = painter;
  const { group, w, h } = sized;
  out.nodes.push({
    kind: 'rect',
    x,
    y,
    w,
    h,
    r: CARD_RADIUS,
    fill: 'surface',
    stroke: 'border',
  });
  out.nodes.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + 0.5,
    w: w - 1,
    h: GROUP_HEADER_H - 0.5,
    r: CARD_RADIUS - 0.5,
    fill: 'surfaceAlt',
  });
  out.nodes.push({
    kind: 'rect',
    x: x + 0.5,
    y: y + GROUP_HEADER_H / 2,
    w: w - 1,
    h: GROUP_HEADER_H / 2,
    r: 0,
    fill: 'surfaceAlt',
  });
  out.nodes.push({
    kind: 'rect',
    x,
    y: y + GROUP_HEADER_H,
    w,
    h: 1,
    r: 0,
    fill: 'border',
  });

  const title = group.label.toUpperCase();
  const count = group.count.toLocaleString();
  const fitted = fitMapText(title, w - 24 - textWidth(count, 11, 600) - 8, 11, 600);
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
    x: x + 10 + textWidth(fitted, 11, 600) + fitted.length * 0.6 + 8,
    y: y + 17,
    text: count,
    size: 11,
    weight: 600,
    fill: 'text',
  });

  out.inspects?.push({
    kind: 'inspect',
    x,
    y,
    w,
    h: GROUP_HEADER_H,
    r: CARD_RADIUS,
    label: `Inspect ${group.label} (${count})`,
    target: { kind: 'group', groupKey: group.key },
  });

  const bodyX = x + CARD_PAD;
  const bodyW = w - CARD_PAD * 2;
  const bodyY = y + GROUP_HEADER_H + CARD_PAD;

  if (sized.mode === 'compact') {
    const pitch = COMPACT_SIZE + COMPACT_GAP;
    const tilesWidth = sized.columns * pitch - COMPACT_GAP;
    const left = x + (w - tilesWidth) / 2;
    sized.regularTiles.forEach((tile, index) => {
      drawCompactTile(
        painter,
        tile,
        left + (index % sized.columns) * pitch,
        bodyY + Math.floor(index / sized.columns) * pitch,
        { groupKey: group.key },
      );
    });
    return { x, y, w, h };
  }

  if (sized.mode === 'storage') {
    const rows = Math.ceil(Math.max(1, sized.regularTiles.length) / sized.columns);
    sized.regularTiles.forEach((tile, index) => {
      const col = index % sized.columns;
      const row = Math.floor(index / sized.columns);
      drawStorageBar(
        painter,
        tile,
        bodyX + col * (sized.itemW + COL_GAP),
        bodyY + row * (POOL_BAR_H + POOL_GAP),
        sized.itemW,
        group.key,
      );
    });
    if (sized.overflowTile) {
      const countNum = Number(sized.overflowTile.name.replace('+', '')) || 0;
      drawOverflowBar(
        out,
        bodyX,
        bodyY + rows * POOL_BAR_H + Math.max(0, rows - 1) * POOL_GAP + POOL_GAP,
        bodyW,
        countNum,
        group.key,
        `${sized.overflowTile.name} ${sized.overflowTile.caption}`,
      );
    }
    return { x, y, w, h };
  }

  const rows = Math.ceil(Math.max(1, sized.regularTiles.length) / sized.columns);
  sized.regularTiles.forEach((tile, index) => {
    const col = index % sized.columns;
    const row = Math.floor(index / sized.columns);
    drawHorizontalTile(
      painter,
      tile,
      bodyX + col * (sized.itemW + COL_GAP),
      bodyY + row * (ROW_H + ROW_GAP),
      sized.itemW,
      { groupKey: group.key },
    );
  });
  if (sized.overflowTile) {
    const countNum = Number(sized.overflowTile.name.replace('+', '')) || 0;
    drawOverflowBar(
      out,
      bodyX,
      bodyY + rows * ROW_H + Math.max(0, rows - 1) * ROW_GAP + ROW_GAP,
      bodyW,
      countNum,
      group.key,
      `${sized.overflowTile.name} ${sized.overflowTile.caption}`,
    );
  }
  return { x, y, w, h };
};

const addLine = (
  lines: SceneNode[],
  points: Array<[number, number]>,
  options: {
    tone?: MapTone | MapHealth;
    arrow?: boolean;
    dashed?: boolean;
    activity?: MapTrafficTier;
    strokeWidth?: number;
  } = {},
) => {
  lines.push({
    kind: 'line',
    points,
    tone: options.tone ?? 'accent',
    ...(options.strokeWidth !== undefined ? { strokeWidth: options.strokeWidth } : {}),
    ...(options.arrow ? { arrow: true } : {}),
    ...(options.dashed ? { dashed: true } : {}),
    ...(options.activity && !options.dashed ? { activity: options.activity } : {}),
  });
};

const drawHorizontalBus = (
  lines: SceneNode[],
  leftX: number,
  rightX: number,
  busY: number,
  feedX: number,
  activeXs: number[],
  activity: MapTrafficTier | undefined,
) => {
  if (!activity || activeXs.length === 0 || leftX >= rightX) {
    addLine(lines, [
      [leftX, busY],
      [rightX, busY],
    ]);
    return;
  }
  const activeMin = Math.max(leftX, Math.min(feedX, ...activeXs));
  const activeMax = Math.min(rightX, Math.max(feedX, ...activeXs));
  if (leftX < activeMin) {
    addLine(lines, [
      [leftX, busY],
      [activeMin, busY],
    ]);
  }
  if (activeMin < feedX) {
    addLine(
      lines,
      [
        [feedX, busY],
        [activeMin, busY],
      ],
      { activity },
    );
  }
  if (feedX < activeMax) {
    addLine(
      lines,
      [
        [feedX, busY],
        [activeMax, busY],
      ],
      { activity },
    );
  }
  if (activeMax < rightX) {
    addLine(lines, [
      [activeMax, busY],
      [rightX, busY],
    ]);
  }
};

const connectorLabel = (
  labels: SceneNode[],
  text: string,
  x: number,
  y: number,
  size: 10 | 11 = 11,
  fill: MapTone | MapHealth = 'muted',
) => {
  const h = size === 10 ? 14 : 16;
  const w = textWidth(text, size, 500) + 10;
  labels.push({
    kind: 'rect',
    x: x - w / 2,
    y: y - h / 2,
    w,
    h,
    r: 4,
    fill: 'surface',
  });
  labels.push({
    kind: 'text',
    x,
    y: y + (size === 10 ? 3.5 : 4),
    text,
    size,
    weight: size === 10 ? 500 : 400,
    fill,
    anchor: 'middle',
  });
};

const hostNetworkText = (network: DiagramNetwork): string => {
  const parts: string[] = [];
  if (network.kind === 'bridge') {
    parts.push(network.label);
    if (network.detail) parts.push(network.detail);
    parts.push(`${network.memberCount.toLocaleString()} connected`);
  } else {
    parts.push(
      `${network.label} · ${network.memberCount.toLocaleString()} ${
        network.memberCount === 1 ? 'address' : 'addresses'
      }`,
    );
  }
  if (network.linkSpeedMbps !== undefined && network.linkSpeedMbps > 0) {
    parts.push(formatMapLinkSpeed(network.linkSpeedMbps));
  }
  if (network.trafficTier && network.netTotalBytes !== undefined) {
    parts.push(`${formatCompactSpeed(network.netTotalBytes)}/s`);
  }
  return parts.join(' · ');
};

const drawNetworksStrip = (
  out: MapCursor,
  networks: DiagramNetwork[],
  left: number,
  width: number,
  y: number,
  toneOf: (network: DiagramNetwork) => MapTone,
  liveTraffic: boolean,
): number => {
  const shown = networks.slice(0, DIAGRAM_MAX_NETWORKS);
  const hidden = networks.length - shown.length;
  const pills: Array<{
    text: string;
    tone: MapTone;
    w: number;
    label: string;
    target: MapInspectTarget;
    activity?: MapTrafficTier;
  }> = shown.map((network) => {
    const text = fitMapText(hostNetworkText(network), width - 36, 12, 600);
    return {
      text,
      tone: toneOf(network),
      w: textWidth(text, 12, 600) + 36,
      label: `Inspect network ${network.label}`,
      target: { kind: 'network', networkId: network.id },
      ...(liveTraffic && network.trafficTier ? { activity: network.trafficTier } : {}),
    };
  });
  if (hidden > 0) {
    const text = `+${hidden} more`;
    const hiddenTier = liveTraffic
      ? maxTrafficTier(
          ...networks.slice(DIAGRAM_MAX_NETWORKS).map((network) => network.trafficTier),
        )
      : undefined;
    pills.push({
      text,
      tone: 'netOther',
      w: textWidth(text, 12, 600) + 36,
      label: `Inspect all ${networks.length.toLocaleString()} networks`,
      target: { kind: 'all-networks' },
      ...(hiddenTier ? { activity: hiddenTier } : {}),
    });
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
    let x = left + Math.max(0, (width - total) / 2);
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
      if (pill.activity) {
        out.nodes.push({
          kind: 'rect',
          x,
          y: top,
          w: pill.w,
          h: PILL_HEIGHT,
          r: PILL_HEIGHT / 2,
          fill: 'none',
          activity: pill.activity,
        });
      }
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
      out.inspects?.push({
        kind: 'inspect',
        x,
        y: top,
        w: pill.w,
        h: PILL_HEIGHT,
        r: PILL_HEIGHT / 2,
        label: pill.label,
        target: pill.target,
      });
      x += pill.w + 8;
    }
    top += PILL_HEIGHT + 6;
  }
  return top - 6;
};

const lowerConnectorLabel = (group: DiagramGroup): string => {
  if (group.key === 'backups') return 'backups';
  if (group.key === 'checks') return 'checks';
  return 'disks and volumes';
};

/** Lays out the service diagram with Goblinmine-inspired architectural host frames. */
export function layoutHostDiagram(diagram: ServiceDiagram, options: MapLayoutOptions): MapScene {
  const margin = options.header ? 28 : 0;
  const width = Math.max(MIN_WIDTH + margin * 2, Math.floor(options.width));
  const left = margin + GUTTER;
  const inner = width - (margin + GUTTER) * 2;
  const mid = width / 2;

  const out: MapCursor = { nodes: [], hits: [], inspects: [] };
  const lines: SceneNode[] = [];
  const labels: SceneNode[] = [];

  const toneByNetwork = new Map<string, MapNetworkTone>();
  for (const network of diagram.networks.slice(0, DIAGRAM_MAX_NETWORKS)) {
    toneByNetwork.set(
      network.id,
      network.colorIndex < 0
        ? 'netOther'
        : NETWORK_TONES[network.colorIndex % NETWORK_TONES.length],
    );
  }
  const liveTraffic = Boolean(options.liveTraffic);
  const activeTier = (tier: MapTrafficTier | undefined): MapTrafficTier | undefined =>
    liveTraffic ? tier : undefined;

  const painter: HostPainter = {
    out,
    density: options.density,
    toneFor: (segmentId) =>
      diagram.networks.length > 1 && segmentId ? toneByNetwork.get(segmentId) : undefined,
    liveTraffic,
    networkCount: diagram.networks.length,
  };

  let y = drawPosterHeader(out, options.header, margin, width - margin * 2);
  let source: { x: number; y: number } | null = null;
  let lastBottom = 0;

  // Layer 1: Internet, Ways in, and Default Gateways
  const isCompact = options.density === 'marks';
  const internetW = isCompact ? COMPACT_SIZE : Math.min(136, inner);
  const internetH = isCompact ? COMPACT_SIZE : ROW_H;
  const gatewayW = isCompact ? COMPACT_SIZE : Math.min(168, inner);
  const gatewayH = isCompact ? COMPACT_SIZE : ROW_H;
  const gatewayGap = isCompact ? COMPACT_GAP : COL_GAP;
  const gatewayPitch = gatewayW + gatewayGap;

  const drawGatewayCard = (gateway: DiagramTile, gx: number, gy: number) => {
    if (isCompact) {
      drawCompactTile(painter, gateway, gx, gy, { dashed: true });
    } else {
      drawHorizontalTile(painter, gateway, gx, gy, gatewayW, { dashed: true });
    }
  };

  const drawGatewayRows = (top: number): number => {
    const perRow = Math.max(1, Math.floor((inner + gatewayGap) / gatewayPitch));
    let rowTop = top;
    for (let start = 0; start < diagram.gateways.length; start += perRow) {
      const shown = diagram.gateways.slice(start, start + perRow);
      const rowWidth = shown.length * gatewayPitch - gatewayGap;
      const startX = mid - rowWidth / 2;
      shown.forEach((gateway, index) => {
        drawGatewayCard(gateway, startX + index * gatewayPitch, rowTop);
      });
      rowTop += gatewayH + gatewayGap;
    }
    return rowTop - gatewayGap;
  };

  const internetTile: DiagramTile = {
    id: 'internet',
    name: 'Internet',
    caption: 'not monitored',
    typeLabel: '',
    status: '',
    health: 'unknown',
    platform: 'internet',
    navigable: false,
  };

  const drawInternetCard = (ix: number, iy: number) => {
    if (isCompact) {
      drawCompactTile(painter, internetTile, ix, iy, { dashed: true });
    } else {
      drawHorizontalTile(painter, internetTile, ix, iy, internetW, { dashed: true });
    }
  };

  if (diagram.access) {
    const accessSized = sizeGroupCard(
      diagram.access,
      isCompact ? 'compact' : 'horizontal',
      inner,
      3,
    );
    const boxX = mid - accessSized.w / 2;
    const label = diagram.internet?.label ?? '';
    const reach = Math.max(68, textWidth(label, 11, 500) + 24);
    const beside = boxX - left >= internetW + reach;
    const internetAbove = diagram.internet !== undefined && !beside;
    const roomRight = Math.floor(
      (left + inner - (boxX + accessSized.w) - GROUP_GAP + gatewayGap) / gatewayPitch,
    );
    const gatewaysAbove = diagram.gateways.length > 0 && diagram.gateways.length > roomRight;

    if (internetAbove || gatewaysAbove) {
      let tileBottom = y;
      if (gatewaysAbove) {
        tileBottom = drawGatewayRows(y);
        if (internetAbove) y = tileBottom + gatewayGap;
      }
      if (internetAbove) {
        drawInternetCard(mid - internetW / 2, y);
        tileBottom = y + internetH;
      }
      y = tileBottom + 32;
      if (internetAbove) {
        addLine(
          lines,
          [
            [mid, tileBottom],
            [mid, y],
          ],
          { arrow: true, dashed: true },
        );
        if (label) connectorLabel(labels, label, mid, (tileBottom + y) / 2);
      }
    }

    const accessBox = drawGroupCard(painter, accessSized, boxX, y);
    if (diagram.internet && beside) {
      const tileX = boxX - reach - internetW;
      const lineY = accessBox.y + accessBox.h / 2;
      drawInternetCard(tileX, lineY - internetH / 2);
      addLine(
        lines,
        [
          [tileX + internetW, lineY],
          [boxX, lineY],
        ],
        { arrow: true, dashed: true },
      );
      if (label) connectorLabel(labels, label, tileX + internetW + reach / 2, lineY - 12);
    }
    if (!gatewaysAbove && diagram.gateways.length > 0) {
      const gx = boxX + accessSized.w + GROUP_GAP;
      const gy = accessBox.y + accessBox.h / 2 - gatewayH / 2;
      diagram.gateways.forEach((gateway, index) => {
        drawGatewayCard(gateway, gx + index * gatewayPitch, gy);
      });
    }

    lastBottom = Math.max(lastBottom, bottomOf(accessBox));
    source = { x: mid, y: bottomOf(accessBox) };
    y = bottomOf(accessBox) + 32;
  } else if (diagram.gateways.length > 0) {
    const gwBottom = drawGatewayRows(y);
    lastBottom = Math.max(lastBottom, gwBottom);
    y = gwBottom + 32;
  }

  const accessTier = activeTier(groupTrafficTier(diagram.access));

  // Layer 2: Networks strip
  if (diagram.networks.length > 0) {
    const bandBottom = drawNetworksStrip(
      out,
      diagram.networks,
      left,
      inner,
      y,
      (network) => toneByNetwork.get(network.id) ?? 'netOther',
      liveTraffic,
    );
    if (source) {
      addLine(
        lines,
        [
          [source.x, source.y],
          [mid, y],
        ],
        { arrow: true, activity: accessTier },
      );
    }
    lastBottom = Math.max(lastBottom, bandBottom);
    source = { x: mid, y: bandBottom };
    y = bandBottom + LAYER_GAP;
  }

  // Layer 3: Architectural Host Frames & Automatic Cluster Containers
  const hostFrames = diagram.hostFrames ?? [];
  const units = partitionHostUnits(hostFrames);
  const sizedUnits = units.map((unit) => sizeLayoutUnit(unit, options.density, inner, liveTraffic));
  const unitRows = packIntoRows(sizedUnits, inner, GROUP_GAP);
  const placedUnits: Box[][] = [];
  const placedUnitTiers: Array<Array<MapTrafficTier | undefined>> = [];
  const placedDrops: HostDropTarget[][] = [];
  let rowTop = y;

  for (const row of unitRows) {
    const totalW = row.reduce((sum, entry) => sum + entry.w, 0) + GROUP_GAP * (row.length - 1);
    let x = left + Math.max(0, (inner - totalW) / 2);
    const boxes: Box[] = [];
    const tiers: Array<MapTrafficTier | undefined> = [];
    const drops: HostDropTarget[] = [];
    for (const entry of row) {
      if (entry.kind === 'cluster') {
        const result = drawClusterUnit(painter, entry, x, rowTop, activeTier);
        boxes.push(result.box);
        tiers.push(result.tier);
        drops.push(...result.drops);
      } else {
        const box = drawHostFrame(painter, entry.sized, x, rowTop, false);
        const tier = activeTier(frameTrafficTier(entry.sized.frame));
        boxes.push(box);
        tiers.push(tier);
        drops.push({ box, frame: entry.sized.frame, tier });
      }
      x += entry.w + GROUP_GAP;
    }
    placedUnits.push(boxes);
    placedUnitTiers.push(tiers);
    placedDrops.push(drops);
    const rowBottom = Math.max(...boxes.map(bottomOf));
    lastBottom = Math.max(lastBottom, rowBottom);
    rowTop = rowBottom + FRAME_ROW_GAP;
  }

  const singleHostRow = placedUnits.length === 1;
  const leftRail = margin + GUTTER / 2;
  const rightRail = width - margin - GUTTER / 2;
  const rowMaxTiers = placedUnitTiers.map((tiers) => maxTrafficTier(...tiers));
  const allHostsTier = maxTrafficTier(...rowMaxTiers);

  const drawHostDrop = (drop: HostDropTarget, busY: number) => {
    const { box, frame, tier, clusterLeftRailX } = drop;
    const speed = frame.linkSpeedMbps ?? frame.host.linkSpeedMbps;
    const strokeWidth = strokeWidthForLinkSpeed(speed);
    const utilPct = frame.host.linkUtilizationPercent;
    const saturated = utilPct !== undefined && utilPct >= MAP_LINK_SATURATED_PERCENT;
    const hostTone = hostSingleNetworkTone(frame, diagram.networks.length, painter.toneFor);
    const tone: MapTone | MapHealth = saturated ? 'attention' : (hostTone ?? 'accent');
    const pillText =
      saturated && speed !== undefined && speed > 0
        ? `${formatMapLinkSpeed(speed)} · ${Math.round(utilPct)}%`
        : saturated
          ? `${Math.round(utilPct)}%`
          : speed !== undefined && speed > 0
            ? formatMapLinkSpeed(speed)
            : undefined;
    const pillFill: MapTone | MapHealth = saturated ? 'attention' : 'muted';

    if (clusterLeftRailX !== undefined) {
      const turnY = box.y - 12;
      addLine(
        lines,
        [
          [clusterLeftRailX, busY],
          [clusterLeftRailX, turnY],
          [centreX(box), turnY],
          [centreX(box), box.y],
        ],
        { tone, arrow: true, activity: tier, strokeWidth },
      );
      if (pillText) {
        connectorLabel(labels, pillText, centreX(box), turnY, 10, pillFill);
      }
      return;
    }

    addLine(
      lines,
      [
        [centreX(box), busY],
        [centreX(box), box.y],
      ],
      { tone, arrow: true, activity: tier, strokeWidth },
    );
    if (pillText) {
      connectorLabel(labels, pillText, centreX(box), busY + 12, 10, pillFill);
    }
  };

  const dropBusXs = (drops: HostDropTarget[]): number[] =>
    drops.map((drop) => drop.clusterLeftRailX ?? centreX(drop.box));

  const activeDropBusXs = (drops: HostDropTarget[]): number[] =>
    drops.filter((drop) => drop.tier).map((drop) => drop.clusterLeftRailX ?? centreX(drop.box));

  if (source && placedUnits.length > 0) {
    if (singleHostRow) {
      const unitsRow = placedUnits[0];
      const drops = placedDrops[0];
      const busY = unitsRow[0].y - BUS_DROP;
      addLine(
        lines,
        [
          [source.x, source.y],
          [source.x, busY],
        ],
        { activity: allHostsTier },
      );
      const xs = [...dropBusXs(drops), source.x];
      const activeXs = activeDropBusXs(drops);
      drawHorizontalBus(
        lines,
        Math.min(...xs),
        Math.max(...xs),
        busY,
        source.x,
        activeXs,
        allHostsTier,
      );
      drops.forEach((drop) => drawHostDrop(drop, busY));
    } else {
      const firstBus = placedUnits[0][0].y - BUS_DROP;
      const lastBus = placedUnits[placedUnits.length - 1][0].y - BUS_DROP;
      const lowerRowsTier = maxTrafficTier(...rowMaxTiers.slice(1));
      let lastActiveRow = -1;
      for (let rowIndex = 1; rowIndex < rowMaxTiers.length; rowIndex += 1) {
        if (rowMaxTiers[rowIndex]) lastActiveRow = rowIndex;
      }
      addLine(
        lines,
        [
          [source.x, source.y],
          [source.x, firstBus],
        ],
        { activity: allHostsTier },
      );
      if (lastActiveRow > 0 && lowerRowsTier) {
        const activeRailBottom = placedUnits[lastActiveRow][0].y - BUS_DROP;
        addLine(
          lines,
          [
            [leftRail, firstBus],
            [leftRail, activeRailBottom],
          ],
          { activity: lowerRowsTier },
        );
        if (activeRailBottom < lastBus) {
          addLine(lines, [
            [leftRail, activeRailBottom],
            [leftRail, lastBus],
          ]);
        }
      } else {
        addLine(lines, [
          [leftRail, firstBus],
          [leftRail, lastBus],
        ]);
      }
      placedUnits.forEach((unitsRow, rowIndex) => {
        const drops = placedDrops[rowIndex];
        const busY = unitsRow[0].y - BUS_DROP;
        const reach = busY === firstBus ? source!.x : 0;
        const busRight = Math.max(reach, ...dropBusXs(drops));
        const activeXs = activeDropBusXs(drops);
        if (rowIndex === 0) {
          const row0Targets = lastActiveRow > 0 ? [leftRail, ...activeXs] : activeXs;
          drawHorizontalBus(
            lines,
            leftRail,
            busRight,
            busY,
            source!.x,
            row0Targets,
            maxTrafficTier(rowMaxTiers[0], lowerRowsTier),
          );
        } else {
          drawHorizontalBus(
            lines,
            leftRail,
            busRight,
            busY,
            leftRail,
            activeXs,
            rowMaxTiers[rowIndex],
          );
        }
        drops.forEach((drop) => drawHostDrop(drop, busY));
      });
    }
  }

  const hostsBottom = placedUnits.length > 0 ? rowTop - FRAME_ROW_GAP : y - LAYER_GAP;
  y = placedUnits.length > 0 ? hostsBottom + LAYER_GAP + 10 : y;

  // Layer 4: Bottom shared row (Shared Storage, Backups, Checks)
  const lowerGroups = [diagram.sharedStorage, diagram.backups, diagram.checks].filter(
    (group): group is DiagramGroup => group !== undefined && group.tiles.length > 0,
  );
  const lowerTiers = lowerGroups.map((group) => activeTier(groupTrafficTier(group)));
  const anyLowerTier = maxTrafficTier(...lowerTiers);

  if (lowerGroups.length > 0) {
    const sizedLower = lowerGroups.map((group) => {
      const mode = group.key === 'checks' ? (isCompact ? 'compact' : 'horizontal') : 'storage';
      return sizeGroupCard(group, mode, inner, 2);
    });
    const lowerRows = packIntoRows(sizedLower, inner, GROUP_GAP);
    let lowerBoxes: Box[] = [];
    let top = y;
    for (const row of lowerRows) {
      const totalW = row.reduce((sum, entry) => sum + entry.w, 0) + GROUP_GAP * (row.length - 1);
      let x = left + Math.max(0, (inner - totalW) / 2);
      const boxes: Box[] = [];
      for (const entry of row) {
        boxes.push(drawGroupCard(painter, entry, x, top));
        x += entry.w + GROUP_GAP;
      }
      lowerBoxes = [...lowerBoxes, ...boxes];
      top = Math.max(...boxes.map(bottomOf)) + 32;
    }
    const lowerBottom = top - 32;
    lastBottom = Math.max(lastBottom, lowerBottom);
    const targets = lowerBoxes.filter((box) => box.y === y);

    if (placedUnits.length > 0 && targets.length > 0) {
      const collectY = hostsBottom + 18;
      const wrapped = lowerBoxes.slice(targets.length);
      const targetTiers = targets.map((_, index) => lowerTiers[index]);
      const wrappedTiers = wrapped.map((_, index) => lowerTiers[targets.length + index]);
      const wrappedMaxTier = maxTrafficTier(...wrappedTiers);
      const bridgeTier =
        allHostsTier && anyLowerTier ? maxTrafficTier(allHostsTier, anyLowerTier) : undefined;

      if (singleHostRow) {
        const boxes = placedUnits[0];
        const tiers = placedUnitTiers[0];
        boxes.forEach((box, index) =>
          addLine(
            lines,
            [
              [centreX(box), bottomOf(box)],
              [centreX(box), collectY],
            ],
            { activity: anyLowerTier ? tiers[index] : undefined },
          ),
        );
        const xs = [...boxes.map(centreX), ...targets.map(centreX)];
        const minX = Math.min(...xs);
        const maxX = Math.max(...xs);
        if (bridgeTier) {
          const activeSourceXs = boxes
            .map((box, index) => (tiers[index] ? centreX(box) : undefined))
            .filter((value): value is number => value !== undefined);
          const activeTargetXs = [
            ...targets
              .map((box, index) => (targetTiers[index] ? centreX(box) : undefined))
              .filter((value): value is number => value !== undefined),
            ...(wrappedMaxTier ? [maxX] : []),
          ];
          const minAct = Math.min(...activeSourceXs, ...activeTargetXs);
          const maxAct = Math.max(...activeSourceXs, ...activeTargetXs);
          if (minX < minAct) {
            addLine(lines, [
              [minX, collectY],
              [minAct, collectY],
            ]);
          }
          if (minAct < maxAct) {
            addLine(
              lines,
              [
                [minAct, collectY],
                [maxAct, collectY],
              ],
              { activity: bridgeTier },
            );
          }
          if (maxAct < maxX) {
            addLine(lines, [
              [maxAct, collectY],
              [maxX, collectY],
            ]);
          }
        } else {
          addLine(lines, [
            [minX, collectY],
            [maxX, collectY],
          ]);
        }
      } else {
        let railTop = collectY;
        let firstActiveTickY: number | undefined;
        placedUnits.forEach((boxes, rowIndex) => {
          const last = boxes[boxes.length - 1];
          const tickY = last.y + last.h / 2;
          const rowActivity = anyLowerTier ? rowMaxTiers[rowIndex] : undefined;
          if (rowActivity && (firstActiveTickY === undefined || tickY < firstActiveTickY)) {
            firstActiveTickY = tickY;
          }
          addLine(
            lines,
            [
              [last.x + last.w, tickY],
              [rightRail, tickY],
            ],
            { activity: rowActivity },
          );
          railTop = Math.min(railTop, tickY);
        });
        if (bridgeTier && firstActiveTickY !== undefined) {
          if (railTop < firstActiveTickY) {
            addLine(lines, [
              [rightRail, railTop],
              [rightRail, firstActiveTickY],
            ]);
          }
          addLine(
            lines,
            [
              [rightRail, firstActiveTickY],
              [rightRail, collectY],
            ],
            { activity: bridgeTier },
          );
        } else {
          addLine(lines, [
            [rightRail, railTop],
            [rightRail, collectY],
          ]);
        }
        const activeTargetXs = targets
          .map((box, index) => (targetTiers[index] ? centreX(box) : undefined))
          .filter((value): value is number => value !== undefined);
        drawHorizontalBus(
          lines,
          Math.min(...targets.map(centreX)),
          rightRail,
          collectY,
          rightRail,
          activeTargetXs,
          bridgeTier ? maxTrafficTier(...targetTiers) : undefined,
        );
      }

      targets.forEach((box, index) => {
        const group = lowerGroups[index];
        addLine(
          lines,
          [
            [centreX(box), collectY],
            [centreX(box), box.y],
          ],
          { arrow: true, activity: targetTiers[index] },
        );
        connectorLabel(
          labels,
          lowerConnectorLabel(group),
          centreX(box),
          (collectY + box.y) / 2 + 2,
        );
      });

      if (wrapped.length > 0) {
        const reach = Math.max(...wrapped.map((box) => box.y)) - 18;
        const maxTargetX = Math.max(...targets.map(centreX));
        let lastActiveWrapped = -1;
        for (let index = 0; index < wrappedTiers.length; index += 1) {
          if (wrappedTiers[index]) lastActiveWrapped = index;
        }
        if (lastActiveWrapped >= 0 && wrappedMaxTier && allHostsTier) {
          const activeReach = wrapped[lastActiveWrapped].y - 18;
          addLine(
            lines,
            [
              [maxTargetX, collectY],
              [rightRail, collectY],
              [rightRail, activeReach],
            ],
            { activity: maxTrafficTier(allHostsTier, wrappedMaxTier) },
          );
          if (activeReach < reach) {
            addLine(lines, [
              [rightRail, activeReach],
              [rightRail, reach],
            ]);
          }
        } else {
          addLine(lines, [
            [maxTargetX, collectY],
            [rightRail, collectY],
            [rightRail, reach],
          ]);
        }
        wrapped.forEach((box, index) => {
          const busY = box.y - 18;
          addLine(
            lines,
            [
              [rightRail, busY],
              [centreX(box), busY],
              [centreX(box), box.y],
            ],
            { arrow: true, activity: wrappedTiers[index] },
          );
          connectorLabel(
            labels,
            lowerConnectorLabel(lowerGroups[targets.length + index]),
            centreX(box),
            busY,
          );
        });
      }
    } else if (source && targets.length > 0) {
      // Fallback when there are no monitored host frames (for example checks-only estates)
      const busY = targets[0].y - 18;
      addLine(
        lines,
        [
          [source.x, source.y],
          [source.x, busY],
        ],
        { activity: anyLowerTier },
      );
      const xs = [...targets.map(centreX), source.x];
      const activeXs = targets
        .map((box, index) => (lowerTiers[index] ? centreX(box) : undefined))
        .filter((value): value is number => value !== undefined);
      drawHorizontalBus(
        lines,
        Math.min(...xs),
        Math.max(...xs),
        busY,
        source.x,
        activeXs,
        anyLowerTier,
      );
      targets.forEach((box, index) => {
        addLine(
          lines,
          [
            [centreX(box), busY],
            [centreX(box), box.y],
          ],
          { arrow: true, activity: lowerTiers[index] },
        );
      });
    }

    y = lowerBottom + LAYER_GAP;
  } else if (placedUnits.length > 0) {
    y = hostsBottom + LAYER_GAP;
  }

  // Wire stated connections onto drawn hits so hover arcs, rings, and labels work identically
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
      if (link.label && !end.linkLabels?.[other.id]) {
        end.linkLabels = { ...(end.linkLabels ?? {}), [other.id]: link.label };
      }
    }
  }

  const height = drawPosterFooter(out, options.header, margin, width, lastBottom);
  return {
    width,
    height: Math.ceil(height),
    nodes: [...lines, ...out.nodes, ...labels, ...(out.inspects ?? []), ...out.hits],
  };
}
