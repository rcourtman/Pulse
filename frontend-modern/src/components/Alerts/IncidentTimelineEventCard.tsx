import { Show } from 'solid-js';
import type { IncidentEvent } from '@/types/api';
import {
  getAlertIncidentTimelineCommandClass,
  getAlertIncidentTimelineDetailClass,
  getAlertIncidentTimelineEventCardClass,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertIncidentTimelineOutputClass,
} from '@/utils/alertIncidentPresentation';

export interface IncidentTimelineEventCardProps {
  event: IncidentEvent;
  variant: 'surface' | 'alt';
}

function getEventDetail(event: IncidentEvent, key: 'note' | 'command' | 'output_excerpt') {
  const value = event.details?.[key];
  if (typeof value !== 'string') {
    return null;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : null;
}

export function IncidentTimelineEventCard(props: IncidentTimelineEventCardProps) {
  const note = () => getEventDetail(props.event, 'note');
  const command = () => getEventDetail(props.event, 'command');
  const outputExcerpt = () => getEventDetail(props.event, 'output_excerpt');

  return (
    <div class={getAlertIncidentTimelineEventCardClass(props.variant)}>
      <div class={getAlertIncidentTimelineMetaRowClass()}>
        <span class={getAlertIncidentTimelineHeadingClass()}>{props.event.summary}</span>
        <span>{new Date(props.event.timestamp).toLocaleString()}</span>
      </div>
      <Show when={note()}>
        <p class={getAlertIncidentTimelineDetailClass()}>{note()}</p>
      </Show>
      <Show when={command()}>
        <p class={getAlertIncidentTimelineCommandClass()}>{command()}</p>
      </Show>
      <Show when={outputExcerpt()}>
        <p class={getAlertIncidentTimelineOutputClass()}>{outputExcerpt()}</p>
      </Show>
    </div>
  );
}
