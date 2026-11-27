import {
  getAvailabilityTargetKindInfo,
  isNetworkGearKind,
  type AvailabilityNetworkGearKind,
} from '@/utils/availabilityTargetKinds';
import {
  MAP_CATEGORY_LABEL,
  MAP_CATEGORY_ORDER,
  type MapAppRole,
  type MapCategoryId,
} from './infrastructureMapCatalogue';
import { mapRouteLabel } from './infrastructureMapConnections';
import type { MapGlyphKey } from './infrastructureMapLayout';
import {
  resolveMapTrafficTier,
  type InfrastructureMapModel,
  type MapHealth,
  type MapHost,
  type MapItem,
  type MapMount,
  type MapPool,
  type MapTrafficTier,
} from './infrastructureMapModel';
import {
  OTHER_SEGMENT_ID,
  type MapSegment,
  type MapSegmentKind,
} from './infrastructureMapNetworks';

/**
 * The estate as a service diagram: what it runs, boxed by what each thing is
 * for, between how traffic gets in at the top and the storage and hardware
 * underneath. It is a second reading of the same map model, so it draws only
 * what Pulse monitors. Two drawn things are not monitored: the Internet
 * itself, shown only when an app whose whole job is to reach it (a tunnel, a
 * VPN, a reverse proxy) is running, and a default gateway that machines name
 * in their own route tables and Pulse knows only by its address.
 *
 * A link between two tiles is a connection a collected value states: a
 * container that uses another container's network, a reverse proxy and a
 * container it routes, a machine and what runs on it, a host and the storage
 * set up on it, or a guest and the pool its disk or a mount is on. No line is
 * drawn for one. It is written under the tile or on its hover, and the layout
 * tells each end the other. Where both ends are not drawn there is no link.
 *
 * A mount and a pool are one filesystem only when they are the same size and
 * as full to within a twentieth of a percent, which two separate filesystems
 * with anything on them almost never are. Nothing else on the diagram is matched.
 */

export interface DiagramTile {
  /** A resource id, or a made-up key for a tile that stands for several. */
  id: string;
  name: string;
  caption: string;
  typeLabel: string;
  status: string;
  health: MapHealth;
  /** A logo key for an app the catalogue knows. */
  icon?: string;
  /** An outline glyph, for hardware, storage and the Internet. */
  platform?: MapGlyphKey;
  /** The first network the thing is on, as an id into `networks`. */
  segmentId?: string;
  /** Every planned network the thing is attached to, for accurate drawer membership. */
  segmentIds?: string[];
  /** True when the tile caption states a collected connection (`via` container or route). */
  hasConnectionCaption?: boolean;
  /** Human-readable purpose category for workload tiles (e.g. `Media`, `Monitoring`, `Other`). */
  categoryLabel?: string;
  /** True when targeted by a reverse proxy route or Kubernetes Ingress. */
  isRouted?: boolean;
  vmid?: number;
  composeProject?: string;
  hostId?: string;
  hostName?: string;
  cpuCores?: number;
  memoryTotalBytes?: number;
  versionLabel?: string;
  temperatureC?: number;
  diskSummary?: string;
  diskMaxTempC?: number;
  unhealthyDiskCount?: number;
  usedPercent?: number;
  totalBytes?: number;
  sizeLabel?: string;
  vdevLayout?: string;
  storageType?: string;
  uptimeSeconds?: number;
  pendingUpdates?: number;
  lastBackupAt?: string;
  zfsState?: string;
  rebuildInProgress?: boolean;
  zfsErrorCount?: number;
  cpu?: number;
  memory?: number;
  netRxBytes?: number;
  netTxBytes?: number;
  netTotalBytes?: number;
  trafficTier?: MapTrafficTier;
  latencyMs?: number;
  linkSpeedMbps?: number;
  linkUtilizationPercent?: number;
  linkDownCount?: number;
  firewallEnabled?: boolean;
  dockerNetworks?: string[];
  probeSummary?: string;
  certWarning?: string;
  address?: string;
  detail?: string;
  /** What the thing's connections say about it, one sentence each, for the hover. */
  links?: string[];
  /** False for a tile that stands for several things and opens none. */
  navigable: boolean;
  /**
   * The address of a thing the map draws and nothing monitors. Its tile opens
   * the form that adds a check for it, with the address filled in.
   */
  checkAddress?: string;
}

/** A stated connection between two drawn tiles. No line is drawn for it. */
export interface DiagramLink {
  fromId: string;
  toId: string;
  /** Short factual relationship label shown on hover connection arcs when <= 6 arcs are drawn. */
  label?: string;
}

export interface DiagramGroup {
  key: string;
  label: string;
  /** How many things the box holds, including any folded into one tile. */
  count: number;
  tiles: DiagramTile[];
  /** Every individual resource tile in this group before capping or folding. */
  allTiles: DiagramTile[];
  /** How many tiles in this group have critical or attention health. */
  attentionCount: number;
}

export interface DiagramNetwork {
  id: string;
  label: string;
  /** What the network was read from: a Proxmox bridge counts what is plugged in. */
  kind: MapSegmentKind;
  /** The subnet a bridge carries, whose bridge it is, or the VLANs folded into it. */
  detail?: string;
  memberCount: number;
  colorIndex: number;
  netTotalBytes?: number;
  trafficTier?: MapTrafficTier;
  linkSpeedMbps?: number;
  linkDownCount?: number;
}

export interface DiagramHostStack {
  key: string;
  label: string;
  tiles: DiagramTile[];
}

export interface DiagramHostFrame {
  id: string;
  host: DiagramTile;
  clusterId?: string;
  clusterName?: string;
  clusterTypeLabel?: string;
  clusterPools?: DiagramTile[];
  stacks: DiagramHostStack[];
  looseTiles: DiagramTile[];
  pools: DiagramTile[];
  allTiles: DiagramTile[];
  attentionCount: number;
  totalCount: number;
  overflowCount: number;
  trafficTier?: MapTrafficTier;
  linkSpeedMbps?: number;
}

export interface DiagramExposedEntry {
  tile: DiagramTile;
  groupLabel?: string;
  hostnames: string[];
  ports?: string;
  proxyName?: string;
}

/**
 * How many networks the diagram names. Each one has a colour of its own, and
 * a tile is marked only with the colour of a network that is named, so two
 * unrelated networks never share a mark.
 */
export const DIAGRAM_MAX_NETWORKS = 6;

export interface ServiceDiagram {
  /** Set when something running exists to reach the Internet. */
  internet?: { label: string };
  /** Default gateways no monitored thing answers on, known only by address. */
  gateways: DiagramTile[];
  access?: DiagramGroup;
  links: DiagramLink[];
  networks: DiagramNetwork[];
  groups: DiagramGroup[];
  hostFrames: DiagramHostFrame[];
  sharedStorage?: DiagramGroup;
  checks?: DiagramGroup;
  exposed: DiagramExposedEntry[];
  storage?: DiagramGroup;
  backups?: DiagramGroup;
  hardware?: DiagramGroup;
  serviceCount: number;
  /** Aggregate live estate network throughput in bytes per second, avoiding host/guest double-counting. */
  estateTrafficBytes?: number;
  /** True when any drawn host frame, hardware tile, or network reports a positive physical link speed. */
  hasLinkSpeeds?: boolean;
}

export interface ServiceDiagramFilter {
  /** Keep only resources whose health is critical or attention. */
  attentionOnly?: boolean;
  /** Keep only resources with active live network traffic (> 0 B/s). */
  activeTrafficOnly?: boolean;
  /** Case-insensitive search query matched across names, types, hosts, addresses, and groups. */
  query?: string;
  /** Keep only resources belonging to this planned network id. */
  networkId?: string;
}

export interface ServiceDiagramOptions {
  /** The most tiles one group draws before the rest fold into one. */
  maxTilesPerGroup?: number;
  /** Optional filter applied across the full inventory before per-group capping. */
  filter?: ServiceDiagramFilter;
}

export const DIAGRAM_MAX_TILES_PER_GROUP = 24;
/** The most gateway tiles drawn. An estate has one or two. */
export const DIAGRAM_MAX_GATEWAYS = 3;

const SERVICE_SECTION = /^(vms|lxcs|containers|ns:.*)$/;
const ROLE_ORDER: readonly MapAppRole[] = [
  'tunnel',
  'VPN',
  'reverse proxy',
  'DNS',
  'remote desktop',
];
// The roles that carry traffic between the estate and the Internet. DNS and
// remote desktop are ways in to the estate without being a path to it.
const INTERNET_ROLES = new Set<MapAppRole>(['tunnel', 'VPN', 'reverse proxy']);

const HEALTH_RANK: Record<MapHealth, number> = {
  critical: 5,
  attention: 4,
  stale: 3,
  unknown: 2,
  ok: 1,
  off: 0,
};
const worst = (a: MapHealth, b: MapHealth): MapHealth => (HEALTH_RANK[a] >= HEALTH_RANK[b] ? a : b);

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

const plural = (count: number, one: string, many: string): string =>
  `${count.toLocaleString()} ${count === 1 ? one : many}`;

// Up to three names, then how many more: enough to recognise a short list
// and short enough for one line of a hover.
const nameList = (names: string[]): string => {
  const sorted = [...names].sort(collator.compare);
  return sorted.length <= 3
    ? sorted.join(', ')
    : `${sorted.slice(0, 3).join(', ')} +${sorted.length - 3}`;
};

const hostsOf = (model: InfrastructureMapModel): MapHost[] =>
  model.blocks.flatMap((block) => (block.kind === 'group' ? block.group.hosts : [block.host]));

export const formatMapLinkSpeed = (speedMbps: number): string =>
  speedMbps >= 1000 ? `${Number((speedMbps / 1000).toFixed(1))}G` : `${Math.round(speedMbps)}M`;

