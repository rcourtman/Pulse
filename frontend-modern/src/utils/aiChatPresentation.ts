import { SETTINGS_PROVIDER_MODELS_PATH } from '@/components/Settings/settingsNavigationModel';

export const AI_CHAT_DRAWER_TITLE = 'Pulse Assistant';
export const AI_CHAT_DRAWER_SUBTITLE =
  'Observed context, provider-backed reasoning, and governed actions.';
export const AI_CHAT_COLLAPSE_TITLE = 'Collapse Pulse Assistant';
export const AI_CHAT_LAUNCHER_ARIA_LABEL = 'Ask Pulse Assistant about this view';
export const AI_CHAT_SESSION_MENU_TITLE = 'Pulse Assistant sessions';
export const AI_CHAT_CLOSE_LABEL = 'Close Pulse Assistant';
export const AI_CHAT_NEW_SESSION_SHORT_LABEL = 'New';
export const AI_CHAT_NEW_SESSION_BUTTON_TITLE = 'Start new Assistant session';
export const AI_CHAT_NEW_SESSION_MENU_LABEL = 'New session';
export const AI_CHAT_NEW_SESSION_MENU_ARIA_LABEL = 'Start new Assistant session';
export const AI_CHAT_FORK_SESSION_LABEL = 'Fork current Assistant session';
export const AI_CHAT_FORK_SESSION_EMPTY_MESSAGE = 'No saved Assistant session to fork';
export const AI_CHAT_FORK_SESSION_LOADING_MESSAGE = 'Assistant is still working';
export const AI_CHAT_FORK_SESSION_LOAD_ERROR_MESSAGE =
  'Forked Assistant session but failed to load it';
export const AI_CHAT_FORK_SESSION_ERROR_MESSAGE = 'Failed to fork Assistant session';
export const AI_CHAT_FORK_SESSION_SUCCESS_MESSAGE = 'Assistant session forked';
export const AI_CHAT_UNDO_LAST_TURN_LABEL = 'Undo last Assistant turn';
export const AI_CHAT_UNDO_LAST_TURN_EMPTY_MESSAGE = 'No Assistant turn to undo';
export const AI_CHAT_UNDO_LAST_TURN_LOADING_MESSAGE = 'Assistant is still working';
export const AI_CHAT_UNDO_LAST_TURN_ERROR_MESSAGE = 'Failed to undo Assistant turn';
export const AI_CHAT_UNDO_LAST_TURN_SUCCESS_MESSAGE = 'Last prompt restored for editing';
export const AI_CHAT_REDO_LAST_TURN_LABEL = 'Redo last Assistant turn';
export const AI_CHAT_REDO_LAST_TURN_EMPTY_MESSAGE = 'No undone Assistant turn to redo';
export const AI_CHAT_REDO_LAST_TURN_LOADING_MESSAGE = 'Assistant is still working';
export const AI_CHAT_REDO_LAST_TURN_ERROR_MESSAGE = 'Failed to redo Assistant turn';
export const AI_CHAT_REDO_LAST_TURN_SUCCESS_MESSAGE = 'Assistant turn restored';
export const AI_CHAT_RENAME_SESSION_LABEL = 'Rename Assistant session';
export const AI_CHAT_RENAME_SESSION_SAVE_LABEL = 'Save Assistant session title';
export const AI_CHAT_RENAME_SESSION_CANCEL_LABEL = 'Cancel Assistant session rename';
export const AI_CHAT_RENAME_SESSION_EMPTY_MESSAGE = 'Session title cannot be empty';
export const AI_CHAT_RENAME_SESSION_ERROR_MESSAGE = 'Failed to rename Assistant session';
export const AI_CHAT_COPY_LAST_ANSWER_LABEL = 'Copy last Assistant answer';
export const AI_CHAT_COPY_LAST_ANSWER_SUCCESS_MESSAGE = 'Assistant answer copied';
export const AI_CHAT_COPY_LAST_ANSWER_ERROR_MESSAGE = 'Failed to copy Assistant answer';
export const AI_CHAT_COPY_TRANSCRIPT_LABEL = 'Copy Assistant transcript';
export const AI_CHAT_EXPORT_TRANSCRIPT_LABEL = 'Export Assistant transcript';
export const AI_CHAT_TRANSCRIPT_FALLBACK_CLOSE_LABEL = 'Close Assistant transcript copy panel';
export const AI_CHAT_TRANSCRIPT_FALLBACK_DOWNLOAD_LABEL = 'Download Assistant transcript';
export const AI_CHAT_TRANSCRIPT_FALLBACK_TEXTAREA_LABEL = 'Assistant transcript';
export const AI_CHAT_TRANSCRIPT_FALLBACK_TITLE = 'Transcript ready';
export const AI_CHAT_AUTONOMOUS_WARNING_DISMISS_LABEL = 'Dismiss chat actions warning';
export const AI_CHAT_DISCOVERY_HINT_DISMISS_LABEL = 'Dismiss discovery context warning';
export const AI_CHAT_CONTROL_MODE_LABEL = 'Assistant chat action mode';
export const AI_CHAT_CONTROL_MODE_MENU_LABEL = 'Assistant chat action options';
export const AI_CHAT_SWITCH_TO_APPROVAL_LABEL = 'Switch Assistant chat actions to Ask first';
export const AI_CHAT_COMMAND_HELP_TITLE = 'Assistant commands';
export const AI_CHAT_COMMAND_HELP_BUTTON_LABEL = 'Open Assistant commands';
export const AI_CHAT_COMMAND_HELP_CLOSE_LABEL = 'Close Assistant commands';
export const AI_CHAT_COMMAND_HELP_SEARCH_LABEL = 'Search Assistant commands';
export const AI_CHAT_COMMAND_HELP_SEARCH_PLACEHOLDER = 'Search commands...';
export const AI_CHAT_COMMAND_HELP_EMPTY_STATE = 'No commands match your search';
export const AI_CHAT_SESSION_EMPTY_STATE = 'No previous assistant sessions';
export const AI_CHAT_SESSION_LOADING_STATE = 'Loading assistant sessions...';
export const AI_CHAT_SESSION_SEARCH_PLACEHOLDER = 'Search sessions...';
export const AI_CHAT_SESSION_SEARCH_TITLE = 'Search Assistant sessions';
export const AI_CHAT_SESSION_SEARCH_EMPTY_STATE = 'No sessions match your search';
export const AI_CHAT_SESSION_SEARCH_LOADING_STATE = 'Searching assistant sessions...';
export const AI_CHAT_SESSION_SEARCH_ERROR_STATE = 'Failed to search assistant sessions';
export const AI_CHAT_MODEL_SELECTOR_EMPTY_STATE = 'No matching models.';
export const AI_CHAT_DISCOVERY_HINT_TITLE = 'Discovery is off.';
export const AI_CHAT_DISCOVERY_HINT_BODY =
  'Enable it in Settings so Pulse Assistant can reference real services, versions, and commands instead of generic guidance.';
