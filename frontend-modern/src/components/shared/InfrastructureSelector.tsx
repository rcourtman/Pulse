import {
  Component,
  Show,
  createSignal,
  createEffect,
  createMemo,
  onMount,
  onCleanup,
} from 'solid-js';
import { unwrap } from 'solid-js/store';
import { InfrastructureSummaryTable } from './InfrastructureSummaryTable';
import { useResources } from '@/hooks/useResources';
import type { Agent, Node } from '@/types/api';
import type { Resource } from '@/types/resource';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import { nodeFromResource, pbsInstanceFromResource } from '@/utils/resourceStateAdapters';
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import {
  getActionableAgentIdFromResource,
  hasAgentFacet as resourceHasAgentFacet,
} from '@/utils/agentResources';

interface InfrastructureSelectorProps {
  currentTab: 'dashboard' | 'storage' | 'recovery';
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeSelect?: (nodeId: string | null, nodeType: 'pve' | 'pbs' | null) => void;
  onNamespaceSelect?: (namespace: string) => void;
  nodes?: Node[];
  searchTerm?: string;
  showNodeSummary?: boolean;
}

export const InfrastructureSelector: Component<InfrastructureSelectorProps> = (props) => {
  const { byType, resources } = useResources();
  const recovery = useRecoveryRollups();
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);
  const pd = (r: Resource) =>
    r.platformData ? (unwrap(r.platformData) as Record<string, unknown>) : undefined;
  const asRecord = (value: unknown): Record<string, unknown> | undefined =>
    value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
  const asString = (value: unknown): string | undefined =>
    typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
  const asNumber = (value: unknown): number | undefined =>
    typeof value === 'number' && Number.isFinite(value) ? value : undefined;
  const asBoolean = (value: unknown): boolean | undefined =>
    typeof value === 'boolean' ? value : undefined;
  const hasAgentFacet = (resource: Resource): boolean => resourceHasAgentFacet(resource);

  // Handle ESC key to deselect node
  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Escape' && selectedNode()) {
      setSelectedNode(null);
      props.onNodeSelect?.(null, null);
    }
  };

  onMount(() => {
    document.addEventListener('keydown', handleKeyDown);
  });

  onCleanup(() => {
    document.removeEventListener('keydown', handleKeyDown);
  });

  // Reset selection when tab changes
  createEffect(() => {
    props.currentTab;
    setSelectedNode(null);
  });

  // Compute per-node VM counts from unified resources
  const vmCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    byType('vm').forEach((r) => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Compute per-node container counts (LXC + OCI containers)
  const containerCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    const containers = [...byType('system-container'), ...byType('oci-container')];
    containers.forEach((r) => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Compute per-node storage counts from unified resources
  const storageCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    byType('storage').forEach((r) => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Physical disks now come from unified resources
  const diskCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    byType('physical_disk').forEach((disk) => {
      if (disk.parentId) {
        counts[disk.parentId] = (counts[disk.parentId] || 0) + 1;
      }
    });
    return counts;
  });

  const agentsForNodeSummary = createMemo<Agent[]>(() => {
    const agentFacetResources = resources().filter(
      (resource) =>
        (resource.type === 'agent' ||
          resource.type === 'pbs' ||
          resource.type === 'pmg' ||
          resource.type === 'truenas') &&
        hasAgentFacet(resource),
    );

    const agentsById = new Map<string, Agent>();
    for (const resource of agentFacetResources) {
      const platformData = pd(resource);
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
          const d = asRecord(disk);
          if (!d) return null;
          const total = asNumber(d.total) ?? 0;
          const used = asNumber(d.used) ?? 0;
          const free = asNumber(d.free) ?? Math.max(0, total - used);
          const usage = total > 0 ? (used / total) * 100 : 0;
          return {
            total,
            used,
            free,
            usage,
            mountpoint: asString(d.mountpoint),
            type: asString(d.type),
            device: asString(d.device),
          };
        })
        .filter((disk): disk is NonNullable<typeof disk> => Boolean(disk));

      const hostId =
        getActionableAgentIdFromResource(resource) || resource.id;
      const hostname = getPreferredResourceHostname(resource) || asString(agent.hostname) || hostId;

      if (agentsById.has(hostId)) continue;
      agentsById.set(hostId, {
        id: hostId,
        hostname,
        displayName: getPreferredResourceDisplayName(resource),
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
  });

  const unifiedNodes = createMemo<Node[]>(() =>
    resources()
      .filter((resource) => resource.type === 'agent')
      .map(nodeFromResource)
      .filter((node): node is Node => Boolean(node)),
  );

  const pbsInstances = createMemo(() =>
    resources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance)),
  );

  // Calculate rollup counts for PVE nodes (best-effort based on subjectRef).
  // PBS-per-instance counts require repository attribution (not currently exposed in rollups).
  const backupCounts = createMemo(() => {
    const counts: Record<string, number> = {};

    const nodes = props.nodes || unifiedNodes();
    if (nodes) {
      nodes.forEach((node) => {
        counts[node.id] = 0;
      });
    }

    const rollups = recovery.rollups() || [];
    for (const rollup of rollups) {
      const ref = rollup.subjectRef || null;
      if (!ref?.namespace || !ref?.class) continue;

      const node = (nodes || []).find((n) => n.instance === ref.namespace && n.name === ref.class);
      if (!node) continue;
      counts[node.id] = (counts[node.id] || 0) + 1;
    }

    return counts;
  });

  const handleNodeClick = (nodeId: string, nodeType: 'pve' | 'pbs') => {
    // Toggle selection
    if (selectedNode() === nodeId) {
      setSelectedNode(null);
      props.onNodeSelect?.(null, null);
    } else {
      setSelectedNode(nodeId);
      props.onNodeSelect?.(nodeId, nodeType);
    }
  };

  // Parent components now handle conditional rendering, so we can render directly
  const nodes = createMemo(() => props.nodes || unifiedNodes() || []);
  const showNodeSummary = () => props.showNodeSummary ?? true;

  return (
    <Show when={showNodeSummary()}>
      <div class="space-y-2 mb-4">
        <InfrastructureSummaryTable
          nodes={nodes()}
          pbsInstances={props.currentTab === 'recovery' ? pbsInstances() : undefined}
          vmCounts={vmCounts()}
          containerCounts={containerCounts()}
          storageCounts={storageCounts()}
          diskCounts={diskCounts()}
          agents={agentsForNodeSummary()}
          backupCounts={backupCounts()}
          currentTab={props.currentTab}
          selectedNode={selectedNode()}
          globalTemperatureMonitoringEnabled={props.globalTemperatureMonitoringEnabled}
          onNodeClick={handleNodeClick}
        />
      </div>
    </Show>
  );
};
