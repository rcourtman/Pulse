import {
  Component,
  Show,
  createSignal,
  onMount,
  onCleanup,
  For,
  createMemo,
  createEffect,
} from 'solid-js';
import { unwrap } from 'solid-js/store';
import SendIcon from 'lucide-solid/icons/send';
import SquareIcon from 'lucide-solid/icons/square';
import ClockIcon from 'lucide-solid/icons/clock';
import PencilIcon from 'lucide-solid/icons/pencil';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import XIcon from 'lucide-solid/icons/x';
import { AIAPI } from '@/api/ai';
import { AIChatAPI, type ChatSession, type ChatSessionHandoffSummary } from '@/api/aiChat';
import { SearchField } from '@/components/shared/SearchField';
import { notificationStore } from '@/stores/notifications';
import { aiChatStore, type AIChatContext } from '@/stores/aiChat';
import {
  aiRuntimeModels,
  aiRuntimeModelsError,
  aiRuntimeModelsLoading,
  aiRuntimeSettings,
  loadAIRuntimeModels,
  loadAIRuntimeSettings,
  syncAIRuntimeSettings,
} from '@/stores/aiRuntimeState';
import { logger } from '@/utils/logger';
import {
  AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL,
  AI_CHAT_COLLAPSE_TITLE,
  AI_CHAT_CLOSE_LABEL,
  AI_CHAT_CONTROL_MODE_LABEL,
  AI_CHAT_CONTROL_MODE_MENU_LABEL,
  AI_CHAT_DISCOVERY_HINT_BODY,
  AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL,
  AI_CHAT_DISCOVERY_HINT_TITLE,
  AI_CHAT_DRAWER_TITLE,
  AI_CHAT_INPUT_PLACEHOLDER,
  AI_CHAT_LAST_TURN_USAGE_LABEL,
  AI_CHAT_NEW_SESSION_BUTTON_TITLE,
  AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL,
  AI_CHAT_NEW_SESSION_MENU_LABEL,
  AI_CHAT_NEW_SESSION_SHORT_LABEL,
  AI_CHAT_PROVIDER_READINESS_RETRY_LABEL,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL,
  AI_CHAT_SESSION_MENU_TITLE,
  AI_CHAT_SESSION_EMPTY_STATE,
  AI_CHAT_SESSION_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_EMPTY_STATE,
  AI_CHAT_SESSION_SEARCH_ERROR_STATE,
  AI_CHAT_SESSION_SEARCH_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_PLACEHOLDER,
  AI_CHAT_SESSION_SEARCH_TITLE,
  AI_CHAT_SWITCH_TO_APPROVAL_LABEL,
  getAIChatProviderReadinessPresentation,
} from '@/utils/aiChatPresentation';
import {
  getAIChatControlLevelPresentation,
  normalizeAIControlLevel,
  type AIControlLevel,
} from '@/utils/aiControlLevelPresentation';
import {
  formatAIModelRouteLabel,
  getAIProviderDisplayName,
  getProviderFromModelId,
} from '@/utils/aiProviderPresentation';
import { getCachedUnifiedResources } from '@/hooks/useUnifiedResources';
import type { ModelInfo as RuntimeModelInfo } from '@/types/ai';
import type { Resource } from '@/types/resource';
import { isAppContainerDiscoveryResourceType } from '@/utils/discoveryTarget';
import {
  getActionableAgentIdFromResource,
  isAgentFacetInfrastructureResource,
} from '@/utils/agentResources';
import { normalizeChatMentionKeyPart } from '@/utils/chatIdentifiers';
import { getGlobalWebSocketStore } from '@/stores/websocket-global';
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { useChat, type QueuedFollowUp, type SendMessageOptions } from './hooks/useChat';
import { ChatMessages } from './ChatMessages';
import { ModelSelector } from './ModelSelector';
import { MentionAutocomplete, type MentionResource } from './MentionAutocomplete';
import { getAssistantActiveTurnStatus } from './activeTurnStatus';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
} from './types';
import { formatIdentifierLabel } from '@/utils/textPresentation';

const MODEL_SESSION_STORAGE_KEY = 'pulse:ai_chat_models_by_session';
const MODEL_RECENT_STORAGE_KEY = 'pulse:ai_chat_recent_models';
const PROMPT_HISTORY_STORAGE_KEY = 'pulse:ai_chat_prompt_history';
const DEFAULT_SESSION_KEY = '__default__';
const AI_CHAT_MIN_DOCKED_VIEWPORT_WIDTH = 1200;
const AI_CHAT_PROMPT_HISTORY_LIMIT = 100;
const AI_CHAT_RECENT_MODEL_LIMIT = 8;
const AI_CHAT_SESSION_SEARCH_DEBOUNCE_MS = 150;
const AI_CHAT_SESSION_SEARCH_LIMIT = 30;
const STRUCTURED_PATROL_CONTEXT_TARGETS = new Set(['patrol-configuration', 'patrol-run']);
const STRUCTURED_RESOURCE_CONTEXT_HANDOFF_KINDS = new Set(['resource_context']);

type ChatProviderReadinessStatus = 'idle' | 'checking' | 'ready' | 'error';

interface ChatProviderReadinessState {
  status: ChatProviderReadinessStatus;
  provider: string;
  model?: string;
  message?: string;
  summary?: string;
  recommendation?: string;
  action?: string;
}

interface PromptHistoryEntry {
  prompt: string;
  mentions: MentionResource[];
}

interface ComposerDraftStash {
  input: string;
  mentions: MentionResource[];
  cursorStart: number;
  cursorEnd: number;
  editingQueuedFollowUp: QueuedFollowUp | null;
}

interface AssistantUsageSummary {
  label: string;
  title: string;
}

interface AIChatProps {
  onClose: () => void;
}

let stashedComposerDraft: ComposerDraftStash | null = null;

export const resetAIChatComposerDraftStashForTests = () => {
  stashedComposerDraft = null;
};

const compactText = (items: Array<string | undefined>): string[] =>
  items.filter((item): item is string => typeof item === 'string' && item.trim().length > 0);

const pluralizeCount = (count: number, singular: string, plural: string) =>
  `${count} ${count === 1 ? singular : plural}`;

const tokenNumberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });

const normalizeAssistantTokenCount = (value: number | undefined): number => {
  if (!Number.isFinite(value) || !value || value < 0) return 0;
  return Math.floor(value);
};

const formatAssistantTokenCount = (count: number): string => tokenNumberFormat.format(count);

const getAssistantUsageSummary = (message: ChatMessage): AssistantUsageSummary | null => {
  if (message.role !== 'assistant' || message.isStreaming) return null;
  const input = normalizeAssistantTokenCount(message.tokens?.input);
  const output = normalizeAssistantTokenCount(message.tokens?.output);
  if (output <= 0) return null;

  const total = input + output;
  const totalLabel = `${formatAssistantTokenCount(total)} ${total === 1 ? 'token' : 'tokens'}`;
  const titleDetail = [
    `${formatAssistantTokenCount(total)} total`,
    `${formatAssistantTokenCount(input)} input`,
    `${formatAssistantTokenCount(output)} output`,
  ].join(', ');

  return {
    label: `Last turn: ${totalLabel}`,
    title: `${AI_CHAT_LAST_TURN_USAGE_LABEL}: ${titleDetail}`,
  };
};

const normalizePromptHistoryEntry = (value: unknown): PromptHistoryEntry | null => {
  if (!value || typeof value !== 'object') return null;
  const record = value as Record<string, unknown>;
  const prompt = typeof record.prompt === 'string' ? record.prompt : '';
  if (!prompt.trim()) return null;
  const mentions = Array.isArray(record.mentions)
    ? record.mentions
        .map((mention): MentionResource | null => {
          if (!mention || typeof mention !== 'object') return null;
          const mentionRecord = mention as Record<string, unknown>;
          const id = typeof mentionRecord.id === 'string' ? mentionRecord.id : '';
          const label = typeof mentionRecord.label === 'string' ? mentionRecord.label : '';
          const type = mentionRecord.type;
          if (
            !id ||
            !label ||
            !(
              type === 'vm' ||
              type === 'system-container' ||
              type === 'app-container' ||
              type === 'agent' ||
              type === 'storage'
            )
          ) {
            return null;
          }
          return {
            id,
            label,
            type,
            node: typeof mentionRecord.node === 'string' ? mentionRecord.node : undefined,
            status: typeof mentionRecord.status === 'string' ? mentionRecord.status : undefined,
          };
        })
        .filter((mention): mention is MentionResource => mention !== null)
    : [];
  return { prompt, mentions };
};

const normalizeComparableModelKey = (modelId: string): string => {
  const trimmed = modelId.trim().toLowerCase();
  if (!trimmed) return '';
  const providerSeparator = trimmed.indexOf(':');
  const withoutProvider = providerSeparator > 0 ? trimmed.slice(providerSeparator + 1) : trimmed;
  const routeTail = withoutProvider.split('/').pop() || withoutProvider;
  return routeTail.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
};

const resolveRuntimeModelProvider = (
  model: Pick<RuntimeModelInfo, 'id'> & Partial<Pick<RuntimeModelInfo, 'provider'>>,
): string => model.provider?.trim() || getProviderFromModelId(model.id);

const findProviderReadinessAlternative = (args: {
  avoidProviders?: string[];
  avoidModelIds?: string[];
  configuredProviders?: string[];
  models: RuntimeModelInfo[];
  selectedModel: string;
  selectedProvider: string;
}): ModelRouteRecoveryOption | null => {
  const selectedKey = normalizeComparableModelKey(args.selectedModel);
  const selectedProvider = args.selectedProvider.trim();
  if (!selectedKey || !selectedProvider) return null;

  const configuredProviders = (args.configuredProviders ?? [])
    .map((provider) => provider.trim())
    .filter(Boolean);
  const configuredProviderOrder = new Map(
    configuredProviders.map((provider, index) => [provider, index]),
  );
  const avoidProviders = new Set((args.avoidProviders ?? []).map((provider) => provider.trim()));
  const avoidModelIds = new Set((args.avoidModelIds ?? []).map((modelId) => modelId.trim()));

  const sortedCandidates = args.models
    .map((model) => {
      const provider = resolveRuntimeModelProvider(model).trim();
      return { model, provider };
    })
    .filter(({ model, provider }) => {
      if (!provider || provider === selectedProvider || model.id === args.selectedModel) {
        return false;
      }
      if (avoidProviders.has(provider) || avoidModelIds.has(model.id)) {
        return false;
      }
      if (configuredProviderOrder.size > 0 && !configuredProviderOrder.has(provider)) {
        return false;
      }
      return true;
    })
    .sort((left, right) => {
      const leftProviderOrder =
        configuredProviderOrder.get(left.provider) ?? Number.MAX_SAFE_INTEGER;
      const rightProviderOrder =
        configuredProviderOrder.get(right.provider) ?? Number.MAX_SAFE_INTEGER;
      if (leftProviderOrder !== rightProviderOrder) return leftProviderOrder - rightProviderOrder;
      if (Boolean(left.model.notable) !== Boolean(right.model.notable)) {
        return right.model.notable ? 1 : -1;
      }
      if (Boolean(left.model.is_default) !== Boolean(right.model.is_default)) {
        return right.model.is_default ? 1 : -1;
      }
      return formatAIModelRouteLabel(left.model).localeCompare(
        formatAIModelRouteLabel(right.model),
      );
    });

  const candidate =
    sortedCandidates.find(({ model }) => normalizeComparableModelKey(model.id) === selectedKey) ||
    sortedCandidates[0];
  if (!candidate) return null;

  return {
    id: candidate.model.id,
    label: formatAIModelRouteLabel(candidate.model),
    provider: candidate.provider,
    providerLabel: getAIProviderDisplayName(candidate.provider),
  };
};

const shouldShowStructuredPatrolContext = (targetType: string | undefined) =>
  STRUCTURED_PATROL_CONTEXT_TARGETS.has(targetType ?? '');

const shouldShowStructuredBriefingContext = (context: AIChatContext) =>
  shouldShowStructuredPatrolContext(context.targetType) ||
  STRUCTURED_RESOURCE_CONTEXT_HANDOFF_KINDS.has(context.handoffMetadata?.kind ?? '');

const isPatrolFindingSessionHandoff = (summary: ChatSessionHandoffSummary) =>
  summary.kind === 'patrol_finding' || Boolean(summary.finding_id);

const isPatrolRunSessionHandoff = (summary: ChatSessionHandoffSummary) =>
  summary.kind === 'patrol_run' || Boolean(summary.run_id);

const isPatrolAssessmentSessionHandoff = (summary: ChatSessionHandoffSummary) =>
  summary.kind === 'patrol_assessment';

const isPatrolConfigurationFailureSessionHandoff = (summary: ChatSessionHandoffSummary) =>
  summary.kind === 'patrol_configuration_failure';

