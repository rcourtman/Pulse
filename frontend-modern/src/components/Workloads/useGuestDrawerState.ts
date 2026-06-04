import { createEffect, createMemo, createSignal } from 'solid-js';

import { AgentContextAPI } from '@/api/agentContext';
import { getDiscovery } from '@/api/discovery';
import type { HistoryTimeRange } from '@/api/charts';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { aiChatStore } from '@/stores/aiChat';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { notificationStore } from '@/stores/notifications';
import type { ResourceDiscovery, ResourceType as DiscoveryResourceType } from '@/types/discovery';
import { formatAgentResourceContextForClipboard } from '@/utils/agentContextPresentation';
import { copyToClipboard } from '@/utils/clipboard';
import {
  getDiscoveryIdentifiedSummary,
  getDiscoveryLoadingState,
} from '@/utils/discoveryPresentation';
import type { DisplayMetricType } from '@/utils/metricThresholds';
import {
  getCanonicalWorkloadId,
  getDiscoveryResourceTypeForWorkload,
  hasDiscoverySupportForWorkload,
  getWebInterfaceTargetLabelForWorkload,
} from '@/utils/workloads';

import {
  getDiscoveryHostIdForWorkload,
  getDiscoveryResourceIdForWorkload,
  getWorkloadAlertResourceIdCandidates,
  getWorkloadAlertThresholdScope,
} from './workloadTopology';
import {
  getGuestDrawerAgentLabel,
  getGuestDrawerAgentTitle,
  getGuestDrawerBackupPresentation,
  GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  getGuestDrawerHistoryTarget,
  getGuestDrawerNetworkInterfaces,
  hasGuestDrawerFilesystemDetails,
  hasGuestDrawerOsInfo,
  normalizeGuestDrawerTags,
  type GuestDrawerProps,
  type GuestDrawerTab,
} from './guestDrawerModel';
import { buildGuestAssistantContext } from './guestAssistantContextModel';
import { shouldShowInGuestAgentInstallCue } from './workloadAgentReadiness';

interface GuestDiscoverySourceKey {
  type: DiscoveryResourceType;
  agent: string;
  resource: string;
}

