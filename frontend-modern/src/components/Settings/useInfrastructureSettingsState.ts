import { Accessor, Setter, createEffect, createSignal, onCleanup, onMount } from 'solid-js';
import { logger } from '@/utils/logger';
import type { EventDataMap, EventType } from '@/stores/events';
import type { SettingsTab } from './settingsTypes';
import { useInfrastructureConfiguredNodesState } from './useInfrastructureConfiguredNodesState';
import { useInfrastructureDiscoveryRuntimeState } from './useInfrastructureDiscoveryRuntimeState';

export type {
  DiscoveryScanStatus,
  DiscoveredServer,
  NodeType,
} from './infrastructureSettingsModel';

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
  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);

  const configuredNodes = useInfrastructureConfiguredNodesState({
    temperatureMonitoringEnabled,
    savingTemperatureSetting,
    setSavingTemperatureSetting,
  });

  const discoveryRuntime = useInfrastructureDiscoveryRuntimeState({
    eventBus,
    currentTab,
    nodes: configuredNodes.nodes,
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
  });

  onMount(() => {
    const unsubscribeAutoRegister = eventBus.on('node_auto_registered', () => {
      configuredNodes.setShowNodeModal(false);
      configuredNodes.setEditingNode(null);
      void configuredNodes.loadNodes();
      void discoveryRuntime.loadDiscoveredNodes();
    });

    const unsubscribeRefresh = eventBus.on('refresh_nodes', () => {
      void configuredNodes.loadNodes();
    });

    let pollInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = undefined;
      }

      if (configuredNodes.showNodeModal()) {
        pollInterval = setInterval(() => {
          void configuredNodes.loadNodes();
          void discoveryRuntime.loadDiscoveredNodes();
        }, 3000);
      }
    });

    onCleanup(() => {
      unsubscribeAutoRegister();
      unsubscribeRefresh();
      configuredNodes.clearLoadNodesRetry();
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    });
  });

  onMount(async () => {
    try {
      await loadSecurityStatus();
      await new Promise((resolve) => setTimeout(resolve, 50));
      await configuredNodes.loadNodes();
      await new Promise((resolve) => setTimeout(resolve, 50));
      await discoveryRuntime.loadDiscoveredNodes();
      await initializeSystemSettingsState();
    } catch (error) {
      logger.error('Failed to load configuration', error);
    } finally {
      setInitialLoadComplete(true);
    }
  });

  return {
    initialLoadComplete,
    ...configuredNodes,
    ...discoveryRuntime,
  };
}
