import { Accessor, Setter, createEffect, createSignal, onCleanup, onMount } from 'solid-js';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { SettingsAPI } from '@/api/settings';
import type { EventDataMap, EventType } from '@/stores/events';
import type { NodeConfigWithStatus } from '@/types/nodes';
import type { SettingsTab } from './settingsTypes';
import {
  collectConfiguredInfrastructureHosts,
  type DiscoveryScanStatus,
  type DiscoveredServer,
} from './infrastructureSettingsModel';
import {
  getDiscoveryScanStartErrorMessage,
  getDiscoverySettingUpdateErrorMessage,
  getDiscoverySubnetInvalidFormatMessage,
  getDiscoverySubnetInvalidValuesMessage,
  getDiscoverySubnetRequiredMessage,
  getDiscoverySubnetUpdateErrorMessage,
  getDiscoverySubnetValidEntryRequiredMessage,
  getDiscoverySubnetValuesRequiredMessage,
} from '@/utils/infrastructureSettingsPresentation';

type InfrastructureEventBus = {
  on<T extends EventType>(event: T, handler: (data?: EventDataMap[T]) => void): () => void;
};

type RawDiscoveredServer = {
  ip?: string;
  port?: number;
  type?: string;
  version?: string;
  hostname?: string;
  name?: string;
  release?: string;
};

interface UseInfrastructureDiscoveryRuntimeStateParams {
  eventBus: InfrastructureEventBus;
  currentTab: Accessor<SettingsTab>;
  nodes: Accessor<NodeConfigWithStatus[]>;
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
}

