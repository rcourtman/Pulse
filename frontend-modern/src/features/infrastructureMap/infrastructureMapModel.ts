import type { Resource, ResourceType } from '@/types/resource';
import { getDiskPercent, getMemoryPercent } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import {
  resolveMapApp,
  resolveMapCategory,
  resolveMapDeviceRole,
  type MapApp,
  type MapCategoryId,
  type MapDeviceRole,
} from './infrastructureMapCatalogue';
import { resolveMapConnections, type MapConnections } from './infrastructureMapConnections';
import {
  resolveMapNetworks,
  type MapAttachment,
  type MapSegment,
} from './infrastructureMapNetworks';

/**
 * The infrastructure map draws what Pulse already monitors, and nothing else:
 * every box is a resource in the unified registry and every nesting is a
 * parent the registry reports. There are no hand-placed devices and no links
 * inferred from names, so the map cannot drift from the estate it shows.
 */

export type MapHealth = 'ok' | 'attention' | 'critical' | 'off' | 'stale' | 'unknown';

export type MapPlatform =
  | 'proxmox'
  | 'vmware'
  | 'kubernetes'
  | 'docker'
  | 'truenas'
  | 'backup'
  | 'mail'
  | 'machine'
  | 'storage'
  | 'checks';

export interface MapItem {
  id: string;
  name: string;
  typeLabel: string;
  health: MapHealth;
  status: string;
  cpu?: number;
  memory?: number;
  /** The address the item answers on, when Pulse knows one. */
  address?: string;
  /** Published ports, for a container. */
  detail?: string;
  /** The networks the item is on, as ids into the model's segments. */
  segmentIds?: string[];
  /** The well-known app this guest or container is, when the map knows it. */
  app?: MapApp;
  /** The group a guest or container is drawn in on the service diagram. */
  category?: MapCategoryId;
  /** The network gear a check is on, when its name or its address says so. */
  device?: MapDeviceRole;
}

export interface MapSection {
  key: string;
  label: string;
  items: MapItem[];
}

export interface MapPool {
  id: string;
  name: string;
  health: MapHealth;
  usedPercent?: number;
  sizeLabel?: string;
  /** A place backups are written to, as opposed to one guests run from. */
  backup?: boolean;
}

export interface MapHost {
  id: string;
  /** False for the holders the map composes itself (shared storage, checks). */
  isResource: boolean;
  name: string;
  platform: MapPlatform;
  typeLabel: string;
  health: MapHealth;
  status: string;
  cpu?: number;
  memory?: number;
  sections: MapSection[];
  pools: MapPool[];
  /** The address the host itself answers on. */
  address?: string;
  /** Networks the host has an address on. */
  ownSegmentIds: string[];
}

export interface MapGroup {
  id: string;
  name: string;
  typeLabel: string;
  platform: MapPlatform;
  hosts: MapHost[];
  pools: MapPool[];
}

export type MapBlock = { kind: 'group'; group: MapGroup } | { kind: 'host'; host: MapHost };

export interface MapTotal {
  key: string;
  label: string;
  singular: string;
  count: number;
}

export const formatMapTotalLabel = (total: MapTotal): string =>
  total.count === 1 ? total.singular : total.label;

export interface InfrastructureMapModel {
  blocks: MapBlock[];
  /** Every network the estate's addresses fall into, largest first. */
  segments: MapSegment[];
  /** The connections between things that a collected value states. */
  connections: MapConnections;
  totals: MapTotal[];
  attentionCount: number;
  isEmpty: boolean;
}

const PLATFORM_ORDER: readonly MapPlatform[] = [
  'proxmox',
  'vmware',
  'kubernetes',
  'docker',
  'truenas',
  'backup',
  'mail',
  'machine',
  'storage',
  'checks',
];

const HOST_TYPE_LABEL: Record<MapPlatform, string> = {
  proxmox: 'Proxmox node',
  vmware: 'vSphere host',
  kubernetes: 'Kubernetes cluster',
  docker: 'Docker host',
  truenas: 'TrueNAS system',
  backup: 'Backup server',
  mail: 'Mail gateway',
  machine: 'Machine',
  storage: 'Storage',
  checks: 'Checks',
};