export const formatMapLatency = (latencyMs: number): string => {
  if (latencyMs < 1) return '<1 ms';
  if (latencyMs < 10) return `${Math.round(latencyMs * 10) / 10} ms`;
  return `${Math.round(latencyMs)} ms`;
};

const tileNetworkTelemetry = (
  source: Pick<
    MapHost,
    'netRxBytes' | 'netTxBytes' | 'linkSpeedMbps' | 'linkUtilizationPercent' | 'linkDownCount'
  > & {
    latencyMs?: number;
    firewallEnabled?: boolean;
  },
): Pick<
  DiagramTile,
  | 'netRxBytes'
  | 'netTxBytes'
  | 'netTotalBytes'
  | 'trafficTier'
  | 'latencyMs'
  | 'linkSpeedMbps'
  | 'linkUtilizationPercent'
  | 'linkDownCount'
  | 'firewallEnabled'
> => {
  const hasNet = source.netRxBytes !== undefined || source.netTxBytes !== undefined;
  const netTotalBytes = hasNet ? (source.netRxBytes ?? 0) + (source.netTxBytes ?? 0) : undefined;
  const trafficTier = resolveMapTrafficTier(netTotalBytes);
  return {
    ...(source.netRxBytes !== undefined ? { netRxBytes: source.netRxBytes } : {}),
    ...(source.netTxBytes !== undefined ? { netTxBytes: source.netTxBytes } : {}),
    ...(netTotalBytes !== undefined ? { netTotalBytes } : {}),
    ...(trafficTier !== undefined ? { trafficTier } : {}),
    ...(source.latencyMs !== undefined ? { latencyMs: source.latencyMs } : {}),
    ...(source.linkSpeedMbps !== undefined ? { linkSpeedMbps: source.linkSpeedMbps } : {}),
    ...(source.linkUtilizationPercent !== undefined
      ? { linkUtilizationPercent: source.linkUtilizationPercent }
      : {}),
    ...(source.linkDownCount !== undefined ? { linkDownCount: source.linkDownCount } : {}),
    ...(source.firewallEnabled ? { firewallEnabled: true } : {}),
  };
};

const annotateNetworkMetrics = (
  model: InfrastructureMapModel,
  networks: DiagramNetwork[],
  networkOf: (segmentId: string | undefined) => string | undefined,
): DiagramNetwork[] => {
  const statsByNetwork = new Map<
    string,
    {
      hasWorkloadNet: boolean;
      workloadNetSum: number;
      hasHostNet: boolean;
      hostNetSum: number;
      maxLinkSpeed: number;
      linkDownCount: number;
    }
  >();
  const statsFor = (id: string) => {
    let stats = statsByNetwork.get(id);
    if (!stats) {
      stats = {
        hasWorkloadNet: false,
        workloadNetSum: 0,
        hasHostNet: false,
        hostNetSum: 0,
        maxLinkSpeed: 0,
        linkDownCount: 0,
      };
      statsByNetwork.set(id, stats);
    }
    return stats;
  };
  const plannedIds = (segmentIds: readonly string[] | undefined): string[] => [
    ...new Set(
      (segmentIds ?? [])
        .map((segmentId) => networkOf(segmentId))
        .filter((id): id is string => Boolean(id)),
    ),
  ];

  for (const host of hostsOf(model)) {
    if (host.isResource) {
      const hostHasNet = host.netRxBytes !== undefined || host.netTxBytes !== undefined;
      const hostTotal = hostHasNet ? (host.netRxBytes ?? 0) + (host.netTxBytes ?? 0) : 0;
      for (const id of plannedIds(host.ownSegmentIds)) {
        const stats = statsFor(id);
        if (hostHasNet) {
          stats.hasHostNet = true;
          stats.hostNetSum += hostTotal;
        }
        if (host.linkSpeedMbps !== undefined && host.linkSpeedMbps > stats.maxLinkSpeed) {
          stats.maxLinkSpeed = host.linkSpeedMbps;
        }
        if (host.linkDownCount !== undefined && host.linkDownCount > 0) {
          stats.linkDownCount += host.linkDownCount;
        }
      }
    }
    for (const section of host.sections) {
      if (!SERVICE_SECTION.test(section.key)) continue;
      for (const item of section.items) {
        const itemHasNet = item.netRxBytes !== undefined || item.netTxBytes !== undefined;
        if (!itemHasNet) continue;
        const itemTotal = (item.netRxBytes ?? 0) + (item.netTxBytes ?? 0);
        for (const id of plannedIds(item.segmentIds)) {
          const stats = statsFor(id);
          stats.hasWorkloadNet = true;
          stats.workloadNetSum += itemTotal;
        }
      }
    }
  }

  return networks.map((network) => {
    const stats = statsByNetwork.get(network.id);
    if (!stats) return network;
    const netTotalBytes = stats.hasWorkloadNet
      ? stats.workloadNetSum
      : stats.hasHostNet
        ? stats.hostNetSum
        : undefined;
    const trafficTier = resolveMapTrafficTier(netTotalBytes);
    return {
      ...network,
      ...(netTotalBytes !== undefined ? { netTotalBytes } : {}),
      ...(trafficTier !== undefined ? { trafficTier } : {}),
      ...(stats.maxLinkSpeed > 0 ? { linkSpeedMbps: stats.maxLinkSpeed } : {}),
      ...(stats.linkDownCount > 0 ? { linkDownCount: stats.linkDownCount } : {}),
    };
  });
};

interface NetworkPlan {
  networks: DiagramNetwork[];
  /** The named network a tile on a segment is marked with. */
  networkOf: (segmentId: string | undefined) => string | undefined;
  /** The address a tile answers on, with the bridge or port group it is on. */
  where: (
    address: string | undefined,
    segmentIds: readonly string[] | undefined,
  ) => string | undefined;
}

// The tags a bridge carries, short enough for a pill. The bridge's own name
// already stands for its untagged network.
const vlanPhrase = (vlans: number[]): string => {
  const tags = vlans.filter((vlan) => vlan > 0).sort((a, b) => a - b);
  if (tags.length === 0) return '';
  const shown = tags.slice(0, 4).join(', ') + (tags.length > 4 ? ` +${tags.length - 4}` : '');
  return `${tags.length === 1 ? 'VLAN' : 'VLANs'} ${shown}`;
};

/**
 * Which networks the diagram names. Up to the cap, each network is one. Past
 * it, the VLANs of one Proxmox bridge are named together, the way its owner
 * says it ("vmbr0, VLANs 20, 30 and 40"), and what still does not fit is left
 * to the "+N more" pill, unmarked. A tile's hover always names the exact
 * bridge and VLAN, so folding hides nothing.
 */
const planNetworks = (model: InfrastructureMapModel): NetworkPlan => {
  const segments = model.segments.filter((segment) => segment.id !== OTHER_SEGMENT_ID);
  // A subnet or range only repeats the address beside it.
  const names = new Map(
    segments
      .filter((segment) => segment.kind === 'bridge' || segment.kind === 'portgroup')
      .map((segment) => [segment.id, segment.label]),
  );
  const where: NetworkPlan['where'] = (address, segmentIds) => {
    const on = (segmentIds ?? []).flatMap((id) => names.get(id) ?? []);
    return (
      [address, on.length > 0 ? `on ${on.join(' and ')}` : ''].filter(Boolean).join(' ') ||
      undefined
    );
  };
  const plain = ({ id, label, kind, detail, memberCount, colorIndex }: MapSegment) => ({
    id,
    label,
    kind,
    ...(detail ? { detail } : {}),
    memberCount,
    colorIndex,
  });
  if (segments.length <= DIAGRAM_MAX_NETWORKS) {
    const networkOf: NetworkPlan['networkOf'] = (segmentId) => segmentId;
    return {
      networks: annotateNetworkMetrics(model, segments.map(plain), networkOf),
      networkOf,
      where,
    };
  }

  // Fold the networks of one bridge. A bridge with a single network gains
  // nothing from it and keeps its own name.
  const byBridge = new Map<string, MapSegment[]>();
  for (const segment of segments) {
    if (!segment.bridge) continue;
    const key = `bridge:${segment.bridge.scope}:${segment.bridge.name}`;
    byBridge.set(key, [...(byBridge.get(key) ?? []), segment]);
  }
  const foldOf = new Map<string, string>();
  for (const [key, members] of byBridge) {
    if (members.length < 2) continue;
    for (const member of members) foldOf.set(member.id, key);
  }
  // A guest on two VLANs of one bridge is connected to that bridge once.
  const connected = new Map<string, Set<string>>();
  const count = (id: string, segmentIds: readonly string[] | undefined) => {
    for (const segmentId of segmentIds ?? []) {
      const fold = foldOf.get(segmentId);
      if (!fold) continue;
      connected.set(fold, (connected.get(fold) ?? new Set<string>()).add(id));
    }
  };
  for (const host of hostsOf(model)) {
    count(host.id, host.ownSegmentIds);
    for (const section of host.sections) {
      for (const item of section.items) count(item.id, item.segmentIds);
    }
  }
  const folded: DiagramNetwork[] = [...byBridge]
    .filter(([, members]) => members.length > 1)
    .map(([key, members]) => ({
      id: key,
      label: members[0].bridge?.name ?? members[0].label,
      kind: 'bridge' as const,
      detail: vlanPhrase(members.map((member) => member.bridge?.vlan ?? 0)),
      memberCount:
        connected.get(key)?.size ?? members.reduce((sum, member) => sum + member.memberCount, 0),
      colorIndex: 0,
    }));
  const scopeOf = new Map(
    [...byBridge].map(([key, members]) => [key, members[0].bridge?.scope ?? '']),
  );
  const kept = segments.filter((segment) => !foldOf.has(segment.id)).map(plain);
  // Two clusters can both have a `vmbr0`; say whose each one is.
  const uses = new Map<string, number>();
  for (const network of [...folded, ...kept]) {
    uses.set(network.label, (uses.get(network.label) ?? 0) + 1);
  }
  const networks = [
    ...folded.map((network) => {
      // One fact per pill: whose bridge it is where that tells two apart,
      // otherwise the tags. A tile's hover has the exact VLAN either way.
      const whose = (uses.get(network.label) ?? 0) > 1 ? scopeOf.get(network.id) : '';
      const { detail: tags, ...rest } = network;
      const detail = whose || tags;
      return { ...rest, ...(detail ? { detail } : {}) };
    }),
    ...kept,
  ]
    .sort((a, b) => b.memberCount - a.memberCount || collator.compare(a.label, b.label))
    .map((network, index) => ({ ...network, colorIndex: index }));
  const networkOf: NetworkPlan['networkOf'] = (segmentId) =>
    foldOf.get(segmentId ?? '') ?? segmentId;
  return {
    networks: annotateNetworkMetrics(model, networks, networkOf),
    networkOf,
    where,
  };
};

