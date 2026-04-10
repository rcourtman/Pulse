import { createEffect, createMemo, createSignal, onCleanup } from 'solid-js';
import { useRecoveryRollups } from '@/hooks/useRecoveryRollups';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import {
  buildInfrastructureSelectorAgents,
  buildInfrastructureSelectorBackupCounts,
  buildInfrastructureSelectorCounts,
  buildInfrastructureSelectorPbsInstances,
  buildInfrastructureSelectorUnifiedNodes,
  type InfrastructureSelectorProps,
} from './infrastructureSelectorModel';

export type { InfrastructureSelectorProps } from './infrastructureSelectorModel';

export function useInfrastructureSelectorState(props: InfrastructureSelectorProps) {
  const showNodeSummary = createMemo(() => props.showNodeSummary ?? true);
  const unifiedResources = useUnifiedResources({
    query: '',
    cacheKey: 'all-resources',
    enabled: showNodeSummary,
  });
  const resources = createMemo(() => (showNodeSummary() ? unifiedResources.resources() : []));
  const recovery = useRecoveryRollups(() => (showNodeSummary() ? {} : null));
  const [selectedNode, setSelectedNode] = createSignal<string | null>(null);

  const unifiedNodes = createMemo(() => buildInfrastructureSelectorUnifiedNodes(resources()));
  const nodes = createMemo(() => props.nodes ?? unifiedNodes());
  const pbsInstances = createMemo(() => buildInfrastructureSelectorPbsInstances(resources()));

  const vmCounts = createMemo(() => buildInfrastructureSelectorCounts(resources(), 'vm'));
  const containerCounts = createMemo(() =>
    buildInfrastructureSelectorCounts(resources(), ['system-container', 'oci-container']),
  );
  const storageCounts = createMemo(() =>
    buildInfrastructureSelectorCounts(resources(), 'storage'),
  );
  const diskCounts = createMemo(() =>
    buildInfrastructureSelectorCounts(resources(), 'physical_disk'),
  );

  const agentsForNodeSummary = createMemo(() => buildInfrastructureSelectorAgents(resources()));
  const backupCounts = createMemo(() =>
    buildInfrastructureSelectorBackupCounts({
      nodes: nodes(),
      rollups: (recovery.rollups() || []) as Parameters<
        typeof buildInfrastructureSelectorBackupCounts
      >[0]['rollups'],
    }),
  );

  const handleNodeClick = (nodeId: string, nodeType: 'pve' | 'pbs') => {
    if (selectedNode() === nodeId) {
      setSelectedNode(null);
      props.onNodeSelect?.(null, null);
      return;
    }

    setSelectedNode(nodeId);
    props.onNodeSelect?.(nodeId, nodeType);
  };

  createEffect(() => {
    props.currentTab;
    setSelectedNode(null);
  });

  createEffect(() => {
    if (typeof document === 'undefined') return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !selectedNode()) return;
      setSelectedNode(null);
      props.onNodeSelect?.(null, null);
    };

    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => document.removeEventListener('keydown', handleKeyDown));
  });
  return {
    agentsForNodeSummary,
    backupCounts,
    containerCounts,
    diskCounts,
    handleNodeClick,
    nodes,
    pbsInstances,
    selectedNode,
    showNodeSummary: () => showNodeSummary(),
    storageCounts,
    vmCounts,
  };
}

export type InfrastructureSelectorState = ReturnType<typeof useInfrastructureSelectorState>;