/**
 * What a machine is called with its name hidden: this word and a number.
 * Shorter than the type label, because a tile cuts a name that does not fit
 * it and the number is all that tells two machines apart. The tile's logo
 * and its card still say what the machine is.
 */
const HOST_STAND_IN: Record<MapPlatform, string> = {
  proxmox: 'Proxmox',
  vmware: 'vSphere',
  kubernetes: 'Cluster',
  docker: 'Docker',
  truenas: 'TrueNAS',
  backup: 'Backup',
  mail: 'Mail',
  machine: 'Machine',
  storage: 'Storage',
  checks: 'Checks',
};

const SYSTEM_CONTAINER_LABEL = 'System container';

/** The same for a guest or container: its type, or a shorter word for it. */
const itemStandIn = (typeLabel: string): string =>
  typeLabel === SYSTEM_CONTAINER_LABEL ? 'Container' : typeLabel;

/** And for network gear, which stays the gear it is with its name hidden. */
const DEVICE_STAND_IN: Record<MapDeviceRole, string> = {
  router: 'Router',
  switch: 'Switch',
  'access-point': 'AP',
};

const GROUP_TYPE_LABEL: Partial<Record<MapPlatform, string>> = {
  proxmox: 'Proxmox cluster',
  vmware: 'vSphere cluster',
  docker: 'Swarm',
};

const SYSTEM_CONTAINER_TYPES = new Set<ResourceType>(['system-container', 'oci-container', 'jail']);
const POOL_TYPES = new Set<ResourceType>(['storage', 'datastore', 'pool', 'ceph']);

const SHARED_STORAGE_HOST_ID = 'map:shared-storage';
const CHECKS_HOST_ID = 'map:checks';

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
const byName = <T extends { name: string }>(a: T, b: T) => collator.compare(a.name, b.name);

const clean = (value: unknown): string => (typeof value === 'string' ? value.trim() : '');

const finitePercent = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value)
    ? Math.min(100, Math.max(0, value))
    : undefined;

export const resolveMapHealth = (resource: Resource): MapHealth => {
  const verdict = resource.health?.verdict;
  if (verdict) return verdict;
  switch (resource.status) {
    case 'online':
    case 'running':
      return 'ok';
    case 'degraded':
    case 'warning':
      return 'attention';
    case 'offline':
      return 'critical';
    case 'stopped':
    case 'paused':
      return 'off';
    default:
      return 'unknown';
  }
};

const displayName = (resource: Resource): string =>
  clean(resource.displayName) || clean(resource.name) || resource.id;

const hostPlatform = (resource: Resource): MapPlatform | null => {
  switch (resource.type) {
    case 'docker-host':
      return 'docker';
    case 'k8s-cluster':
      return 'kubernetes';
    case 'pbs':
      return 'backup';
    case 'pmg':
      return 'mail';
    case 'agent':
      if (resource.platformType === 'proxmox-pve') return 'proxmox';
      if (resource.platformType === 'vmware-vsphere') return 'vmware';
      if (resource.platformType === 'truenas') return 'truenas';
      return 'machine';
    default:
      return null;
  }
};

type Attachments = ReadonlyMap<string, MapAttachment[]>;

const publishedPorts = (resource: Resource): string | undefined => {
  const ports = new Set<number>();
  for (const port of resource.docker?.ports ?? []) {
    if (typeof port.publicPort === 'number' && port.publicPort > 0) ports.add(port.publicPort);
  }
  if (ports.size === 0) return undefined;
  const sorted = [...ports].sort((a, b) => a - b);
  const shown = sorted.slice(0, 3).join(', ');
  return sorted.length > 3 ? `:${shown} +${sorted.length - 3}` : `:${shown}`;
};

const toItem = (resource: Resource, typeLabel: string, attachments: Attachments): MapItem => {
  const attached = attachments.get(resource.id) ?? [];
  return {
    id: resource.id,
    name: displayName(resource),
    typeLabel,
    health: resolveMapHealth(resource),
    status: resource.status,
    cpu: finitePercent(resource.cpu?.current),
    memory: finitePercent(getMemoryPercent(resource)),
    ...(attached[0]?.address ? { address: attached[0].address } : {}),
    ...(attached.length > 0 ? { segmentIds: attached.map((entry) => entry.segmentId) } : {}),
    ...(resource.type === 'app-container' && publishedPorts(resource)
      ? { detail: publishedPorts(resource) }
      : {}),
  };
};