const plannedTileNetworks = (
  segmentIds: readonly string[] | undefined,
  plan: NetworkPlan,
): { segmentId?: string; segmentIds?: string[] } => {
  if (!segmentIds?.length) return {};
  const mapped = [
    ...new Set(
      segmentIds.map((id) => plan.networkOf(id)).filter((id): id is string => Boolean(id)),
    ),
  ];
  if (mapped.length === 0) return {};
  return { segmentId: mapped[0], segmentIds: mapped };
};

const serviceTile = (item: MapItem, host: MapHost, plan: NetworkPlan): DiagramTile => {
  const ingressHosts = item.ingressHosts?.length ? item.ingressHosts : undefined;
  return {
    id: item.id,
    name: item.name,
    caption: ingressHosts ? mapRouteLabel(ingressHosts) : (item.app?.role ?? host.name),
    typeLabel: item.app ? `${item.app.label} · ${item.typeLabel}` : item.typeLabel,
    status: item.status,
    health: item.health,
    ...(item.app?.icon ? { icon: item.app.icon } : {}),
    ...plannedTileNetworks(item.segmentIds, plan),
    ...(ingressHosts ? { hasConnectionCaption: true } : {}),
    categoryLabel: MAP_CATEGORY_LABEL[item.category ?? 'other'],
    ...(item.vmid !== undefined ? { vmid: item.vmid } : {}),
    ...(item.composeProject ? { composeProject: item.composeProject } : {}),
    ...(host.isResource ? { hostId: host.id, hostName: host.name } : {}),
    cpu: item.cpu,
    memory: item.memory,
    ...(item.uptimeSeconds !== undefined ? { uptimeSeconds: item.uptimeSeconds } : {}),
    ...(item.pendingUpdates !== undefined ? { pendingUpdates: item.pendingUpdates } : {}),
    ...(item.lastBackupAt ? { lastBackupAt: item.lastBackupAt } : {}),
    ...tileNetworkTelemetry(item),
    ...(item.dockerNetworks?.length ? { dockerNetworks: item.dockerNetworks } : {}),
    address: plan.where(item.address, item.segmentIds),
    detail: item.detail,
    ...(ingressHosts ? { links: [`Ingress: ${ingressHosts.join(', ')}`] } : {}),
    navigable: true,
  };
};

const GEAR_GLYPH: Record<AvailabilityNetworkGearKind, MapGlyphKey> = {
  router: 'gateway',
  switch: 'switch',
  access_point: 'access-point',
};

// A check on a router, a switch or an access point. It is drawn as the
// device and says which, where a plain check says its address.
const gearTile = (
  item: MapItem,
  kind: AvailabilityNetworkGearKind,
  plan: NetworkPlan,
): DiagramTile => {
  const noun = getAvailabilityTargetKindInfo(kind)?.noun ?? kind;
  return {
    id: item.id,
    name: item.name,
    caption: item.latencyMs !== undefined ? `${noun} · ${formatMapLatency(item.latencyMs)}` : noun,
    typeLabel: item.typeLabel,
    status: item.status,
    health: item.health,
    platform: GEAR_GLYPH[kind],
    ...plannedTileNetworks(item.segmentIds, plan),
    address: plan.where(item.address, item.segmentIds),
    ...tileNetworkTelemetry(item),
    ...(item.probeSummary ? { probeSummary: item.probeSummary } : {}),
    ...(item.certWarning ? { certWarning: item.certWarning } : {}),
    navigable: true,
  };
};

const foldTile = (key: string, name: string, caption: string): DiagramTile => ({
  id: key,
  name,
  caption,
  typeLabel: caption,
  status: '',
  health: 'off',
  navigable: false,
});

export const isDiagramTileAttention = (health: MapHealth): boolean =>
  health === 'critical' || health === 'attention';

/** Sort tiles with Critical first, then Needs attention, then Stale/Unknown, Healthy, Off, then name. */
export const compareDiagramTilesByAttention = (a: DiagramTile, b: DiagramTile): number =>
  HEALTH_RANK[b.health] - HEALTH_RANK[a.health] || collator.compare(a.name, b.name);

/** Sort tiles by live combined network throughput descending, then attention and name. */
export const compareDiagramTilesByTraffic = (a: DiagramTile, b: DiagramTile): number =>
  (b.netTotalBytes ?? -1) - (a.netTotalBytes ?? -1) || compareDiagramTilesByAttention(a, b);

