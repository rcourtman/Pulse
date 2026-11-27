import type { Resource } from '@/types/resource';

/**
 * The connections between things that a collected value states outright.
 * Each one names both of its ends in data Pulse already holds:
 *
 * - a container whose network mode is `container:<id>` uses that container's
 *   network, so everything it sends leaves through the other one,
 * - a container carrying Traefik router labels or Caddy Docker Proxy labels is
 *   routed by the one running proxy container on the same Docker host that
 *   shares a Docker network with it, at the hostnames its labels name,
 * - a machine's default route names the address its outbound traffic goes
 *   to, and its resolver list names where it asks for names.
 *
 * Nothing here is inferred from a name, a port, a shared subnet or a missing
 * value. A reference that matches nothing, or more than one thing, draws
 * nothing.
 */

export type MapLinkKind = 'network' | 'route' | 'volume' | 'mount' | 'storage' | 'stack' | 'peer';

export interface MapLink {
  kind: MapLinkKind;
  /**
   * Where the connection starts, in the direction traffic takes: the
   * container that uses another's network, the proxy that routes to a
   * container, the workload mounting storage, or the source of a canonical
   * resource relationship.
   */
  fromId: string;
  /** Where that traffic or dependency goes. */
  toId: string;
  /**
   * The hostnames a route answers at, from the routed container's own
   * labels. Always empty for non-route links. They identify an estate, so
   * the map with names hidden carries none.
   */
  hosts: string[];
  /** Optional short factual qualifier (e.g. volume name, `:5432`, or relationship verb). */
  detail?: string;
  /** True when `detail` holds an estate-specific name that Hide names must clear. */
  sensitiveDetail?: boolean;
}

export interface MapGateway {
  /** Empty on a map with names hidden. */
  address: string;
  /** The machines whose default route goes through it. */
  machineIds: string[];
  /** The one thing on the map that answers on the address, when there is one. */
  resourceId?: string;
}

export interface MapResolver {
  /** Empty on a map with names hidden. */
  address: string;
  /** The machines configured to resolve names there. */
  machineIds: string[];
  /** The one thing on the map that answers on the address, when there is one. */
  resourceId?: string;
  /** Index of the gateway with the same address, when the resolver is one. */
  gateway?: number;
}

export interface MapConnections {
  links: MapLink[];
  gateways: MapGateway[];
  resolvers: MapResolver[];
}

export const EMPTY_MAP_CONNECTIONS: MapConnections = { links: [], gateways: [], resolvers: [] };

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

const CONTAINER_MODE = /^container:([0-9a-f]{12,64})$/i;
const ROUTER_RULE = /^traefik\.(?:http|tcp)\.routers\.[^.]+\.rule$/i;
const CADDY_ADDRESS_LABEL = /^caddy(?:_\d+)?$/i;
const VALID_HOSTNAME =
  /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/;
