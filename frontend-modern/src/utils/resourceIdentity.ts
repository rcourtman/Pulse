import type { Agent, Node } from '@/types/api';
import type { Resource, ResourceCanonicalIdentity } from '@/types/resource';
import type { NodeConfig } from '@/types/nodes';
import {
  getActionableAgentIdFromResource,
  getActionableDockerRuntimeIdFromResource,
  getActionableKubernetesClusterIdFromResource,
  getPlatformAgentRecord,
  getPlatformDataRecord,
} from '@/utils/agentResources';

export type ResourceIdentityRow = {
  label: string;
  value: string;
};

type APINormalizedIdentityResource = {
  id: string;
  name?: string;
  proxmox?: {
    nodeName?: string;
  };
  agent?: {
    hostname?: string;
  };
  docker?: {
    hostname?: string;
  };
};

type NamedEntity = {
  id: string;
  displayName?: string;
  hostname?: string;
  name?: string;
};

const asTrimmedString = (value: unknown): string | undefined => {
  if (typeof value !== 'string') return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
};

const dedupeTrimmedValues = (values: Array<string | undefined>): string[] => {
  const seen = new Set<string>();
  const deduped: string[] = [];
  for (const value of values) {
    if (!value) continue;
    const trimmed = value.trim();
    if (!trimmed) continue;
    const normalized = trimmed.toLowerCase();
    if (seen.has(normalized)) continue;
    seen.add(normalized);
    deduped.push(trimmed);
  }

  return deduped;
};

const formatIdentityTarget = (resourceType?: string, resourceId?: string): string | null => {
  const type = asTrimmedString(resourceType);
  const id = asTrimmedString(resourceId);
  return type && id ? `${type}:${id}` : null;
};

const getCanonicalIdentityRecord = (resource: Resource): ResourceCanonicalIdentity | undefined => {
  return resource.canonicalIdentity;
};

export const getPrimaryResourceIdentity = (resource: Resource): string => {
  const canonical = getCanonicalIdentityRecord(resource);
  const canonicalPrimaryId = asTrimmedString(canonical?.primaryId);
  if (canonicalPrimaryId) return canonicalPrimaryId;

  const metricsIdentity = formatIdentityTarget(
    resource.metricsTarget?.resourceType,
    resource.metricsTarget?.resourceId,
  );
  if (metricsIdentity) return metricsIdentity;

  const discoveryIdentity = formatIdentityTarget(
    resource.discoveryTarget?.resourceType,
    resource.discoveryTarget?.resourceId,
  );
  if (discoveryIdentity) return discoveryIdentity;

  const dockerRuntimeId = getActionableDockerRuntimeIdFromResource(resource);
  if (resource.type === 'docker-host' && dockerRuntimeId) {
    return `docker-host:${dockerRuntimeId}`;
  }

  const kubernetesId = getActionableKubernetesClusterIdFromResource(resource);
  if (
    kubernetesId &&
    (resource.type === 'k8s-cluster' || resource.type === 'k8s-node' || resource.type === 'pod')
  ) {
    return `k8s:${kubernetesId}`;
  }

  const agentId = getActionableAgentIdFromResource(resource);
  if (agentId) return `agent:${agentId}`;

  const platformData = getPlatformDataRecord(resource);
  const pbs = platformData?.pbs as Record<string, unknown> | undefined;
  const pmg = platformData?.pmg as Record<string, unknown> | undefined;
  const pbsInstanceId = asTrimmedString(pbs?.instanceId);
  if (pbsInstanceId) return `pbs:${pbsInstanceId}`;
  const pmgInstanceId = asTrimmedString(pmg?.instanceId);
  if (pmgInstanceId) return `pmg:${pmgInstanceId}`;

  return resource.id;
};

