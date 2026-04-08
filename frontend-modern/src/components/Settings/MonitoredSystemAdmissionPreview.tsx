import { For, Show, type Component } from 'solid-js';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { formatMonitoredSystemSurfaceAttribution } from '@/utils/monitoredSystemPresentation';

interface MonitoredSystemAdmissionPreviewProps {
  preview: MonitoredSystemLedgerPreviewResponse | null;
  loading?: boolean;
  error?: string | null;
  errorTitle?: string | null;
}

const formatUsage = (count: number, limit: number): string =>
  limit > 0 ? `${count} / ${limit}` : `${count}`;

const formatDelta = (count: number): string => {
  if (count > 0) return `+${count}`;
  return `${count}`;
};

const previewTone = (preview: MonitoredSystemLedgerPreviewResponse | null) =>
  preview?.would_exceed_limit ? 'warning' : 'info';

const previewTitle = (preview: MonitoredSystemLedgerPreviewResponse | null) => {
  if (!preview) return 'Monitored-system impact';
  if (preview.would_exceed_limit) {
    return 'This change exceeds your monitored-system limit';
  }
  if (preview.additional_count > 0) {
    return 'This change adds monitored systems';
  }
  return 'This change reuses your current monitored-system capacity';
};

const previewSummary = (preview: MonitoredSystemLedgerPreviewResponse): string => {
  const before = formatUsage(preview.current_count, preview.limit);
  const after = formatUsage(preview.projected_count, preview.limit);
  if (preview.additional_count > 0) {
    return `Current usage ${before}. Saving this change would move usage to ${after} (${formatDelta(
      preview.additional_count,
    )}).`;
  }
  return `Current usage ${before}. Saving this change keeps usage at ${after}.`;
};

export const MonitoredSystemAdmissionPreview: Component<MonitoredSystemAdmissionPreviewProps> = (
  props,
) => {
  return (
    <>
      <Show when={props.loading}>
        <div class="rounded-md border border-border bg-surface-alt px-4 py-3 text-sm text-muted">
          Calculating monitored-system impact…
        </div>
      </Show>

      <Show when={!props.loading && props.error}>
        {(error) => (
          <CalloutCard
            tone="warning"
            title={props.errorTitle || 'Could not preview monitored-system impact'}
            description={<p>{error()}</p>}
          />
        )}
      </Show>

      <Show when={!props.loading && props.preview}>
        {(preview) => (
          <CalloutCard
            tone={previewTone(preview())}
            title={previewTitle(preview())}
            description={
              <div class="space-y-3 text-sm">
                <p>{previewSummary(preview())}</p>
                <Show when={preview().current_systems.length > 0}>
                  <div class="space-y-1">
                    <p class="text-xs font-medium uppercase tracking-wide text-muted">
                      Current matched systems
                    </p>
                    <ul class="space-y-1 text-sm text-base-content">
                      <For each={preview().current_systems}>
                        {(system) => <li>{formatMonitoredSystemSurfaceAttribution(system)}</li>}
                      </For>
                    </ul>
                  </div>
                </Show>
                <Show when={preview().projected_systems.length > 0}>
                  <div class="space-y-1">
                    <p class="text-xs font-medium uppercase tracking-wide text-muted">
                      Projected systems
                    </p>
                    <ul class="space-y-1 text-sm text-base-content">
                      <For each={preview().projected_systems}>
                        {(system) => <li>{formatMonitoredSystemSurfaceAttribution(system)}</li>}
                      </For>
                    </ul>
                  </div>
                </Show>
              </div>
            }
          />
        )}
      </Show>
    </>
  );
};

export default MonitoredSystemAdmissionPreview;
