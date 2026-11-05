import { Show, createContext, useContext, type Accessor, type Component } from 'solid-js';
import type { Resource } from '@/types/resource';
import { LXC_DOCKER_HOST_PREFIX } from '@/utils/resourceBadgePresentation';
import { asTrimmedString } from '@/utils/stringUtils';

// A hostname does not identify a Docker host: two sites can reuse one, and each
// host still has its own row, containers and status. A unique hostname stays
// the page's key, so links and bookmarks keep reading well. Hosts sharing one
// key on their IDs instead and carry a few words that tell them apart.

export interface DockerHostIdentities {
  /** URL scope and group key: the hostname, or the host's ID when others share it. */
  scopeFor: (resource: Resource) => string;
  /** What tells this resource's host from others with its hostname, if any do. */
  qualifierFor: (resource: Resource) => string | null;
  /** The hostname, qualified when others share it. */
  labelFor: (resource: Resource) => string;
}

const hostSourceId = (resource: Resource): string | undefined =>
  asTrimmedString(resource.docker?.hostSourceId);

export const dockerHostName = (resource: Resource): string =>
  asTrimmedString(resource.docker?.hostname) || '—';

// Most recognisable first. A probed row's ID can lag a node move, so the agent
// ID names its guest; the node is left out for the same reason.
const qualifierCandidates = (host: Resource, sourceId: string): string[] => {
  const name = asTrimmedString(host.name);
  const hostname = asTrimmedString(host.docker?.hostname);
  const probeId = [host.docker?.agentId, sourceId].find((id) =>
    id?.startsWith(LXC_DOCKER_HOST_PREFIX),
  );
  const probe = probeId?.slice(LXC_DOCKER_HOST_PREFIX.length).split(':') ?? [];
  const connection = probe.slice(0, -2).join(':');
  const vmid = probe.at(-1);
  const machineId = asTrimmedString(host.docker?.machineId);
  return [
    name !== hostname ? name : undefined,
    connection,
    vmid ? `LXC ${vmid}` : undefined,
    host.identity?.ips?.[0],
    machineId?.slice(0, 8),
    sourceId,
  ].filter((candidate): candidate is string => Boolean(candidate));
};

export const buildDockerHostIdentities = (hosts: readonly Resource[]): DockerHostIdentities => {
  const hostsByHostname = new Map<string, Map<string, Resource>>();
  for (const host of hosts) {
    const hostname = asTrimmedString(host.docker?.hostname);
    const sourceId = hostSourceId(host);
    if (!hostname || !sourceId) continue;
    const sharing = hostsByHostname.get(hostname) ?? new Map<string, Resource>();
    sharing.set(sourceId, host);
    hostsByHostname.set(hostname, sharing);
  }

  const qualifiers = new Map<string, string>();
  for (const sharing of hostsByHostname.values()) {
    if (sharing.size < 2) continue;
    const candidates = [...sharing].map(
      ([sourceId, host]) => [sourceId, qualifierCandidates(host, sourceId)] as const,
    );
    for (const [sourceId, own] of candidates) {
      const others = candidates.flatMap(([other, values]) => (other === sourceId ? [] : values));
      qualifiers.set(sourceId, own.find((value) => !others.includes(value)) ?? sourceId);
    }
  }

  const qualifierFor = (resource: Resource): string | null =>
    qualifiers.get(hostSourceId(resource) ?? '') ?? null;
  return {
    scopeFor: (resource) => {
      const sourceId = hostSourceId(resource);
      return sourceId && qualifiers.has(sourceId) ? sourceId : dockerHostName(resource);
    },
    qualifierFor,
    labelFor: (resource) => {
      const qualifier = qualifierFor(resource);
      const hostname = dockerHostName(resource);
      return qualifier ? `${hostname} (${qualifier})` : hostname;
    },
  };
};

/**
 * Whether a URL host scope selects this resource. A hostname still selects
 * every host with it, so links made before those hosts shared one keep working.
 */
export const dockerResourceMatchesHostScope = (resource: Resource, scope: string): boolean =>
  dockerHostName(resource) === scope || hostSourceId(resource) === scope;

/** A host row's own name and, when it does not already say, what tells it apart. */
export const getDockerHostRowName = (
  host: Resource,
  identities: DockerHostIdentities,
): { name: string; qualifier: string | null; label: string } => {
  const name = asTrimmedString(host.name) || host.id;
  const qualifier = identities.qualifierFor(host);
  return qualifier && qualifier !== name
    ? { name, qualifier, label: `${name} (${qualifier})` }
    : { name, qualifier: null, label: name };
};

const NO_SHARED_HOSTNAMES = buildDockerHostIdentities([]);

const DockerHostIdentitiesContext = createContext<Accessor<DockerHostIdentities>>(
  () => NO_SHARED_HOSTNAMES,
);

export const DockerHostIdentitiesProvider = DockerHostIdentitiesContext.Provider;

export const useDockerHostIdentities = (): Accessor<DockerHostIdentities> =>
  useContext(DockerHostIdentitiesContext);

/** Beside a truncating name: stays one line, and the name gives way first. */
export const DockerHostQualifier: Component<{ value: string | null }> = (props) => (
  <Show when={props.value}>
    {(value) => (
      <span
        class="max-w-[45%] shrink-0 truncate text-[10px] font-normal normal-case text-muted"
        title={value()}
        data-docker-host-qualifier
      >
        {value()}
      </span>
    )}
  </Show>
);
