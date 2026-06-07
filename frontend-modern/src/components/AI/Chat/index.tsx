import {
  Component,
  Show,
  createSignal,
  onMount,
  onCleanup,
  For,
  createMemo,
  createEffect,
  untrack,
} from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { unwrap } from 'solid-js/store';
import SendIcon from 'lucide-solid/icons/send';
import SquareIcon from 'lucide-solid/icons/square';
import ClockIcon from 'lucide-solid/icons/clock';
import ClipboardCopyIcon from 'lucide-solid/icons/clipboard-copy';
import CopyIcon from 'lucide-solid/icons/copy';
import DownloadIcon from 'lucide-solid/icons/download';
import GitForkIcon from 'lucide-solid/icons/git-fork';
import PencilIcon from 'lucide-solid/icons/pencil';
import Redo2Icon from 'lucide-solid/icons/redo-2';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import Undo2Icon from 'lucide-solid/icons/undo-2';
import XIcon from 'lucide-solid/icons/x';
import BookmarkIcon from 'lucide-solid/icons/bookmark';
import CheckIcon from 'lucide-solid/icons/check';
import CircleHelpIcon from 'lucide-solid/icons/circle-help';
import LoaderCircleIcon from 'lucide-solid/icons/loader-circle';
import Minimize2Icon from 'lucide-solid/icons/minimize-2';
import PlusIcon from 'lucide-solid/icons/plus';
import Trash2Icon from 'lucide-solid/icons/trash-2';
import { AIAPI } from '@/api/ai';
import {
  AIChatAPI,
  type ChatMention,
  type ChatSession,
  type ChatSessionHandoffSummary,
} from '@/api/aiChat';
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
  AI_CHAT_LAST_TURN_USAGE_LABEL,
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
import {
  getPreferredResourceDisplayName,
  getPreferredResourceHostname,
} from '@/utils/resourceIdentity';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import {
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
import {
  formatAssistantWorkflowStatus,
  getAssistantActiveTurnStatus,
  withAssistantQueuedFollowUpStatus,
} from './activeTurnStatus';
import { getLastAssistantAnswerText } from './assistantAnswerText';
import {
  getNextAssistantRecentModelRoute,
  isAssistantExplicitModelRoute,
  normalizeAssistantRecentModelRoutes,
} from './assistantModelRoutes';
import {
  filterAssistantSlashCommands,
  parseAssistantSlashCommandInput,
  type AssistantSlashCommand,
  type AssistantSlashCommandAction,
} from './assistantSlashCommands';
import {
  buildAssistantTranscriptFilename,
  downloadAssistantTranscriptFile,
  formatAssistantTranscript,
  hasAssistantTranscriptContent,
} from './transcriptExport';
import {
  latestWorkflowStatus,
  normalizeWorkflowStatusSequence,
  workflowStatusRenderKey,
} from './workflowStatusPresentation';
import type {
  ChatMessage,
  ModelRouteRecoveryOption,
  PendingApproval,
  PendingQuestion,
  WorkflowStatus,
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
const AI_CHAT_WORKFLOW_STATUS_BURST_VISIBLE_MS = 650;
const STRUCTURED_PATROL_CONTEXT_TARGETS = new Set(['patrol-configuration', 'patrol-run']);
const STRUCTURED_RESOURCE_CONTEXT_HANDOFF_KINDS = new Set(['resource_context']);
const AI_CHAT_CYCLE_RECENT_MODEL_LABEL = 'Cycle recent Assistant model';
const AI_CHAT_CONTROL_LEVEL_ORDER: AIControlLevel[] = ['read_only', 'controlled', 'autonomous'];
const AI_CHAT_MODEL_SLASH_HELP =
  'Use /model provider:model-id, /model default, /model next, or /model previous.';
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

interface AssistantUsageSummary {
  label: string;
  title: string;
}

interface TranscriptCopyFallback {
  generatedAt: Date;
  transcript: string;
}

interface AIChatProps {
  onClose: () => void;
}

let stashedComposerDraft: ComposerDraftStash | null = null;

interface AssistantFallbackRouteAdoptionCandidate {
  route: string;
  failedModel: string;
}

interface AssistantFallbackRouteNotice extends AssistantFallbackRouteAdoptionCandidate {
  failedModelLabel: string;
  messageId: string;
  routeLabel: string;
}

interface AssistantProviderReadinessRouteNotice {
  failedProviderLabel: string;
  failedRouteLabel: string;
  routeLabel: string;
}

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
      (Boolean(event.failedModel?.trim()) ||
        event.modelEvent === 'fallback' ||
        event.modelEvent === 'switch'),
  );
};

