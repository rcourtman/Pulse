import { Accessor, Setter, createEffect, createSignal, onCleanup, onMount } from 'solid-js';
import { logger } from '@/utils/logger';
import type { EventDataMap, EventType } from '@/stores/events';
import { useInfrastructureConfiguredNodesState } from './useInfrastructureConfiguredNodesState';
import { useInfrastructureDiscoveryRuntimeState } from './useInfrastructureDiscoveryRuntimeState';
import { useTrueNASSettingsPanelState } from './useTrueNASSettingsPanelState';
import { useVMwareSettingsPanelState } from './useVMwareSettingsPanelState';

type InfrastructureEventBus = {
  on<T extends EventType>(event: T, handler: (data?: EventDataMap[T]) => void): () => void;
};

interface UseInfrastructureSettingsStateParams {
  eventBus: InfrastructureEventBus;
  // Every endpoint this hook reads is RequireAdmin, but Settings.tsx mounts it
  // for every settings tab. Without the gate a non-admin session fires the
  // whole bootstrap (nodes, discovery, system settings, TrueNAS, VMware) on any
  // settings page and then keeps polling discovery.
  canReadInfrastructure: Accessor<boolean>;
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
  canReadInfrastructure,
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
    canReadInfrastructure,
    temperatureMonitoringEnabled,
    savingTemperatureSetting,
    setSavingTemperatureSetting,
  });
  const trueNASSettings = useTrueNASSettingsPanelState({ canLoad: canReadInfrastructure });
  const vmwareSettings = useVMwareSettingsPanelState({ canLoad: canReadInfrastructure });

  const discoveryRuntime = useInfrastructureDiscoveryRuntimeState({
    eventBus,
    nodes: configuredNodes.nodes,
    canReadInfrastructure,
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
      // loadSecurityStatus is what resolves canReadInfrastructure, so it always
      // runs; /api/security/status is readable by any session.
      await loadSecurityStatus();
      if (!canReadInfrastructure()) {
        // Sessions without infrastructure read still render the Settings
        // shell, and the system-settings state degrades to the viewer-safe
        // runtime display projection on its own (it handles the admin 403
        // internally). Skipping it here left viewers staring at defaults —
        // issue #1601's "Realtime (10s)" cadence misreport.
        await initializeSystemSettingsState();
        return;
      }
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
    trueNASSettings,
    vmwareSettings,
    ...configuredNodes,
    ...discoveryRuntime,
  };
}