const isPatrolSessionHandoff = (summary: ChatSessionHandoffSummary) =>
  isPatrolFindingSessionHandoff(summary) ||
  isPatrolRunSessionHandoff(summary) ||
  isPatrolAssessmentSessionHandoff(summary) ||
  isPatrolConfigurationFailureSessionHandoff(summary);

const getSessionHandoffSourceLabel = (summary: ChatSessionHandoffSummary) =>
  isPatrolSessionHandoff(summary) ? 'Pulse Patrol' : 'Pulse Assistant';

const formatSessionHandoffResourceLabel = (summary: ChatSessionHandoffSummary) => {
  const resource = summary.primary_resource;
  const label = resource?.name?.trim() || resource?.id?.trim() || '';
  if (label) return label;
  return formatIdentifierLabel(resource?.type, { fallback: '' });
};

const formatSessionHandoffResourceDetail = (summary: ChatSessionHandoffSummary) => {
  const label = formatSessionHandoffResourceLabel(summary);
  const node = summary.primary_resource?.node?.trim();
  if (label && node) return `${label} on ${node}`;
  return label;
};

const formatSessionHandoffStatus = (summary: ChatSessionHandoffSummary) => {
  if (isPatrolRunSessionHandoff(summary)) {
    return compactText([
      summary.run_status ? `run ${formatIdentifierLabel(summary.run_status)}` : undefined,
      summary.runtime_failure ? 'runtime issue' : undefined,
    ]).join(' · ');
  }
  if (isPatrolConfigurationFailureSessionHandoff(summary)) {
    return summary.runtime_failure ? 'runtime issue' : '';
  }
  return compactText([
    summary.last_known_approval_status
      ? `approval ${formatIdentifierLabel(summary.last_known_approval_status)}`
      : undefined,
    summary.last_known_action_state
      ? `action ${formatIdentifierLabel(summary.last_known_action_state)}`
      : undefined,
    summary.last_known_action_risk
      ? `${formatIdentifierLabel(summary.last_known_action_risk)} risk`
      : undefined,
  ]).join(' · ');
};

const getSessionHandoffBadgeLabel = (summary: ChatSessionHandoffSummary) => {
  if (summary.requires_approval) return 'Approval required';
  if ((summary.action_count ?? 0) > 0) return 'Action context';
  if (isPatrolRunSessionHandoff(summary)) {
    return summary.runtime_failure ? 'Runtime issue' : 'Run context';
  }
  if (isPatrolConfigurationFailureSessionHandoff(summary)) return 'Runtime issue';
  if (isPatrolAssessmentSessionHandoff(summary)) return 'Assessment context';
  return summary.has_model_context ? 'Context attached' : 'Scoped handoff';
};

const buildSessionHandoffContext = (session?: ChatSession): AIChatContext | undefined => {
  const summary = session?.handoff_summary;
  if (!summary) return undefined;

  const sourceLabel = getSessionHandoffSourceLabel(summary);
  const resourceLabel = formatSessionHandoffResourceLabel(summary);
  const resourceDetail = formatSessionHandoffResourceDetail(summary);
  const statusLabel = formatSessionHandoffStatus(summary);
  const actionCount = summary.action_count ?? 0;
  const resourceCount = summary.resource_count ?? 0;
  const findingId = summary.finding_id?.trim() || undefined;
  const runId = summary.run_id?.trim() || undefined;
  const runType = summary.run_type?.trim() || undefined;
  const runStatus = summary.run_status?.trim() || undefined;
  const isPatrolFinding = isPatrolFindingSessionHandoff(summary);
  const isPatrolRun = isPatrolRunSessionHandoff(summary);
  const isPatrolAssessment = isPatrolAssessmentSessionHandoff(summary);
  const isPatrolConfigurationFailure = isPatrolConfigurationFailureSessionHandoff(summary);
  const title = isPatrolRun
    ? runId
      ? `Patrol run ${runId}`
      : 'Patrol run handoff'
    : isPatrolAssessment
      ? 'Patrol assessment handoff'
      : isPatrolConfigurationFailure
        ? 'Patrol configuration failure'
        : isPatrolFinding
          ? resourceLabel
            ? `Patrol finding on ${resourceLabel}`
            : findingId
              ? `Patrol finding ${findingId}`
              : 'Patrol finding handoff'
          : resourceLabel
            ? `Assistant handoff for ${resourceLabel}`
            : 'Assistant handoff';

  return {
    targetType: isPatrolRun
      ? 'patrol-run'
      : isPatrolAssessment
        ? 'patrol-assessment'
        : isPatrolConfigurationFailure
          ? 'patrol-configuration'
          : summary.primary_resource?.type,
    targetId: isPatrolRun
      ? runId
      : isPatrolAssessment
        ? 'pulse-patrol-assessment'
        : isPatrolConfigurationFailure
          ? 'pulse-patrol-configuration'
          : summary.primary_resource?.id,
    context: {
      source: 'session_handoff_summary',
      kind: summary.kind,
      findingId,
      runId,
      runType,
      runStatus,
      runtimeFailure: summary.runtime_failure ?? false,
      hasModelContext: summary.has_model_context,
      resourceCount,
      actionCount,
      requiresApproval: summary.requires_approval ?? false,
      lastKnownApprovalStatus: summary.last_known_approval_status,
      lastKnownActionState: summary.last_known_action_state,
      lastKnownActionRisk: summary.last_known_action_risk,
      updatedAt: summary.updated_at,
    },
    findingId,
    autonomousMode: false,
    briefing: {
      sourceLabel,
      title,
      subject:
        isPatrolRun && runId
          ? `Run ${runId}`
          : isPatrolAssessment
            ? 'Current Patrol assessment'
            : isPatrolConfigurationFailure
              ? 'Patrol configuration'
              : findingId
                ? `Finding ${findingId}`
                : undefined,
      statusLabel:
        statusLabel ||
        (summary.requires_approval ? 'approval required' : undefined) ||
        (isPatrolRun && summary.runtime_failure ? 'runtime issue' : undefined),
      detailLines: compactText([
        isPatrolRun && runType ? `Run type: ${runType}` : undefined,
        resourceDetail ? `Resource: ${resourceDetail}` : undefined,
        resourceCount > 1
          ? pluralizeCount(resourceCount, 'linked resource', 'linked resources')
          : undefined,
        actionCount > 0
          ? pluralizeCount(actionCount, 'governed action', 'governed actions')
          : undefined,
        summary.requires_approval ? 'Approval required before action' : undefined,
      ]),
      actionLabel: summary.requires_approval
        ? 'Approval required'
        : isPatrolConfigurationFailure
          ? 'Review Patrol configuration issue'
          : isPatrolAssessment
            ? 'Review Patrol assessment'
            : isPatrolRun && summary.runtime_failure
              ? 'Review Patrol runtime issue'
              : isPatrolRun
                ? 'Review Patrol run'
                : actionCount > 0
                  ? 'Governed action context'
                  : undefined,
      commandSummary: statusLabel
        ? isPatrolRun
          ? `Run state: ${statusLabel}`
          : `Last known state: ${statusLabel}`
        : undefined,
      safetyNote: isPatrolRun
        ? 'Patrol run context is review-only; actions still require governed approval.'
        : actionCount > 0
          ? 'Detailed command payloads stay in governed approval context.'
          : undefined,
    },
  };
};

/**
 * AIChat - Main chat panel component.
 *
 * Provides a terminal-like chat experience with clear status indicators,
 * session management, and streaming response display.
 */
