import { Component, Show, createSignal, createEffect, createMemo, onMount, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { NodeSummaryTable } from './NodeSummaryTable';
import { useResources } from '@/hooks/useResources';
import type { Node } from '@/types/api';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';

interface UnifiedNodeSelectorProps {
  currentTab: 'dashboard' | 'storage' | 'recovery';
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
  const recovery = useRecoveryRollups();
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

  // Calculate rollup counts for PVE nodes (best-effort based on subjectRef).
  // PBS-per-instance counts require repository attribution (not currently exposed in rollups).
  const backupCounts = createMemo(() => {
    const counts: Record<string, number> = {};

    const nodes = props.nodes || state.nodes;
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
  const nodes = createMemo(() => props.nodes || state.nodes || []);
  const showNodeSummary = () => props.showNodeSummary ?? true;

  return (
    <Show when={showNodeSummary()}>
      <div class="space-y-2 mb-4">
        <NodeSummaryTable
          nodes={nodes()}
          pbsInstances={props.currentTab === 'recovery' ? state.pbs : undefined}
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
