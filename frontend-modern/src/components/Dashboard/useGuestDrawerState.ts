import { createMemo, createSignal } from 'solid-js';

import { getDiscoveryLoadingState } from '@/utils/discoveryPresentation';
import {
  getCanonicalWorkloadId,
  getDiscoveryResourceTypeForWorkload,
  getWebInterfaceTargetLabelForWorkload,
} from '@/utils/workloads';
import { buildInfrastructureHrefForWorkload } from '@/routing/resourceLinks';

import {
  getDiscoveryHostIdForWorkload,
  getDiscoveryResourceIdForWorkload,
} from './workloadSelectors';
import {
  getGuestDrawerAgentLabel,
  getGuestDrawerAgentTitle,
  getGuestDrawerBackupPresentation,
  getGuestDrawerMemoryExtraLines,
  getGuestDrawerNetworkInterfaces,
  hasGuestDrawerFilesystemDetails,
  hasGuestDrawerOsInfo,
  normalizeGuestDrawerTags,
  type GuestDrawerProps,
  type GuestDrawerTab,
} from './guestDrawerModel';

export function useGuestDrawerState(props: GuestDrawerProps) {
  const [activeTab, setActiveTab] = createSignal<GuestDrawerTab>('overview');

  const guestId = createMemo(() => getCanonicalWorkloadId(props.guest));
  const infrastructureHref = createMemo(() => buildInfrastructureHrefForWorkload(props.guest));
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
  const ipAddresses = createMemo(() => props.guest.ipAddresses || []);
  const memoryExtraLines = createMemo(() => getGuestDrawerMemoryExtraLines(props.guest));
  const hasFilesystemDetails = createMemo(() => hasGuestDrawerFilesystemDetails(props.guest));
  const networkInterfaces = createMemo(() => getGuestDrawerNetworkInterfaces(props.guest));
  const hasNetworkInterfaces = createMemo(() => networkInterfaces().length > 0);
  const normalizedTags = createMemo(() => normalizeGuestDrawerTags(props.guest.tags));
  const backupPresentation = createMemo(() =>
    props.guest.lastBackup ? getGuestDrawerBackupPresentation(props.guest.lastBackup) : null,
  );
  const discoveryAgentId = createMemo(() => getDiscoveryHostIdForWorkload(props.guest));
  const discoveryResourceId = createMemo(() => getDiscoveryResourceIdForWorkload(props.guest));
  const discoveryResourceType = createMemo(() =>
    getDiscoveryResourceTypeForWorkload(props.guest),
  );
  const webInterfaceTargetLabel = createMemo(() =>
    getWebInterfaceTargetLabelForWorkload(props.guest),
  );

  const switchTab = (tab: GuestDrawerTab) => {
    setActiveTab(tab);
  };

  return {
    activeTab,
    agentLabel,
    agentTitle,
    backupPresentation,
    discoveryAgentId,
    discoveryLoadingState: getDiscoveryLoadingState(),
    discoveryResourceId,
    discoveryResourceType,
    guestId,
    guestOsSummary,
    hasAgentInfo,
    hasFilesystemDetails,
    hasNetworkInterfaces,
    hasOsInfo,
    infrastructureHref,
    ipAddresses,
    memoryExtraLines,
    networkInterfaces,
    normalizedTags,
    osName,
    osVersion,
    switchTab,
    webInterfaceTargetLabel,
  };
}
