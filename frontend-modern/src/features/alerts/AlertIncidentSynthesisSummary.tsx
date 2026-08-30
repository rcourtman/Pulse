import { For, Show } from 'solid-js';

import type { Alert } from '@/types/api';
import {
  getAlertFailureClassLabel,
  getAlertIncidentSynthesisPresentation,
} from './incidentSynthesisPresentation';

interface AlertIncidentSynthesisSummaryProps {
  alert: Alert;
}

export function AlertIncidentSynthesisSummary(props: AlertIncidentSynthesisSummaryProps) {
  const correlation = () => props.alert.correlation;
  const presentation = () =>
    correlation() ? getAlertIncidentSynthesisPresentation(correlation()!) : null;

  return (
    <Show when={correlation()?.kind === 'infrastructure-incident' && presentation()}>
      {(value) => (
        <section class={value().panelClass} data-testid="incident-synthesis-summary">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold text-base-content">{value().title}</h3>
            <span class={value().badgeClass}>{value().badge}</span>
          </div>
          <p class="mt-1 text-sm text-base-content/80">{correlation()!.reason}</p>
          <p class="mt-1 text-xs text-muted">{value().counts}</p>
          <details class="mt-2">
            <summary class="cursor-pointer text-xs font-medium text-blue-700 dark:text-blue-300">
              {value().review}
            </summary>
            <div class="mt-2 space-y-2 border-l-2 border-border pl-3">
              <For each={correlation()!.observations ?? []}>
                {(observation) => (
                  <div class="text-xs text-base-content/80">
                    <div class="font-medium text-base-content">
                      {observation.resourceName || observation.resourceId}
                    </div>
                    <div>
                      {getAlertFailureClassLabel(observation.failureClass)} ·{' '}
                      {new Date(observation.observedAt).toLocaleString()}
                    </div>
                    <Show when={(observation.evidenceIds?.length ?? 0) > 0}>
                      <div class="font-mono text-[11px] text-muted break-all">
                        {observation.evidenceIds!.join(', ')}
                      </div>
                    </Show>
                  </div>
                )}
              </For>
              <p class="text-xs text-muted">{value().challenge}</p>
            </div>
          </details>
        </section>
      )}
    </Show>
  );
}
