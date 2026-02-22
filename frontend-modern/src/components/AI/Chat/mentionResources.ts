import type { State } from '@/types/api';

export interface MentionResource {
  id: string;
  name: string;
  type: 'vm' | 'container' | 'node' | 'storage' | 'docker' | 'host';
  status?: string;
  node?: string;
}

type MentionStateSubset = Pick<State, 'vms' | 'containers' | 'dockerHosts' | 'nodes' | 'hosts'>;

function normalizeKeyPart(value: string | undefined | null): string {
  return (value || '').trim().toLowerCase();
}

function mentionStatusRank(status?: string): number {
  switch (normalizeKeyPart(status)) {
    case 'online':
    case 'running':
    case 'healthy':
    case 'up':
      return 3
    case 'degraded':
    case 'warning':
    case 'warn':
    case 'paused':
      return 2
    case 'offline':
    case 'stopped':
    case 'error':
    case 'failed':
      return 1
    default:
      return 0
  }
}

function preferMentionResource(existing: MentionResource, candidate: MentionResource): MentionResource {
  const existingRank = mentionStatusRank(existing.status);
  const candidateRank = mentionStatusRank(candidate.status);

  if (candidateRank > existingRank) {
    return {
      ...candidate,
      id: existing.id || candidate.id,
    };
  }
  if (candidateRank < existingRank) {
    return existing;
  }

  if (!existing.node && candidate.node) {
    return {
      ...existing,
      node: candidate.node,
    };
  }
  if (!existing.status && candidate.status) {
    return {
      ...existing,
      status: candidate.status,
    };
  }

  return existing;
}

function upsertMentionResource(
  byKey: Map<string, MentionResource>,
  aliasToKey: Map<string, string>,
  resource: MentionResource,
  aliases: string[] = [],
): void {
  const normalizedPrimary = normalizeKeyPart(resource.id);
  const primaryAlias = normalizedPrimary ? `id:${normalizedPrimary}` : '';

  // Collect ALL existing keys that any alias points to (union-find style).
  // This fixes the 3-way chain bug where a host agent bridges a VM and a
  // DockerHost but only the first match was merged (#1252).
  const matchedKeys: string[] = [];
  let firstUnmatchedAlias = '';

  const allAliases = [primaryAlias, ...aliases];
  for (const rawAlias of allAliases) {
    const alias = normalizeKeyPart(rawAlias);
    if (!alias) continue;
    const existingKey = aliasToKey.get(alias);
    if (existingKey) {
      if (!matchedKeys.includes(existingKey)) {
        matchedKeys.push(existingKey);
      }
    } else if (!firstUnmatchedAlias) {
      firstUnmatchedAlias = alias;
    }
  }

  // Pick the canonical key: first matched key, or first unmatched alias, or fallback.
  const canonicalKey =
    matchedKeys[0] ||
    firstUnmatchedAlias ||
    `fallback:${normalizeKeyPart(resource.type)}:${normalizeKeyPart(resource.name)}`;

  // Merge all other matched keys' resources into the canonical winner.
  for (let i = 1; i < matchedKeys.length; i++) {
    const loserKey = matchedKeys[i];
    const loserResource = byKey.get(loserKey);
    if (loserResource) {
      const current = byKey.get(canonicalKey);
      byKey.set(canonicalKey, current ? preferMentionResource(current, loserResource) : loserResource);
      byKey.delete(loserKey);
    }
    // Re-point all aliases that referenced the loser to the winner.
    for (const [alias, key] of aliasToKey) {
      if (key === loserKey) {
        aliasToKey.set(alias, canonicalKey);
      }
    }
  }

  // Merge the incoming resource.
  const existing = byKey.get(canonicalKey);
  if (existing) {
    byKey.set(canonicalKey, preferMentionResource(existing, resource));
  } else {
    byKey.set(canonicalKey, resource);
  }

  // Register all aliases to point at the canonical key.
  for (const rawAlias of allAliases) {
    const alias = normalizeKeyPart(rawAlias);
    if (!alias) continue;
    aliasToKey.set(alias, canonicalKey);
  }
}

