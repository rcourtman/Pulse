import {
  createMemo,
  createSignal,
  createUniqueId,
  onCleanup,
  onMount,
  Show,
  type Accessor,
} from 'solid-js';

import { AlertsAPI } from '@/api/alerts';
import { Button } from '@/components/shared/Button';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import type { DeadManStatus } from '@/types/alerts';
import { formatRelativeTime } from '@/utils/format';
import { logger } from '@/utils/logger';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';

interface AlertDeadManDestinationSectionProps {
  pingUrl: Accessor<string>;
  setPingUrl: (value: string) => void;
  setHasUnsavedChanges: (value: boolean) => void;
}

const REDACTED_PING_URL = '***REDACTED***';
// Pulse pings the watchdog every minute, so a status read once at mount goes
// stale while the panel stays open.
export const DEAD_MAN_STATUS_POLL_MS = 30_000;
// A read pending this long at a poll tick counts as a full interval old. The
// slack absorbs timer jitter: the next tick can land a few milliseconds short
// of a full interval after the read the previous tick started.
const DEAD_MAN_STATUS_STALE_MS = DEAD_MAN_STATUS_POLL_MS - 1_000;
// Allow an ordinary background read time to settle. Starting another request
// does not make the last received snapshot current.
const DEAD_MAN_STATUS_READ_OVERDUE_MS = 2 * DEAD_MAN_STATUS_POLL_MS;

const mutedStatusClass = 'bg-surface text-muted ring-1 ring-inset ring-border';
const warningStatusClass = 'bg-amber-100 text-amber-900 dark:bg-amber-900/25 dark:text-amber-100';
const dangerStatusClass = 'bg-red-100 text-red-900 dark:bg-red-900/25 dark:text-red-100';

const statusPresentation: Record<DeadManStatus['state'], { label: string; class: string }> = {
  disabled: {
    label: 'Not configured',
    class: mutedStatusClass,
  },
  starting: {
    label: 'Starting',
    class: 'bg-blue-100 text-blue-800 dark:bg-blue-900/25 dark:text-blue-100',
  },
  healthy: {
    label: 'Heartbeat healthy',
    class: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/25 dark:text-emerald-100',
  },
  delivery_failed: {
    label: 'Delivery failing',
    class: warningStatusClass,
  },
  monitor_stalled: {
    label: 'Monitoring stalled',
    class: dangerStatusClass,
  },
  misconfigured: {
    label: 'Configuration invalid',
    class: dangerStatusClass,
  },
  configuration_unavailable: {
    label: 'Configuration unavailable',
    class: dangerStatusClass,
  },
};

const readPresentation = {
  checking: { label: 'Checking status…', class: mutedStatusClass },
  overdue: { label: 'Status read overdue', class: warningStatusClass },
  unavailable: { label: 'Status unavailable', class: warningStatusClass },
};
const oldHeartbeat = { label: 'No recent heartbeat', class: warningStatusClass };

