import { Show, For, createMemo, type Accessor } from 'solid-js';
import type { Incident } from '@/types/api';
import { filterIncidentEvents } from '@/features/alerts/types';
import { IncidentEventFilters } from '@/components/Alerts/IncidentEventFilters';
import { IncidentTimelineEventCard } from '@/components/Alerts/IncidentTimelineEventCard';
import {
  getAlertTimelineEmptyState,
  getAlertTimelineFailureState,
  getAlertTimelineFilterEmptyState,
  getAlertTimelineLoadingState,
  getAlertTimelineUnavailableState,
} from '@/utils/alertOverviewPresentation';
import {
  type AlertIncidentEventFilterVariant,
  getAlertIncidentAcknowledgedBadgeClass,
  getAlertIncidentNoteSaveButtonClass,
  getAlertIncidentNoteTextareaClass,
  getAlertIncidentTimelineHeadingClass,
  getAlertIncidentTimelineMetaRowClass,
  getAlertResourceIncidentNotePlaceholder,
  getAlertResourceIncidentSaveNoteLabel,
} from '@/utils/alertIncidentPresentation';

export interface IncidentTimelinePanelProps {
  timeline?: Incident | null;
  loading: boolean;
  error: boolean;
  filters: Accessor<Set<string>>;
  setFilters: (next: Set<string>) => void;
  filterVariant: AlertIncidentEventFilterVariant;
  eventCardVariant: 'surface' | 'alt';
  noteDraft: string;
  onNoteDraftChange: (value: string) => void;
  noteSaving: boolean;
  onSaveNote: () => void;
  onRetry: () => void;
}

export function IncidentTimelinePanel(props: IncidentTimelinePanelProps) {
  const timeline = () => props.timeline;
  const events = createMemo(() => timeline()?.events || []);
  const filteredEvents = createMemo(() => filterIncidentEvents(events(), props.filters()));

  return (
    <>
      <Show when={props.loading}>
        <p class="text-xs text-muted">{getAlertTimelineLoadingState().text}</p>
      </Show>
      <Show when={!props.loading && timeline()}>
        {(loadedTimeline) => (
          <div class="space-y-3">
            <div class={getAlertIncidentTimelineMetaRowClass()}>
              <span class={getAlertIncidentTimelineHeadingClass()}>Incident</span>
              <span>{loadedTimeline().status}</span>
              <Show when={loadedTimeline().acknowledged}>
                <span class={getAlertIncidentAcknowledgedBadgeClass()}>acknowledged</span>
              </Show>
              <Show when={loadedTimeline().openedAt}>
                <span>opened {new Date(loadedTimeline().openedAt).toLocaleString()}</span>
              </Show>
              <Show when={loadedTimeline().closedAt}>
                <span>closed {new Date(loadedTimeline().closedAt as string).toLocaleString()}</span>
              </Show>
            </div>
            <Show when={events().length > 0}>
              <IncidentEventFilters
                filters={props.filters}
                setFilters={props.setFilters}
                variant={props.filterVariant}
                showQuickSelection={props.filterVariant === 'compact'}
              />
            </Show>
            <Show when={filteredEvents().length > 0}>
              <div class="space-y-2">
                <For each={filteredEvents()}>
                  {(event) => (
                    <IncidentTimelineEventCard event={event} variant={props.eventCardVariant} />
                  )}
                </For>
              </div>
            </Show>
            <Show when={events().length > 0 && filteredEvents().length === 0}>
              <p class="text-xs text-muted">{getAlertTimelineFilterEmptyState().text}</p>
            </Show>
            <Show when={events().length === 0}>
              <p class="text-xs text-muted">{getAlertTimelineEmptyState().text}</p>
            </Show>
            <div class="flex flex-col gap-2">
              <textarea
                class={getAlertIncidentNoteTextareaClass()}
                rows={2}
                placeholder={getAlertResourceIncidentNotePlaceholder()}
                value={props.noteDraft}
                onInput={(event) => props.onNoteDraftChange(event.currentTarget.value)}
              />
              <div class="flex justify-end">
                <button
                  class={getAlertIncidentNoteSaveButtonClass()}
                  disabled={props.noteSaving || !props.noteDraft.trim()}
                  onClick={() => props.onSaveNote()}
                >
                  {getAlertResourceIncidentSaveNoteLabel(props.noteSaving)}
                </button>
              </div>
            </div>
          </div>
        )}
      </Show>
      <Show when={!props.loading && !timeline()}>
        <Show
          when={props.error}
          fallback={<p class="text-xs text-muted">{getAlertTimelineUnavailableState().text}</p>}
        >
          <div class="flex items-center gap-2">
            <p class="text-xs text-error">{getAlertTimelineFailureState().text}</p>
            <button class="text-xs text-primary hover:underline" onClick={() => props.onRetry()}>
              {getAlertTimelineFailureState().actionLabel}
            </button>
          </div>
        </Show>
      </Show>
    </>
  );
}
