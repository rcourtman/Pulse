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
import type { DiscoverySettingsFormProps } from './discoverySettingsModel';
import type { TrueNASSettingsPanelState } from './useTrueNASSettingsPanelState';
import type { VMwareSettingsPanelState } from './useVMwareSettingsPanelState';

export interface InfrastructurePlatformSettingsProps {
  selectedAgent: Accessor<NodeType>;
  onSelectAgent: (agent: NodeType) => void;
  initialLoadComplete: Accessor<boolean>;
  discoveryEnabled: DiscoverySettingsFormProps['discoveryEnabled'];
  discoveryMode: DiscoverySettingsFormProps['discoveryMode'];
  discoverySubnetDraft: DiscoverySettingsFormProps['discoverySubnetDraft'];
  discoverySubnetError: DiscoverySettingsFormProps['discoverySubnetError'];
  discoveryScanStatus: Accessor<DiscoveryScanStatus>;
  discoveredNodes: Accessor<DiscoveredServer[]>;
  savingDiscoverySettings: DiscoverySettingsFormProps['savingDiscoverySettings'];
  envOverrides: DiscoverySettingsFormProps['envOverrides'];
  agentStateResources: Accessor<Resource[]>;
  pbsInstances: Accessor<PBSInstance[]>;
  pmgInstances: Accessor<PMGInstance[]>;
  pveNodes: Accessor<NodeConfigWithStatus[]>;
  pbsNodes: Accessor<NodeConfigWithStatus[]>;
  pmgNodes: Accessor<NodeConfigWithStatus[]>;
  trueNASSettings: TrueNASSettingsPanelState;
  vmwareSettings: VMwareSettingsPanelState;
  temperatureMonitoringEnabled: Accessor<boolean>;
  triggerDiscoveryScan: (options?: { quiet?: boolean }) => Promise<void>;
  loadDiscoveredNodes: () => Promise<void>;
  handleDiscoveryEnabledChange: DiscoverySettingsFormProps['handleDiscoveryEnabledChange'];
  handleDiscoveryModeChange: DiscoverySettingsFormProps['handleDiscoveryModeChange'];
  setDiscoveryMode: DiscoverySettingsFormProps['setDiscoveryMode'];
  setDiscoverySubnetDraft: DiscoverySettingsFormProps['setDiscoverySubnetDraft'];
  setDiscoverySubnetError: DiscoverySettingsFormProps['setDiscoverySubnetError'];
  setLastCustomSubnet: DiscoverySettingsFormProps['setLastCustomSubnet'];
  commitDiscoverySubnet: DiscoverySettingsFormProps['commitDiscoverySubnet'];
  parseSubnetList: DiscoverySettingsFormProps['parseSubnetList'];
  normalizeSubnetList: DiscoverySettingsFormProps['normalizeSubnetList'];
  isValidCIDR: DiscoverySettingsFormProps['isValidCIDR'];
  currentDraftSubnetValue: DiscoverySettingsFormProps['currentDraftSubnetValue'];
  discoverySubnetInputRef?: DiscoverySettingsFormProps['discoverySubnetInputRef'];
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
  saveNode: (
    nodeData: Partial<NodeConfig>,
    existingNode?: NodeConfigWithStatus | null,
  ) => Promise<boolean>;
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