/** Multi-term case-insensitive search matcher across a tile's visible and contextual metadata. */
export const matchesDiagramTileQuery = (
  tile: DiagramTile,
  query: string,
  extraText = '',
): boolean => {
  const tokens = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return true;
  const haystack = [
    tile.name,
    tile.caption,
    tile.typeLabel,
    tile.status,
    tile.health,
    tile.vmid !== undefined ? String(tile.vmid) : undefined,
    tile.composeProject,
    tile.hostName,
    tile.versionLabel,
    tile.diskSummary,
    tile.sizeLabel,
    tile.vdevLayout,
    tile.storageType,
    tile.zfsState,
    tile.rebuildInProgress ? 'rebuilding' : undefined,
    tile.address,
    tile.detail,
    ...(tile.dockerNetworks ?? []),
    tile.probeSummary,
    tile.certWarning,
    extraText,
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
  return tokens.every((token) => haystack.includes(token));
};

/** Return every populated group in a service diagram in visual order. */
export const collectDiagramGroups = (diagram: ServiceDiagram): DiagramGroup[] =>
  [diagram.access, ...diagram.groups, diagram.storage, diagram.backups, diagram.hardware].filter(
    (group): group is DiagramGroup => group !== undefined,
  );

/** Find a group by key, including shared storage, checks, and `host:<id>` host frames. */
export const findDiagramGroup = (
  diagram: ServiceDiagram,
  groupKey: string,
): DiagramGroup | undefined => {
  const direct = collectDiagramGroups(diagram).find((group) => group.key === groupKey);
  if (direct) return direct;
  if (diagram.sharedStorage?.key === groupKey) return diagram.sharedStorage;
  if (diagram.checks?.key === groupKey) return diagram.checks;
  if (groupKey.startsWith('host:')) {
    const frame = diagram.hostFrames.find((candidate) => `host:${candidate.id}` === groupKey);
    if (frame) {
      return {
        key: `host:${frame.id}`,
        label: frame.host.name,
        count: frame.allTiles.length,
        tiles: [...frame.stacks.flatMap((stack) => stack.tiles), ...frame.looseTiles],
        allTiles: frame.allTiles,
        attentionCount: frame.attentionCount,
      };
    }
  }
  return undefined;
};

/** Return every exposed service (reverse-proxied hostnames or published ports) in the diagram. */
export const collectExposedEntries = (diagram: ServiceDiagram): DiagramExposedEntry[] =>
  diagram.exposed;

/** Count all individual resources represented across every group in a service diagram. */
export const countDiagramResources = (diagram: ServiceDiagram): number =>
  collectDiagramGroups(diagram).reduce((sum, group) => sum + group.allTiles.length, 0);

/** Return every individual resource tile attached to the given planned network id, sorted by attention. */
export const collectNetworkTiles = (
  diagram: ServiceDiagram,
  networkId: string,
): Array<{ tile: DiagramTile; groupLabel: string }> => {
  const seen = new Set<string>();
  const out: Array<{ tile: DiagramTile; groupLabel: string }> = [];
  for (const group of collectDiagramGroups(diagram)) {
    for (const tile of group.allTiles) {
      if (seen.has(tile.id)) continue;
      if (tile.segmentIds?.includes(networkId) || tile.segmentId === networkId) {
        seen.add(tile.id);
        out.push({ tile, groupLabel: group.label });
      }
    }
  }
  return out.sort((a, b) => compareDiagramTilesByAttention(a.tile, b.tile));
};

/** Return every unique navigable tile across the diagram with live traffic (> 0 B/s), busiest first. */
export const collectActiveTrafficTiles = (
  diagram: ServiceDiagram,
): Array<{ tile: DiagramTile; groupLabel: string }> => {
  const seen = new Set<string>();
  const out: Array<{ tile: DiagramTile; groupLabel: string }> = [];
  for (const group of collectDiagramGroups(diagram)) {
    for (const tile of group.allTiles) {
      if (!tile.navigable || seen.has(tile.id)) continue;
      if (tile.netTotalBytes !== undefined && tile.netTotalBytes > 0) {
        seen.add(tile.id);
        out.push({ tile, groupLabel: group.label });
      }
    }
  }
  return out.sort((a, b) => compareDiagramTilesByTraffic(a.tile, b.tile));
};

// Running things first, then by name, so the tiles that fold away when a
// group is large are the ones that are switched off.
const byRunningThenName = (a: DiagramTile, b: DiagramTile): number =>
  Number(a.health === 'off') - Number(b.health === 'off') || collator.compare(a.name, b.name);

const capTiles = (key: string, tiles: DiagramTile[], max: number): DiagramTile[] => {
  if (tiles.length <= max) return tiles;
  const rest = tiles.length - (max - 1);
  return [...tiles.slice(0, max - 1), foldTile(`${key}:more`, `+${rest}`, 'more')];
};

// A box of tiles, cut to size, that still knows how many things it stands for.
const box = (
  key: string,
  label: string,
  tiles: DiagramTile[],
  max: number,
  foldedTiles: DiagramTile[] = [],
): DiagramGroup => {
  const allTiles = [...tiles.filter((tile) => tile.navigable), ...foldedTiles];
  return {
    key,
    label,
    count: allTiles.length,
    tiles: capTiles(key, tiles, max),
    allTiles,
    attentionCount: allTiles.filter((tile) => isDiagramTileAttention(tile.health)).length,
  };
};

interface PoolEntry {
  pool: MapPool;
  /** What the pool belongs to when no host is named: its cluster. */
  owner: string;
  /** The hosts the pool is on or set up on. */
  hostIds: readonly string[];
}

/** A drawn pool tile and every pool it stands for. */
interface PoolTile {
  tile: DiagramTile;
  pools: MapPool[];
  hostIds: Set<string>;
}

const poolTiles = (
  pools: PoolEntry[],
  platform: MapGlyphKey,
  hostName: (id: string) => string | undefined,
): PoolTile[] => {
  // The same pool name on several hosts is one tile: a cluster's `local` is
  // three disks, and three identical tiles would say less than one.
  const byName = new Map<
    string,
    {
      pool: MapPool;
      pools: MapPool[];
      owners: Set<string>;
      hostIds: Set<string>;
      health: MapHealth;
    }
  >();
  for (const { pool, owner, hostIds } of pools) {
    // Hosts Pulse draws say more than the cluster they are in.
    const named = hostIds.map(hostName).filter((name): name is string => Boolean(name));
    const owners = named.length > 0 ? named : owner ? [owner] : [];
    const entry = byName.get(pool.name);
    if (entry) {
      for (const name of owners) entry.owners.add(name);
      for (const id of hostIds) entry.hostIds.add(id);
      entry.pools.push(pool);
      entry.health = worst(entry.health, pool.health);
    } else {
      byName.set(pool.name, {
        pool,
        pools: [pool],
        owners: new Set(owners),
        hostIds: new Set(hostIds),
        health: pool.health,
      });
    }
  }
  return [...byName.values()]
    .map(({ pool, pools: members, owners, hostIds, health }): PoolTile => {
      const caption =
        owners.size > 1 ? plural(owners.size, 'host', 'hosts') : ([...owners][0] ?? 'Shared');
      const usedPercent = members.reduce<number | undefined>(
        (acc, member) =>
          member.usedPercent === undefined
            ? acc
            : acc === undefined
              ? member.usedPercent
              : Math.max(acc, member.usedPercent),
        undefined,
      );
      const totalBytes = members.reduce<number | undefined>((maxBytes, member) => {
        if (member.totalBytes === undefined || !(member.totalBytes > 0)) return maxBytes;
        return maxBytes === undefined || member.totalBytes > maxBytes
          ? member.totalBytes
          : maxBytes;
      }, undefined);
      const sizeLabel = members.find((member) => member.sizeLabel)?.sizeLabel;
      const vdevLayout = members.find((member) => member.vdevLayout)?.vdevLayout;
      const storageType = members.find((member) => member.storageType)?.storageType;
      const zfsState =
        members.find(
          (member) => member.zfsState && /FAULT|OFFLINE|UNAVAIL|FAIL/i.test(member.zfsState),
        )?.zfsState ?? members.find((member) => member.zfsState)?.zfsState;
      const rebuildInProgress = members.some((member) => member.rebuildInProgress === true);
      const zfsErrorTotal = members.reduce((sum, member) => sum + (member.zfsErrorCount ?? 0), 0);
      return {
        tile: {
          id: pool.id,
          name: pool.name,
          caption,
          typeLabel: [pool.backup ? 'Backup target' : 'Storage', sizeLabel]
            .filter(Boolean)
            .join(' · '),
          status: usedPercent === undefined ? '' : `${Math.round(usedPercent)}% used`,
          health,
          platform,
          ...(usedPercent !== undefined ? { usedPercent } : {}),
          ...(totalBytes !== undefined ? { totalBytes } : {}),
          ...(sizeLabel ? { sizeLabel } : {}),
          ...(vdevLayout ? { vdevLayout } : {}),
          ...(storageType ? { storageType } : {}),
          ...(zfsState ? { zfsState } : {}),
          ...(rebuildInProgress ? { rebuildInProgress: true } : {}),
          ...(zfsErrorTotal > 0 ? { zfsErrorCount: zfsErrorTotal } : {}),
          navigable: true,
        },
        pools: members,
        hostIds,
      };
    })
    .sort((a, b) => collator.compare(a.tile.name, b.tile.name));
};

const localPoolTile = (
  pool: MapPool,
  host: MapHost,
  drawn: ReadonlyMap<string, DiagramTile>,
  allTilesById: ReadonlyMap<string, DiagramTile>,
): DiagramTile => {
  const existing = drawn.get(pool.id);
  let links = existing?.links;
  if (!links && pool.consumerIds?.length) {
    const consumerNames = [
      ...new Set(
        pool.consumerIds
          .map((id) => allTilesById.get(id)?.name)
          .filter((name): name is string => Boolean(name)),
      ),
    ];
    if (consumerNames.length > 0) links = [`Used by ${nameList(consumerNames)}`];
  }
  return {
    id: pool.id,
    name: pool.name,
    caption: host.name,
    typeLabel: ['Storage', pool.sizeLabel].filter(Boolean).join(' · '),
    status: pool.usedPercent === undefined ? '' : `${Math.round(pool.usedPercent)}% used`,
    health: pool.health,
    platform: 'storage',
    hostId: host.id,
    hostName: host.name,
    ...(pool.usedPercent !== undefined ? { usedPercent: pool.usedPercent } : {}),
    ...(pool.totalBytes !== undefined && pool.totalBytes > 0
      ? { totalBytes: pool.totalBytes }
      : {}),
    ...(pool.sizeLabel ? { sizeLabel: pool.sizeLabel } : {}),
    ...(pool.vdevLayout ? { vdevLayout: pool.vdevLayout } : {}),
    ...(pool.storageType ? { storageType: pool.storageType } : {}),
    ...(pool.zfsState ? { zfsState: pool.zfsState } : {}),
    ...(pool.rebuildInProgress ? { rebuildInProgress: true } : {}),
    ...(pool.zfsErrorCount !== undefined ? { zfsErrorCount: pool.zfsErrorCount } : {}),
    ...(links?.length ? { links: [...links] } : {}),
    navigable: true,
  };
};

/**
 * How close two readings of one filesystem have to be. A share is read by
 * the guest that mounts it and by the machine that holds it, at different
 * moments and with different rounding, so the bytes never match exactly; a
 * filesystem being written to moves between two polls too. Two different
 * filesystems the same size are told apart by what is on them, so a nearly
 * empty one is never matched.
 */
const SAME_FILESYSTEM_TOLERANCE = 0.0005;
const SAME_FILESYSTEM_MIN_USED = 0.01;

/** The most guests sharing an unknown filesystem that are joined pair by pair. */
const SHARED_MOUNT_MAX_JOINED = 6;

const isSameFilesystem = (a: MapMount, b: MapMount): boolean => {
  const size = Math.max(a.total, b.total);
  if (!(size > 0)) return false;
  if (Math.abs(a.total - b.total) > size * SAME_FILESYSTEM_TOLERANCE) return false;
  if (Math.min(a.used, b.used) < size * SAME_FILESYSTEM_MIN_USED) return false;
  return Math.abs(a.used - b.used) <= size * SAME_FILESYSTEM_TOLERANCE;
};

const poolBytes = (pool: MapPool): MapMount | undefined =>
  pool.totalBytes === undefined || pool.usedBytes === undefined
    ? undefined
    : { total: pool.totalBytes, used: pool.usedBytes };

// The one drawn pool a filesystem is on, or none when two could be.
const poolHolding = (mounts: readonly MapMount[], candidates: readonly PoolTile[]) => {
  const matches = candidates.filter(({ pools }) =>
    pools.some((pool) => {
      const bytes = poolBytes(pool);
      return bytes !== undefined && mounts.some((mount) => isSameFilesystem(mount, bytes));
    }),
  );
  return matches.length === 1 ? matches[0] : undefined;
};

// A container that carries other containers' networks is drawn first, then
// those containers, so the ones that share a network sit together in a box.
const withNetworkRuns = (
  tiles: DiagramTile[],
  providerOf: ReadonlyMap<string, string>,
): DiagramTile[] => {
  const here = new Set(tiles.map((tile) => tile.id));
  const placed = new Set<string>();
  const ordered: DiagramTile[] = [];
  // Running things stay ahead of switched-off ones, which are the ones a
  // full box folds away: a switched-off container keeps its place at the
  // back, beside nothing, and a running one never follows a switched-off one.
  const off = (tile: DiagramTile) => tile.health === 'off';
  const byId = new Map(tiles.map((tile) => [tile.id, tile]));
  const follows = (user: DiagramTile, provider: DiagramTile | undefined) =>
    provider !== undefined && here.has(provider.id) && off(user) === off(provider);
  for (const tile of tiles) {
    if (placed.has(tile.id)) continue;
    if (follows(tile, byId.get(providerOf.get(tile.id) ?? ''))) continue;
    ordered.push(tile);
    placed.add(tile.id);
    for (const user of tiles) {
      if (providerOf.get(user.id) === tile.id && follows(user, tile) && !placed.has(user.id)) {
        ordered.push(user);
        placed.add(user.id);
      }
    }
  }
  // A container whose provider is in the box but never placed (a loop of
  // two) still has to be drawn.
  for (const tile of tiles) if (!placed.has(tile.id)) ordered.push(tile);
  return ordered;
};

export function buildServiceDiagram(
  model: InfrastructureMapModel,
  options: ServiceDiagramOptions = {},
): ServiceDiagram {
  const max = Math.max(2, options.maxTilesPerGroup ?? DIAGRAM_MAX_TILES_PER_GROUP);
  const attentionOnly = Boolean(options.filter?.attentionOnly);
  const activeTrafficOnly = Boolean(options.filter?.activeTrafficOnly);
  const query = (options.filter?.query ?? '').trim();
  const networkId = options.filter?.networkId?.trim() || undefined;
  const hasFilter = attentionOnly || activeTrafficOnly || query.length > 0 || Boolean(networkId);
  const byCategory = new Map<MapCategoryId, DiagramTile[]>();
  const roles = new Set<MapAppRole>();
  const pools: PoolEntry[] = [];
  interface ClusterMeta {
    id: string;
    name: string;
    typeLabel: string;
    poolEntries: PoolEntry[];
  }
  const clusterMetaById = new Map<string, ClusterMeta>();
  const hostClusterById = new Map<string, ClusterMeta>();
  const standaloneSharedPoolEntries: PoolEntry[] = [];
  const hardware: DiagramTile[] = [];
  const backupServers: DiagramTile[] = [];
  const checks: DiagramTile[] = [];
  const hostIds = new Set<string>();
  const hostResources: MapHost[] = [];
  const hostTilesById = new Map<string, DiagramTile>();
  const hostWorkloads = new Map<string, Array<{ tile: DiagramTile; category: MapCategoryId }>>();
  const ingressHostsById = new Map<string, string[]>();
  const allTilesById = new Map<string, DiagramTile>();
  const checkTargets = new Map<string, string>();
  // What each machine runs, by tile id and by how many of each kind.
  const runsOn = new Map<string, Array<{ id: string; category: MapCategoryId }>>();
  const mountsOf = new Map<string, MapMount[]>();
  let serviceCount = 0;
  const foldedOffTiles: DiagramTile[] = [];
  const plan = planNetworks(model);
  const networkSearchText = new Map(
    plan.networks.map((network) => [
      network.id,
      [network.label, network.detail].filter(Boolean).join(' '),
    ]),
  );
  const tileNetworkIds = new Map<string, string[]>();
  const recordTileNetworks = (tileId: string, segmentIds: readonly string[] | undefined) => {
    if (!segmentIds?.length) return;
    const ids = [
      ...new Set(
        segmentIds.map((id) => plan.networkOf(id)).filter((id): id is string => Boolean(id)),
      ),
    ];
    if (ids.length > 0) tileNetworkIds.set(tileId, ids);
  };
  const matchesFilter = (tile: DiagramTile, groupLabel: string): boolean => {
    if (attentionOnly && !isDiagramTileAttention(tile.health)) return false;
    if (activeTrafficOnly && !(tile.netTotalBytes !== undefined && tile.netTotalBytes > 0)) {
      return false;
    }
    const netIds = tileNetworkIds.get(tile.id) ?? (tile.segmentId ? [tile.segmentId] : []);
    if (networkId && !netIds.includes(networkId)) {
      return false;
    }
    if (!query) return true;
    const netText = netIds
      .map((id) => networkSearchText.get(id) ?? '')
      .filter(Boolean)
      .join(' ');
    return matchesDiagramTileQuery(tile, query, `${groupLabel} ${netText}`);
  };
  const hostNames = new Map<string, string>();
  for (const host of hostsOf(model)) if (host.isResource) hostNames.set(host.id, host.name);
  const hostName = (id: string) => hostNames.get(id);

  for (const block of model.blocks) {
    if (block.kind === 'group') {
      const nonBackupEntries: PoolEntry[] = [];
      for (const pool of block.group.pools) {
        const entry: PoolEntry = { pool, owner: block.group.name, hostIds: pool.hostIds ?? [] };
        pools.push(entry);
        if (!pool.backup) nonBackupEntries.push(entry);
      }
      const meta: ClusterMeta = {
        id: block.group.id,
        name: block.group.name,
        typeLabel: block.group.typeLabel,
        poolEntries: nonBackupEntries,
      };
      clusterMetaById.set(meta.id, meta);
      for (const host of block.group.hosts) {
        if (host.isResource) hostClusterById.set(host.id, meta);
      }
    }
  }
  for (const host of hostsOf(model)) {
    let services = 0;
    const runs: Array<{ id: string; category: MapCategoryId }> = [];
    const workloadsForHost: Array<{ tile: DiagramTile; category: MapCategoryId }> = [];
    for (const section of host.sections) {
      if (!SERVICE_SECTION.test(section.key)) continue;
      for (const item of section.items) {
        services += 1;
        const category = item.category ?? 'other';
        runs.push({ id: item.id, category });
        const tile = serviceTile(item, host, plan);
        allTilesById.set(tile.id, tile);
        if (item.ingressHosts?.length) ingressHostsById.set(tile.id, item.ingressHosts);
        workloadsForHost.push({ tile, category });
        recordTileNetworks(tile.id, item.segmentIds);
        // Something switched off that the catalogue cannot name says nothing
        // on an unfiltered diagram of what the estate does, so those fold into
        // one tile while staying searchable and inspectable in allTiles.
        if (category === 'other' && !item.app && item.health === 'off') {
          foldedOffTiles.push(tile);
          continue;
        }
        if (item.app?.role && item.health !== 'off') roles.add(item.app.role);
        const list = byCategory.get(category) ?? [];
        list.push(tile);
        byCategory.set(category, list);
        if (item.mounts?.length) mountsOf.set(item.id, item.mounts);
      }
    }
    if (host.isResource) {
      hostResources.push(host);
      if (services > 0) runsOn.set(host.id, runs);
      hostWorkloads.set(host.id, workloadsForHost);
    }
    serviceCount += services;
    // What Pulse probes from outside (a site, a printer) is not a service on
    // any host here, so each check is a tile in a box of its own. Network
    // gear is the exception: it is how traffic gets in and around, so it
    // sits with the ways in, where an unmonitored gateway is drawn too. A
    // machine check stays a check here as it does everywhere: only a Pulse
    // agent makes a machine.
    if (host.platform === 'checks' && !host.isResource) {
      for (const section of host.sections) {
        for (const item of section.items) {
          if (item.linkedResourceId) checkTargets.set(item.id, item.linkedResourceId);
          if (isNetworkGearKind(item.checkKind)) {
            const list = byCategory.get('access') ?? [];
            const gear = gearTile(item, item.checkKind, plan);
            allTilesById.set(gear.id, gear);
            recordTileNetworks(gear.id, item.segmentIds);
            list.push(gear);
            byCategory.set('access', list);
            continue;
          }
          recordTileNetworks(item.id, item.segmentIds);
          const checkTile: DiagramTile = {
            id: item.id,
            name: item.name,
            caption: item.address ?? 'check',
            typeLabel: item.typeLabel,
            status: item.status,
            health: item.health,
            platform: 'checks',
            ...plannedTileNetworks(item.segmentIds, plan),
            address: plan.where(item.address, item.segmentIds),
            ...tileNetworkTelemetry(item),
            ...(item.probeSummary ? { probeSummary: item.probeSummary } : {}),
            ...(item.certWarning ? { certWarning: item.certWarning } : {}),
            navigable: true,
          };
          allTilesById.set(checkTile.id, checkTile);
          checks.push(checkTile);
        }
      }
    }
    for (const pool of host.pools) {
      const entry: PoolEntry = {
        pool,
        owner: host.isResource ? host.name : '',
        hostIds: host.isResource ? [host.id] : (pool.hostIds ?? []),
      };
      pools.push(entry);
      if (!host.isResource && host.platform === 'storage' && !pool.backup) {
        standaloneSharedPoolEntries.push(entry);
      }
    }
    if (!host.isResource) continue;
    hostIds.add(host.id);
    recordTileNetworks(host.id, host.ownSegmentIds);
    const tile: DiagramTile = {
      id: host.id,
      name: host.name,
      caption: services > 0 ? plural(services, 'service', 'services') : host.typeLabel,
      typeLabel: host.typeLabel,
      status: host.status,
      health: host.health,
      platform: host.platform,
      ...plannedTileNetworks(host.ownSegmentIds, plan),
      ...(host.cpuCores !== undefined ? { cpuCores: host.cpuCores } : {}),
      ...(host.memoryTotalBytes !== undefined ? { memoryTotalBytes: host.memoryTotalBytes } : {}),
      ...(host.versionLabel ? { versionLabel: host.versionLabel } : {}),
      ...(host.temperatureC !== undefined ? { temperatureC: host.temperatureC } : {}),
      ...(host.diskSummary ? { diskSummary: host.diskSummary } : {}),
      ...(host.diskMaxTempC !== undefined ? { diskMaxTempC: host.diskMaxTempC } : {}),
      ...(host.unhealthyDiskCount !== undefined
        ? { unhealthyDiskCount: host.unhealthyDiskCount }
        : {}),
      ...(host.uptimeSeconds !== undefined ? { uptimeSeconds: host.uptimeSeconds } : {}),
      ...(host.pendingUpdates !== undefined ? { pendingUpdates: host.pendingUpdates } : {}),
      cpu: host.cpu,
      memory: host.memory,
      ...tileNetworkTelemetry(host),
      address: plan.where(host.address, host.ownSegmentIds),
      navigable: true,
    };
    hostTilesById.set(host.id, tile);
    allTilesById.set(tile.id, tile);
    (host.platform === 'backup' ? backupServers : hardware).push(tile);
  }

  // A container that carries other containers' networks is there for them,
  // so it is drawn in the box most of them are in, and they follow it.
  const categoryOf = new Map<string, MapCategoryId>();
  for (const [category, tiles] of byCategory) {
    for (const tile of tiles) categoryOf.set(tile.id, category);
  }
  const providerOf = new Map<string, string>();
  const usersOf = new Map<string, string[]>();
  for (const link of model.connections.links) {
    if (link.kind !== 'network' || !categoryOf.has(link.fromId) || !categoryOf.has(link.toId)) {
      continue;
    }
    providerOf.set(link.fromId, link.toId);
    usersOf.set(link.toId, [...(usersOf.get(link.toId) ?? []), link.fromId]);
  }
  // The ways in and the backups are what a thing is, not what it serves.
  const fixed = (category: MapCategoryId) => category === 'access' || category === 'backup';
  for (const [provider, users] of usersOf) {
    const own = categoryOf.get(provider);
    if (!own || fixed(own)) continue;
    const counts = new Map<MapCategoryId, number>();
    for (const user of users) {
      const category = categoryOf.get(user);
      if (category && !fixed(category)) counts.set(category, (counts.get(category) ?? 0) + 1);
    }
    const most = Math.max(0, ...counts.values());
    if (most === 0 || counts.get(own) === most) continue;
    const to = MAP_CATEGORY_ORDER.find((category) => counts.get(category) === most);
    const from = byCategory.get(own);
    const index = from?.findIndex((tile) => tile.id === provider) ?? -1;
    if (!to || !from || index < 0) continue;
    const [tile] = from.splice(index, 1);
    if (from.length === 0) byCategory.delete(own);
    byCategory.set(to, [...(byCategory.get(to) ?? []), tile]);
    categoryOf.set(provider, to);
  }

  const filterTiles = (tiles: DiagramTile[], groupLabel: string): DiagramTile[] =>
    hasFilter
      ? tiles.filter((tile) => matchesFilter(tile, groupLabel)).sort(compareDiagramTilesByAttention)
      : tiles;

  const categoryTiles = (category: MapCategoryId): DiagramTile[] =>
    withNetworkRuns([...(byCategory.get(category) ?? [])].sort(byRunningThenName), providerOf);

  const group = (category: MapCategoryId): DiagramGroup | undefined => {
    const label = MAP_CATEGORY_LABEL[category];
    const base = categoryTiles(category);
    const extraOff = category === 'other' ? foldedOffTiles : [];
    if (hasFilter) {
      const matched = filterTiles([...base, ...extraOff], label);
      if (matched.length === 0) return undefined;
      return box(category, label, matched, max);
    }
    const tiles = [...base];
    if (extraOff.length > 0)
      tiles.push(foldTile('other:off', `+${extraOff.length}`, 'switched off'));
    if (tiles.length === 0) return undefined;
    return box(category, label, tiles, max, extraOff);
  };

  const groups: DiagramGroup[] = [];
  for (const category of MAP_CATEGORY_ORDER) {
    if (category === 'access' || category === 'backup') continue;
    const built = group(category);
    if (built) groups.push(built);
  }
  const visibleChecks = filterTiles(checks, 'Checks');
  const checksBox =
    visibleChecks.length > 0 ? box('checks', 'Checks', visibleChecks, max) : undefined;
  if (checksBox) groups.push(checksBox);

  const storagePools = poolTiles(
    pools.filter(({ pool }) => !pool.backup),
    'storage',
    hostName,
  );
  const backupPools = poolTiles(
    pools.filter(({ pool }) => pool.backup),
    'backup',
    hostName,
  );
  const storageTilesAll = storagePools.map(({ tile }) => tile);
  const storageTiles = filterTiles(storageTilesAll, 'Storage');

  const backupTilesAll = [
    ...backupServers,
    ...categoryTiles('backup'),
    ...backupPools.map(({ tile }) => tile),
  ];
  const backupTiles = filterTiles(backupTilesAll, 'Backups');

  const hardwareSorted = [...hardware].sort((a, b) => collator.compare(a.name, b.name));
  const hardwareTiles = filterTiles(hardwareSorted, 'Hardware');

  const access = group('access');
  const internetRoles = ROLE_ORDER.filter((role) => roles.has(role) && INTERNET_ROLES.has(role));

  // Everything drawn as a tile of its own, by id, so a connection can find
  // both of its ends and say something on each.
  const drawn = new Map<string, DiagramTile>();
  const storageBox =
    storageTiles.length > 0 ? box('storage', 'Storage', storageTiles, max) : undefined;
  const backupBox =
    backupTiles.length > 0 ? box('backups', 'Backups', backupTiles, max) : undefined;
  const hardwareBox =
    hardwareTiles.length > 0 ? box('hardware', 'Hardware', hardwareTiles, max) : undefined;
  const boxes = [access, ...groups, storageBox, backupBox, hardwareBox];
  for (const entry of boxes) {
    if (!entry) continue;
    for (const tile of entry.tiles) if (tile.navigable) drawn.set(tile.id, tile);
  }
  const say = (tile: DiagramTile, sentence: string) => {
    tile.links = [...(tile.links ?? []), sentence];
  };

  const linkLabelRank = (label: string | undefined): number => {
    if (!label) return 0;
    if (label === 'runs on') return 1;
    if (label === 'storage') return 2;
    if (label === 'mounts' || label === 'backups') return 3;
    if (label === 'DNS') return 4;
    if (label === 'gateway') return 5;
    if (label === 'checks') return 6;
    if (label === 'routes') return 7;
    if (label.startsWith('via ')) return 8;
    return 9;
  };
  const links: DiagramLink[] = [];
  const linkByPair = new Map<string, DiagramLink>();
  const tie = (fromId: string, toId: string, label?: string) => {
    if (fromId === toId || !drawn.has(fromId) || !drawn.has(toId)) return;
    const trimmed = label?.trim() || undefined;
    const key = fromId < toId ? `${fromId}>${toId}` : `${toId}>${fromId}`;
    const existing = linkByPair.get(key);
    if (existing) {
      if (trimmed && linkLabelRank(trimmed) > linkLabelRank(existing.label)) {
        existing.label = trimmed;
      }
      return;
    }
    const entry: DiagramLink = trimmed ? { fromId, toId, label: trimmed } : { fromId, toId };
    linkByPair.set(key, entry);
    links.push(entry);
  };

  // A machine is linked to what runs on it and the storage it has, and
  // says what kind of things it runs, most first, counted in the boxes they
  // are drawn in.
  for (const [hostId, runs] of runsOn) {
    const tile = drawn.get(hostId);
    if (!tile) continue;
    const kinds = new Map<MapCategoryId, number>();
    for (const { id, category } of runs) {
      const drawnIn = categoryOf.get(id) ?? category;
      kinds.set(drawnIn, (kinds.get(drawnIn) ?? 0) + 1);
    }
    say(
      tile,
      [...kinds]
        .sort(
          ([a, countA], [b, countB]) =>
            countB - countA || MAP_CATEGORY_ORDER.indexOf(a) - MAP_CATEGORY_ORDER.indexOf(b),
        )
        .map(([category, count]) => `${MAP_CATEGORY_LABEL[category]} ${count.toLocaleString()}`)
        .join(' · '),
    );
    for (const { id } of runs) tie(hostId, id, 'runs on');
  }
  const drawnPools = (list: PoolTile[]) => list.filter(({ tile }) => drawn.has(tile.id));
  for (const { tile, hostIds } of drawnPools(storagePools)) {
    for (const hostId of hostIds) tie(hostId, tile.id, 'storage');
  }
  for (const { tile, hostIds } of drawnPools(backupPools)) {
    for (const hostId of hostIds) tie(hostId, tile.id, 'backups');
  }
  // Backup storage on a host that runs things is where that host writes
  // its backups. A backup server's own datastores are where others write,
  // which the server cannot say.
  const backupsOf = new Map<string, string[]>();
  for (const { tile, hostIds } of drawnPools(backupPools)) {
    const writers = [...hostIds].filter((id) => {
      const host = drawn.get(id);
      return host !== undefined && host.platform !== 'backup';
    });
    if (writers.length === 0) continue;
    say(tile, `Backup storage of ${nameList(writers.map((id) => drawn.get(id)?.name ?? ''))}`);
    for (const id of writers) backupsOf.set(id, [...(backupsOf.get(id) ?? []), tile.name]);
  }
  for (const [hostId, targets] of backupsOf) {
    const tile = drawn.get(hostId);
    if (tile) say(tile, `Backs up to ${nameList(targets)}`);
  }

  // Where a guest keeps its data: a disk the platform reports on a pool, or
  // a filesystem it mounts that is the same size and as full as one drawn.
  const usedBy = new Map<string, string[]>();
  const uses = (guest: DiagramTile, pool: DiagramTile, sentence: string, label: string) => {
    say(guest, sentence);
    tie(guest.id, pool.id, label);
    usedBy.set(pool.id, [...(usedBy.get(pool.id) ?? []), guest.name]);
  };
  // A backup target's consumers are the guests it holds backups of, which
  // is not where they keep their data, so only storage pools count here.
  const placedOn = new Set<string>();
  for (const { tile: pool, pools: members } of drawnPools(storagePools)) {
    const consumers = new Set(members.flatMap((member) => member.consumerIds ?? []));
    for (const id of consumers) {
      const guest = drawn.get(id);
      if (!guest || placedOn.has(`${id}>${pool.id}`)) continue;
      placedOn.add(`${id}>${pool.id}`);
      uses(guest, pool, `Data on ${pool.name}`, 'storage');
    }
  }
  // Each mount is matched on its own: two folders from two pools are two
  // joins, and a mount no pool holds is left for the next step.
  const unplaced: Array<[DiagramTile, MapMount[]]> = [];
  for (const [id, mounts] of mountsOf) {
    const guest = drawn.get(id);
    if (!guest) continue;
    const loose: MapMount[] = [];
    for (const mount of mounts) {
      const holder = poolHolding([mount], drawnPools(storagePools));
      if (!holder) {
        loose.push(mount);
        continue;
      }
      if (placedOn.has(`${id}>${holder.tile.id}`)) continue;
      placedOn.add(`${id}>${holder.tile.id}`);
      uses(guest, holder.tile, `Mounts ${holder.tile.name}`, 'mounts');
    }
    if (loose.length > 0) unplaced.push([guest, loose]);
  }
  for (const [poolId, names] of usedBy) {
    const pool = drawn.get(poolId);
    if (pool) say(pool, `Used by ${nameList(names)}`);
  }
  // Guests that mount one filesystem nothing drawn holds say so to each
  // other: the map cannot name where it is, only that they share it. A few
  // are joined pair by pair; past that every guest would ring every other.
  const sharing: Array<Array<[DiagramTile, MapMount[]]>> = [];
  for (const entry of unplaced) {
    const [, mounts] = entry;
    const set = sharing.find((members) =>
      members.some(([, theirs]) =>
        mounts.some((mount) => theirs.some((their) => isSameFilesystem(mount, their))),
      ),
    );
    if (set) set.push(entry);
    else sharing.push([entry]);
  }
  for (const members of sharing) {
    if (members.length < 2) continue;
    for (const [guest] of members) {
      const others = members.filter(([other]) => other.id !== guest.id).map(([other]) => other);
      say(guest, `Shares a filesystem with ${nameList(others.map((other) => other.name))}`);
      if (members.length > SHARED_MOUNT_MAX_JOINED) continue;
      for (const other of others) if (guest.id < other.id) tie(guest.id, other.id, 'mounts');
    }
  }
  // A backup target that is the same size and as full as a drawn pool is
  // kept on it: a backup server's datastore on a disk Pulse watches.
  const keptOn = new Map<string, string[]>();
  for (const { tile, pools: members } of drawnPools(backupPools)) {
    const bytes = members.map(poolBytes).filter((value): value is MapMount => Boolean(value));
    const holder = bytes.length > 0 ? poolHolding(bytes, drawnPools(storagePools)) : undefined;
    if (!holder) continue;
    say(tile, `Stored on ${holder.tile.name}`);
    tie(tile.id, holder.tile.id, 'backups');
    keptOn.set(holder.tile.id, [...(keptOn.get(holder.tile.id) ?? []), tile.name]);
  }
  for (const [poolId, names] of keptOn) {
    const pool = drawn.get(poolId);
    if (pool) say(pool, `Holds ${nameList(names)}`);
  }
  const carried = new Map<string, number>();
  const routedBy = new Map<string, number>();
  for (const link of model.connections.links) {
    const from = drawn.get(link.fromId);
    const to = drawn.get(link.toId);
    // An end folded into a `+N` tile is not drawn, so the link has no place.
    if (!from || !to) continue;
    if (link.kind === 'network') {
      say(from, `Uses the network of ${to.name}`);
      // The container it reaches the world through says more than the host
      // it runs on, which the hardware box already shows.
      from.caption = `via ${to.name}`;
      from.hasConnectionCaption = true;
      carried.set(to.id, (carried.get(to.id) ?? 0) + 1);
      tie(link.fromId, link.toId, `via ${to.name}`);
    } else {
      say(
        to,
        link.hosts.length > 0
          ? `Routed by ${from.name} at ${link.hosts.join(', ')}`
          : `Routed by ${from.name}`,
      );
      // Where it answers says more than the host it runs on.
      if (link.hosts.length > 0) {
        to.caption = mapRouteLabel(link.hosts);
        to.hasConnectionCaption = true;
      }
      routedBy.set(from.id, (routedBy.get(from.id) ?? 0) + 1);
      tie(link.fromId, link.toId, link.hosts.length > 0 ? mapRouteLabel(link.hosts) : 'routes');
    }
  }
  for (const [id, count] of carried) {
    const tile = drawn.get(id);
    if (tile) say(tile, `Carries the network of ${plural(count, 'container', 'containers')}`);
  }
  for (const [id, count] of routedBy) {
    const tile = drawn.get(id);
    if (tile) say(tile, `Routes ${plural(count, 'container', 'containers')}`);
  }

  for (const [checkId, targetId] of checkTargets) {
    if (checkId === targetId) continue;
    const check = drawn.get(checkId);
    const target = drawn.get(targetId);
    if (!check || !target) continue;
    tie(check.id, target.id, 'checks');
    say(check, `Checks ${target.name}`);
    say(
      target,
      check.latencyMs !== undefined
        ? `Checked by ${check.name} · ${formatMapLatency(check.latencyMs)}`
        : `Checked by ${check.name}`,
    );
  }

  const machineNames = (ids: string[]): string[] =>
    ids.map((id) => drawn.get(id)?.name).filter((name): name is string => Boolean(name));
  const gateways: DiagramTile[] = [];
  const gatewayTiles = new Map<number, DiagramTile>();
  const gatewayLinks = new Map<string, Array<{ machineId: string; label: string }>>();
  model.connections.gateways.forEach((gateway, index) => {
    const names = machineNames(gateway.machineIds);
    // A gateway only machines that are not drawn point at says nothing here.
    if (names.length === 0) return;
    const owner = gateway.resourceId ? drawn.get(gateway.resourceId) : undefined;
    if (owner) {
      say(owner, `Default gateway of ${nameList(names)}`);
      for (const machineId of gateway.machineIds) {
        if (machineId !== owner.id) tie(owner.id, machineId, 'gateway');
      }
      return;
    }
    const tile: DiagramTile = {
      id: `gateway:${index}`,
      name: gateway.address || 'Gateway',
      caption: 'default gateway',
      typeLabel: 'Default gateway, not monitored',
      status: '',
      health: 'unknown',
      platform: 'gateway',
      links: [`Used by ${nameList(names)}`],
      navigable: false,
      // With names hidden the address is gone, and so is the link that
      // would carry it.
      ...(gateway.address ? { checkAddress: gateway.address } : {}),
    };
    gateways.push(tile);
    gatewayTiles.set(index, tile);
    gatewayLinks.set(
      tile.id,
      gateway.machineIds.map((machineId) => ({ machineId, label: 'gateway' })),
    );
  });
  // A resolver is only said where it is already on the map: a monitored
  // thing, or a gateway with the same address. An address nothing else
  // names would be one more box of nothing.
  for (const resolver of model.connections.resolvers) {
    const owner =
      (resolver.resourceId ? drawn.get(resolver.resourceId) : undefined) ??
      (resolver.gateway === undefined ? undefined : gatewayTiles.get(resolver.gateway));
    const machineIds = resolver.machineIds.filter((id) => id !== resolver.resourceId);
    const names = machineNames(machineIds);
    if (owner && names.length > 0) {
      say(owner, `Resolves names for ${nameList(names)}`);
      if (resolver.resourceId && drawn.has(owner.id)) {
        for (const machineId of machineIds) {
          if (machineId !== owner.id) tie(owner.id, machineId, 'DNS');
        }
      } else if (resolver.gateway !== undefined) {
        const existing = gatewayLinks.get(owner.id) ?? [];
        gatewayLinks.set(owner.id, [
          ...existing,
          ...machineIds.map((machineId) => ({ machineId, label: 'DNS' })),
        ]);
      }
    }
  }

  const visibleGateways = (
    hasFilter ? gateways.filter((tile) => matchesFilter(tile, 'Gateway')) : gateways
  ).slice(0, DIAGRAM_MAX_GATEWAYS);
  for (const gatewayTile of visibleGateways) {
    drawn.set(gatewayTile.id, gatewayTile);
    for (const { machineId, label } of gatewayLinks.get(gatewayTile.id) ?? []) {
      tie(gatewayTile.id, machineId, label);
    }
  }

  // Aggregate live network throughput across the estate without double-counting
  // hosts alongside their reporting workloads: for each host, contribute the
  // sum of its reporting children when any report traffic, otherwise the host's
  // own rate. Include any standalone reporting workloads not counted under a host.
  const allNavigable = new Map<string, DiagramTile>();
  for (const entry of boxes) {
    if (!entry) continue;
    for (const tile of entry.allTiles) {
      if (tile.navigable && !allNavigable.has(tile.id)) allNavigable.set(tile.id, tile);
    }
  }
  const countedTrafficIds = new Set<string>();
  let estateTrafficSum = 0;
  for (const hostId of hostIds) {
    const hostTile = allNavigable.get(hostId);
    if (!hostTile) continue;
    countedTrafficIds.add(hostId);
    let childrenReporting = 0;
    let childrenSum = 0;
    for (const { id: childId } of runsOn.get(hostId) ?? []) {
      const child = allNavigable.get(childId);
      if (!child || countedTrafficIds.has(childId)) continue;
      countedTrafficIds.add(childId);
      if (child.netTotalBytes !== undefined && child.netTotalBytes > 0) {
        childrenReporting += 1;
        childrenSum += child.netTotalBytes;
      }
    }
    if (childrenReporting > 0) {
      estateTrafficSum += childrenSum;
    } else if (hostTile.netTotalBytes !== undefined && hostTile.netTotalBytes > 0) {
      estateTrafficSum += hostTile.netTotalBytes;
    }
  }
  for (const [id, tile] of allNavigable) {
    if (countedTrafficIds.has(id)) continue;
    if (tile.netTotalBytes !== undefined && tile.netTotalBytes > 0) {
      estateTrafficSum += tile.netTotalBytes;
    }
  }

  const routeInfoByTarget = new Map<string, { hosts: Set<string>; proxyNames: Set<string> }>();
  for (const link of model.connections.links) {
    if (link.kind !== 'route') continue;
    const entry = routeInfoByTarget.get(link.toId) ?? {
      hosts: new Set<string>(),
      proxyNames: new Set<string>(),
    };
    for (const host of link.hosts) {
      const normalized = host.trim().toLowerCase();
      if (normalized) entry.hosts.add(normalized);
    }
    const proxyTile = allTilesById.get(link.fromId);
    if (proxyTile?.name) entry.proxyNames.add(proxyTile.name);
    routeInfoByTarget.set(link.toId, entry);
  }
  for (const targetId of routeInfoByTarget.keys()) {
    const tile = allTilesById.get(targetId);
    if (tile) tile.isRouted = true;
  }
  for (const [tileId, hosts] of ingressHostsById) {
    if (hosts.length === 0) continue;
    const tile = allTilesById.get(tileId);
    if (tile) tile.isRouted = true;
  }

  const hostFrames: DiagramHostFrame[] = [];
  for (const host of hostResources) {
    const hostTile = hostTilesById.get(host.id);
    if (!hostTile) continue;
    const clusterMeta = hostClusterById.get(host.id);
    const clusterName = clusterMeta?.name;
    const rawWorkloads = (hostWorkloads.get(host.id) ?? []).filter(
      ({ category }) => category !== 'access',
    );
    const rawPools = host.pools
      .filter((pool) => !pool.backup)
      .map((pool) => localPoolTile(pool, host, drawn, allTilesById));

    let visibleWorkloads: DiagramTile[];
    let visiblePools: DiagramTile[];
    if (hasFilter) {
      const hostContext = clusterName ? `${clusterName} Hardware` : 'Hardware';
      const hostMatches = matchesFilter(hostTile, hostContext);
      visibleWorkloads = rawWorkloads
        .filter(({ tile, category }) =>
          matchesFilter(tile, `${host.name} ${clusterName ?? ''} ${MAP_CATEGORY_LABEL[category]}`),
        )
        .map(({ tile }) => tile);
      visiblePools = rawPools.filter((poolTile) =>
        matchesFilter(poolTile, `${host.name} ${clusterName ?? ''} Storage`),
      );
      if (!hostMatches && visibleWorkloads.length === 0 && visiblePools.length === 0) continue;
    } else {
      visibleWorkloads = rawWorkloads.map(({ tile }) => tile);
      visiblePools = rawPools;
    }

    const frameAllTiles = [...visibleWorkloads].sort(compareDiagramTilesByAttention);
    const displayed = frameAllTiles.slice(0, max);
    const overflowCount = Math.max(0, frameAllTiles.length - displayed.length);

    const byProject = new Map<string, DiagramTile[]>();
    for (const tile of displayed) {
      if (!tile.composeProject) continue;
      const list = byProject.get(tile.composeProject) ?? [];
      list.push(tile);
      byProject.set(tile.composeProject, list);
    }
    const stacks: DiagramHostStack[] = [];
    const stackedIds = new Set<string>();
    for (const [project, members] of byProject) {
      if (members.length < 2) continue;
      for (const member of members) stackedIds.add(member.id);
      stacks.push({
        key: `${host.id}:stack:${project}`,
        label: project,
        tiles: [...members].sort(compareDiagramTilesByAttention),
      });
    }
    stacks.sort((a, b) => collator.compare(a.label, b.label));
    const looseTiles = displayed
      .filter((tile) => !stackedIds.has(tile.id))
      .sort(compareDiagramTilesByAttention);

    let childTrafficSum = 0;
    for (const { tile } of hostWorkloads.get(host.id) ?? []) {
      if (tile.netTotalBytes !== undefined && tile.netTotalBytes > 0) {
        childTrafficSum += tile.netTotalBytes;
      }
    }
    const frameTrafficBytes = Math.max(hostTile.netTotalBytes ?? 0, childTrafficSum);
    const trafficTier = resolveMapTrafficTier(
      frameTrafficBytes > 0 ? frameTrafficBytes : undefined,
    );

    hostFrames.push({
      id: host.id,
      host: hostTile,
      ...(clusterMeta
        ? {
            clusterId: clusterMeta.id,
            clusterName: clusterMeta.name,
            clusterTypeLabel: clusterMeta.typeLabel,
          }
        : {}),
      stacks,
      looseTiles,
      pools: visiblePools,
      allTiles: frameAllTiles,
      attentionCount: frameAllTiles.filter((tile) => isDiagramTileAttention(tile.health)).length,
      totalCount: frameAllTiles.length,
      overflowCount,
      ...(trafficTier !== undefined ? { trafficTier } : {}),
      ...(hostTile.linkSpeedMbps !== undefined ? { linkSpeedMbps: hostTile.linkSpeedMbps } : {}),
    });
  }

  const buildSharedPoolTiles = (entries: PoolEntry[]): DiagramTile[] =>
    poolTiles(entries, 'storage', hostName).map(({ tile, pools: members }) => {
      const drawnTile = drawn.get(tile.id);
      if (drawnTile?.links?.length) {
        return { ...tile, links: [...drawnTile.links] };
      }
      const consumerNames = [
        ...new Set(
          members.flatMap((member) =>
            (member.consumerIds ?? [])
              .map((id) => allTilesById.get(id)?.name)
              .filter((name): name is string => Boolean(name)),
          ),
        ),
      ];
      return consumerNames.length > 0
        ? { ...tile, links: [`Used by ${nameList(consumerNames)}`] }
        : tile;
    });

  const visibleFramesByCluster = new Map<string, DiagramHostFrame[]>();
  for (const frame of hostFrames) {
    if (!frame.clusterId) continue;
    const list = visibleFramesByCluster.get(frame.clusterId) ?? [];
    list.push(frame);
    visibleFramesByCluster.set(frame.clusterId, list);
  }

  const sharedPoolEntriesForBand: PoolEntry[] = [...standaloneSharedPoolEntries];
  for (const meta of clusterMetaById.values()) {
    const clusterFrames = visibleFramesByCluster.get(meta.id) ?? [];
    if (meta.poolEntries.length > 0 && clusterFrames.length > 0) {
      const rawClusterPools = buildSharedPoolTiles(meta.poolEntries);
      const visibleClusterPools = hasFilter
        ? rawClusterPools.filter((poolTile) => matchesFilter(poolTile, `${meta.name} Storage`))
        : rawClusterPools;
      if (visibleClusterPools.length > 0) {
        for (const frame of clusterFrames) {
          frame.clusterPools = visibleClusterPools;
        }
      }
    }
    if (clusterFrames.length < 2) {
      sharedPoolEntriesForBand.push(...meta.poolEntries);
    }
  }

  const sharedStorageTilesAll = buildSharedPoolTiles(sharedPoolEntriesForBand);
  const sharedStorageTiles = filterTiles(sharedStorageTilesAll, 'Storage');
  const sharedStorageBox =
    sharedStorageTiles.length > 0
      ? box('shared-storage', 'Shared storage', sharedStorageTiles, max)
      : undefined;

  const seenExposed = new Set<string>();
  const exposed: DiagramExposedEntry[] = [];
  for (const entry of [access, ...groups, backupBox]) {
    if (!entry) continue;
    for (const tile of entry.allTiles) {
      if (!tile.navigable || seenExposed.has(tile.id)) continue;
      const routeInfo = routeInfoByTarget.get(tile.id);
      const ingress = ingressHostsById.get(tile.id);
      const ports = tile.detail;
      if (!routeInfo && !ingress?.length && !ports) continue;
      const hostSet = new Set<string>();
      for (const host of routeInfo?.hosts ?? []) hostSet.add(host);
      for (const host of ingress ?? []) {
        const normalized = host.trim().toLowerCase();
        if (normalized) hostSet.add(normalized);
      }
      const hostnames = [...hostSet].sort(collator.compare);
      const proxyName =
        routeInfo && routeInfo.proxyNames.size > 0
          ? [...routeInfo.proxyNames].sort(collator.compare).join(', ')
          : undefined;
      seenExposed.add(tile.id);
      exposed.push({
        tile,
        groupLabel: entry.label,
        hostnames,
        ...(ports ? { ports } : {}),
        ...(proxyName ? { proxyName } : {}),
      });
    }
  }
  exposed.sort(
    (a, b) =>
      Number(b.hostnames.length > 0 || Boolean(b.proxyName)) -
        Number(a.hostnames.length > 0 || Boolean(a.proxyName)) ||
      collator.compare(a.tile.name, b.tile.name),
  );

  const hasLinkSpeeds =
    hostFrames.some((frame) => frame.linkSpeedMbps !== undefined && frame.linkSpeedMbps > 0) ||
    (hardwareBox?.tiles ?? []).some(
      (tile) => tile.linkSpeedMbps !== undefined && tile.linkSpeedMbps > 0,
    ) ||
    plan.networks.some(
      (network) => network.linkSpeedMbps !== undefined && network.linkSpeedMbps > 0,
    );

  return {
    ...(internetRoles.length > 0 && (!hasFilter || access !== undefined)
      ? { internet: { label: internetRoles.join(' · ') } }
      : {}),
    gateways: visibleGateways,
    ...(access ? { access } : {}),
    links,
    networks: plan.networks,
    groups,
    hostFrames,
    ...(sharedStorageBox ? { sharedStorage: sharedStorageBox } : {}),
    ...(checksBox ? { checks: checksBox } : {}),
    exposed,
    ...(storageBox ? { storage: storageBox } : {}),
    ...(backupBox ? { backups: backupBox } : {}),
    ...(hardwareBox ? { hardware: hardwareBox } : {}),
    serviceCount,
    ...(estateTrafficSum > 0 ? { estateTrafficBytes: estateTrafficSum } : {}),
    ...(hasLinkSpeeds ? { hasLinkSpeeds: true } : {}),
  };
}
