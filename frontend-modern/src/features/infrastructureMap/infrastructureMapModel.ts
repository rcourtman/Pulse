import type { Resource, ResourceType } from '@/types/resource';
import { getDiskPercent, getMemoryPercent } from '@/types/resource';
import { formatBytes } from '@/utils/format';
import {
  dockerHostWire,
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
  /** Published ports, for a container or an exposed service. */
  detail?: string;
  /** The networks the item is on, as ids into the model's segments. */
  segmentIds?: string[];
}

export interface MapSection {
  key: string;
  label: string;
  /** A fact about the section itself, such as a container network's subnet. */
  note?: string;
  /** Sections that only ever draw as marks (physical disks). */
  marksOnly?: boolean;
  items: MapItem[];
}

export interface MapPool {
  id: string;
  name: string;
  health: MapHealth;
  usedPercent?: number;
  sizeLabel?: string;
}

export interface MapHost {
  id: string;
  /** False for the cards the map composes itself (shared storage, checks). */
  isResource: boolean;
  name: string;
  platform: MapPlatform;
  typeLabel: string;
  subtitle: string;
  health: MapHealth;
  status: string;
  cpu?: number;
  memory?: number;
  disk?: number;
  sections: MapSection[];
  /** A Docker host's containers again, grouped by the network they share. */
  networkSections?: MapSection[];
  pools: MapPool[];
  /** The address the host itself answers on. */
  address?: string;
  /** Networks the host has an address on. The first is where it is drawn. */
  ownSegmentIds: string[];
  /** Networks only its guests are on, which it therefore carries. */
  guestSegmentIds: string[];
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

const joinFacts = (facts: Array<string | undefined | false>): string =>
  facts.filter((fact): fact is string => Boolean(fact)).join(' · ');

const coresFact = (count: number | undefined): string | undefined =>
  typeof count === 'number' && count > 0 ? `${count} ${count === 1 ? 'core' : 'cores'}` : undefined;

const memoryFact = (resource: Resource): string | undefined => {
  const total = resource.memory?.total;
  return typeof total === 'number' && total > 0 ? `${formatBytes(total, 0)} RAM` : undefined;
};

const osFact = (resource: Resource): string | undefined => {
  const name = clean(resource.agent?.osName);
  const version = clean(resource.agent?.osVersion);
  if (!name) return undefined;
  return version && !name.includes(version) ? `${name} ${version}` : name;
};

const hostSubtitle = (resource: Resource, platform: MapPlatform): string => {
  switch (platform) {
    case 'proxmox': {
      const version = clean(resource.proxmox?.pveVersion).replace(/^pve-manager\//, '');
      return joinFacts([
        version ? `PVE ${version.split('/')[0]}` : 'Proxmox VE',
        coresFact(resource.agent?.cpuCount ?? resource.proxmox?.cpus),
        memoryFact(resource),
      ]);
    }
    case 'docker': {
      const runtime =
        clean(resource.docker?.runtime).toLowerCase() === 'podman' ? 'Podman' : 'Docker';
      const version =
        clean(resource.docker?.runtimeVersion) || clean(resource.docker?.dockerVersion);
      return joinFacts([
        version ? `${runtime} ${version}` : runtime,
        coresFact(dockerHostWire(resource).cpus),
        memoryFact(resource),
      ]);
    }
    case 'kubernetes':
      return joinFacts(['Kubernetes', clean(resource.kubernetes?.version)]);
    case 'backup': {
      const version = clean(resource.pbs?.version);
      return version ? `Proxmox Backup Server ${version}` : 'Proxmox Backup Server';
    }
    case 'mail': {
      const version = clean(resource.pmg?.version);
      return version ? `Proxmox Mail Gateway ${version}` : 'Proxmox Mail Gateway';
    }
    case 'truenas':
      return joinFacts([osFact(resource) ?? 'TrueNAS', memoryFact(resource)]);
    case 'vmware':
      return joinFacts([
        osFact(resource) ?? 'VMware ESXi',
        coresFact(resource.agent?.cpuCount),
        memoryFact(resource),
      ]);
    default:
      return joinFacts([
        osFact(resource),
        coresFact(resource.agent?.cpuCount),
        memoryFact(resource),
      ]);
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

const toPool = (resource: Resource): MapPool => {
  const total = resource.disk?.total;
  return {
    id: resource.id,
    name: displayName(resource),
    health: resolveMapHealth(resource),
    usedPercent: finitePercent(getDiskPercent(resource)),
    sizeLabel: typeof total === 'number' && total > 0 ? formatBytes(total, 0) : undefined,
  };
};

interface HostDraft {
  host: MapHost;
  resource: Resource;
  sections: Map<string, MapSection>;
  containers: Resource[];
  containerNetworks: Resource[];
}

const sectionFor = (
  draft: HostDraft,
  key: string,
  label: string,
  marksOnly = false,
): MapSection => {
  let section = draft.sections.get(key);
  if (!section) {
    section = { key, label, items: [], ...(marksOnly ? { marksOnly } : {}) };
    draft.sections.set(key, section);
  }
  return section;
};

// The order sections read in on a card: what runs there first, then what it
// is made of.
const SECTION_RANK = (key: string): number => {
  if (key === 'nodes') return 0;
  if (key === 'vms') return 1;
  if (key === 'lxcs') return 2;
  if (key === 'containers') return 3;
  if (key.startsWith('ns:')) return 4;
  if (key === 'exposed') return 8;
  if (key === 'disks') return 9;
  return 5;
};

const exposedServiceDetail = (resource: Resource): string | undefined => {
  const ports = resource.kubernetes?.servicePorts ?? [];
  const numbers = ports
    .map((port) => port.nodePort ?? port.port)
    .filter((port): port is number => typeof port === 'number' && port > 0);
  return numbers.length > 0 ? `:${numbers.slice(0, 3).join(', ')}` : undefined;
};

const placeChild = (
  draft: HostDraft,
  child: Resource,
  attachments: Attachments,
): 'placed' | 'pool' | 'skipped' => {
  const platform = draft.host.platform;
  if (child.type === 'vm') {
    sectionFor(draft, 'vms', 'VMs').items.push(toItem(child, 'VM', attachments));
    return 'placed';
  }
  if (SYSTEM_CONTAINER_TYPES.has(child.type)) {
    const proxmox = platform === 'proxmox';
    sectionFor(draft, 'lxcs', proxmox ? 'LXCs' : 'System containers').items.push(
      toItem(child, proxmox ? 'LXC' : 'System container', attachments),
    );
    return 'placed';
  }
  if (child.type === 'app-container') {
    const app = platform === 'truenas';
    sectionFor(draft, 'containers', app ? 'Apps' : 'Containers').items.push(
      toItem(child, app ? 'App' : 'Container', attachments),
    );
    if (platform === 'docker') draft.containers.push(child);
    return 'placed';
  }
  if (child.type === 'pod') {
    const namespace = clean(child.kubernetes?.namespace) || 'default';
    sectionFor(draft, `ns:${namespace}`, namespace).items.push(toItem(child, 'Pod', attachments));
    return 'placed';
  }
  if (child.type === 'k8s-node' || (child.type === 'agent' && platform === 'kubernetes')) {
    sectionFor(draft, 'nodes', 'Nodes').items.push(toItem(child, 'Node', attachments));
    return 'placed';
  }
  if (child.type === 'physical_disk') {
    sectionFor(draft, 'disks', 'Disks', true).items.push(toItem(child, 'Disk', attachments));
    return 'placed';
  }
  if (POOL_TYPES.has(child.type)) {
    draft.host.pools.push(toPool(child));
    return 'pool';
  }
  if (child.type === 'docker-network') {
    draft.containerNetworks.push(child);
    return 'skipped';
  }
  // What a cluster offers outside itself: a service with a node port or a
  // load balancer, and every ingress.
  const serviceType = child.kubernetes?.serviceType;
  if (
    child.type === 'k8s-ingress' ||
    (child.type === 'k8s-service' && (serviceType === 'NodePort' || serviceType === 'LoadBalancer'))
  ) {
    const exposed = child.type === 'k8s-ingress' ? 'Ingress' : 'Service';
    sectionFor(draft, 'exposed', 'Exposed').items.push({
      ...toItem(child, exposed, attachments),
      ...(exposedServiceDetail(child) ? { detail: exposedServiceDetail(child) } : {}),
    });
    return 'placed';
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

/**
 * A Docker host's containers grouped by the container network they share,
 * which is the isolation Docker itself enforces: two containers with no
 * network in common cannot reach each other by name. A container on several
 * networks is listed under each, because that is what joins them.
 */
const containerNetworkSections = (draft: HostDraft, attachments: Attachments): MapSection[] => {
  const facts = new Map<string, string>();
  for (const network of draft.containerNetworks) {
    const subnet = network.docker?.subnets?.[0]?.subnet;
    const driver = clean(network.docker?.driver);
    facts.set(
      clean(network.name),
      [subnet, driver && driver !== 'bridge' ? driver : ''].filter(Boolean).join(' · '),
    );
  }
  const sections = new Map<string, MapSection>();
  for (const container of draft.containers) {
    const names = (container.docker?.networks ?? []).map((network) => clean(network.name));
    const joined = names.filter(Boolean);
    for (const name of joined.length > 0 ? joined : ['']) {
      const key = `net:${name}`;
      let section = sections.get(key);
      if (!section) {
        const note = facts.get(name);
        section = {
          key,
          label: name || 'No network reported',
          items: [],
          ...(note ? { note } : {}),
        };
        sections.set(key, section);
      }
      section.items.push(toItem(container, 'Container', attachments));
    }
  }
  return [...sections.values()]
    .map((section) => ({ ...section, items: [...section.items].sort(byName) }))
    .sort(
      (a, b) =>
        Number(a.key === 'net:') - Number(b.key === 'net:') ||
        b.items.length - a.items.length ||
        collator.compare(a.label, b.label),
    );
};

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
      containers: [],
      containerNetworks: [],
      host: {
        id: resource.id,
        isResource: true,
        name: displayName(resource),
        platform,
        typeLabel: HOST_TYPE_LABEL[platform],
        subtitle: hostSubtitle(resource, platform),
        health: resolveMapHealth(resource),
        status: resource.status,
        cpu: finitePercent(resource.cpu?.current),
        memory: finitePercent(getMemoryPercent(resource)),
        disk: finitePercent(getDiskPercent(resource)),
        sections: [],
        pools: [],
        ...(own[0]?.address ? { address: own[0].address } : {}),
        ownSegmentIds: own.map((entry) => entry.segmentId),
        guestSegmentIds: [],
      },
    });
  }

  // The host a resource sits on: its parent, or its parent's parent for a
  // disk that reports the pool it belongs to.
  const owningDraft = (resource: Resource): HostDraft | undefined => {
    const parent = resource.parentId ? byId.get(resource.parentId) : undefined;
    if (!parent) return undefined;
    const direct = drafts.get(parent.id);
    if (direct) return direct;
    if (resource.type !== 'physical_disk' || !parent.parentId) return undefined;
    return drafts.get(parent.parentId);
  };

  const sharedPools: Resource[] = [];
  const checks: MapItem[] = [];
  for (const resource of resources) {
    if (drafts.has(resource.id)) continue;
    if (resource.type === 'network-endpoint') {
      checks.push(toItem(resource, 'Check', attachments));
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
        items: section.marksOnly ? section.items : [...section.items].sort(byName),
      }))
      .sort(
        (a, b) => SECTION_RANK(a.key) - SECTION_RANK(b.key) || collator.compare(a.label, b.label),
      );
    host.pools.sort(byName);
    const carried = new Set<string>();
    for (const section of host.sections) {
      for (const item of section.items) {
        for (const id of item.segmentIds ?? [])
          if (!host.ownSegmentIds.includes(id)) carried.add(id);
      }
    }
    host.guestSegmentIds = [...carried];
    if (host.platform === 'docker' && draft.containers.length > 0) {
      host.networkSections = containerNetworkSections(draft, attachments);
    }

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
        subtitle: `${pools.length} ${pools.length === 1 ? 'pool' : 'pools'}`,
        health: worstHealth(pools.map((pool) => pool.health)),
        status: '',
        sections: [],
        pools,
        ownSegmentIds: [],
        guestSegmentIds: [],
      },
    });
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
        subtitle: `${checks.length} ${checks.length === 1 ? 'endpoint' : 'endpoints'} Pulse probes`,
        health: worstHealth(checks.map((check) => check.health)),
        status: '',
        sections: [{ key: 'checks', label: 'Endpoints', items: checks }],
        pools: [],
        ownSegmentIds: [],
        guestSegmentIds: [...new Set(checks.flatMap((check) => check.segmentIds ?? []))],
      },
    });
  }

  return summarize(blocks, networks.segments);
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