// A guest or container is what the service diagram draws as one tile, so it
// also carries the app it is and the group that puts it in.
const toService = (resource: Resource, typeLabel: string, attachments: Attachments): MapItem => {
  const app = resolveMapApp({ name: displayName(resource), image: resource.docker?.image });
  return {
    ...toItem(resource, typeLabel, attachments),
    ...(app ? { app } : {}),
    category: resolveMapCategory(app, resource.tags),
  };
};

const isBackupTarget = (resource: Resource): boolean =>
  resource.type === 'datastore' ||
  resource.platformType === 'proxmox-pbs' ||
  clean(resource.storage?.type).toLowerCase() === 'pbs';

const toPool = (resource: Resource): MapPool => {
  const total = resource.disk?.total;
  return {
    id: resource.id,
    name: displayName(resource),
    health: resolveMapHealth(resource),
    usedPercent: finitePercent(getDiskPercent(resource)),
    sizeLabel: typeof total === 'number' && total > 0 ? formatBytes(total, 0) : undefined,
    ...(isBackupTarget(resource) ? { backup: true } : {}),
  };
};

interface HostDraft {
  host: MapHost;
  resource: Resource;
  sections: Map<string, MapSection>;
}

const sectionFor = (draft: HostDraft, key: string, label: string): MapSection => {
  let section = draft.sections.get(key);
  if (!section) {
    section = { key, label, items: [] };
    draft.sections.set(key, section);
  }
  return section;
};

// The order a host's sections are kept in.
const SECTION_RANK = (key: string): number => {
  if (key === 'vms') return 1;
  if (key === 'lxcs') return 2;
  if (key === 'containers') return 3;
  return 4;
};

const placeChild = (
  draft: HostDraft,
  child: Resource,
  attachments: Attachments,
): 'placed' | 'pool' | 'skipped' => {
  const platform = draft.host.platform;
  if (child.type === 'vm') {
    sectionFor(draft, 'vms', 'VMs').items.push(toService(child, 'VM', attachments));
    return 'placed';
  }
  if (SYSTEM_CONTAINER_TYPES.has(child.type)) {
    const proxmox = platform === 'proxmox';
    sectionFor(draft, 'lxcs', proxmox ? 'LXCs' : 'System containers').items.push(
      toService(child, proxmox ? 'LXC' : SYSTEM_CONTAINER_LABEL, attachments),
    );
    return 'placed';
  }
  if (child.type === 'app-container') {
    const app = platform === 'truenas';
    sectionFor(draft, 'containers', app ? 'Apps' : 'Containers').items.push(
      toService(child, app ? 'App' : 'Container', attachments),
    );
    return 'placed';
  }
  if (child.type === 'pod') {
    const namespace = clean(child.kubernetes?.namespace) || 'default';
    sectionFor(draft, `ns:${namespace}`, namespace).items.push(
      toService(child, 'Pod', attachments),
    );
    return 'placed';
  }
  if (POOL_TYPES.has(child.type)) {
    draft.host.pools.push(toPool(child));
    return 'pool';
  }
  return 'skipped';
};

const groupKeyFor = (resource: Resource, platform: MapPlatform): string | null => {
  const cluster = clean(resource.clusterId);
  if (!cluster) return null;
  if (platform === 'proxmox') {
    return resource.proxmox?.isClusterMember === false ? null : `proxmox:${cluster}`;
  }
  if (platform === 'vmware') return `vmware:${cluster}`;
  if (platform === 'docker') return `docker:${cluster}`;
  return null;
};

const isProblem = (health: MapHealth): boolean => health === 'attention' || health === 'critical';

