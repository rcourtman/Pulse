import { createMemo, createSignal } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import {
  buildProxmoxDiscoveryPrefillNode,
  getProxmoxVariantPresentation,
} from '@/utils/proxmoxSettingsPresentation';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { ProxmoxSettingsPanelProps } from './proxmoxSettingsModel';
import type { DiscoveredServer, NodeType } from './useInfrastructureSettingsState';

export function useProxmoxDirectWorkspaceState(props: ProxmoxSettingsPanelProps) {
  const [prefillNode, setPrefillNode] = createSignal<Partial<NodeConfig> | null>(null);

  const activeAgent = () => props.selectedAgent();
  const activeConfig = createMemo(() => getProxmoxVariantPresentation(activeAgent()));
  const activeDiscoveredNodes = createMemo(() =>
    props.discoveredNodes().filter((node) => node.type === activeAgent()),
  );
  const activeConfiguredNodes = createMemo(() => {
    switch (activeAgent()) {
      case 'pve':
        return props.pveNodes();
      case 'pbs':
        return props.pbsNodes();
      case 'pmg':
        return props.pmgNodes();
    }
  });
  const hasDiscoveryTimeouts = () =>
    props.discoveryMode() === 'auto' &&
    (props.discoveryScanStatus().errors || []).some((error) => /timed out|timeout/i.test(error));

  const openCreateNode = (type: NodeType) => {
    setPrefillNode(null);
    props.setEditingNode(null);
    props.setCurrentNodeType(type);
    props.setModalResetKey((previous) => previous + 1);
    props.setShowNodeModal(true);
  };

  const openEditNode = (type: NodeType, node: NodeConfigWithStatus) => {
    setPrefillNode(null);
    props.setEditingNode(node);
    props.setCurrentNodeType(type);
    props.setShowNodeModal(true);
  };

  const openDiscoveredNode = (server: DiscoveredServer) => {
    setPrefillNode(buildProxmoxDiscoveryPrefillNode(server));
    props.setEditingNode(null);
    props.setCurrentNodeType(server.type);
    props.setModalResetKey((previous) => previous + 1);
    props.setShowNodeModal(true);
  };

  const closeNodeModal = () => {
    setPrefillNode(null);
    props.setShowNodeModal(false);
    props.setEditingNode(null);
    props.setModalResetKey((previous) => previous + 1);
  };

  const handleRefreshDiscovery = async () => {
    notificationStore.info('Refreshing discovery...', 2000);
    try {
      await props.triggerDiscoveryScan({ quiet: true });
    } finally {
      await props.loadDiscoveredNodes();
    }
  };

  const handleDiscoveryToggle = async (enabled: boolean) => {
    if (props.envOverrides().discoveryEnabled || props.savingDiscoverySettings()) {
      return props.discoveryEnabled();
    }

    const success = await props.handleDiscoveryEnabledChange(enabled);
    return success ? enabled : props.discoveryEnabled();
  };

  return {
    activeAgent,
    activeConfig,
    activeConfiguredNodes,
    activeDiscoveredNodes,
    closeNodeModal,
    handleDiscoveryToggle,
    handleRefreshDiscovery,
    hasDiscoveryTimeouts,
    openCreateNode,
    openDiscoveredNode,
    openEditNode,
    prefillNode,
  };
}
