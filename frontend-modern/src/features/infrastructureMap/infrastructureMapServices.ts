import {
  MAP_CATEGORY_LABEL,
  MAP_CATEGORY_ORDER,
  type MapAppRole,
  type MapCategoryId,
} from './infrastructureMapCatalogue';
import type { MapGlyphKey } from './infrastructureMapLayout';
import type {
  InfrastructureMapModel,
  MapHealth,
  MapHost,
  MapItem,
  MapPool,
} from './infrastructureMapModel';
import { OTHER_SEGMENT_ID } from './infrastructureMapNetworks';

/**
 * The estate as a service diagram: what it runs, boxed by what each thing is
 * for, between how traffic gets in at the top and the storage and hardware
 * underneath. It is a second reading of the same map model, so it draws only
 * what Pulse monitors. The one drawn thing Pulse does not monitor is the
 * Internet itself, shown only when an app whose whole job is to reach it
 * (a tunnel, a VPN, a reverse proxy) is running.
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
  cpu?: number;
  memory?: number;
  address?: string;
  detail?: string;
  /** False for a tile that stands for several things and opens none. */
  navigable: boolean;
}

export interface DiagramGroup {
  key: string;
  label: string;
  /** How many things the box holds, including any folded into one tile. */
  count: number;
  tiles: DiagramTile[];
}

export interface DiagramNetwork {
  id: string;
  label: string;
  memberCount: number;
  colorIndex: number;
}

export interface ServiceDiagram {
  /** Set when something running exists to reach the Internet. */
  internet?: { label: string };
  access?: DiagramGroup;
  networks: DiagramNetwork[];
  groups: DiagramGroup[];
  storage?: DiagramGroup;
  backups?: DiagramGroup;
  hardware?: DiagramGroup;
  serviceCount: number;
}

export interface ServiceDiagramOptions {
  /** The most tiles one group draws before the rest fold into one. */
  maxTilesPerGroup?: number;
}

export const DIAGRAM_MAX_TILES_PER_GROUP = 24;

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

const hostsOf = (model: InfrastructureMapModel): MapHost[] =>
  model.blocks.flatMap((block) => (block.kind === 'group' ? block.group.hosts : [block.host]));

const serviceTile = (item: MapItem, host: MapHost): DiagramTile => ({
  id: item.id,
  name: item.name,
  caption: item.app?.role ?? host.name,
  typeLabel: item.app ? `${item.app.label} · ${item.typeLabel}` : item.typeLabel,
  status: item.status,
  health: item.health,
  ...(item.app?.icon ? { icon: item.app.icon } : {}),
  ...(item.segmentIds?.[0] ? { segmentId: item.segmentIds[0] } : {}),
  cpu: item.cpu,
  memory: item.memory,
  address: item.address,
  detail: item.detail,
  navigable: true,
});

const foldTile = (key: string, name: string, caption: string): DiagramTile => ({
  id: key,
  name,
  caption,
  typeLabel: caption,
  status: '',
  health: 'off',
  navigable: false,
});

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
  folded = 0,
): DiagramGroup => ({
  key,
  label,
  count: tiles.filter((tile) => tile.navigable).length + folded,
  tiles: capTiles(key, tiles, max),
});

const poolTiles = (
  pools: Array<{ pool: MapPool; owner: string }>,
  platform: MapGlyphKey,
): DiagramTile[] => {
  // The same pool name on several hosts is one tile: a cluster's `local` is
  // three disks, and three identical tiles would say less than one.
  const byName = new Map<string, { pool: MapPool; owners: Set<string>; health: MapHealth }>();
  for (const { pool, owner } of pools) {
    const entry = byName.get(pool.name);
    if (entry) {
      if (owner) entry.owners.add(owner);
      entry.health = worst(entry.health, pool.health);
    } else {
      byName.set(pool.name, { pool, owners: new Set(owner ? [owner] : []), health: pool.health });
    }
  }
  return [...byName.values()]
    .map(({ pool, owners, health }): DiagramTile => {
      const caption =
        owners.size > 1 ? plural(owners.size, 'host', 'hosts') : ([...owners][0] ?? 'Shared');
      return {
        id: pool.id,
        name: pool.name,
        caption,
        typeLabel: [pool.backup ? 'Backup target' : 'Storage', pool.sizeLabel]
          .filter(Boolean)
          .join(' · '),
        status: pool.usedPercent === undefined ? '' : `${Math.round(pool.usedPercent)}% used`,
        health,
        platform,
        navigable: true,
      };
    })
    .sort((a, b) => collator.compare(a.name, b.name));
};