export function buildInfrastructureMapModel(
  resources: readonly Resource[],
): InfrastructureMapModel {
  const byId = new Map<string, Resource>();
  for (const resource of resources) byId.set(resource.id, resource);
  const networks = resolveMapNetworks(resources);
  const attachments = networks.attachments;

  const drafts = new Map<string, HostDraft>();
  for (const resource of resources) {
    const platform = hostPlatform(resource);
    if (!platform) continue;
    // A Kubernetes node is a machine too, but on the map it belongs inside
    // the cluster the registry parents it to.
    const parent = resource.parentId ? byId.get(resource.parentId) : undefined;
    if (resource.type === 'agent' && parent?.type === 'k8s-cluster') continue;
    const own = attachments.get(resource.id) ?? [];
    drafts.set(resource.id, {
      resource,
      sections: new Map(),
      host: {
        id: resource.id,
        isResource: true,
        name: displayName(resource),
        platform,
        typeLabel: HOST_TYPE_LABEL[platform],
        health: resolveMapHealth(resource),
        status: resource.status,
        cpu: finitePercent(resource.cpu?.current),
        memory: finitePercent(getMemoryPercent(resource)),
        sections: [],
        pools: [],
        ...(own[0]?.address ? { address: own[0].address } : {}),
        ownSegmentIds: own.map((entry) => entry.segmentId),
      },
    });
  }

  // The host a resource sits on is the parent the registry gives it.
  const owningDraft = (resource: Resource): HostDraft | undefined =>
    resource.parentId ? drafts.get(resource.parentId) : undefined;

  const sharedPools: Resource[] = [];
  const checks: MapItem[] = [];
  for (const resource of resources) {
    if (drafts.has(resource.id)) continue;
    if (resource.type === 'network-endpoint') {
      const device = resolveMapDeviceRole(displayName(resource));
      checks.push({ ...toItem(resource, 'Check', attachments), ...(device ? { device } : {}) });
      continue;
    }
    const draft = owningDraft(resource);
    if (draft) {
      placeChild(draft, resource, attachments);
      continue;
    }
    if (!resource.parentId && POOL_TYPES.has(resource.type)) sharedPools.push(resource);
  }

  const groups = new Map<string, MapGroup>();
  const looseHosts: MapHost[] = [];
  const hostGroup = new Map<string, MapGroup>();
  for (const draft of drafts.values()) {
    const host = draft.host;
    host.sections = [...draft.sections.values()]
      .map((section) => ({
        ...section,
        items: [...section.items].sort(byName),
      }))
      .sort(
        (a, b) => SECTION_RANK(a.key) - SECTION_RANK(b.key) || collator.compare(a.label, b.label),
      );
    host.pools.sort(byName);
    const key = groupKeyFor(draft.resource, host.platform);
    if (!key) {
      looseHosts.push(host);
      continue;
    }
    let group = groups.get(key);
    if (!group) {
      group = {
        id: `map:group:${key}`,
        name: clean(draft.resource.clusterId),
        typeLabel: GROUP_TYPE_LABEL[host.platform] ?? 'Cluster',
        platform: host.platform,
        hosts: [],
        pools: [],
      };
      groups.set(key, group);
    }
    group.hosts.push(host);
    hostGroup.set(host.id, group);
  }

  // Shared storage belongs to the cluster that reports it. Proxmox names the
  // connection, vSphere names the hosts that mount it; anything else stays in
  // its own card rather than being guessed into a cluster.
  const unplacedPools: MapPool[] = [];
  const vmwareHostsByName = new Map<string, MapHost | null>();
  for (const draft of drafts.values()) {
    if (draft.host.platform !== 'vmware') continue;
    const name = clean(draft.resource.name).toLowerCase();
    vmwareHostsByName.set(name, vmwareHostsByName.has(name) ? null : draft.host);
  }
  for (const resource of sharedPools) {
    const pool = toPool(resource);
    let target: MapGroup | undefined;
    if (resource.platformType === 'proxmox-pve') {
      target = groups.get(`proxmox:${clean(resource.proxmox?.instance)}`);
    } else if (resource.platformType === 'vmware-vsphere') {
      const mounted = (resource.storage?.nodes ?? []).map((node) =>
        vmwareHostsByName.get(clean(node).toLowerCase()),
      );
      const owners = new Set(mounted.map((host) => (host ? hostGroup.get(host.id) : undefined)));
      if (mounted.length > 0 && owners.size === 1) target = [...owners][0];
    }
    if (target) target.pools.push(pool);
    else unplacedPools.push(pool);
  }

  const blocks: MapBlock[] = [];
  for (const group of groups.values()) {
    group.hosts.sort(byName);
    group.pools.sort(byName);
    blocks.push({ kind: 'group', group });
  }
  for (const host of looseHosts) blocks.push({ kind: 'host', host });

  const blockPlatform = (block: MapBlock) =>
    block.kind === 'group' ? block.group.platform : block.host.platform;
  const blockName = (block: MapBlock) =>
    block.kind === 'group' ? block.group.name : block.host.name;
  blocks.sort(
    (a, b) =>
      PLATFORM_ORDER.indexOf(blockPlatform(a)) - PLATFORM_ORDER.indexOf(blockPlatform(b)) ||
      (a.kind === b.kind ? 0 : a.kind === 'group' ? -1 : 1) ||
      collator.compare(blockName(a), blockName(b)),
  );

  if (unplacedPools.length > 0) {
    const pools = unplacedPools.sort(byName);
    blocks.push({
      kind: 'host',
      host: {
        id: SHARED_STORAGE_HOST_ID,
        isResource: false,
        name: 'Shared storage',
        platform: 'storage',
        typeLabel: HOST_TYPE_LABEL.storage,
        health: worstHealth(pools.map((pool) => pool.health)),
        status: '',
        sections: [],
        pools,
        ownSegmentIds: [],
      },
    });
  }
  const connections = ownedConnections(resources, attachments);
  // A check that probes a machine's default gateway is on the router,
  // whatever its owner called it.
  const gatewayOwners = new Set(connections.gateways.map((gateway) => gateway.resourceId));
  for (const check of checks) {
    if (!check.device && gatewayOwners.has(check.id)) check.device = 'router';
  }
  if (checks.length > 0) {
    checks.sort(byName);
    blocks.push({
      kind: 'host',
      host: {
        id: CHECKS_HOST_ID,
        isResource: false,
        name: 'Availability checks',
        platform: 'checks',
        typeLabel: HOST_TYPE_LABEL.checks,
        health: worstHealth(checks.map((check) => check.health)),
        status: '',
        sections: [{ key: 'checks', label: 'Endpoints', items: checks }],
        pools: [],
        ownSegmentIds: [],
      },
    });
  }

  return summarize(blocks, networks.segments, connections);
}

