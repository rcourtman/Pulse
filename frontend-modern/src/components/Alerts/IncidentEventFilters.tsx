import { For, type Accessor } from 'solid-js';
import { INCIDENT_EVENT_TYPES } from '@/features/alerts/types';
import {
  getAlertTimelineEventTypeLabel,
  getAlertTimelineFilterLabel,
  getAlertTimelineQuickFilterLabel,
} from '@/utils/alertOverviewPresentation';
import {
  getAlertIncidentEventFilterActionButtonClass,
  getAlertIncidentEventFilterChipClass,
  getAlertIncidentEventFilterContainerClass,
  getAlertIncidentEventFilterLabelClass,
  type AlertIncidentEventFilterVariant,
} from '@/utils/alertIncidentPresentation';

export interface IncidentEventFiltersProps {
  filters: Accessor<Set<string>>;
  setFilters: (next: Set<string>) => void;
  variant: AlertIncidentEventFilterVariant;
  showQuickSelection?: boolean;
}

export function IncidentEventFilters(props: IncidentEventFiltersProps) {
  const toggleFilter = (type: (typeof INCIDENT_EVENT_TYPES)[number]) => {
    const next = new Set(props.filters());
    if (next.has(type)) {
      next.delete(type);
    } else {
      next.add(type);
    }
    props.setFilters(next);
  };

  const quickSelectionActions = [
    { id: 'all', label: () => getAlertTimelineQuickFilterLabel('all') },
    { id: 'none', label: () => getAlertTimelineQuickFilterLabel('none') },
  ] as const;

  return (
    <div class={getAlertIncidentEventFilterContainerClass(props.variant)}>
      <span class={getAlertIncidentEventFilterLabelClass(props.variant)}>
        {getAlertTimelineFilterLabel(props.variant)}
      </span>
      <For each={props.showQuickSelection ? quickSelectionActions : []}>
        {(action) => (
          <button
            type="button"
            class={getAlertIncidentEventFilterActionButtonClass()}
            onClick={() =>
              props.setFilters(action.id === 'all' ? new Set(INCIDENT_EVENT_TYPES) : new Set())
            }
          >
            {action.label()}
          </button>
        )}
      </For>
      <For each={INCIDENT_EVENT_TYPES}>
        {(type) => {
          const selected = () => props.filters().has(type);
          return (
            <button
              type="button"
              class={getAlertIncidentEventFilterChipClass(selected(), props.variant)}
              onClick={() => toggleFilter(type)}
            >
              {getAlertTimelineEventTypeLabel(type)}
            </button>
          );
        }}
      </For>
    </div>
  );
}
