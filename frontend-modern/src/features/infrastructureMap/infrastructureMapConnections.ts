import type { Resource } from '@/types/resource';

/**
 * The connections between things that a collected value states outright.
 * Each one names both of its ends in data Pulse already holds:
 *
 * - a container whose network mode is `container:<id>` uses that container's
 *   network, so everything it sends leaves through the other one;
 * - a container carrying Traefik router labels is routed by the one running
 *   Traefik container on the same Docker host that shares a Docker network
 *   with it, at the hostnames its rules name;
 * - a machine's default route names the address its outbound traffic goes
 *   to, and its resolver list names where it asks for names.
 *
 * Nothing here is inferred from a name, a port, a shared subnet or a missing
 * value. A reference that matches nothing, or more than one thing, draws
 * nothing.
 */

export type MapLinkKind = 'network' | 'route';

export interface MapLink {
  kind: MapLinkKind;
  /**
   * Where the connection starts, in the direction traffic takes: the
   * container that uses another's network, or the proxy that routes to a
   * container.
   */
  fromId: string;
  /** Where that traffic goes. */
  toId: string;
  /**
   * The hostnames a route answers at, from the routed container's own
   * labels. Always empty for a shared network. They identify an estate, so
   * the map with names hidden carries none.
   */
  hosts: string[];
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

/** What is written for a route: its first hostname, and how many more it has. */
export const mapRouteLabel = (hosts: readonly string[]): string =>
  hosts.length === 0 ? '' : hosts.length === 1 ? hosts[0] : `${hosts[0]} +${hosts.length - 1}`;

const isRunning = (resource: Resource): boolean =>
  (resource.docker?.containerState ?? resource.status ?? '').toLowerCase() === 'running';

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

export function resolveMapConnections(resources: readonly Resource[]): MapConnections {
  const containers = resources.filter((resource) => resource.type === 'app-container');
  const byHost = new Map<string, Resource[]>();
  for (const container of containers) {
    const key = hostKeyOf(container);
    if (!key) continue;
    const list = byHost.get(key) ?? [];
    list.push(container);
    byHost.set(key, list);
  }

  const links: MapLink[] = [];
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
      links.push({ kind: 'network', fromId: container.id, toId: owners[0].id, hosts: [] });
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

    const proxies = siblings.filter((container) => isRunning(container) && isTraefik(container));
    for (const container of siblings) {
      const { routed, hosts } = traefikRouteHosts(container.docker?.labels);
      if (!routed) continue;
      // The labels are written for a Traefik that can reach the container.
      // With none that can, or several, they do not say which one routes.
      const able = proxies.filter(
        (proxy) => proxy.id !== container.id && canReach(proxy, container),
      );
      if (able.length === 1) {
        links.push({ kind: 'route', fromId: able[0].id, toId: container.id, hosts });
      }
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