// A gateway or resolver address that exactly one thing on the map answers on
// is that thing. Held by none, or by several, it stays a bare address.
function ownedConnections(
  resources: readonly Resource[],
  attachments: Attachments,
): MapConnections {
  const connections = resolveMapConnections(resources);
  const owners = new Map<string, Set<string>>();
  for (const [id, attached] of attachments) {
    for (const { address } of attached) {
      if (!address) continue;
      const ids = owners.get(address) ?? new Set<string>();
      ids.add(id);
      owners.set(address, ids);
    }
  }
  const own = <T extends { address: string }>(entry: T): T & { resourceId?: string } => {
    const ids = owners.get(entry.address);
    return ids?.size === 1 ? { ...entry, resourceId: [...ids][0] } : entry;
  };
  return {
    links: connections.links,
    gateways: connections.gateways.map(own),
    // A router is often the resolver too. The same address is the same box.
    resolvers: connections.resolvers.map(own).map((resolver) => {
      const gateway = connections.gateways.findIndex((entry) => entry.address === resolver.address);
      return gateway >= 0 ? { ...resolver, gateway } : resolver;
    }),
  };
}

const HEALTH_RANK: Record<MapHealth, number> = {
  critical: 5,
  attention: 4,
  stale: 3,
  unknown: 2,
  off: 1,
  ok: 0,
};

function worstHealth(values: readonly MapHealth[]): MapHealth {
  let worst: MapHealth = 'ok';
  for (const value of values) if (HEALTH_RANK[value] > HEALTH_RANK[worst]) worst = value;
  return worst;
}

const blockHosts = (block: MapBlock): MapHost[] =>
  block.kind === 'group' ? block.group.hosts : [block.host];

