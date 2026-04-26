import { For, Show, type Component } from 'solid-js';
import type { MonitoredSystemLedgerPreviewResponse } from '@/api/monitoredSystemLedger';
import { CalloutCard } from '@/components/shared/CalloutCard';
import {
  formatMonitoredSystemAdmissionPreviewSummary,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemAdmissionPreviewTitle,
  getMonitoredSystemAdmissionPreviewRequiredState,
} from '@/utils/monitoredSystemPresentation';

interface MonitoredSystemAdmissionPreviewProps {
  preview: MonitoredSystemLedgerPreviewResponse | null;
  loading?: boolean;
  error?: string | null;
  errorTitle?: string | null;
}

const previewTone = (preview: MonitoredSystemLedgerPreviewResponse | null) =>
  preview?.would_exceed_limit ? 'warning' : 'info';

export const MonitoredSystemAdmissionPreview: Component<MonitoredSystemAdmissionPreviewProps> = (
  props,
) => {
  const requiredState = getMonitoredSystemAdmissionPreviewRequiredState();

  return (
    <>
      <Show when={!props.loading && !props.error && !props.preview}>
        <CalloutCard
          tone="info"
          title={requiredState.title}
          description={<p>{requiredState.message}</p>}
        />
      </Show>

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
            title={getMonitoredSystemAdmissionPreviewTitle(preview())}
            description={
              <div class="space-y-3 text-sm">
                <p>{formatMonitoredSystemAdmissionPreviewSummary(preview())}</p>
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