export const AI_CHAT_INPUT_PLACEHOLDER = 'Ask about your infrastructure...';
export const AI_CHAT_QUESTION_CARD_TITLE = 'Pulse Assistant needs your input';
export const AI_CHAT_QUESTION_CARD_PLACEHOLDER = 'Type your answer...';
export const AI_CHAT_ASSISTANT_MESSAGE_LABEL = 'Pulse Assistant';
export const AI_CHAT_CONTEXT_USED_LABEL = 'Context used';
export const AI_CHAT_LAST_TURN_SUMMARY_LABEL = 'Last assistant turn summary';
export const AI_CHAT_PROVIDER_READINESS_SETTINGS_HREF = SETTINGS_PROVIDER_MODELS_PATH;
export const AI_CHAT_PROVIDER_READINESS_SETTINGS_LABEL = 'Open settings';
export const AI_CHAT_PROVIDER_READINESS_RETRY_LABEL = 'Retry';

export type AIChatProviderReadinessStatus = 'checking' | 'ready' | 'error';

export interface AIChatProviderReadinessPresentation {
  body: string;
  recommendation?: string;
  title: string;
  tone: AIChatProviderReadinessStatus;
}

export function getAIChatLauncherTitle(contextName?: unknown) {
  if (typeof contextName === 'string' && contextName.trim().length > 0) {
    return `Ask Pulse Assistant about ${contextName}`;
  }

  return 'Ask Pulse Assistant about this view';
}

export function getAIChatProviderReadinessPresentation(args: {
  message?: string;
  providerLabel?: string;
  recommendation?: string;
  routeLabel?: string;
  status: AIChatProviderReadinessStatus;
  summary?: string;
}): AIChatProviderReadinessPresentation {
  const providerLabel = args.providerLabel?.trim() || 'Selected';
  const routeLabel = args.routeLabel?.trim();
  const providerSuffix =
    providerLabel && providerLabel.toLowerCase() !== 'selected' ? ` through ${providerLabel}` : '';
  const fallbackSubject =
    providerLabel.toLowerCase() === 'selected'
      ? 'Selected model route'
      : `${providerLabel} provider route`;
  if (args.status === 'checking') {
    return {
      tone: 'checking',
      title: routeLabel ? 'Verifying selected model route' : `Verifying ${fallbackSubject}`,
      body: routeLabel
        ? `Pulse is checking ${routeLabel}${providerSuffix}.`
        : 'Pulse is checking the selected model route.',
    };
  }

  if (args.status === 'ready') {
    return {
      tone: 'ready',
      title: routeLabel ? 'Selected model route ready' : `${fallbackSubject} ready`,
      body:
        args.summary?.trim() ||
        args.message?.trim() ||
        (routeLabel
          ? `Pulse can reach ${routeLabel}${providerSuffix}.`
          : 'Pulse can reach the selected model route.'),
    };
  }

  const body =
    args.summary?.trim() ||
    args.message?.trim() ||
    'Pulse could not verify the selected model route.';
  const recommendation = args.recommendation?.trim() || undefined;

  return {
    tone: 'error',
    title: routeLabel ? 'Selected model route issue' : `${fallbackSubject} issue`,
    body,
    recommendation,
  };
}
