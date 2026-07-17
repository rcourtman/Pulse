import { Show, createSignal } from 'solid-js';

import { Card } from '@/components/shared/Card';
import Toggle from '@/components/shared/Toggle';
import { FACTORY_DOCKER_DEFAULTS } from '@/utils/alertThresholdDefaults';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerUpdateAlertsSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  const updateAlertsEnabled = () => tableProps.dockerDefaults.updateAlertDelayHours > 0;
  // Remember the last positive delay so toggling off and back on restores it.
  const [lastDelayHours, setLastDelayHours] = createSignal(
    updateAlertsEnabled()
      ? tableProps.dockerDefaults.updateAlertDelayHours
      : FACTORY_DOCKER_DEFAULTS.updateAlertDelayHours,
  );

  return (
    <Card padding="md" tone="card" class="mb-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-sm font-semibold text-base-content">
            {state.dockerUpdatePresentation.title}
          </h3>
          <p class="mt-1 text-xs text-muted">{state.dockerUpdatePresentation.description}</p>
        </div>
        <Toggle
          checked={updateAlertsEnabled()}
          onToggle={() => {
            if (updateAlertsEnabled()) {
              setLastDelayHours(tableProps.dockerDefaults.updateAlertDelayHours);
              tableProps.setDockerDefaults((prev) => ({ ...prev, updateAlertDelayHours: -1 }));
            } else {
              tableProps.setDockerDefaults((prev) => ({
                ...prev,
                updateAlertDelayHours: lastDelayHours(),
              }));
            }
            tableProps.setHasUnsavedChanges(true);
          }}
          label={
            <span class="text-sm font-medium text-base-content">
              {state.dockerUpdatePresentation.toggleLabel}
            </span>
          }
          description={
            <span class="text-xs text-muted">
              {state.dockerUpdatePresentation.toggleDescription}
            </span>
          }
          size="sm"
        />
      </div>

      <Show when={updateAlertsEnabled()}>
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <label
              for={state.dockerUpdateDelayInputId}
              class="text-xs font-medium uppercase tracking-wide text-muted"
            >
              {state.dockerUpdatePresentation.delayLabel}
            </label>
            <input
              type="number"
              min="1"
              step="1"
              id={state.dockerUpdateDelayInputId}
              value={tableProps.dockerDefaults.updateAlertDelayHours}
              onInput={(event) => {
                const value = Number(event.currentTarget.value);
                const normalized = Number.isFinite(value)
                  ? Math.max(1, Math.round(value))
                  : FACTORY_DOCKER_DEFAULTS.updateAlertDelayHours;
                setLastDelayHours(normalized);
                tableProps.setDockerDefaults((prev) => ({
                  ...prev,
                  updateAlertDelayHours: normalized,
                }));
                tableProps.setHasUnsavedChanges(true);
              }}
              class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
            />
            <p class="mt-1 text-xs text-muted">{state.dockerUpdatePresentation.delayDescription}</p>
          </div>
        </div>
      </Show>
    </Card>
  );
}