export const AIChat: Component<AIChatProps> = (props) => {
  // UI state - use store's isOpenSignal for reactivity
  const isOpen = aiChatStore.isOpenSignal;
  const { width } = useBreakpoint();
  const [input, setInput] = createSignal('');
  const [editingQueuedFollowUp, setEditingQueuedFollowUp] = createSignal<QueuedFollowUp | null>(
    null,
  );
  const [interruptArmed, setInterruptArmed] = createSignal(false);
  const [promptHistory, setPromptHistory] = createSignal<PromptHistoryEntry[]>([]);
  const [promptHistoryIndex, setPromptHistoryIndex] = createSignal(-1);
  const [savedPromptDraft, setSavedPromptDraft] = createSignal<PromptHistoryEntry | null>(null);
  const [sessions, setSessions] = createSignal<ChatSession[]>([]);
  const [showSessions, setShowSessions] = createSignal(false);
  const [sessionRefreshLoading, setSessionRefreshLoading] = createSignal(false);
  const [sessionSearchQuery, setSessionSearchQuery] = createSignal('');
  const [sessionSearchResults, setSessionSearchResults] = createSignal<ChatSession[] | null>(null);
  const [sessionSearchLoading, setSessionSearchLoading] = createSignal(false);
  const [sessionSearchError, setSessionSearchError] = createSignal('');
  const [sessionDropdownPosition, setSessionDropdownPosition] = createSignal({ top: 0, right: 0 });
  let sessionButtonRef: HTMLButtonElement | undefined;
  let sessionSearchInputRef: HTMLInputElement | undefined;
  const sessionOptionRefs = new Map<string, HTMLButtonElement>();
  let sessionSearchRequestId = 0;
  const [modelSelectorOpenRequest, setModelSelectorOpenRequest] = createSignal(0);
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');
  const [providerReadiness, setProviderReadiness] = createSignal<ChatProviderReadinessState>({
    status: 'idle',
    provider: '',
  });
  const [providerReadinessRetryNonce, setProviderReadinessRetryNonce] = createSignal(0);
  const [controlLevel, setControlLevel] = createSignal<AIControlLevel>('read_only');
  const [showControlMenu, setShowControlMenu] = createSignal(false);
  const [controlSaving, setControlSaving] = createSignal(false);
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal<boolean | null>(null); // null = loading
  const [discoveryHintDismissed, setDiscoveryHintDismissed] = createSignal(false);
  const [autonomousBannerDismissed, setAutonomousBannerDismissed] = createSignal(false);
  const wsStore = getGlobalWebSocketStore();
  const allResources = createMemo<Resource[]>(() => {
    const liveResources = wsStore.state.resources ?? [];
    if (Array.isArray(liveResources) && liveResources.length > 0) {
      return liveResources;
    }
    return getCachedUnifiedResources({ cacheKey: 'all-resources' });
  });
  const byType = (type: Resource['type']) =>
    allResources().filter((resource) => resource.type === type);

  // @ mention autocomplete state
  const [mentionActive, setMentionActive] = createSignal(false);
  const [mentionQuery, setMentionQuery] = createSignal('');
  const [mentionStartIndex, setMentionStartIndex] = createSignal(0);
  const [mentionResources, setMentionResources] = createSignal<MentionResource[]>([]);
  const [accumulatedMentions, setAccumulatedMentions] = createSignal<MentionResource[]>([]);
  let textareaRef: HTMLTextAreaElement | undefined;
  let interruptArmTimeout: ReturnType<typeof setTimeout> | undefined;
  let composerSubmitDispatchLocked = false;

  const focusComposer = () => {
    queueMicrotask(() => {
      textareaRef?.focus();
    });
  };

  const clearInterruptArm = () => {
    if (interruptArmTimeout) {
      clearTimeout(interruptArmTimeout);
      interruptArmTimeout = undefined;
    }
    setInterruptArmed(false);
  };

  const armKeyboardInterrupt = () => {
    clearInterruptArm();
    setInterruptArmed(true);
    interruptArmTimeout = setTimeout(() => {
      interruptArmTimeout = undefined;
      setInterruptArmed(false);
    }, 5000);
  };

  const resizeTextarea = () => {
    if (!textareaRef) return;
    textareaRef.style.height = 'auto';
    textareaRef.style.height = `${Math.min(textareaRef.scrollHeight, 160)}px`;
  };

  const cloneMentions = (mentions: MentionResource[]) =>
    mentions.map((mention) => ({ ...mention }));

  const cloneSendMessageOptions = (
    sendOptions?: SendMessageOptions,
  ): SendMessageOptions | undefined => {
    if (!sendOptions) return undefined;
    return {
      ...sendOptions,
      handoffResources: sendOptions.handoffResources?.map((resource) => ({ ...resource })),
      handoffActions: sendOptions.handoffActions?.map((action) => ({ ...action })),
      handoffMetadata: sendOptions.handoffMetadata ? { ...sendOptions.handoffMetadata } : undefined,
    };
  };

  const cloneQueuedFollowUp = (queued: QueuedFollowUp): QueuedFollowUp => ({
    ...queued,
    mentions: queued.mentions?.map((mention) => ({ ...mention })),
    sendOptions: cloneSendMessageOptions(queued.sendOptions),
    timestamp: new Date(queued.timestamp),
  });

  const loadPromptHistory = (): PromptHistoryEntry[] => {
    try {
      const raw = localStorage.getItem(PROMPT_HISTORY_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : [];
      if (!Array.isArray(parsed)) return [];
      return parsed
        .map(normalizePromptHistoryEntry)
        .filter((entry): entry is PromptHistoryEntry => entry !== null)
        .slice(0, AI_CHAT_PROMPT_HISTORY_LIMIT);
    } catch (error) {
      logger.warn('[AIChat] Failed to read prompt history:', error);
      return [];
    }
  };

  const persistPromptHistory = (history: PromptHistoryEntry[]) => {
    try {
      localStorage.setItem(PROMPT_HISTORY_STORAGE_KEY, JSON.stringify(history));
    } catch (error) {
      logger.warn('[AIChat] Failed to persist prompt history:', error);
    }
  };

  const promptHistoryEntriesEqual = (a: PromptHistoryEntry, b: PromptHistoryEntry) => {
    if (a.prompt.trim() !== b.prompt.trim()) return false;
    if (a.mentions.length !== b.mentions.length) return false;
    return a.mentions.every((mention, index) => {
      const other = b.mentions[index];
      return (
        other &&
        mention.id === other.id &&
        mention.label === other.label &&
        mention.type === other.type &&
        mention.node === other.node
      );
    });
  };

  const addPromptHistoryEntry = (prompt: string, mentions: MentionResource[]) => {
    const trimmedPrompt = prompt.trim();
    if (!trimmedPrompt) return;

    const entry: PromptHistoryEntry = {
      prompt: trimmedPrompt,
      mentions: cloneMentions(mentions),
    };

    setPromptHistory((prev) => {
      if (prev[0] && promptHistoryEntriesEqual(prev[0], entry)) return prev;
      const next = [entry, ...prev].slice(0, AI_CHAT_PROMPT_HISTORY_LIMIT);
      persistPromptHistory(next);
      return next;
    });
  };

  const resetPromptHistoryNavigation = () => {
    setPromptHistoryIndex(-1);
    setSavedPromptDraft(null);
  };

  const stashComposerDraftForRemount = () => {
    const text = input();
    const mentions = accumulatedMentions();
    const queuedDraft = editingQueuedFollowUp();
    if (!text.trim() && mentions.length === 0 && !queuedDraft) {
      stashedComposerDraft = null;
      return;
    }

    const fallbackCursor = text.length;
    stashedComposerDraft = {
      input: text,
      mentions: cloneMentions(mentions),
      cursorStart: textareaRef?.selectionStart ?? fallbackCursor,
      cursorEnd: textareaRef?.selectionEnd ?? fallbackCursor,
      editingQueuedFollowUp: queuedDraft ? cloneQueuedFollowUp(queuedDraft) : null,
    };
  };

  const restoreStashedComposerDraft = () => {
    const draft = stashedComposerDraft;
    stashedComposerDraft = null;
    if (!draft) return;
    if (input().trim() || accumulatedMentions().length > 0 || editingQueuedFollowUp()) return;

    setInput(draft.input);
    setAccumulatedMentions(cloneMentions(draft.mentions));
    setEditingQueuedFollowUp(
      draft.editingQueuedFollowUp ? cloneQueuedFollowUp(draft.editingQueuedFollowUp) : null,
    );
    setMentionActive(false);
    resetPromptHistoryNavigation();
    queueMicrotask(() => {
      resizeTextarea();
      textareaRef?.focus();
      const start = Math.min(draft.cursorStart, draft.input.length);
      const end = Math.min(draft.cursorEnd, draft.input.length);
      textareaRef?.setSelectionRange(start, end);
    });
  };

  const applyPromptHistoryEntry = (entry: PromptHistoryEntry, cursor: 'start' | 'end') => {
    setInput(entry.prompt);
    setAccumulatedMentions(cloneMentions(entry.mentions));
    setMentionActive(false);
    queueMicrotask(() => {
      resizeTextarea();
      textareaRef?.focus();
      const nextPosition = cursor === 'start' ? 0 : entry.prompt.length;
      textareaRef?.setSelectionRange(nextPosition, nextPosition);
    });
  };

  const canNavigatePromptHistory = (direction: 'up' | 'down') => {
    if (!textareaRef) return false;
    if (textareaRef.selectionStart !== textareaRef.selectionEnd) return false;
    const text = input();
    const cursor = textareaRef.selectionStart ?? text.length;
    const inHistory = promptHistoryIndex() >= 0;
    if (inHistory) return cursor === 0 || cursor === text.length;
    if (direction === 'up') return cursor === 0;
    return false;
  };

  const navigatePromptHistory = (direction: 'up' | 'down') => {
    const entries = promptHistory();
    if (entries.length === 0) return false;

    const currentIndex = promptHistoryIndex();
    if (direction === 'up') {
      const nextIndex = currentIndex < 0 ? 0 : Math.min(currentIndex + 1, entries.length - 1);
      if (nextIndex === currentIndex) return false;
      if (currentIndex < 0) {
        setSavedPromptDraft({
          prompt: input(),
          mentions: cloneMentions(accumulatedMentions()),
        });
      }
      setPromptHistoryIndex(nextIndex);
      applyPromptHistoryEntry(entries[nextIndex], 'start');
      return true;
    }

    if (currentIndex > 0) {
      const nextIndex = currentIndex - 1;
      setPromptHistoryIndex(nextIndex);
      applyPromptHistoryEntry(entries[nextIndex], 'end');
      return true;
    }

    if (currentIndex === 0) {
      const saved = savedPromptDraft();
      setPromptHistoryIndex(-1);
      setSavedPromptDraft(null);
      applyPromptHistoryEntry(saved || { prompt: '', mentions: [] }, 'end');
      return true;
    }

    return false;
  };

  const loadModelSelections = (): Record<string, string> => {
    try {
      const raw = localStorage.getItem(MODEL_SESSION_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : {};
      if (!parsed || typeof parsed !== 'object') return {};

      const selections: Record<string, string> = {};
      for (const [key, value] of Object.entries(parsed as Record<string, unknown>)) {
        const sessionId = key.trim();
        const modelId = typeof value === 'string' ? value.trim() : '';
        if (!sessionId || sessionId === DEFAULT_SESSION_KEY || !modelId) continue;
        selections[sessionId] = modelId;
      }

      if (raw && JSON.stringify(parsed) !== JSON.stringify(selections)) {
        if (Object.keys(selections).length > 0) {
          localStorage.setItem(MODEL_SESSION_STORAGE_KEY, JSON.stringify(selections));
        } else {
          localStorage.removeItem(MODEL_SESSION_STORAGE_KEY);
        }
      }

      return selections;
    } catch (error) {
      logger.warn('[AIChat] Failed to read stored models:', error);
      return {};
    }
  };

  const persistModelSelections = (selections: Record<string, string>) => {
    try {
      localStorage.setItem(MODEL_SESSION_STORAGE_KEY, JSON.stringify(selections));
    } catch (error) {
      logger.warn('[AIChat] Failed to persist model selections:', error);
    }
  };

  const initialModelSelections = loadModelSelections();
  const [modelSelections, setModelSelections] =
    createSignal<Record<string, string>>(initialModelSelections);

  const loadRecentModelIds = (): string[] => {
    try {
      const raw = localStorage.getItem(MODEL_RECENT_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : [];
      if (!Array.isArray(parsed)) return [];
      const seen = new Set<string>();
      const recentModelIds: string[] = [];
      for (const value of parsed) {
        const modelId = typeof value === 'string' ? value.trim() : '';
        if (!modelId || !modelId.includes(':') || seen.has(modelId)) continue;
        seen.add(modelId);
        recentModelIds.push(modelId);
        if (recentModelIds.length >= AI_CHAT_RECENT_MODEL_LIMIT) break;
      }
      return recentModelIds;
    } catch (error) {
      logger.warn('[AIChat] Failed to read recent models:', error);
      return [];
    }
  };

  const persistRecentModelIds = (modelIds: string[]) => {
    try {
      if (modelIds.length > 0) {
        localStorage.setItem(MODEL_RECENT_STORAGE_KEY, JSON.stringify(modelIds));
      } else {
        localStorage.removeItem(MODEL_RECENT_STORAGE_KEY);
      }
    } catch (error) {
      logger.warn('[AIChat] Failed to persist recent models:', error);
    }
  };

  const [recentModelIds, setRecentModelIds] = createSignal<string[]>(loadRecentModelIds());

  const rememberRecentModel = (modelId: string) => {
    const normalizedModelId = modelId.trim();
    if (!normalizedModelId || !normalizedModelId.includes(':')) return;
    setRecentModelIds((prev) => {
      const next = [
        normalizedModelId,
        ...prev.filter((candidate) => candidate !== normalizedModelId),
      ].slice(0, AI_CHAT_RECENT_MODEL_LIMIT);
      persistRecentModelIds(next);
      return next;
    });
  };

  const getStoredModel = (sessionId: string) => {
    const key = sessionId.trim();
    if (!key) return '';
    return modelSelections()[key] || '';
  };

  const updateStoredModel = (sessionId: string, modelId: string) => {
    const key = sessionId.trim();
    if (!key) return;
    setModelSelections((prev) => {
      const next = { ...prev };
      const normalizedModel = modelId.trim();
      if (normalizedModel) {
        next[key] = normalizedModel;
      } else {
        delete next[key];
      }
      persistModelSelections(next);
      return next;
    });
  };

  const refreshSessions = async () => {
    const sessionList = await AIChatAPI.listSessions({ limit: AI_CHAT_SESSION_SEARCH_LIMIT });
    setSessions(sessionList);
  };

  const normalizedSessionSearchQuery = createMemo(() => sessionSearchQuery().trim());
  const sessionPickerSessions = createMemo(() =>
    normalizedSessionSearchQuery() ? (sessionSearchResults() ?? []) : sessions(),
  );
  const sessionPickerEmptyText = createMemo(() => {
    if (sessionSearchError()) return sessionSearchError();
    if (!normalizedSessionSearchQuery()) {
      return sessionRefreshLoading() ? AI_CHAT_SESSION_LOADING_STATE : AI_CHAT_SESSION_EMPTY_STATE;
    }
    if (sessionSearchLoading()) return AI_CHAT_SESSION_SEARCH_LOADING_STATE;
    return AI_CHAT_SESSION_SEARCH_EMPTY_STATE;
  });
  const sessionPickerLoadingText = createMemo(() => {
    if (normalizedSessionSearchQuery()) {
      return sessionSearchLoading() ? AI_CHAT_SESSION_SEARCH_LOADING_STATE : '';
    }
    return sessionRefreshLoading() ? AI_CHAT_SESSION_LOADING_STATE : '';
  });
  const resetSessionSearch = () => {
    sessionSearchRequestId += 1;
    setSessionSearchQuery('');
    setSessionSearchResults(null);
    setSessionSearchLoading(false);
    setSessionSearchError('');
  };
  const focusSessionSearch = () => {
    queueMicrotask(() => sessionSearchInputRef?.focus());
  };

  const findKnownSession = (sessionId: string) =>
    sessions().find((candidate) => candidate.id === sessionId) ||
    sessionSearchResults()?.find((candidate) => candidate.id === sessionId);

  createEffect(() => {
    const validSessionIds = new Set(sessionPickerSessions().map((session) => session.id));
    for (const sessionId of sessionOptionRefs.keys()) {
      if (!validSessionIds.has(sessionId)) {
        sessionOptionRefs.delete(sessionId);
      }
    }
  });

  const focusSessionOptionAtIndex = (index: number) => {
    const session = sessionPickerSessions()[index];
    if (!session) return false;
    sessionOptionRefs.get(session.id)?.focus();
    return true;
  };

  const focusSessionOptionRelativeTo = (sessionId: string, offset: number) => {
    const sessionList = sessionPickerSessions();
    const currentIndex = sessionList.findIndex((session) => session.id === sessionId);
    if (currentIndex < 0) return false;
    const nextIndex = Math.min(Math.max(currentIndex + offset, 0), sessionList.length - 1);
    return focusSessionOptionAtIndex(nextIndex);
  };

  const handleSessionSearchKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'ArrowDown' || event.altKey || event.ctrlKey || event.metaKey) return;
    if (focusSessionOptionAtIndex(0)) {
      event.preventDefault();
    }
  };

  const handleSessionOptionKeyDown = (
    event: KeyboardEvent & { currentTarget: HTMLButtonElement },
    sessionId: string,
  ) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;

    if (event.key === 'ArrowDown' && focusSessionOptionRelativeTo(sessionId, 1)) {
      event.preventDefault();
      return;
    }
    if (event.key === 'ArrowUp' && focusSessionOptionRelativeTo(sessionId, -1)) {
      event.preventDefault();
      return;
    }
    if (event.key === 'Home' && focusSessionOptionAtIndex(0)) {
      event.preventDefault();
      return;
    }
    if (event.key === 'End' && focusSessionOptionAtIndex(sessionPickerSessions().length - 1)) {
      event.preventDefault();
      return;
    }
    if (event.key === 'Escape') {
      event.preventDefault();
      setShowSessions(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      sessionButtonRef?.focus();
    }
  };

  // Chat hook
  const chat = useChat({
    model: '',
    defaultModel: () => defaultModel().trim(),
    onConversationChanged: refreshSessions,
  });

  const stopActiveResponse = () => {
    clearInterruptArm();
    chat.stop();
    focusComposer();
  };

  const queuedFollowUpPreview = (prompt: string) => {
    const firstLine = prompt
      .split(/\r?\n/)
      .map((line) => line.trim())
      .find((line) => line.length > 0);
    return firstLine || 'Queued follow-up';
  };

  const restoreQueuedMentions = (mentions?: QueuedFollowUp['mentions']) => {
    setAccumulatedMentions(
      (mentions || []).map((mention) => ({
        id: mention.id,
        label: mention.name,
        type: mention.type,
        node: mention.node,
      })),
    );
  };

  const editQueuedFollowUp = (id: string) => {
    const queued = chat.takeQueuedFollowUp(id);
    if (!queued) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(queued);
    setInput(queued.prompt);
    restoreQueuedMentions(queued.mentions);
    setMentionActive(false);
    focusComposer();
    queueMicrotask(resizeTextarea);
  };

  const defaultModelLabel = createMemo(() => {
    const fallback = defaultModel().trim();
    if (!fallback) return '';
    const match = aiRuntimeModels().find((model) => model.id === fallback);
    return match ? formatAIModelRouteLabel(match) : formatAIModelRouteLabel(fallback);
  });

  const chatOverrideLabel = createMemo(() => {
    const override = chatOverrideModel().trim();
    if (!override) return '';
    const match = aiRuntimeModels().find((model) => model.id === override);
    return match ? formatAIModelRouteLabel(match) : formatAIModelRouteLabel(override);
  });

  const selectedChatModel = createMemo(() => {
    const selected = chat.model().trim();
    return selected || defaultModel().trim();
  });

  const selectedChatProvider = createMemo(() => {
    const model = selectedChatModel();
    if (!model) return '';
    const match = aiRuntimeModels().find((candidate) => candidate.id === model);
    return match?.provider?.trim() || getProviderFromModelId(model);
  });

  const formatChatMessageModelRoute = (modelId: string) => {
    const normalized = modelId.trim();
    if (!normalized) return '';
    const match = aiRuntimeModels().find((candidate) => candidate.id === normalized);
    return match ? formatAIModelRouteLabel(match) : formatAIModelRouteLabel(normalized);
  };

  const providerForModelRoute = (modelId: string) => {
    const normalized = modelId.trim();
    if (!normalized) return '';
    const match = aiRuntimeModels().find((candidate) => candidate.id === normalized);
    return match?.provider?.trim() || getProviderFromModelId(normalized);
  };

  const failedModelRouteHistory = createMemo(() => {
    const modelIds = new Set<string>();
    const providers = new Set<string>();

    for (const message of chat.messages()) {
      const modelId = message.error && message.model?.trim();
      if (!modelId) continue;
      modelIds.add(modelId);
      const provider = providerForModelRoute(modelId);
      if (provider) providers.add(provider);
    }

    return {
      modelIds: Array.from(modelIds),
      providers: Array.from(providers),
    };
  });

  const modelRouteAlternativeFor = (modelId: string): ModelRouteRecoveryOption | null => {
    const normalized = modelId.trim();
    if (!normalized) return null;
    const failedHistory = failedModelRouteHistory();
    return findProviderReadinessAlternative({
      avoidModelIds: failedHistory.modelIds.filter((failedModelId) => failedModelId !== normalized),
      avoidProviders: failedHistory.providers.filter(
        (failedProvider) => failedProvider !== providerForModelRoute(normalized),
      ),
      configuredProviders: aiRuntimeSettings()?.configured_providers,
      models: aiRuntimeModels(),
      selectedModel: normalized,
      selectedProvider: providerForModelRoute(normalized),
    });
  };

  const providerReadinessPresentation = createMemo(() => {
    const readiness = providerReadiness();
    if (!readiness.provider || readiness.status === 'idle' || readiness.status === 'ready') {
      return null;
    }
    if (readiness.status === 'checking' && !chat.isLoading()) {
      return null;
    }
    return getAIChatProviderReadinessPresentation({
      status: readiness.status === 'checking' ? 'checking' : 'error',
      providerLabel: getAIProviderDisplayName(readiness.provider),
      message: readiness.message,
      summary: readiness.summary,
      recommendation: readiness.recommendation,
    });
  });

  const providerReadinessAlternative = createMemo(() => {
    const readiness = providerReadiness();
    if (readiness.status !== 'error') return null;
    return modelRouteAlternativeFor(selectedChatModel());
  });

  let providerReadinessRequestId = 0;
  let lastProviderReadinessKey = '';

  const refreshSelectedProviderReadiness = async (provider: string, model: string) => {
    const requestId = ++providerReadinessRequestId;
    setProviderReadiness({
      status: 'checking',
      provider,
      model,
    });

    try {
      const result = await AIAPI.testProvider(provider, model);
      if (requestId !== providerReadinessRequestId) return;
      setProviderReadiness({
        status: result.success ? 'ready' : 'error',
        provider: result.provider || provider,
        model: result.model,
        message: result.message,
        summary: result.summary,
        recommendation: result.recommendation,
        action: result.action,
      });
    } catch (error) {
      if (requestId !== providerReadinessRequestId) return;
      logger.error('[AIChat] Failed to check selected provider readiness:', error);
      setProviderReadiness({
        status: 'error',
        provider,
        model,
        message: 'Provider check failed',
        summary: 'Pulse could not verify the selected provider route.',
        recommendation:
          'Check provider settings and network reachability, then retry the provider check.',
        action: 'open_provider_settings',
      });
    }
  };

  const retrySelectedProviderReadiness = () => {
    setProviderReadinessRetryNonce((value) => value + 1);
    focusComposer();
  };

  const isOverlayLayout = createMemo(() => width() < AI_CHAT_MIN_DOCKED_VIEWPORT_WIDTH);
  const rootClassName = createMemo(() => {
    if (isOverlayLayout()) {
      return `fixed inset-y-0 right-0 z-50 flex h-full w-full flex-col bg-surface transition-transform duration-300 sm:w-[560px] sm:max-w-[calc(100vw-1rem)] ${
        isOpen()
          ? 'translate-x-0 overflow-visible border-l border-border shadow-2xl'
          : 'translate-x-full overflow-hidden border-l-0'
      }`;
    }

    return `relative flex h-full flex-shrink-0 flex-col bg-surface transition-all duration-300 ${
      isOpen()
        ? 'w-full overflow-visible border-l border-border sm:w-[560px]'
        : 'w-0 overflow-hidden border-l-0'
    }`;
  });

  const loadModels = async (notify = false) => {
    if (notify) {
      notificationStore.info('Refreshing models...', 2000);
    }
    try {
      const nextModels = await loadAIRuntimeModels(true);
      const modelLoadError = aiRuntimeModelsError();
      if (modelLoadError) {
        if (notify) {
          notificationStore.warning(modelLoadError, 6000);
        }
      } else if (notify) {
        notificationStore.success(`Models refreshed (${nextModels.length})`, 2000);
      }
    } catch (error) {
      logger.error('[AIChat] Failed to load models:', error);
      const message = error instanceof Error ? error.message : 'Failed to load models.';
      notificationStore.error(message);
    }
  };

  createEffect(() => {
    const settings = aiRuntimeSettings();
    const chatOverride = (settings?.chat_model || '').trim();
    const fallback = chatOverride || (settings?.model || '').trim();
    setDefaultModel(fallback);
    setChatOverrideModel(chatOverride);
    setControlLevel(normalizeAIControlLevel(settings?.control_level));
    setDiscoveryEnabled(settings?.discovery_enabled ?? false);
  });

  const updateControlLevel = async (nextLevel: 'read_only' | 'controlled' | 'autonomous') => {
    if (controlSaving() || nextLevel === controlLevel()) {
      setShowControlMenu(false);
      return;
    }
    setControlSaving(true);
    const previous = controlLevel();
    try {
      const updated = await AIAPI.updateSettings({ control_level: nextLevel });
      const resolved = normalizeAIControlLevel(updated.control_level || nextLevel);
      syncAIRuntimeSettings(updated);
      setControlLevel(resolved);
      if (resolved === 'autonomous') setAutonomousBannerDismissed(false);
      notificationStore.success(
        `Control mode set to ${getAIChatControlLevelPresentation(resolved).label}`,
        2000,
      );
    } catch (error) {
      logger.error('[AIChat] Failed to update control level:', error);
      setControlLevel(previous);
      const message = error instanceof Error ? error.message : 'Failed to update control mode.';
      notificationStore.error(message);
    } finally {
      setControlSaving(false);
      setShowControlMenu(false);
    }
  };

  const selectModel = (modelId: string) => {
    chat.setModel(modelId);
    updateStoredModel(chat.sessionId(), modelId);
    rememberRecentModel(modelId);
  };

  const openModelSelectorFromError = () => {
    setModelSelectorOpenRequest((value) => value + 1);
  };

  const getFailedTurnModelRouteAlternative = (message: ChatMessage) => {
    if (!message.error) return null;
    return modelRouteAlternativeFor(message.model || selectedChatModel());
  };

  const switchToModelRoute = (modelId: string, failedMessageId?: string) => {
    selectModel(modelId);
    if (failedMessageId) {
      chat.retryMessage(failedMessageId, { model: modelId });
    }
    focusComposer();
  };

  const switchToProviderReadinessAlternative = () => {
    const alternative = providerReadinessAlternative();
    if (!alternative) return;
    switchToModelRoute(alternative.id);
  };

  const selectProviderReadinessAlternativeForSend = () => {
    const readiness = providerReadiness();
    if (readiness.status !== 'error') return null;

    const currentProvider = selectedChatProvider().trim();
    const failedProvider = readiness.provider.trim();
    if (failedProvider && currentProvider && failedProvider !== currentProvider) {
      return null;
    }

    const alternative = providerReadinessAlternative();
    if (!alternative) return null;

    selectModel(alternative.id);
    return alternative;
  };

  createEffect(() => {
    const sessionId = chat.sessionId();
    const storedModel = getStoredModel(sessionId);
    if (storedModel) {
      if (chat.model() !== storedModel) {
        chat.setModel(storedModel);
      }
      return;
    }
    // If there's no stored model for this session but we have a current selection,
    // preserve it (and migrate it to this session)
    const currentModel = chat.model();
    if (currentModel && sessionId) {
      updateStoredModel(sessionId, currentModel);
    }
  });

  const initializeWhenOpen = async () => {
    try {
      const status = await AIChatAPI.getStatus();
      if (!status.running) {
        // AI not running - silently clear dynamic state instead of logging on
        // unrelated routes or after the assistant is disabled.
        setSessions([]);
        setDefaultModel('');
        setChatOverrideModel('');
        setDiscoveryEnabled(false);
        setControlLevel('read_only');
        return;
      }
      await Promise.all([refreshSessions(), loadAIRuntimeSettings(), loadAIRuntimeModels()]);
    } catch (error) {
      logger.error('[AIChat] Failed to initialize:', error);
    }
  };

  const contextBriefing = createMemo(() => aiChatStore.context.briefing);
  const hasScopedApprovalHandoff = createMemo(() => aiChatStore.context.autonomousMode === false);
  const controlPresentation = createMemo(() =>
    getAIChatControlLevelPresentation(
      hasScopedApprovalHandoff() && controlLevel() === 'autonomous' ? 'controlled' : controlLevel(),
    ),
  );
  const contextBriefingTitle = createMemo(() => {
    const briefing = contextBriefing();
    if (!briefing) return '';

    const title = briefing.title?.trim() ?? '';
    const subject = briefing.subject?.trim() ?? '';
    if (shouldShowStructuredBriefingContext(aiChatStore.context)) {
      return title || subject || 'Context attached';
    }
    if (subject && /attached|briefing/i.test(title)) {
      return subject;
    }

    return title || subject || 'Context attached';
  });
  const contextBriefingDetailLines = createMemo(() => {
    const briefing = contextBriefing();
    if (!briefing) return [];
    if (!shouldShowStructuredBriefingContext(aiChatStore.context)) return [];

    const seen = new Set<string>();
    const lines: string[] = [];
    const addLine = (value: string | undefined) => {
      const normalized = value?.trim();
      if (!normalized || seen.has(normalized)) return;
      seen.add(normalized);
      lines.push(normalized);
    };

    const subject = briefing.subject?.trim();
    if (subject !== contextBriefingTitle()) {
      addLine(subject);
    }
    addLine(briefing.actionLabel);
    for (const line of briefing.detailLines ?? []) {
      addLine(line);
    }
    for (const line of briefing.evidence ?? []) {
      addLine(line);
    }
    addLine(briefing.safetyNote);

    return lines.slice(0, 8);
  });
  const contextBriefingNote = createMemo(() => {
    if (hasScopedApprovalHandoff()) {
      return 'Approval required before any action.';
    }

    return '';
  });
  const currentStatus = createMemo(() =>
    getAssistantActiveTurnStatus(chat.messages(), chat.isLoading()),
  );
  const [currentStatusNow, setCurrentStatusNow] = createSignal(Date.now());
  createEffect(() => {
    const status = currentStatus();
    if (!status?.startedAt) return;
    setCurrentStatusNow(Date.now());
    const interval = window.setInterval(() => setCurrentStatusNow(Date.now()), 1000);
    onCleanup(() => window.clearInterval(interval));
  });
  const currentStatusText = createMemo(() => {
    const status = currentStatus();
    if (!status) return '';
    if (!status.startedAt) return status.text;
    const elapsedSeconds = Math.max(0, Math.floor((currentStatusNow() - status.startedAt) / 1000));
    if (elapsedSeconds < 2) return status.text;
    return `${status.text} (${elapsedSeconds}s)`;
  });
  const lastAssistantUsage = createMemo(() => {
    const messages = chat.messages();
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const usage = getAssistantUsageSummary(messages[index]);
      if (usage) return usage;
    }
    return null;
  });

  createEffect(() => {
    input();
    queueMicrotask(resizeTextarea);
  });

  createEffect(() => {
    if (!isOpen()) {
      return;
    }
    void initializeWhenOpen();
  });

  createEffect(() => {
    if (!showSessions()) {
      return;
    }

    const query = normalizedSessionSearchQuery();
    if (!query) {
      sessionSearchRequestId += 1;
      setSessionSearchResults(null);
      setSessionSearchLoading(false);
      setSessionSearchError('');
      return;
    }

    const requestId = ++sessionSearchRequestId;
    setSessionSearchResults(null);
    setSessionSearchLoading(true);
    setSessionSearchError('');

    const timeoutId = window.setTimeout(() => {
      AIChatAPI.listSessions({ search: query, limit: AI_CHAT_SESSION_SEARCH_LIMIT })
        .then((sessionList) => {
          if (requestId !== sessionSearchRequestId) return;
          setSessionSearchResults(sessionList);
        })
        .catch((error) => {
          if (requestId !== sessionSearchRequestId) return;
          logger.error('[AIChat] Failed to search sessions:', error);
          setSessionSearchResults([]);
          setSessionSearchError(AI_CHAT_SESSION_SEARCH_ERROR_STATE);
        })
        .finally(() => {
          if (requestId !== sessionSearchRequestId) return;
          setSessionSearchLoading(false);
        });
    }, AI_CHAT_SESSION_SEARCH_DEBOUNCE_MS);

    onCleanup(() => window.clearTimeout(timeoutId));
  });

  createEffect(() => {
    if (!chat.isLoading() && interruptArmed()) {
      clearInterruptArm();
    }
  });

  createEffect(() => {
    const open = isOpen();
    const model = selectedChatModel().trim();
    const provider = selectedChatProvider().trim();
    const retryNonce = providerReadinessRetryNonce();

    if (!open) {
      providerReadinessRequestId += 1;
      lastProviderReadinessKey = '';
      setProviderReadiness({ status: 'idle', provider: '' });
      return;
    }

    if (!provider || !model) {
      providerReadinessRequestId += 1;
      lastProviderReadinessKey = '';
      setProviderReadiness({ status: 'idle', provider: '' });
      return;
    }

    const key = `${provider}:${model}:${retryNonce}`;
    if (key === lastProviderReadinessKey) {
      return;
    }

    lastProviderReadinessKey = key;
    void refreshSelectedProviderReadiness(provider, model);
  });

  createEffect(() => {
    const readiness = providerReadiness();
    const override = chatOverrideModel().trim();
    if (!isOpen() || readiness.status !== 'error' || !override) {
      return;
    }

    const current = selectedChatModel().trim();
    if (!current || current === override) {
      return;
    }

    const failedProvider = readiness.provider.trim();
    const currentProvider = providerForModelRoute(current);
    const overrideProvider = providerForModelRoute(override);
    if (failedProvider && currentProvider !== failedProvider) {
      return;
    }
    if (failedProvider && overrideProvider === failedProvider) {
      return;
    }

    selectModel(override);
  });

  // Click outside handler to close all dropdowns
  onMount(() => {
    setPromptHistory(loadPromptHistory());
    aiChatStore.registerInput?.(textareaRef ?? null);
    restoreStashedComposerDraft();
    focusComposer();

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      // Only close if click is outside dropdown containers
      if (!target.closest('[data-dropdown]')) {
        setShowSessions(false);
        setSessionRefreshLoading(false);
        resetSessionSearch();
        setShowControlMenu(false);
      }
      // Close mention autocomplete when clicking outside
      if (!target.closest('[data-mention-autocomplete]') && !target.closest('textarea')) {
        setMentionActive(false);
      }
    };
    document.addEventListener('click', handleClickOutside);
    onCleanup(() => {
      stashComposerDraftForRemount();
      document.removeEventListener('click', handleClickOutside);
      aiChatStore.registerInput?.(null);
      clearInterruptArm();
    });
  });

  const mentionStatusRank = (status?: string) => {
    switch (normalizeChatMentionKeyPart(status || '')) {
      case 'running':
      case 'online':
        return 3;
      case 'stopped':
      case 'offline':
      case 'exited':
        return 2;
      case 'unknown':
        return 1;
      default:
        return 0;
    }
  };

  const dedupeMentionResources = (resources: MentionResource[]) => {
    // Only dedupe agent mentions, and use the stable mention id so redacted labels
    // do not collapse distinct resources into one suggestion.
    const byKey = new Map<string, { resource: MentionResource; index: number }>();
    const out: MentionResource[] = [];

    for (const resource of resources) {
      if (resource.type !== 'agent') {
        out.push(resource);
        continue;
      }

      const key = resource.id;
      const existing = byKey.get(key);
      if (!existing) {
        const index = out.length;
        byKey.set(key, { resource, index });
        out.push(resource);
        continue;
      }

      if (mentionStatusRank(resource.status) > mentionStatusRank(existing.resource.status)) {
        existing.resource = resource;
        out[existing.index] = resource;
      }
    }

    return out;
  };

  // Build resources for @ mention autocomplete from unified selectors
  createEffect(() => {
    const readPlatformData = (resource: Resource): Record<string, unknown> | undefined => {
      return resource.platformData
        ? (unwrap(resource.platformData) as Record<string, unknown>)
        : undefined;
    };
    const asRecord = (value: unknown): Record<string, unknown> | undefined =>
      value && typeof value === 'object' ? (value as Record<string, unknown>) : undefined;
    const asString = (value: unknown): string | undefined =>
      typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
    const getAgentActionId = (resource: Resource): string => {
      return getActionableAgentIdFromResource(resource) || resource.id;
    };
    const getDockerActionId = (resource: Resource): string => {
      const platformData = readPlatformData(resource);
      const platformDocker = asRecord(platformData?.docker);
      const discoveryTarget = resource.discoveryTarget;
      return (
        (isAppContainerDiscoveryResourceType(discoveryTarget?.resourceType)
          ? discoveryTarget?.resourceId
          : undefined) ||
        asString(platformDocker?.hostSourceId) ||
        asString(platformData?.hostSourceId) ||
        discoveryTarget?.agentId ||
        resource.id
      );
    };
    const getAppContainerMentionHost = (resource: Resource): string => {
      const platformData = readPlatformData(resource);
      const platformDocker = asRecord(platformData?.docker);
      const platformTrueNAS = asRecord(platformData?.truenas);
      return (
        resource.parentName ||
        asString(platformDocker?.hostname) ||
        asString(platformTrueNAS?.hostname) ||
        getPreferredResourceHostname(resource) ||
        ''
      );
    };
    const getAppContainerMentionId = (resource: Resource): string => {
      if (resource.id.startsWith('app-container:')) {
        return resource.id;
      }

      const host = getAppContainerMentionHost(resource);
      const platformData = readPlatformData(resource);
      const platformDocker = asRecord(platformData?.docker);
      const containerID =
        asString(platformDocker?.containerId) ||
        asString(platformDocker?.name) ||
        asString(resource.canonicalIdentity?.primaryId) ||
        asString(resource.name) ||
        asString(resource.id);

      if (host && containerID) {
        return `app-container:${host}:${containerID}`;
      }

      return resource.id;
    };

    const parseLegacyVmid = (
      platformData: Record<string, unknown> | undefined,
    ): number | undefined => {
      const vmidRaw = platformData?.vmid;
      if (typeof vmidRaw === 'number' && Number.isFinite(vmidRaw) && vmidRaw > 0) return vmidRaw;
      if (typeof vmidRaw === 'string') {
        const parsed = parseInt(vmidRaw, 10);
        if (Number.isFinite(parsed) && parsed > 0) return parsed;
      }
      return undefined;
    };

    const getVMMentionNode = (resource: Resource): string => {
      const platformData = readPlatformData(resource);
      return (
        asString(platformData?.node) ||
        asString(resource.vmware?.runtimeHostName) ||
        asString(resource.vmware?.runtimeHostId) ||
        resource.parentName ||
        ''
      );
    };

    const getVMMentionId = (resource: Resource): string => {
      if (resource.id.startsWith('vm:')) {
        return resource.id;
      }

      const platformData = readPlatformData(resource);
      const node = getVMMentionNode(resource);
      const vmid = parseLegacyVmid(platformData);
      if (node && vmid !== undefined) {
        return `vm:${node}:${vmid}`;
      }

      return resource.id;
    };

    const getSystemContainerMentionId = (resource: Resource): string => {
      if (resource.id.startsWith('system-container:')) {
        return resource.id;
      }

      const platformData = readPlatformData(resource);
      const node = asString(platformData?.node) || resource.parentName || '';
      const vmid = parseLegacyVmid(platformData);
      if (node && vmid !== undefined) {
        return `system-container:${node}:${vmid}`;
      }

      return resource.id;
    };

    const getStorageMentionNode = (resource: Resource): string => {
      return (
        resource.parentName ||
        asString(resource.vmware?.connectionName) ||
        asString(resource.vmware?.runtimeHostName) ||
        ''
      );
    };

    const nodes = byType('agent');
    const vms = byType('vm');
    const containers = [...byType('system-container'), ...byType('oci-container')];
    const dockerHosts = byType('docker-host');
    const appContainers = byType('app-container');
    const storageResources = byType('storage');
    const agentResources = allResources().filter((resource) =>
      isAgentFacetInfrastructureResource(resource),
    );
    const mentionCandidates: MentionResource[] = [];

    // Add VMs
    for (const vm of vms) {
      const node = getVMMentionNode(vm);
      mentionCandidates.push({
        id: getVMMentionId(vm),
        label: getPreferredResourceDisplayName(vm),
        type: 'vm',
        status: vm.status === 'running' ? 'running' : 'stopped',
        node,
      });
    }

    // Add LXC/system containers (includes OCI containers).
    for (const container of containers) {
      const platformData = readPlatformData(container);
      const node = asString(platformData?.node) || container.parentName || '';
      mentionCandidates.push({
        id: getSystemContainerMentionId(container),
        label: getPreferredResourceDisplayName(container),
        type: 'system-container',
        status: container.status === 'running' ? 'running' : 'stopped',
        node,
      });
    }

    // Add container runtimes
    for (const runtime of dockerHosts) {
      const dockerActionId = getDockerActionId(runtime);
      const label = getPreferredResourceDisplayName(runtime);
      const runtimeStatus =
        runtime.status === 'online' || runtime.status === 'running'
          ? 'online'
          : runtime.status || 'online';
      mentionCandidates.push({
        id: `agent:${dockerActionId}`,
        label,
        type: 'agent',
        status: runtimeStatus,
      });
    }

    // Add app containers through canonical unified resource identity rather than
    // a Docker-only transport shape, so API-backed platforms such as TrueNAS use
    // the same mention contract as Docker-backed workloads.
    for (const container of appContainers) {
      mentionCandidates.push({
        id: getAppContainerMentionId(container),
        label: getPreferredResourceDisplayName(container),
        type: 'app-container',
        status: container.status === 'running' ? 'running' : 'exited',
        node: getAppContainerMentionHost(container),
      });
    }

    for (const storage of storageResources) {
      mentionCandidates.push({
        id: storage.id,
        label: getPreferredResourceDisplayName(storage),
        type: 'storage',
        status: storage.status,
        node: getStorageMentionNode(storage),
      });
    }

    // Add nodes
    for (const node of nodes) {
      mentionCandidates.push({
        id: `node:${node.platformId || ''}:${node.name}`,
        label: getPreferredResourceDisplayName(node),
        type: 'agent',
        status: node.status,
      });
    }

    // Add standalone agents
    for (const agentResource of agentResources) {
      const agentActionId = getAgentActionId(agentResource);
      const label = getPreferredResourceDisplayName(agentResource);
      const agentStatus =
        agentResource.status === 'online' || agentResource.status === 'running'
          ? 'online'
          : agentResource.status;
      mentionCandidates.push({
        id: `agent:${agentActionId}`,
        label,
        type: 'agent',
        status: agentStatus,
      });
    }

    setMentionResources(dedupeMentionResources(mentionCandidates));
  });

  const restoreFailedSubmitDraft = (draft: string, mentions: MentionResource[]) => {
    if (input().trim() || accumulatedMentions().length > 0) {
      return;
    }
    setInput(draft);
    setAccumulatedMentions(cloneMentions(mentions));
    setMentionActive(false);
    queueMicrotask(() => {
      resizeTextarea();
      focusComposer();
    });
  };

  const readComposerInputForSubmit = () => {
    // Composition/IME updates can reach the textarea before the controlled signal
    // flushes, so the live DOM value is authoritative for submit.
    if (typeof textareaRef?.value === 'string') return textareaRef.value;
    return input();
  };

  // Handle submit
  const handleSubmit = () => {
    if (composerSubmitDispatchLocked) return;

    const submittedInput = readComposerInputForSubmit();
    const prompt = submittedInput.trim();
    if (!prompt) return;
    composerSubmitDispatchLocked = true;
    queueMicrotask(() => {
      composerSubmitDispatchLocked = false;
    });
    if (submittedInput !== input()) {
      setInput(submittedInput);
    }
    const mentions = accumulatedMentions();
    const submittedMentions = cloneMentions(mentions);
    const mentionsForAPI =
      mentions.length > 0
        ? mentions.map((mention) => ({
            id: mention.id,
            name: mention.label,
            type: mention.type,
            node: mention.node,
          }))
        : undefined;
    // Pass findingId from context on the first message, clear after success
    const ctx = aiChatStore.context;
    const queuedDraft = editingQueuedFollowUp();
    const findingId = queuedDraft ? queuedDraft.findingId : ctx.findingId;
    const sendOptions: SendMessageOptions = queuedDraft?.sendOptions
      ? { ...queuedDraft.sendOptions }
      : {};
    if (!queuedDraft) {
      if (typeof ctx.autonomousMode === 'boolean') {
        sendOptions.autonomousMode = ctx.autonomousMode;
      }
      if (ctx.handoffContext && ctx.handoffContext.trim()) {
        sendOptions.handoffContext = ctx.handoffContext;
      }
      if (ctx.handoffResources && ctx.handoffResources.length > 0) {
        sendOptions.handoffResources = ctx.handoffResources;
      }
      if (ctx.handoffActions && ctx.handoffActions.length > 0) {
        sendOptions.handoffActions = ctx.handoffActions;
      }
      if (ctx.handoffMetadata) {
        sendOptions.handoffMetadata = ctx.handoffMetadata;
      }
    }
    const routeAlternative = selectProviderReadinessAlternativeForSend();
    if (routeAlternative) {
      sendOptions.model = routeAlternative.id;
    }

    const hasSendOptions =
      Boolean(sendOptions.model) ||
      typeof sendOptions.autonomousMode === 'boolean' ||
      Boolean(sendOptions.handoffContext) ||
      Boolean(sendOptions.handoffResources?.length) ||
      Boolean(sendOptions.handoffActions?.length) ||
      Boolean(sendOptions.handoffMetadata);

    const sendPromise = hasSendOptions
      ? chat.sendMessage(prompt, mentionsForAPI, findingId, sendOptions)
      : chat.sendMessage(prompt, mentionsForAPI, findingId);
    addPromptHistoryEntry(prompt, mentions);
    resetPromptHistoryNavigation();
    const hasRequestHandoffPayload =
      Boolean(ctx.handoffContext?.trim()) ||
      Boolean(ctx.handoffResources?.length) ||
      Boolean(ctx.handoffActions?.length) ||
      Boolean(ctx.handoffMetadata);
    sendPromise
      .then((ok) => {
        if (!ok) {
          restoreFailedSubmitDraft(submittedInput, submittedMentions);
          return;
        }
        stashedComposerDraft = null;
        setEditingQueuedFollowUp(null);
        if (!queuedDraft && findingId) {
          aiChatStore.clearFindingId?.();
        }
        if (!queuedDraft && hasRequestHandoffPayload) {
          aiChatStore.clearRequestHandoffPayload?.();
        }
      })
      .catch((error) => {
        logger.warn('[AIChat] Failed to send Assistant message:', error);
        restoreFailedSubmitDraft(submittedInput, submittedMentions);
      });
    stashedComposerDraft = null;
    setInput('');
    setAccumulatedMentions([]);
    setMentionActive(false);
    focusComposer();
  };

  // Handle input change with @ mention detection
  const handleInputChange = (e: InputEvent & { currentTarget: HTMLTextAreaElement }) => {
    const value = e.currentTarget.value;
    if (promptHistoryIndex() >= 0) {
      resetPromptHistoryNavigation();
    }
    setInput(value);
    resizeTextarea();

    const cursorPos = e.currentTarget.selectionStart || 0;
    const textBeforeCursor = value.slice(0, cursorPos);

    // Find the last @ before cursor
    const lastAtIndex = textBeforeCursor.lastIndexOf('@');

    if (lastAtIndex !== -1) {
      // Check if @ is at start or preceded by whitespace
      const charBefore = lastAtIndex > 0 ? textBeforeCursor[lastAtIndex - 1] : ' ';
      if (charBefore === ' ' || charBefore === '\n' || lastAtIndex === 0) {
        const query = textBeforeCursor.slice(lastAtIndex + 1);
        // Only activate if query doesn't contain spaces (still typing the mention)
        if (!query.includes(' ')) {
          setMentionActive(true);
          setMentionQuery(query);
          setMentionStartIndex(lastAtIndex);
          return;
        }
      }
    }

    setMentionActive(false);
  };

  // Handle mention selection
  const handleMentionSelect = (resource: MentionResource) => {
    const currentInput = input();
    const startIndex = mentionStartIndex();
    const cursorPos = textareaRef?.selectionStart || currentInput.length;

    // Replace @query with the resource name
    const before = currentInput.slice(0, startIndex);
    const after = currentInput.slice(cursorPos);
    const newValue = `${before}@${resource.label} ${after}`;

    setInput(newValue);
    setMentionActive(false);

    // Accumulate the structured mention data so we can send it with the prompt
    setAccumulatedMentions((prev) => {
      // Deduplicate by id
      if (prev.some((m) => m.id === resource.id)) return prev;
      return [...prev, resource];
    });

    // Focus textarea and set cursor position after the inserted name
    setTimeout(() => {
      if (textareaRef) {
        textareaRef.focus();
        const newCursorPos = startIndex + resource.label.length + 2; // +2 for @ and space
        textareaRef.setSelectionRange(newCursorPos, newCursorPos);
      }
    }, 0);
  };

  // Handle key down - submit when not loading, but let autocomplete handle keys when active
  const handleKeyDown = (e: KeyboardEvent) => {
    // Let mention autocomplete handle navigation keys
    if (mentionActive()) {
      if (['ArrowDown', 'ArrowUp', 'Enter', 'Tab', 'Escape'].includes(e.key)) {
        // These are handled by MentionAutocomplete component
        return;
      }
    }

    if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
      const direction = e.key === 'ArrowUp' ? 'up' : 'down';
      if (canNavigatePromptHistory(direction) && navigatePromptHistory(direction)) {
        e.preventDefault();
      }
      return;
    }

    if (e.key === 'Escape' && chat.isLoading()) {
      e.preventDefault();
      e.stopPropagation();
      if (interruptArmed()) {
        stopActiveResponse();
      } else {
        armKeyboardInterrupt();
        focusComposer();
      }
      return;
    }

    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  // New conversation
  const handleNewConversation = async () => {
    const started = await chat.newSession();
    if (!started) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(null);
    aiChatStore.clearContext?.();
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    focusComposer();
  };

  const handleToggleSessions = async () => {
    const next = !showSessions();
    if (!next) {
      setShowSessions(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      return;
    }

    if (sessionButtonRef) {
      const rect = sessionButtonRef.getBoundingClientRect();
      setSessionDropdownPosition({
        top: rect.bottom + 4,
        right: window.innerWidth - rect.right,
      });
    }

    resetSessionSearch();
    setShowSessions(true);
    focusSessionSearch();
    setSessionRefreshLoading(true);

    try {
      await refreshSessions();
    } catch (error) {
      logger.error('[AIChat] Failed to refresh sessions before opening picker:', error);
      notificationStore.error('Failed to refresh assistant sessions');
    } finally {
      setSessionRefreshLoading(false);
    }
  };

  const formatSessionPickerMessageCount = (count: number) =>
    `${count} ${count === 1 ? 'message' : 'messages'}`;
  const getSessionPickerOptionLabel = (session: ChatSession) =>
    `Resume ${session.title || 'Untitled'}, ${formatSessionPickerMessageCount(session.message_count)}`;
  const getSessionDeleteLabel = (session: ChatSession) =>
    `Delete Assistant session: ${session.title || 'Untitled'}`;

  // Load session
  const handleLoadSession = async (sessionId: string) => {
    const session = findKnownSession(sessionId);
    const loaded = await chat.loadSession(sessionId);
    if (!loaded) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(null);
    const restoredContext = buildSessionHandoffContext(session);
    if (restoredContext) {
      aiChatStore.setContext(restoredContext);
    } else {
      aiChatStore.clearContext?.();
    }
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    focusComposer();
  };

  // Delete session
  const handleDeleteSession = async (sessionId: string, e: Event) => {
    e.stopPropagation();
    if (!confirm('Delete this conversation?')) return;
    try {
      await AIChatAPI.deleteSession(sessionId);
      setSessions((prev) => prev.filter((s) => s.id !== sessionId));
      setSessionSearchResults((prev) => prev?.filter((s) => s.id !== sessionId) ?? null);
      updateStoredModel(sessionId, '');
      if (chat.sessionId() === sessionId) {
        chat.clearMessages();
        resetPromptHistoryNavigation();
        setEditingQueuedFollowUp(null);
      }
    } catch (_error) {
      notificationStore.error('Failed to delete session');
    }
  };

  // Approval handlers
  const handleApprove = async (messageId: string, approval: PendingApproval) => {
    if (!approval.approvalId) {
      notificationStore.error('No approval ID available');
      return;
    }

    // Mark as executing
    chat.updateApproval(messageId, approval.toolId, { isExecuting: true });

    try {
      // Call the approve endpoint - this marks it as approved in the backend
      // The agentic loop will detect this and execute the command
      // Execution results will come via tool_end event in the stream
      await AIChatAPI.approveCommand(approval.approvalId);

      // Remove from pending approvals - the tool_end event will show the result
      chat.updateApproval(messageId, approval.toolId, { removed: true });

      logger.debug('[AIChat] Command approved, waiting for agentic loop to execute', {
        approvalId: approval.approvalId,
        toolName: approval.toolName,
      });

      // Note: We don't manually add tool results or send continuation messages here.
      // The agentic loop will:
      // 1. Detect the approval
      // 2. Re-execute the tool with the approval_id
      // 3. Send a tool_end event with the result
      // 4. Continue the conversation automatically
    } catch (error) {
      logger.error('[AIChat] Approval failed:', error);
      notificationStore.error('Failed to approve command');
      chat.updateApproval(messageId, approval.toolId, { isExecuting: false });
    }
  };

  const handleSkip = async (messageId: string, toolId: string) => {
    // Find the approval to get the approvalId
    const msg = chat.messages().find((m) => m.id === messageId);
    const approval = msg?.pendingApprovals?.find((a) => a.toolId === toolId);

    if (!approval?.approvalId) {
      // Just remove from UI if no approval ID
      chat.updateApproval(messageId, toolId, { removed: true });
      return;
    }

    try {
      await AIChatAPI.denyCommand(approval.approvalId, 'User skipped');
      chat.updateApproval(messageId, toolId, { removed: true });
    } catch (error) {
      logger.error('[AIChat] Skip/deny failed:', error);
      notificationStore.error('Failed to skip approval');
    }
  };

  // Question handlers
  const handleAnswerQuestion = async (
    messageId: string,
    question: PendingQuestion,
    answers: Array<{ id: string; value: string }>,
  ) => {
    await chat.answerQuestion(messageId, question.questionId, answers);
  };

  const handleSkipQuestion = (messageId: string, questionId: string) => {
    // Just remove from UI - skipping a question
    chat.updateQuestion(messageId, questionId, { removed: true });
  };

  return (
    <>
      <Show when={isOpen() && isOverlayLayout()}>
        <button
          type="button"
          class="fixed inset-0 z-40 bg-slate-950/45 backdrop-blur-[1px]"
          onClick={props.onClose}
          aria-label="Close Pulse Assistant backdrop"
        />
      </Show>
      <div class={rootClassName()} data-layout-mode={isOverlayLayout() ? 'overlay' : 'docked'}>
        <Show when={isOpen()}>
          {/* Floating Close Handle (Desktop docked layout only) */}
          <Show when={!isOverlayLayout()}>
            <button
              type="button"
              onClick={props.onClose}
              class="hidden sm:flex absolute left-0 top-1/2 -translate-x-full -translate-y-1/2 items-center justify-center w-8 py-3 rounded-l-lg bg-surface text-blue-600 dark:text-blue-400 border border-r-0 border-border hover:bg-surface-hover hover:text-blue-700 dark:hover:text-blue-300 transition-colors z-50 cursor-pointer"
              title={AI_CHAT_COLLAPSE_TITLE}
              aria-label={AI_CHAT_COLLAPSE_TITLE}
            >
              <svg
                class="h-5 w-5 flex-shrink-0"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
                stroke-width="1.5"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z"
                />
              </svg>
            </button>
          </Show>
          {/* Header - wraps on mobile */}
          <div class="flex flex-wrap items-center gap-2 px-4 py-3 border-b border-border bg-surface-alt">
            <h2 class="min-w-0 flex-1 text-sm font-semibold text-base-content">
              {AI_CHAT_DRAWER_TITLE}
            </h2>

            <div
              class="order-3 flex w-full min-w-0 items-center gap-1.5 sm:order-none sm:w-auto sm:flex-none"
              data-testid="assistant-header-actions"
            >
              {/* New chat */}
              <button
                type="button"
                onClick={handleNewConversation}
                class="flex flex-shrink-0 items-center gap-1.5 px-2.5 py-1.5 text-[11px] text-muted hover:text-base-content rounded-md border border-border hover:border-border bg-surface transition-colors"
                title={AI_CHAT_NEW_SESSION_BUTTON_TITLE}
                aria-label={AI_CHAT_NEW_SESSION_BUTTON_TITLE}
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 4v16m8-8H4"
                  />
                </svg>
                <span class="font-medium">{AI_CHAT_NEW_SESSION_SHORT_LABEL}</span>
              </button>

              {/* Session picker */}
              <div class="relative" data-dropdown>
                <button
                  type="button"
                  ref={sessionButtonRef}
                  onClick={() => {
                    void handleToggleSessions();
                  }}
                  class="flex-shrink-0 p-2 hover:text-base-content rounded-md hover:bg-surface-hover transition-colors"
                  title={AI_CHAT_SESSION_MENU_TITLE}
                  aria-label={AI_CHAT_SESSION_MENU_TITLE}
                  aria-haspopup="dialog"
                  aria-expanded={showSessions()}
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                    />
                  </svg>
                </button>

                <Show when={showSessions()}>
                  <div
                    class="fixed w-80 max-h-[28rem] bg-surface rounded-md shadow-sm border border-border z-[9999] overflow-hidden"
                    role="dialog"
                    aria-label={AI_CHAT_SESSION_MENU_TITLE}
                    style={{
                      top: `${sessionDropdownPosition().top}px`,
                      right: `${sessionDropdownPosition().right}px`,
                    }}
                  >
                    <button
                      type="button"
                      onClick={handleNewConversation}
                      class="w-full px-3 py-2.5 text-left text-sm flex items-center gap-2 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900 border-b border-border"
                      aria-label={AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL}
                    >
                      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          stroke-width="2"
                          d="M12 4v16m8-8H4"
                        />
                      </svg>
                      <span class="font-medium">{AI_CHAT_NEW_SESSION_MENU_LABEL}</span>
                    </button>

                    <div class="border-b border-border px-3 py-2">
                      <SearchField
                        value={sessionSearchQuery()}
                        onChange={setSessionSearchQuery}
                        onKeyDown={handleSessionSearchKeyDown}
                        placeholder={AI_CHAT_SESSION_SEARCH_PLACEHOLDER}
                        title={AI_CHAT_SESSION_SEARCH_TITLE}
                        inputClass="py-1.5 text-xs"
                        inputRef={(input) => {
                          sessionSearchInputRef = input;
                        }}
                      />
                    </div>

                    <Show when={sessionPickerLoadingText()}>
                      <div
                        class="border-b border-border px-3 py-1.5 text-[11px] text-muted"
                        role="status"
                        aria-live="polite"
                      >
                        {sessionPickerLoadingText()}
                      </div>
                    </Show>

                    <div
                      class="max-h-72 overflow-y-auto"
                      role="listbox"
                      aria-label="Assistant session history"
                    >
                      <Show
                        when={sessionPickerSessions().length > 0}
                        fallback={
                          <div class="px-3 py-6 text-center text-xs text-muted">
                            {sessionPickerEmptyText()}
                          </div>
                        }
                      >
                        <For each={sessionPickerSessions()}>
                          {(session) => (
                            <div
                              class={`group relative flex items-start gap-2 px-3 py-2.5 hover:bg-surface-hover focus-within:bg-surface-hover ${chat.sessionId() === session.id ? 'bg-blue-50 dark:bg-blue-900' : ''}`}
                            >
                              <button
                                type="button"
                                ref={(button) => {
                                  sessionOptionRefs.set(session.id, button);
                                }}
                                role="option"
                                aria-selected={chat.sessionId() === session.id}
                                aria-label={getSessionPickerOptionLabel(session)}
                                class="min-w-0 flex-1 text-left focus:outline-none"
                                onClick={() => handleLoadSession(session.id)}
                                onKeyDown={(event) => handleSessionOptionKeyDown(event, session.id)}
                              >
                                <div class="text-sm font-medium truncate text-base-content">
                                  {session.title || 'Untitled'}
                                </div>
                                <div class="text-xs text-muted">
                                  {formatSessionPickerMessageCount(session.message_count)}
                                </div>
                                <Show when={session.handoff_summary}>
                                  {(summary) => (
                                    <div class="mt-1 flex max-w-full flex-wrap gap-1.5">
                                      <span class="rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200">
                                        {getSessionHandoffSourceLabel(summary())}
                                      </span>
                                      <span class="rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] text-muted">
                                        {getSessionHandoffBadgeLabel(summary())}
                                      </span>
                                      <Show when={formatSessionHandoffStatus(summary())}>
                                        {(status) => (
                                          <span class="max-w-full truncate rounded border border-border bg-surface-alt px-1.5 py-0.5 text-[10px] text-muted">
                                            {status()}
                                          </span>
                                        )}
                                      </Show>
                                    </div>
                                  )}
                                </Show>
                              </button>
                              <button
                                type="button"
                                class="flex-shrink-0 p-1 rounded opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 focus:opacity-100 hover:bg-red-100 dark:hover:bg-red-900 hover:text-red-500 transition-opacity"
                                onClick={(e) => handleDeleteSession(session.id, e)}
                                aria-label={getSessionDeleteLabel(session)}
                                title={getSessionDeleteLabel(session)}
                              >
                                <svg
                                  class="w-3.5 h-3.5"
                                  fill="none"
                                  stroke="currentColor"
                                  viewBox="0 0 24 24"
                                >
                                  <path
                                    stroke-linecap="round"
                                    stroke-linejoin="round"
                                    stroke-width="2"
                                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                                  />
                                </svg>
                              </button>
                            </div>
                          )}
                        </For>
                      </Show>
                    </div>
                  </div>
                </Show>
              </div>
            </div>

            {/* Close button (Always visible as fallback) */}
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                props.onClose();
              }}
              class="order-2 flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-md hover:text-base-content hover:bg-surface-hover transition-colors sm:order-none"
              title={AI_CHAT_CLOSE_LABEL}
              aria-label={AI_CHAT_CLOSE_LABEL}
              data-testid="assistant-close-button"
            >
              <XIcon class="h-5 w-5" />
            </button>
          </div>

          <Show
            when={
              controlLevel() === 'autonomous' &&
              !autonomousBannerDismissed() &&
              !hasScopedApprovalHandoff()
            }
          >
            <div class="px-4 py-2 border-b border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900 flex items-center justify-between gap-3 text-[11px] text-red-700 dark:text-red-200">
              <span>Commands execute without approval.</span>
              <div class="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => updateControlLevel('controlled')}
                  class="px-2 py-1 rounded-md border border-red-200 dark:border-red-800 bg-surface dark:bg-red-900 text-[10px] font-medium text-red-700 dark:text-red-200 hover:bg-red-100 dark:hover:bg-red-900"
                  aria-label={AI_CHAT_SWITCH_TO_APPROVAL_LABEL}
                >
                  Switch to Approval
                </button>
                <button
                  type="button"
                  onClick={() => setAutonomousBannerDismissed(true)}
                  class="p-1 rounded-md text-red-400 hover:text-red-600 dark:hover:text-red-200 hover:bg-red-100 dark:hover:bg-red-900 transition-colors"
                  title={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                  aria-label={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                >
                  <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M6 18L18 6M6 6l12 12"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </Show>

          <Show when={providerReadinessPresentation()}>
            {(presentation) => (
              <section
                class={`border-b px-4 py-2.5 text-[11px] ${
                  presentation().tone === 'checking'
                    ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200'
                    : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100'
                }`}
                aria-label="Assistant provider status"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div class="flex min-w-0 items-start gap-2.5">
                    <span
                      class={`mt-1 h-2 w-2 flex-shrink-0 rounded-full ${
                        presentation().tone === 'checking'
                          ? 'bg-blue-500 dark:bg-blue-300'
                          : 'bg-amber-500 dark:bg-amber-300'
                      }`}
                    />
                    <div class="min-w-0">
                      <div class="font-semibold text-base-content">{presentation().title}</div>
                      <div class="mt-0.5 leading-5">{presentation().body}</div>
                      <Show when={presentation().recommendation}>
                        {(recommendation) => <div class="mt-0.5 leading-5">{recommendation()}</div>}
                      </Show>
                    </div>
                  </div>
                  <Show when={providerReadiness().status === 'error'}>
                    <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                      <Show when={providerReadinessAlternative()}>
                        {(alternative) => (
                          <button
                            type="button"
                            onClick={switchToProviderReadinessAlternative}
                            class="inline-flex max-w-[11rem] items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                            aria-label={`Use ${alternative().providerLabel} provider route`}
                            title={alternative().label}
                          >
                            <span class="truncate">Use {alternative().providerLabel}</span>
                          </button>
                        )}
                      </Show>
                      <button
                        type="button"
                        onClick={retrySelectedProviderReadiness}
                        class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                        aria-label="Retry provider check"
                      >
                        <RefreshCwIcon class="h-3.5 w-3.5" />
                        <span>{AI_CHAT_PROVIDER_READINESS_RETRY_LABEL}</span>
                      </button>
                      <a
                        href={AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF}
                        class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                      >
                        <SettingsIcon class="h-3.5 w-3.5" />
                        <span>{AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL}</span>
                      </a>
                    </div>
                  </Show>
                </div>
              </section>
            )}
          </Show>

          {/* Discovery hint - show when discovery is disabled */}
          <Show when={discoveryEnabled() === false && !discoveryHintDismissed()}>
            <div class="px-4 py-2 border-b border-cyan-200 dark:border-cyan-800 bg-cyan-50 dark:bg-cyan-900 flex items-center justify-between gap-3 text-[11px] text-cyan-700 dark:text-cyan-200">
              <div class="flex items-center gap-2">
                <svg
                  class="w-4 h-4 text-cyan-500 dark:text-cyan-400 flex-shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <span>
                  <span class="font-medium">{AI_CHAT_DISCOVERY_HINT_TITLE}</span>{' '}
                  {AI_CHAT_DISCOVERY_HINT_BODY}
                </span>
              </div>
              <button
                type="button"
                onClick={() => setDiscoveryHintDismissed(true)}
                class="p-1 rounded hover:bg-cyan-100 dark:hover:bg-cyan-800 text-cyan-500 dark:text-cyan-400"
                title={AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL}
                aria-label={AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL}
              >
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </Show>

          <Show when={contextBriefing()}>
            <section
              class="border-b border-border bg-surface px-4 py-2.5"
              aria-label="Assistant context"
            >
              <div class="flex flex-wrap items-center gap-2 text-[10px] font-medium uppercase text-muted">
                <span>{contextBriefing()!.sourceLabel}</span>
                <Show when={contextBriefing()!.statusLabel}>
                  <span class="h-1 w-1 rounded-full bg-border" />
                  <span class="normal-case">{contextBriefing()!.statusLabel}</span>
                </Show>
              </div>
              <div class="mt-1 text-sm font-semibold text-base-content">
                {contextBriefingTitle()}
              </div>
              <Show when={contextBriefingDetailLines().length > 0}>
                <div class="mt-1 space-y-0.5 text-xs text-muted">
                  <For each={contextBriefingDetailLines()}>{(line) => <div>{line}</div>}</For>
                </div>
              </Show>
              <Show when={contextBriefingNote()}>
                <div class="mt-0.5 text-xs text-muted">{contextBriefingNote()}</div>
              </Show>
              <Show when={contextBriefing()!.actionHref && contextBriefing()!.actionLabel}>
                <a
                  href={contextBriefing()!.actionHref}
                  class="mt-2 inline-flex rounded border border-border bg-surface px-2 py-1 text-[11px] font-medium text-base-content hover:bg-surface-alt"
                >
                  {contextBriefing()!.actionLabel}
                </a>
              </Show>
            </section>
          </Show>

          {/* Messages */}
          <ChatMessages
            messages={chat.messages()}
            onApprove={handleApprove}
            onSkip={handleSkip}
            onAnswerQuestion={handleAnswerQuestion}
            onSkipQuestion={handleSkipQuestion}
            onRetry={(messageId) => chat.retryMessage(messageId)}
            onChangeModel={openModelSelectorFromError}
            getModelRouteLabel={formatChatMessageModelRoute}
            getModelRouteAlternative={getFailedTurnModelRouteAlternative}
            onUseModelRoute={switchToModelRoute}
            queuedFollowUps={chat.queuedFollowUps()}
            onEditQueuedFollowUp={editQueuedFollowUp}
            onCancelQueuedFollowUp={(id) => {
              chat.cancelQueuedFollowUp(id);
              focusComposer();
            }}
            recentSessions={sessions()
              .filter((s) => s.id !== chat.sessionId() && s.message_count > 0)
              .slice(0, 3)}
            onLoadSession={handleLoadSession}
          />

          {/* Status indicator bar */}
          <Show when={currentStatus()}>
            <div
              class="px-4 py-2 bg-surface-alt border-t border-border flex min-w-0 items-center gap-2.5 text-xs"
              role="status"
              aria-label="Assistant active turn status"
              aria-live="polite"
            >
              {/* Status icon based on type */}
              <Show when={currentStatus()?.type === 'thinking'}>
                <div class="flex items-center justify-center w-4 h-4">
                  <svg
                    class="w-3.5 h-3.5 text-blue-500 dark:text-blue-400 animate-pulse"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
                    />
                  </svg>
                </div>
              </Show>
              <Show when={currentStatus()?.type === 'tool'}>
                <div class="flex items-center justify-center w-4 h-4">
                  <svg
                    class="w-3.5 h-3.5 text-blue-500 dark:text-blue-400 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="3"
                    />
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    />
                  </svg>
                </div>
              </Show>
              <Show when={currentStatus()?.type === 'generating'}>
                <div class="flex items-center justify-center w-4 h-4">
                  <svg
                    class="w-3.5 h-3.5 text-emerald-500 dark:text-emerald-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                    />
                  </svg>
                </div>
              </Show>

              <span class="min-w-0 truncate text-muted font-medium">{currentStatusText()}</span>

              {/* Subtle animated dots */}
              <div class="flex gap-0.5 ml-1">
                <span
                  class="w-1 h-1 rounded-full bg-slate-400 animate-bounce"
                  style="animation-delay: 0ms; animation-duration: 1s"
                />
                <span
                  class="w-1 h-1 rounded-full bg-slate-400 animate-bounce"
                  style="animation-delay: 150ms; animation-duration: 1s"
                />
                <span
                  class="w-1 h-1 rounded-full bg-slate-400 animate-bounce"
                  style="animation-delay: 300ms; animation-duration: 1s"
                />
              </div>
            </div>
          </Show>

          {/* Input */}
          <div class="border-t border-border bg-surface px-4 py-3">
            <Show when={chat.queuedFollowUpCount() > 0}>
              <div
                class="mb-2 rounded-md border border-blue-200 bg-blue-50 px-2.5 py-1.5 text-blue-800 shadow-sm dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200"
                role="status"
                aria-label="Queued follow-up messages"
              >
                <div class="flex min-h-7 items-center gap-2">
                  <ClockIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                  <span class="min-w-0 flex-1 truncate text-xs font-medium">
                    {pluralizeCount(chat.queuedFollowUpCount(), 'follow-up', 'follow-ups')} queued
                  </span>
                  <button
                    type="button"
                    onClick={() => {
                      chat.clearQueuedFollowUps();
                      focusComposer();
                    }}
                    class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-900 dark:text-blue-200 dark:hover:bg-blue-900/50"
                    title="Clear queued follow-ups"
                    aria-label="Clear queued follow-up messages"
                  >
                    <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                  </button>
                </div>
                <div class="mt-1 max-h-24 space-y-1 overflow-y-auto">
                  <For each={chat.queuedFollowUps()}>
                    {(queued) => {
                      const preview = () => queuedFollowUpPreview(queued.prompt);
                      return (
                        <div class="flex min-h-7 items-center gap-2 rounded-md bg-white/70 px-2 py-1 text-xs text-blue-900 dark:bg-blue-900/30 dark:text-blue-100">
                          <span class="min-w-0 flex-1 truncate">{preview()}</span>
                          <button
                            type="button"
                            onClick={() => editQueuedFollowUp(queued.id)}
                            class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60"
                            title="Edit queued follow-up"
                            aria-label={`Edit queued follow-up: ${preview()}`}
                          >
                            <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
                          </button>
                          <button
                            type="button"
                            onClick={() => {
                              chat.cancelQueuedFollowUp(queued.id);
                              focusComposer();
                            }}
                            class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60"
                            title="Remove queued follow-up"
                            aria-label={`Remove queued follow-up: ${preview()}`}
                          >
                            <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                          </button>
                        </div>
                      );
                    }}
                  </For>
                </div>
              </div>
            </Show>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleSubmit();
              }}
              class="relative"
            >
              <div
                class={`relative flex min-h-[56px] items-end rounded-lg border bg-surface-alt shadow-sm transition-colors ${
                  mentionActive()
                    ? 'border-blue-400 ring-2 ring-blue-500/20'
                    : 'border-border focus-within:border-blue-500 focus-within:ring-2 focus-within:ring-blue-500/20'
                }`}
              >
                <textarea
                  ref={textareaRef}
                  value={input()}
                  onInput={handleInputChange}
                  onKeyDown={handleKeyDown}
                  placeholder={AI_CHAT_INPUT_PLACEHOLDER}
                  rows={1}
                  class="max-h-40 min-h-[54px] flex-1 resize-none bg-transparent px-3.5 py-3.5 pr-24 text-sm leading-5 text-base-content placeholder-slate-400 focus:outline-none"
                />
                <div data-mention-autocomplete>
                  <MentionAutocomplete
                    query={mentionQuery()}
                    resources={mentionResources()}
                    position={{ top: 58, left: 0 }}
                    onSelect={handleMentionSelect}
                    onClose={() => setMentionActive(false)}
                    visible={mentionActive()}
                  />
                </div>
                <div class="absolute bottom-2 right-2 flex items-center gap-1.5">
                  <Show when={chat.isLoading()}>
                    <button
                      type="button"
                      onClick={stopActiveResponse}
                      class={`flex h-9 w-9 items-center justify-center rounded-md border bg-surface text-base-content shadow-sm transition-colors hover:bg-surface-hover ${
                        interruptArmed()
                          ? 'border-blue-400 ring-2 ring-blue-500/30'
                          : 'border-border'
                      }`}
                      title={interruptArmed() ? 'Stop response armed' : 'Stop'}
                      aria-label={interruptArmed() ? 'Stop response armed' : 'Stop response'}
                    >
                      <SquareIcon class="h-4 w-4" />
                    </button>
                  </Show>
                  <button
                    type="submit"
                    disabled={!input().trim()}
                    class="flex h-9 w-9 items-center justify-center rounded-md bg-blue-600 text-white shadow-sm transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-45"
                    title={chat.isLoading() ? 'Queue follow-up' : 'Send'}
                    aria-label={chat.isLoading() ? 'Queue follow-up' : 'Send message'}
                  >
                    <SendIcon class="h-4 w-4" />
                  </button>
                </div>
              </div>
            </form>
            <div
              class="mt-1.5 flex min-h-7 min-w-0 flex-wrap items-center justify-between gap-2"
              data-testid="assistant-composer-chrome"
            >
              <div
                class="flex min-w-0 flex-1 flex-wrap items-center gap-1.5"
                data-testid="assistant-composer-route-controls"
              >
                <ModelSelector
                  models={aiRuntimeModels()}
                  selectedModel={chat.model()}
                  defaultModel={defaultModel()}
                  defaultModelLabel={defaultModelLabel()}
                  chatOverrideModel={chatOverrideModel()}
                  chatOverrideLabel={chatOverrideLabel()}
                  recentModelIds={recentModelIds()}
                  isLoading={aiRuntimeModelsLoading()}
                  error={aiRuntimeModelsError()}
                  openRequest={modelSelectorOpenRequest()}
                  onModelSelect={selectModel}
                  onRefresh={() => loadModels(true)}
                />

                <div class="relative" data-dropdown>
                  <button
                    type="button"
                    onClick={() => setShowControlMenu(!showControlMenu())}
                    class={`flex flex-shrink-0 items-center gap-1.5 px-2.5 py-1.5 text-[11px] font-medium rounded-md border transition-colors ${controlPresentation().pillClassName} ${controlSaving() ? 'opacity-70 cursor-wait' : 'hover:opacity-90'}`}
                    title={AI_CHAT_CONTROL_MODE_LABEL}
                    aria-label={`${AI_CHAT_CONTROL_MODE_LABEL}: ${controlPresentation().label}`}
                    aria-haspopup="menu"
                    aria-expanded={showControlMenu()}
                    disabled={controlSaving()}
                  >
                    <span
                      class={`h-1.5 w-1.5 rounded-full ${controlPresentation().dotClassName}`}
                    />
                    <span>{controlPresentation().label}</span>
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        stroke-width="2"
                        d="M19 9l-7 7-7-7"
                      />
                    </svg>
                  </button>

                  <Show when={showControlMenu()}>
                    <div
                      class="absolute bottom-full left-0 z-50 mb-2 w-60 overflow-hidden rounded-md border border-border bg-surface shadow-sm"
                      role="menu"
                      aria-label={AI_CHAT_CONTROL_MODE_MENU_LABEL}
                    >
                      <div class="border-b border-border px-3 py-2 text-[11px] text-muted">
                        Default control mode
                      </div>
                      <button
                        type="button"
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'read_only'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'read_only' ? getAIChatControlLevelPresentation('read_only').selectedClassName : ''}`}
                        onClick={() => updateControlLevel('read_only')}
                      >
                        <div class="font-medium text-base-content">
                          {getAIChatControlLevelPresentation('read_only').label}
                        </div>
                        <div class="text-[11px] text-muted">
                          {getAIChatControlLevelPresentation('read_only').description}
                        </div>
                      </button>
                      <button
                        type="button"
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'controlled'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'controlled' ? getAIChatControlLevelPresentation('controlled').selectedClassName : ''}`}
                        onClick={() => updateControlLevel('controlled')}
                      >
                        <div class="font-medium text-base-content">
                          {getAIChatControlLevelPresentation('controlled').label}
                        </div>
                        <div class="text-[11px] text-muted">
                          {getAIChatControlLevelPresentation('controlled').description}
                        </div>
                      </button>
                      <button
                        type="button"
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'autonomous'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'autonomous' ? getAIChatControlLevelPresentation('autonomous').selectedClassName : ''}`}
                        onClick={() => updateControlLevel('autonomous')}
                      >
                        <div class="font-medium text-base-content">
                          {getAIChatControlLevelPresentation('autonomous').label}
                        </div>
                        <div class="text-[11px] text-muted">
                          {getAIChatControlLevelPresentation('autonomous').description}
                        </div>
                      </button>
                    </div>
                  </Show>
                </div>
              </div>

              <Show when={lastAssistantUsage()}>
                {(usage) => (
                  <div
                    class="flex min-h-4 min-w-0 items-center justify-end text-[10px] font-medium text-muted"
                    aria-label={usage().title}
                    title={usage().title}
                  >
                    <span class="truncate">{usage().label}</span>
                  </div>
                )}
              </Show>
            </div>
          </div>
        </Show>
      </div>
    </>
  );
};

export default AIChat;
