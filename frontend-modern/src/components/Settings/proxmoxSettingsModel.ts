import type { Accessor, Setter } from 'solid-js';
import type { PBSInstance, PMGInstance } from '@/types/api';
import type { SecurityStatus as SecurityStatusInfo } from '@/types/config';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { Resource } from '@/types/resource';
import type {
  DiscoveredServer,
  DiscoveryScanStatus,
  NodeType,
} from './infrastructureSettingsModel';
import type { TrueNASSettingsPanelState } from './useTrueNASSettingsPanelState';
import type { VMwareSettingsPanelState } from './useVMwareSettingsPanelState';

export type DiscoveryMode = 'auto' | 'custom';

export interface PlatformConnectionsSummary {
  pveCount: number;
  pbsCount: number;
  pmgCount: number;
  truenasCount: number;
  truenasAvailable: boolean;
  vmwareCount: number;
  vmwareAvailable: boolean;
}

export interface InfrastructurePlatformSettingsProps {
  selectedAgent: Accessor<NodeType>;
  onSelectAgent: (agent: NodeType) => void;
  initialLoadComplete: Accessor<boolean>;
  discoveryEnabled: Accessor<boolean>;
  discoveryMode: Accessor<DiscoveryMode>;
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  discoveredNodes: Accessor<DiscoveredServer[]>;
  savingDiscoverySettings: Accessor<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  agentStateResources: Accessor<Resource[]>;
  pbsInstances: Accessor<PBSInstance[]>;
  pmgInstances: Accessor<PMGInstance[]>;
  pveNodes: Accessor<NodeConfigWithStatus[]>;
  pbsNodes: Accessor<NodeConfigWithStatus[]>;
  pmgNodes: Accessor<NodeConfigWithStatus[]>;
  trueNASSettings: TrueNASSettingsPanelState;
  vmwareSettings: VMwareSettingsPanelState;
  platformConnectionsSummary: Accessor<PlatformConnectionsSummary>;
  temperatureMonitoringEnabled: Accessor<boolean>;
  triggerDiscoveryScan: (options?: { quiet?: boolean }) => Promise<void>;
  loadDiscoveredNodes: () => Promise<void>;
  handleDiscoveryEnabledChange: (enabled: boolean) => Promise<boolean>;
  testNodeConnection: (nodeId: string) => void;
  requestDeleteNode: (node: NodeConfigWithStatus) => void;
  refreshClusterNodes: (nodeId: string) => Promise<void>;
  setShowNodeModal: Setter<boolean>;
  editingNode: Accessor<NodeConfigWithStatus | null>;
  setEditingNode: Setter<NodeConfigWithStatus | null>;
  setCurrentNodeType: Setter<NodeType>;
  modalResetKey: Accessor<number>;
  setModalResetKey: Setter<number>;
  isNodeModalVisible: (type: NodeType) => boolean;
  securityStatus: Accessor<SecurityStatusInfo | null>;
  resolveTemperatureMonitoringEnabled: (node?: NodeConfigWithStatus | null) => boolean;
  temperatureMonitoringLocked: Accessor<boolean>;
  savingTemperatureSetting: Accessor<boolean>;
  handleTemperatureMonitoringChange: (enabled: boolean) => Promise<void>;
  disableDockerUpdateActions: Accessor<boolean>;
  disableDockerUpdateActionsLocked: Accessor<boolean>;
  savingDockerUpdateActions: Accessor<boolean>;
  handleDisableDockerUpdateActionsChange: (enabled: boolean) => Promise<void>;
  handleNodeTemperatureMonitoringChange: (nodeId: string, enabled: boolean | null) => Promise<void>;
  saveNode: (nodeData: Partial<NodeConfig>) => Promise<void>;
  showDeleteNodeModal: Accessor<boolean>;
  cancelDeleteNode: () => void;
  deleteNode: () => Promise<void>;
  deleteNodeLoading: Accessor<boolean>;
  nodePendingDeleteLabel: () => string;
  nodePendingDeleteHost: () => string;
  nodePendingDeleteType: () => string;
  nodePendingDeleteTypeLabel: () => string;
  embedded?: boolean;
}