export const useInfrastructureDiscoveryRuntimeState = ({
  eventBus,
  currentTab,
  nodes,
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
}: UseInfrastructureDiscoveryRuntimeStateParams) => {
  const [discoveredNodes, setDiscoveredNodes] = createSignal<DiscoveredServer[]>([]);
  const [discoveryScanStatus, setDiscoveryScanStatus] = createSignal<DiscoveryScanStatus>({
    scanning: false,
  });

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

    const { configuredHosts, clusterMemberIPs } = collectConfiguredInfrastructureHosts(nodes());
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

        return {
          ip,
          port: typeof server.port === 'number' ? server.port : type === 'pbs' ? 8007 : 8006,
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
      setDiscoveredNodes((previous) => {
        const existingMap = new Map(previous.map((item) => [`${item.ip}:${item.port}`, item]));
        filtered.forEach((server) => {
          existingMap.set(`${server.ip}:${server.port}`, server);
        });
        return Array.from(existingMap.values());
      });
    } else {
      setDiscoveredNodes(filtered);
    }

    setDiscoveryScanStatus((previous) => ({
      ...previous,
      lastResultAt: Date.now(),
    }));
  };

  const loadDiscoveredNodes = async () => {
    try {
      const { apiFetch } = await import('@/utils/apiClient');
      const response = await apiFetch('/api/discover');
      if (!response.ok) {
        return;
      }

      const data = await response.json();
      if (Array.isArray(data.servers)) {
        updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[]);
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          lastResultAt: typeof data.timestamp === 'number' ? data.timestamp : Date.now(),
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
        return;
      }

      updateDiscoveredNodesFromServers([]);
      setDiscoveryScanStatus((previous) => ({
        ...previous,
        lastResultAt: typeof data?.timestamp === 'number' ? data.timestamp : previous.lastResultAt,
        errors: Array.isArray(data?.errors) && data.errors.length > 0 ? data.errors : undefined,
      }));
    } catch (error) {
      logger.error('Failed to load discovered nodes', error);
    }
  };

  const triggerDiscoveryScan = async (options: { quiet?: boolean } = {}) => {
    const { quiet = false } = options;

    setDiscoveryScanStatus((previous) => ({
      ...previous,
      scanning: true,
      subnet: discoverySubnet() || previous.subnet,
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
      notificationStore.error(getDiscoveryScanStartErrorMessage());
      setDiscoveryScanStatus((previous) => ({
        ...previous,
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
          setDiscoverySubnetError(getDiscoverySubnetValuesRequiredMessage());
          notificationStore.error(getDiscoverySubnetValuesRequiredMessage());
          return false;
        }
        if (!isValidCIDR(trimmedDraft)) {
          setDiscoverySubnetError(getDiscoverySubnetInvalidFormatMessage());
          notificationStore.error(getDiscoverySubnetInvalidValuesMessage());
          return false;
        }
        subnetToSend = normalizeSubnetList(trimmedDraft);
        setDiscoverySubnetDraft(subnetToSend);
        setDiscoverySubnetError(undefined);
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
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: false,
        }));
      }

      return true;
    } catch (error) {
      logger.error('Failed to update discovery setting', error);
      notificationStore.error(getDiscoverySettingUpdateErrorMessage());
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
      setDiscoverySubnetError(getDiscoverySubnetRequiredMessage());
      return false;
    }
    if (!isValidCIDR(value)) {
      setDiscoverySubnetError(getDiscoverySubnetInvalidFormatMessage());
      return false;
    }

    const normalizedValue = normalizeSubnetList(value);
    if (!normalizedValue) {
      setDiscoverySubnetError(getDiscoverySubnetValidEntryRequiredMessage());
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
      notificationStore.error(getDiscoverySubnetUpdateErrorMessage());
      applySavedDiscoverySubnet(previousSubnet);
      setDiscoverySubnetDraft(previousSubnet === 'auto' ? '' : normalizeSubnetList(previousSubnet));
      return false;
    } finally {
      setDiscoverySubnetError(undefined);
      setSavingDiscoverySettings(false);
      await loadDiscoveredNodes();
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
        notificationStore.error(getDiscoverySubnetUpdateErrorMessage());
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

  onMount(() => {
    const unsubscribeDiscovery = eventBus.on('discovery_updated', (data) => {
      if (!data) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: false,
        }));
        return;
      }

      if (Array.isArray(data.servers)) {
        updateDiscoveredNodesFromServers(data.servers as RawDiscoveredServer[], {
          merge: !!data.immediate,
        });
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: data.scanning ?? previous.scanning,
          lastResultAt: data.timestamp ?? Date.now(),
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else if (!data.immediate) {
        updateDiscoveredNodesFromServers([]);
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: data.scanning ?? previous.scanning,
          lastResultAt: data.timestamp ?? previous.lastResultAt,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      } else {
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: data.scanning ?? previous.scanning,
          errors: Array.isArray(data.errors) && data.errors.length > 0 ? data.errors : undefined,
        }));
      }
    });

    const unsubscribeDiscoveryStatus = eventBus.on('discovery_status', (data) => {
      if (!data) {
        setDiscoveryScanStatus((previous) => ({
          ...previous,
          scanning: false,
        }));
        return;
      }

      setDiscoveryScanStatus((previous) => ({
        ...previous,
        scanning: !!data.scanning,
        subnet: data.subnet || previous.subnet,
        lastScanStartedAt: data.scanning ? (data.timestamp ?? Date.now()) : previous.lastScanStartedAt,
        lastResultAt: !data.scanning && data.timestamp ? data.timestamp : previous.lastResultAt,
      }));

      if (typeof data.subnet === 'string' && data.subnet !== discoverySubnet()) {
        applySavedDiscoverySubnet(data.subnet);
      }
    });

    let discoveryInterval: ReturnType<typeof setInterval> | undefined;
    createEffect(() => {
      if (discoveryInterval) {
        clearInterval(discoveryInterval);
        discoveryInterval = undefined;
      }

      if (currentTab() === 'infrastructure-operations') {
        discoveryInterval = setInterval(() => {
          void loadDiscoveredNodes();
        }, 30000);
      }
    });

    onCleanup(() => {
      unsubscribeDiscovery();
      unsubscribeDiscoveryStatus();
      if (discoveryInterval) {
        clearInterval(discoveryInterval);
      }
    });
  });

  return {
    discoveredNodes,
    discoveryScanStatus,
    updateDiscoveredNodesFromServers,
    loadDiscoveredNodes,
    triggerDiscoveryScan,
    handleDiscoveryEnabledChange,
    commitDiscoverySubnet,
    handleDiscoveryModeChange,
  };
};
