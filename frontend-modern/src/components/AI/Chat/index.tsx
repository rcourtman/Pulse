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
import { useNavigate } from '@solidjs/router';
import { unwrap } from 'solid-js/store';
import SendIcon from 'lucide-solid/icons/send';
import SquareIcon from 'lucide-solid/icons/square';
import ClockIcon from 'lucide-solid/icons/clock';
import ClipboardCopyIcon from 'lucide-solid/icons/clipboard-copy';
import CopyIcon from 'lucide-solid/icons/copy';
import CpuIcon from 'lucide-solid/icons/cpu';
import DownloadIcon from 'lucide-solid/icons/download';
import GitForkIcon from 'lucide-solid/icons/git-fork';
import PencilIcon from 'lucide-solid/icons/pencil';
import Redo2Icon from 'lucide-solid/icons/redo-2';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import ShieldAlertIcon from 'lucide-solid/icons/shield-alert';
import SparklesIcon from 'lucide-solid/icons/sparkles';
import Undo2Icon from 'lucide-solid/icons/undo-2';
import XIcon from 'lucide-solid/icons/x';
import BellIcon from 'lucide-solid/icons/bell';
import BellOffIcon from 'lucide-solid/icons/bell-off';
import BookmarkIcon from 'lucide-solid/icons/bookmark';
import CheckIcon from 'lucide-solid/icons/check';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import LoaderCircleIcon from 'lucide-solid/icons/loader-circle';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import PlusIcon from 'lucide-solid/icons/plus';
import Trash2Icon from 'lucide-solid/icons/trash-2';
import WrenchIcon from 'lucide-solid/icons/wrench';
import { AIAPI } from '@/api/ai';
import {
  AIChatAPI,
  type ChatMention,
  type ChatSession,
  type ChatSessionHandoffSummary,
} from '@/api/aiChat';
import {
  fetchAgentCapabilitiesManifest,
  getAgentSurfaceToolPosturePresentation,
  getAgentWorkflowPrompts,
  type AgentSurfaceToolContract,
  type AgentWorkflowPrompt,
} from '@/api/agentCapabilities';
import { ActionIconButton } from '@/components/shared/Button';
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
  AI_CHAT_COMMAND_HELP_BUTTON_LABEL,
  AI_CHAT_CONTROL_MODE_LABEL,
  AI_CHAT_CONTROL_MODE_MENU_LABEL,
  AI_CHAT_COPY_LAST_ANSWER_ERROR_MESSAGE,
  AI_CHAT_COPY_LAST_ANSWER_LABEL,
  AI_CHAT_COPY_LAST_ANSWER_SUCCESS_MESSAGE,
  AI_CHAT_COPY_TRANSCRIPT_LABEL,
  AI_CHAT_DISCOVERY_HINT_BODY,
  AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL,
  AI_CHAT_DISCOVERY_HINT_TITLE,
  AI_CHAT_DRAWER_TITLE,
  AI_CHAT_EXPORT_TRANSCRIPT_LABEL,
  AI_CHAT_FORK_SESSION_EMPTY_MESSAGE,
  AI_CHAT_FORK_SESSION_ERROR_MESSAGE,
  AI_CHAT_FORK_SESSION_LABEL,
  AI_CHAT_FORK_SESSION_LOAD_ERROR_MESSAGE,
  AI_CHAT_FORK_SESSION_LOADING_MESSAGE,
  AI_CHAT_FORK_SESSION_SUCCESS_MESSAGE,
  AI_CHAT_INPUT_PLACEHOLDER,
  AI_CHAT_NEW_SESSION_BUTTON_TITLE,
  AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL,
  AI_CHAT_NEW_SESSION_MENU_LABEL,
  AI_CHAT_NEW_SESSION_SHORT_LABEL,
  AI_CHAT_PROVIDER_READINESS_RETRY_LABEL,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF,
  AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL,
  AI_CHAT_REDO_LAST_TURN_EMPTY_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_ERROR_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_LABEL,
  AI_CHAT_REDO_LAST_TURN_LOADING_MESSAGE,
  AI_CHAT_REDO_LAST_TURN_SUCCESS_MESSAGE,
  AI_CHAT_RENAME_SESSION_CANCEL_LABEL,
  AI_CHAT_RENAME_SESSION_EMPTY_MESSAGE,
  AI_CHAT_RENAME_SESSION_ERROR_MESSAGE,
  AI_CHAT_RENAME_SESSION_LABEL,
  AI_CHAT_RENAME_SESSION_SAVE_LABEL,
  AI_CHAT_SESSION_MENU_TITLE,
  AI_CHAT_SESSION_EMPTY_STATE,
  AI_CHAT_SESSION_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_EMPTY_STATE,
  AI_CHAT_SESSION_SEARCH_ERROR_STATE,
  AI_CHAT_SESSION_SEARCH_LOADING_STATE,
  AI_CHAT_SESSION_SEARCH_PLACEHOLDER,
  AI_CHAT_SESSION_SEARCH_TITLE,
  AI_CHAT_SWITCH_TO_APPROVAL_LABEL,
  AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL,
  AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL,
  AI_CHAT_TRANSCRIPT_FALLBACK_TEXTAREA_LABEL,
  AI_CHAT_TRANSCRIPT_FALLBACK_TITLE,
  AI_CHAT_UNDO_LAST_TURN_EMPTY_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_ERROR_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_LABEL,
  AI_CHAT_UNDO_LAST_TURN_LOADING_MESSAGE,
  AI_CHAT_UNDO_LAST_TURN_SUCCESS_MESSAGE,
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
import { copyToClipboard } from '@/utils/clipboard';
import { getAssistantTurnSummary } from './assistantTurnSummary';
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import {
  latestExplicitModelRouteFromTranscript,
  useChat,
  type QueuedFollowUp,
  type RestoredPromptDraft,
  type SendMessageOptions,
} from './hooks/useChat';
import { ChatMessages } from './ChatMessages';
import { AssistantCommandHelpDialog } from './AssistantCommandHelpDialog';
import { ModelSelector } from './ModelSelector';
import { MentionAutocomplete, type MentionResource } from './MentionAutocomplete';
import { SlashCommandAutocomplete } from './SlashCommandAutocomplete';
import { getAssistantActiveTurnStatus } from './activeTurnStatus';
import { selectQuickResumeSessions } from './recentSessionsModel';
import {
  createPacedWorkflowStatus,
  replaceLatestWorkflowStatusEventForDisplay,
  WORKFLOW_STATUS_REFRESH_MS,
} from './workflowStatusDisplay';
import { getLastAssistantAnswerText } from './assistantAnswerText';
import {
  getNextAssistantRecentModelRoute,
  isAssistantExplicitModelRoute,
  normalizeAssistantModelRouteArgument,
  normalizeAssistantRecentModelRoutes,
} from './assistantModelRoutes';
import {
  type AssistantSlashCommandAvailability,
  parseAssistantSlashCommandInput,
  type AssistantSlashCommand,
  type AssistantSlashCommandAction,
} from './assistantSlashCommands';
import { getAssistantWorkflowStarters, type AssistantWorkflowStarter } from './workflowStarters';
import {
  composePromptWithPastedBlocks,
  createPastedTextBlock,
  pastedBlockLabel,
  shouldCollapsePastedText,
  type PastedTextBlock,
} from './composerPaste';
import {
  assistantNotificationsEnabled,
  assistantNotificationsSupported,
  setAssistantNotificationsEnabled,
} from './assistantNotifications';
import {
  buildAssistantTranscriptFilename,
  downloadAssistantTranscriptFile,
  formatAssistantTranscript,
  hasAssistantTranscriptContent,
} from './transcriptExport';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
} from './types';
import { formatIdentifierLabel } from '@/utils/textPresentation';

const MODEL_SESSION_STORAGE_KEY = 'pulse:ai_chat_models_by_session';
const MODEL_RECENT_STORAGE_KEY = 'pulse:ai_chat_recent_models';
const SESSION_PINNED_STORAGE_KEY = 'pulse:ai_chat_pinned_sessions';
const PROMPT_HISTORY_STORAGE_KEY = 'pulse:ai_chat_prompt_history';
const DEFAULT_SESSION_KEY = '__default__';
const AI_CHAT_MIN_DOCKED_VIEWPORT_WIDTH = 1200;
const AI_CHAT_PROMPT_HISTORY_LIMIT = 100;
const AI_CHAT_RECENT_MODEL_LIMIT = 8;
const AI_CHAT_PINNED_SESSION_LIMIT = 30;
const AI_CHAT_SESSION_TITLE_MAX_LENGTH = 120;
const AI_CHAT_SESSION_SEARCH_DEBOUNCE_MS = 150;
const AI_CHAT_SESSION_SEARCH_LIMIT = 30;
const STRUCTURED_PATROL_CONTEXT_TARGETS = new Set(['patrol-configuration', 'patrol-run']);
const STRUCTURED_RESOURCE_CONTEXT_HANDOFF_KINDS = new Set(['resource_context']);
const AI_CHAT_CYCLE_RECENT_MODEL_LABEL = 'Cycle recent Assistant model';
const AI_CHAT_CONTROL_LEVEL_ORDER: AIControlLevel[] = ['read_only', 'controlled', 'autonomous'];
const AI_CHAT_COMPACT_SESSION_LABEL = 'Compact session';
const AI_CHAT_COMPACT_SESSION_EMPTY_MESSAGE = 'No Assistant session to compact';
const AI_CHAT_COMPACT_SESSION_LOADING_MESSAGE =
  'Wait for the active Assistant response before compacting this session';
const AI_CHAT_COMPACT_SESSION_START_MESSAGE = 'Compacting Assistant session...';
const AI_CHAT_COMPACT_SESSION_SUCCESS_MESSAGE = 'Assistant session compacted';
const AI_CHAT_COMPACT_SESSION_ERROR_MESSAGE = 'Failed to compact Assistant session';
const AI_CHAT_COMPACT_SESSION_LOAD_ERROR_MESSAGE = 'Session compacted, but reload failed';

type SessionPickerSection = {
  title: string;
  sessions: ChatSession[];
};

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

interface AssistantProviderRouteHealthPresentation {
  label: string;
  title: string;
  className: string;
  dotClassName: string;
}

interface AssistantActiveRoutePresentation {
  label: string;
  title: string;
}

type AssistantSurfaceToolsStatus = 'idle' | 'loading' | 'ready' | 'unavailable';

interface AssistantSurfaceToolsState {
  status: AssistantSurfaceToolsStatus;
  contract?: AgentSurfaceToolContract;
  message?: string;
}

interface AssistantSurfaceToolHealthPresentation {
  label: string;
  title: string;
  className: string;
  dotClassName: string;
  iconClassName: string;
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
  restoredPromptDraft: RestoredPromptDraft | null;
}

interface TranscriptCopyFallback {
  generatedAt: Date;
  transcript: string;
}

interface AIChatProps {
  onClose: () => void;
}

let stashedComposerDraft: ComposerDraftStash | null = null;

const PROVIDER_ROUTE_FAILURE_PATTERN =
  /\b(ai provider|provider endpoint|provider credentials|provider api key|provider connection|selected provider url|provider url|model route|openrouter|openai|anthropic|deepseek|gemini|ollama|api key|rate limit|quota|credits?|upstream|llm|429|402)\b/i;
const LOCAL_ASSISTANT_FAILURE_PATTERN = /\bUnknown Assistant fixture\b|\bAvailable fixtures:\b/i;

export const resetAIChatComposerDraftStashForTests = () => {
  stashedComposerDraft = null;
};

const hasProviderRouteFailureEvidence = (message: ChatMessage): boolean => {
  if (message.role !== 'assistant' || !message.error) return false;
  if (LOCAL_ASSISTANT_FAILURE_PATTERN.test(message.error)) return false;
  if (PROVIDER_ROUTE_FAILURE_PATTERN.test(message.error)) return true;

  return (message.streamEvents || []).some(
    (event) =>
      event.type === 'model_switch' &&
      (Boolean(event.failedModel?.trim()) || event.modelEvent === 'switch'),
  );
};

const compactText = (items: Array<string | undefined>): string[] =>
  items.filter((item): item is string => typeof item === 'string' && item.trim().length > 0);

