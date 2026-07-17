import { createEffect, createSignal } from 'solid-js';
import { AgentContextAPI } from '@/api/agentContext';
import type { Resource } from '@/types/resource';
import type { HistoryTimeRange } from '@/api/charts';
import { GUEST_DRAWER_HISTORY_DEFAULT_RANGE } from '@/components/Workloads/guestDrawerModel';
import { aiChatStore } from '@/stores/aiChat';
import { notificationStore } from '@/stores/notifications';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { hasKubernetesDetailSections } from './resourceDetailDrawerKubernetesModel';
import { isPulseAgentPlatformResource } from '@/utils/agentResources';
import { copyToClipboard } from '@/utils/clipboard';
import { formatAgentResourceContextForClipboard } from '@/utils/agentContextPresentation';
import { useResourceDetailDrawerDockerActionsState } from './useResourceDetailDrawerDockerActionsState';
import { useResourceDetailDrawerHistoryState } from './useResourceDetailDrawerHistoryState';
import { useResourceDetailDrawerDerivedState } from './useResourceDetailDrawerDerivedState';
import { buildResourceAssistantContext } from '@/utils/resourceAssistantContextModel';
import type { ResourceDetailDrawerPresentation } from './resourceDetailDrawerPresentation';

type DrawerTab =
  'overview' | 'history' | 'discovery' | 'mail' | 'namespaces' | 'deployments' | 'swarm' | 'debug';

export interface UseResourceDetailDrawerStateOptions {
  resource: Resource;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
  presentation?: ResourceDetailDrawerPresentation;
  initialShowAccessContext?: boolean;
  initialShowHostDetails?: boolean;
  initialShowTrueNASDetails?: boolean;
}

export const useResourceDetailDrawerState = (options: UseResourceDetailDrawerStateOptions) => {
  const { resource, resolveResourceLabel: resolveResourceLabelInput } = options;
  const [activeTab, setActiveTab] = createSignal<DrawerTab>('overview');
  const [metricsHistoryRange, setMetricsHistoryRange] = createSignal<HistoryTimeRange>(
    GUEST_DRAWER_HISTORY_DEFAULT_RANGE,
  );
  const [debugEnabled] = createLocalStorageBooleanSignal(STORAGE_KEYS.DEBUG_MODE, false);
  const [copied, setCopied] = createSignal(false);
  const [copyingAgentContext, setCopyingAgentContext] = createSignal(false);
  const [agentContextCopied, setAgentContextCopied] = createSignal(false);
  const [showReportModal, setShowReportModal] = createSignal(false);
  const [showHistoryFilters, setShowHistoryFilters] = createSignal(false);
  const [showAccessContext, setShowAccessContext] = createSignal(
    options.initialShowAccessContext === true,
  );
  const [showInvestigationContext, setShowInvestigationContext] = createSignal(false);
  const [showDiscoveryContext, setShowDiscoveryContext] = createSignal(false);
  const [showHostDetails, setShowHostDetails] = createSignal(
    options.initialShowHostDetails ??
      (options.presentation === 'table-row' && isPulseAgentPlatformResource(resource)),
  );
  const [showServiceDetails, setShowServiceDetails] = createSignal(false);
  const [showVMwareDetails, setShowVMwareDetails] = createSignal(false);
  const [showTrueNASDetails, setShowTrueNASDetails] = createSignal(
    options.initialShowTrueNASDetails === true,
  );
  const [showKubernetesDetails, setShowKubernetesDetails] = createSignal(
    hasKubernetesDetailSections(resource),
  );
  const [showPbsJobDetail, setShowPbsJobDetail] = createSignal(false);
  const [showPmgMailFlowDetail, setShowPmgMailFlowDetail] = createSignal(false);
  const [k8sDeploymentsPrefillNamespace, setK8sDeploymentsPrefillNamespace] = createSignal('');

  // Table-row drawers defer the history/intelligence/action-audit reads
  // until the user opens the History disclosure; activation latches so a
  // later collapse keeps the loaded data instead of tearing the queries
  // down and refetching on every toggle.
  const [showRowHistory, setShowRowHistorySignal] = createSignal(false);
  const [rowHistoryActivated, setRowHistoryActivated] = createSignal(false);
  const setShowRowHistory = (next: boolean) => {
    if (next) setRowHistoryActivated(true);
    setShowRowHistorySignal(next);
  };

  const history = useResourceDetailDrawerHistoryState({
    resource,
    enableRemoteHistory: () => options.presentation !== 'table-row' || rowHistoryActivated(),
  });
  const derived = useResourceDetailDrawerDerivedState({
    resource,
    resolveResourceLabel: resolveResourceLabelInput,
    debugEnabled,
    resourceIntelligence: history.resourceIntelligence,
    resourceRelationships: history.resourceFacetRelationships,
  });
  const dockerActions = useResourceDetailDrawerDockerActionsState({
    dockerHostSourceId: derived.dockerHostSourceId,
    dockerUpdatesAvailable: derived.dockerUpdatesAvailable,
  });

  createEffect(() => {
    if (!debugEnabled() && activeTab() === 'debug') {
      setActiveTab('overview');
    }
  });

  createEffect(() => {
    if (options.initialShowAccessContext === true) {
      setShowAccessContext(true);
    }
  });

  createEffect(() => {
    const current = activeTab();
    const available = new Set(derived.tabs().map((tab) => tab.id));
    if (!available.has(current)) {
      setActiveTab('overview');
    }
  });

  const handleCopyJson = async () => {
    const payload = derived.debugJson();
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(payload);
      } else {
        const textarea = document.createElement('textarea');
        textarea.value = payload;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };

  const assistantAvailable = () => aiChatStore.enabled === true;

  const openAssistantForResource = () => {
    if (!assistantAvailable()) return;
    aiChatStore.open(buildResourceAssistantContext(resource));
  };

  const copyAgentContext = async () => {
    if (copyingAgentContext()) return;
    setCopyingAgentContext(true);
    setAgentContextCopied(false);

    try {
      const context = await AgentContextAPI.getResourceContext(resource.id);
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

  return {
    activeTab,
    setActiveTab,
    metricsHistoryRange,
    setMetricsHistoryRange,
    debugEnabled,
    copied,
    copyingAgentContext,
    agentContextCopied,
    showReportModal,
    setShowReportModal,
    showHistoryFilters,
    setShowHistoryFilters,
    showAccessContext,
    setShowAccessContext,
    showInvestigationContext,
    setShowInvestigationContext,
    showDiscoveryContext,
    setShowDiscoveryContext,
    showHostDetails,
    setShowHostDetails,
    showServiceDetails,
    setShowServiceDetails,
    showVMwareDetails,
    setShowVMwareDetails,
    showTrueNASDetails,
    setShowTrueNASDetails,
    showKubernetesDetails,
    setShowKubernetesDetails,
    showPbsJobDetail,
    setShowPbsJobDetail,
    showPmgMailFlowDetail,
    setShowPmgMailFlowDetail,
    k8sDeploymentsPrefillNamespace,
    setK8sDeploymentsPrefillNamespace,
    showRowHistory,
    setShowRowHistory,
    ...history,
    ...derived,
    ...dockerActions,
    assistantAvailable,
    openAssistantForResource,
    copyAgentContext,
    handleCopyJson,
  };
};

export type UseResourceDetailDrawerStateResult = ReturnType<typeof useResourceDetailDrawerState>;
