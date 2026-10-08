import { For } from 'solid-js';

import type { AlertResourceRelaxedTriggerSummary } from './alertResourceTableModel';

/**
 * One line under a pulse-relaxed guest's threshold row naming the tag and the
 * thresholds it raises. The tag and each threshold never wrap mid-token, so a
 * narrow card does not split "pulse-relaxed" at its hyphen.
 */
export function AlertResourceRelaxedTriggers(props: {
  summary: AlertResourceRelaxedTriggerSummary;
  class: string;
}) {
  return (
    <p
      class={props.class}
      title={props.summary.title}
      data-testid="alert-resource-relaxed-triggers"
    >
      Proxmox tag <span class="whitespace-nowrap">pulse-relaxed</span>: alerts at{' '}
      <For each={props.summary.raised}>
        {(threshold, index) => (
          <>
            {index() > 0 ? ' · ' : ''}
            <span class="whitespace-nowrap">{threshold}</span>
          </>
        )}
      </For>
    </p>
  );
}
