import { stripAssistantOutputArtifacts } from './assistantOutputHygiene';
import type { ChatMessage } from './types';

const normalizeAnswerText = (text?: string): string =>
  stripAssistantOutputArtifacts(text || '').text.trim();

export const getAssistantAnswerText = (message: ChatMessage): string => {
  if (message.role !== 'assistant') return '';

  const messageContent = normalizeAnswerText(message.content);
  if (messageContent) return messageContent;

  return (message.streamEvents || [])
    .filter((event) => event.type === 'content')
    .map((event) => normalizeAnswerText(event.content))
    .filter(Boolean)
    .join('\n\n')
    .trim();
};

export const getLastAssistantAnswerText = (messages: ChatMessage[]): string => {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (message?.role !== 'assistant') continue;
    return getAssistantAnswerText(message);
  }

  return '';
};
