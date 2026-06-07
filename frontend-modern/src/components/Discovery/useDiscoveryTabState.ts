import { createEffect, createMemo, createResource, createSignal, onCleanup } from 'solid-js';

import {
  getConnectedAgents,
  getDiscovery,
  getDiscoveryInfo,
  triggerDiscovery,
  updateDiscoveryNotes,
} from '@/api/discovery';
import { AIAPI } from '@/api/ai';
import { eventBus } from '@/stores/events';
import type { DiscoveryProgress, ResourceType } from '@/types/discovery';
import {
  getDiscoveryNoConnectedAgentMessage,
  hasMeaningfulDiscoveryContext,
} from '@/utils/discoveryPresentation';
import { copyToClipboard } from '@/utils/clipboard';
import { toDiscoveryAPIResourceType } from '@/utils/discoveryTarget';
import { computeDiscoveryReadiness, type DiscoveryReadiness } from './discoveryReadiness';

export interface DiscoveryTabStateProps {
  resourceType: ResourceType;
  agentId?: string;
  resourceId: string;
  hostname: string;
  commandsEnabled?: boolean;
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

  const targetAgentId = createMemo(() => props.agentId || '');
  const discoverySourceKey = createMemo(
    () => `${props.resourceType}|${targetAgentId()}|${props.resourceId}`,
  );
  const resourceId = createMemo(() =>
    makeResourceId(props.resourceType, targetAgentId(), props.resourceId),
  );

  const [aiSettings] = createResource(async () => {
    try {
      return await AIAPI.getSettings();
    } catch {
      return null;
    }
  });
  const discoveryFeatureResolved = createMemo(() => !aiSettings.loading);
  const discoveryFeatureEnabled = createMemo(
    () => discoveryFeatureResolved() && aiSettings()?.discovery_enabled !== false,
  );
  const discoveryFeatureKnownDisabled = createMemo(
    () => discoveryFeatureResolved() && aiSettings()?.discovery_enabled === false,
  );

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

  const canTriggerDiscovery = createMemo(
    () => discoveryFeatureEnabled() && Boolean(targetAgentId()),
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
    if (!discoveryFeatureEnabled()) {
      setScanError('AI discovery is disabled in Settings -> AI.');
      return;
    }

    setIsScanning(true);
    setHttpScanInProgress(true);
    setScanProgress(null);
    setScanError(null);
    setScanSuccess(false);
    setScanStartTime(Date.now());
    setLiveElapsedSeconds(0);

    try {
      const agentId = targetAgentId();
      if (!agentId) {
        setScanError('Agent identifier unavailable for discovery');
        return;
      }

      const result = await triggerDiscovery(props.resourceType, agentId, props.resourceId, {
        force,
        hostname: props.hostname,
      });
      if (result) {
        mutate(result);
      }

      setHttpScanInProgress(false);
      setIsScanning(false);
      setScanProgress(null);
      setScanStartTime(null);
      setScanSuccess(true);
      setTimeout(() => setScanSuccess(false), 2000);
    } catch (err) {
      console.error('Discovery failed:', err);
      const message = err instanceof Error ? err.message : 'Discovery scan failed';

      if (message.includes('no connected agent')) {
        setScanError(getDiscoveryNoConnectedAgentMessage(props.commandsEnabled));
      } else {
        setScanError(message);
      }

      setHttpScanInProgress(false);
      setIsScanning(false);
      setScanProgress(null);
      setScanStartTime(null);
    }
  };

  const handleSaveNotes = async () => {
    setSaveError(null);
    const agentId = targetAgentId();
    if (!agentId) {
      setSaveError('Agent identifier unavailable for discovery');
      return;
    }

    try {
      await updateDiscoveryNotes(props.resourceType, agentId, props.resourceId, {
        user_notes: notesText(),
      });
      setEditingNotes(false);
      await refetch();
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : 'Failed to save notes');
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
    clearCopyFeedbackTimer();
  });

  const handleCopyDiscoveryValue = async (value?: string | null) => {
    const text = (value || '').trim();
    if (!text) return;
    const copied = await copyToClipboard(text);
    if (!copied) return;

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

      if (
        (progress.status === 'completed' || progress.status === 'failed') &&
        !httpScanInProgress()
      ) {
        setIsScanning(false);
        setTimeout(async () => {
          const agentId = targetAgentId();
          if (!agentId) return;

          try {
            const result = await getDiscovery(props.resourceType, agentId, props.resourceId);
            if (result) {
              mutate(result);
            }
          } catch (err) {
            console.error('Failed to fetch discovery after completion:', err);
          }

          setScanProgress(null);
        }, 500);
      }
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
