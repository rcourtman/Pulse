export const AI_CHAT_DRAWER_TITLE = 'Pulse Assistant';
export const AI_CHAT_DRAWER_SUBTITLE =
  'Observed context, provider-backed reasoning, and governed actions.';
export const AI_CHAT_COLLAPSE_TITLE = 'Collapse Pulse Assistant';
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
  'Pulse Assistant uses observed infrastructure context and your configured provider to inspect state, explain findings, and suggest safe next steps.';
export const AI_CHAT_INPUT_PLACEHOLDER = 'Ask about your infrastructure...';
export const AI_CHAT_SUGGESTIONS_LABEL = 'Try asking';
export const AI_CHAT_QUESTION_CARD_TITLE = 'Pulse Assistant needs your input';
export const AI_CHAT_QUESTION_CARD_PLACEHOLDER = 'Type your answer...';
export const AI_CHAT_ASSISTANT_MESSAGE_LABEL = 'Pulse Assistant';
export const AI_CHAT_CONTEXT_USED_LABEL = 'Context used';

const AI_CHAT_CLUSTER_EMPTY_STATE_SUGGESTIONS = [
  'Summarize cluster health',
  'Find failed services',
  'Check node load and pressure',
];

const AI_CHAT_SINGLE_SYSTEM_EMPTY_STATE_SUGGESTIONS = [
  'Summarize system health',
  'Check storage pressure',
  'Explain recent Patrol findings',
];

export function getAIChatEmptyStateSuggestions(isCluster: boolean) {
  return isCluster
    ? AI_CHAT_CLUSTER_EMPTY_STATE_SUGGESTIONS
    : AI_CHAT_SINGLE_SYSTEM_EMPTY_STATE_SUGGESTIONS;
}
