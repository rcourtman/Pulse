import {
  createEffect,
  createMemo,
  createResource,
  createSignal,
  onCleanup,
  onMount,
} from 'solid-js';
import { fetchAgentCapabilitiesManifest } from '@/api/agentCapabilities';
import type { APITokenRecord } from '@/api/security';
import { SecurityAPI } from '@/api/security';
import { useWebSocket } from '@/contexts/appRuntime';
import { MONITORING_READ_SCOPE, API_SCOPE_LABELS } from '@/constants/apiScopes';
import {
  API_TOKEN_PRESET_QUERY_PARAM,
  PULSE_INTELLIGENCE_AGENT_TOKEN_PRESET,
} from '@/routing/resourceLinks';
import { useResources } from '@/hooks/useResources';
import { notificationStore } from '@/stores/notifications';
import { showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import type { Resource } from '@/types/resource';
import { formatRelativeTime } from '@/utils/format';
import {
  getAPITokenDockerPodmanUsageCountLabel,
  getAPITokenGenerateErrorMessage,
  getAPITokenRevealSettingsNote,
  getAPITokensLoadErrorMessage,
  getAPITokenRevokeErrorMessage,
} from '@/utils/apiTokenPresentation';
import { logger } from '@/utils/logger';
import { getPulseBaseUrl } from '@/utils/url';
import {
  API_TOKEN_SCOPES_DOC_URL,
  API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID,
  API_TOKEN_WILDCARD_SCOPE,
  agentActionIdForResource,
  buildAgentTokenUsage,
  buildDockerTokenUsage,
  countWildcardTokens,
  dockerActionIdForResource,
  getAPITokenScopePresets,
  getAPITokenDialogName,
  getAPITokenHint,
  groupAPITokenScopes,
  hasAgentScopeResource,
  matchesScopePreset,
  revokedTokenIdForResource,
  sortAPITokensByCreatedAt,
  tokenIdForResource,
  tokenRevokedAtForResource,
} from './apiTokenManagerModel';

interface APITokenManagerProps {
  currentTokenHint?: string;
  onTokensChanged?: () => void;
  refreshing?: boolean;
  canManage?: boolean;
}

export const useAPITokenManagerState = (props: APITokenManagerProps) => {
  const { markDockerRuntimesTokenRevoked, markAgentsTokenRevoked } = useWebSocket();
  const { byType, resources } = useResources();

  const dockerRuntimeResources = createMemo(() => byType('docker-host'));
  const agentCapableResources = createMemo(() =>
    resources().filter((resource: Resource) => hasAgentScopeResource(resource)),
  );

  const dockerTokenUsage = createMemo(() => buildDockerTokenUsage(dockerRuntimeResources()));
  const agentTokenUsage = createMemo(() => buildAgentTokenUsage(agentCapableResources()));

  const [tokens, setTokens] = createSignal<APITokenRecord[]>([]);
  const [tokensLoaded, setTokensLoaded] = createSignal(false);
  const [loading, setLoading] = createSignal(true);
  const [isGenerating, setIsGenerating] = createSignal(false);
  const [newTokenValue, setNewTokenValue] = createSignal<string | null>(null);
  const [newTokenRecord, setNewTokenRecord] = createSignal<APITokenRecord | null>(null);
  const [nameInput, setNameInput] = createSignal('');
  const [selectedScopes, setSelectedScopes] = createSignal<string[]>([]);
  const [agentCapabilitiesManifest] = createResource(fetchAgentCapabilitiesManifest);
  const tokenRevealState = useTokenRevealState();
  const canManage = () => props.canManage !== false;

  const sortedTokens = createMemo(() => sortAPITokensByCreatedAt(tokens()));
  const totalTokens = createMemo(() => sortedTokens().length);
  const wildcardCount = createMemo(() => countWildcardTokens(sortedTokens()));
  const scopedTokenCount = createMemo(() => totalTokens() - wildcardCount());
  const hasWildcardTokens = createMemo(() => wildcardCount() > 0);
  const scopeGroups = createMemo(() => groupAPITokenScopes());
  const scopePresets = createMemo(() =>
    getAPITokenScopePresets(agentCapabilitiesManifest()?.requiredScopes ?? []),
  );
  const pulseIntelligenceAgentPreset = createMemo(() =>
    scopePresets().find((preset) => preset.id === API_TOKEN_PULSE_INTELLIGENCE_AGENT_PRESET_ID),
  );
  const hasScopeSelection = () => selectedScopes().length > 0;
  const isFullAccessSelected = () => selectedScopes().includes(API_TOKEN_WILDCARD_SCOPE);
  const canGenerateToken = () => canManage() && hasScopeSelection() && !isGenerating();

  let createSectionRef: HTMLDivElement | undefined;
  const [createHighlight, setCreateHighlight] = createSignal(false);
  const [createSectionReady, setCreateSectionReady] = createSignal(false);
  const [requestedTokenPreset, setRequestedTokenPreset] = createSignal('');
  const [appliedRoutePreset, setAppliedRoutePreset] = createSignal('');
  const [focusedRoutePreset, setFocusedRoutePreset] = createSignal('');
  let highlightTimer: number | undefined;
  let routeFocusTimer: number | undefined;

  const readRequestedTokenPreset = () => {
    if (typeof window === 'undefined') return '';
    return (
      new URLSearchParams(window.location.search).get(API_TOKEN_PRESET_QUERY_PARAM)?.trim() ?? ''
    );
  };

  const setCreateSectionRef = (element: HTMLDivElement) => {
    createSectionRef = element;
    setCreateSectionReady(true);
  };

  const findScrollableAncestor = (element: HTMLElement): HTMLElement | null => {
    let current = element.parentElement;
    while (current) {
      const style = window.getComputedStyle(current);
      const canScroll =
        (style.overflowY === 'auto' || style.overflowY === 'scroll') &&
        current.scrollHeight > current.clientHeight;
      if (canScroll) {
        return current;
      }
      current = current.parentElement;
    }
    return null;
  };

  const focusCreateSection = () => {
    if (!createSectionRef) return;
    if (typeof createSectionRef.scrollIntoView === 'function') {
      createSectionRef.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
    const scrollParent = findScrollableAncestor(createSectionRef);
    if (scrollParent && typeof scrollParent.scrollTo === 'function') {
      const parentRect = scrollParent.getBoundingClientRect();
      const sectionRect = createSectionRef.getBoundingClientRect();
      scrollParent.scrollTo({
        top: Math.max(0, scrollParent.scrollTop + sectionRect.top - parentRect.top - 16),
        behavior: 'smooth',
      });
    }
    setCreateHighlight(true);
    window.clearTimeout(highlightTimer);
    highlightTimer = window.setTimeout(() => setCreateHighlight(false), 1600);
  };

  onCleanup(() => {
    if (highlightTimer) window.clearTimeout(highlightTimer);
    if (routeFocusTimer) window.clearTimeout(routeFocusTimer);
  });

  const loadTokens = async () => {
    setLoading(true);
    setTokensLoaded(false);
    try {
      const list = await SecurityAPI.listTokens();
      setTokens(list);
      setTokensLoaded(true);
    } catch (err) {
      logger.error('Failed to load API tokens', err);
      notificationStore.error(getAPITokensLoadErrorMessage());
    } finally {
      setLoading(false);
    }
  };

  onMount(() => {
    setRequestedTokenPreset(readRequestedTokenPreset());
    const handleRoutePresetChange = () => {
      setRequestedTokenPreset(readRequestedTokenPreset());
    };
    window.addEventListener('hashchange', handleRoutePresetChange);
    window.addEventListener('popstate', handleRoutePresetChange);
    void loadTokens();
    onCleanup(() => {
      window.removeEventListener('hashchange', handleRoutePresetChange);
      window.removeEventListener('popstate', handleRoutePresetChange);
    });
  });

  createEffect(() => {
    const requestedPreset = requestedTokenPreset();
    if (
      requestedPreset !== PULSE_INTELLIGENCE_AGENT_TOKEN_PRESET ||
      appliedRoutePreset() === requestedPreset
    ) {
      return;
    }

    const preset = pulseIntelligenceAgentPreset();
    if (!preset || preset.scopes.length === 0) {
      return;
    }

    applyScopePreset(preset.scopes);
    if (!nameInput().trim()) {
      setNameInput(preset.label);
    }
    setAppliedRoutePreset(requestedPreset);
  });

  createEffect(() => {
    const requestedPreset = requestedTokenPreset();
    if (requestedPreset !== PULSE_INTELLIGENCE_AGENT_TOKEN_PRESET) {
      setFocusedRoutePreset('');
      return;
    }
    if (
      !createSectionReady() ||
      loading() ||
      appliedRoutePreset() !== requestedPreset ||
      focusedRoutePreset() === requestedPreset
    ) {
      return;
    }

    focusCreateSection();
    window.clearTimeout(routeFocusTimer);
    routeFocusTimer = window.setTimeout(() => {
      if (requestedTokenPreset() === requestedPreset) {
        focusCreateSection();
      }
    }, 0);
    setFocusedRoutePreset(requestedPreset);
  });

  createEffect(() => {
    if (!tokensLoaded()) return;
    const activeTokenIds = new Set(tokens().map((token) => token.id));
    const pendingRuntimesByToken = new Map<string, string[]>();

    for (const resource of dockerRuntimeResources()) {
      const tokenId = tokenIdForResource(resource);
      if (!tokenId || activeTokenIds.has(tokenId)) continue;
      if (revokedTokenIdForResource(resource) === tokenId) continue;

      if (!pendingRuntimesByToken.has(tokenId)) {
        pendingRuntimesByToken.set(tokenId, []);
      }
      const runtimeIds = pendingRuntimesByToken.get(tokenId)!;
      const runtimeId = dockerActionIdForResource(resource);
      if (!runtimeIds.includes(runtimeId)) runtimeIds.push(runtimeId);
    }

    pendingRuntimesByToken.forEach((runtimeIds, tokenId) => {
      if (runtimeIds.length > 0) markDockerRuntimesTokenRevoked(tokenId, runtimeIds);
    });

    const pendingAgentsByToken = new Map<string, string[]>();
    for (const resource of agentCapableResources()) {
      const tokenId = tokenIdForResource(resource);
      if (!tokenId || activeTokenIds.has(tokenId)) continue;
      if (revokedTokenIdForResource(resource) === tokenId && tokenRevokedAtForResource(resource)) {
        continue;
      }

      if (!pendingAgentsByToken.has(tokenId)) {
        pendingAgentsByToken.set(tokenId, []);
      }
      const agentIds = pendingAgentsByToken.get(tokenId)!;
      const agentId = agentActionIdForResource(resource);
      if (!agentIds.includes(agentId)) agentIds.push(agentId);
    }

    pendingAgentsByToken.forEach((agentIds, tokenId) => {
      if (agentIds.length > 0) markAgentsTokenRevoked(tokenId, agentIds);
    });
  });

  const applyScopePreset = (scopes: string[]) => {
    setSelectedScopes(Array.from(new Set(scopes)).filter(Boolean));
  };

  const applyFullAccessPreset = () => applyScopePreset([API_TOKEN_WILDCARD_SCOPE]);

  const clearScopes = () => setSelectedScopes([]);

  const toggleScope = (scope: string) => {
    setSelectedScopes((previous) => {
      if (previous.includes(scope)) {
        return previous.filter((value) => value !== scope);
      }
      return [...previous, scope];
    });
  };

  const handleGenerate = async () => {
    if (!canManage()) return;
    if (!hasScopeSelection()) {
      notificationStore.error('Choose a scope preset or custom scope before generating a token.');
      return;
    }
    setIsGenerating(true);
    try {
      const trimmedName = nameInput().trim() || undefined;
      const scopeSelection = [...selectedScopes()].sort();
      const scopePayload = scopeSelection.length > 0 ? scopeSelection : undefined;
      const { token, record } = await SecurityAPI.createToken(trimmedName, scopePayload);

      setTokens((previous) => [record, ...previous]);
      setNewTokenRecord(record);
      setNewTokenValue(token);
      setNameInput('');

      showTokenReveal({
        token,
        record,
        source: 'security',
        note: getAPITokenRevealSettingsNote(),
      });
      notificationStore.success(
        'New API token generated. Copy it below while it is still visible.',
      );
      props.onTokensChanged?.();
    } catch (err) {
      logger.error('Failed to generate API token', err);
      notificationStore.error(getAPITokenGenerateErrorMessage(err));
    } finally {
      setIsGenerating(false);
    }
  };

  const handleDelete = async (record: APITokenRecord) => {
    if (!canManage()) return;
    const dockerUsage = dockerTokenUsage().get(record.id);
    const agentUsage = agentTokenUsage().get(record.id);
    const displayName = getAPITokenDialogName(record);

    const affectedRuntimeIds = dockerUsage ? dockerUsage.items.map((item) => item.id) : [];
    const affectedAgentIds = agentUsage ? agentUsage.items.map((item) => item.id) : [];
    let revokeMessage: string | undefined;
    const messageChunks: string[] = [];

    if (dockerUsage) {
      const preview = dockerUsage.items
        .slice(0, 5)
        .map((item) => item.label)
        .join(', ');
      const extraCount = dockerUsage.items.length - 5;
      const summary = extraCount > 0 ? `${preview}, +${extraCount} more` : preview;
      const label = getAPITokenDockerPodmanUsageCountLabel(dockerUsage.count);
      messageChunks.push(`${label}: ${summary}`);
    }

    if (agentUsage) {
      const preview = agentUsage.items
        .slice(0, 5)
        .map((item) => item.label)
        .join(', ');
      const extraCount = agentUsage.items.length - 5;
      const summary = extraCount > 0 ? `${preview}, +${extraCount} more` : preview;
      const label = agentUsage.count === 1 ? 'agent' : `${agentUsage.count} agents`;
      messageChunks.push(`${label}: ${summary}`);
    }

    if (messageChunks.length > 0) {
      revokeMessage = `Token "${displayName}" was previously used by ${messageChunks.join(' • ')}. Update those agents with a new token.`;
    }

    try {
      await SecurityAPI.deleteToken(record.id);
      setTokens((previous) => previous.filter((token) => token.id !== record.id));
      notificationStore.success(
        revokeMessage ? `Token revoked: ${revokeMessage}` : 'Token revoked',
      );
      props.onTokensChanged?.();

      if (affectedRuntimeIds.length > 0) {
        markDockerRuntimesTokenRevoked(record.id, affectedRuntimeIds);
      }
      if (affectedAgentIds.length > 0) {
        markAgentsTokenRevoked(record.id, affectedAgentIds);
      }

      const current = newTokenRecord();
      if (current && current.id === record.id) {
        setNewTokenValue(null);
        setNewTokenRecord(null);
      }
    } catch (err) {
      logger.error('Failed to revoke API token', err);
      notificationStore.error(getAPITokenRevokeErrorMessage());
    }
  };

  const isRevealActiveForCurrentToken = () => {
    const active = tokenRevealState();
    return newTokenValue() !== null && Boolean(active && active.token === newTokenValue());
  };

  const reopenTokenDialog = () => {
    const token = newTokenValue();
    const record = newTokenRecord();
    if (!token || !record) return;
    showTokenReveal({
      token,
      record,
      source: 'security',
      note: 'Copy this token now. Close the dialog once you have stored it safely.',
    });
  };

  const dismissNewToken = () => {
    setNewTokenValue(null);
    setNewTokenRecord(null);
  };

  const newMonitoringKioskLink = createMemo(() => {
    if (
      !newTokenValue() ||
      newTokenRecord()?.scopes?.length !== 1 ||
      newTokenRecord()?.scopes?.[0] !== MONITORING_READ_SCOPE
    ) {
      return null;
    }
    return `${getPulseBaseUrl()}/?token=${newTokenValue()}&kiosk=1`;
  });

  const copyNewMonitoringKioskLink = async () => {
    const link = newMonitoringKioskLink();
    if (!link) return;
    await navigator.clipboard.writeText(link);
    notificationStore.success('Link copied to clipboard');
  };

  return {
    API_SCOPE_LABELS,
    API_TOKEN_SCOPES_DOC_URL,
    agentTokenUsage,
    applyFullAccessPreset,
    applyScopePreset,
    canManage,
    clearScopes,
    copyNewMonitoringKioskLink,
    createHighlight,
    dismissNewToken,
    dockerTokenUsage,
    focusCreateSection,
    formatRelativeTime,
    handleDelete,
    handleGenerate,
    hasWildcardTokens,
    hasScopeSelection,
    isFullAccessSelected,
    isGenerating,
    isRevealActiveForCurrentToken,
    loading,
    nameInput,
    newMonitoringKioskLink,
    newTokenRecord,
    newTokenValue,
    reopenTokenDialog,
    scopedTokenCount,
    scopeGroups,
    scopePresets,
    selectedScopes,
    setCreateSectionRef,
    setNameInput,
    sortedTokens,
    tokenHint: getAPITokenHint,
    toggleScope,
    totalTokens,
    wildcardCount,
    presetMatchesSelection: (presetScopes: string[]) =>
      matchesScopePreset(selectedScopes(), presetScopes),
    canGenerateToken,
  };
};