export function useGuestDrawerState(props: GuestDrawerProps) {
  const alertsActivation = useAlertsActivation();
  const [activeTab, setActiveTab] = createSignal<GuestDrawerTab>('overview');
  const [historyRange, setHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const [copyingAgentContext, setCopyingAgentContext] = createSignal(false);
  const [agentContextCopied, setAgentContextCopied] = createSignal(false);

  const guestId = createMemo(() => getCanonicalWorkloadId(props.guest));
  const alertThresholdScope = createMemo(() => getWorkloadAlertThresholdScope(props.guest));
  const alertResourceIdCandidates = createMemo(() =>
    getWorkloadAlertResourceIdCandidates(props.guest),
  );
  const metricThresholds = (metric: DisplayMetricType) =>
    createMemo(() =>
      alertsActivation.getMetricThresholds(
        alertThresholdScope(),
        metric,
        alertResourceIdCandidates(),
      ),
    );
  const diskThresholds = metricThresholds('disk');
  const osName = createMemo(() => props.guest.osName || '');
  const osVersion = createMemo(() => props.guest.osVersion || '');
  const guestOsSummary = createMemo(() => {
    const name = osName().trim();
    const version = osVersion().trim();
    if (name && version) return `${name} • ${version}`;
    return name || version;
  });
  const hasOsInfo = createMemo(() => hasGuestDrawerOsInfo(props.guest));
  const agentLabel = createMemo(() => getGuestDrawerAgentLabel(props.guest));
  const agentTitle = createMemo(() => getGuestDrawerAgentTitle(props.guest));
  const hasAgentInfo = createMemo(() => agentLabel().length > 0);
  const showInGuestAgentInstallCue = createMemo(() =>
    shouldShowInGuestAgentInstallCue(props.guest, props.parentNodeOnline !== false),
  );
  const ipAddresses = createMemo(() => props.guest.ipAddresses || []);
  const hasFilesystemDetails = createMemo(() => hasGuestDrawerFilesystemDetails(props.guest));
  const networkInterfaces = createMemo(() => getGuestDrawerNetworkInterfaces(props.guest));
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const normalizedTags = createMemo(() => normalizeGuestDrawerTags(props.guest.tags));
  const backupPresentation = createMemo(() =>
    props.guest.lastBackup ? getGuestDrawerBackupPresentation(props.guest.lastBackup) : null,
  );
  const hasDiscoverySupport = createMemo(() => hasDiscoverySupportForWorkload(props.guest));
  const historyTarget = createMemo(() => getGuestDrawerHistoryTarget(props.guest));
  const hasHistorySupport = createMemo(() => historyTarget() !== null);
  const discoveryAgentId = createMemo(() => getDiscoveryHostIdForWorkload(props.guest));
  const discoveryResourceId = createMemo(() => getDiscoveryResourceIdForWorkload(props.guest));
  const discoveryResourceType = createMemo(() => getDiscoveryResourceTypeForWorkload(props.guest));
  const webInterfaceTargetLabel = createMemo(() =>
    getWebInterfaceTargetLabelForWorkload(props.guest),
  );

  // Load the existing discovery record (if any) so the Overview can surface
  // the identified service alongside CPU/agent/OS metadata. This is a passive
  // read — no scan is triggered. The discovery sub-tab continues to own
  // manual scan triggering and progress UI.
  const discoverySourceKey = createMemo(() => {
    const type = discoveryResourceType();
    const agent = discoveryAgentId();
    const resource = discoveryResourceId();
    if (!type || !agent || !resource) return null;
    return { type, agent, resource } as GuestDiscoverySourceKey;
  });
  const discoveryRecord = createNonSuspendingQuery<
    ResourceDiscovery | null,
    GuestDiscoverySourceKey
  >({
    source: discoverySourceKey,
    initialValue: null,
    cacheKey: (key) => `guest-drawer-discovery:${key.type}:${key.agent}:${key.resource}`,
    fetcher: async (key) => {
      try {
        return await getDiscovery(key.type, key.agent, key.resource);
      } catch {
        return null;
      }
    },
  });
  const discoveryIdentifiedSummary = createMemo(() =>
    getDiscoveryIdentifiedSummary(discoveryRecord.value()),
  );

  const switchTab = (tab: GuestDrawerTab) => {
    setActiveTab(tab);
  };

  const assistantAvailable = () => aiChatStore.enabled === true;

  const openAssistantForGuest = () => {
    if (!assistantAvailable()) return;
    aiChatStore.open(buildGuestAssistantContext(props.guest));
  };

  const copyAgentContext = async () => {
    if (copyingAgentContext()) return;
    setCopyingAgentContext(true);
    setAgentContextCopied(false);

    try {
      const context = await AgentContextAPI.getResourceContext(guestId());
      const copiedContext = await copyToClipboard(formatAgentResourceContextForClipboard(context));
      if (!copiedContext) {
        throw new Error('Clipboard unavailable');
      }
      setAgentContextCopied(true);
      notificationStore.success('Resource context copied.');
      setTimeout(() => setAgentContextCopied(false), 2000);
    } catch {
      notificationStore.error('Unable to copy resource context.');
      setAgentContextCopied(false);
    } finally {
      setCopyingAgentContext(false);
    }
  };

  createEffect(() => {
    if (activeTab() === 'discovery' && !hasDiscoverySupport()) {
      setActiveTab('overview');
    }
    if (activeTab() === 'history' && !hasHistorySupport()) {
      setActiveTab('overview');
    }
  });

  return {
    activeTab,
    agentLabel,
    agentTitle,
    backupPresentation,
    copyingAgentContext,
    agentContextCopied,
    discoveryAgentId,
    discoveryIdentifiedSummary,
    discoveryLoadingState: getDiscoveryLoadingState(),
    discoveryResourceId,
    discoveryResourceType,
    diskThresholds,
    guestId,
    guestOsSummary,
    hasAgentInfo,
    hasDiscoverySupport,
    hasFilesystemDetails,
    hasHistorySupport,
    hasNetworkInterfaces,
    hasOsInfo,
    historyTarget,
    historyRange,
    ipAddresses,
    networkInterfaces,
    normalizedTags,
    osName,
    osVersion,
    showInGuestAgentInstallCue,
    assistantAvailable,
    openAssistantForGuest,
    copyAgentContext,
    switchTab,
    setHistoryRange,
    webInterfaceTargetLabel,
  };
}
