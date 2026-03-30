import { createMemo, type Accessor } from 'solid-js';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import type { Resource } from '@/types/resource';
import { pbsInstanceFromResource, pmgInstanceFromResource } from '@/utils/resourceStateAdapters';
import type { NodeType } from './infrastructureSettingsModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import type { useDiscoverySettingsState } from './useDiscoverySettingsState';
import type { useInfrastructureSettingsState } from './useInfrastructureSettingsState';
import type { useSystemSettingsState } from './useSystemSettingsState';

interface UseSettingsInfrastructurePanelPropsParams {
  selectedAgent: Accessor<NodeType>;
  onSelectAgent: (agent: NodeType) => void;
  resources: Accessor<Resource[]>;
  discoverySettings: ReturnType<typeof useDiscoverySettingsState>;
  systemSettings: ReturnType<typeof useSystemSettingsState>;
  infrastructureSettings: ReturnType<typeof useInfrastructureSettingsState>;
  securityStatus: Accessor<SecurityStatusInfo | null>;
}

export function useSettingsInfrastructurePanelProps(
  params: UseSettingsInfrastructurePanelPropsParams,
) {
  const agentStateResources = createMemo(() =>
    params.resources().filter((resource) => resource.type === 'agent'),
  );
  const pbsInstances = createMemo(() =>
    params
      .resources()
      .filter((resource) => resource.type === 'pbs')
      .map(pbsInstanceFromResource)
      .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance)),
  );
  const pmgInstances = createMemo(() =>
    params
      .resources()
      .filter((resource) => resource.type === 'pmg')
      .map(pmgInstanceFromResource)
      .filter((instance): instance is NonNullable<typeof instance> => Boolean(instance)),
  );

  const platformConnectionsSummary = createMemo(() => ({
    pveCount: params.infrastructureSettings.pveNodes().length,
    pbsCount: params.infrastructureSettings.pbsNodes().length,
    pmgCount: params.infrastructureSettings.pmgNodes().length,
    truenasCount: params.infrastructureSettings.trueNASSettings.connections().length,
    truenasAvailable: !params.infrastructureSettings.trueNASSettings.featureDisabled(),
    vmwareCount: params.infrastructureSettings.vmwareSettings.connections().length,
    vmwareAvailable: !params.infrastructureSettings.vmwareSettings.featureDisabled(),
  }));

  const getInfrastructurePanelProps = (): InfrastructurePlatformSettingsProps => ({
    selectedAgent: params.selectedAgent,
    onSelectAgent: params.onSelectAgent,
    initialLoadComplete: params.infrastructureSettings.initialLoadComplete,
    discoveryEnabled: params.discoverySettings.discoveryEnabled,
    discoveryMode: params.discoverySettings.discoveryMode,
    discoveryScanStatus: params.infrastructureSettings.discoveryScanStatus,
    discoveredNodes: params.infrastructureSettings.discoveredNodes,
    savingDiscoverySettings: params.discoverySettings.savingDiscoverySettings,
    envOverrides: params.systemSettings.envOverrides,
    agentStateResources,
    pbsInstances,
    pmgInstances,
    pveNodes: params.infrastructureSettings.pveNodes,
    pbsNodes: params.infrastructureSettings.pbsNodes,
    pmgNodes: params.infrastructureSettings.pmgNodes,
    trueNASSettings: params.infrastructureSettings.trueNASSettings,
    vmwareSettings: params.infrastructureSettings.vmwareSettings,
    platformConnectionsSummary,
    temperatureMonitoringEnabled: params.systemSettings.temperatureMonitoringEnabled,
    triggerDiscoveryScan: params.infrastructureSettings.triggerDiscoveryScan,
    loadDiscoveredNodes: params.infrastructureSettings.loadDiscoveredNodes,
    handleDiscoveryEnabledChange: params.infrastructureSettings.handleDiscoveryEnabledChange,
    testNodeConnection: params.infrastructureSettings.testNodeConnection,
    requestDeleteNode: params.infrastructureSettings.requestDeleteNode,
    refreshClusterNodes: params.infrastructureSettings.refreshClusterNodes,
    setShowNodeModal: params.infrastructureSettings.setShowNodeModal,
    editingNode: params.infrastructureSettings.editingNode,
    setEditingNode: params.infrastructureSettings.setEditingNode,
    setCurrentNodeType: params.infrastructureSettings.setCurrentNodeType,
    modalResetKey: params.infrastructureSettings.modalResetKey,
    setModalResetKey: params.infrastructureSettings.setModalResetKey,
    isNodeModalVisible: params.infrastructureSettings.isNodeModalVisible,
    securityStatus: params.securityStatus,
    resolveTemperatureMonitoringEnabled:
      params.infrastructureSettings.resolveTemperatureMonitoringEnabled,
    temperatureMonitoringLocked: params.systemSettings.temperatureMonitoringLocked,
    savingTemperatureSetting: params.systemSettings.savingTemperatureSetting,
    handleTemperatureMonitoringChange: params.systemSettings.handleTemperatureMonitoringChange,
    handleNodeTemperatureMonitoringChange:
      params.infrastructureSettings.handleNodeTemperatureMonitoringChange,
    saveNode: params.infrastructureSettings.saveNode,
    showDeleteNodeModal: params.infrastructureSettings.showDeleteNodeModal,
    cancelDeleteNode: params.infrastructureSettings.cancelDeleteNode,
    deleteNode: params.infrastructureSettings.deleteNode,
    deleteNodeLoading: params.infrastructureSettings.deleteNodeLoading,
    nodePendingDeleteLabel: params.infrastructureSettings.nodePendingDeleteLabel,
    nodePendingDeleteHost: params.infrastructureSettings.nodePendingDeleteHost,
    nodePendingDeleteType: params.infrastructureSettings.nodePendingDeleteType,
    nodePendingDeleteTypeLabel: params.infrastructureSettings.nodePendingDeleteTypeLabel,
    disableDockerUpdateActions: params.systemSettings.disableDockerUpdateActions,
    disableDockerUpdateActionsLocked: params.systemSettings.disableDockerUpdateActionsLocked,
    savingDockerUpdateActions: params.systemSettings.savingDockerUpdateActions,
    handleDisableDockerUpdateActionsChange:
      params.systemSettings.handleDisableDockerUpdateActionsChange,
  });

  return {
    getInfrastructurePanelProps,
  };
}