const pluralizeCount = (count: number, singular: string, plural: string) =>
  `${count} ${count === 1 ? singular : plural}`;

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

  const candidateKey = normalizeComparableModelKey(candidate.model.id);
  return {
    id: candidate.model.id,
    kind: candidateKey === selectedKey ? 'same-model-route' : 'alternate-model-route',
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
        ? 'Patrol mode save failure'
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
              ? 'Patrol mode'
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
          ? 'Review Patrol mode issue'
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
  const navigate = useNavigate();
  // UI state - use store's isOpenSignal for reactivity
  const isOpen = aiChatStore.isOpenSignal;
  const { width } = useBreakpoint();
  const [input, setInput] = createSignal('');
  const [editingQueuedFollowUp, setEditingQueuedFollowUp] = createSignal<QueuedFollowUp | null>(
    null,
  );
  const [queuedFollowUpCommandTargetId, setQueuedFollowUpCommandTargetId] = createSignal<
    string | null
  >(null);
  const [restoredPromptDraft, setRestoredPromptDraft] = createSignal<RestoredPromptDraft | null>(
    null,
  );
  const [interruptArmed, setInterruptArmed] = createSignal(false);
  const [promptHistory, setPromptHistory] = createSignal<PromptHistoryEntry[]>([]);
  const [promptHistoryIndex, setPromptHistoryIndex] = createSignal(-1);
  const [savedPromptDraft, setSavedPromptDraft] = createSignal<PromptHistoryEntry | null>(null);
  const [sessions, setSessions] = createSignal<ChatSession[]>([]);
  const [showSessions, setShowSessions] = createSignal(false);
  const [showCommandHelp, setShowCommandHelp] = createSignal(false);
  const [sessionRefreshLoading, setSessionRefreshLoading] = createSignal(false);
  const [sessionSearchQuery, setSessionSearchQuery] = createSignal('');
  const [sessionSearchResults, setSessionSearchResults] = createSignal<ChatSession[] | null>(null);
  const [sessionSearchLoading, setSessionSearchLoading] = createSignal(false);
  const [sessionSearchError, setSessionSearchError] = createSignal('');
  const [renamingSessionId, setRenamingSessionId] = createSignal('');
  const [sessionRenameDraft, setSessionRenameDraft] = createSignal('');
  const [sessionRenameSaving, setSessionRenameSaving] = createSignal(false);
  const [sessionDropdownPosition, setSessionDropdownPosition] = createSignal({ top: 0, right: 0 });
  const [forkingSession, setForkingSession] = createSignal(false);
  const [compactingSession, setCompactingSession] = createSignal(false);
  const [undoingLastTurn, setUndoingLastTurn] = createSignal(false);
  const [redoingLastTurn, setRedoingLastTurn] = createSignal(false);
  const [redoLastTurnAvailable, setRedoLastTurnAvailable] = createSignal(false);
  let sessionButtonRef: HTMLButtonElement | undefined;
  let sessionSearchInputRef: HTMLInputElement | undefined;
  let sessionRenameInputRef: HTMLInputElement | undefined;
  let controlModeButtonRef: HTMLButtonElement | undefined;
  const sessionOptionRefs = new Map<string, HTMLButtonElement>();
  const controlModeOptionRefs = new Map<AIControlLevel, HTMLButtonElement>();
  let sessionSearchRequestId = 0;
  const [modelSelectorOpenRequest, setModelSelectorOpenRequest] = createSignal(0);
  const [modelSelectorInitialSearch, setModelSelectorInitialSearch] = createSignal('');
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');
  const [providerReadiness, setProviderReadiness] = createSignal<ChatProviderReadinessState>({
    status: 'idle',
    provider: '',
  });
  const [providerReadinessVisible, setProviderReadinessVisible] = createSignal(false);
  const [providerReadinessRetryNonce, setProviderReadinessRetryNonce] = createSignal(0);
  const [assistantSurfaceTools, setAssistantSurfaceTools] =
    createSignal<AssistantSurfaceToolsState>({
      status: 'idle',
    });
  const [controlLevel, setControlLevel] = createSignal<AIControlLevel>('read_only');
  const [showControlMenu, setShowControlMenu] = createSignal(false);
  const [controlSaving, setControlSaving] = createSignal(false);
  const [transcriptCopyFallback, setTranscriptCopyFallback] =
    createSignal<TranscriptCopyFallback | null>(null);
  const [discoveryEnabled, setDiscoveryEnabled] = createSignal<boolean | null>(null); // null = loading
  const [discoveryHintDismissed, setDiscoveryHintDismissed] = createSignal(false);
  const [autonomousBannerDismissed, setAutonomousBannerDismissed] = createSignal(false);
  const [workflowPrompts, setWorkflowPrompts] = createSignal<AgentWorkflowPrompt[]>([]);
  const [renderingWorkflowStarterId, setRenderingWorkflowStarterId] = createSignal('');
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
  const [pastedBlocks, setPastedBlocks] = createSignal<PastedTextBlock[]>([]);
  const [slashCommandActive, setSlashCommandActive] = createSignal(false);
  const [slashCommandQuery, setSlashCommandQuery] = createSignal('');
  let textareaRef: HTMLTextAreaElement | undefined;
  let transcriptFallbackTextareaRef: HTMLTextAreaElement | undefined;
  let interruptArmTimeout: ReturnType<typeof setTimeout> | undefined;
  let queuedFollowUpCommandTargetTimeout: ReturnType<typeof setTimeout> | undefined;
  let composerSubmitDispatchLocked = false;
  let handledAssistantCommandRequestId = 0;
  const queuedFollowUpRowRefs = new Map<string, HTMLDivElement>();

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

  const markQueuedFollowUpCommandTarget = (id: string | null) => {
    if (queuedFollowUpCommandTargetTimeout) {
      clearTimeout(queuedFollowUpCommandTargetTimeout);
      queuedFollowUpCommandTargetTimeout = undefined;
    }
    setQueuedFollowUpCommandTargetId(id);
    if (!id) return;
    queuedFollowUpCommandTargetTimeout = setTimeout(() => {
      queuedFollowUpCommandTargetTimeout = undefined;
      setQueuedFollowUpCommandTargetId((current) => (current === id ? null : current));
    }, 2500);
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

  const cloneRestoredPromptDraft = (
    draft: RestoredPromptDraft | null,
  ): RestoredPromptDraft | null => {
    if (!draft) return null;
    const request = draft.request;
    return {
      prompt: draft.prompt,
      request: request
        ? {
            ...request,
            mentions: request.mentions?.map((mention) => ({ ...mention })),
            handoffResources: request.handoffResources?.map((resource) => ({ ...resource })),
            handoffActions: request.handoffActions?.map((action) => ({ ...action })),
            handoffMetadata: request.handoffMetadata ? { ...request.handoffMetadata } : undefined,
          }
        : undefined,
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

  const isTransientSlashCommandDraft = () => {
    const text = input();
    const cursor = textareaRef?.selectionStart ?? text.length;
    const textBeforeCursor = text.slice(0, cursor);
    const textAfterCursor = text.slice(cursor);
    return (
      textBeforeCursor.startsWith('/') && !/\s/.test(textBeforeCursor) && !textAfterCursor.trim()
    );
  };

  const closeSlashCommandAutocomplete = (options?: { clearTransientDraft?: boolean }) => {
    const shouldClearDraft = Boolean(
      options?.clearTransientDraft && isTransientSlashCommandDraft(),
    );
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    if (!shouldClearDraft) return;

    setInput('');
    setAccumulatedMentions([]);
    resetPromptHistoryNavigation();
    queueMicrotask(() => {
      resizeTextarea();
      textareaRef?.focus();
      textareaRef?.setSelectionRange(0, 0);
    });
  };

  const stashComposerDraftForRemount = () => {
    const text = input();
    const mentions = accumulatedMentions();
    const queuedDraft = editingQueuedFollowUp();
    const restoredDraft = restoredPromptDraft();
    if (!text.trim() && mentions.length === 0 && !queuedDraft && !restoredDraft) {
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
      restoredPromptDraft: cloneRestoredPromptDraft(restoredDraft),
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
    setRestoredPromptDraft(cloneRestoredPromptDraft(draft.restoredPromptDraft));
    setMentionActive(false);
    setSlashCommandActive(false);
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
    setSlashCommandActive(false);
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
    if (inHistory) {
      return direction === 'up' ? cursor === 0 : cursor === text.length;
    }
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
      return normalizeAssistantRecentModelRoutes(parsed, AI_CHAT_RECENT_MODEL_LIMIT);
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
    if (!isAssistantExplicitModelRoute(normalizedModelId)) return;
    setRecentModelIds((prev) => {
      const next = [
        normalizedModelId,
        ...prev.filter((candidate) => candidate !== normalizedModelId),
      ].slice(0, AI_CHAT_RECENT_MODEL_LIMIT);
      persistRecentModelIds(next);
      return next;
    });
  };

  const loadPinnedSessionIds = (): string[] => {
    try {
      const raw = localStorage.getItem(SESSION_PINNED_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) : [];
      if (!Array.isArray(parsed)) return [];
      const seen = new Set<string>();
      const sessionIds: string[] = [];
      for (const value of parsed) {
        const sessionId = typeof value === 'string' ? value.trim() : '';
        if (!sessionId || seen.has(sessionId)) continue;
        seen.add(sessionId);
        sessionIds.push(sessionId);
        if (sessionIds.length >= AI_CHAT_PINNED_SESSION_LIMIT) break;
      }
      return sessionIds;
    } catch (error) {
      logger.warn('[AIChat] Failed to read pinned sessions:', error);
      return [];
    }
  };

  const persistPinnedSessionIds = (sessionIds: string[]) => {
    try {
      if (sessionIds.length > 0) {
        localStorage.setItem(SESSION_PINNED_STORAGE_KEY, JSON.stringify(sessionIds));
      } else {
        localStorage.removeItem(SESSION_PINNED_STORAGE_KEY);
      }
    } catch (error) {
      logger.warn('[AIChat] Failed to persist pinned sessions:', error);
    }
  };

  const [pinnedSessionIds, setPinnedSessionIds] = createSignal<string[]>(loadPinnedSessionIds());

  const isSessionPinned = (sessionId: string) => pinnedSessionIds().includes(sessionId);

  const removePinnedSession = (sessionId: string) => {
    const normalizedSessionId = sessionId.trim();
    if (!normalizedSessionId) return;
    setPinnedSessionIds((prev) => {
      const next = prev.filter((candidate) => candidate !== normalizedSessionId);
      if (next.length === prev.length) return prev;
      persistPinnedSessionIds(next);
      return next;
    });
  };

  const togglePinnedSession = (sessionId: string, event: Event) => {
    event.stopPropagation();
    const normalizedSessionId = sessionId.trim();
    if (!normalizedSessionId) return;
    setPinnedSessionIds((prev) => {
      const isPinned = prev.includes(normalizedSessionId);
      const next = isPinned
        ? prev.filter((candidate) => candidate !== normalizedSessionId)
        : [
            normalizedSessionId,
            ...prev.filter((candidate) => candidate !== normalizedSessionId),
          ].slice(0, AI_CHAT_PINNED_SESSION_LIMIT);
      persistPinnedSessionIds(next);
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
  const rawSessionPickerSessions = createMemo(() =>
    normalizedSessionSearchQuery() ? (sessionSearchResults() ?? []) : sessions(),
  );
  const getSessionUpdatedTimestamp = (session: ChatSession) => {
    const value = Date.parse(session.updated_at || '');
    return Number.isFinite(value) ? value : 0;
  };
  const sortSessionsByRecency = (sessionList: ChatSession[]) =>
    sessionList
      .map((session, index) => ({
        session,
        index,
        updatedAt: getSessionUpdatedTimestamp(session),
      }))
      .sort((a, b) => {
        const recency = b.updatedAt - a.updatedAt;
        return recency || a.index - b.index;
      })
      .map((entry) => entry.session);
  const getSessionSectionTitle = (session: ChatSession) => {
    const timestamp = getSessionUpdatedTimestamp(session);
    if (!timestamp) return 'Recent';
    const updatedAt = new Date(timestamp);
    const today = new Date();
    if (updatedAt.toDateString() === today.toDateString()) return 'Today';
    const dateOptions: Intl.DateTimeFormatOptions = {
      month: 'short',
      day: 'numeric',
    };
    if (updatedAt.getFullYear() !== today.getFullYear()) {
      dateOptions.year = 'numeric';
    }
    return updatedAt.toLocaleDateString(undefined, dateOptions);
  };
  const sessionPickerSections = createMemo<SessionPickerSection[]>(() => {
    const source = rawSessionPickerSessions();
    if (source.length === 0) return [];

    const sessionMap = new Map(source.map((session) => [session.id, session]));
    const pinnedSessions = pinnedSessionIds().flatMap((sessionId) => {
      const session = sessionMap.get(sessionId);
      return session ? [session] : [];
    });
    const pinnedSet = new Set(pinnedSessions.map((session) => session.id));
    const remaining = sortSessionsByRecency(source.filter((session) => !pinnedSet.has(session.id)));
    const sections: SessionPickerSection[] = [];

    if (pinnedSessions.length > 0) {
      sections.push({ title: 'Pinned', sessions: pinnedSessions });
    }

    for (const session of remaining) {
      const title = getSessionSectionTitle(session);
      const lastSection = sections[sections.length - 1];
      if (lastSection && lastSection.title === title) {
        lastSection.sessions.push(session);
      } else {
        sections.push({ title, sessions: [session] });
      }
    }

    return sections;
  });
  const sessionPickerSessions = createMemo(() =>
    sessionPickerSections().flatMap((section) => section.sessions),
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
  const resetSessionRename = () => {
    setRenamingSessionId('');
    setSessionRenameDraft('');
    setSessionRenameSaving(false);
  };
  const resetSessionSearch = () => {
    sessionSearchRequestId += 1;
    setSessionSearchQuery('');
    setSessionSearchResults(null);
    setSessionSearchLoading(false);
    setSessionSearchError('');
    resetSessionRename();
  };
  const focusSessionSearch = () => {
    queueMicrotask(() => {
      sessionSearchInputRef?.focus();
      window.setTimeout(() => sessionSearchInputRef?.focus(), 0);
    });
  };
  const focusSessionTriggerAfterClose = () => {
    const trigger = sessionButtonRef;
    if (!trigger) return;
    window.setTimeout(() => trigger.focus(), 0);
  };
  const closeSessionPickerAndFocusTrigger = () => {
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    focusSessionTriggerAfterClose();
  };
  const focusSessionRenameInput = () => {
    queueMicrotask(() => {
      sessionRenameInputRef?.focus();
      sessionRenameInputRef?.select();
    });
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
    const sessionList = sessionPickerSessions();
    if (sessionList.length === 0) return false;
    const nextIndex = ((index % sessionList.length) + sessionList.length) % sessionList.length;
    const session = sessionList[nextIndex];
    if (!session) return false;
    sessionOptionRefs.get(session.id)?.focus();
    return true;
  };

  const focusSessionOptionFromSearchByOffset = (offset: number) => {
    const sessionList = sessionPickerSessions();
    if (sessionList.length === 0) return false;
    let nextIndex = offset;
    if (nextIndex < 0) nextIndex = sessionList.length - 1;
    if (nextIndex >= sessionList.length) nextIndex = 0;
    return focusSessionOptionAtIndex(nextIndex);
  };

  const consumeSessionPickerKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  };

  const focusSessionOptionRelativeTo = (sessionId: string, offset: number) => {
    const sessionList = sessionPickerSessions();
    const currentIndex = sessionList.findIndex((session) => session.id === sessionId);
    if (currentIndex < 0) return false;
    return focusSessionOptionAtIndex(currentIndex + offset);
  };

  const handleSessionSearchKeyDown = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'ArrowDown' && focusSessionOptionAtIndex(0)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'ArrowUp' && focusSessionOptionAtIndex(sessionPickerSessions().length - 1)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'PageDown' && focusSessionOptionFromSearchByOffset(10)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'PageUp' && focusSessionOptionFromSearchByOffset(-10)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'Home' && focusSessionOptionAtIndex(0)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'End' && focusSessionOptionAtIndex(sessionPickerSessions().length - 1)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'Escape') {
      consumeSessionPickerKey(event);
      closeSessionPickerAndFocusTrigger();
    }
  };

  const handleSessionOptionKeyDown = (
    event: KeyboardEvent & { currentTarget: HTMLButtonElement },
    sessionId: string,
  ) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;

    if (event.key === 'ArrowDown' && focusSessionOptionRelativeTo(sessionId, 1)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'ArrowUp' && focusSessionOptionRelativeTo(sessionId, -1)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'Home' && focusSessionOptionAtIndex(0)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'End' && focusSessionOptionAtIndex(sessionPickerSessions().length - 1)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'PageDown' && focusSessionOptionRelativeTo(sessionId, 10)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'PageUp' && focusSessionOptionRelativeTo(sessionId, -10)) {
      consumeSessionPickerKey(event);
      return;
    }
    if (event.key === 'Escape') {
      consumeSessionPickerKey(event);
      closeSessionPickerAndFocusTrigger();
    }
  };

  const getCurrentControlLevelIndex = () => {
    const index = AI_CHAT_CONTROL_LEVEL_ORDER.indexOf(controlLevel());
    return index >= 0 ? index : 0;
  };

  const focusControlModeOptionAtIndex = (index: number) => {
    const nextIndex =
      ((index % AI_CHAT_CONTROL_LEVEL_ORDER.length) + AI_CHAT_CONTROL_LEVEL_ORDER.length) %
      AI_CHAT_CONTROL_LEVEL_ORDER.length;
    const level = AI_CHAT_CONTROL_LEVEL_ORDER[nextIndex];
    const option = controlModeOptionRefs.get(level);
    if (!option) return false;
    option.focus();
    return true;
  };

  const focusCurrentControlModeOption = () => {
    queueMicrotask(() => {
      if (!focusControlModeOptionAtIndex(getCurrentControlLevelIndex())) {
        focusControlModeOptionAtIndex(0);
      }
    });
  };

  const focusControlModeTriggerAfterClose = () => {
    const trigger = controlModeButtonRef;
    if (!trigger) return;
    window.setTimeout(() => trigger.focus(), 0);
  };

  const closeControlMenuAndFocusTrigger = () => {
    setShowControlMenu(false);
    focusControlModeTriggerAfterClose();
  };

  const openControlMenuAndFocusSelection = () => {
    if (controlSaving()) return;
    setShowControlMenu(true);
    focusCurrentControlModeOption();
  };

  const toggleControlMenu = () => {
    if (showControlMenu()) {
      setShowControlMenu(false);
      return;
    }
    openControlMenuAndFocusSelection();
  };

  const consumeControlMenuKey = (event: KeyboardEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.stopImmediatePropagation();
  };

  const focusControlModeOptionRelativeTo = (level: AIControlLevel, offset: number) => {
    const currentIndex = AI_CHAT_CONTROL_LEVEL_ORDER.indexOf(level);
    if (currentIndex < 0) return false;
    return focusControlModeOptionAtIndex(currentIndex + offset);
  };

  const handleControlModeTriggerKeyDown = (event: KeyboardEvent) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      consumeControlMenuKey(event);
      openControlMenuAndFocusSelection();
    }
  };

  const handleControlModeOptionKeyDown = (
    event: KeyboardEvent & { currentTarget: HTMLButtonElement },
    level: AIControlLevel,
  ) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;

    if (event.key === 'ArrowDown' && focusControlModeOptionRelativeTo(level, 1)) {
      consumeControlMenuKey(event);
      return;
    }
    if (event.key === 'ArrowUp' && focusControlModeOptionRelativeTo(level, -1)) {
      consumeControlMenuKey(event);
      return;
    }
    if (event.key === 'Home' && focusControlModeOptionAtIndex(0)) {
      consumeControlMenuKey(event);
      return;
    }
    if (
      event.key === 'End' &&
      focusControlModeOptionAtIndex(AI_CHAT_CONTROL_LEVEL_ORDER.length - 1)
    ) {
      consumeControlMenuKey(event);
      return;
    }
    if (event.key === 'Escape') {
      consumeControlMenuKey(event);
      closeControlMenuAndFocusTrigger();
    }
  };

  // Chat hook
  const chat = useChat({
    model: '',
    defaultModel: () => defaultModel().trim(),
    onConversationChanged: refreshSessions,
  });

  createEffect(() => {
    const currentSessionId = chat.sessionId().trim();
    if (!currentSessionId) return;
    const session = findKnownSession(currentSessionId);
    if (!session) return;
    setRedoLastTurnAvailable(Boolean(session.can_redo));
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

  const restoreChatMentions = (mentions?: ChatMention[]) => {
    setAccumulatedMentions(
      (mentions || []).map((mention) => ({
        id: mention.id,
        label: mention.name || mention.id,
        type: mention.type,
        node: mention.node,
      })),
    );
  };

  const sendOptionsFromRestoredRequest = (
    request?: RestoredPromptDraft['request'],
  ): SendMessageOptions => {
    if (!request) return {};
    const sendOptions: SendMessageOptions = {};
    if (request.model) {
      sendOptions.model = request.model;
    }
    if (typeof request.autonomousMode === 'boolean') {
      sendOptions.autonomousMode = request.autonomousMode;
    }
    if (request.handoffContext) {
      sendOptions.handoffContext = request.handoffContext;
    }
    if (request.handoffResources?.length) {
      sendOptions.handoffResources = request.handoffResources.map((resource) => ({ ...resource }));
    }
    if (request.handoffActions?.length) {
      sendOptions.handoffActions = request.handoffActions.map((action) => ({ ...action }));
    }
    if (request.handoffMetadata) {
      sendOptions.handoffMetadata = { ...request.handoffMetadata };
    }
    return sendOptions;
  };

  const editQueuedFollowUp = (id: string) => {
    const queued = chat.takeQueuedFollowUp(id);
    if (!queued) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(queued);
    setRestoredPromptDraft(null);
    setInput(queued.prompt);
    restoreChatMentions(queued.mentions);
    setMentionActive(false);
    focusComposer();
    queueMicrotask(resizeTextarea);
  };

  const sendQueuedFollowUpNext = (id: string) => {
    void chat.sendQueuedFollowUpNow(id).finally(() => {
      focusComposer();
    });
  };

  const focusQueuedFollowUps = () => {
    const firstQueued = chat.queuedFollowUps()[0];
    const findQueuedRow = () =>
      (firstQueued ? queuedFollowUpRowRefs.get(firstQueued.id) : undefined) ||
      document.querySelector<HTMLElement>('[data-testid="assistant-queued-follow-up-row"]');
    const queuedRow = findQueuedRow();
    const targetQueuedId = firstQueued?.id ?? queuedRow?.dataset.assistantQueuedFollowUpId ?? null;

    if (!firstQueued && !queuedRow) {
      markQueuedFollowUpCommandTarget(null);
      notificationStore.info('No queued follow-ups.', 2000);
      focusComposer();
      return;
    }

    setShowSessions(false);
    setShowCommandHelp(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    markQueuedFollowUpCommandTarget(targetQueuedId);
    const focusRow = () => {
      const row = findQueuedRow();
      if (!row) return;
      row.scrollIntoView({ block: 'nearest' });
      row.focus({ preventScroll: true });
      if (document.activeElement !== row) {
        row.querySelector<HTMLElement>('button:not(:disabled)')?.focus({ preventScroll: true });
      }
    };
    queueMicrotask(focusRow);
    window.requestAnimationFrame(() => {
      focusRow();
    });
    window.setTimeout(focusRow, 0);
    window.setTimeout(focusRow, 50);
    window.setTimeout(focusRow, 150);
  };

  const handleQueuedFollowUpRowKeyDown = (
    event: KeyboardEvent & { currentTarget: HTMLDivElement },
    id: string,
  ) => {
    if (event.defaultPrevented || event.target !== event.currentTarget) return;

    if (event.key === 'Enter') {
      event.preventDefault();
      editQueuedFollowUp(id);
      return;
    }

    if (event.key === 'Delete' || event.key === 'Backspace') {
      event.preventDefault();
      chat.cancelQueuedFollowUp(id);
      focusComposer();
    }
  };

  const restoreLastTurnDraft = (draft: RestoredPromptDraft) => {
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(null);
    setRestoredPromptDraft(cloneRestoredPromptDraft(draft));
    setInput(draft.prompt);
    restoreChatMentions(draft.request?.mentions);
    setMentionActive(false);
    setSlashCommandActive(false);
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

  const knownAssistantModelProviders = createMemo(() => {
    const providers = new Set<string>();
    const addProvider = (provider: string) => {
      const normalized = provider.trim();
      if (normalized) providers.add(normalized);
    };

    for (const provider of aiRuntimeSettings()?.configured_providers || []) {
      addProvider(provider);
    }
    for (const model of aiRuntimeModels()) {
      addProvider(resolveRuntimeModelProvider(model));
    }

    addProvider(selectedChatProvider());
    return Array.from(providers);
  });

  const formatChatMessageModelRoute = (modelId: string) => {
    const normalized = modelId.trim();
    if (!normalized) return '';
    const match = aiRuntimeModels().find((candidate) => candidate.id === normalized);
    return match ? formatAIModelRouteLabel(match) : formatAIModelRouteLabel(normalized);
  };
  const queuedFollowUpRouteLabel = (queued: QueuedFollowUp) => {
    const model = queued.sendOptions?.model?.trim();
    return model ? formatChatMessageModelRoute(model) : '';
  };

  const providerForModelRoute = (modelId: string) => {
    const normalized = modelId.trim();
    if (!normalized) return '';
    const match = aiRuntimeModels().find((candidate) => candidate.id === normalized);
    return match?.provider?.trim() || getProviderFromModelId(normalized);
  };

  const hasCurrentTranscript = createMemo(() => hasAssistantTranscriptContent(chat.messages()));
  const lastAssistantAnswerText = createMemo(() => getLastAssistantAnswerText(chat.messages()));
  const hasLastAssistantAnswer = createMemo(() => !!lastAssistantAnswerText());
  const canForkCurrentSession = createMemo(
    () =>
      Boolean(chat.sessionId().trim()) &&
      hasCurrentTranscript() &&
      !chat.isLoading() &&
      !forkingSession(),
  );
  const canCompactCurrentSession = createMemo(
    () =>
      Boolean(chat.sessionId().trim()) &&
      hasCurrentTranscript() &&
      !chat.isLoading() &&
      !forkingSession() &&
      !compactingSession() &&
      !undoingLastTurn() &&
      !redoingLastTurn(),
  );
  const hasUndoableUserTurn = createMemo(() =>
    chat
      .messages()
      .some(
        (message) =>
          message.role === 'user' &&
          message.delivery !== 'queued' &&
          Boolean(message.content.trim()),
      ),
  );
  const canUndoLastTurn = createMemo(
    () =>
      Boolean(chat.sessionId().trim()) &&
      !chat.isLoading() &&
      !undoingLastTurn() &&
      !redoingLastTurn() &&
      hasUndoableUserTurn(),
  );
  const canRedoLastTurn = createMemo(
    () =>
      Boolean(chat.sessionId().trim()) &&
      !chat.isLoading() &&
      !undoingLastTurn() &&
      !redoingLastTurn() &&
      redoLastTurnAvailable(),
  );
  const assistantCommandAvailability = createMemo<AssistantSlashCommandAvailability>(() => ({
    compact: {
      disabled: !canCompactCurrentSession(),
      reason: chat.isLoading()
        ? 'Available after the active response finishes.'
        : !chat.sessionId().trim()
          ? 'Requires a saved Assistant session.'
          : !hasCurrentTranscript()
            ? 'Requires transcript content.'
            : 'Unavailable while another session action is running.',
    },
    copy: {
      disabled: !hasCurrentTranscript(),
      reason: 'Requires transcript content.',
    },
    export: {
      disabled: !hasCurrentTranscript(),
      reason: 'Requires transcript content.',
    },
    fork: {
      disabled: !canForkCurrentSession(),
      reason: chat.isLoading()
        ? 'Available after the active response finishes.'
        : !chat.sessionId().trim()
          ? 'Requires a saved Assistant session.'
          : !hasCurrentTranscript()
            ? 'Requires transcript content.'
            : 'Forking is already running.',
    },
    redo: {
      disabled: !canRedoLastTurn(),
      reason: 'No undone Assistant turn is available.',
    },
    queue: {
      disabled: chat.queuedFollowUpCount() === 0,
      reason: chat.queuedFollowUpCount() === 0 ? 'No queued follow-ups.' : undefined,
    },
    status: {
      disabled: providerReadiness().status === 'checking',
      reason: 'Route health check already running.',
    },
    undo: {
      disabled: !canUndoLastTurn(),
      reason: chat.isLoading()
        ? 'Available after the active response finishes.'
        : 'Requires a sent Assistant prompt.',
    },
  }));
  const disabledAssistantCommandReason = (command: AssistantSlashCommandAction) => {
    const state = assistantCommandAvailability()[command];
    return state?.disabled ? state.reason || 'Assistant command is unavailable right now.' : '';
  };

  createEffect(() => {
    const validQueuedIds = new Set(chat.queuedFollowUps().map((queued) => queued.id));
    for (const queuedId of queuedFollowUpRowRefs.keys()) {
      if (!validQueuedIds.has(queuedId)) {
        queuedFollowUpRowRefs.delete(queuedId);
      }
    }
    const targetQueuedId = queuedFollowUpCommandTargetId();
    if (targetQueuedId && !validQueuedIds.has(targetQueuedId)) {
      markQueuedFollowUpCommandTarget(null);
    }
  });

  const buildCurrentTranscript = (generatedAt = new Date()) => {
    const sessionId = chat.sessionId().trim();
    const knownSession = sessionId ? findKnownSession(sessionId) : undefined;
    return formatAssistantTranscript({
      messages: chat.messages(),
      session: {
        id: sessionId || undefined,
        title: knownSession?.title,
      },
      generatedAt,
      getModelRouteLabel: formatChatMessageModelRoute,
    });
  };

  const downloadTranscript = (transcript: string, generatedAt: Date) => {
    downloadAssistantTranscriptFile(
      transcript,
      buildAssistantTranscriptFilename(chat.sessionId(), generatedAt),
    );
  };

  const copyAssistantTranscript = async () => {
    const generatedAt = new Date();
    const transcript = buildCurrentTranscript(generatedAt);
    if (!transcript.trim()) {
      notificationStore.info('No Assistant transcript to copy', 2000);
      return;
    }

    const copied = await copyToClipboard(transcript);
    if (copied) {
      setTranscriptCopyFallback(null);
      notificationStore.success('Assistant transcript copied', 2000);
      return;
    }
    setTranscriptCopyFallback({ generatedAt, transcript });
    notificationStore.warning('Clipboard blocked; transcript opened for manual copy', 4000);
  };

  const copyLastAssistantAnswer = async () => {
    const answer = lastAssistantAnswerText();
    if (!answer) return;

    const copied = await copyToClipboard(answer);
    if (copied) {
      notificationStore.success(AI_CHAT_COPY_LAST_ANSWER_SUCCESS_MESSAGE, 2000);
      return;
    }

    notificationStore.error(AI_CHAT_COPY_LAST_ANSWER_ERROR_MESSAGE);
  };

  const exportAssistantTranscript = () => {
    const generatedAt = new Date();
    const transcript = buildCurrentTranscript(generatedAt);
    if (!transcript.trim()) {
      notificationStore.info('No Assistant transcript to export', 2000);
      return;
    }

    try {
      downloadTranscript(transcript, generatedAt);
      notificationStore.success('Assistant transcript exported', 2000);
    } catch (error) {
      logger.error('[AIChat] Failed to export Assistant transcript:', error);
      notificationStore.error('Failed to export Assistant transcript');
    }
  };

  const upsertSession = (session: ChatSession) => {
    setSessions((prev) => [session, ...prev.filter((candidate) => candidate.id !== session.id)]);
    setSessionSearchResults(
      (prev) => prev && [session, ...prev.filter((candidate) => candidate.id !== session.id)],
    );
  };

  const downloadFallbackTranscript = () => {
    const fallback = transcriptCopyFallback();
    if (!fallback) return;
    try {
      downloadTranscript(fallback.transcript, fallback.generatedAt);
      notificationStore.success('Assistant transcript exported', 2000);
    } catch (error) {
      logger.error('[AIChat] Failed to export Assistant transcript:', error);
      notificationStore.error('Failed to export Assistant transcript');
    }
  };

  const failedModelRouteHistory = createMemo(() => {
    const modelIds = new Set<string>();
    const providers = new Set<string>();

    for (const message of chat.messages()) {
      if (!hasProviderRouteFailureEvidence(message)) continue;
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
    const visible = providerReadinessVisible();
    const routeLabel = readiness.model?.trim() ? formatChatMessageModelRoute(readiness.model) : '';
    if (!readiness.provider || readiness.status === 'idle') {
      return null;
    }
    if (readiness.status === 'ready' && !visible) {
      return null;
    }
    if (readiness.status === 'checking' && !chat.isLoading() && !visible) {
      return null;
    }

    return getAIChatProviderReadinessPresentation({
      status:
        readiness.status === 'ready'
          ? 'ready'
          : readiness.status === 'checking'
            ? 'checking'
            : 'error',
      providerLabel: getAIProviderDisplayName(readiness.provider),
      routeLabel,
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

  const providerRouteHealth = createMemo<AssistantProviderRouteHealthPresentation | null>(() => {
    const readiness = providerReadiness();
    if (!readiness.provider || readiness.status === 'idle') return null;

    const providerLabel = getAIProviderDisplayName(readiness.provider);
    const routeLabel = readiness.model?.trim() ? formatChatMessageModelRoute(readiness.model) : '';
    const title = compactText([
      readiness.status === 'checking'
        ? `Checking ${providerLabel} provider route`
        : readiness.status === 'ready'
          ? `${providerLabel} provider route ready`
          : `${providerLabel} provider route issue`,
      routeLabel ? `Route: ${routeLabel}` : undefined,
      readiness.summary || readiness.message,
    ]).join('. ');

    if (readiness.status === 'checking') {
      return {
        label: `Checking ${providerLabel}`,
        title,
        className:
          'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200',
        dotClassName: 'bg-blue-500 animate-pulse',
      };
    }

    if (readiness.status === 'ready') {
      return {
        label: `${providerLabel} ready`,
        title,
        className:
          'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200',
        dotClassName: 'bg-emerald-500',
      };
    }

    return {
      label: `${providerLabel} issue`,
      title,
      className:
        'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100',
      dotClassName: 'bg-amber-500',
    };
  });

  const assistantSurfaceToolHealth = createMemo<AssistantSurfaceToolHealthPresentation | null>(
    () => {
      const state = assistantSurfaceTools();

      if (state.status === 'loading') {
        return {
          label: 'Checking capabilities',
          title: 'Checking Assistant capability availability',
          className:
            'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200',
          dotClassName: 'bg-blue-500 animate-pulse',
          iconClassName: 'text-blue-500 dark:text-blue-300',
        };
      }

      if (state.status === 'unavailable') {
        return {
          label: 'Capabilities unavailable',
          title:
            state.message ||
            'Pulse could not load the Assistant capability contract for this session.',
          className:
            'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100',
          dotClassName: 'bg-amber-500',
          iconClassName: 'text-amber-500 dark:text-amber-300',
        };
      }

      if (state.status !== 'ready') return null;
      const posture = getAgentSurfaceToolPosturePresentation(state.contract);
      if (!posture) return null;

      const readyClassName =
        posture.tone === 'ready'
          ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-200'
          : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100';
      const readyDotClassName = posture.tone === 'ready' ? 'bg-emerald-500' : 'bg-amber-500';
      const readyIconClassName =
        posture.tone === 'ready'
          ? 'text-emerald-500 dark:text-emerald-300'
          : 'text-amber-500 dark:text-amber-300';

      return {
        label: posture.label,
        title: posture.title,
        className: readyClassName,
        dotClassName: readyDotClassName,
        iconClassName: readyIconClassName,
      };
    },
  );

  let providerReadinessRequestId = 0;
  let lastProviderReadinessKey = '';
  let assistantSurfaceToolsRequestId = 0;

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
      logger.warn('[AIChat] Failed to check selected provider readiness:', error);
      setProviderReadiness({
        status: 'error',
        provider,
        model,
        message: 'Provider check failed',
        summary: 'Pulse could not verify the selected model route.',
        recommendation:
          'Check provider settings and network reachability, then retry the route check.',
        action: 'open_provider_settings',
      });
    }
  };

  const retrySelectedProviderReadiness = () => {
    setProviderReadinessRetryNonce((value) => value + 1);
    focusComposer();
  };

  const runSelectedProviderStatusCheck = () => {
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    setProviderReadinessVisible(true);

    const model = selectedChatModel().trim();
    const provider = selectedChatProvider().trim();

    if (!provider || !model) {
      providerReadinessRequestId += 1;
      lastProviderReadinessKey = '';
      setProviderReadiness({
        status: 'error',
        provider: provider || 'Selected',
        model,
        message: 'No provider route selected',
        summary: 'Pulse does not have a selected Assistant model route to check.',
        recommendation: 'Choose an Assistant model route, then run /status again.',
        action: 'open_provider_settings',
      });
      return;
    }

    lastProviderReadinessKey = '';
    void refreshSelectedProviderReadiness(provider, model);
  };

  const openAssistantProviderSettings = () => {
    setShowSessions(false);
    setShowCommandHelp(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    navigate(AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF);
    props.onClose();
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

  const loadWorkflowPrompts = async () => {
    try {
      const manifest = await fetchAgentCapabilitiesManifest();
      setWorkflowPrompts(getAgentWorkflowPrompts(manifest));
    } catch (error) {
      logger.debug('[AIChat] Workflow prompt manifest unavailable during initialization:', error);
      setWorkflowPrompts([]);
    }
  };

  const loadAssistantSurfaceTools = async () => {
    const requestId = ++assistantSurfaceToolsRequestId;
    setAssistantSurfaceTools({ status: 'loading' });

    try {
      const contract = await AIChatAPI.getAssistantSurfaceTools();
      if (requestId !== assistantSurfaceToolsRequestId) return;
      setAssistantSurfaceTools({ status: 'ready', contract });
    } catch (error) {
      if (requestId !== assistantSurfaceToolsRequestId) return;
      logger.debug(
        '[AIChat] Assistant surface tool contract unavailable during initialization:',
        error,
      );
      setAssistantSurfaceTools({
        status: 'unavailable',
        message: 'Assistant capabilities could not be loaded.',
      });
    }
  };

  const assistantWorkflowStarters = createMemo(() =>
    getAssistantWorkflowStarters(workflowPrompts(), aiChatStore.context),
  );

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
        `Assistant chat action mode set to ${getAIChatControlLevelPresentation(resolved).label}`,
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

  const selectModel = (modelId: string, options: { rememberRecent?: boolean } = {}) => {
    chat.setModel(modelId);
    updateStoredModel(chat.sessionId(), modelId);
    if (options.rememberRecent !== false) {
      rememberRecentModel(modelId);
    }
  };

  const openModelSelector = (initialSearch = '') => {
    setModelSelectorInitialSearch(initialSearch.trim());
    setModelSelectorOpenRequest((value) => value + 1);
  };

  const recentModelRouteByDirection = (direction: 1 | -1) =>
    getNextAssistantRecentModelRoute({
      currentModel: selectedChatModel(),
      direction,
      recentModelIds: recentModelIds(),
    });
  const nextRecentModelRoute = createMemo(() => recentModelRouteByDirection(1));
  const nextRecentModelRouteLabel = createMemo(() => {
    const next = nextRecentModelRoute();
    return next ? formatChatMessageModelRoute(next) : '';
  });

  const cycleRecentModelRoute = (direction: 1 | -1 = 1) => {
    const next = recentModelRouteByDirection(direction);
    if (!next) return;
    selectModel(next, { rememberRecent: false });
    focusComposer();
  };

  const runModelSlashCommand = (argument: string) => {
    const target = argument.trim();
    if (!target) {
      setShowSessions(false);
      setShowCommandHelp(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      openModelSelector();
      return true;
    }

    const normalizedTarget = target.toLowerCase();
    if (normalizedTarget === 'default' || normalizedTarget === 'configured') {
      selectModel('', { rememberRecent: false });
      notificationStore.success('Assistant model route set to default', 2000);
      focusComposer();
      return true;
    }

    if (normalizedTarget === 'next' || normalizedTarget === 'recent') {
      const next = recentModelRouteByDirection(1);
      if (!next) {
        notificationStore.info('No recent Assistant model route to cycle to.', 2000);
        focusComposer();
        return true;
      }
      selectModel(next, { rememberRecent: false });
      notificationStore.success(
        `Assistant model route set to ${formatChatMessageModelRoute(next)}`,
        2000,
      );
      focusComposer();
      return true;
    }

    if (normalizedTarget === 'previous' || normalizedTarget === 'prev') {
      const previous = recentModelRouteByDirection(-1);
      if (!previous) {
        notificationStore.info('No previous Assistant model route to cycle to.', 2000);
        focusComposer();
        return true;
      }
      selectModel(previous, { rememberRecent: false });
      notificationStore.success(
        `Assistant model route set to ${formatChatMessageModelRoute(previous)}`,
        2000,
      );
      focusComposer();
      return true;
    }

    const modelRoute = normalizeAssistantModelRouteArgument(target, knownAssistantModelProviders());
    if (!modelRoute) {
      openModelSelector(target);
      focusComposer();
      return true;
    }

    selectModel(modelRoute);
    notificationStore.success(
      `Assistant model route set to ${formatChatMessageModelRoute(modelRoute)}`,
      2000,
    );
    focusComposer();
    return true;
  };

  const openModelSelectorFromError = () => {
    openModelSelector();
  };

  const getFailedTurnModelRouteAlternative = (message: ChatMessage) => {
    if (!hasProviderRouteFailureEvidence(message)) return null;
    return modelRouteAlternativeFor(message.model || selectedChatModel());
  };

  const switchToModelRoute = (modelId: string, failedMessageId?: string) => {
    selectModel(modelId);
    if (failedMessageId) {
      void chat.retryMessage(failedMessageId, { model: modelId });
    }
    focusComposer();
  };

  const switchToProviderReadinessAlternative = () => {
    const alternative = providerReadinessAlternative();
    if (!alternative) return;
    switchToModelRoute(alternative.id);
  };

  const providerReadinessAlternativeButtonLabel = () => {
    const alternative = providerReadinessAlternative();
    if (!alternative) return '';
    return alternative.kind === 'same-model-route'
      ? `Switch to ${alternative.providerLabel} route`
      : `Switch to ${alternative.providerLabel} model route`;
  };

  createEffect(() => {
    const sessionId = chat.sessionId();
    const transcriptModel = latestExplicitModelRouteFromTranscript(chat.messages());
    if (transcriptModel && sessionId) {
      updateStoredModel(sessionId, transcriptModel);
      if (chat.model() !== transcriptModel) {
        chat.setModel(transcriptModel);
      }
      return;
    }

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
        setWorkflowPrompts([]);
        assistantSurfaceToolsRequestId += 1;
        setAssistantSurfaceTools({ status: 'idle' });
        return;
      }
      const [
        sessionsResult,
        settingsResult,
        modelsResult,
        workflowPromptsResult,
        assistantSurfaceToolsResult,
      ] = await Promise.allSettled([
        refreshSessions(),
        loadAIRuntimeSettings(),
        loadAIRuntimeModels(),
        loadWorkflowPrompts(),
        loadAssistantSurfaceTools(),
      ]);
      if (sessionsResult.status === 'rejected') {
        logger.error('[AIChat] Failed to load sessions:', sessionsResult.reason);
      }
      if (settingsResult.status === 'rejected') {
        logger.error('[AIChat] Failed to load AI settings:', settingsResult.reason);
      }
      if (modelsResult.status === 'rejected') {
        logger.debug(
          '[AIChat] Model catalog unavailable during initialization:',
          modelsResult.reason,
        );
      }
      if (workflowPromptsResult.status === 'rejected') {
        logger.debug(
          '[AIChat] Workflow prompt manifest unavailable during initialization:',
          workflowPromptsResult.reason,
        );
      }
      if (assistantSurfaceToolsResult.status === 'rejected') {
        logger.debug(
          '[AIChat] Assistant surface tool contract unavailable during initialization:',
          assistantSurfaceToolsResult.reason,
        );
      }
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
  const autonomousWarningVisible = createMemo(
    () =>
      controlLevel() === 'autonomous' &&
      !autonomousBannerDismissed() &&
      !hasScopedApprovalHandoff(),
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
  // The activity dock is Pulse's single live "assistant is working" footer (the
  // OpenCode model: live status lives in a pinned footer, never narrated into the
  // transcript). It must stay up for the whole turn. `chat.isLoading()` flips
  // false at visible-turn-complete (as soon as the answer is shown), which is too
  // early — so the dock would flash its status for a moment and vanish. Gate the
  // dock on the streaming assistant message instead, which stays true until the
  // turn fully settles.
  const assistantTurnActive = createMemo(() => {
    if (chat.isLoading()) return true;
    const messages = chat.messages();
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role === 'assistant') return message.isStreaming !== false;
      if (message.role === 'user') return false;
    }
    return false;
  });
  const activeAssistantMessage = createMemo(() => {
    if (!assistantTurnActive()) return undefined;
    const messages = chat.messages();
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role === 'assistant' && message.isStreaming !== false) {
        return message;
      }
    }
    return undefined;
  });
  const activeWorkflowStatusHistory = createMemo(() => {
    const message = activeAssistantMessage();
    if (!message || message.isStreaming === false) return [];
    return (
      message.workflowStatusHistory || (message.workflowStatus ? [message.workflowStatus] : [])
    );
  });
  const activeWorkflowStatusPaceSequenceKey = createMemo(() => {
    const message = activeAssistantMessage();
    const first = activeWorkflowStatusHistory()[0];
    return [message?.id, first?.phase, first?.message, first?.startedAt]
      .map((value) => String(value ?? ''))
      .join(':');
  });
  const pacedActiveWorkflowStatus = createPacedWorkflowStatus(
    activeWorkflowStatusHistory,
    () => assistantTurnActive() && activeWorkflowStatusHistory().length > 1,
    activeWorkflowStatusPaceSequenceKey,
  );
  const messagesWithPacedActiveWorkflowStatus = createMemo(() => {
    const activeMessage = activeAssistantMessage();
    const workflowStatus = pacedActiveWorkflowStatus();
    const messages = chat.messages();
    if (!activeMessage || !workflowStatus) return messages;

    return messages.map((message) => {
      if (message.id !== activeMessage.id) return message;
      return {
        ...message,
        workflowStatus,
        streamEvents: replaceLatestWorkflowStatusEventForDisplay(
          message.streamEvents,
          workflowStatus,
        ),
      };
    });
  });
  const [currentStatusNow, setCurrentStatusNow] = createSignal(Date.now());
  const currentStatus = createMemo(() => {
    return getAssistantActiveTurnStatus(
      messagesWithPacedActiveWorkflowStatus(),
      assistantTurnActive(),
      currentStatusNow(),
    );
  });
  const activityDockQueuedFollowUpCount = createMemo(() =>
    Math.max(currentStatus()?.queuedFollowUpCount || 0, chat.queuedFollowUpCount()),
  );
  createEffect(() => {
    const status = currentStatus();
    if (!status?.startedAt) return;
    setCurrentStatusNow(Date.now());
    const interval = window.setInterval(
      () => setCurrentStatusNow(Date.now()),
      WORKFLOW_STATUS_REFRESH_MS,
    );
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
  const currentStatusKind = createMemo(() => currentStatus()?.type);
  const currentStatusRowClass = createMemo(() => {
    if (currentStatusKind() === 'retrying') {
      return 'bg-amber-50/80 text-amber-900 dark:bg-amber-950/25 dark:text-amber-100';
    }
    return '';
  });
  const currentStatusIconClass = createMemo(() => {
    if (currentStatusKind() === 'retrying') {
      return 'animate-spin text-amber-600 dark:text-amber-300';
    }
    if (currentStatusKind() === 'generating') {
      return 'text-emerald-500 dark:text-emerald-300';
    }
    return 'animate-spin text-blue-600 dark:text-blue-300';
  });
  const currentStatusDotClass = createMemo(() =>
    currentStatusKind() === 'retrying' ? 'bg-amber-400' : 'bg-blue-400',
  );
  const activeTurnRoute = createMemo<AssistantActiveRoutePresentation | null>(() => {
    if (!currentStatus()) return null;
    const model = activeAssistantMessage()?.model?.trim() || selectedChatModel().trim();
    if (!model) return null;
    const label = formatChatMessageModelRoute(model);
    if (!label) return null;
    const provider = providerForModelRoute(model);
    const providerLabel = provider ? getAIProviderDisplayName(provider) : '';
    return {
      label,
      title: compactText([
        'Active Assistant model route',
        label,
        providerLabel ? `Provider: ${providerLabel}` : undefined,
      ]).join('. '),
    };
  });
  const lastAssistantTurnSummary = createMemo(() => {
    const messages = chat.messages();
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const summary = getAssistantTurnSummary(messages[index], {
        getModelRouteLabel: formatChatMessageModelRoute,
      });
      if (summary) return summary;
    }
    return null;
  });

  createEffect(() => {
    if (!transcriptCopyFallback()) return;
    queueMicrotask(() => {
      transcriptFallbackTextareaRef?.focus();
      transcriptFallbackTextareaRef?.select();
    });
  });

  createEffect(() => {
    input();
    queueMicrotask(resizeTextarea);
  });

  createEffect(() => {
    if (!isOpen()) {
      setShowCommandHelp(false);
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
      setProviderReadinessVisible(false);
      setProviderReadiness({ status: 'idle', provider: '' });
      return;
    }

    if (!provider || !model) {
      providerReadinessRequestId += 1;
      lastProviderReadinessKey = '';
      setProviderReadinessVisible(false);
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

  // Click outside handler to close all dropdowns
  onMount(() => {
    setPromptHistory(loadPromptHistory());
    aiChatStore.registerInput?.(textareaRef ?? null);
    restoreStashedComposerDraft();
    focusComposer();

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      const path = typeof e.composedPath === 'function' ? e.composedPath() : [];
      const isInsideDropdown = path.some(
        (node) => node instanceof Element && Boolean(node.closest('[data-dropdown]')),
      );
      const isInsideComposerPopup = path.some(
        (node) =>
          node instanceof Element &&
          (Boolean(node.closest('[data-mention-autocomplete]')) ||
            Boolean(node.closest('[data-slash-command-autocomplete]'))),
      );
      const isInsideComposer = Boolean(target.closest('[data-assistant-composer]'));
      // Only close if click is outside dropdown containers
      if (!isInsideDropdown && !isInsideComposerPopup) {
        setShowSessions(false);
        setSessionRefreshLoading(false);
        resetSessionSearch();
        setShowControlMenu(false);
      }
      // Close mention autocomplete when clicking outside
      if (!target.closest('[data-mention-autocomplete]') && !isInsideComposer) {
        setMentionActive(false);
      }
      if (!target.closest('[data-slash-command-autocomplete]') && !isInsideComposer) {
        closeSlashCommandAutocomplete({ clearTransientDraft: true });
      }
    };
    document.addEventListener('click', handleClickOutside);
    onCleanup(() => {
      stashComposerDraftForRemount();
      document.removeEventListener('click', handleClickOutside);
      aiChatStore.registerInput?.(null);
      clearInterruptArm();
      if (queuedFollowUpCommandTargetTimeout) {
        clearTimeout(queuedFollowUpCommandTargetTimeout);
      }
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

  const restoreFailedSubmitDraft = (
    draft: string,
    mentions: MentionResource[],
    blocks: PastedTextBlock[] = [],
  ) => {
    if (input().trim() || accumulatedMentions().length > 0 || pastedBlocks().length > 0) {
      return;
    }
    setInput(draft);
    setAccumulatedMentions(cloneMentions(mentions));
    setPastedBlocks(blocks);
    setMentionActive(false);
    setSlashCommandActive(false);
    queueMicrotask(() => {
      resizeTextarea();
      focusComposer();
    });
  };

  // Long pastes collapse into chips instead of flooding the textarea; the
  // full text rejoins the prompt at send time (composePromptWithPastedBlocks).
  const handleComposerPaste = (e: ClipboardEvent) => {
    const text = e.clipboardData?.getData('text/plain') ?? '';
    if (!shouldCollapsePastedText(text)) return;
    e.preventDefault();
    setPastedBlocks((prev) => [...prev, createPastedTextBlock(text)]);
  };

  const removePastedBlock = (id: string) => {
    setPastedBlocks((prev) => prev.filter((block) => block.id !== id));
    focusComposer();
  };

  // Clicking the chip body un-collapses: the text returns to the textarea so
  // the user can trim or edit it inline.
  const expandPastedBlock = (id: string) => {
    const block = pastedBlocks().find((candidate) => candidate.id === id);
    if (!block) return;
    setPastedBlocks((prev) => prev.filter((candidate) => candidate.id !== id));
    const current = input();
    setInput(current ? `${current}\n${block.text}` : block.text);
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

  const clearLocalComposerCommand = (options: { refocusComposer?: boolean } = {}) => {
    const shouldRefocusComposer = options.refocusComposer !== false;
    stashedComposerDraft = null;
    setEditingQueuedFollowUp(null);
    setRestoredPromptDraft(null);
    setInput('');
    setAccumulatedMentions([]);
    setPastedBlocks([]);
    setMentionActive(false);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    resetPromptHistoryNavigation();
    queueMicrotask(() => {
      resizeTextarea();
      if (shouldRefocusComposer) {
        focusComposer();
      }
    });
  };

  const openAssistantCommandHelp = () => {
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    setMentionActive(false);
    closeSlashCommandAutocomplete({ clearTransientDraft: true });
    setShowControlMenu(false);
    setShowCommandHelp(true);
  };

  const insertSlashCommandDraft = (command: AssistantSlashCommand) => {
    const nextInput = command.insertText || `/${command.name}`;
    setShowCommandHelp(false);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    setInput(nextInput);
    queueMicrotask(() => {
      if (textareaRef) {
        textareaRef.selectionStart = nextInput.length;
        textareaRef.selectionEnd = nextInput.length;
      }
      resizeTextarea();
      focusComposer();
    });
  };

  const runAssistantFixtureSlashCommand = (args: string) => {
    const fixturePrompt = args.trim() ? `/fixture ${args.trim()}` : '/fixture';
    const submittedMentions: MentionResource[] = [];
    const sendPromise = chat.sendMessage(fixturePrompt, undefined, undefined);
    addPromptHistoryEntry(fixturePrompt, submittedMentions);
    resetPromptHistoryNavigation();
    sendPromise
      .then((ok) => {
        if (!ok) {
          restoreFailedSubmitDraft(fixturePrompt, submittedMentions);
        }
      })
      .catch((error) => {
        logger.warn('[AIChat] Failed to run Assistant fixture command:', error);
        restoreFailedSubmitDraft(fixturePrompt, submittedMentions);
      });
    clearLocalComposerCommand();
    setShowCommandHelp(false);
    return true;
  };

  const runSessionsSlashCommand = (args: string) => {
    setShowCommandHelp(false);
    if (args.trim()) {
      void openSessionPicker(args);
      return;
    }
    void handleToggleSessions();
  };

  const executeSlashCommand = (command: AssistantSlashCommandAction, args = '') => {
    const commandArgs = args.trim();
    const disabledReason = disabledAssistantCommandReason(command);
    if (disabledReason) {
      clearLocalComposerCommand();
      setShowCommandHelp(false);
      notificationStore.info(disabledReason, 2000);
      focusComposer();
      return true;
    }

    if (command === 'models' && commandArgs) {
      const consumed = runModelSlashCommand(commandArgs);
      if (consumed) {
        clearLocalComposerCommand();
      }
      return consumed;
    }

    if (command === 'fixture') {
      return runAssistantFixtureSlashCommand(commandArgs);
    }

    clearLocalComposerCommand({ refocusComposer: command !== 'queue' });

    switch (command) {
      case 'new':
        setShowCommandHelp(false);
        void handleNewConversation();
        break;
      case 'sessions':
        runSessionsSlashCommand(commandArgs);
        break;
      case 'queue':
        focusQueuedFollowUps();
        break;
      case 'compact':
        setShowCommandHelp(false);
        void handleCompactSession();
        break;
      case 'help':
        openAssistantCommandHelp();
        break;
      case 'models':
        setShowSessions(false);
        setShowCommandHelp(false);
        setSessionRefreshLoading(false);
        resetSessionSearch();
        openModelSelector();
        break;
      case 'providers':
        openAssistantProviderSettings();
        break;
      case 'status':
        setShowCommandHelp(false);
        runSelectedProviderStatusCheck();
        break;
      case 'copy':
        setShowCommandHelp(false);
        void copyAssistantTranscript();
        break;
      case 'export':
        setShowCommandHelp(false);
        exportAssistantTranscript();
        break;
      case 'fork':
        setShowCommandHelp(false);
        void handleForkSession();
        break;
      case 'undo':
        setShowCommandHelp(false);
        void handleUndoLastTurn();
        break;
      case 'redo':
        setShowCommandHelp(false);
        void handleRedoLastTurn();
        break;
    }
    return true;
  };

  createEffect(() => {
    const request = aiChatStore.commandRequestSignal?.();
    if (!request || request.id === handledAssistantCommandRequestId || !isOpen()) return;

    handledAssistantCommandRequestId = request.id;
    executeSlashCommand(request.action);
    aiChatStore.ackCommandRequest?.(request.id);
  });

  // Handle submit
  const handleSubmit = () => {
    if (composerSubmitDispatchLocked) return;

    const submittedInput = readComposerInputForSubmit();
    const typedPrompt = submittedInput.trim();
    const submittedBlocks = pastedBlocks();
    if (!typedPrompt && submittedBlocks.length === 0) return;
    composerSubmitDispatchLocked = true;
    queueMicrotask(() => {
      composerSubmitDispatchLocked = false;
    });
    if (submittedInput !== input()) {
      setInput(submittedInput);
    }
    const slashCommand = typedPrompt ? parseAssistantSlashCommandInput(typedPrompt) : null;
    if (slashCommand) {
      executeSlashCommand(slashCommand.action, slashCommand.args);
      return;
    }
    const prompt = composePromptWithPastedBlocks(typedPrompt, submittedBlocks);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
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
    const restoredDraft = restoredPromptDraft();
    const findingId = queuedDraft
      ? queuedDraft.findingId
      : (restoredDraft?.request?.findingId ?? ctx.findingId);
    const sendOptions: SendMessageOptions = queuedDraft?.sendOptions
      ? { ...queuedDraft.sendOptions }
      : restoredDraft?.request
        ? sendOptionsFromRestoredRequest(restoredDraft.request)
        : {};
    if (!queuedDraft && !restoredDraft) {
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
      Boolean(ctx.handoffMetadata) ||
      Boolean(ctx.preferredWorkflowPromptName?.trim());
    sendPromise
      .then((ok) => {
        if (!ok) {
          restoreFailedSubmitDraft(submittedInput, submittedMentions, submittedBlocks);
          return;
        }
        stashedComposerDraft = null;
        setEditingQueuedFollowUp(null);
        setRestoredPromptDraft(null);
        setRedoLastTurnAvailable(false);
        if (!queuedDraft && !restoredDraft && findingId) {
          aiChatStore.clearFindingId?.();
        }
        if (!queuedDraft && !restoredDraft && hasRequestHandoffPayload) {
          aiChatStore.clearRequestHandoffPayload?.();
        }
      })
      .catch((error) => {
        logger.warn('[AIChat] Failed to send Assistant message:', error);
        restoreFailedSubmitDraft(submittedInput, submittedMentions, submittedBlocks);
      });
    stashedComposerDraft = null;
    setInput('');
    setAccumulatedMentions([]);
    setPastedBlocks([]);
    setMentionActive(false);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    focusComposer();
  };

  const renderWorkflowStarterIntoComposer = async (
    starter: AssistantWorkflowStarter,
    options: { onlyWhenComposerEmpty?: boolean } = {},
  ) => {
    if (renderingWorkflowStarterId()) return;
    if (options.onlyWhenComposerEmpty && input().trim()) return;

    setRenderingWorkflowStarterId(starter.id);
    try {
      const rendered = await AIChatAPI.renderWorkflowPrompt({
        name: starter.name,
        arguments: starter.arguments,
      });
      const starterText = rendered.text.trim();
      if (!starterText) {
        notificationStore.warning('Workflow starter returned an empty prompt.', 3000);
        return;
      }

      setInput((current) => {
        const draft = current.trim();
        if (options.onlyWhenComposerEmpty && draft) return current;
        return draft ? `${draft}\n\n${starterText}` : starterText;
      });
      setSlashCommandActive(false);
      setSlashCommandQuery('');
      setMentionActive(false);
      resetPromptHistoryNavigation();
      focusComposer();
    } catch (error) {
      logger.error('[AIChat] Failed to render workflow starter:', error);
      const message =
        error instanceof Error ? error.message : 'Failed to prepare workflow starter.';
      notificationStore.error(message);
    } finally {
      setRenderingWorkflowStarterId('');
    }
  };

  const handleWorkflowStarterSelect = async (starter: AssistantWorkflowStarter) => {
    await renderWorkflowStarterIntoComposer(starter);
  };

  let autoRenderedWorkflowStarterKey = '';
  createEffect(() => {
    const context = aiChatStore.context;
    const preferredWorkflowPromptName = (context.preferredWorkflowPromptName ?? '').trim();
    if (!preferredWorkflowPromptName) {
      autoRenderedWorkflowStarterKey = '';
      return;
    }

    const starter = assistantWorkflowStarters().find(
      (candidate) => candidate.name === preferredWorkflowPromptName,
    );
    if (!starter || renderingWorkflowStarterId()) return;

    const starterKey = [
      starter.name,
      context.findingId ?? '',
      context.targetType ?? '',
      context.targetId ?? '',
      JSON.stringify(starter.arguments),
    ].join('|');
    if (autoRenderedWorkflowStarterKey === starterKey) return;

    autoRenderedWorkflowStarterKey = starterKey;
    if (input().trim()) return;

    void renderWorkflowStarterIntoComposer(starter, { onlyWhenComposerEmpty: true });
  });

  const updateSlashCommandAutocomplete = (value: string, cursorPos: number) => {
    const textBeforeCursor = value.slice(0, cursorPos);
    const textAfterCursor = value.slice(cursorPos);

    if (
      textBeforeCursor.startsWith('/') &&
      !/\s/.test(textBeforeCursor) &&
      !textAfterCursor.trim()
    ) {
      const query = textBeforeCursor.slice(1);
      setSlashCommandQuery(query);
      setSlashCommandActive(true);
      setMentionActive(false);
      return true;
    }

    setSlashCommandActive(false);
    setSlashCommandQuery('');
    return false;
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

    if (updateSlashCommandAutocomplete(value, cursorPos)) {
      return;
    }

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

  const handleSlashCommandSelect = (command: AssistantSlashCommand) => {
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    if (command.insertText) {
      insertSlashCommandDraft(command);
      return;
    }
    setInput(`/${command.name}`);
    executeSlashCommand(command.action);
  };

  const handleCommandHelpRun = (command: AssistantSlashCommand) => {
    setShowCommandHelp(false);
    if (command.insertText) {
      queueMicrotask(() => insertSlashCommandDraft(command));
      return;
    }
    queueMicrotask(() => executeSlashCommand(command.action));
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
    setSlashCommandActive(false);
    setSlashCommandQuery('');

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
    if (slashCommandActive()) {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        closeSlashCommandAutocomplete({ clearTransientDraft: true });
        return;
      }
      if (['ArrowDown', 'ArrowUp', 'Enter', 'Tab'].includes(e.key)) {
        // These are handled by SlashCommandAutocomplete.
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

  const handleUndoLastTurn = async () => {
    if (undoingLastTurn() || redoingLastTurn()) return;
    if (chat.isLoading()) {
      notificationStore.info(AI_CHAT_UNDO_LAST_TURN_LOADING_MESSAGE, 2000);
      return;
    }
    if (!canUndoLastTurn()) {
      notificationStore.info(AI_CHAT_UNDO_LAST_TURN_EMPTY_MESSAGE, 2000);
      return;
    }

    setUndoingLastTurn(true);
    try {
      const draft = await chat.undoLastTurn();
      if (!draft) return;
      setRedoLastTurnAvailable(true);
      restoreLastTurnDraft(draft);
      notificationStore.success(AI_CHAT_UNDO_LAST_TURN_SUCCESS_MESSAGE, 2000);
    } catch (error) {
      logger.error('[AIChat] Failed to undo Assistant turn:', error);
      notificationStore.error(AI_CHAT_UNDO_LAST_TURN_ERROR_MESSAGE);
    } finally {
      setUndoingLastTurn(false);
    }
  };

  const handleRedoLastTurn = async () => {
    if (undoingLastTurn() || redoingLastTurn()) return;
    if (chat.isLoading()) {
      notificationStore.info(AI_CHAT_REDO_LAST_TURN_LOADING_MESSAGE, 2000);
      return;
    }
    if (!chat.sessionId().trim() || !redoLastTurnAvailable()) {
      notificationStore.info(AI_CHAT_REDO_LAST_TURN_EMPTY_MESSAGE, 2000);
      return;
    }

    setRedoingLastTurn(true);
    try {
      const result = await chat.redoLastTurn();
      setRedoLastTurnAvailable(result.canRedo);
      if (!result.success) return;
      setRestoredPromptDraft(null);
      setEditingQueuedFollowUp(null);
      setInput('');
      setAccumulatedMentions([]);
      setPastedBlocks([]);
      setMentionActive(false);
      setSlashCommandActive(false);
      notificationStore.success(AI_CHAT_REDO_LAST_TURN_SUCCESS_MESSAGE, 2000);
      focusComposer();
      queueMicrotask(resizeTextarea);
    } catch (error) {
      logger.error('[AIChat] Failed to redo Assistant turn:', error);
      notificationStore.error(AI_CHAT_REDO_LAST_TURN_ERROR_MESSAGE);
    } finally {
      setRedoingLastTurn(false);
    }
  };

  // New conversation
  const handleNewConversation = async () => {
    const started = await chat.newSession();
    if (!started) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(null);
    setRestoredPromptDraft(null);
    setRedoLastTurnAvailable(false);
    aiChatStore.clearContext?.();
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    focusComposer();
  };

  const handleToggleAttentionNotifications = async () => {
    if (assistantNotificationsEnabled()) {
      await setAssistantNotificationsEnabled(false);
      return;
    }
    const granted = await setAssistantNotificationsEnabled(true);
    if (!granted) {
      notificationStore.info(
        'Notifications are blocked for this site. Allow them in your browser to be alerted when the Assistant needs you.',
        4000,
      );
    }
  };

  const handleForkSession = async () => {
    if (forkingSession()) return;
    const sourceSessionId = chat.sessionId().trim();
    if (!sourceSessionId || !hasCurrentTranscript()) {
      notificationStore.info(AI_CHAT_FORK_SESSION_EMPTY_MESSAGE, 2000);
      return;
    }
    if (chat.isLoading()) {
      notificationStore.info(AI_CHAT_FORK_SESSION_LOADING_MESSAGE, 2000);
      return;
    }

    setForkingSession(true);
    try {
      const sourceModel = getStoredModel(sourceSessionId) || chat.model().trim();
      const forkedSession = await AIChatAPI.forkSession(sourceSessionId);
      upsertSession(forkedSession);
      if (sourceModel) {
        updateStoredModel(forkedSession.id, sourceModel);
      }

      const loaded = await chat.loadSession(forkedSession.id);
      if (!loaded) {
        notificationStore.error(AI_CHAT_FORK_SESSION_LOAD_ERROR_MESSAGE);
        return;
      }

      resetPromptHistoryNavigation();
      setEditingQueuedFollowUp(null);
      setRestoredPromptDraft(null);
      setRedoLastTurnAvailable(false);
      const restoredContext = buildSessionHandoffContext(forkedSession);
      if (restoredContext) {
        aiChatStore.setContext(restoredContext);
      } else {
        aiChatStore.clearContext?.();
      }
      setShowSessions(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      notificationStore.success(AI_CHAT_FORK_SESSION_SUCCESS_MESSAGE, 2000);
      focusComposer();
    } catch (error) {
      logger.error('[AIChat] Failed to fork Assistant session:', error);
      notificationStore.error(AI_CHAT_FORK_SESSION_ERROR_MESSAGE);
    } finally {
      setForkingSession(false);
    }
  };

  const handleCompactSession = async () => {
    if (compactingSession()) return;
    const sessionId = chat.sessionId().trim();
    if (!sessionId || !hasCurrentTranscript()) {
      notificationStore.info(AI_CHAT_COMPACT_SESSION_EMPTY_MESSAGE, 2000);
      return;
    }
    if (chat.isLoading()) {
      notificationStore.info(AI_CHAT_COMPACT_SESSION_LOADING_MESSAGE, 2500);
      return;
    }

    setCompactingSession(true);
    notificationStore.info(AI_CHAT_COMPACT_SESSION_START_MESSAGE, 2000);
    try {
      const result = await AIChatAPI.summarizeSession(sessionId);
      if (!result.success) {
        notificationStore.info(result.message || AI_CHAT_COMPACT_SESSION_EMPTY_MESSAGE, 2500);
        return;
      }

      const loaded = await chat.loadSession(sessionId);
      if (!loaded) {
        notificationStore.error(AI_CHAT_COMPACT_SESSION_LOAD_ERROR_MESSAGE);
        return;
      }
      await refreshSessions();
      setRedoLastTurnAvailable(false);
      setShowSessions(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      notificationStore.success(result.message || AI_CHAT_COMPACT_SESSION_SUCCESS_MESSAGE, 2500);
      focusComposer();
    } catch (error) {
      logger.error('[AIChat] Failed to compact Assistant session:', error);
      const message =
        error instanceof Error ? error.message : AI_CHAT_COMPACT_SESSION_ERROR_MESSAGE;
      notificationStore.error(message);
    } finally {
      setCompactingSession(false);
    }
  };

  const openSessionPicker = async (initialSearchQuery = '') => {
    if (sessionButtonRef) {
      const rect = sessionButtonRef.getBoundingClientRect();
      setSessionDropdownPosition({
        top: rect.bottom + 4,
        right: window.innerWidth - rect.right,
      });
    }

    const searchQuery = initialSearchQuery.trim();
    resetSessionSearch();
    if (searchQuery) {
      setSessionSearchQuery(searchQuery);
    }
    setShowSessions(true);
    focusSessionSearch();

    if (searchQuery) {
      setSessionRefreshLoading(false);
      return;
    }

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

  const handleToggleSessions = async () => {
    const next = !showSessions();
    if (!next) {
      setShowSessions(false);
      setSessionRefreshLoading(false);
      resetSessionSearch();
      return;
    }

    await openSessionPicker();
  };

  const formatSessionPickerMessageCount = (count: number) =>
    `${count} ${count === 1 ? 'message' : 'messages'}`;
  const formatSessionPickerUpdatedAt = (session: ChatSession) => {
    const timestamp = getSessionUpdatedTimestamp(session);
    if (!timestamp) return '';
    const updatedAt = new Date(timestamp);
    const today = new Date();
    if (updatedAt.toDateString() === today.toDateString()) {
      return updatedAt.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' });
    }
    return updatedAt.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  };
  const isSessionCurrent = (sessionId: string) =>
    Boolean(sessionId && chat.sessionId() === sessionId);
  const isSessionWorking = (sessionId: string) => isSessionCurrent(sessionId) && chat.isLoading();
  const getSessionPickerOptionLabel = (session: ChatSession) =>
    [
      `Resume ${session.title || 'Untitled'}`,
      formatSessionPickerMessageCount(session.message_count),
      formatSessionPickerUpdatedAt(session)
        ? `Updated ${formatSessionPickerUpdatedAt(session)}`
        : '',
      isSessionCurrent(session.id) ? 'Current' : '',
      isSessionWorking(session.id) ? 'Working' : '',
    ]
      .filter(Boolean)
      .join(', ');
  const getSessionPinLabel = (session: ChatSession) =>
    `${isSessionPinned(session.id) ? 'Unpin' : 'Pin'} Assistant session: ${session.title || 'Untitled'}`;
  const getSessionRenameLabel = (session: ChatSession) =>
    `${AI_CHAT_RENAME_SESSION_LABEL}: ${session.title || 'Untitled'}`;
  const getSessionDeleteLabel = (session: ChatSession) =>
    `Delete Assistant session: ${session.title || 'Untitled'}`;
  const isSessionRenaming = (sessionId: string) => renamingSessionId() === sessionId;
  const normalizeSessionRenameTitle = (title: string) => {
    const normalized = title.trim().replace(/\s+/g, ' ');
    const chars = Array.from(normalized);
    return chars.length > AI_CHAT_SESSION_TITLE_MAX_LENGTH
      ? chars.slice(0, AI_CHAT_SESSION_TITLE_MAX_LENGTH).join('')
      : normalized;
  };

  const startRenamingSession = (session: ChatSession, event: Event) => {
    event.stopPropagation();
    if (sessionRenameSaving()) return;
    setRenamingSessionId(session.id);
    setSessionRenameDraft(session.title || '');
    focusSessionRenameInput();
  };

  const cancelRenamingSession = (event?: Event) => {
    event?.preventDefault();
    event?.stopPropagation();
    const sessionId = renamingSessionId();
    resetSessionRename();
    queueMicrotask(() => sessionOptionRefs.get(sessionId)?.focus());
  };

  const handleSessionRenameKeyDown = (event: KeyboardEvent) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      cancelRenamingSession(event);
    }
  };

  const submitSessionRename = async (session: ChatSession, event: Event) => {
    event.preventDefault();
    event.stopPropagation();
    if (sessionRenameSaving()) return;

    const title = normalizeSessionRenameTitle(sessionRenameDraft());
    if (!title) {
      notificationStore.error(AI_CHAT_RENAME_SESSION_EMPTY_MESSAGE);
      focusSessionRenameInput();
      return;
    }
    if (title === normalizeSessionRenameTitle(session.title || '')) {
      cancelRenamingSession();
      return;
    }

    setSessionRenameSaving(true);
    try {
      const updatedSession = await AIChatAPI.renameSession(session.id, title);
      upsertSession(updatedSession);
      resetSessionRename();
      queueMicrotask(() => sessionOptionRefs.get(updatedSession.id)?.focus());
    } catch (error) {
      logger.error('[AIChat] Failed to rename session:', error);
      notificationStore.error(AI_CHAT_RENAME_SESSION_ERROR_MESSAGE);
      setSessionRenameSaving(false);
      focusSessionRenameInput();
    }
  };

  // Load session
  const handleLoadSession = async (sessionId: string) => {
    const session = findKnownSession(sessionId);
    const loaded = await chat.loadSession(sessionId);
    if (!loaded) return;
    resetPromptHistoryNavigation();
    setEditingQueuedFollowUp(null);
    setRestoredPromptDraft(null);
    setRedoLastTurnAvailable(Boolean(session?.can_redo));
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
      removePinnedSession(sessionId);
      if (chat.sessionId() === sessionId) {
        chat.clearMessages();
        resetPromptHistoryNavigation();
        setEditingQueuedFollowUp(null);
        setRestoredPromptDraft(null);
        setRedoLastTurnAvailable(false);
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

  const workflowStarterIcon = (starter: AssistantWorkflowStarter) => {
    switch (starter.kind) {
      case 'resource':
        return <CpuIcon class="h-3.5 w-3.5" aria-hidden="true" />;
      case 'finding':
        return <ShieldAlertIcon class="h-3.5 w-3.5" aria-hidden="true" />;
      case 'fleet':
      case 'workflow':
      default:
        return <SparklesIcon class="h-3.5 w-3.5" aria-hidden="true" />;
    }
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
          <Show when={showCommandHelp()}>
            <AssistantCommandHelpDialog
              availability={assistantCommandAvailability()}
              onClose={() => setShowCommandHelp(false)}
              onRunCommand={handleCommandHelpRun}
            />
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
                <PlusIcon class="h-3.5 w-3.5" aria-hidden="true" />
                <span class="font-medium">{AI_CHAT_NEW_SESSION_SHORT_LABEL}</span>
              </button>

              <Show when={assistantNotificationsSupported()}>
                <ActionIconButton
                  onClick={() => {
                    void handleToggleAttentionNotifications();
                  }}
                  tone={assistantNotificationsEnabled() ? 'outlineSelected' : 'outline'}
                  size="md"
                  label="Toggle Assistant attention notifications"
                  title={
                    assistantNotificationsEnabled()
                      ? 'Notifications on: you are alerted when the Assistant finishes or needs you while this tab is in the background'
                      : 'Notify me when the Assistant finishes or needs me while this tab is in the background'
                  }
                  aria-pressed={assistantNotificationsEnabled()}
                >
                  <Show
                    when={assistantNotificationsEnabled()}
                    fallback={<BellOffIcon class="h-4 w-4" aria-hidden="true" />}
                  >
                    <BellIcon class="h-4 w-4" aria-hidden="true" />
                  </Show>
                </ActionIconButton>
              </Show>

              <ActionIconButton
                onClick={() => {
                  void handleForkSession();
                }}
                disabled={!canForkCurrentSession()}
                tone="outline"
                size="md"
                title={AI_CHAT_FORK_SESSION_LABEL}
                label={AI_CHAT_FORK_SESSION_LABEL}
                aria-busy={forkingSession()}
              >
                <Show
                  when={forkingSession()}
                  fallback={<GitForkIcon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </ActionIconButton>

              <ActionIconButton
                onClick={() => {
                  void handleCompactSession();
                }}
                disabled={!canCompactCurrentSession()}
                tone="outline"
                size="md"
                title={AI_CHAT_COMPACT_SESSION_LABEL}
                label={AI_CHAT_COMPACT_SESSION_LABEL}
                aria-busy={compactingSession()}
              >
                <Show
                  when={compactingSession()}
                  fallback={<Minimize2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </ActionIconButton>

              <ActionIconButton
                onClick={() => {
                  void handleUndoLastTurn();
                }}
                disabled={!canUndoLastTurn()}
                tone="outline"
                size="md"
                title={AI_CHAT_UNDO_LAST_TURN_LABEL}
                label={AI_CHAT_UNDO_LAST_TURN_LABEL}
                aria-busy={undoingLastTurn()}
              >
                <Show
                  when={undoingLastTurn()}
                  fallback={<Undo2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </ActionIconButton>

              <ActionIconButton
                onClick={() => {
                  void handleRedoLastTurn();
                }}
                disabled={!canRedoLastTurn()}
                tone="outline"
                size="md"
                title={AI_CHAT_REDO_LAST_TURN_LABEL}
                label={AI_CHAT_REDO_LAST_TURN_LABEL}
                aria-busy={redoingLastTurn()}
              >
                <Show
                  when={redoingLastTurn()}
                  fallback={<Redo2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </ActionIconButton>

              <ActionIconButton
                onClick={() => {
                  void copyLastAssistantAnswer();
                }}
                disabled={!hasLastAssistantAnswer()}
                tone="outline"
                size="md"
                title={AI_CHAT_COPY_LAST_ANSWER_LABEL}
                label={AI_CHAT_COPY_LAST_ANSWER_LABEL}
              >
                <ClipboardCopyIcon class="h-4 w-4" aria-hidden="true" />
              </ActionIconButton>

              <ActionIconButton
                onClick={() => {
                  void copyAssistantTranscript();
                }}
                disabled={!hasCurrentTranscript()}
                tone="outline"
                size="md"
                title={AI_CHAT_COPY_TRANSCRIPT_LABEL}
                label={AI_CHAT_COPY_TRANSCRIPT_LABEL}
              >
                <CopyIcon class="h-4 w-4" aria-hidden="true" />
              </ActionIconButton>

              <ActionIconButton
                onClick={exportAssistantTranscript}
                disabled={!hasCurrentTranscript()}
                tone="outline"
                size="md"
                title={AI_CHAT_EXPORT_TRANSCRIPT_LABEL}
                label={AI_CHAT_EXPORT_TRANSCRIPT_LABEL}
              >
                <DownloadIcon class="h-4 w-4" aria-hidden="true" />
              </ActionIconButton>

              {/* Session picker */}
              <div class="relative" data-dropdown>
                <ActionIconButton
                  ref={sessionButtonRef}
                  onClick={() => {
                    void handleToggleSessions();
                  }}
                  tone="muted"
                  size="md"
                  title={AI_CHAT_SESSION_MENU_TITLE}
                  label={AI_CHAT_SESSION_MENU_TITLE}
                  aria-haspopup="dialog"
                  aria-expanded={showSessions()}
                >
                  <ClockIcon class="h-4 w-4" aria-hidden="true" />
                </ActionIconButton>

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
                      <PlusIcon class="h-4 w-4" aria-hidden="true" />
                      <span class="font-medium">{AI_CHAT_NEW_SESSION_MENU_LABEL}</span>
                    </button>

                    <div class="border-b border-border px-3 py-2">
                      <SearchField
                        value={sessionSearchQuery()}
                        onChange={setSessionSearchQuery}
                        onKeyDown={handleSessionSearchKeyDown}
                        clearOnFocusedEscape={false}
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
                        <For each={sessionPickerSections()}>
                          {(section) => (
                            <>
                              <div class="sticky top-0 z-10 border-b border-border/60 bg-surface-alt px-3 py-1.5 text-[11px] font-semibold text-muted">
                                {section.title}
                              </div>
                              <For each={section.sessions}>
                                {(session) => (
                                  <div
                                    class={`group relative flex items-start gap-2 px-3 py-2.5 hover:bg-surface-hover focus-within:bg-surface-hover ${
                                      isSessionCurrent(session.id)
                                        ? 'bg-blue-50 dark:bg-blue-900'
                                        : ''
                                    }`}
                                  >
                                    <Show
                                      when={isSessionRenaming(session.id)}
                                      fallback={
                                        <button
                                          type="button"
                                          ref={(button) => {
                                            sessionOptionRefs.set(session.id, button);
                                          }}
                                          role="option"
                                          aria-selected={isSessionCurrent(session.id)}
                                          aria-label={getSessionPickerOptionLabel(session)}
                                          class="min-w-0 flex-1 text-left focus:outline-none"
                                          onClick={() => handleLoadSession(session.id)}
                                          onKeyDown={(event) =>
                                            handleSessionOptionKeyDown(event, session.id)
                                          }
                                        >
                                          <div class="flex min-w-0 items-center gap-2">
                                            <div class="min-w-0 flex-1 truncate text-sm font-medium text-base-content">
                                              {session.title || 'Untitled'}
                                            </div>
                                            <Show when={isSessionWorking(session.id)}>
                                              <span class="inline-flex shrink-0 items-center gap-1 rounded border border-blue-200 bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold text-blue-700 dark:border-blue-800 dark:bg-blue-950/60 dark:text-blue-200">
                                                <LoaderCircleIcon class="h-3 w-3 animate-spin" />
                                                Working
                                              </span>
                                            </Show>
                                            <Show
                                              when={
                                                isSessionCurrent(session.id) &&
                                                !isSessionWorking(session.id)
                                              }
                                            >
                                              <span class="shrink-0 rounded border border-blue-200 bg-blue-100 px-1.5 py-0.5 text-[10px] font-semibold uppercase text-blue-700 dark:border-blue-800 dark:bg-blue-950/60 dark:text-blue-200">
                                                Current
                                              </span>
                                            </Show>
                                          </div>
                                          <div class="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-muted">
                                            <span>
                                              {formatSessionPickerMessageCount(
                                                session.message_count,
                                              )}
                                            </span>
                                            <Show when={formatSessionPickerUpdatedAt(session)}>
                                              {(updatedAt) => (
                                                <>
                                                  <span aria-hidden="true">·</span>
                                                  <span>{updatedAt()}</span>
                                                </>
                                              )}
                                            </Show>
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
                                      }
                                    >
                                      <form
                                        class="min-w-0 flex-1"
                                        onSubmit={(event) => submitSessionRename(session, event)}
                                        onClick={(event) => event.stopPropagation()}
                                      >
                                        <div class="flex min-w-0 items-center gap-1.5">
                                          <input
                                            ref={(input) => {
                                              sessionRenameInputRef = input;
                                            }}
                                            value={sessionRenameDraft()}
                                            onInput={(event) =>
                                              setSessionRenameDraft(event.currentTarget.value)
                                            }
                                            onKeyDown={handleSessionRenameKeyDown}
                                            maxLength={AI_CHAT_SESSION_TITLE_MAX_LENGTH}
                                            disabled={sessionRenameSaving()}
                                            aria-label={`New title for ${session.title || 'Untitled'}`}
                                            class="min-w-0 flex-1 rounded border border-blue-300 bg-surface px-2 py-1 text-sm font-medium text-base-content outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 disabled:opacity-70 dark:border-blue-800"
                                          />
                                          <ActionIconButton
                                            type="submit"
                                            disabled={sessionRenameSaving()}
                                            tone="accentGhost"
                                            size="sm"
                                            label={AI_CHAT_RENAME_SESSION_SAVE_LABEL}
                                            title={AI_CHAT_RENAME_SESSION_SAVE_LABEL}
                                          >
                                            <Show
                                              when={sessionRenameSaving()}
                                              fallback={
                                                <CheckIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                              }
                                            >
                                              <LoaderCircleIcon
                                                class="h-3.5 w-3.5 animate-spin"
                                                aria-hidden="true"
                                              />
                                            </Show>
                                          </ActionIconButton>
                                          <ActionIconButton
                                            disabled={sessionRenameSaving()}
                                            onClick={cancelRenamingSession}
                                            tone="muted"
                                            size="sm"
                                            label={AI_CHAT_RENAME_SESSION_CANCEL_LABEL}
                                            title={AI_CHAT_RENAME_SESSION_CANCEL_LABEL}
                                          >
                                            <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                          </ActionIconButton>
                                        </div>
                                      </form>
                                    </Show>
                                    <Show when={!isSessionRenaming(session.id)}>
                                      <div class="flex shrink-0 items-center gap-1">
                                        <ActionIconButton
                                          tone="muted"
                                          size="xs"
                                          class="opacity-0 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100"
                                          onClick={(event) => startRenamingSession(session, event)}
                                          label={getSessionRenameLabel(session)}
                                          title={getSessionRenameLabel(session)}
                                        >
                                          <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                        </ActionIconButton>
                                        <ActionIconButton
                                          tone={
                                            isSessionPinned(session.id) ? 'accentGhost' : 'muted'
                                          }
                                          size="xs"
                                          class={`transition-opacity focus:opacity-100 ${
                                            isSessionPinned(session.id)
                                              ? 'opacity-100'
                                              : 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100'
                                          }`}
                                          onClick={(event) =>
                                            togglePinnedSession(session.id, event)
                                          }
                                          aria-pressed={isSessionPinned(session.id)}
                                          label={getSessionPinLabel(session)}
                                          title={getSessionPinLabel(session)}
                                        >
                                          <BookmarkIcon
                                            class={`h-3.5 w-3.5 ${isSessionPinned(session.id) ? 'fill-current' : ''}`}
                                          />
                                        </ActionIconButton>
                                        <ActionIconButton
                                          tone="danger"
                                          size="xs"
                                          class="opacity-0 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100"
                                          onClick={(event) =>
                                            handleDeleteSession(session.id, event)
                                          }
                                          label={getSessionDeleteLabel(session)}
                                          title={getSessionDeleteLabel(session)}
                                        >
                                          <Trash2Icon class="h-3.5 w-3.5" />
                                        </ActionIconButton>
                                      </div>
                                    </Show>
                                  </div>
                                )}
                              </For>
                            </>
                          )}
                        </For>
                      </Show>
                    </div>
                  </div>
                </Show>
              </div>
            </div>

            {/* Close button (Always visible as fallback) */}
            <ActionIconButton
              onClick={(e) => {
                e.stopPropagation();
                props.onClose();
              }}
              tone="neutral"
              size="lg"
              class="order-2 sm:order-none"
              title={AI_CHAT_CLOSE_LABEL}
              label={AI_CHAT_CLOSE_LABEL}
              data-testid="assistant-close-button"
            >
              <XIcon class="h-5 w-5" />
            </ActionIconButton>
          </div>

          <Show when={transcriptCopyFallback()}>
            {(fallback) => (
              <section
                class="border-b border-amber-200 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-100"
                aria-label={AI_CHAT_TRANSCRIPT_FALLBACK_TITLE}
              >
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-xs font-semibold">{AI_CHAT_TRANSCRIPT_FALLBACK_TITLE}</div>
                  <div class="flex items-center gap-1.5">
                    <ActionIconButton
                      onClick={downloadFallbackTranscript}
                      tone="warningOutline"
                      size="sm"
                      title={AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL}
                      label={AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL}
                    >
                      <DownloadIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ActionIconButton>
                    <ActionIconButton
                      onClick={() => setTranscriptCopyFallback(null)}
                      tone="warningGhost"
                      size="sm"
                      title={AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL}
                      label={AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL}
                    >
                      <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ActionIconButton>
                  </div>
                </div>
                <textarea
                  ref={transcriptFallbackTextareaRef}
                  class="h-36 w-full resize-y rounded-md border border-amber-200 bg-surface px-2 py-2 font-mono text-[11px] leading-relaxed text-base-content outline-none focus:border-amber-400 dark:border-amber-800 dark:bg-surface"
                  readonly
                  value={fallback().transcript}
                  aria-label={AI_CHAT_TRANSCRIPT_FALLBACK_TEXTAREA_LABEL}
                />
              </section>
            )}
          </Show>

          <Show when={providerReadinessPresentation()}>
            {(presentation) => (
              <section
                class={`border-b px-4 py-2.5 text-[11px] ${
                  presentation().tone === 'checking'
                    ? 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-950 dark:text-blue-200'
                    : presentation().tone === 'ready'
                      ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950 dark:text-emerald-100'
                      : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100'
                }`}
                aria-label="Assistant selected model route status"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div class="flex min-w-0 items-start gap-2.5">
                    <span
                      class={`mt-1 h-2 w-2 flex-shrink-0 rounded-full ${
                        presentation().tone === 'checking'
                          ? 'bg-blue-500 dark:bg-blue-300'
                          : presentation().tone === 'ready'
                            ? 'bg-emerald-500 dark:bg-emerald-300'
                            : 'bg-amber-500 dark:bg-amber-300'
                      }`}
                    />
                    <div class="min-w-0">
                      <div class="font-semibold text-base-content">{presentation().title}</div>
                      <div class="mt-0.5 leading-5">{presentation().body}</div>
                      <Show when={providerReadiness().model}>
                        {(model) => (
                          <div
                            class="mt-0.5 truncate text-[10px] leading-5 text-muted"
                            title={`Selected route ID: ${model()}`}
                          >
                            Route: {formatAIModelRouteLabel(model())}
                          </div>
                        )}
                      </Show>
                      <Show
                        when={providerReadiness().status === 'error' && providerReadiness().model}
                      >
                        <div class="mt-0.5 text-[10px] leading-5 text-muted">
                          This route stays selected until you choose another route.
                        </div>
                      </Show>
                      <Show when={presentation().recommendation}>
                        {(recommendation) => <div class="mt-0.5 leading-5">{recommendation()}</div>}
                      </Show>
                    </div>
                  </div>
                  <Show when={providerReadiness().status === 'error' || providerReadinessVisible()}>
                    <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                      <Show when={providerReadinessAlternative()}>
                        {(alternative) => (
                          <button
                            type="button"
                            onClick={switchToProviderReadinessAlternative}
                            class="inline-flex max-w-[11rem] items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                            aria-label={providerReadinessAlternativeButtonLabel()}
                            title={alternative().label}
                          >
                            <span class="truncate">
                              {providerReadinessAlternativeButtonLabel()}
                            </span>
                          </button>
                        )}
                      </Show>
                      <button
                        type="button"
                        onClick={retrySelectedProviderReadiness}
                        disabled={providerReadiness().status === 'checking'}
                        class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
                        aria-label="Retry route check"
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
                      <Show when={providerReadinessVisible()}>
                        <button
                          type="button"
                          onClick={() => setProviderReadinessVisible(false)}
                          class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                          aria-label="Hide route status"
                        >
                          <XIcon class="h-3.5 w-3.5" />
                          <span>Hide</span>
                        </button>
                      </Show>
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
              <ActionIconButton
                onClick={() => setDiscoveryHintDismissed(true)}
                tone="infoGhost"
                size="xs"
                title={AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL}
                label={AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL}
              >
                <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
              </ActionIconButton>
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
            onRetry={(messageId) => void chat.retryMessage(messageId)}
            onRegenerate={
              chat.isLoading() ? undefined : (messageId) => void chat.retryMessage(messageId)
            }
            onEditPrompt={canUndoLastTurn() ? () => void handleUndoLastTurn() : undefined}
            onChangeModel={openModelSelectorFromError}
            getModelRouteLabel={formatChatMessageModelRoute}
            getModelRouteAlternative={getFailedTurnModelRouteAlternative}
            onUseModelRoute={switchToModelRoute}
            queuedFollowUps={chat.queuedFollowUps()}
            queuedFollowUpsPaused={chat.queuedFollowUpsPaused()}
            onEditQueuedFollowUp={editQueuedFollowUp}
            onCancelQueuedFollowUp={(id) => {
              chat.cancelQueuedFollowUp(id);
              focusComposer();
            }}
            recentSessions={selectQuickResumeSessions(sessions(), chat.sessionId())}
            onLoadSession={handleLoadSession}
          />

          {/* Input */}
          <div class="border-t border-border bg-surface px-4 py-3">
            <Show
              when={
                currentStatus() ||
                autonomousWarningVisible() ||
                activityDockQueuedFollowUpCount() > 0
              }
            >
              <div
                class="mb-2 overflow-hidden rounded-md border border-border bg-surface-alt text-base-content shadow-sm"
                data-testid="assistant-activity-dock"
              >
                <Show when={currentStatus()}>
                  <div
                    class={`flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs ${currentStatusRowClass()}`}
                    data-status-kind={currentStatusKind()}
                  >
                    <div
                      class="flex min-w-0 flex-1 items-center gap-2"
                      role="status"
                      aria-label="Assistant active turn status"
                      aria-live="polite"
                    >
                      <Show
                        when={currentStatusKind() === 'retrying'}
                        fallback={
                          <LoaderCircleIcon
                            class={`h-3.5 w-3.5 shrink-0 ${currentStatusIconClass()}`}
                            aria-hidden="true"
                          />
                        }
                      >
                        <RefreshCwIcon
                          class={`h-3.5 w-3.5 shrink-0 ${currentStatusIconClass()}`}
                          aria-hidden="true"
                        />
                      </Show>
                      <span class="min-w-0 flex-1 truncate font-medium">{currentStatusText()}</span>
                      <span class="flex shrink-0 gap-0.5" aria-hidden="true">
                        <span
                          class={`h-1 w-1 rounded-full animate-bounce ${currentStatusDotClass()}`}
                          style="animation-delay: 0ms; animation-duration: 1s"
                        />
                        <span
                          class={`h-1 w-1 rounded-full animate-bounce ${currentStatusDotClass()}`}
                          style="animation-delay: 150ms; animation-duration: 1s"
                        />
                        <span
                          class={`h-1 w-1 rounded-full animate-bounce ${currentStatusDotClass()}`}
                          style="animation-delay: 300ms; animation-duration: 1s"
                        />
                      </span>
                    </div>
                    <Show when={activeTurnRoute()}>
                      {(route) => (
                        <div
                          class="inline-flex h-7 min-w-0 max-w-[42%] shrink items-center gap-1.5 rounded-md border border-border-subtle bg-surface px-2 text-[10px] font-medium text-muted"
                          role="status"
                          aria-label="Assistant active model route"
                          title={route().title}
                        >
                          <CpuIcon class="h-3 w-3 shrink-0 text-blue-500" aria-hidden="true" />
                          <span class="min-w-0 truncate">{route().label}</span>
                        </div>
                      )}
                    </Show>
                    <ActionIconButton
                      onClick={stopActiveResponse}
                      tone="outline"
                      size="sm"
                      class={
                        interruptArmed()
                          ? 'border-blue-400 ring-2 ring-blue-500/30'
                          : 'border-border'
                      }
                      title={interruptArmed() ? 'Stop response armed' : 'Stop'}
                      label={interruptArmed() ? 'Stop response armed' : 'Stop response'}
                    >
                      <SquareIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ActionIconButton>
                  </div>
                </Show>
                <Show when={autonomousWarningVisible()}>
                  <div
                    class={`flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs text-red-700 dark:text-red-200 ${
                      currentStatus() ? 'border-t border-border/70' : ''
                    }`}
                    role="status"
                    aria-label="Assistant chat actions warning"
                    aria-live="polite"
                  >
                    <span
                      class="h-1.5 w-1.5 shrink-0 rounded-full bg-red-500 dark:bg-red-300"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 flex-1 font-medium leading-4 sm:truncate">
                      Chat-only actions are allowed.
                    </span>
                    <button
                      type="button"
                      onClick={() => updateControlLevel('controlled')}
                      class="inline-flex shrink-0 items-center rounded-md border border-red-200 bg-surface px-2 py-1 text-[10px] font-medium text-red-700 transition-colors hover:bg-red-50 hover:text-red-900 dark:border-red-800 dark:bg-surface dark:text-red-200 dark:hover:bg-red-950/40"
                      aria-label={AI_CHAT_SWITCH_TO_APPROVAL_LABEL}
                    >
                      Switch to Ask first
                    </button>
                    <ActionIconButton
                      onClick={() => setAutonomousBannerDismissed(true)}
                      tone="danger"
                      size="xs"
                      title={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                      label={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                    >
                      <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </ActionIconButton>
                  </div>
                </Show>
                <Show when={activityDockQueuedFollowUpCount() > 0}>
                  <div
                    class={`px-2.5 py-1.5 ${
                      currentStatus() || autonomousWarningVisible()
                        ? 'border-t border-border/70'
                        : ''
                    }`}
                    role="status"
                    aria-label="Queued follow-up messages"
                  >
                    <div class="flex min-h-7 items-center gap-2">
                      <ClockIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                      <span class="min-w-0 flex-1 truncate text-xs font-medium">
                        {pluralizeCount(
                          activityDockQueuedFollowUpCount(),
                          'follow-up',
                          'follow-ups',
                        )}{' '}
                        {chat.queuedFollowUpsPaused() ? 'paused' : 'queued'}
                      </span>
                      <ActionIconButton
                        onClick={() => {
                          chat.clearQueuedFollowUps();
                          focusComposer();
                        }}
                        tone="accentGhost"
                        size="xs"
                        title="Clear queued follow-ups"
                        label="Clear queued follow-up messages"
                      >
                        <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                      </ActionIconButton>
                    </div>
                    <div class="mt-1 max-h-24 space-y-1 overflow-y-auto" role="list">
                      <For each={chat.queuedFollowUps()}>
                        {(queued, index) => {
                          const preview = () => queuedFollowUpPreview(queued.prompt);
                          const routeLabel = () => queuedFollowUpRouteLabel(queued);
                          const rowLabel = () =>
                            routeLabel()
                              ? `Queued follow-up: ${preview()}. Route: ${routeLabel()}. Press Enter to edit or Delete to remove.`
                              : `Queued follow-up: ${preview()}. Press Enter to edit or Delete to remove.`;
                          return (
                            <div
                              class="flex min-h-7 items-center gap-2 rounded-md bg-white/70 px-2 py-1 text-xs text-blue-900 outline-none transition-colors focus:bg-white focus:ring-2 focus:ring-blue-500/40 dark:bg-blue-900/30 dark:text-blue-100 dark:focus:bg-blue-900/50"
                              classList={{
                                'bg-blue-50 ring-2 ring-blue-500/60 dark:bg-blue-800/50':
                                  queuedFollowUpCommandTargetId() === queued.id,
                              }}
                              ref={(element) => {
                                queuedFollowUpRowRefs.set(queued.id, element);
                              }}
                              role="listitem"
                              tabIndex={0}
                              aria-label={rowLabel()}
                              data-testid="assistant-queued-follow-up-row"
                              data-assistant-queued-follow-up-id={queued.id}
                              data-assistant-queue-command-target={
                                queuedFollowUpCommandTargetId() === queued.id ? 'true' : undefined
                              }
                              onKeyDown={(event) =>
                                handleQueuedFollowUpRowKeyDown(event, queued.id)
                              }
                            >
                              <span class="min-w-0 flex-1">
                                <span class="block truncate">{preview()}</span>
                                <Show when={routeLabel()}>
                                  {(label) => (
                                    <span
                                      class="block truncate text-[10px] font-medium text-blue-700 dark:text-blue-200"
                                      title={queued.sendOptions?.model}
                                    >
                                      Route: {label()}
                                    </span>
                                  )}
                                </Show>
                              </span>
                              <Show when={chat.queuedFollowUpCount() > 1 && index() > 0}>
                                <ActionIconButton
                                  onClick={() => sendQueuedFollowUpNext(queued.id)}
                                  tone="accentGhost"
                                  size="xs"
                                  title="Send queued follow-up next"
                                  label={`Send queued follow-up next: ${preview()}`}
                                >
                                  <SendIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                </ActionIconButton>
                              </Show>
                              <Show when={chat.queuedFollowUpsPaused() && index() === 0}>
                                <ActionIconButton
                                  onClick={() => sendQueuedFollowUpNext(queued.id)}
                                  tone="accentGhost"
                                  size="xs"
                                  title="Resume queued follow-up"
                                  label={`Resume queued follow-up: ${preview()}`}
                                >
                                  <SendIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                </ActionIconButton>
                              </Show>
                              <ActionIconButton
                                onClick={() => editQueuedFollowUp(queued.id)}
                                tone="accentGhost"
                                size="xs"
                                title="Edit queued follow-up"
                                label={`Edit queued follow-up: ${preview()}`}
                              >
                                <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
                              </ActionIconButton>
                              <ActionIconButton
                                onClick={() => {
                                  chat.cancelQueuedFollowUp(queued.id);
                                  focusComposer();
                                }}
                                tone="accentGhost"
                                size="xs"
                                title="Remove queued follow-up"
                                label={`Remove queued follow-up: ${preview()}`}
                              >
                                <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                              </ActionIconButton>
                            </div>
                          );
                        }}
                      </For>
                    </div>
                  </div>
                </Show>
              </div>
            </Show>
            <Show when={assistantWorkflowStarters().length > 0}>
              <div
                class="mb-2 flex min-h-7 min-w-0 flex-wrap items-center gap-1.5"
                aria-label="Assistant workflow starters"
                data-testid="assistant-workflow-starters"
              >
                <For each={assistantWorkflowStarters()}>
                  {(starter) => {
                    const isRendering = () => renderingWorkflowStarterId() === starter.id;
                    const label = () =>
                      starter.description
                        ? `${starter.label}: ${starter.description}`
                        : starter.label;
                    return (
                      <button
                        type="button"
                        onClick={() => {
                          void handleWorkflowStarterSelect(starter);
                        }}
                        disabled={Boolean(renderingWorkflowStarterId())}
                        class="inline-flex h-7 max-w-full items-center gap-1.5 rounded-md border border-border bg-surface-alt px-2 text-[11px] font-medium text-base-content transition-colors hover:border-blue-300 hover:bg-blue-50 disabled:cursor-wait disabled:opacity-60 dark:hover:border-blue-800 dark:hover:bg-blue-950/40"
                        title={label()}
                        aria-label={label()}
                        data-testid={`assistant-workflow-starter-${starter.name}`}
                      >
                        <Show
                          when={isRendering()}
                          fallback={
                            <span class="shrink-0 text-muted">{workflowStarterIcon(starter)}</span>
                          }
                        >
                          <LoaderCircleIcon class="h-3.5 w-3.5 shrink-0 animate-spin text-blue-600 dark:text-blue-300" />
                        </Show>
                        <span class="min-w-0 truncate">{starter.label}</span>
                      </button>
                    );
                  }}
                </For>
              </div>
            </Show>
            <Show when={pastedBlocks().length > 0}>
              <div
                class="mb-2 flex min-w-0 flex-wrap items-center gap-1.5"
                aria-label="Pasted text attachments"
                data-testid="assistant-pasted-blocks"
              >
                <For each={pastedBlocks()}>
                  {(block) => (
                    <div class="inline-flex h-7 max-w-full items-center gap-0.5 rounded-md border border-border bg-surface-alt pl-2 pr-0.5 text-[11px] font-medium text-base-content">
                      <ClipboardCopyIcon class="h-3 w-3 shrink-0 text-muted" aria-hidden="true" />
                      <button
                        type="button"
                        onClick={() => expandPastedBlock(block.id)}
                        class="min-w-0 truncate px-1 hover:text-blue-700 dark:hover:text-blue-300"
                        title="Click to edit the pasted text in the composer"
                        aria-label={`Edit ${pastedBlockLabel(block)} in the composer`}
                      >
                        {pastedBlockLabel(block)}
                      </button>
                      <ActionIconButton
                        onClick={() => removePastedBlock(block.id)}
                        tone="muted"
                        size="2xs"
                        title="Remove pasted text"
                        label={`Remove ${pastedBlockLabel(block)}`}
                      >
                        <XIcon class="h-3 w-3" aria-hidden="true" />
                      </ActionIconButton>
                    </div>
                  )}
                </For>
              </div>
            </Show>
            <form
              data-assistant-composer
              onSubmit={(e) => {
                e.preventDefault();
                handleSubmit();
              }}
              class="relative"
            >
              <div
                class={`relative flex min-h-[56px] items-end rounded-lg border bg-surface-alt shadow-sm transition-colors ${
                  mentionActive() || slashCommandActive()
                    ? 'border-blue-400 ring-2 ring-blue-500/20'
                    : 'border-border focus-within:border-blue-500 focus-within:ring-2 focus-within:ring-blue-500/20'
                }`}
              >
                <textarea
                  ref={textareaRef}
                  value={input()}
                  onInput={handleInputChange}
                  onKeyDown={handleKeyDown}
                  onPaste={handleComposerPaste}
                  placeholder={AI_CHAT_INPUT_PLACEHOLDER}
                  rows={1}
                  class="max-h-40 min-h-[54px] flex-1 resize-none bg-transparent px-3.5 py-3.5 pr-14 text-sm leading-5 text-base-content placeholder-slate-400 focus:outline-none"
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
                <SlashCommandAutocomplete
                  availability={assistantCommandAvailability()}
                  query={slashCommandQuery()}
                  position={{ top: 58, left: 0 }}
                  onSelect={handleSlashCommandSelect}
                  onClose={() => closeSlashCommandAutocomplete({ clearTransientDraft: true })}
                  visible={slashCommandActive()}
                />
                <div class="absolute bottom-2 right-2 flex items-center gap-1.5">
                  <ActionIconButton
                    type="submit"
                    disabled={!input().trim() && pastedBlocks().length === 0}
                    tone="primary"
                    size="lg"
                    title={chat.isLoading() ? 'Queue follow-up' : 'Send'}
                    label={chat.isLoading() ? 'Queue follow-up' : 'Send message'}
                  >
                    <SendIcon class="h-4 w-4" />
                  </ActionIconButton>
                </div>
              </div>
            </form>
            <div
              class="mt-1.5 flex min-h-7 min-w-0 flex-col items-stretch gap-1 sm:flex-row sm:items-center sm:justify-between sm:gap-2"
              data-testid="assistant-composer-chrome"
            >
              <div
                class="flex min-w-0 flex-wrap items-center gap-1.5 sm:flex-1 sm:flex-nowrap sm:overflow-hidden"
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
                  initialSearchQuery={modelSelectorInitialSearch()}
                  onModelSelect={selectModel}
                  onRefresh={() => loadModels(true)}
                  onManageProviders={openAssistantProviderSettings}
                />
                <Show when={providerRouteHealth()}>
                  {(health) => (
                    <div
                      role="status"
                      aria-label="Assistant selected model route health"
                      aria-live="polite"
                      title={health().title}
                      class={`inline-flex h-7 max-w-[11rem] shrink-0 items-center gap-1.5 rounded-md border px-2 text-[10px] font-medium ${health().className}`}
                      data-testid="assistant-provider-route-health"
                    >
                      <span
                        class={`h-1.5 w-1.5 shrink-0 rounded-full ${health().dotClassName}`}
                        aria-hidden="true"
                      />
                      <span class="min-w-0 truncate">{health().label}</span>
                    </div>
                  )}
                </Show>
                <Show when={assistantSurfaceToolHealth()}>
                  {(health) => (
                    <div
                      role="status"
                      aria-label="Assistant capability availability"
                      aria-live="polite"
                      title={health().title}
                      class={`inline-flex h-7 max-w-[9rem] shrink-0 items-center gap-1.5 rounded-md border px-2 text-[10px] font-medium ${health().className}`}
                      data-testid="assistant-surface-tools-health"
                    >
                      <WrenchIcon
                        class={`h-3 w-3 shrink-0 ${health().iconClassName}`}
                        aria-hidden="true"
                      />
                      <span
                        class={`h-1.5 w-1.5 shrink-0 rounded-full ${health().dotClassName}`}
                        aria-hidden="true"
                      />
                      <span class="min-w-0 truncate">{health().label}</span>
                    </div>
                  )}
                </Show>
                <ActionIconButton
                  onClick={openAssistantCommandHelp}
                  tone="outline"
                  size="sm"
                  title={AI_CHAT_COMMAND_HELP_BUTTON_LABEL}
                  label={AI_CHAT_COMMAND_HELP_BUTTON_LABEL}
                  data-testid="assistant-command-help-trigger"
                >
                  <CircleHelpIcon class="h-3.5 w-3.5" aria-hidden="true" />
                </ActionIconButton>
                <ActionIconButton
                  onClick={() => cycleRecentModelRoute()}
                  disabled={!nextRecentModelRoute()}
                  tone="outline"
                  size="sm"
                  title={
                    nextRecentModelRouteLabel()
                      ? `${AI_CHAT_CYCLE_RECENT_MODEL_LABEL}: ${nextRecentModelRouteLabel()}`
                      : AI_CHAT_CYCLE_RECENT_MODEL_LABEL
                  }
                  label={
                    nextRecentModelRouteLabel()
                      ? `${AI_CHAT_CYCLE_RECENT_MODEL_LABEL}: ${nextRecentModelRouteLabel()}`
                      : AI_CHAT_CYCLE_RECENT_MODEL_LABEL
                  }
                >
                  <RotateCwIcon class="h-3.5 w-3.5" aria-hidden="true" />
                </ActionIconButton>

                <div class="relative" data-dropdown>
                  <button
                    type="button"
                    ref={controlModeButtonRef}
                    onClick={toggleControlMenu}
                    onKeyDown={handleControlModeTriggerKeyDown}
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
                    <span>Chat: {controlPresentation().label}</span>
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
                        ref={(button) => {
                          controlModeOptionRefs.set('read_only', button);
                        }}
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'read_only'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'read_only' ? getAIChatControlLevelPresentation('read_only').selectedClassName : ''}`}
                        onKeyDown={(event) => handleControlModeOptionKeyDown(event, 'read_only')}
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
                        ref={(button) => {
                          controlModeOptionRefs.set('controlled', button);
                        }}
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'controlled'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'controlled' ? getAIChatControlLevelPresentation('controlled').selectedClassName : ''}`}
                        onKeyDown={(event) => handleControlModeOptionKeyDown(event, 'controlled')}
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
                        ref={(button) => {
                          controlModeOptionRefs.set('autonomous', button);
                        }}
                        role="menuitemradio"
                        aria-checked={controlLevel() === 'autonomous'}
                        class={`w-full text-left px-3 py-2.5 text-xs hover:bg-surface-hover transition-colors ${controlLevel() === 'autonomous' ? getAIChatControlLevelPresentation('autonomous').selectedClassName : ''}`}
                        onKeyDown={(event) => handleControlModeOptionKeyDown(event, 'autonomous')}
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

              <Show when={lastAssistantTurnSummary()}>
                {(summary) => (
                  <div
                    class="flex h-5 min-w-0 items-center justify-end self-end text-[10px] font-medium text-muted sm:h-7 sm:max-w-[14rem] sm:shrink-0 sm:self-auto"
                    aria-label={summary().title}
                    title={summary().title}
                  >
                    <span class="truncate">{summary().label}</span>
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
