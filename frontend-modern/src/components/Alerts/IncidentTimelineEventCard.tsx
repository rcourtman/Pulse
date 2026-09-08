import { Show } from 'solid-js';
import type { IncidentEvent } from '@/types/api';
import {
  formatIncidentEvidenceTime,
  INCIDENT_EVIDENCE_DETAILS,
  INCIDENT_TIME_UNAVAILABLE,
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
        <span>
          {formatIncidentEvidenceTime(props.event.timestamp) ?? INCIDENT_TIME_UNAVAILABLE}
        </span>
      </div>
      <Show when={props.event.evidence}>
        {(evidence) => (
          <details class="text-xs text-muted mt-2">
            <summary class="cursor-pointer rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2">
              {INCIDENT_EVIDENCE_DETAILS}
            </summary>
            <dl class="mt-2 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 break-words">
              <dt>Observed</dt>
              <dd>
                {formatIncidentEvidenceTime(evidence().observedAt) ?? INCIDENT_TIME_UNAVAILABLE}
              </dd>
              <dt>Occurred</dt>
              <dd>
                {formatIncidentEvidenceTime(evidence().occurredAt) ?? INCIDENT_TIME_UNAVAILABLE}
              </dd>
              <dt>Source</dt>
              <dd>{evidence().sourceAdapter || evidence().sourceType || 'Unknown'}</dd>
              <Show when={evidence().actor}>
                <dt>Recorded actor</dt>
                <dd>{evidence().actor}</dd>
              </Show>
              <dt>Record</dt>
              <dd>{evidence().id}</dd>
            </dl>
          </details>
        )}
      </Show>
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
