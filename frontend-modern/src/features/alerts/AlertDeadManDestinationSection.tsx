import { createSignal, createUniqueId, onMount, Show, type Accessor } from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import type { DeadManStatus } from '@/types/alerts';
import { formatRelativeTime } from '@/utils/format';
import { logger } from '@/utils/logger';

interface AlertDeadManDestinationSectionProps {
  pingUrl: Accessor<string>;
  setPingUrl: (value: string) => void;
  setHasUnsavedChanges: (value: boolean) => void;
}

const REDACTED_PING_URL = '***REDACTED***';

const statusPresentation: Record<DeadManStatus['state'], { label: string; class: string }> = {
  disabled: { label: 'Not configured', class: 'bg-base text-muted' },
  starting: {
    label: 'Starting',
    class: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100',
  },
  healthy: {
    label: 'Heartbeat healthy',
    class: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100',
  },
  delivery_failed: {
    label: 'Delivery failing',
    class: 'bg-amber-100 text-amber-900 dark:bg-amber-900 dark:text-amber-100',
  },
  monitor_stalled: {
    label: 'Monitoring stalled',
    class: 'bg-red-100 text-red-900 dark:bg-red-900 dark:text-red-100',
  },
  misconfigured: {
    label: 'Configuration invalid',
    class: 'bg-red-100 text-red-900 dark:bg-red-900 dark:text-red-100',
  },
  configuration_unavailable: {
    label: 'Configuration unavailable',
    class: 'bg-red-100 text-red-900 dark:bg-red-900 dark:text-red-100',
  },
};

export function AlertDeadManDestinationSection(props: AlertDeadManDestinationSectionProps) {
  const inputId = `alert-deadman-url-${createUniqueId()}`;
  const titleId = `${inputId}-title`;
  const [status, setStatus] = createSignal<DeadManStatus | null>(null);
  const [loading, setLoading] = createSignal(false);
  const [unavailable, setUnavailable] = createSignal(false);
  const [showUrl, setShowUrl] = createSignal(false);

  const loadStatus = async () => {
    if (loading()) return;
    setLoading(true);
    try {
      setStatus(await AlertsAPI.getDeadManStatus());
      setUnavailable(false);
    } catch (error) {
      logger.error('Failed to load external watchdog status', error);
      setUnavailable(true);
    } finally {
      setLoading(false);
    }
  };

  onMount(() => void loadStatus());

  const presentation = () => {
    const current = status();
    return current ? statusPresentation[current.state] : statusPresentation.disabled;
  };
  const hasStoredUrl = () => props.pingUrl() === REDACTED_PING_URL;
  const inputValue = () => (hasStoredUrl() ? '' : props.pingUrl());

  return (
    <SettingsPanel
      titleId={titleId}
      title="External watchdog"
      description="Detect when Pulse itself stops monitoring by pinging a watchdog on a different host."
      action={
        <div class="flex items-center gap-2">
          <span class={`rounded-full px-2.5 py-1 text-xs font-semibold ${presentation().class}`}>
            {unavailable() ? 'Status unavailable' : presentation().label}
          </span>
          <button
            type="button"
            class="rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content hover:bg-surface-hover disabled:opacity-50"
            disabled={loading()}
            onClick={() => void loadStatus()}
          >
            {loading() ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      }
      class="min-w-0"
    >
      <div class="space-y-4">
        <div>
          <label for={inputId} class="mb-1.5 block text-sm font-medium text-base-content">
            Healthchecks-compatible success ping URL
          </label>
          <div class="flex min-w-0 gap-2">
            <input
              id={inputId}
              type={showUrl() ? 'text' : 'password'}
              value={inputValue()}
              maxLength={2048}
              spellcheck={false}
              autocomplete="off"
              placeholder={
                hasStoredUrl() ? 'Configured — enter a new URL to replace' : 'https://hc-ping.com/…'
              }
              aria-describedby={`${inputId}-help`}
              class="min-w-0 flex-1 rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm text-base-content shadow-sm focus:border-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500/20"
              onInput={(event) => {
                props.setPingUrl(event.currentTarget.value);
                props.setHasUnsavedChanges(true);
              }}
            />
            <Show
              when={!hasStoredUrl()}
              fallback={
                <button
                  type="button"
                  class="rounded-md border border-red-300 px-3 py-2 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-950"
                  onClick={() => {
                    props.setPingUrl('');
                    props.setHasUnsavedChanges(true);
                  }}
                >
                  Remove
                </button>
              }
            >
              <button
                type="button"
                class="rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content hover:bg-surface-hover"
                aria-pressed={showUrl()}
                onClick={() => setShowUrl((value) => !value)}
              >
                {showUrl() ? 'Hide' : 'Show'}
              </button>
            </Show>
          </div>
          <p id={`${inputId}-help`} class="mt-2 text-xs leading-5 text-muted">
            Pulse sends a success signal every minute and a <code>/fail</code> signal if its
            monitoring loop stalls. Configure a three-minute grace period at the watchdog. Use a
            service on another machine or network. A watchdog on this Pulse host cannot detect host
            failure. Clearing this field does not pause the remote check.
          </p>
        </div>

        <Show when={status()}>
          {(current) => (
            <div class="grid gap-3 rounded-md border border-border bg-base p-3 text-xs sm:grid-cols-2 lg:grid-cols-4">
              <div>
                <div class="font-medium text-muted">Last success</div>
                <div class="mt-1 text-base-content">
                  {current().lastSuccessAt
                    ? formatRelativeTime(current().lastSuccessAt, { emptyText: 'Never' })
                    : 'Never'}
                </div>
              </div>
              <div>
                <div class="font-medium text-muted">Monitor progress</div>
                <div class="mt-1 text-base-content">
                  {current().lastMonitoringProgress
                    ? formatRelativeTime(current().lastMonitoringProgress, { emptyText: 'Waiting' })
                    : 'Waiting'}
                </div>
              </div>
              <div>
                <div class="font-medium text-muted">Consecutive failures</div>
                <div class="mt-1 text-base-content">{current().consecutiveFailures}</div>
              </div>
              <div>
                <div class="font-medium text-muted">Last interruption</div>
                <div class="mt-1 text-base-content">
                  {current().lastInterruption
                    ? `${Math.max(1, Math.round(current().lastInterruption!.durationSeconds / 60))} min (${current().lastInterruption!.cleanShutdown ? 'clean restart' : 'unexpected stop'})`
                    : 'None recorded'}
                </div>
              </div>
            </div>
          )}
        </Show>

        <Show when={status()?.lastError}>
          <p class="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-100">
            {status()!.lastError}
          </p>
        </Show>

        <p class="text-xs leading-5 text-muted">
          After a restart, Pulse reports any monitoring gap of two minutes or longer to the watchdog
          and records it in alert history. The ping contains only Pulse health and UTC
          timestamps—never infrastructure names, alert contents, or credentials.
        </p>
      </div>
    </SettingsPanel>
  );
}
