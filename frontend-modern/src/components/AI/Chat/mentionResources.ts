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
  let key = normalizedPrimary ? `id:${normalizedPrimary}` : '';

  for (const rawAlias of aliases) {
    const alias = normalizeKeyPart(rawAlias);
    if (!alias) continue;
    const existingKey = aliasToKey.get(alias);
    if (existingKey) {
      key = existingKey;
      break;
    }
    if (!key) {
      key = alias;
    }
  }

  if (!key) {
    key = `fallback:${normalizeKeyPart(resource.type)}:${normalizeKeyPart(resource.name)}`;
  }

  const existing = byKey.get(key);
  if (existing) {
    byKey.set(key, preferMentionResource(existing, resource));
  } else {
    byKey.set(key, resource);
  }

  const allAliases = [normalizedPrimary ? `id:${normalizedPrimary}` : '', ...aliases];
  for (const rawAlias of allAliases) {
    const alias = normalizeKeyPart(rawAlias);
    if (!alias) continue;
    aliasToKey.set(alias, key);
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
      [`vm-id:${vm.node}:${vm.vmid}`],
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
      [`lxc-id:${container.node}:${container.vmid}`],
    );
  }

  for (const host of state.dockerHosts || []) {
    const hostName = host.displayName || host.hostname || host.id;
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `host:${host.id}`,
        name: hostName,
        type: 'host',
        status: host.status || 'online',
      },
      [
        `docker-host-id:${host.id}`,
        `host-name:${hostName}`,
        `host-hostname:${host.hostname}`,
      ],
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
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `node:${node.instance}:${node.name}`,
        name: node.name,
        type: 'node',
        status: node.status,
      },
      [
        `node-id:${node.instance}:${node.name}`,
        nodeAlias(node.instance, node.clusterName, node.name),
        ...(node.linkedHostAgentId ? [`host-link:${node.linkedHostAgentId}`] : []),
        `node-backend-id:${node.id}`,
      ],
    );
  }

  for (const host of state.hosts || []) {
    const hostName = host.displayName || host.hostname || host.id;
    upsertMentionResource(
      byKey,
      aliasToKey,
      {
        id: `host:${host.id}`,
        name: hostName,
        type: 'host',
        status: host.status,
      },
      [
        `agent-host-id:${host.id}`,
        `host-name:${hostName}`,
        `host-hostname:${host.hostname}`,
        `host-link:${host.id}`,
        ...(host.linkedNodeId ? [`node-backend-id:${host.linkedNodeId}`] : []),
      ],
    );
  }

  return Array.from(byKey.values());
}
