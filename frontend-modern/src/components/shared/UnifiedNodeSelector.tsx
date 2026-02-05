import { Component, Show, createSignal, createEffect, createMemo, onMount, onCleanup } from 'solid-js';
import { useWebSocket } from '@/App';
import { NodeSummaryTable } from './NodeSummaryTable';
import type { Node, VM, Container, Storage } from '@/types/api';

interface UnifiedNodeSelectorProps {
  currentTab: 'dashboard' | 'storage' | 'backups';
  globalTemperatureMonitoringEnabled?: boolean;
  onNodeSelect?: (nodeId: string | null, nodeType: 'pve' | 'pbs' | null) => void;
  onNamespaceSelect?: (namespace: string) => void;
  nodes?: Node[];
  filteredVms?: VM[];
  filteredContainers?: Container[];
  filteredStorage?: Storage[];
  searchTerm?: string;
  showNodeSummary?: boolean;
}

export const UnifiedNodeSelector: Component<UnifiedNodeSelectorProps> = (props) => {
  const { state } = useWebSocket();
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

  // No longer syncing with search term - selection is independent
  // This allows users to select a node AND search within it

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
          vms={state.vms} // Always use unfiltered data for counts
          containers={state.containers} // Always use unfiltered data for counts
          storage={state.storage} // Always use unfiltered data for counts
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