export const getResourceIdentityAliases = (resource: Resource): string[] => {
  const canonical = getCanonicalIdentityRecord(resource);
  const platformData = getPlatformDataRecord(resource);
  const platformAgent = getPlatformAgentRecord(resource);
  const proxmox = platformData?.proxmox as Record<string, unknown> | undefined;
  const pbs = platformData?.pbs as Record<string, unknown> | undefined;
  const pmg = platformData?.pmg as Record<string, unknown> | undefined;
  const canonicalAliases = Array.isArray(canonical?.aliases)
    ? canonical.aliases
        .map((alias) => asTrimmedString(alias))
        .filter((alias): alias is string => Boolean(alias))
    : [];

  const raw = [
    ...canonicalAliases,
    resource.metricsTarget?.resourceId,
    resource.discoveryTarget?.agentId,
    resource.discoveryTarget?.resourceId,
    getActionableDockerRuntimeIdFromResource(resource),
    getActionableKubernetesClusterIdFromResource(resource),
    getActionableAgentIdFromResource(resource),
    asTrimmedString(platformData?.linkedAgentId),
    asTrimmedString(proxmox?.nodeName),
    asTrimmedString(platformAgent?.agentId),
    asTrimmedString(platformAgent?.hostname),
    asTrimmedString(pbs?.instanceId),
    asTrimmedString(pbs?.hostname),
    asTrimmedString(pmg?.instanceId),
    asTrimmedString(pmg?.hostname),
    resource.identity?.hostname,
    resource.identity?.machineId,
  ];

  return dedupeTrimmedValues(raw);
};

export const getAgentLikeIdentityAliases = (agent: Agent): string[] => {
  const agentRecord = agent as unknown as Record<string, unknown>;
  const discoveryTarget = agentRecord.discoveryTarget as Record<string, unknown> | undefined;
  const platformData = agentRecord.platformData as Record<string, unknown> | undefined;
  const platformAgent = platformData?.agent as Record<string, unknown> | undefined;
  const canonical = agentRecord.canonicalIdentity as Record<string, unknown> | undefined;

  return dedupeTrimmedValues([
    asTrimmedString(discoveryTarget?.resourceId),
    asTrimmedString(discoveryTarget?.agentId),
    asTrimmedString(platformAgent?.agentId),
    asTrimmedString(platformData?.agentId),
    asTrimmedString(platformData?.linkedAgentId),
    asTrimmedString(agent.id),
    asTrimmedString(canonical?.hostname),
    asTrimmedString(agent.hostname),
    asTrimmedString(platformAgent?.hostname),
  ]);
};

export const getAgentLikeMetadataIds = (agent: Agent): string[] => {
  const agentRecord = agent as unknown as Record<string, unknown>;
  const discoveryTarget = agentRecord.discoveryTarget as Record<string, unknown> | undefined;
  const platformData = agentRecord.platformData as Record<string, unknown> | undefined;
  const platformAgent = platformData?.agent as Record<string, unknown> | undefined;

  return dedupeTrimmedValues([
    asTrimmedString(discoveryTarget?.resourceId),
    asTrimmedString(discoveryTarget?.agentId),
    asTrimmedString(platformAgent?.agentId),
    asTrimmedString(platformData?.agentId),
    asTrimmedString(platformData?.linkedAgentId),
    asTrimmedString(agent.id),
  ]);
};

const getAgentLikeDiscoveryHostname = (agent?: Agent): string | undefined => {
  if (!agent) return undefined;
  const agentRecord = agent as unknown as Record<string, unknown>;
  const canonical = agentRecord.canonicalIdentity as Record<string, unknown> | undefined;
  const discoveryTarget = agentRecord.discoveryTarget as Record<string, unknown> | undefined;
  const platformData = agentRecord.platformData as Record<string, unknown> | undefined;
  const platformAgent = platformData?.agent as Record<string, unknown> | undefined;

  return (
    asTrimmedString(canonical?.hostname) ||
    asTrimmedString(discoveryTarget?.hostname) ||
    asTrimmedString(agent.hostname) ||
    asTrimmedString(platformAgent?.hostname)
  );
};

export const getInfrastructureMetadataId = (
  node: Pick<Node, 'id' | 'name' | 'linkedAgentId'>,
  agent?: Agent,
): string => {
  const agentMetadataId = agent ? getAgentLikeMetadataIds(agent)[0] : undefined;
  return agentMetadataId || asTrimmedString(node.linkedAgentId) || node.id || node.name;
};