const getAssistantFallbackRouteAdoptionCandidate = (
  message: ChatMessage,
): AssistantFallbackRouteAdoptionCandidate | null => {
  if (message.role !== 'assistant' || message.error) return null;

  const events = [...(message.streamEvents || [])].reverse();
  const modelSwitch = events.find(
    (event) =>
      event.type === 'model_switch' &&
      typeof event.model === 'string' &&
      event.model.trim() &&
      typeof event.failedModel === 'string' &&
      event.failedModel.trim(),
  );
  if (!modelSwitch) return null;

  const route = modelSwitch.model?.trim() || '';
  const failedModel = modelSwitch.failedModel?.trim() || '';
  if (!route || !failedModel) return null;

  const completedModel = (message.model || '').trim();
  if (message.isStreaming === false && completedModel && completedModel !== route) {
    return null;
  }

  return { route, failedModel };
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
  const navigate = useNavigate();
  // UI state - use store's isOpenSignal for reactivity
  const isOpen = aiChatStore.isOpenSignal;
  const { width } = useBreakpoint();
  const [input, setInput] = createSignal('');
  const [editingQueuedFollowUp, setEditingQueuedFollowUp] = createSignal<QueuedFollowUp | null>(
    null,
  );
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
  const [defaultModel, setDefaultModel] = createSignal('');
  const [chatOverrideModel, setChatOverrideModel] = createSignal('');
  const pendingFallbackRouteAdoptions = new Map<string, AssistantFallbackRouteAdoptionCandidate>();
  const adoptedFallbackRouteMessageIds = new Set<string>();
  const [fallbackRouteNotice, setFallbackRouteNotice] =
    createSignal<AssistantFallbackRouteNotice | null>(null);
  const [providerReadinessRouteNotice, setProviderReadinessRouteNotice] =
    createSignal<AssistantProviderReadinessRouteNotice | null>(null);
  const [providerReadiness, setProviderReadiness] = createSignal<ChatProviderReadinessState>({
    status: 'idle',
    provider: '',
  });
  const [providerReadinessVisible, setProviderReadinessVisible] = createSignal(false);
  const [providerReadinessRetryNonce, setProviderReadinessRetryNonce] = createSignal(0);
  const [controlLevel, setControlLevel] = createSignal<AIControlLevel>('read_only');
  const [showControlMenu, setShowControlMenu] = createSignal(false);
  const [controlSaving, setControlSaving] = createSignal(false);
  const [transcriptCopyFallback, setTranscriptCopyFallback] =
    createSignal<TranscriptCopyFallback | null>(null);
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
  const [slashCommandActive, setSlashCommandActive] = createSignal(false);
  const [slashCommandQuery, setSlashCommandQuery] = createSignal('');
  let textareaRef: HTMLTextAreaElement | undefined;
  let transcriptFallbackTextareaRef: HTMLTextAreaElement | undefined;
  let interruptArmTimeout: ReturnType<typeof setTimeout> | undefined;
  let composerSubmitDispatchLocked = false;
  let handledAssistantCommandRequestId = 0;

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
    queueMicrotask(() => sessionSearchInputRef?.focus());
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

  const surfaceProviderReadinessRouteAdoption = (args: {
    failedProvider: string;
    failedRoute: string;
    route: string;
  }) => {
    const routeLabel = formatChatMessageModelRoute(args.route);
    const failedRouteLabel = formatChatMessageModelRoute(args.failedRoute);
    const failedProviderLabel = getAIProviderDisplayName(args.failedProvider) || 'selected';
    setProviderReadinessRouteNotice({
      failedProviderLabel,
      failedRouteLabel,
      routeLabel,
    });
    notificationStore.success(
      `Assistant model route switched to ${routeLabel} after ${failedProviderLabel} provider check`,
      2500,
    );
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

  const selectModel = (modelId: string, options: { rememberRecent?: boolean } = {}) => {
    chat.setModel(modelId);
    updateStoredModel(chat.sessionId(), modelId);
    if (options.rememberRecent !== false) {
      rememberRecentModel(modelId);
    }
    setFallbackRouteNotice(null);
    setProviderReadinessRouteNotice(null);
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
      setModelSelectorOpenRequest((value) => value + 1);
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

    if (!isAssistantExplicitModelRoute(target)) {
      notificationStore.error(AI_CHAT_MODEL_SLASH_HELP);
      focusComposer();
      return false;
    }

    selectModel(target);
    notificationStore.success(
      `Assistant model route set to ${formatChatMessageModelRoute(target)}`,
      2000,
    );
    focusComposer();
    return true;
  };

  const openModelSelectorFromError = () => {
    setModelSelectorOpenRequest((value) => value + 1);
  };

  const getFailedTurnModelRouteAlternative = (message: ChatMessage) => {
    if (!hasProviderRouteFailureEvidence(message)) return null;
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

    const failedRoute = selectedChatModel().trim();
    const currentProvider = selectedChatProvider().trim();
    const failedProvider = readiness.provider.trim();
    if (failedProvider && currentProvider && failedProvider !== currentProvider) {
      return null;
    }

    const alternative = providerReadinessAlternative();
    if (!alternative) return null;

    selectModel(alternative.id);
    surfaceProviderReadinessRouteAdoption({
      failedProvider: failedProvider || currentProvider || providerForModelRoute(failedRoute),
      failedRoute,
      route: alternative.id,
    });
    return alternative;
  };

  createEffect(() => {
    const messages = chat.messages();
    const visibleMessageIds = new Set(messages.map((message) => message.id));
    for (const messageId of [...pendingFallbackRouteAdoptions.keys()]) {
      if (!visibleMessageIds.has(messageId)) {
        pendingFallbackRouteAdoptions.delete(messageId);
      }
    }
    for (const messageId of [...adoptedFallbackRouteMessageIds]) {
      if (!visibleMessageIds.has(messageId)) {
        adoptedFallbackRouteMessageIds.delete(messageId);
      }
    }

    for (const message of messages) {
      const candidate = getAssistantFallbackRouteAdoptionCandidate(message);
      if (!candidate) continue;

      if (message.isStreaming !== false) {
        pendingFallbackRouteAdoptions.set(message.id, candidate);
        continue;
      }

      const pending = pendingFallbackRouteAdoptions.get(message.id);
      if (
        !pending ||
        pending.route !== candidate.route ||
        adoptedFallbackRouteMessageIds.has(message.id)
      ) {
        continue;
      }

      const currentRoute = selectedChatModel().trim();
      if (currentRoute && currentRoute !== pending.failedModel) {
        pendingFallbackRouteAdoptions.delete(message.id);
        continue;
      }

      adoptedFallbackRouteMessageIds.add(message.id);
      pendingFallbackRouteAdoptions.delete(message.id);
      selectModel(candidate.route);
      const routeLabel = formatChatMessageModelRoute(candidate.route);
      const failedModelLabel = formatChatMessageModelRoute(pending.failedModel);
      setFallbackRouteNotice({
        ...candidate,
        failedModelLabel,
        messageId: message.id,
        routeLabel,
      });
      notificationStore.success(`Assistant model route switched to ${routeLabel}`, 2500);
    }
  });

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
  const activeAssistantMessage = createMemo(() => {
    if (!chat.isLoading()) return undefined;
    const messages = chat.messages();
    for (let index = messages.length - 1; index >= 0; index -= 1) {
      const message = messages[index];
      if (message.role === 'assistant' && message.isStreaming !== false) {
        return message;
      }
    }
    return undefined;
  });
  const activeWorkflowStatusSequence = createMemo(() => {
    const message = activeAssistantMessage();
    if (!message) return [];
    return normalizeWorkflowStatusSequence([
      ...(message.workflowStatusHistory || []),
      message.workflowStatus,
    ]);
  });
  const [displayedActiveWorkflowStatusKey, setDisplayedActiveWorkflowStatusKey] =
    createSignal('');
  const [pacingWorkflowStatusBurst, setPacingWorkflowStatusBurst] = createSignal(false);
  let lastWorkflowStatusSequenceFingerprint = '';
  let lastWorkflowStatusSequenceLength = 0;
  let workflowStatusAdvanceTimer: number | undefined;

  const clearWorkflowStatusAdvanceTimer = () => {
    if (workflowStatusAdvanceTimer) {
      window.clearTimeout(workflowStatusAdvanceTimer);
      workflowStatusAdvanceTimer = undefined;
    }
  };

  const scheduleWorkflowStatusAdvance = (keys: string[], currentIndex: number) => {
    clearWorkflowStatusAdvanceTimer();
    if (currentIndex >= keys.length - 1) {
      setPacingWorkflowStatusBurst(false);
      return;
    }

    setPacingWorkflowStatusBurst(true);
    workflowStatusAdvanceTimer = window.setTimeout(() => {
      const nextIndex = Math.min(currentIndex + 1, keys.length - 1);
      setDisplayedActiveWorkflowStatusKey(keys[nextIndex] || '');
      scheduleWorkflowStatusAdvance(keys, nextIndex);
    }, AI_CHAT_WORKFLOW_STATUS_BURST_VISIBLE_MS);
  };

  // Burst status histories can arrive in one render after cold starts. Pace
  // only those bursts so the dock shows visible progress instead of jumping.
  createEffect(() => {
    const sequence = activeWorkflowStatusSequence();
    const keys = sequence.map(workflowStatusRenderKey);
    const fingerprint = keys.join('\u001e');
    const previousFingerprint = lastWorkflowStatusSequenceFingerprint;
    const previousLength = lastWorkflowStatusSequenceLength;
    const sequenceChanged = fingerprint !== previousFingerprint;
    const addedCount = sequenceChanged ? Math.max(0, sequence.length - previousLength) : 0;

    lastWorkflowStatusSequenceFingerprint = fingerprint;
    lastWorkflowStatusSequenceLength = sequence.length;
    clearWorkflowStatusAdvanceTimer();

    if (sequence.length === 0) {
      setDisplayedActiveWorkflowStatusKey('');
      setPacingWorkflowStatusBurst(false);
      return;
    }

    const currentKey = untrack(displayedActiveWorkflowStatusKey);
    const currentIndex = keys.indexOf(currentKey);
    if (currentIndex === -1) {
      const initialIndex = sequence.length > 1 ? 0 : sequence.length - 1;
      setDisplayedActiveWorkflowStatusKey(keys[initialIndex] || '');
      if (initialIndex < sequence.length - 1) {
        scheduleWorkflowStatusAdvance(keys, initialIndex);
      } else {
        setPacingWorkflowStatusBurst(false);
      }
      return;
    }

    if (currentIndex >= sequence.length - 1) {
      setPacingWorkflowStatusBurst(false);
      return;
    }

    const pendingCount = sequence.length - currentIndex - 1;
    const shouldPace = untrack(pacingWorkflowStatusBurst) || addedCount > 1 || pendingCount > 1;
    if (shouldPace) {
      scheduleWorkflowStatusAdvance(keys, currentIndex);
      return;
    }

    setDisplayedActiveWorkflowStatusKey(keys[sequence.length - 1] || '');
    setPacingWorkflowStatusBurst(false);
  });

  onCleanup(clearWorkflowStatusAdvanceTimer);

  const displayedActiveWorkflowStatus = createMemo<WorkflowStatus | undefined>(() => {
    const sequence = activeWorkflowStatusSequence();
    const key = displayedActiveWorkflowStatusKey();
    if (key) {
      const matched = sequence.find((status) => workflowStatusRenderKey(status) === key);
      if (matched) return matched;
    }
    return latestWorkflowStatus(sequence);
  });
  const [currentStatusNow, setCurrentStatusNow] = createSignal(Date.now());
  const currentStatus = createMemo(() => {
    const status = getAssistantActiveTurnStatus(
      chat.messages(),
      chat.isLoading(),
      currentStatusNow(),
    );
    const message = activeAssistantMessage();
    const latestWorkflow = message?.workflowStatus;
    const pacedWorkflowStatus = displayedActiveWorkflowStatus();
    if (!status || !latestWorkflow || !pacedWorkflowStatus) return status;
    const messages = chat.messages();

    const latestWorkflowText = formatAssistantWorkflowStatus(latestWorkflow, currentStatusNow());
    const expectedType = latestWorkflow.tool ? 'tool' : 'thinking';
    const statusStartedAt = status.startedAt;
    const workflowStartedAt = latestWorkflow.startedAt;
    const latestWorkflowStatus = withAssistantQueuedFollowUpStatus(
      {
        type: expectedType,
        text: latestWorkflowText,
        ...(workflowStartedAt !== undefined ? { startedAt: workflowStartedAt } : {}),
      },
      messages,
    );
    if (
      !latestWorkflowText ||
      status.text !== latestWorkflowStatus.text ||
      status.type !== latestWorkflowStatus.type ||
      (statusStartedAt !== undefined &&
        workflowStartedAt !== undefined &&
        statusStartedAt !== workflowStartedAt)
    ) {
      return status;
    }

    const pacedText = formatAssistantWorkflowStatus(pacedWorkflowStatus, currentStatusNow());
    if (!pacedText) return status;
    return withAssistantQueuedFollowUpStatus(
      {
        ...status,
        type: pacedWorkflowStatus.tool ? 'tool' : 'thinking',
        text: pacedText,
        startedAt: pacedWorkflowStatus.startedAt,
      },
      messages,
    );
  });
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

    const failedRoute = current;
    selectModel(override);
    surfaceProviderReadinessRouteAdoption({
      failedProvider: failedProvider || currentProvider,
      failedRoute,
      route: override,
    });
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
      // Only close if click is outside dropdown containers
      if (!isInsideDropdown && !isInsideComposerPopup) {
        setShowSessions(false);
        setSessionRefreshLoading(false);
        resetSessionSearch();
        setShowControlMenu(false);
      }
      // Close mention autocomplete when clicking outside
      if (!target.closest('[data-mention-autocomplete]') && !target.closest('textarea')) {
        setMentionActive(false);
      }
      if (!target.closest('[data-slash-command-autocomplete]') && !target.closest('textarea')) {
        closeSlashCommandAutocomplete({ clearTransientDraft: true });
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
    setSlashCommandActive(false);
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

  const clearLocalComposerCommand = () => {
    stashedComposerDraft = null;
    setEditingQueuedFollowUp(null);
    setRestoredPromptDraft(null);
    setInput('');
    setAccumulatedMentions([]);
    setMentionActive(false);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    resetPromptHistoryNavigation();
    queueMicrotask(() => {
      resizeTextarea();
      focusComposer();
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

  const executeSlashCommand = (command: AssistantSlashCommandAction, args = '') => {
    const commandArgs = args.trim();
    if (command === 'models' && commandArgs) {
      const consumed = runModelSlashCommand(commandArgs);
      if (consumed) {
        clearLocalComposerCommand();
      }
      return consumed;
    }

    clearLocalComposerCommand();

    switch (command) {
      case 'new':
        setShowCommandHelp(false);
        void handleNewConversation();
        break;
      case 'sessions':
        setShowCommandHelp(false);
        void handleToggleSessions();
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
        setModelSelectorOpenRequest((value) => value + 1);
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
    const prompt = submittedInput.trim();
    if (!prompt) return;
    setFallbackRouteNotice(null);
    setProviderReadinessRouteNotice(null);
    composerSubmitDispatchLocked = true;
    queueMicrotask(() => {
      composerSubmitDispatchLocked = false;
    });
    if (submittedInput !== input()) {
      setInput(submittedInput);
    }
    const slashCommand = parseAssistantSlashCommandInput(prompt);
    if (slashCommand) {
      executeSlashCommand(slashCommand.action, slashCommand.args);
      return;
    }
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
        restoreFailedSubmitDraft(submittedInput, submittedMentions);
      });
    stashedComposerDraft = null;
    setInput('');
    setAccumulatedMentions([]);
    setMentionActive(false);
    setSlashCommandActive(false);
    setSlashCommandQuery('');
    focusComposer();
  };

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
      const hasMatches = filterAssistantSlashCommands(query, 1).length > 0;
      setSlashCommandActive(hasMatches);
      if (hasMatches) {
        setMentionActive(false);
      }
      return hasMatches;
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
    setInput(`/${command.name}`);
    executeSlashCommand(command.action);
  };

  const handleCommandHelpRun = (command: AssistantSlashCommand) => {
    setShowCommandHelp(false);
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
    setFallbackRouteNotice(null);
    setProviderReadinessRouteNotice(null);
    setRedoLastTurnAvailable(false);
    aiChatStore.clearContext?.();
    setShowSessions(false);
    setSessionRefreshLoading(false);
    resetSessionSearch();
    focusComposer();
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
      const message = error instanceof Error ? error.message : AI_CHAT_COMPACT_SESSION_ERROR_MESSAGE;
      notificationStore.error(message);
    } finally {
      setCompactingSession(false);
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
        setFallbackRouteNotice(null);
        setProviderReadinessRouteNotice(null);
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

              <button
                type="button"
                onClick={() => {
                  void handleForkSession();
                }}
                disabled={!canForkCurrentSession()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_FORK_SESSION_LABEL}
                aria-label={AI_CHAT_FORK_SESSION_LABEL}
                aria-busy={forkingSession()}
              >
                <Show
                  when={forkingSession()}
                  fallback={<GitForkIcon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </button>

              <button
                type="button"
                onClick={() => {
                  void handleCompactSession();
                }}
                disabled={!canCompactCurrentSession()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_COMPACT_SESSION_LABEL}
                aria-label={AI_CHAT_COMPACT_SESSION_LABEL}
                aria-busy={compactingSession()}
              >
                <Show
                  when={compactingSession()}
                  fallback={<Minimize2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </button>

              <button
                type="button"
                onClick={() => {
                  void handleUndoLastTurn();
                }}
                disabled={!canUndoLastTurn()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_UNDO_LAST_TURN_LABEL}
                aria-label={AI_CHAT_UNDO_LAST_TURN_LABEL}
                aria-busy={undoingLastTurn()}
              >
                <Show
                  when={undoingLastTurn()}
                  fallback={<Undo2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </button>

              <button
                type="button"
                onClick={() => {
                  void handleRedoLastTurn();
                }}
                disabled={!canRedoLastTurn()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_REDO_LAST_TURN_LABEL}
                aria-label={AI_CHAT_REDO_LAST_TURN_LABEL}
                aria-busy={redoingLastTurn()}
              >
                <Show
                  when={redoingLastTurn()}
                  fallback={<Redo2Icon class="h-4 w-4" aria-hidden="true" />}
                >
                  <LoaderCircleIcon class="h-4 w-4 animate-spin" aria-hidden="true" />
                </Show>
              </button>

              <button
                type="button"
                onClick={() => {
                  void copyLastAssistantAnswer();
                }}
                disabled={!hasLastAssistantAnswer()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_COPY_LAST_ANSWER_LABEL}
                aria-label={AI_CHAT_COPY_LAST_ANSWER_LABEL}
              >
                <ClipboardCopyIcon class="h-4 w-4" aria-hidden="true" />
              </button>

              <button
                type="button"
                onClick={() => {
                  void copyAssistantTranscript();
                }}
                disabled={!hasCurrentTranscript()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_COPY_TRANSCRIPT_LABEL}
                aria-label={AI_CHAT_COPY_TRANSCRIPT_LABEL}
              >
                <CopyIcon class="h-4 w-4" aria-hidden="true" />
              </button>

              <button
                type="button"
                onClick={exportAssistantTranscript}
                disabled={!hasCurrentTranscript()}
                class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-surface disabled:hover:text-muted"
                title={AI_CHAT_EXPORT_TRANSCRIPT_LABEL}
                aria-label={AI_CHAT_EXPORT_TRANSCRIPT_LABEL}
              >
                <DownloadIcon class="h-4 w-4" aria-hidden="true" />
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
                  <ClockIcon class="h-4 w-4" aria-hidden="true" />
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
                                          <button
                                            type="submit"
                                            disabled={sessionRenameSaving()}
                                            class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 disabled:cursor-wait disabled:opacity-70 dark:text-blue-200 dark:hover:bg-blue-900/60"
                                            aria-label={AI_CHAT_RENAME_SESSION_SAVE_LABEL}
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
                                          </button>
                                          <button
                                            type="button"
                                            disabled={sessionRenameSaving()}
                                            onClick={cancelRenamingSession}
                                            class="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content disabled:opacity-50"
                                            aria-label={AI_CHAT_RENAME_SESSION_CANCEL_LABEL}
                                            title={AI_CHAT_RENAME_SESSION_CANCEL_LABEL}
                                          >
                                            <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                          </button>
                                        </div>
                                      </form>
                                    </Show>
                                    <Show when={!isSessionRenaming(session.id)}>
                                      <div class="flex shrink-0 items-center gap-1">
                                        <button
                                          type="button"
                                          class="rounded p-1 text-muted opacity-0 transition-opacity hover:bg-blue-100 hover:text-blue-600 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100 dark:hover:bg-blue-900 dark:hover:text-blue-300"
                                          onClick={(event) => startRenamingSession(session, event)}
                                          aria-label={getSessionRenameLabel(session)}
                                          title={getSessionRenameLabel(session)}
                                        >
                                          <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                        </button>
                                        <button
                                          type="button"
                                          class={`rounded p-1 transition-opacity focus:opacity-100 hover:bg-blue-100 hover:text-blue-600 dark:hover:bg-blue-900 dark:hover:text-blue-300 ${
                                            isSessionPinned(session.id)
                                              ? 'opacity-100 text-blue-600 dark:text-blue-300'
                                              : 'opacity-0 text-muted group-hover:opacity-100 group-focus-within:opacity-100'
                                          }`}
                                          onClick={(event) =>
                                            togglePinnedSession(session.id, event)
                                          }
                                          aria-pressed={isSessionPinned(session.id)}
                                          aria-label={getSessionPinLabel(session)}
                                          title={getSessionPinLabel(session)}
                                        >
                                          <BookmarkIcon
                                            class={`h-3.5 w-3.5 ${isSessionPinned(session.id) ? 'fill-current' : ''}`}
                                          />
                                        </button>
                                        <button
                                          type="button"
                                          class="rounded p-1 text-muted opacity-0 transition-opacity hover:bg-red-100 hover:text-red-500 focus:opacity-100 group-hover:opacity-100 group-focus-within:opacity-100 dark:hover:bg-red-900"
                                          onClick={(event) =>
                                            handleDeleteSession(session.id, event)
                                          }
                                          aria-label={getSessionDeleteLabel(session)}
                                          title={getSessionDeleteLabel(session)}
                                        >
                                          <Trash2Icon class="h-3.5 w-3.5" />
                                        </button>
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

          <Show when={transcriptCopyFallback()}>
            {(fallback) => (
              <section
                class="border-b border-amber-200 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-100"
                aria-label={AI_CHAT_TRANSCRIPT_FALLBACK_TITLE}
              >
                <div class="mb-2 flex items-center justify-between gap-2">
                  <div class="text-xs font-semibold">{AI_CHAT_TRANSCRIPT_FALLBACK_TITLE}</div>
                  <div class="flex items-center gap-1.5">
                    <button
                      type="button"
                      onClick={downloadFallbackTranscript}
                      class="flex h-7 w-7 items-center justify-center rounded-md border border-amber-200 bg-surface text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:border-amber-800 dark:bg-amber-950/60 dark:text-amber-200 dark:hover:bg-amber-900"
                      title={AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL}
                      aria-label={AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL}
                    >
                      <DownloadIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setTranscriptCopyFallback(null)}
                      class="flex h-7 w-7 items-center justify-center rounded-md text-amber-700 transition-colors hover:bg-amber-100 hover:text-amber-900 dark:text-amber-200 dark:hover:bg-amber-900"
                      title={AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL}
                      aria-label={AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL}
                    >
                      <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
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
                aria-label="Assistant provider status"
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
                          <div class="mt-0.5 truncate text-[10px] leading-5 text-muted">
                            Route: {formatAIModelRouteLabel(model())}
                          </div>
                        )}
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
                        disabled={providerReadiness().status === 'checking'}
                        class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60"
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
                      <Show when={providerReadinessVisible()}>
                        <button
                          type="button"
                          onClick={() => setProviderReadinessVisible(false)}
                          class="inline-flex items-center gap-1.5 rounded-md border border-current/20 bg-surface px-2 py-1 text-[10px] font-medium text-base-content hover:bg-surface-hover"
                          aria-label="Hide provider status"
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
            queuedFollowUpsPaused={chat.queuedFollowUpsPaused()}
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

          {/* Input */}
          <div class="border-t border-border bg-surface px-4 py-3">
            <Show
              when={
                currentStatus() ||
                autonomousWarningVisible() ||
                fallbackRouteNotice() ||
                providerReadinessRouteNotice() ||
                chat.queuedFollowUpCount() > 0
              }
            >
              <div
                class="mb-2 overflow-hidden rounded-md border border-border bg-surface-alt text-base-content shadow-sm"
                data-testid="assistant-activity-dock"
              >
                <Show when={currentStatus()}>
                  <div
                    class="flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs"
                  >
                    <div
                      class="flex min-w-0 flex-1 items-center gap-2"
                      role="status"
                      aria-label="Assistant active turn status"
                      aria-live="polite"
                    >
                      <LoaderCircleIcon
                        class={`h-3.5 w-3.5 shrink-0 ${
                          currentStatus()?.type === 'generating'
                            ? 'text-emerald-500 dark:text-emerald-300'
                            : 'animate-spin text-blue-600 dark:text-blue-300'
                        }`}
                        aria-hidden="true"
                      />
                      <span class="min-w-0 flex-1 truncate font-medium">
                        {currentStatusText()}
                      </span>
                      <span class="flex shrink-0 gap-0.5" aria-hidden="true">
                        <span
                          class="h-1 w-1 rounded-full bg-blue-400 animate-bounce"
                          style="animation-delay: 0ms; animation-duration: 1s"
                        />
                        <span
                          class="h-1 w-1 rounded-full bg-blue-400 animate-bounce"
                          style="animation-delay: 150ms; animation-duration: 1s"
                        />
                        <span
                          class="h-1 w-1 rounded-full bg-blue-400 animate-bounce"
                          style="animation-delay: 300ms; animation-duration: 1s"
                        />
                      </span>
                    </div>
                    <button
                      type="button"
                      onClick={stopActiveResponse}
                      class={`inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border bg-surface text-base-content shadow-sm transition-colors hover:bg-surface-hover ${
                        interruptArmed()
                          ? 'border-blue-400 ring-2 ring-blue-500/30'
                          : 'border-border'
                      }`}
                      title={interruptArmed() ? 'Stop response armed' : 'Stop'}
                      aria-label={interruptArmed() ? 'Stop response armed' : 'Stop response'}
                    >
                      <SquareIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                  </div>
                </Show>
                <Show when={autonomousWarningVisible()}>
                  <div
                    class={`flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs text-red-700 dark:text-red-200 ${
                      currentStatus() ? 'border-t border-border/70' : ''
                    }`}
                    role="status"
                    aria-label="Assistant autonomous control warning"
                    aria-live="polite"
                  >
                    <span
                      class="h-1.5 w-1.5 shrink-0 rounded-full bg-red-500 dark:bg-red-300"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 flex-1 font-medium leading-4 sm:truncate">
                      Autonomous: commands execute without approval.
                    </span>
                    <button
                      type="button"
                      onClick={() => updateControlLevel('controlled')}
                      class="inline-flex shrink-0 items-center rounded-md border border-red-200 bg-surface px-2 py-1 text-[10px] font-medium text-red-700 transition-colors hover:bg-red-50 hover:text-red-900 dark:border-red-800 dark:bg-surface dark:text-red-200 dark:hover:bg-red-950/40"
                      aria-label={AI_CHAT_SWITCH_TO_APPROVAL_LABEL}
                    >
                      Switch to Approval
                    </button>
                    <button
                      type="button"
                      onClick={() => setAutonomousBannerDismissed(true)}
                      class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-red-500 transition-colors hover:bg-red-50 hover:text-red-700 dark:text-red-200 dark:hover:bg-red-950/40"
                      title={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                      aria-label={AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL}
                    >
                      <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                    </button>
                  </div>
                </Show>
                <Show when={fallbackRouteNotice()}>
                  {(notice) => (
                    <div
                      class={`flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs ${
                        currentStatus() || autonomousWarningVisible()
                          ? 'border-t border-border/70'
                          : ''
                      }`}
                      role="status"
                      aria-label="Assistant fallback route adopted"
                      aria-live="polite"
                    >
                      <CheckIcon
                        class="h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-300"
                        aria-hidden="true"
                      />
                      <span class="min-w-0 flex-1 truncate font-medium">
                        Using {notice().routeLabel} after fallback from {notice().failedModelLabel}
                      </span>
                      <button
                        type="button"
                        onClick={() => {
                          setFallbackRouteNotice(null);
                          focusComposer();
                        }}
                        class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-900 dark:text-blue-200 dark:hover:bg-blue-900/50"
                        title="Dismiss fallback route notice"
                        aria-label="Dismiss fallback route notice"
                      >
                        <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                      </button>
                    </div>
                  )}
                </Show>
                <Show when={providerReadinessRouteNotice()}>
                  {(notice) => (
                    <div
                      class={`flex min-h-8 min-w-0 items-center gap-2 px-2.5 py-1.5 text-xs ${
                        currentStatus() || autonomousWarningVisible() || fallbackRouteNotice()
                          ? 'border-t border-border/70'
                          : ''
                      }`}
                      role="status"
                      aria-label="Assistant provider readiness route adopted"
                      aria-live="polite"
                    >
                      <CheckIcon
                        class="h-3.5 w-3.5 shrink-0 text-emerald-600 dark:text-emerald-300"
                        aria-hidden="true"
                      />
                      <span class="min-w-0 flex-1 truncate font-medium">
                        Using {notice().routeLabel} after {notice().failedProviderLabel} provider
                        check failed
                        <Show when={notice().failedRouteLabel}>
                          {(failedRouteLabel) => <> for {failedRouteLabel()}</>}
                        </Show>
                      </span>
                      <button
                        type="button"
                        onClick={() => {
                          setProviderReadinessRouteNotice(null);
                          focusComposer();
                        }}
                        class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-900 dark:text-blue-200 dark:hover:bg-blue-900/50"
                        title="Dismiss provider route notice"
                        aria-label="Dismiss provider route notice"
                      >
                        <XIcon class="h-3.5 w-3.5" aria-hidden="true" />
                      </button>
                    </div>
                  )}
                </Show>
                <Show when={chat.queuedFollowUpCount() > 0}>
                  <div
                    class={`px-2.5 py-1.5 ${
                      currentStatus() ||
                      autonomousWarningVisible() ||
                      fallbackRouteNotice() ||
                      providerReadinessRouteNotice()
                        ? 'border-t border-border/70'
                        : ''
                    }`}
                    role="status"
                    aria-label="Queued follow-up messages"
                  >
                    <div class="flex min-h-7 items-center gap-2">
                      <ClockIcon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                      <span class="min-w-0 flex-1 truncate text-xs font-medium">
                        {pluralizeCount(chat.queuedFollowUpCount(), 'follow-up', 'follow-ups')}{' '}
                        {chat.queuedFollowUpsPaused() ? 'paused' : 'queued'}
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
                    <div class="mt-1 max-h-24 space-y-1 overflow-y-auto" role="list">
                      <For each={chat.queuedFollowUps()}>
                        {(queued, index) => {
                          const preview = () => queuedFollowUpPreview(queued.prompt);
                          return (
                            <div
                              class="flex min-h-7 items-center gap-2 rounded-md bg-white/70 px-2 py-1 text-xs text-blue-900 outline-none transition-colors focus:bg-white focus:ring-2 focus:ring-blue-500/40 dark:bg-blue-900/30 dark:text-blue-100 dark:focus:bg-blue-900/50"
                              role="listitem"
                              tabIndex={0}
                              aria-label={`Queued follow-up: ${preview()}. Press Enter to edit or Delete to remove.`}
                              data-testid="assistant-queued-follow-up-row"
                              onKeyDown={(event) =>
                                handleQueuedFollowUpRowKeyDown(event, queued.id)
                              }
                            >
                              <span class="min-w-0 flex-1 truncate">{preview()}</span>
                              <Show when={chat.queuedFollowUpCount() > 1 && index() > 0}>
                                <button
                                  type="button"
                                  onClick={() => sendQueuedFollowUpNext(queued.id)}
                                  class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60"
                                  title="Send queued follow-up next"
                                  aria-label={`Send queued follow-up next: ${preview()}`}
                                >
                                  <SendIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                </button>
                              </Show>
                              <Show when={chat.queuedFollowUpsPaused() && index() === 0}>
                                <button
                                  type="button"
                                  onClick={() => sendQueuedFollowUpNext(queued.id)}
                                  class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-blue-700 transition-colors hover:bg-blue-100 hover:text-blue-950 dark:text-blue-200 dark:hover:bg-blue-900/60"
                                  title="Resume queued follow-up"
                                  aria-label={`Resume queued follow-up: ${preview()}`}
                                >
                                  <SendIcon class="h-3.5 w-3.5" aria-hidden="true" />
                                </button>
                              </Show>
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
                  query={slashCommandQuery()}
                  position={{ top: 58, left: 0 }}
                  onSelect={handleSlashCommandSelect}
                  onClose={() => closeSlashCommandAutocomplete({ clearTransientDraft: true })}
                  visible={slashCommandActive()}
                />
                <div class="absolute bottom-2 right-2 flex items-center gap-1.5">
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
                  onModelSelect={selectModel}
                  onRefresh={() => loadModels(true)}
                />
                <button
                  type="button"
                  onClick={openAssistantCommandHelp}
                  class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:bg-surface-hover hover:text-base-content focus:outline-none focus:ring-2 focus:ring-blue-500/30"
                  title={AI_CHAT_COMMAND_HELP_BUTTON_LABEL}
                  aria-label={AI_CHAT_COMMAND_HELP_BUTTON_LABEL}
                  data-testid="assistant-command-help-trigger"
                >
                  <CircleHelpIcon class="h-3.5 w-3.5" aria-hidden="true" />
                </button>
                <button
                  type="button"
                  onClick={() => cycleRecentModelRoute()}
                  disabled={!nextRecentModelRoute()}
                  class="flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-surface text-muted transition-colors hover:border-border hover:text-base-content disabled:cursor-not-allowed disabled:opacity-45"
                  title={
                    nextRecentModelRouteLabel()
                      ? `${AI_CHAT_CYCLE_RECENT_MODEL_LABEL}: ${nextRecentModelRouteLabel()}`
                      : AI_CHAT_CYCLE_RECENT_MODEL_LABEL
                  }
                  aria-label={
                    nextRecentModelRouteLabel()
                      ? `${AI_CHAT_CYCLE_RECENT_MODEL_LABEL}: ${nextRecentModelRouteLabel()}`
                      : AI_CHAT_CYCLE_RECENT_MODEL_LABEL
                  }
                >
                  <RotateCwIcon class="h-3.5 w-3.5" aria-hidden="true" />
                </button>

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

              <Show when={lastAssistantUsage()}>
                {(usage) => (
                  <div
                    class="flex h-5 min-w-0 items-center justify-end self-end text-[10px] font-medium text-muted sm:h-7 sm:max-w-[12rem] sm:shrink-0 sm:self-auto"
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
