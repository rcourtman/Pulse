import { Card } from '@/components/shared/Card';
import Toggle from '@/components/shared/Toggle';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';

export function ThresholdsTableDockerServiceGapSection(props: ThresholdsTableSectionProps) {
  const { state, tableProps } = props;

  return (
    <Card padding="md" tone="card" class="mb-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h3 class="text-sm font-semibold text-base-content">
            {state.dockerServicePresentation.title}
          </h3>
          <p class="mt-1 text-xs text-muted">{state.dockerServicePresentation.description}</p>
        </div>
        <Toggle
          checked={!tableProps.disableAllDockerServices()}
          onToggle={() => {
            tableProps.setDisableAllDockerServices(!tableProps.disableAllDockerServices());
            tableProps.setHasUnsavedChanges(true);
          }}
          label={
            <span class="text-sm font-medium text-base-content">
              {state.dockerServicePresentation.toggleLabel}
            </span>
          }
          description={
            <span class="text-xs text-muted">
              {state.dockerServicePresentation.toggleDescription}
            </span>
          }
          size="sm"
        />
      </div>

      <div class="mt-4 grid gap-4 sm:grid-cols-2">
        <div>
          <label
            for={state.serviceWarnInputId}
            class="text-xs font-medium uppercase tracking-wide text-muted"
          >
            {state.dockerServicePresentation.warningGapLabel}
          </label>
          <input
            type="number"
            min="0"
            max="100"
            id={state.serviceWarnInputId}
            value={tableProps.dockerDefaults.serviceWarnGapPercent}
            onInput={(event) => {
              const value = Number(event.currentTarget.value);
              const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
              tableProps.setDockerDefaults((prev) => ({
                ...prev,
                serviceWarnGapPercent: normalized,
              }));
              tableProps.setHasUnsavedChanges(true);
            }}
            class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
          />
          <p class="mt-1 text-xs text-muted">{state.dockerServicePresentation.warningGapDescription}</p>
        </div>
        <div>
          <label
            for={state.serviceCriticalInputId}
            class="text-xs font-medium uppercase tracking-wide text-muted"
          >
            {state.dockerServicePresentation.criticalGapLabel}
          </label>
          <input
            type="number"
            min="0"
            max="100"
            id={state.serviceCriticalInputId}
            value={tableProps.dockerDefaults.serviceCriticalGapPercent}
            onInput={(event) => {
              const value = Number(event.currentTarget.value);
              const normalized = Number.isFinite(value) ? Math.max(0, Math.min(100, value)) : 0;
              tableProps.setDockerDefaults((prev) => ({
                ...prev,
                serviceCriticalGapPercent: normalized,
              }));
              tableProps.setHasUnsavedChanges(true);
            }}
            class="mt-1 w-full rounded-md border border-border bg-surface p-2 text-sm text-base-content focus:border-sky-500 focus:outline-none focus:ring-2 focus:ring-sky-200 dark:focus:border-sky-400 dark:focus:ring-sky-600"
          />
          <p class="mt-1 text-xs text-muted">
            {state.dockerServicePresentation.criticalGapDescription}
          </p>
        </div>
      </div>
      {state.serviceGapValidationMessage() && (
        <p class="mt-1.5 text-xs font-medium text-red-600 dark:text-red-400">
          {state.serviceGapValidationMessage()}
        </p>
      )}
    </Card>
  );
}
