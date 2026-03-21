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
import { toDiscoveryAPIResourceType } from '@/utils/discoveryTarget';

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

  const targetAgentId = createMemo(() => props.agentId || '');
  const discoverySourceKey = createMemo(
    () => `${props.resourceType}|${targetAgentId()}|${props.resourceId}`,
  );
  const resourceId = createMemo(() =>
    makeResourceId(props.resourceType, targetAgentId(), props.resourceId),
  );

  const [discoveryInfo] = createResource(
    () => props.resourceType,
    async (type) => {
      try {
        return await getDiscoveryInfo(type);
      } catch {
        return null;
      }
    },
  );

  const [connectedAgents] = createResource(async () => {
    try {
      return await getConnectedAgents();
    } catch {
      return { count: 0, agents: [] };
    }
  });

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

  const [discovery, { refetch, mutate }] = createResource(discoverySourceKey, async () => {
    const agentId = targetAgentId();
    if (!agentId) return null;

    try {
      return await getDiscovery(props.resourceType, agentId, props.resourceId);
    } catch {
      return null;
    }
  });

  createEffect(() => {
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
    if (discovery.loading) {
      const timer = setTimeout(() => {
        if (discovery.loading) {
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
        if (props.commandsEnabled === false) {
          setScanError(
            'Commands not enabled. Enable "Pulse Commands" in Settings → Unified Agents for this agent.',
          );
        } else if (props.commandsEnabled === true) {
          setScanError(
            'Agent not connected for command execution. The API token may be missing the "agent:exec" scope. Check Settings → API Tokens.',
          );
        } else {
          setScanError(
            'No agent available for command execution. Ensure "Pulse Commands" is enabled in Settings → Unified Agents and the API token has "agent:exec" scope.',
          );
        }
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

  createEffect(() => {
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
    const current = discovery();
    if (!current) return false;

    const hasServiceName = current.service_name && current.service_name.toLowerCase() !== 'unknown';
    const hasConfidence = typeof current.confidence === 'number' && current.confidence > 0;
    const hasPorts = current.ports && current.ports.length > 0;
    const hasFacts = current.facts && current.facts.length > 0;
    const hasPaths =
      (current.config_paths && current.config_paths.length > 0) ||
      (current.data_paths && current.data_paths.length > 0) ||
      (current.log_paths && current.log_paths.length > 0);
    const hasCliAccess = Boolean(current.cli_access);

    return Boolean(hasServiceName || hasConfidence || hasPorts || hasFacts || hasPaths || hasCliAccess);
  });

  const validDiscovery = createMemo(() =>
    !discovery.loading && hasValidDiscovery() ? discovery() : null,
  );

  return {
    connectedAgents,
    discovery,
    discoveryInfo,
    editingNotes,
    handleSaveNotes,
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