export const getInfrastructureDiscoveryHostname = (
  node: Pick<Node, 'name'>,
  agent?: Pick<Agent, 'hostname'>,
): string => getAgentLikeDiscoveryHostname(agent as Agent | undefined) || node.name;

export const getPreferredConfiguredNodeLabel = (
  node: Pick<NodeConfig, 'displayName' | 'name' | 'host' | 'id'>,
): string =>
  asTrimmedString(node.displayName) ||
  asTrimmedString(node.name) ||
  asTrimmedString(node.host) ||
  node.id;

export const getPreferredNamedEntityLabel = (entity: NamedEntity): string =>
  asTrimmedString(entity.displayName) ||
  asTrimmedString(entity.hostname) ||
  asTrimmedString(entity.name) ||
  entity.id;

export const getPreferredNormalizedPlatformId = (resource: APINormalizedIdentityResource): string =>
  asTrimmedString(resource.proxmox?.nodeName) ||
  asTrimmedString(resource.agent?.hostname) ||
  asTrimmedString(resource.docker?.hostname) ||
  asTrimmedString(resource.name) ||
  resource.id;

export const getPrimaryResourceIdentityRows = (resource: Resource): ResourceIdentityRow[] => {
  const canonical = getCanonicalIdentityRecord(resource);
  const rows: ResourceIdentityRow[] = [];
  const hostname = asTrimmedString(canonical?.hostname) || resource.identity?.hostname;
  if (hostname) {
    rows.push({ label: 'Hostname', value: hostname });
  }
  if (resource.identity?.machineId) {
    rows.push({ label: 'Machine ID', value: resource.identity.machineId });
  }
  if (resource.clusterId) {
    rows.push({ label: 'Cluster', value: resource.clusterId });
  }
  if (resource.parentId) {
    rows.push({ label: 'Parent', value: resource.parentId });
  }

  const discoveryIdentity = formatIdentityTarget(
    resource.discoveryTarget?.resourceType,
    resource.discoveryTarget?.resourceId,
  );
  if (discoveryIdentity) {
    rows.push({ label: 'Discovery', value: discoveryIdentity });
  }

  const metricsIdentity = formatIdentityTarget(
    resource.metricsTarget?.resourceType,
    resource.metricsTarget?.resourceId,
  );
  if (metricsIdentity) {
    rows.push({ label: 'Metrics Target', value: metricsIdentity });
  }

  return rows;
};

export const getPreferredResourceHostname = (resource: Resource): string | undefined => {
  const canonical = getCanonicalIdentityRecord(resource);
  const platformData = getPlatformDataRecord(resource);
  const platformAgent = getPlatformAgentRecord(resource);
  const docker = platformData?.docker as Record<string, unknown> | undefined;
  const pbs = platformData?.pbs as Record<string, unknown> | undefined;
  const pmg = platformData?.pmg as Record<string, unknown> | undefined;

  return (
    asTrimmedString(canonical?.hostname) ||
    resource.identity?.hostname ||
    asTrimmedString(platformAgent?.hostname) ||
    asTrimmedString(docker?.hostname) ||
    asTrimmedString(pbs?.hostname) ||
    asTrimmedString(pmg?.hostname) ||
    asTrimmedString(resource.name) ||
    asTrimmedString(resource.platformId)
  );
};

export const getPreferredResourceDisplayName = (resource: Resource): string =>
  resource.displayName ||
  asTrimmedString(getCanonicalIdentityRecord(resource)?.displayName) ||
  getPreferredResourceHostname(resource) ||
  getPrimaryResourceIdentity(resource);

export const getPreferredWorkloadsAgentHint = (resource: Resource): string | undefined => {
  const platformData = getPlatformDataRecord(resource);
  const proxmox = platformData?.proxmox as Record<string, unknown> | undefined;
  const docker = platformData?.docker as Record<string, unknown> | undefined;

  if (resource.type === 'docker-host') {
    return (
      asTrimmedString(docker?.hostname) || getPreferredResourceHostname(resource) || resource.id
    );
  }

  if (resource.type === 'agent') {
    return (
      asTrimmedString(proxmox?.nodeName) || getPreferredResourceHostname(resource) || resource.id
    );
  }

  return undefined;
};
