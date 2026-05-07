import SparklesIcon from 'lucide-solid/icons/sparkles';
import { aiChatStore } from '@/stores/aiChat';
import type { Incident } from '@/types/api';
import { buildAlertIncidentAssistantHandoff } from './incidentAssistantHandoffModel';

interface IncidentAssistantHandoffButtonProps {
  incident: Incident;
  label?: string;
  class?: string;
}

export function IncidentAssistantHandoffButton(props: IncidentAssistantHandoffButtonProps) {
  if (aiChatStore.enabled !== true) {
    return null;
  }

  const handleClick = (event: MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();

    const handoff = buildAlertIncidentAssistantHandoff({ incident: props.incident });
    aiChatStore.openWithPrompt(handoff.prompt, handoff.context);
  };

  return (
    <button
      type="button"
      class={
        props.class ||
        'inline-flex shrink-0 items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs font-medium text-blue-600 transition-colors hover:bg-surface-hover hover:text-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:text-blue-400 dark:hover:text-blue-300'
      }
      title="Discuss this incident with Pulse Assistant"
      aria-label={`Discuss incident ${props.incident.id} with Pulse Assistant`}
      onClick={handleClick}
    >
      <SparklesIcon class="h-3.5 w-3.5" aria-hidden="true" />
      <span>{props.label || 'Discuss with Assistant'}</span>
    </button>
  );
}
