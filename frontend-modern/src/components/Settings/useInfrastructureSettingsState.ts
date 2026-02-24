import {
  Accessor,
  Setter,
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
  onMount,
} from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { SettingsAPI } from '@/api/settings';
import { NodesAPI } from '@/api/nodes';
import { useResources } from '@/hooks/useResources';
import type { Resource } from '@/types/resource';
import type { Temperature } from '@/types/api';
import type { NodeConfig, NodeConfigWithStatus } from '@/types/nodes';
import type { EventDataMap, EventType } from '@/stores/events';
import type { SettingsTab } from './settingsTypes';

interface DiscoveredServer {
  ip: string;
  port: number;
  type: 'pve' | 'pbs' | 'pmg';
  version: string;
  hostname?: string;
  release?: string;
}

type RawDiscoveredServer = {
  ip?: string;
  port?: number;
  type?: string;
  version?: string;
  hostname?: string;
  name?: string;
  release?: string;
};

interface ClusterEndpoint {
  Host?: string;
  IP?: string;
}

interface DiscoveryScanStatus {
  scanning: boolean;
  subnet?: string;
  lastScanStartedAt?: number;
  lastResultAt?: number;
  errors?: string[];
}

type NodeType = 'pve' | 'pbs' | 'pmg';

type InfrastructureEventBus = {
  on<T extends EventType>(event: T, handler: (data?: EventDataMap[T]) => void): () => void;
};

interface UseInfrastructureSettingsStateParams {
  eventBus: InfrastructureEventBus;
  currentTab: Accessor<SettingsTab>;
  discoveryEnabled: Accessor<boolean>;
  setDiscoveryEnabled: Setter<boolean>;
  discoverySubnet: Accessor<string>;
  discoveryMode: Accessor<'auto' | 'custom'>;
  setDiscoveryMode: Setter<'auto' | 'custom'>;
  discoverySubnetDraft: Accessor<string>;
  setDiscoverySubnetDraft: Setter<string>;
  lastCustomSubnet: Accessor<string>;
  setLastCustomSubnet: Setter<string>;
  setDiscoverySubnetError: Setter<string | undefined>;
  savingDiscoverySettings: Accessor<boolean>;
  setSavingDiscoverySettings: Setter<boolean>;
  envOverrides: Accessor<Record<string, boolean>>;
  normalizeSubnetList: (value: string) => string;
  isValidCIDR: (value: string) => boolean;
  applySavedDiscoverySubnet: (subnet?: string | null) => void;
  getDiscoverySubnetInputRef?: () => HTMLInputElement | undefined;
  temperatureMonitoringEnabled: Accessor<boolean>;
  savingTemperatureSetting: Accessor<boolean>;
  setSavingTemperatureSetting: Setter<boolean>;
  loadSecurityStatus: () => Promise<void>;
  initializeSystemSettingsState: () => Promise<void>;
}