function summarize(blocks: MapBlock[], segments: MapSegment[]): InfrastructureMapModel {
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
    ['disks', 'Disks', 'Disk'],
    ['checks', 'Checks', 'Check'],
  ];
  const totals = TOTAL_LABELS.map(([key, label, singular]) => ({
    key,
    label,
    singular,
    count: counts.get(key) ?? 0,
  })).filter((total) => total.count > 0);

  return { blocks, segments, totals, attentionCount, isEmpty: blocks.length === 0 };
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
  const anonymizePools = (pools: MapPool[]): MapPool[] =>
    pools.map((pool) => ({ ...pool, name: nameFor(pool.id, 'Pool') }));
  const anonymizeItems = (items: MapItem[]): MapItem[] =>
    items.map(({ address: _address, ...item }) => ({
      ...item,
      name: nameFor(item.id, item.typeLabel),
    }));
  const anonymizeHost = ({ address: _address, ...host }: MapHost): MapHost => ({
    ...host,
    name: host.isResource ? nameFor(host.id, host.typeLabel) : host.name,
    pools: anonymizePools(host.pools),
    sections: host.sections.map((section) => ({
      ...section,
      label: section.key.startsWith('ns:') ? next('Namespace') : section.label,
      items: anonymizeItems(section.items),
    })),
    ...(host.networkSections
      ? {
          networkSections: host.networkSections.map(({ note: _note, ...section }) => ({
            ...section,
            label: section.key === 'net:' ? section.label : next('Container network'),
            items: anonymizeItems(section.items),
          })),
        }
      : {}),
  });

  return {
    ...model,
    segments: model.segments.map((segment) => ({
      ...segment,
      label: segment.kind === 'other' ? segment.label : next('Network'),
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