// `Host(...)` and `HostSNI(...)`, but not `HostRegexp(...)`: a pattern is not
// a hostname anyone can read off the map.
const HOST_CALL = /\bHost(?:SNI)?\s*\(([^)]*)\)/gi;
const QUOTED = /[`'"]([^`'"]+)[`'"]/g;

// The image a container runs, as its repository's last name without tag or
// digest: `traefik:v3`, `docker.io/library/traefik` and
// `registry.example.test:5000/mirror/traefik@sha256:...` all give `traefik`.
const imageRepository = (image: string | undefined): string => {
  const path = (image ?? '')
    .trim()
    .split('@')[0]
    .replace(/:[^/:]*$/, '');
  return (path.split('/').filter(Boolean).pop() ?? '').toLowerCase();
};

// The Docker networks a container is attached to, by name.
const attachedNetworks = (container: Resource): string[] =>
  (container.docker?.networks ?? [])
    .map((network) => (network.name ?? '').trim())
    .filter((name) => name !== '');

// Traefik itself, by the image it runs. A name is not enough, and neither is
// the publisher: `traefik/whoami` is Traefik's test server, not a proxy.
const isTraefik = (container: Resource): boolean =>
  imageRepository(container.docker?.image) === 'traefik';

// Caddy or lucaslorentz/caddy-docker-proxy, by the image repository name.
const isCaddy = (container: Resource): boolean => {
  const repo = imageRepository(container.docker?.image);
  return repo === 'caddy' || repo === 'caddy-docker-proxy';
};

/** What is written for a route: its first hostname, and how many more it has. */
export const mapRouteLabel = (hosts: readonly string[]): string =>
  hosts.length === 0 ? '' : hosts.length === 1 ? hosts[0] : `${hosts[0]} +${hosts.length - 1}`;

const isRunning = (resource: Resource): boolean => {
  const state = (resource.docker?.containerState ?? resource.status ?? '').toLowerCase();
  return state === 'running' || state === 'online';
};

// Containers on one Docker host share a daemon, which is the only scope a
// container ID or a proxy's label discovery reaches.
const hostKeyOf = (resource: Resource): string | undefined =>
  resource.docker?.hostSourceId || resource.parentId || undefined;

/** The hostnames a container's Traefik router rules name, in label order. */
export const traefikRouteHosts = (
  labels: Record<string, string> | undefined,
): { routed: boolean; hosts: string[] } => {
  if (!labels) return { routed: false, hosts: [] };
  // A container can opt out of a proxy that exposes everything by default.
  const enable = Object.entries(labels).find(([key]) => key.toLowerCase() === 'traefik.enable');
  if (enable && enable[1].trim().toLowerCase() === 'false') return { routed: false, hosts: [] };
  const hosts: string[] = [];
  let routed = false;
  for (const key of Object.keys(labels).sort()) {
    if (!ROUTER_RULE.test(key)) continue;
    routed = true;
    for (const call of labels[key].matchAll(HOST_CALL)) {
      for (const quoted of call[1].matchAll(QUOTED)) {
        const host = quoted[1].trim().toLowerCase();
        if (host && host !== '*' && !hosts.includes(host)) hosts.push(host);
      }
    }
  }
  return { routed, hosts };
};

/**
 * The hostnames a container's Caddy Docker Proxy labels name, in label order.
 * Parses comma or whitespace separated site addresses on `caddy` / `caddy_N`
 * labels, ignoring wildcards, catch-all ports, paths, and bare IP addresses.
 */
export const caddyRouteHosts = (
  labels: Record<string, string> | undefined,
): { routed: boolean; hosts: string[] } => {
  if (!labels) return { routed: false, hosts: [] };
  const hosts: string[] = [];
  for (const key of Object.keys(labels).sort()) {
    if (!CADDY_ADDRESS_LABEL.test(key)) continue;
    for (const raw of labels[key].split(/[\s,]+/)) {
      const stripped = raw.trim().replace(/^https?:\/\//i, '');
      if (
        !stripped ||
        stripped.includes('*') ||
        stripped.includes('/') ||
        stripped.startsWith(':')
      ) {
        continue;
      }
      const withoutPort = stripped.replace(/:\d+$/, '').toLowerCase();
      if (!VALID_HOSTNAME.test(withoutPort) || !/[a-z]/.test(withoutPort)) continue;
      if (!hosts.includes(withoutPort)) hosts.push(withoutPort);
    }
  }
  return { routed: hosts.length > 0, hosts };
};

const groupByAddress = (
  entries: Array<{ address: string; machineId: string }>,
): Array<{ address: string; machineIds: string[] }> => {
  const byAddress = new Map<string, string[]>();
  for (const { address, machineId } of entries) {
    const list = byAddress.get(address) ?? [];
    if (!list.includes(machineId)) list.push(machineId);
    byAddress.set(address, list);
  }
  return [...byAddress.entries()]
    .map(([address, machineIds]) => ({ address, machineIds }))
    .sort(
      (a, b) => b.machineIds.length - a.machineIds.length || collator.compare(a.address, b.address),
    );
};

const isLoopback = (address: string): boolean => address === '::1' || address.startsWith('127.');

const SHARED_CONTAINER_MAX_PEERS = 6;
const ANONYMOUS_DOCKER_VOLUME = /^[0-9a-f]{64}$/i;
const DOCKER_VOLUME_PATH = /^\/var\/lib\/docker\/volumes\/([^/]+)(?:\/_data)?\/?$/i;
const DEFAULT_DOCKER_NETWORKS = new Set(['bridge', 'host', 'none', 'ingress', 'docker_gwbridge']);

const SYSTEM_BIND_EXACT = new Set([
  '/',
  '/etc/localtime',
  '/etc/timezone',
  '/etc/hosts',
  '/etc/resolv.conf',
  '/etc/passwd',
  '/etc/group',
  '/var/run/docker.sock',
  '/run/docker.sock',
]);

const SYSTEM_BIND_PREFIXES = [
  '/proc',
  '/sys',
  '/dev',
  '/run',
  '/boot',
  '/lib',
  '/lib64',
  '/usr',
  '/bin',
  '/sbin',
];

const DATA_BACKEND_REPOS = new Set([
  'postgres',
  'postgresql',
  'postgis',
  'timescaledb',
  'pgvector',
  'mysql',
  'mariadb',
  'redis',
  'valkey',
  'dragonfly',
  'memcached',
  'mongo',
  'mongodb',
  'influxdb',
  'clickhouse',
  'clickhouse-server',
  'rabbitmq',
  'mosquitto',
  'eclipse-mosquitto',
  'emqx',
  'nats',
  'kafka',
  'redpanda',
  'meilisearch',
  'elasticsearch',
  'opensearch',
  'qdrant',
  'chroma',
  'chromadb',
  'minio',
  'garage',
  'victoriametrics',
  'victoria-metrics',
]);

const normalizeUnixPath = (raw: string | undefined): string => {
  const trimmed = (raw ?? '').trim();
  if (!trimmed.startsWith('/')) return '';
  if (trimmed === '/') return '/';
  return trimmed.replace(/\/+$/, '');
};

const isSystemBindPath = (path: string): boolean => {
  if (!path || SYSTEM_BIND_EXACT.has(path)) return true;
  return SYSTEM_BIND_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
};

const extractNamedDockerVolume = (mount: {
  type?: string;
  source?: string;
}): string | undefined => {
  const kind = (mount.type ?? '').trim().toLowerCase();
  const source = (mount.source ?? '').trim();
  if (!source) return undefined;
  if (kind === 'volume') {
    const fromPath = DOCKER_VOLUME_PATH.exec(source);
    const name = (fromPath ? fromPath[1] : source).trim();
    if (!name || name.includes('/') || ANONYMOUS_DOCKER_VOLUME.test(name)) return undefined;
    return name;
  }
  const fromPath = DOCKER_VOLUME_PATH.exec(source);
  if (fromPath) {
    const name = fromPath[1].trim();
    if (name && !ANONYMOUS_DOCKER_VOLUME.test(name)) return name;
  }
  return undefined;
};

const isDataBackendContainer = (container: Resource): boolean => {
  const repo = imageRepository(container.docker?.image);
  if (DATA_BACKEND_REPOS.has(repo)) return true;
  for (const token of DATA_BACKEND_REPOS) {
    if (repo.startsWith(`${token}-`) || repo.endsWith(`-${token}`)) return true;
  }
  return false;
};

const pathMatchesPrefix = (childPath: string, rootPath: string): boolean => {
  if (!childPath || !rootPath || rootPath === '/') return false;
  return childPath === rootPath || childPath.startsWith(`${rootPath}/`);
};

const relationshipPortDetail = (
  metadata: Record<string, unknown> | undefined,
): string | undefined => {
  if (!metadata) return undefined;
  const rawPort = metadata.port;
  if (typeof rawPort === 'number' && Number.isFinite(rawPort) && rawPort > 0) {
    return `:${Math.round(rawPort)}`;
  }
  const rawPorts = metadata.ports;
  if (Array.isArray(rawPorts) && rawPorts.length > 0) {
    const valid = rawPorts.filter(
      (value): value is number => typeof value === 'number' && Number.isFinite(value) && value > 0,
    );
    if (valid.length === 1) return `:${Math.round(valid[0])}`;
    if (valid.length > 1) return `:${Math.round(valid[0])} +${valid.length - 1}`;
  }
  return undefined;
};

const PEER_RELATIONSHIP_TYPES = new Set([
  'communicates_with',
  'depends_on',
  'stores_on',
  'protected_by',
  'mounted_to',
  'exposed_by',
]);

export function resolveMapConnections(resources: readonly Resource[]): MapConnections {
  const containers = resources.filter((resource) => resource.type === 'app-container');
  const byHost = new Map<string, Resource[]>();
  const hostPoolsByParent = new Map<string, Array<{ id: string; path: string }>>();
  const resourceIds = new Set<string>();
  for (const resource of resources) {
    resourceIds.add(resource.id);
    if (
      (resource.type === 'storage' ||
        resource.type === 'pool' ||
        resource.type === 'datastore' ||
        resource.type === 'ceph') &&
      resource.parentId
    ) {
      const poolPath = normalizeUnixPath(resource.storage?.path);
      if (poolPath && poolPath !== '/') {
        const list = hostPoolsByParent.get(resource.parentId) ?? [];
        list.push({ id: resource.id, path: poolPath });
        hostPoolsByParent.set(resource.parentId, list);
      }
    }
  }
  for (const container of containers) {
    const key = hostKeyOf(container);
    if (!key) continue;
    const list = byHost.get(key) ?? [];
    list.push(container);
    byHost.set(key, list);
  }

  const links: MapLink[] = [];
  const emittedKindPairs = new Set<string>();
  const pushUniqueLink = (link: MapLink, undirected = false) => {
    if (link.fromId === link.toId) return;
    const a = undirected && link.fromId > link.toId ? link.toId : link.fromId;
    const b = undirected && link.fromId > link.toId ? link.fromId : link.toId;
    const key = `${link.kind}\0${a}\0${b}`;
    if (emittedKindPairs.has(key)) return;
    emittedKindPairs.add(key);
    links.push(link);
  };

  for (const siblings of byHost.values()) {
    // Whose network each container uses, when its mode names one of these.
    const owner = new Map<string, Resource>();
    for (const container of siblings) {
      const mode = CONTAINER_MODE.exec((container.docker?.networkMode ?? '').trim());
      if (!mode) continue;
      const reference = mode[1].toLowerCase();
      const owners = siblings.filter(
        (other) =>
          other.id !== container.id &&
          (other.docker?.containerId ?? '').toLowerCase().startsWith(reference),
      );
      if (owners.length !== 1) continue;
      owner.set(container.id, owners[0]);
      pushUniqueLink({ kind: 'network', fromId: container.id, toId: owners[0].id, hosts: [] });
    }

    // A container inside another's network is reached where that one is.
    const networksOf = (container: Resource): string[] =>
      attachedNetworks(owner.get(container.id) ?? container);
    const onHostNetwork = (container: Resource): boolean =>
      (container.docker?.networkMode ?? '').trim().toLowerCase() === 'host' ||
      networksOf(container).includes('host');
    const canReach = (proxy: Resource, target: Resource): boolean => {
      if (onHostNetwork(proxy) || onHostNetwork(target)) return true;
      const reachable = new Set(networksOf(proxy));
      return networksOf(target).some((name) => reachable.has(name));
    };

    const addRoute = (fromId: string, toId: string, hosts: string[]) => {
      pushUniqueLink({ kind: 'route', fromId, toId, hosts });
    };

    const traefikProxies = siblings.filter(
      (container) => isRunning(container) && isTraefik(container),
    );
    const caddyProxies = siblings.filter((container) => isRunning(container) && isCaddy(container));
    for (const container of siblings) {
      const traefik = traefikRouteHosts(container.docker?.labels);
      if (traefik.routed) {
        // The labels are written for a Traefik that can reach the container.
        // With none that can, or several, they do not say which one routes.
        const able = traefikProxies.filter(
          (proxy) => proxy.id !== container.id && canReach(proxy, container),
        );
        if (able.length === 1) {
          addRoute(able[0].id, container.id, traefik.hosts);
        }
      }
      const caddy = caddyRouteHosts(container.docker?.labels);
      if (caddy.routed) {
        const able = caddyProxies.filter(
          (proxy) => proxy.id !== container.id && canReach(proxy, container),
        );
        if (able.length === 1) {
          addRoute(able[0].id, container.id, caddy.hosts);
        }
      }
    }

    // 1. Docker named volume sharing, shared non-system bind mounts, and
    //    bind-mount-to-host-storage-pool attribution on this Docker host.
    const byVolume = new Map<string, Resource[]>();
    const byBindSource = new Map<string, Resource[]>();
    for (const container of siblings) {
      const parentPools = container.parentId
        ? (hostPoolsByParent.get(container.parentId) ?? [])
        : [];
      const sortedPools =
        parentPools.length > 1
          ? [...parentPools].sort((a, b) => b.path.length - a.path.length)
          : parentPools;
      const seenVolumes = new Set<string>();
      const seenBinds = new Set<string>();

      for (const mount of container.docker?.mounts ?? []) {
        const volumeName = extractNamedDockerVolume(mount);
        if (volumeName) {
          if (!seenVolumes.has(volumeName)) {
            seenVolumes.add(volumeName);
            const list = byVolume.get(volumeName) ?? [];
            list.push(container);
            byVolume.set(volumeName, list);
          }
          continue;
        }
        const mountType = (mount.type ?? '').trim().toLowerCase();
        if (mountType && mountType !== 'bind') continue;
        const sourcePath = normalizeUnixPath(mount.source);
        if (!sourcePath || isSystemBindPath(sourcePath)) continue;

        if (!seenBinds.has(sourcePath)) {
          seenBinds.add(sourcePath);
          const list = byBindSource.get(sourcePath) ?? [];
          list.push(container);
          byBindSource.set(sourcePath, list);
        }

        // Match deepest storage pool path on the same host (e.g. /mnt/cachepool/appdata -> cachepool).
        if (sortedPools.length > 0) {
          const matchedPool = sortedPools.find((pool) => pathMatchesPrefix(sourcePath, pool.path));
          if (matchedPool) {
            pushUniqueLink({
              kind: 'storage',
              fromId: container.id,
              toId: matchedPool.id,
              hosts: [],
            });
          }
        }
      }
    }

    for (const [volumeName, members] of byVolume) {
      if (members.length < 2 || members.length > SHARED_CONTAINER_MAX_PEERS) continue;
      const sorted = [...members].sort((a, b) => collator.compare(a.id, b.id));
      for (let i = 0; i < sorted.length; i += 1) {
        for (let j = i + 1; j < sorted.length; j += 1) {
          pushUniqueLink(
            {
              kind: 'volume',
              fromId: sorted[i].id,
              toId: sorted[j].id,
              hosts: [],
              detail: volumeName,
              sensitiveDetail: true,
            },
            true,
          );
        }
      }
    }

    for (const members of byBindSource.values()) {
      if (members.length < 2 || members.length > SHARED_CONTAINER_MAX_PEERS) continue;
      const sorted = [...members].sort((a, b) => collator.compare(a.id, b.id));
      for (let i = 0; i < sorted.length; i += 1) {
        for (let j = i + 1; j < sorted.length; j += 1) {
          pushUniqueLink(
            {
              kind: 'mount',
              fromId: sorted[i].id,
              toId: sorted[j].id,
              hosts: [],
            },
            true,
          );
        }
      }
    }

    // 2. Compose stack and small private Docker network app -> database/broker wiring.
    const linkAppsToBackends = (members: readonly Resource[]) => {
      if (members.length < 2 || members.length > SHARED_CONTAINER_MAX_PEERS) return;
      const backends = members.filter((container) => isDataBackendContainer(container));
      if (backends.length === 0 || backends.length === members.length) return;
      const apps = members.filter((container) => !isDataBackendContainer(container));
      for (const app of apps) {
        for (const backend of backends) {
          pushUniqueLink({
            kind: 'stack',
            fromId: app.id,
            toId: backend.id,
            hosts: [],
          });
        }
      }
    };

    const byCompose = new Map<string, Resource[]>();
    const byPrivateNetwork = new Map<string, Resource[]>();
    for (const container of siblings) {
      const project = (
        container.docker?.composeProject ??
        container.docker?.podman?.composeProject ??
        ''
      ).trim();
      if (project) {
        const list = byCompose.get(project) ?? [];
        list.push(container);
        byCompose.set(project, list);
      }
      for (const netName of attachedNetworks(container)) {
        if (DEFAULT_DOCKER_NETWORKS.has(netName.toLowerCase())) continue;
        const list = byPrivateNetwork.get(netName) ?? [];
        list.push(container);
        byPrivateNetwork.set(netName, list);
      }
    }
    for (const members of byCompose.values()) linkAppsToBackends(members);
    for (const members of byPrivateNetwork.values()) linkAppsToBackends(members);
  }

  // 3. Canonical resource.relationships[] (passive socket peers, dependencies, storage/backup links).
  for (const resource of resources) {
    for (const rel of resource.relationships ?? []) {
      if (rel.active === false) continue;
      if (!PEER_RELATIONSHIP_TYPES.has(rel.type)) continue;
      const fromId = (rel.sourceId || resource.id).trim();
      const toId = (rel.targetId || '').trim();
      if (!fromId || !toId || fromId === toId) continue;
      if (!resourceIds.has(fromId) || !resourceIds.has(toId)) continue;
      const portDetail =
        rel.type === 'communicates_with' ? relationshipPortDetail(rel.metadata) : undefined;
      pushUniqueLink({
        kind: 'peer',
        fromId,
        toId,
        hosts: [],
        detail: portDetail ?? rel.type,
      });
    }
  }

  links.sort(
    (a, b) =>
      collator.compare(a.kind, b.kind) ||
      collator.compare(a.toId, b.toId) ||
      collator.compare(a.fromId, b.fromId),
  );

  const gateways: Array<{ address: string; machineId: string }> = [];
  const resolvers: Array<{ address: string; machineId: string }> = [];
  for (const resource of resources) {
    // The first default route is the one with the lowest metric: the one the
    // machine uses. A second is a standby and carries nothing while the
    // first is up.
    const gateway = resource.agent?.defaultGateways?.[0]?.address?.trim();
    if (gateway) gateways.push({ address: gateway, machineId: resource.id });
    for (const raw of resource.agent?.dnsServers ?? []) {
      const address = raw.trim();
      // A resolver on loopback is the machine itself, which is no connection.
      if (address && !isLoopback(address)) resolvers.push({ address, machineId: resource.id });
    }
  }

  return {
    links,
    gateways: groupByAddress(gateways),
    resolvers: groupByAddress(resolvers),
  };
}
