import { createEffect, createMemo, createResource, createSignal, onCleanup } from 'solid-js';

import {
  getConnectedAgents,
  getDiscovery,
  getDiscoveryInfo,
  triggerDiscovery,
  updateDiscoveryNotes,
} from '@/api/discovery';
import { eventBus } from '@/stores/events';
import type { DiscoveryProgress, ResourceType } from '@/types/discovery';
import {
  getDiscoveryNoConnectedAgentMessage,
  hasMeaningfulDiscoveryContext,
} from '@/utils/discoveryPresentation';
import { copyToClipboard } from '@/utils/clipboard';
import { toDiscoveryAPIResourceType } from '@/utils/discoveryTarget';
import { computeDiscoveryReadiness, type DiscoveryReadiness } from './discoveryReadiness';
import { useDiscoveryFeatureAvailability } from './useDiscoveryFeatureAvailability';

export interface DiscoveryTabStateProps {
  resourceType: ResourceType;
  agentId?: string;
  resourceId: string;
  hostname: string;
  commandsEnabled?: boolean;
  /** Snapshot-based safety pause for manual runs; saved reads remain available. */
  runBlockReason?: string | null;
}

const makeResourceId = (type: ResourceType, agentId: string, resourceId: string) =>
  `${toDiscoveryAPIResourceType(type) || type}:${agentId}:${resourceId}`;

