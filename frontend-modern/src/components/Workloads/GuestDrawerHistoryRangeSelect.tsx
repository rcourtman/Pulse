import { For, createUniqueId, type Component } from 'solid-js';

import { type HistoryTimeRange } from '@/api/charts';
import { filterSelectClass } from '@/components/shared/FilterToolbar';
import { FormSelect } from '@/components/shared/FormSelect';
import {
  HISTORY_CHART_RANGES,
  formatHistoryRangeOptionLabel,
} from '@/components/shared/historyChartModel';

export interface GuestDrawerHistoryRangeSelectProps {
  onRangeChange: (range: HistoryTimeRange) => void;
  range: HistoryTimeRange;
  class?: string;
}

export const GuestDrawerHistoryRangeSelect: Component<GuestDrawerHistoryRangeSelectProps> = (
  props,
) => {
  // Created once: a getter prop would mint a new id on every read, splitting
  // the label from its select.
  const id = `guest-history-range-${createUniqueId()}`;
  return (
    <FormSelect
      id={id}
      label="History range"
      labelClass="sr-only"
      fieldBaseClass="contents"
      selectBaseClass={`${filterSelectClass} h-7 min-h-11 py-0 text-[11px] sm:min-h-0`}
      selectClass={props.class}
      data-testid="guest-history-range-control"
      value={props.range}
      onChange={(event) => props.onRangeChange(event.currentTarget.value as HistoryTimeRange)}
    >
      <For each={HISTORY_CHART_RANGES}>
        {(option) => <option value={option}>{formatHistoryRangeOptionLabel(option)}</option>}
      </For>
    </FormSelect>
  );
};
