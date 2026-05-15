export const AI_CHAT_DRAWER_TITLE = 'Pulse Assistant';
export const AI_CHAT_DRAWER_SUBTITLE =
  'Observed context, provider-backed reasoning, and governed actions.';
export const AI_CHAT_COLLAPSE_TITLE = 'Collapse Pulse Assistant';
export const AI_CHAT_LAUNCHER_ARIA_LABEL = 'Expand Pulse Assistant';
export const AI_CHAT_SESSION_MENU_TITLE = 'Pulse Assistant sessions';
export const AI_CHAT_NEW_SESSION_SHORT_LABEL = 'New';
export const AI_CHAT_NEW_SESSION_BUTTON_TITLE = 'New assistant session';
export const AI_CHAT_NEW_SESSION_MENU_LABEL = 'New session';
export const AI_CHAT_SESSION_EMPTY_STATE = 'No previous assistant sessions';
export const AI_CHAT_MODEL_SELECTOR_EMPTY_STATE = 'No matching models.';
export const AI_CHAT_DISCOVERY_HINT_TITLE = 'Workload Discovery is off.';
export const AI_CHAT_DISCOVERY_HINT_BODY =
  'Enable it in Settings so Pulse Assistant can reference real services, versions, and commands instead of generic guidance.';
export const AI_CHAT_EMPTY_STATE_TITLE = 'Ask about your infrastructure';
export const AI_CHAT_EMPTY_STATE_SUBTITLE =
  'Chat with your configured model using Pulse context and governed tools.';
export const AI_CHAT_INPUT_PLACEHOLDER = 'Ask about your infrastructure...';
export const AI_CHAT_SUGGESTIONS_LABEL = 'Try asking';
export const AI_CHAT_QUESTION_CARD_TITLE = 'Pulse Assistant needs your input';
export const AI_CHAT_QUESTION_CARD_PLACEHOLDER = 'Type your answer...';
export const AI_CHAT_ASSISTANT_MESSAGE_LABEL = 'Pulse Assistant';
export const AI_CHAT_CONTEXT_USED_LABEL = 'Context used';

export interface AIChatEmptyStateBriefingInput {
  sourceLabel?: string;
  subject?: string;
  suggestedPrompts?: string[];
  title?: string;
}

export interface AIChatEmptyStatePresentation {
  suggestions: string[];
  subtitle?: string;
  title: string;
}

export function getAIChatLauncherTitle(contextName?: unknown) {
  if (typeof contextName === 'string' && contextName.trim().length > 0) {
    return `Open Pulse Assistant for ${contextName}`;
  }

  return 'Open Pulse Assistant';
}

export function getAIChatEmptyStateSuggestions(isCluster: boolean) {
  void isCluster;
  return [];
}

export function getAIChatEmptyStatePresentation(args: {
  briefing?: AIChatEmptyStateBriefingInput;
  isCluster: boolean;
}): AIChatEmptyStatePresentation {
  const sourceLabel = args.briefing?.sourceLabel?.trim();
  const title = args.briefing?.title?.trim();
  const subject = args.briefing?.subject?.trim();
  const hasSuggestedPrompts = (args.briefing?.suggestedPrompts ?? []).some(
    (prompt) => prompt.trim().length > 0,
  );

  if (args.briefing && (sourceLabel || title || subject || hasSuggestedPrompts)) {
    return {
      title: sourceLabel ? `Review ${sourceLabel} context` : 'Review attached context',
      subtitle: [title, subject].filter(Boolean).join(' · ') || undefined,
      suggestions: [],
    };
  }

  return {
    title: AI_CHAT_EMPTY_STATE_TITLE,
    subtitle: AI_CHAT_EMPTY_STATE_SUBTITLE,
    suggestions: getAIChatEmptyStateSuggestions(args.isCluster),
  };
}