export function buildServiceDiagram(
  model: InfrastructureMapModel,
  options: ServiceDiagramOptions = {},
): ServiceDiagram {
  const max = Math.max(2, options.maxTilesPerGroup ?? DIAGRAM_MAX_TILES_PER_GROUP);
  const byCategory = new Map<MapCategoryId, DiagramTile[]>();
  const roles = new Set<MapAppRole>();
  const pools: Array<{ pool: MapPool; owner: string }> = [];
  const hardware: DiagramTile[] = [];
  const backupServers: DiagramTile[] = [];
  const checks: DiagramTile[] = [];
  let serviceCount = 0;
  let foldedOff = 0;

  for (const block of model.blocks) {
    if (block.kind === 'group') {
      for (const pool of block.group.pools) pools.push({ pool, owner: block.group.name });
    }
  }
  for (const host of hostsOf(model)) {
    let services = 0;
    for (const section of host.sections) {
      if (!SERVICE_SECTION.test(section.key)) continue;
      for (const item of section.items) {
        services += 1;
        const category = item.category ?? 'other';
        // Something switched off that the catalogue cannot name says nothing
        // on a diagram of what the estate does, so those fold into one tile.
        if (category === 'other' && !item.app && item.health === 'off') {
          foldedOff += 1;
          continue;
        }
        if (item.app?.role && item.health !== 'off') roles.add(item.app.role);
        const list = byCategory.get(category) ?? [];
        list.push(serviceTile(item, host));
        byCategory.set(category, list);
      }
    }
    serviceCount += services;
    // What Pulse probes from outside (a router, a site) is not a service on
    // any host here, so each check is a tile in a box of its own.
    if (host.platform === 'checks' && !host.isResource) {
      for (const section of host.sections) {
        for (const item of section.items) {
          checks.push({
            id: item.id,
            name: item.name,
            caption: item.address ?? 'check',
            typeLabel: item.typeLabel,
            status: item.status,
            health: item.health,
            platform: 'checks',
            address: item.address,
            navigable: true,
          });
        }
      }
    }
    for (const pool of host.pools) pools.push({ pool, owner: host.isResource ? host.name : '' });
    if (!host.isResource) continue;
    const tile: DiagramTile = {
      id: host.id,
      name: host.name,
      caption: services > 0 ? plural(services, 'service', 'services') : host.typeLabel,
      typeLabel: host.typeLabel,
      status: host.status,
      health: host.health,
      platform: host.platform,
      ...(host.ownSegmentIds[0] ? { segmentId: host.ownSegmentIds[0] } : {}),
      cpu: host.cpu,
      memory: host.memory,
      address: host.address,
      navigable: true,
    };
    (host.platform === 'backup' ? backupServers : hardware).push(tile);
  }

  const group = (category: MapCategoryId): DiagramGroup | undefined => {
    const tiles = [...(byCategory.get(category) ?? [])].sort(byRunningThenName);
    const folded = category === 'other' ? foldedOff : 0;
    if (folded > 0) tiles.push(foldTile('other:off', `+${folded}`, 'switched off'));
    if (tiles.length === 0) return undefined;
    return box(category, MAP_CATEGORY_LABEL[category], tiles, max, folded);
  };

  const groups: DiagramGroup[] = [];
  for (const category of MAP_CATEGORY_ORDER) {
    if (category === 'access' || category === 'backup') continue;
    const built = group(category);
    if (built) groups.push(built);
  }
  if (checks.length > 0) groups.push(box('checks', 'Checks', checks, max));

  const storageTiles = poolTiles(
    pools.filter(({ pool }) => !pool.backup),
    'storage',
  );
  const backupTiles = [
    ...backupServers,
    ...(group('backup')?.tiles ?? []),
    ...poolTiles(
      pools.filter(({ pool }) => pool.backup),
      'backup',
    ),
  ];
  const access = group('access');
  const internetRoles = ROLE_ORDER.filter((role) => roles.has(role) && INTERNET_ROLES.has(role));

  return {
    ...(internetRoles.length > 0 ? { internet: { label: internetRoles.join(' · ') } } : {}),
    ...(access ? { access } : {}),
    networks: model.segments
      .filter((segment) => segment.id !== OTHER_SEGMENT_ID)
      .map(({ id, label, memberCount, colorIndex }) => ({ id, label, memberCount, colorIndex })),
    groups,
    ...(storageTiles.length > 0 ? { storage: box('storage', 'Storage', storageTiles, max) } : {}),
    ...(backupTiles.length > 0 ? { backups: box('backups', 'Backups', backupTiles, max) } : {}),
    ...(hardware.length > 0
      ? {
          hardware: box(
            'hardware',
            'Hardware',
            hardware.sort((a, b) => collator.compare(a.name, b.name)),
            max,
          ),
        }
      : {}),
    serviceCount,
  };
}
