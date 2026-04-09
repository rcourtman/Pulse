import { Accessor, Setter, createEffect, createMemo, createSignal, on } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { NodesAPI } from '@/api/nodes';
import { useResources } from '@/hooks/useResources';
import { getPreferredConfiguredNodeLabel } from '@/utils/resourceIdentity';
import type { Temperature } from '@/types/api';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import { settingsAgentNodeLabel } from './settingsRouting';
import {
  matchConfiguredNodeToResource,
  type NodeType,
} from './infrastructureSettingsModel';
import {
  getNodeDeleteErrorMessage,
  getNodeTemperatureMonitoringUpdateErrorMessage,
} from '@/utils/infrastructureSettingsPresentation';

interface UseInfrastructureConfiguredNodesStateParams {
  temperatureMonitoringEnabled: Accessor<boolean>;
  savingTemperatureSetting: Accessor<boolean>;
  setSavingTemperatureSetting: Setter<boolean>;
}

export const useInfrastructureConfiguredNodesState = ({
  temperatureMonitoringEnabled,
  savingTemperatureSetting,
  setSavingTemperatureSetting,
}: UseInfrastructureConfiguredNodesStateParams) => {
  const { byType } = useResources();
  const [nodes, setNodes] = createSignal<NodeConfigWithStatus[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfigWithStatus | null>(null);
  const [currentNodeType, setCurrentNodeType] = createSignal<NodeType>('pve');
  const [modalResetKey, setModalResetKey] = createSignal(0);
  const [showDeleteNodeModal, setShowDeleteNodeModal] = createSignal(false);
  const [nodePendingDelete, setNodePendingDelete] = createSignal<NodeConfigWithStatus | null>(null);
  const [deleteNodeLoading, setDeleteNodeLoading] = createSignal(false);

  const isNodeModalVisible = (type: NodeType) =>
    Boolean(showNodeModal() && currentNodeType() === type);

  const pveNodes = createMemo(() => nodes().filter((node) => node.type === 'pve'));
  const pbsNodes = createMemo(() => nodes().filter((node) => node.type === 'pbs'));
  const pmgNodes = createMemo(() => nodes().filter((node) => node.type === 'pmg'));

  const orgNodeUsage = createMemo(
    () =>
      byType('agent').length +
      byType('docker-host').length +
      byType('k8s-cluster').length +
      byType('pbs').length +
      byType('pmg').length,
  );
  const orgGuestUsage = createMemo(
    () => byType('vm').length + byType('system-container').length + byType('oci-container').length,
  );

  let pendingLoadNodesRetry: ReturnType<typeof setTimeout> | undefined;
  let loadNodesRetryAttempts = 0;

  const clearLoadNodesRetry = () => {
    if (!pendingLoadNodesRetry) return;
    clearTimeout(pendingLoadNodesRetry);
    pendingLoadNodesRetry = undefined;
  };

  const loadNodes = async () => {
    try {
      clearLoadNodesRetry();
      const nodesList = await NodesAPI.getNodes();
      const nodeResources = byType('agent');
      const nodesWithStatus = nodesList.map((node) => {
        const stateNode = matchConfiguredNodeToResource(node, nodeResources);
        const tempValue = stateNode?.temperature;
        const temperature: Temperature | undefined =
          typeof tempValue === 'number' && tempValue > 0
            ? {
                cpuPackage: tempValue,
                cpuMax: tempValue,
                available: true,
                hasCPU: true,
                lastUpdate: new Date(stateNode!.lastSeen).toISOString(),
              }
            : node.temperature;

        return {
          ...node,
          hasPassword: node.hasPassword ?? !!node.password,
          hasToken: node.hasToken ?? !!node.tokenValue,
          status: node.status || ('pending' as const),
          temperature,
        };
      });

      loadNodesRetryAttempts = 0;
      setNodes(nodesWithStatus);
    } catch (error) {
      logger.error('Failed to load nodes', error);
      if (
        error instanceof Error &&
        (error.message.includes('429') || error.message.includes('fetch'))
      ) {
        if (pendingLoadNodesRetry) {
          return;
        }

        const delayMs = Math.min(3000 * Math.pow(2, Math.min(loadNodesRetryAttempts, 2)), 12000);
        loadNodesRetryAttempts++;
        logger.info('Retrying node load after delay', { delayMs, attempt: loadNodesRetryAttempts });
        pendingLoadNodesRetry = setTimeout(() => {
          pendingLoadNodesRetry = undefined;
          void loadNodes();
        }, delayMs);
      }
    }
  };

  const handleNodeTemperatureMonitoringChange = async (
    nodeId: string,
    enabled: boolean | null,
  ): Promise<void> => {
    if (savingTemperatureSetting()) {
      return;
    }

    const node = nodes().find((item) => item.id === nodeId);
    if (!node) {
      return;
    }

    const previous = node.temperatureMonitoringEnabled;
    setSavingTemperatureSetting(true);

    setNodes(
      nodes().map((item) =>
        item.id === nodeId ? { ...item, temperatureMonitoringEnabled: enabled } : item,
      ),
    );

    if (editingNode()?.id === nodeId) {
      setEditingNode({ ...editingNode()!, temperatureMonitoringEnabled: enabled });
    }

    try {
      await NodesAPI.updateNode(nodeId, { temperatureMonitoringEnabled: enabled } as NodeConfig);
      if (enabled === true) {
        notificationStore.success('Temperature monitoring enabled for this node', 2000);
      } else if (enabled === false) {
        notificationStore.info('Temperature monitoring disabled for this node', 2000);
      } else {
        notificationStore.info('Using global temperature monitoring setting', 2000);
      }
    } catch (error) {
      logger.error('Failed to update node temperature monitoring setting', error);
      notificationStore.error(
        getNodeTemperatureMonitoringUpdateErrorMessage(
          error instanceof Error ? error.message : undefined,
        ),
      );
      setNodes(
        nodes().map((item) =>
          item.id === nodeId ? { ...item, temperatureMonitoringEnabled: previous } : item,
        ),
      );
      if (editingNode()?.id === nodeId) {
        setEditingNode({ ...editingNode()!, temperatureMonitoringEnabled: previous });
      }
    } finally {
      setSavingTemperatureSetting(false);
    }
  };

  const resolveTemperatureMonitoringEnabled = (node?: NodeConfigWithStatus | null) => {
    if (node && typeof node.temperatureMonitoringEnabled === 'boolean') {
      return node.temperatureMonitoringEnabled;
    }
    return temperatureMonitoringEnabled();
  };

  const nodePendingDeleteLabel = () => {
    const node = nodePendingDelete();
    if (!node) return '';
    return getPreferredConfiguredNodeLabel(node);
  };

  const nodePendingDeleteHost = () => nodePendingDelete()?.host || '';
  const nodePendingDeleteType = () => nodePendingDelete()?.type || '';

  const nodePendingDeleteTypeLabel = () => {
    const type = nodePendingDeleteType();
    if (type === 'pve' || type === 'pbs' || type === 'pmg') {
      return settingsAgentNodeLabel(type);
    }
    return 'Pulse node';
  };

  const requestDeleteNode = (node: NodeConfigWithStatus) => {
    setNodePendingDelete(node);
    setShowDeleteNodeModal(true);
  };

  const cancelDeleteNode = () => {
    if (deleteNodeLoading()) return;
    setShowDeleteNodeModal(false);
    setNodePendingDelete(null);
  };

  const deleteNode = async () => {
    const pending = nodePendingDelete();
    if (!pending) return;

    setDeleteNodeLoading(true);
    try {
      await NodesAPI.deleteNode(pending.id);
      setNodes(nodes().filter((node) => node.id !== pending.id));
      notificationStore.success(`${getPreferredConfiguredNodeLabel(pending)} removed successfully`);
    } catch (error) {
      notificationStore.error(
        getNodeDeleteErrorMessage(error instanceof Error ? error.message : undefined),
      );
    } finally {
      setDeleteNodeLoading(false);
      setShowDeleteNodeModal(false);
      setNodePendingDelete(null);
    }
  };

  const testNodeConnection = async (nodeId: string) => {
    try {
      const node = nodes().find((item) => item.id === nodeId);
      if (!node) {
        throw new Error('Node not found');
      }

      const result = await NodesAPI.testExistingNode(nodeId);
      if (result.status === 'success') {
        if (result.warnings && Array.isArray(result.warnings) && result.warnings.length > 0) {
          const warningMessage =
            result.message +
            '\n\nWarnings:\n' +
            result.warnings.map((warning: string) => '• ' + warning).join('\n');
          notificationStore.warning(warningMessage);
        } else {
          notificationStore.success(result.message || 'Connection successful');
        }
        return;
      }

      throw new Error(result.message || 'Connection failed');
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Connection test failed');
    }
  };

  const refreshClusterNodes = async (nodeId: string) => {
    try {
      notificationStore.info('Refreshing cluster membership...', 2000);
      const result = await NodesAPI.refreshClusterNodes(nodeId);
      if (result.status === 'success') {
        if (result.nodesAdded && result.nodesAdded > 0) {
          notificationStore.success(
            `Found ${result.nodesAdded} new node(s) in cluster "${result.clusterName}"`,
          );
        } else {
          notificationStore.success(
            `Cluster "${result.clusterName}" membership verified (${result.newNodeCount} nodes)`,
          );
        }
        await loadNodes();
        return;
      }

      throw new Error('Failed to refresh cluster');
    } catch (error) {
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to refresh cluster membership',
      );
    }
  };

  const saveNode = async (nodeData: Partial<NodeConfig>) => {
    try {
      const existingNode = editingNode();
      if (existingNode?.id) {
        await NodesAPI.updateNode(existingNode.id, nodeData as NodeConfig);
        setNodes(
          nodes().map((node) =>
            node.id === existingNode.id
              ? {
                  ...node,
                  ...nodeData,
                  hasPassword: nodeData.password ? true : node.hasPassword,
                  hasToken: nodeData.tokenValue ? true : node.hasToken,
                  status: 'pending',
                }
              : node,
          ),
        );
        notificationStore.success('Node updated successfully');
      } else {
        await NodesAPI.addNode(nodeData as NodeConfig);
        await loadNodes();
        notificationStore.success('Node added successfully');
      }

      setShowNodeModal(false);
      setEditingNode(null);
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Operation failed');
    }
  };

  createEffect(
    on(
      () => byType('agent'),
      (nodeResources) => {
        const currentNodes = nodes();
        if (!nodeResources || nodeResources.length === 0 || currentNodes.length === 0) {
          return;
        }

        setNodes(
          currentNodes.map((node) => {
            const stateNode = matchConfiguredNodeToResource(node, nodeResources);
            const tempValue = stateNode?.temperature;
            if (typeof tempValue !== 'number' || tempValue <= 0) {
              return node;
            }

            const temperature: Temperature = {
              cpuPackage: tempValue,
              cpuMax: tempValue,
              available: true,
              hasCPU: true,
              lastUpdate: new Date(stateNode!.lastSeen).toISOString(),
            };
            return { ...node, temperature };
          }),
        );
      },
    ),
  );

  return {
    nodes,
    pveNodes,
    pbsNodes,
    pmgNodes,
    orgNodeUsage,
    orgGuestUsage,
    showNodeModal,
    setShowNodeModal,
    editingNode,
    setEditingNode,
    currentNodeType,
    setCurrentNodeType,
    modalResetKey,
    setModalResetKey,
    showDeleteNodeModal,
    deleteNodeLoading,
    isNodeModalVisible,
    resolveTemperatureMonitoringEnabled,
    loadNodes,
    clearLoadNodesRetry,
    handleNodeTemperatureMonitoringChange,
    requestDeleteNode,
    cancelDeleteNode,
    deleteNode,
    testNodeConnection,
    refreshClusterNodes,
    nodePendingDeleteLabel,
    nodePendingDeleteHost,
    nodePendingDeleteType,
    nodePendingDeleteTypeLabel,
    saveNode,
  };
};
