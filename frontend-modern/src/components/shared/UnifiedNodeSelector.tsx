import { Component, Show, createSignal, createEffect, createMemo, onMount, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { NodeSummaryTable } from './NodeSummaryTable';
import { useResources } from '@/hooks/useResources';
import type { Node } from '@/types/api';

interface UnifiedNodeSelectorProps {
  currentTab: 'dashboard' | 'storage' | 'backups';
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeSelect?: (nodeId: string | null, nodeType: 'pve' | 'pbs' | null) => void;
  onNamespaceSelect?: (namespace: string) => void;
  nodes?: Node[];
  searchTerm?: string;
  showNodeSummary?: boolean;
}

export const UnifiedNodeSelector: Component<UnifiedNodeSelectorProps> = (props) => {
  const { state } = useWebSocket();
  const { byType } = useResources();
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);

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
    byType('vm').forEach(r => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Compute per-node container counts (LXC + OCI containers)
  const containerCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    const containers = [...byType('container'), ...byType('oci-container')];
    containers.forEach(r => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Compute per-node storage counts from unified resources
  const storageCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    byType('storage').forEach(r => {
      if (r.parentId) counts[r.parentId] = (counts[r.parentId] || 0) + 1;
    });
    return counts;
  });

  // Physical disks are NOT unified resources — compute from legacy state
  const diskCounts = createMemo(() => {
    const counts: Record<string, number> = {};
    const nodes = props.nodes || state.nodes || [];
    (state.physicalDisks ?? []).forEach((disk) => {
      const node = nodes.find(n => n.instance === disk.instance && n.name === disk.node);
      if (node) counts[node.id] = (counts[node.id] || 0) + 1;
    });
    return counts;
  });

  // Calculate backup counts for nodes and PBS instances
  const pveBackupsState = () => state.backups?.pve ?? state.pveBackups;
  const pbsBackupsState = () => state.backups?.pbs ?? state.pbsBackups;
  const backupCounts = createMemo(() => {
    const counts: Record<string, number> = {};

    // Count PVE backups and snapshots by node instance (to handle duplicate hostnames)
    const nodes = props.nodes || state.nodes;
    if (nodes) {
      nodes.forEach((node) => {
        let count = 0;

        // Count storage backups (excluding PBS backups which are counted separately)
        const pveBackups = pveBackupsState();
        if (pveBackups?.storageBackups) {
          count += pveBackups.storageBackups.filter(
            (b) => b.instance === node.instance && b.node === node.name && !b.isPBS,
          ).length;
        }

        // Count snapshots
        if (pveBackups?.guestSnapshots) {
          count += pveBackups.guestSnapshots.filter((s) => s.instance === node.instance && s.node === node.name).length;
        }

        counts[node.id] = count;
      });
    }

    // Count PBS backups by instance
    const pbsBackups = pbsBackupsState();
    if (state.pbs && pbsBackups) {
      state.pbs.forEach((pbs) => {
        counts[pbs.name] = pbsBackups.filter((b) => b.instance === pbs.name).length || 0;
      });
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
  const nodes = createMemo(() => props.nodes || state.nodes || []);
  const showNodeSummary = () => props.showNodeSummary ?? true;

  return (
    <Show when={showNodeSummary()}>
      <div class="space-y-2 mb-4">
        <NodeSummaryTable
          nodes={nodes()}
          pbsInstances={props.currentTab === 'backups' ? state.pbs : undefined}
          vmCounts={vmCounts()}
          containerCounts={containerCounts()}
          storageCounts={storageCounts()}
          diskCounts={diskCounts()}
          hosts={state.hosts}
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
