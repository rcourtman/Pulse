import {
  createEffect,
  createSignal,
} from 'solid-js';
import type { Resource } from '@/types/resource';
import { createLocalStorageBooleanSignal, STORAGE_KEYS } from '@/utils/localStorage';
import { useResourceDetailDrawerDockerActionsState } from './useResourceDetailDrawerDockerActionsState';
import { useResourceDetailDrawerHistoryState } from './useResourceDetailDrawerHistoryState';
import { useResourceDetailDrawerDerivedState } from './useResourceDetailDrawerDerivedState';

type DrawerTab = 'overview' | 'mail' | 'namespaces' | 'deployments' | 'swarm' | 'debug';

export interface UseResourceDetailDrawerStateOptions {
  resource: Resource;
  resolveResourceLabel?: (resourceId: string) => string | null | undefined;
}

export const useResourceDetailDrawerState = (options: UseResourceDetailDrawerStateOptions) => {
  const { resource, resolveResourceLabel: resolveResourceLabelInput } = options;
  const [activeTab, setActiveTab] = createSignal<DrawerTab>('overview');
  const [debugEnabled] = createLocalStorageBooleanSignal(STORAGE_KEYS.DEBUG_MODE, false);
  const [copied, setCopied] = createSignal(false);
  const [showReportModal, setShowReportModal] = createSignal(false);
  const [showHistoryFilters, setShowHistoryFilters] = createSignal(false);
  const [showAccessContext, setShowAccessContext] = createSignal(false);
  const [showInvestigationContext, setShowInvestigationContext] = createSignal(false);
  const [showCorrelationContext, setShowCorrelationContext] = createSignal(false);
  const [showDiscoveryContext, setShowDiscoveryContext] = createSignal(false);
  const [showHostDetails, setShowHostDetails] = createSignal(false);
  const [showServiceDetails, setShowServiceDetails] = createSignal(false);
  const [showPbsJobDetail, setShowPbsJobDetail] = createSignal(false);
  const [showPmgMailFlowDetail, setShowPmgMailFlowDetail] = createSignal(false);
  const [k8sDeploymentsPrefillNamespace, setK8sDeploymentsPrefillNamespace] = createSignal('');

  const history = useResourceDetailDrawerHistoryState({ resource });
  const derived = useResourceDetailDrawerDerivedState({
    resource,
    resolveResourceLabel: resolveResourceLabelInput,
    debugEnabled,
    resourceIntelligence: history.resourceIntelligence,
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

  return {
    activeTab,
    setActiveTab,
    debugEnabled,
    copied,
    showReportModal,
    setShowReportModal,
    showHistoryFilters,
    setShowHistoryFilters,
    showAccessContext,
    setShowAccessContext,
    showInvestigationContext,
    setShowInvestigationContext,
    showCorrelationContext,
    setShowCorrelationContext,
    showDiscoveryContext,
    setShowDiscoveryContext,
    showHostDetails,
    setShowHostDetails,
    showServiceDetails,
    setShowServiceDetails,
    showPbsJobDetail,
    setShowPbsJobDetail,
    showPmgMailFlowDetail,
    setShowPmgMailFlowDetail,
    k8sDeploymentsPrefillNamespace,
    setK8sDeploymentsPrefillNamespace,
    ...history,
    ...derived,
    ...dockerActions,
    handleCopyJson,
  };
};

export type UseResourceDetailDrawerStateResult = ReturnType<typeof useResourceDetailDrawerState>;