export function useDiscoveryTabState(props: DiscoveryTabStateProps) {
  const [isScanning, setIsScanning] = createSignal(false);
  const [editingNotes, setEditingNotes] = createSignal(false);
  const [liveElapsedSeconds, setLiveElapsedSeconds] = createSignal(0);
  const [scanStartTime, setScanStartTime] = createSignal<number | null>(null);
  const [showLoadingSpinner, setShowLoadingSpinner] = createSignal(false);
  const [notesText, setNotesText] = createSignal('');
  const [saveError, setSaveError] = createSignal<string | null>(null);
  const [scanError, setScanError] = createSignal<string | null>(null);
  const [scanProgress, setScanProgress] = createSignal<DiscoveryProgress | null>(null);
  const [scanSuccess, setScanSuccess] = createSignal(false);
  const [showCommandsPreview, setShowCommandsPreview] = createSignal(false);
  const [showExplanation, setShowExplanation] = createSignal(true);
  const [httpScanInProgress, setHttpScanInProgress] = createSignal(false);
  const [copiedDiscoveryValue, setCopiedDiscoveryValue] = createSignal('');
  let copyFeedbackTimer: ReturnType<typeof setTimeout> | undefined;
  let successTimer: ReturnType<typeof setTimeout> | undefined;
  let completionTimer: ReturnType<typeof setTimeout> | undefined;
  let contextVersion = 0;
  let runVersion = 0;
  let disposed = false;

  const clearScanTimers = () => {
    clearTimeout(successTimer);
    clearTimeout(completionTimer);
    successTimer = undefined;
    completionTimer = undefined;
  };

  const targetAgentId = createMemo(() => props.agentId || '');
  const discoverySourceKey = createMemo(
    () => `${props.resourceType}|${targetAgentId()}|${props.resourceId}`,
  );
  const resourceId = createMemo(() =>
    makeResourceId(props.resourceType, targetAgentId(), props.resourceId),
  );

  const { discoveryFeatureEnabled, discoveryFeatureKnownDisabled } =
    useDiscoveryFeatureAvailability();

  // A target key alone is insufficient: leaving and returning to the same
  // guest must not admit a response from the previous visit. A precaution on
  // the same target does not change this ownership or cancel dispatched work.
  const captureContext = () => ({
    version: contextVersion,
    key: discoverySourceKey(),
    type: props.resourceType,
    agentId: targetAgentId(),
    resourceId: props.resourceId,
    hostname: props.hostname,
  });
  const isCurrentContext = (context: ReturnType<typeof captureContext>) =>
    !disposed &&
    context.version === contextVersion &&
    context.key === discoverySourceKey() &&
    discoveryFeatureEnabled();

  const [discoveryInfo] = createResource(
    () => (discoveryFeatureEnabled() ? props.resourceType : null),
    async (type) => {
      if (!type) return null;
      try {
        return await getDiscoveryInfo(type);
      } catch {
        return null;
      }
    },
  );

  const [connectedAgents] = createResource(
    () => discoveryFeatureEnabled(),
    async (enabled) => {
      if (!enabled) {
        return { count: 0, agents: [] };
      }
      try {
        return await getConnectedAgents();
      } catch {
        return { count: 0, agents: [] };
      }
    },
  );

  const hasConnectedAgent = createMemo(() => {
    const agentId = targetAgentId();
    const agents = connectedAgents()?.agents || [];

    if (!agentId) return false;
    if (agents.some((agent) => agent.agent_id === agentId)) return true;
    if (agents.some((agent) => agent.hostname === props.hostname || agent.hostname === agentId)) {
      return true;
    }

    return agents.length === 1;
  });

  // Whether an AI provider is configured to analyze discovery evidence. The
  // info fetch only resolves an `ai_provider` when one has credentials, so an
  // absent provider (or a still-loading fetch) reads as "not configured".
  const aiProviderConfigured = createMemo(
    () => !discoveryInfo.loading && Boolean(discoveryInfo()?.ai_provider),
  );

  // Single prerequisite verdict — the canonical source every surface should
  // render from instead of re-deriving disabled/provider/commands/connectivity
  // ad hoc. Ordered most-fundamental-first inside computeDiscoveryReadiness.
  const discoveryReadiness = createMemo<DiscoveryReadiness>(() =>
    computeDiscoveryReadiness({
      discoveryEnabled: discoveryFeatureEnabled(),
      aiProviderConfigured: aiProviderConfigured(),
      commandsEnabled: props.commandsEnabled,
      hasConnectedAgent: hasConnectedAgent(),
    }),
  );

  // Gate every run affordance on a configured provider: discovery uses the AI
  // to analyze evidence, so without a provider a scan is guaranteed to fail —
  // the tab-wide banner already tells the user to configure one before
  // scanning. (Command/connectivity gaps are surfaced separately, not blocked
  // here, since their backend semantics are murkier.)
  const runBlockReason = createMemo(() => props.runBlockReason?.trim() || null);
  const canTriggerDiscovery = createMemo(
    () =>
      discoveryFeatureEnabled() &&
      aiProviderConfigured() &&
      Boolean(targetAgentId()) &&
      !runBlockReason(),
  );

  const [discovery, { refetch, mutate }] = createResource(
    () => (discoveryFeatureEnabled() ? discoverySourceKey() : null),
    async (sourceKey) => {
      if (!sourceKey) return null;

      const agentId = targetAgentId();
      if (!agentId) return null;

      try {
        return await getDiscovery(props.resourceType, agentId, props.resourceId);
      } catch {
        return null;
      }
    },
  );

  createEffect(() => {
    void discoveryFeatureEnabled();
    void discoverySourceKey();
    contextVersion++;
    runVersion++;
    clearScanTimers();
    setIsScanning(false);
    setHttpScanInProgress(false);
    setScanProgress(null);
    setScanError(null);
    setScanSuccess(false);
    setScanStartTime(null);
    setLiveElapsedSeconds(0);
    setShowLoadingSpinner(false);
    setEditingNotes(false);
    setSaveError(null);
  });

  createEffect(() => {
    if (discoveryFeatureKnownDisabled()) {
      setShowLoadingSpinner(false);
      return;
    }
    if (discovery.loading) {
      const timer = setTimeout(() => {
        if (discovery.loading && !discoveryFeatureKnownDisabled()) {
          setShowLoadingSpinner(true);
        }
      }, 150);
      onCleanup(() => clearTimeout(timer));
      return;
    }

    setShowLoadingSpinner(false);
  });

  createEffect(() => {
    const startedAt = scanStartTime();
    if (!isScanning() || !startedAt) return;

    const interval = setInterval(() => {
      setLiveElapsedSeconds(Math.floor((Date.now() - startedAt) / 1000));
    }, 1000);

    onCleanup(() => clearInterval(interval));
  });

  const handleTriggerDiscovery = async (force = false) => {
    // Check the current snapshot at the dispatch boundary as well as disabling
    // buttons. Do not cancel a dispatched scan when a precaution appears.
    if (disposed || runBlockReason() || isScanning()) return;
    if (!discoveryFeatureEnabled()) {
      setScanError('Service context is disabled in Settings -> Pulse Intelligence -> Assistant.');
      return;
    }
    const context = captureContext();
    if (!context.agentId) {
      setScanError('Agent identifier unavailable for discovery');
      return;
    }
    if (!aiProviderConfigured()) return;

    clearScanTimers();
    const version = ++runVersion;
    const ownsRun = () => isCurrentContext(context) && version === runVersion;
    setIsScanning(true);
    setHttpScanInProgress(true);
    setScanProgress(null);
    setScanError(null);
    setScanSuccess(false);
    setScanStartTime(Date.now());
    setLiveElapsedSeconds(0);

    try {
      const result = await triggerDiscovery(context.type, context.agentId, context.resourceId, {
        force,
        hostname: context.hostname,
      });
      if (!ownsRun()) return;
      if (!result) throw new Error('Discovery returned no saved result.');
      mutate(result);
      setScanError(null);
      setScanSuccess(true);
      successTimer = setTimeout(() => {
        successTimer = undefined;
        if (ownsRun()) setScanSuccess(false);
      }, 2000);
    } catch (err) {
      if (!ownsRun()) return;
      console.error('Discovery failed:', err);
      const message = err instanceof Error ? err.message : 'Discovery scan failed';
      setScanError(
        message.includes('no connected agent')
          ? getDiscoveryNoConnectedAgentMessage(props.commandsEnabled)
          : message,
      );
    } finally {
      if (ownsRun()) {
        setHttpScanInProgress(false);
        setIsScanning(false);
        setScanProgress(null);
        setScanStartTime(null);
      }
    }
  };

  const handleSaveNotes = async () => {
    if (disposed || !discoveryFeatureEnabled()) return;
    setSaveError(null);
    const context = captureContext();
    if (!context.agentId) {
      setSaveError('Agent identifier unavailable for discovery');
      return;
    }

    try {
      await updateDiscoveryNotes(context.type, context.agentId, context.resourceId, {
        user_notes: notesText(),
      });
      if (!isCurrentContext(context)) return;
      setEditingNotes(false);
      await refetch();
    } catch (err) {
      if (isCurrentContext(context)) {
        setSaveError(err instanceof Error ? err.message : 'Failed to save notes');
      }
    }
  };

  const startEditingNotes = () => {
    setNotesText(discovery()?.user_notes || '');
    setEditingNotes(true);
  };

  const clearCopyFeedbackTimer = () => {
    if (copyFeedbackTimer === undefined) return;
    clearTimeout(copyFeedbackTimer);
    copyFeedbackTimer = undefined;
  };

  onCleanup(() => {
    disposed = true;
    contextVersion++;
    runVersion++;
    clearScanTimers();
    clearCopyFeedbackTimer();
  });

  const handleCopyDiscoveryValue = async (value?: string | null) => {
    const text = (value || '').trim();
    if (!text) return;
    const context = captureContext();
    const copied = await copyToClipboard(text);
    if (!copied || !isCurrentContext(context)) return;

    clearCopyFeedbackTimer();
    setCopiedDiscoveryValue(text);
    copyFeedbackTimer = setTimeout(() => {
      setCopiedDiscoveryValue('');
      copyFeedbackTimer = undefined;
    }, 2000);
  };

  createEffect(() => {
    if (!discoveryFeatureEnabled()) return;

    const unsubscribe = eventBus.on('ai_discovery_progress', (progress) => {
      if (!progress || progress.resource_id !== resourceId()) return;

      setScanProgress(progress);
      if (progress.status === 'running' || progress.status === 'pending') {
        if (!httpScanInProgress() && !isScanning()) {
          clearScanTimers();
          runVersion++;
          setScanError(null);
          setScanSuccess(false);
          const startedAt = Date.parse(progress.started_at || '');
          setScanStartTime(Number.isFinite(startedAt) ? startedAt : Date.now());
        }
        setIsScanning(true);
        return;
      }
      if (progress.status !== 'completed' && progress.status !== 'failed') return;

      // The scanner can emit completed-with-error when QGA is paused or no
      // command evidence was collected. It is not successful saved Discovery.
      // Keep that reason visible even when this tab did not initiate the scan.
      const error =
        progress.error?.trim() || (progress.status === 'failed' ? 'Discovery scan failed.' : null);
      if (error) {
        setScanError(error);
        setScanSuccess(false);
      }
      // Scanner completion can precede analysis/persistence. A manual request
      // owns its final result until HTTP settles, not an intermediate event.
      if (httpScanInProgress()) return;

      clearScanTimers();
      const version = ++runVersion;
      setScanSuccess(false);
      setIsScanning(false);
      setScanStartTime(null);
      setScanProgress(null);
      if (error) return;

      const context = captureContext();
      const ownsRefresh = () => isCurrentContext(context) && version === runVersion;
      completionTimer = setTimeout(async () => {
        completionTimer = undefined;
        if (!context.agentId || !ownsRefresh()) return;
        try {
          const result = await getDiscovery(context.type, context.agentId, context.resourceId);
          if (ownsRefresh()) mutate(result);
        } catch (err) {
          if (!ownsRefresh()) return;
          const status = (err as { status?: number } | null)?.status;
          if (status === 401 || status === 403) mutate(null);
          console.error('Failed to fetch discovery after completion:', err);
        }
      }, 500);
    });

    onCleanup(() => {
      unsubscribe();
    });
  });

  const hasValidDiscovery = createMemo(() => {
    return hasMeaningfulDiscoveryContext(discovery());
  });

  const validDiscovery = createMemo(() =>
    !discovery.loading && hasValidDiscovery() ? discovery() : null,
  );

  return {
    connectedAgents,
    canTriggerDiscovery,
    copiedDiscoveryValue,
    discovery,
    discoveryFeatureKnownDisabled,
    discoveryReadiness,
    discoveryInfo,
    editingNotes,
    handleSaveNotes,
    handleCopyDiscoveryValue,
    handleTriggerDiscovery,
    hasConnectedAgent,
    hasValidDiscovery,
    isScanning,
    liveElapsedSeconds,
    notesText,
    mutateDiscovery: mutate,
    refetchDiscovery: refetch,
    runBlockReason,
    saveError,
    scanError,
    scanProgress,
    scanSuccess,
    setEditingNotes,
    setNotesText,
    setScanError,
    setShowCommandsPreview,
    setShowExplanation,
    showCommandsPreview,
    showExplanation,
    showLoadingSpinner,
    startEditingNotes,
    validDiscovery,
  };
}
