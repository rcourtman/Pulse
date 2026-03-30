import { unwrap } from 'solid-js/store';
import type { Agent, Node } from '@/types/api';
import type { Resource } from '@/types/resource';
import { nodeFromResource, pbsInstanceFromResource } from '@/utils/resourceStateAdapters';
import {
  getPreferredInfrastructureDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  getActionableAgentIdFromResource,
  isAgentFacetInfrastructureResource,
  hasAgentFacet as resourceHasAgentFacet,
} from '@/utils/agentResources';

export interface InfrastructureSelectorProps {
  currentTab: 'dashboard' | 'storage' | 'recovery';
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeSelect?: (nodeId: string | null, nodeType: 'pve' | 'pbs' | null) => void;
  onNamespaceSelect?: (namespace: string) => void;
  nodes?: Node[];
  searchTerm?: string;
  showNodeSummary?: boolean;
}

export interface InfrastructureSelectorRecoveryRollup {
  subjectRef?: {
    namespace?: string;
    class?: string;
  } | null;
}

export type InfrastructureSelectorPBSInstance = NonNullable<
  ReturnType<typeof pbsInstanceFromResource>
>;

const asRecord = (value: unknown): Record<string, unknown> | undefined =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;

const asString = (value: unknown): string | undefined =>
  typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;

const asNumber = (value: unknown): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const asBoolean = (value: unknown): boolean | undefined =>
  typeof value === 'boolean' ? value : undefined;

export const hasInfrastructureSelectorAgentFacet = (resource: Resource): boolean =>
  resourceHasAgentFacet(resource);

export function buildInfrastructureSelectorCounts(
  resources: Resource[],
  resourceTypes: Resource['type'] | Resource['type'][],
): Record<string, number> {
  const counts: Record<string, number> = {};
  const typeSet = new Set(Array.isArray(resourceTypes) ? resourceTypes : [resourceTypes]);

  resources.forEach((resource) => {
    if (!typeSet.has(resource.type) || !resource.parentId) return;
    counts[resource.parentId] = (counts[resource.parentId] || 0) + 1;
  });

  return counts;
}

export function buildInfrastructureSelectorAgents(resources: Resource[]): Agent[] {
  const agentFacetResources = resources.filter((resource) =>
    isAgentFacetInfrastructureResource(resource),
  );

  const agentsById = new Map<string, Agent>();
  for (const resource of agentFacetResources) {
    const platformData = resource.platformData
      ? (unwrap(resource.platformData) as Record<string, unknown>)
      : undefined;
    const agent = {
      ...(asRecord(platformData?.agent) || {}),
      ...(resource.agent || {}),
    } as Record<string, unknown>;
    const agentMemory = asRecord(agent.memory);
    const totalMemory = asNumber(agentMemory?.total) ?? resource.memory?.total ?? 0;
    const usedMemory = asNumber(agentMemory?.used) ?? resource.memory?.used ?? 0;
    const freeMemory =
      asNumber(agentMemory?.free) ??
      (totalMemory > 0 ? Math.max(0, totalMemory - usedMemory) : 0);
    const usageMemory =
      asNumber(agentMemory?.usage) ??
      (totalMemory > 0 ? (usedMemory / totalMemory) * 100 : (resource.memory?.current ?? 0));

    const rawDisks = Array.isArray(agent.disks) ? agent.disks : [];
    const disks = rawDisks
      .map((disk) => {
        const normalizedDisk = asRecord(disk);
        if (!normalizedDisk) return null;
        const total = asNumber(normalizedDisk.total) ?? 0;
        const used = asNumber(normalizedDisk.used) ?? 0;
        const free = asNumber(normalizedDisk.free) ?? Math.max(0, total - used);
        const usage = total > 0 ? (used / total) * 100 : 0;
        return {
          total,
          used,
          free,
          usage,
          mountpoint: asString(normalizedDisk.mountpoint),
          type: asString(normalizedDisk.type),
          device: asString(normalizedDisk.device),
        };
      })
      .filter((disk): disk is NonNullable<typeof disk> => Boolean(disk));

    const hostId = getActionableAgentIdFromResource(resource) || resource.id;
    const hostname = getPreferredResourceHostname(resource) || hostId;

    if (agentsById.has(hostId)) continue;

    agentsById.set(hostId, {
      id: hostId,
      hostname,
      displayName: getPreferredInfrastructureDisplayName(resource),
      platform: asString(agent.platform),
      osName: asString(agent.osName),
      osVersion: asString(agent.osVersion),
      kernelVersion: asString(agent.kernelVersion),
      architecture: asString(agent.architecture),
      cpuCount: asNumber(agent.cpuCount),
      memory: {
        total: totalMemory,
        used: usedMemory,
        free: freeMemory,
        usage: usageMemory,
        swapUsed: asNumber(agentMemory?.swapUsed),
        swapTotal: asNumber(agentMemory?.swapTotal),
      },
      disks,
      networkInterfaces: Array.isArray(agent.networkInterfaces)
        ? (agent.networkInterfaces as Agent['networkInterfaces'])
        : [],
      sensors: asRecord(agent.sensors) as Agent['sensors'],
      raid: Array.isArray(agent.raid) ? (agent.raid as Agent['raid']) : [],
      status: resource.status || 'unknown',
      uptimeSeconds: asNumber(agent.uptimeSeconds) ?? resource.uptime,
      lastSeen: resource.lastSeen,
      agentVersion: asString(agent.agentVersion),
      commandsEnabled: asBoolean(agent.commandsEnabled),
      tokenId: asString(agent.tokenId),
      tokenName: asString(agent.tokenName),
      tokenHint: asString(agent.tokenHint),
      tokenLastUsedAt: asNumber(agent.tokenLastUsedAt),
      tags: resource.tags,
      isLegacy: asBoolean(platformData?.isLegacy),
      linkedNodeId: asString(platformData?.linkedNodeId),
    });
  }

  return Array.from(agentsById.values());
}

export function buildInfrastructureSelectorUnifiedNodes(resources: Resource[]): Node[] {
  return resources
    .filter((resource) => resource.type === 'agent')
    .map(nodeFromResource)
    .filter((node): node is Node => Boolean(node));
}

export function buildInfrastructureSelectorPbsInstances(
  resources: Resource[],
): InfrastructureSelectorPBSInstance[] {
  return resources
    .filter((resource) => resource.type === 'pbs')
    .map(pbsInstanceFromResource)
    .filter((instance): instance is InfrastructureSelectorPBSInstance => Boolean(instance));
}

export function buildInfrastructureSelectorBackupCounts(options: {
  nodes: Node[];
  rollups: InfrastructureSelectorRecoveryRollup[];
}): Record<string, number> {
  const counts: Record<string, number> = {};

  options.nodes.forEach((node) => {
    counts[node.id] = 0;
  });

  for (const rollup of options.rollups) {
    const ref = rollup.subjectRef || null;
    if (!ref?.namespace || !ref?.class) continue;

    const node = options.nodes.find(
      (candidate) => candidate.instance === ref.namespace && candidate.name === ref.class,
    );
    if (!node) continue;

    counts[node.id] = (counts[node.id] || 0) + 1;
  }

  return counts;
}