function nodeAlias(instance: string, clusterName: string | undefined, nodeName: string): string {
  const normalizedName = normalizeKeyPart(nodeName);
  if (!normalizedName) {
    return '';
  }
  const normalizedCluster = normalizeKeyPart(clusterName);
  if (normalizedCluster) {
    return `node-cluster:${normalizedCluster}:${normalizedName}`;
  }
  const normalizedInstance = normalizeKeyPart(instance);
  if (normalizedInstance) {
    return `node-instance:${normalizedInstance}:${normalizedName}`;
  }
  return `node-name:${normalizedName}`;
}

export function buildMentionResources(state: MentionStateSubset): MentionResource[] {
  const byKey = new Map<string, MentionResource>();
  const aliasToKey = new Map<string, string>();

  for (const vm of state.vms || []) {
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `vm:${vm.node}:${vm.vmid}`,
        name: vm.name,
        type: 'vm',
        status: vm.status,
        node: vm.node,
      },
      [
        `vm-id:${vm.node}:${vm.vmid}`,
        // Register backend VM ID so host agents with linkedVmId can merge (#1252)
        `vm-backend-id:${vm.id}`,
      ],
    );
  }

  for (const container of state.containers || []) {
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `lxc:${container.node}:${container.vmid}`,
        name: container.name,
        type: 'container',
        status: container.status,
        node: container.node,
      },
      [
        `lxc-id:${container.node}:${container.vmid}`,
        // Register backend container ID so host agents with linkedContainerId can merge (#1252)
        `lxc-backend-id:${container.id}`,
      ],
    );
  }

  for (const host of state.dockerHosts || []) {
    const hostName = host.displayName || host.hostname || host.id;
    const dockerAliases = [
      `docker-host-id:${host.id}`,
      `host-name:${hostName}`,
      `host-hostname:${host.hostname}`,
    ];
    // Link docker host to its host agent via agentId so they merge (#1252)
    if (host.agentId) {
      dockerAliases.push(`agent-host-id:${host.agentId}`);
    }
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `host:${host.id}`,
        name: hostName,
        type: 'host',
        status: host.status || 'online',
      },
      dockerAliases,
    );

    for (const container of host.containers || []) {
      upsertMentionResource(
        byKey,
        aliasToKey,
        {
          id: `docker:${host.id}:${container.id}`,
          name: container.name,
          type: 'docker',
          status: container.state,
          node: host.hostname || host.id,
        },
        [`docker-container-id:${host.id}:${container.id}`],
      );
    }
  }

  for (const node of state.nodes || []) {
    const aliases = [
      `node-id:${node.instance}:${node.name}`,
      nodeAlias(node.instance, node.clusterName, node.name),
      // Register backend node ID so host agents with linkedNodeId can merge (#1252)
      `node-backend-id:${node.id}`,
    ];
    // If this node has a linked host agent, add the agent's alias so they merge
    // instead of appearing as separate entries (#1252)
    if (node.linkedHostAgentId) {
      aliases.push(`agent-host-id:${node.linkedHostAgentId}`);
    }
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `node:${node.instance}:${node.name}`,
        name: node.name,
        type: 'node',
        status: node.status,
      },
      aliases,
    );
  }

  for (const host of state.hosts || []) {
    const hostName = host.displayName || host.hostname || host.id;
    const aliases = [
      `agent-host-id:${host.id}`,
      `host-name:${hostName}`,
      `host-hostname:${host.hostname}`,
    ];
    // If this host agent is linked to a PVE entity, add its backend ID alias so they merge (#1252).
    if (host.linkedNodeId) {
      aliases.push(`node-backend-id:${host.linkedNodeId}`);
    }
    if (host.linkedVmId) {
      aliases.push(`vm-backend-id:${host.linkedVmId}`);
    }
    if (host.linkedContainerId) {
      aliases.push(`lxc-backend-id:${host.linkedContainerId}`);
    }
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `host:${host.id}`,
        name: hostName,
        type: 'host',
        status: host.status || 'online',
      },
      aliases,
    );
  }

  return Array.from(byKey.values());
}
