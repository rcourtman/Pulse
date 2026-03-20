import { For, type Accessor } from 'solid-js';
import { INCIDENT_EVENT_LABELS, INCIDENT_EVENT_TYPES } from '@/features/alerts/types';
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

  const label = () => (props.variant === 'panel' ? 'Filter events:' : 'Filters');

  return (
    <div class={getAlertIncidentEventFilterContainerClass(props.variant)}>
      <span class={getAlertIncidentEventFilterLabelClass(props.variant)}>{label()}</span>
      <For each={props.showQuickSelection ? ['All', 'None'] : []}>
        {(action) => (
          <button
            type="button"
            class={getAlertIncidentEventFilterActionButtonClass()}
            onClick={() =>
              props.setFilters(action === 'All' ? new Set(INCIDENT_EVENT_TYPES) : new Set())
            }
          >
            {action}
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
              {INCIDENT_EVENT_LABELS[type]}
            </button>
          );
        }}
      </For>
    </div>
  );
}