export function AlertDeadManDestinationSection(props: AlertDeadManDestinationSectionProps) {
  const inputId = `alert-deadman-url-${createUniqueId()}`;
  const titleId = `${inputId}-title`;
  const [statusRead, setStatusRead] = createSignal({
    snapshot: null as DeadManStatus | null,
    checkedAt: Date.now(),
    failed: false,
  });
  const status = () => statusRead().snapshot;
  const [loading, setLoading] = createSignal(false);
  const [showUrl, setShowUrl] = createSignal(false);

  // The status read has no timeout. Every read gets a sequence number, and a
  // read's outcome applies only when it is newer than the last read that
  // settled: a success records a fresh snapshot, a failure keeps the last
  // snapshot but marks the read unavailable. A slow or hung read cannot overwrite a newer
  // outcome or block later reads, and reads slower than the poll interval
  // still land in order. A background read waits while the newest read is
  // younger than one poll interval (less a second of timer slack); a Refresh
  // still pending at such a poll counts as stalled and frees the button.
  // Nothing applies after the panel unmounts.
  let latestStarted = 0;
  let latestSettled = 0;
  let pendingSince: number | undefined;
  let foregroundRequest = 0;
  let foregroundSince = 0;
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });
  const loadStatus = async (background = false) => {
    const startedAt = Date.now();
    if (background) {
      if (loading() && startedAt - foregroundSince >= DEAD_MAN_STATUS_STALE_MS) {
        setLoading(false);
      }
      if (pendingSince !== undefined && startedAt - pendingSince < DEAD_MAN_STATUS_STALE_MS) {
        return;
      }
    } else if (loading()) {
      return;
    }
    const request = ++latestStarted;
    pendingSince = startedAt;
    if (!background) {
      foregroundRequest = request;
      foregroundSince = startedAt;
      setLoading(true);
    }
    try {
      const next = await AlertsAPI.getDeadManStatus();
      if (disposed || request < latestSettled) return;
      latestSettled = request;
      setStatusRead({ snapshot: next, checkedAt: Date.now(), failed: false });
    } catch (error) {
      if (disposed || request < latestSettled) return;
      latestSettled = request;
      logger.error('Failed to load external watchdog status', error);
      setStatusRead((previous) => ({ ...previous, failed: true }));
    } finally {
      if (!disposed) {
        if (request === latestStarted) pendingSince = undefined;
        // A Refresh is answered by its own read or by any newer one.
        if (request >= foregroundRequest) setLoading(false);
      }
    }
  };

  // Last success and Monitor progress keep aging on the shared clock, and the
  // status is re-read in the background so a moving age never describes a
  // heartbeat that has since been sent.
  const now = useRelativeTimeNow();
  onMount(() => {
    void loadStatus();
    const timer = setInterval(() => void loadStatus(true), DEAD_MAN_STATUS_POLL_MS);
    onCleanup(() => clearInterval(timer));
  });

  const readIssue = createMemo(() => {
    const { checkedAt, failed } = statusRead();
    if (failed) return 'unavailable';
    if (now() - checkedAt >= DEAD_MAN_STATUS_READ_OVERDUE_MS) return 'overdue';
    return null;
  });
  const presentation = createMemo(() => {
    const issue = readIssue();
    if (issue) return readPresentation[issue];
    const snapshot = status();
    if (!snapshot) return readPresentation.checking;
    // Reading status is not a heartbeat. Don't keep reporting a healthy
    // heartbeat past Pulse's recommended grace if the sender hasn't advanced it.
    if (
      snapshot.state === 'healthy' &&
      now() - Date.parse(snapshot.lastSuccessAt ?? '') >= snapshot.recommendedGraceSeconds * 1_000
    ) {
      return oldHeartbeat;
    }
    return statusPresentation[snapshot.state];
  });
  const hasStoredUrl = () => props.pingUrl() === REDACTED_PING_URL;
  const inputValue = () => (hasStoredUrl() ? '' : props.pingUrl());

  return (
    <SettingsPanel
      titleId={titleId}
      title="External watchdog"
      description="Detect when Pulse itself stops monitoring by pinging a watchdog on a different host."
      action={
        <div class="flex items-center gap-2">
          <span
            role="status"
            class={`rounded-full px-2.5 py-1 text-xs font-semibold ${presentation().class}`}
          >
            {presentation().label}
          </span>
          <Button
            variant="outline"
            size="xs"
            disabled={loading()}
            onClick={() => void loadStatus()}
          >
            {loading() ? 'Refreshing…' : 'Refresh'}
          </Button>
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
                hasStoredUrl() ? 'Configured. Enter a new URL to replace' : 'https://hc-ping.com/…'
              }
              aria-describedby={`${inputId}-help`}
              class="min-w-0 flex-1 rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm text-base-content shadow-xs focus:border-blue-500 focus:outline-hidden focus:ring-2 focus:ring-blue-500/20"
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
                  class="rounded-md border border-red-300 px-3 py-2 text-sm font-medium text-red-700 hover:bg-red-50 dark:border-red-800 dark:text-red-300 dark:hover:bg-red-950/25"
                  onClick={() => {
                    props.setPingUrl('');
                    props.setHasUnsavedChanges(true);
                  }}
                >
                  Remove
                </button>
              }
            >
              <Button
                variant="outline"
                size="settingsAction"
                aria-pressed={showUrl()}
                onClick={() => setShowUrl((value) => !value)}
              >
                {showUrl() ? 'Hide' : 'Show'}
              </Button>
            </Show>
          </div>
          <p id={`${inputId}-help`} class="mt-2 text-xs leading-5 text-muted">
            Pulse sends a success signal every minute and a <code>/fail</code> signal if its
            monitoring loop stalls. Configure a three-minute grace period at the watchdog. Use a
            service on another machine or network. A watchdog on this Pulse host cannot detect host
            failure. Clearing this field does not pause the remote check.
          </p>
        </div>

        <Show when={readIssue()}>
          <p class="text-xs leading-5 text-muted">
            <Show when={status()} fallback="No watchdog status received yet.">
              {(lastKnown) => `Last known status: ${statusPresentation[lastKnown().state].label}.`}
            </Show>
          </p>
        </Show>

        <Show when={status()}>
          {(current) => (
            <div class="grid gap-3 rounded-md border border-border bg-page p-3 text-xs sm:grid-cols-2 lg:grid-cols-4">
              <div>
                <div class="font-medium text-muted">Last success</div>
                <div class="mt-1 text-base-content">
                  {formatRelativeTime(current().lastSuccessAt, { emptyText: 'Never', now: now() })}
                </div>
              </div>
              <div>
                <div class="font-medium text-muted">Monitor progress</div>
                <div class="mt-1 text-base-content">
                  {formatRelativeTime(current().lastMonitoringProgress, {
                    emptyText: 'Waiting',
                    now: now(),
                  })}
                </div>
              </div>
              <div>
                <div class="font-medium text-muted">Consecutive failures</div>
                <div class="mt-1 text-base-content">{current().consecutiveFailures}</div>
              </div>
              <div>
                <div class="font-medium text-muted">Last interruption</div>
                <div class="mt-1 text-base-content">
                  <Show when={current().lastInterruption} fallback="None recorded">
                    {(gap) =>
                      `${Math.max(1, Math.round(gap().durationSeconds / 60))} min (${gap().cleanShutdown ? 'clean restart' : 'unexpected stop'})`
                    }
                  </Show>
                </div>
              </div>
            </div>
          )}
        </Show>

        <Show when={status()?.lastError}>
          {(message) => (
            <p class="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950/25 dark:text-amber-100">
              {message()}
            </p>
          )}
        </Show>

        <p class="text-xs leading-5 text-muted">
          After a restart, Pulse reports any monitoring gap of two minutes or longer to the watchdog
          and records it in alert history. The ping contains only Pulse health and UTC timestamps,
          never infrastructure names, alert contents, or credentials.
        </p>
      </div>
    </SettingsPanel>
  );
}