function summarize(
  blocks: MapBlock[],
  segments: MapSegment[],
  connections: MapConnections,
): InfrastructureMapModel {
  const counts = new Map<string, number>();
  const add = (key: string, by: number) => counts.set(key, (counts.get(key) ?? 0) + by);
  let attentionCount = 0;

  for (const block of blocks) {
    if (block.kind === 'group') {
      add('pools', block.group.pools.length);
      attentionCount += block.group.pools.filter((pool) => isProblem(pool.health)).length;
    }
    for (const host of blockHosts(block)) {
      if (host.isResource) {
        add('hosts', 1);
        if (isProblem(host.health)) attentionCount += 1;
      }
      add('pools', host.pools.length);
      attentionCount += host.pools.filter((pool) => isProblem(pool.health)).length;
      for (const section of host.sections) {
        const key = section.key.startsWith('ns:') ? 'pods' : section.key;
        add(key, section.items.length);
        attentionCount += section.items.filter((item) => isProblem(item.health)).length;
      }
    }
  }

  const TOTAL_LABELS: Array<[string, string, string]> = [
    ['hosts', 'Hosts', 'Host'],
    ['vms', 'VMs', 'VM'],
    ['lxcs', 'LXCs', 'LXC'],
    ['containers', 'Containers', 'Container'],
    ['pods', 'Pods', 'Pod'],
    ['pools', 'Storage', 'Storage'],
    ['checks', 'Checks', 'Check'],
  ];
  const totals = TOTAL_LABELS.map(([key, label, singular]) => ({
    key,
    label,
    singular,
    count: counts.get(key) ?? 0,
  })).filter((total) => total.count > 0);

  return { blocks, segments, connections, totals, attentionCount, isEmpty: blocks.length === 0 };
}

/**
 * The same map with every name replaced by what the thing is, for an image
 * that can be posted without publishing hostnames. Versions, sizes, counts
 * and health stay: they are what makes the picture worth sharing.
 */
export function anonymizeInfrastructureMapModel(
  model: InfrastructureMapModel,
): InfrastructureMapModel {
  const counters = new Map<string, number>();
  const next = (label: string): string => {
    const count = (counters.get(label) ?? 0) + 1;
    counters.set(label, count);
    return `${label} ${count}`;
  };
  // One resource keeps one stand-in name wherever it is drawn.
  const names = new Map<string, string>();
  const nameFor = (id: string, label: string): string => {
    let name = names.get(id);
    if (!name) {
      name = next(label);
      names.set(id, name);
    }
    return name;
  };
  // Pools that share a name share a stand-in. The map draws a cluster's
  // `local` on every node as one tile, and it has to stay one tile with the
  // names hidden, or the image shows a different estate from the page.
  const anonymizePools = (pools: MapPool[]): MapPool[] =>
    pools.map((pool) => ({ ...pool, name: nameFor(`pool-name:${pool.name}`, 'Pool') }));
  const anonymizeItems = (items: MapItem[]): MapItem[] =>
    items.map(({ address: _address, ...item }) => ({
      ...item,
      // A well-known app keeps its own name: it says what runs here and
      // gives nothing away about the estate.
      name: item.app
        ? item.app.label
        : nameFor(
            item.id,
            item.device ? DEVICE_STAND_IN[item.device] : itemStandIn(item.typeLabel),
          ),
    }));
  const anonymizeHost = ({ address: _address, ...host }: MapHost): MapHost => ({
    ...host,
    name: host.isResource ? nameFor(host.id, HOST_STAND_IN[host.platform]) : host.name,
    pools: anonymizePools(host.pools),
    sections: host.sections.map((section) => ({
      ...section,
      label: section.key.startsWith('ns:') ? next('Namespace') : section.label,
      items: anonymizeItems(section.items),
    })),
  });

  return {
    ...model,
    // A route's hostnames and a gateway's address identify the estate. The
    // connections and the counts stay: they are the shape of it, not its names.
    connections: {
      links: model.connections.links.map((link) => ({ ...link, hosts: [] })),
      gateways: model.connections.gateways.map((gateway) => ({ ...gateway, address: '' })),
      resolvers: model.connections.resolvers.map((resolver) => ({ ...resolver, address: '' })),
    },
    segments: model.segments.map((segment) => ({
      ...segment,
      label: segment.kind === 'other' ? segment.label : next('Network'),
      // A subnet, a cluster name or a bridge name says as much as the label.
      detail: undefined,
      bridge: undefined,
    })),
    blocks: model.blocks.map((block) =>
      block.kind === 'host'
        ? { kind: 'host', host: anonymizeHost(block.host) }
        : {
            kind: 'group',
            group: {
              ...block.group,
              name: next(block.group.typeLabel),
              hosts: block.group.hosts.map(anonymizeHost),
              pools: anonymizePools(block.group.pools),
            },
          },
    ),
  };
}