export function useInfrastructureSettingsState({
  eventBus,
  currentTab,
  discoveryEnabled,
  setDiscoveryEnabled,
  discoverySubnet,
  discoveryMode,
  setDiscoveryMode,
  discoverySubnetDraft,
  setDiscoverySubnetDraft,
  lastCustomSubnet,
  setLastCustomSubnet,
  setDiscoverySubnetError,
  savingDiscoverySettings,
  setSavingDiscoverySettings,
  envOverrides,
  normalizeSubnetList,
  isValidCIDR,
  applySavedDiscoverySubnet,
  getDiscoverySubnetInputRef,
  temperatureMonitoringEnabled,
  savingTemperatureSetting,
  setSavingTemperatureSetting,
  loadSecurityStatus,
  initializeSystemSettingsState,
}: UseInfrastructureSettingsStateParams) {
  const { byType } = useResources();
  const [nodes, setNodes] = createSignal<NodeConfigWithStatus[]>([]);
  const [discoveredNodes, setDiscoveredNodes] = createSignal<DiscoveredServer[]>([]);
  const [showNodeModal, setShowNodeModal] = createSignal(false);
  const [editingNode, setEditingNode] = createSignal<NodeConfigWithStatus | null>(null);
  const [currentNodeType, setCurrentNodeType] = createSignal<NodeType>('pve');
  const [modalResetKey, setModalResetKey] = createSignal(0);
  const [showDeleteNodeModal, setShowDeleteNodeModal] = createSignal(false);
  const [nodePendingDelete, setNodePendingDelete] = createSignal<NodeConfigWithStatus | null>(null);
  const [deleteNodeLoading, setDeleteNodeLoading] = createSignal(false);
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  const [discoveryScanStatus, setDiscoveryScanStatus] = createSignal<DiscoveryScanStatus>({
    scanning: false,
  });

  const isNodeModalVisible = (type: NodeType) =>
    Boolean(showNodeModal() && currentNodeType() === type);

  const pveNodes = createMemo(() => nodes().filter((n) => n.type === 'pve'));
  const pbsNodes = createMemo(() => nodes().filter((n) => n.type === 'pbs'));
  const pmgNodes = createMemo(() => nodes().filter((n) => n.type === 'pmg'));

  const orgNodeUsage = createMemo(
    () =>
      byType('node').length +
      byType('host').length +
      byType('docker-host').length +
      byType('k8s-cluster').length +
      byType('pbs').length +
      byType('pmg').length,
  );
  const orgGuestUsage = createMemo(
    () => byType('vm').length + byType('container').length + byType('oci-container').length,
  );

  const matchStateNode = (
    configNode: NodeConfigWithStatus | NodeConfig,
    nodeResources: Resource[] | undefined,
  ) => {
    if (!nodeResources || nodeResources.length === 0) {
      return undefined;
    }

    return nodeResources.find((n) => {
      if (n.id === configNode.id) return true;
      if (n.name === configNode.name) return true;
      const configNameBase = configNode.name.replace(/\.lan$/, '');
      const stateNameBase = n.name.replace(/\.lan$/, '');
      if (configNameBase === stateNameBase) return true;
      if (n.id.includes(configNode.name) || configNode.name.includes(n.name)) return true;
      return false;
    });
  };

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
      const nodeResources = byType('node');
      const nodesWithStatus = nodesList.map((node) => {
        const stateNode = matchStateNode(node, nodeResources);

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

  const updateDiscoveredNodesFromServers = (
    servers: RawDiscoveredServer[] | undefined | null,
    options: { merge?: boolean } = {},
  ) => {
    const { merge = false } = options;

    if (!servers || servers.length === 0) {
      if (!merge) {
        setDiscoveredNodes([]);
      }
      return;
    }

    const configuredHosts = new Set<string>();
    const clusterMemberIPs = new Set<string>();

    nodes().forEach((n) => {
      const cleanedHost = n.host.replace(/^https?:\/\//, '').replace(/:\d+$/, '');
      configuredHosts.add(cleanedHost.toLowerCase());

      if (
        n.type === 'pve' &&
        'isCluster' in n &&
        n.isCluster &&
        'clusterEndpoints' in n &&
        n.clusterEndpoints
      ) {
        n.clusterEndpoints.forEach((endpoint: ClusterEndpoint) => {
          if (endpoint.IP) {
            clusterMemberIPs.add(endpoint.IP.toLowerCase());
          }
          if (endpoint.Host) {
            clusterMemberIPs.add(endpoint.Host.toLowerCase());
          }
        });
      }
    });

    const recognizedTypes = ['pve', 'pbs', 'pmg'] as const;
    type RecognizedType = (typeof recognizedTypes)[number];
    const isRecognizedType = (value: string): value is RecognizedType =>
      (recognizedTypes as readonly string[]).includes(value);

    const normalized = servers
      .map((server): DiscoveredServer | null => {
        const ip = (server.ip || '').trim();
        let type = (server.type || '').toLowerCase();
        const hostname = (server.hostname || server.name || '').trim();
        const version = (server.version || '').trim();
        const release = (server.release || '').trim();

        if (!isRecognizedType(type)) {
          const metadata = `${hostname} ${version} ${release}`.toLowerCase();
          if (metadata.includes('pmg') || metadata.includes('mail gateway')) {
            type = 'pmg';
          } else if (metadata.includes('pbs') || metadata.includes('backup server')) {
            type = 'pbs';
          } else if (metadata.includes('pve') || metadata.includes('virtual environment')) {
            type = 'pve';
          }
        }

        if (!ip || !isRecognizedType(type)) {
          return null;
        }

        const port = typeof server.port === 'number' ? server.port : type === 'pbs' ? 8007 : 8006;

        return {
          ip,
          port,
          type,
          version: version || 'Unknown',
          hostname: hostname || undefined,
          release: release || undefined,
        };
      })
      .filter((server): server is DiscoveredServer => server !== null);

    const filtered = normalized.filter((server) => {
      const serverIP = server.ip.toLowerCase();
      const serverHostname = server.hostname?.toLowerCase();

      if (
        configuredHosts.has(serverIP) ||
        (serverHostname && configuredHosts.has(serverHostname))
      ) {
        return false;
      }

      if (
        clusterMemberIPs.has(serverIP) ||
        (serverHostname && clusterMemberIPs.has(serverHostname))
      ) {
        return false;
      }

      return true;
    });

    if (merge) {
      setDiscoveredNodes((prev) => {
        const existingMap = new Map(prev.map((item) => [`${item.ip}:${item.port}`, item]));
        filtered.forEach((server) => {
          existingMap.set(`${server.ip}:${server.port}`, server);
        });
        return Array.from(existingMap.values());
      });
    } else {
      setDiscoveredNodes(filtered);
    }

    setDiscoveryScanStatus((prev) => ({
      ...prev,
      lastResultAt: Date.now(),
    }));
  };

  const loadDiscoveredNodes = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover');
      if (response.ok) {
        const data = await response.json();
        if (Array.isArray(data.servers)) {
          updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[]);
          setDiscoveryScanStatus((prev) => ({
            ...prev,
            lastResultAt: typeof data.timestamp === 'number' ? data.timestamp : Date.now(),
            errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
          }));
        } else {
          updateDiscoveredNodesFromServers([]);
          setDiscoveryScanStatus((prev) => ({
            ...prev,
            lastResultAt: typeof data?.timestamp === 'number' ? data.timestamp : prev.lastResultAt,
            errors: Array.isArray(data?.errors) && data.errors.length > 0 ? data.errors : undefined,
          }));
        }
      }
    } catch (error) {
      logger.error('Failed to load discovered nodes', error);
    }
  };

  const triggerDiscoveryScan = async (options: { quiet?: boolean } = {}) => {
    const { quiet = false } = options;

    setDiscoveryScanStatus((prev) => ({
      ...prev,
      scanning: true,
      subnet: discoverySubnet() || prev.subnet,
      lastScanStartedAt: Date.now(),
      errors: undefined,
    }));

    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ subnet: discoverySubnet() || 'auto' }),
      });

      if (!response.ok) {
        const message = await response.text();
        throw new Error(message || 'Discovery request failed');
      }

      if (!quiet) {
        notificationStore.info('Discovery scan started', 2000);
      }
    } catch (error) {
      logger.error('Failed to start discovery scan', error);
      notificationStore.error('Failed to start discovery scan');
      setDiscoveryScanStatus((prev) => ({
        ...prev,
        scanning: false,
      }));
    }
  };

  const handleDiscoveryEnabledChange = async (enabled: boolean): Promise<boolean> => {
    if (envOverrides().discoveryEnabled || savingDiscoverySettings()) {
      return false;
    }

    const previousEnabled = discoveryEnabled();
    const previousSubnet = discoverySubnet();
    let subnetToSend = discoverySubnet();

    if (enabled) {
      if (discoveryMode() === 'custom') {
        const trimmedDraft = discoverySubnetDraft().trim();
        if (!trimmedDraft) {
          setDiscoverySubnetError('Enter at least one subnet before enabling discovery');
          notificationStore.error('Enter at least one subnet before enabling discovery');
          return false;
        }
        if (!isValidCIDR(trimmedDraft)) {
          setDiscoverySubnetError(
            'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
          );
          notificationStore.error('Enter valid CIDR subnet values before enabling discovery');
          return false;
        }
        const normalizedDraft = normalizeSubnetList(trimmedDraft);
        setDiscoverySubnetDraft(normalizedDraft);
        setDiscoverySubnetError(undefined);
        subnetToSend = normalizedDraft;
      } else {
        subnetToSend = 'auto';
        setDiscoverySubnetError(undefined);
      }
    }

    setDiscoveryEnabled(enabled);
    setSavingDiscoverySettings(true);

    try {
      await SettingsAPI.updateSystemSettings({
        discoveryEnabled: enabled,
        discoverySubnet: subnetToSend,
      });
      applySavedDiscoverySubnet(subnetToSend);
      if (enabled && subnetToSend !== 'auto') {
        setLastCustomSubnet(subnetToSend);
      }

      if (enabled) {
        await triggerDiscoveryScan({ quiet: true });
        notificationStore.success('Discovery enabled — scanning network...', 2000);
      } else {
        notificationStore.info('Discovery disabled', 2000);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
      }

      return true;
    } catch (error) {
      logger.error('Failed to update discovery setting', error);
      notificationStore.error('Failed to update discovery setting');
      setDiscoveryEnabled(previousEnabled);
      applySavedDiscoverySubnet(previousSubnet);
      return false;
    } finally {
      setSavingDiscoverySettings(false);
      await loadDiscoveredNodes();
    }
  };

  const commitDiscoverySubnet = async (rawValue: string): Promise<boolean> => {
    if (envOverrides().discoverySubnet) {
      return false;
    }

    const value = rawValue.trim();
    if (!value) {
      setDiscoverySubnetError('Enter at least one subnet in CIDR format (e.g., 192.168.1.0/24)');
      return false;
    }
    if (!isValidCIDR(value)) {
      setDiscoverySubnetError(
        'Use CIDR format such as 192.168.1.0/24 (comma-separated for multiple)',
      );
      return false;
    }

    const normalizedValue = normalizeSubnetList(value);
    if (!normalizedValue) {
      setDiscoverySubnetError('Enter at least one valid subnet in CIDR format');
      return false;
    }

    const previousSubnet = discoverySubnet();
    const previousNormalized =
      previousSubnet.toLowerCase() === 'auto' ? '' : normalizeSubnetList(previousSubnet);

    if (normalizedValue === previousNormalized) {
      setDiscoverySubnetDraft(normalizedValue);
      setDiscoverySubnetError(undefined);
      setLastCustomSubnet(normalizedValue);
      return true;
    }

    setSavingDiscoverySettings(true);

    try {
      setDiscoverySubnetError(undefined);
      await SettingsAPI.updateSystemSettings({
        discoveryEnabled: discoveryEnabled(),
        discoverySubnet: normalizedValue,
      });
      setLastCustomSubnet(normalizedValue);
      applySavedDiscoverySubnet(normalizedValue);
      if (discoveryEnabled()) {
        await triggerDiscoveryScan({ quiet: true });
        notificationStore.success('Discovery subnet updated — scanning network...', 2000);
      } else {
        notificationStore.success('Discovery subnet saved', 2000);
      }
      return true;
    } catch (error) {
      logger.error('Failed to update discovery subnet', error);
      notificationStore.error('Failed to update discovery subnet');
      applySavedDiscoverySubnet(previousSubnet);
      setDiscoverySubnetDraft(previousSubnet === 'auto' ? '' : normalizeSubnetList(previousSubnet));
      return false;
    } finally {
      setDiscoverySubnetError(undefined);
      setSavingDiscoverySettings(false);
      await loadDiscoveredNodes();
    }
  };

  const handleNodeTemperatureMonitoringChange = async (
    nodeId: string,
    enabled: boolean | null,
  ): Promise<void> => {
    if (savingTemperatureSetting()) {
      return;
    }

    const node = nodes().find((n) => n.id === nodeId);
    if (!node) {
      return;
    }

    const previous = node.temperatureMonitoringEnabled;
    setSavingTemperatureSetting(true);

    setNodes(
      nodes().map((n) => (n.id === nodeId ? { ...n, temperatureMonitoringEnabled: enabled } : n)),
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
        error instanceof Error ? error.message : 'Failed to update temperature monitoring setting',
      );
      setNodes(
        nodes().map((n) =>
          n.id === nodeId ? { ...n, temperatureMonitoringEnabled: previous } : n,
        ),
      );
      if (editingNode()?.id === nodeId) {
        setEditingNode({ ...editingNode()!, temperatureMonitoringEnabled: previous });
      }
    } finally {
      setSavingTemperatureSetting(false);
    }
  };

  const handleDiscoveryModeChange = async (mode: 'auto' | 'custom') => {
    if (envOverrides().discoverySubnet || savingDiscoverySettings()) {
      return;
    }
    if (mode === discoveryMode()) {
      return;
    }

    if (mode === 'auto') {
      const previousSubnet = discoverySubnet();
      setDiscoveryMode('auto');
      setDiscoverySubnetDraft('');
      setDiscoverySubnetError(undefined);
      setSavingDiscoverySettings(true);
      try {
        await SettingsAPI.updateSystemSettings({
          discoveryEnabled: discoveryEnabled(),
          discoverySubnet: 'auto',
        });
        applySavedDiscoverySubnet('auto');
        if (discoveryEnabled()) {
          await triggerDiscoveryScan({ quiet: true });
        }
        notificationStore.info(
          'Auto discovery scans each network phase. Large networks may take longer.',
          4000,
        );
      } catch (error) {
        logger.error('Failed to update discovery subnet', error);
        notificationStore.error('Failed to update discovery subnet');
        applySavedDiscoverySubnet(previousSubnet);
      } finally {
        setSavingDiscoverySettings(false);
        await loadDiscoveredNodes();
      }
      return;
    }

    setDiscoveryMode('custom');
    const rawDraft = discoverySubnet() !== 'auto' ? discoverySubnet() : lastCustomSubnet() || '';
    const normalizedDraft = normalizeSubnetList(rawDraft);
    setDiscoverySubnetDraft(normalizedDraft);
    setDiscoverySubnetError(undefined);
    queueMicrotask(() => {
      getDiscoverySubnetInputRef?.()?.focus();
      getDiscoverySubnetInputRef?.()?.select();
    });
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
    return node.displayName || node.name || node.host || node.id;
  };

  const nodePendingDeleteHost = () => nodePendingDelete()?.host || '';
  const nodePendingDeleteType = () => nodePendingDelete()?.type || '';

  const nodePendingDeleteTypeLabel = () => {
    switch (nodePendingDeleteType()) {
      case 'pve':
        return 'Proxmox VE node';
      case 'pbs':
        return 'Proxmox Backup Server';
      case 'pmg':
        return 'Proxmox Mail Gateway';
      default:
        return 'Pulse node';
    }
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
      setNodes(nodes().filter((n) => n.id !== pending.id));
      const label = pending.displayName || pending.name || pending.host || pending.id;
      notificationStore.success(`${label} removed successfully`);
    } catch (error) {
      notificationStore.error(error instanceof Error ? error.message : 'Failed to delete node');
    } finally {
      setDeleteNodeLoading(false);
      setShowDeleteNodeModal(false);
      setNodePendingDelete(null);
    }
  };

  const testNodeConnection = async (nodeId: string) => {
    try {
      const node = nodes().find((n) => n.id === nodeId);
      if (!node) {
        throw new Error('Node not found');
      }

      const result = await NodesAPI.testExistingNode(nodeId);
      if (result.status === 'success') {
        if (result.warnings && Array.isArray(result.warnings) && result.warnings.length > 0) {
          const warningMessage =
            result.message +
            '\n\nWarnings:\n' +
            result.warnings.map((w: string) => '• ' + w).join('\n');
          notificationStore.warning(warningMessage);
        } else {
          notificationStore.success(result.message || 'Connection successful');
        }
      } else {
        throw new Error(result.message || 'Connection failed');
      }
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
      } else {
        throw new Error('Failed to refresh cluster');
      }
    } catch (error) {
      notificationStore.error(
        error instanceof Error ? error.message : 'Failed to refresh cluster membership',
      );
    }
  };

  const saveNode = async (nodeData: Partial<NodeConfig>) => {
    try {
      const existingNode = editingNode();
      if (existingNode && existingNode.id) {
        await NodesAPI.updateNode(existingNode.id, nodeData as NodeConfig);

        setNodes(
          nodes().map((n) =>
            n.id === existingNode.id
              ? {
                  ...n,
                  ...nodeData,
                  hasPassword: nodeData.password ? true : n.hasPassword,
                  hasToken: nodeData.tokenValue ? true : n.hasToken,
                  status: 'pending',
                }
              : n,
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

  onMount(async () => {
    const unsubscribeAutoRegister = eventBus.on('node_auto_registered', () => {
      setShowNodeModal(false);
      setEditingNode(null);
      void loadNodes();
      void loadDiscoveredNodes();
    });

    const unsubscribeRefresh = eventBus.on('refresh_nodes', () => {
      void loadNodes();
    });

    const unsubscribeDiscovery = eventBus.on('discovery_updated', (data) => {
      if (!data) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
        return;
      }

      if (Array.isArray(data.servers)) {
        updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[], {
          merge: !!data.immediate,
        });
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? Date.now(),
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else if (!data.immediate) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          lastResultAt: data.timestamp ?? prev.lastResultAt,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else {
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: data.scanning ?? prev.scanning,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      }
    });

    const unsubscribeDiscoveryStatus = eventBus.on('discovery_status', (data) => {
      if (!data) {
        setDiscoveryScanStatus((prev) => ({
          ...prev,
          scanning: false,
        }));
        return;
      }

      setDiscoveryScanStatus((prev) => ({
        ...prev,
        scanning: !!data.scanning,
        subnet: data.subnet || prev.subnet,
        lastScanStartedAt: data.scanning ? (data.timestamp ?? Date.now()) : prev.lastScanStartedAt,
        lastResultAt: !data.scanning && data.timestamp ? data.timestamp : prev.lastResultAt,
      }));

      if (typeof data.subnet === 'string' && data.subnet !== discoverySubnet()) {
        applySavedDiscoverySubnet(data.subnet);
      }
    });

    let pollInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = undefined;
      }

      if (showNodeModal()) {
        pollInterval = setInterval(() => {
          void loadNodes();
          void loadDiscoveredNodes();
        }, 3000);
      }
    });

    let discoveryInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      if (discoveryInterval) {
        clearInterval(discoveryInterval);
        discoveryInterval = undefined;
      }
      if (currentTab() === 'proxmox') {
        discoveryInterval = setInterval(() => {
          void loadDiscoveredNodes();
        }, 30000);
      }
    });

    onCleanup(() => {
      unsubscribeAutoRegister();
      unsubscribeRefresh();
      unsubscribeDiscovery();
      unsubscribeDiscoveryStatus();
      clearLoadNodesRetry();
      if (pollInterval) {
        clearInterval(pollInterval);
      }
      if (discoveryInterval) {
        clearInterval(discoveryInterval);
      }
    });

    try {
      await loadSecurityStatus();
      await new Promise((resolve) => setTimeout(resolve, 50));
      await loadNodes();
      await new Promise((resolve) => setTimeout(resolve, 50));
      await loadDiscoveredNodes();
      await initializeSystemSettingsState();
    } catch (error) {
      logger.error('Failed to load configuration', error);
    } finally {
      setInitialLoadComplete(true);
    }
  });

  createEffect(
    on(
      () => byType('node'),
      (nodeResources) => {
        const currentNodes = nodes();
        if (nodeResources && nodeResources.length > 0 && currentNodes.length > 0) {
          const updatedNodes = currentNodes.map((node) => {
            const stateNode = matchStateNode(node, nodeResources);
            const tempValue = stateNode?.temperature;
            if (typeof tempValue === 'number' && tempValue > 0) {
              const temp: Temperature = {
                cpuPackage: tempValue,
                cpuMax: tempValue,
                available: true,
                hasCPU: true,
                lastUpdate: new Date(stateNode!.lastSeen).toISOString(),
              };
              return { ...node, temperature: temp };
            }
            return node;
          });
          setNodes(updatedNodes);
        }
      },
    ),
  );

  return {
    nodes,
    discoveredNodes,
    showNodeModal,
    setShowNodeModal,
    editingNode,
    setEditingNode,
    currentNodeType,
    setCurrentNodeType,
    modalResetKey,
    setModalResetKey,
    initialLoadComplete,
    discoveryScanStatus,
    showDeleteNodeModal,
    nodePendingDelete,
    deleteNodeLoading,
    pveNodes,
    pbsNodes,
    pmgNodes,
    orgNodeUsage,
    orgGuestUsage,
    isNodeModalVisible,
    resolveTemperatureMonitoringEnabled,
    loadNodes,
    updateDiscoveredNodesFromServers,
    loadDiscoveredNodes,
    triggerDiscoveryScan,
    handleDiscoveryEnabledChange,
    commitDiscoverySubnet,
    handleDiscoveryModeChange,
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
}
