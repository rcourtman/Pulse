import { createMemo, createSignal, createUniqueId, For, Show } from 'solid-js';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';

import type {
  NotificationDeliveryLog,
  NotificationDeliveryLogEntry,
  Webhook,
} from '@/api/notifications';
import { Card } from '@/components/shared/Card';
import type { AlertEvent } from '@/types/api';
import {
  getAlertDeliveryLogFailureClassLabel,
  getAlertDeliveryLogOutcomeLabel,
  getAlertDestinationsDeliveryLogDescription,
  getAlertDestinationsDeliveryLogEmpty,
  getAlertDestinationsDeliveryLogTitle,
  getAlertDestinationsDeliveryLogUnavailable,
  getAlertDestinationsDeliveryLogLoading,
  getAlertDestinationsHeldEventsLoading,
  getAlertDestinationsHeldEventsUnavailable,
  getAlertDestinationsDeliveryRefreshLabel,
} from '@/utils/alertDestinationsPresentation';
import { formatRelativeTime } from '@/utils/format';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';

import { describeAlertEventReason } from './deliveryDiagnosisPresentation';

interface AlertDeliveryLogCardProps {
  log: NotificationDeliveryLog | null;
  unavailable: boolean;
  refreshing: boolean;
  onRefresh: () => void;
  webhooks: Webhook[];
  heldEvents?: AlertEvent[];
  heldEventsUnavailable?: boolean;
  refreshingHeldEvents?: boolean;
}

export type DeliveryLogRow =
  | { kind: 'attempt'; timestamp: string; entry: NotificationDeliveryLogEntry }
  | { kind: 'held'; timestamp: string; event: AlertEvent };

export type DeliveryLogFilter = 'all' | 'failed' | 'retry' | 'held' | 'sent' | 'cancelled';

type DeliveryLogBucket = Exclude<DeliveryLogFilter, 'all'>;

const FILTER_BUCKETS: readonly { id: DeliveryLogBucket; label: string }[] = [
  { id: 'failed', label: 'Failures' },
  { id: 'retry', label: 'Retrying' },
  { id: 'held', label: 'Held' },
  { id: 'sent', label: 'Delivered' },
  { id: 'cancelled', label: 'Cancelled' },
];

export const getDeliveryLogRowBucket = (row: DeliveryLogRow): DeliveryLogBucket => {
  if (row.kind === 'held') return 'held';
  if (row.entry.outcome === 'failed' || row.entry.outcome === 'dead_letter') return 'failed';
  if (row.entry.outcome === 'retry') return 'retry';
  if (row.entry.outcome === 'cancelled') return 'cancelled';
  return 'sent';
};

// mergeDeliveryLogRows interleaves delivery attempts with held-notification
// events, newest first, so the activity list tells the whole story: what was
// sent, what failed, and what was deliberately not attempted and why.
export const mergeDeliveryLogRows = (
  entries: NotificationDeliveryLogEntry[],
  heldEvents: AlertEvent[],
): DeliveryLogRow[] => {
  const rows: DeliveryLogRow[] = [
    ...entries.map((entry): DeliveryLogRow => ({
      kind: 'attempt',
      timestamp: entry.timestamp,
      entry,
    })),
    ...heldEvents.map((event): DeliveryLogRow => ({
      kind: 'held',
      timestamp: event.occurredAt,
      event,
    })),
  ];
  return rows.sort((a, b) => {
    const at = new Date(a.timestamp).getTime() || 0;
    const bt = new Date(b.timestamp).getTime() || 0;
    return bt - at;
  });
};

const WEBHOOK_DESTINATION_PREFIX = 'webhook:';

const outcomeBadgeClasses: Record<NotificationDeliveryLogEntry['outcome'], string> = {
  sent: 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  retry: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  failed: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  dead_letter: 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300',
  cancelled: 'bg-surface-alt text-muted',
};

const filterButtonClass = (id: DeliveryLogFilter, active: boolean): string => {
  if (!active) {
    return 'border-border bg-surface text-muted hover:bg-surface-hover hover:text-base-content';
  }
  if (id === 'failed') {
    return 'border-red-300 bg-red-100 text-red-800 dark:border-red-800 dark:bg-red-900/40 dark:text-red-300';
  }
  if (id === 'retry') {
    return 'border-amber-300 bg-amber-100 text-amber-800 dark:border-amber-800 dark:bg-amber-900/40 dark:text-amber-300';
  }
  return 'border-blue-300 bg-blue-50 text-blue-700 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-300';
};

const formatAttemptCount = (entry: NotificationDeliveryLogEntry): string | null => {
  if (!Number.isFinite(entry.attempts) || entry.attempts <= 0) return null;
  if (entry.attempts <= 1 && entry.outcome !== 'retry') return null;
  return entry.attempts === 1 ? '1 attempt' : `${entry.attempts} attempts`;
};

const heldDetailMessage = (event: AlertEvent): string | null => {
  const message = event.message?.trim();
  if (!message) return null;
  const reasonLabel = describeAlertEventReason(event.reason).trim();
  if (reasonLabel && message.toLowerCase() === reasonLabel.toLowerCase()) return null;
  return message;
};

export function AlertDeliveryLogCard(props: AlertDeliveryLogCardProps) {
  const titleId = createUniqueId();
  const [selectedFilter, setSelectedFilter] = createSignal<DeliveryLogFilter>('all');
  const [searchQuery, setSearchQuery] = createSignal('');

  // Log rows stay mounted between refreshes and their times never change, so
  // the age in each row's tooltip reads the shared clock.
  const now = useRelativeTimeNow();
  // Destination ids are opaque on purpose (hashes for email/apprise, config
  // ids for webhooks); only webhook ids can be resolved to a configured name.
  const destinationLabel = (entry: NotificationDeliveryLogEntry): string => {
    if (entry.destinationId?.startsWith(WEBHOOK_DESTINATION_PREFIX)) {
      const webhookId = entry.destinationId.slice(WEBHOOK_DESTINATION_PREFIX.length);
      const webhook = props.webhooks.find((candidate) => candidate.id === webhookId);
      if (webhook?.name) return webhook.name;
    }
    switch (entry.type) {
      case 'email':
        return 'Email';
      case 'apprise':
        return 'Apprise';
      case 'webhook':
        return 'Webhook';
      default:
        return entry.type || 'Destination';
    }
  };

  const alertSummary = (entry: NotificationDeliveryLogEntry): string => {
    if (entry.alertIds.length === 0) {
      return entry.alertCount === 1 ? '1 alert' : `${entry.alertCount} alerts`;
    }
    const [first, ...rest] = entry.alertIds;
    return rest.length > 0 ? `${first} +${rest.length} more` : first;
  };

  const entries = () => (props.unavailable ? [] : (props.log?.entries ?? []));
  const completedRetentionDays = () => props.log?.completedRetentionDays ?? 7;
  const deadLetterRetentionDays = () => props.log?.deadLetterRetentionDays ?? 30;
  const rows = createMemo(() =>
    mergeDeliveryLogRows(entries(), props.heldEventsUnavailable ? [] : (props.heldEvents ?? [])),
  );

  const bucketCounts = createMemo(() => {
    const counts: Record<DeliveryLogBucket, number> = {
      failed: 0,
      retry: 0,
      held: 0,
      sent: 0,
      cancelled: 0,
    };
    for (const row of rows()) {
      counts[getDeliveryLogRowBucket(row)] += 1;
    }
    return counts;
  });

  const availableBuckets = createMemo(() =>
    FILTER_BUCKETS.filter((bucket) => bucketCounts()[bucket.id] > 0),
  );

  // If a refresh clears the selected bucket (for example after dismissing
  // retained failures), return to All rather than stranding the view empty.
  const effectiveFilter = createMemo<DeliveryLogFilter>(() => {
    const current = selectedFilter();
    if (current !== 'all' && bucketCounts()[current] === 0) return 'all';
    return current;
  });

  const normalizedSearch = createMemo(() => searchQuery().trim().toLowerCase());

  const heldResourceLabel = (event: AlertEvent): string => {
    const resource = event.resourceName || event.alertId;
    if (event.alertType) return `${resource} (${event.alertType})`;
    return resource;
  };

  const rowMatchesSearch = (row: DeliveryLogRow, query: string): boolean => {
    if (!query) return true;
    if (row.kind === 'attempt') {
      const { entry } = row;
      const haystack = [
        destinationLabel(entry),
        entry.type,
        getAlertDeliveryLogOutcomeLabel(entry.outcome),
        ...entry.alertIds,
        entry.failureClass ? getAlertDeliveryLogFailureClassLabel(entry.failureClass) : '',
        entry.errorMessage ?? '',
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    }
    const { event } = row;
    const haystack = [
      event.type === 'notification_deferred' ? 'deferred' : 'held',
      heldResourceLabel(event),
      event.alertId,
      describeAlertEventReason(event.reason),
      event.reason ?? '',
      event.message ?? '',
    ]
      .join(' ')
      .toLowerCase();
    return haystack.includes(query);
  };

  const filteredRows = createMemo(() => {
    const filter = effectiveFilter();
    const query = normalizedSearch();
    if (filter === 'all' && !query) return rows();
    return rows().filter((row) => {
      if (filter !== 'all' && getDeliveryLogRowBucket(row) !== filter) return false;
      return rowMatchesSearch(row, query);
    });
  });

  const showOutcomeFilters = () => availableBuckets().length > 1;
  const showSearch = () => rows().length > 1;
  const hasActiveFilters = () => effectiveFilter() !== 'all' || normalizedSearch().length > 0;
  const clearFilters = () => {
    setSelectedFilter('all');
    setSearchQuery('');
  };

  const attemptsLoading = () => props.refreshing || (!props.log && !props.unavailable);
  const confirmedEmpty = () =>
    props.log !== null &&
    !props.unavailable &&
    !attemptsLoading() &&
    !props.heldEventsUnavailable &&
    !props.refreshingHeldEvents;

  const absoluteTimestamp = (value: string): string => {
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return parsed.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  return (
    <Card id="notification-delivery-activity" padding="sm" class="scroll-mt-24 sm:p-4">
      <div class="flex flex-col gap-3">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <h3 id={titleId} class="text-sm font-semibold text-base-content">
              {getAlertDestinationsDeliveryLogTitle()}
            </h3>
            <p class="mt-1 text-sm leading-6 text-muted">
              {getAlertDestinationsDeliveryLogDescription(
                completedRetentionDays(),
                deadLetterRetentionDays(),
              )}
            </p>
          </div>
          <button
            type="button"
            class="inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-base-content transition hover:bg-surface-hover aria-disabled:cursor-not-allowed aria-disabled:opacity-50 sm:min-h-0"
            aria-disabled={props.refreshing}
            onClick={() => {
              if (!props.refreshing) props.onRefresh();
            }}
          >
            <RefreshCwIcon
              class={`h-4 w-4 ${props.refreshing ? 'animate-spin' : ''}`}
              aria-hidden="true"
            />
            {getAlertDestinationsDeliveryRefreshLabel()}
          </button>
        </div>

        <Show when={props.unavailable}>
          <p class="text-sm text-red-800 dark:text-red-300" role="alert">
            {getAlertDestinationsDeliveryLogUnavailable()}
          </p>
        </Show>
        <Show when={props.heldEventsUnavailable}>
          <p class="text-sm text-red-800 dark:text-red-300" role="alert">
            {getAlertDestinationsHeldEventsUnavailable()}
          </p>
        </Show>
        <Show when={attemptsLoading() && entries().length === 0}>
          <p class="text-sm text-muted" role="status">
            {getAlertDestinationsDeliveryLogLoading()}
          </p>
        </Show>
        <Show when={props.refreshingHeldEvents}>
          <p class="text-sm text-muted" role="status">
            {getAlertDestinationsHeldEventsLoading()}
          </p>
        </Show>
        <Show
          when={rows().length > 0}
          fallback={
            <Show when={confirmedEmpty()}>
              <p class="text-sm text-muted">{getAlertDestinationsDeliveryLogEmpty()}</p>
            </Show>
          }
        >
          <Show when={showOutcomeFilters() || showSearch()}>
            <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
              <Show when={showOutcomeFilters()}>
                <div
                  role="group"
                  aria-label="Filter delivery activity by outcome"
                  class="flex flex-wrap items-center gap-1.5"
                >
                  <button
                    type="button"
                    aria-pressed={effectiveFilter() === 'all'}
                    onClick={() => setSelectedFilter('all')}
                    class={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium transition ${filterButtonClass('all', effectiveFilter() === 'all')}`}
                  >
                    All ({rows().length})
                  </button>
                  <For each={availableBuckets()}>
                    {(bucket) => (
                      <button
                        type="button"
                        aria-pressed={effectiveFilter() === bucket.id}
                        onClick={() => setSelectedFilter(bucket.id)}
                        class={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-medium transition ${filterButtonClass(bucket.id, effectiveFilter() === bucket.id)}`}
                      >
                        {bucket.label} ({bucketCounts()[bucket.id]})
                      </button>
                    )}
                  </For>
                </div>
              </Show>
              <Show when={showSearch()}>
                <div class="w-full sm:ml-auto sm:w-64">
                  <input
                    type="search"
                    value={searchQuery()}
                    onInput={(event) => setSearchQuery(event.currentTarget.value)}
                    onKeyDown={(event) => {
                      if (event.key === 'Escape' && searchQuery()) {
                        event.preventDefault();
                        setSearchQuery('');
                      }
                    }}
                    placeholder="Filter destination, alert, or error"
                    aria-label="Filter delivery activity"
                    class="w-full rounded-md border border-border bg-surface px-2.5 py-1.5 text-xs text-base-content placeholder:text-muted focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-blue-500"
                  />
                </div>
              </Show>
            </div>
          </Show>

          <Show
            when={filteredRows().length > 0}
            fallback={
              <div class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-surface-alt px-3 py-3 text-sm text-muted">
                <span>No matching delivery activity.</span>
                <Show when={hasActiveFilters()}>
                  <button
                    type="button"
                    onClick={clearFilters}
                    class="rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-base-content transition hover:bg-surface-hover"
                  >
                    Clear filters
                  </button>
                </Show>
              </div>
            }
          >
            <ul
              tabIndex={0}
              aria-labelledby={titleId}
              class="max-h-80 divide-y divide-border overflow-y-auto rounded-sm focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
            >
              <For each={filteredRows()}>
                {(row) =>
                  row.kind === 'attempt' ? (
                    <li class="flex flex-col gap-1 py-2">
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                        <span
                          class={`inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${outcomeBadgeClasses[row.entry.outcome]}`}
                        >
                          {getAlertDeliveryLogOutcomeLabel(row.entry.outcome)}
                        </span>
                        <span class="min-w-0 truncate text-sm font-medium text-base-content">
                          {destinationLabel(row.entry)}
                        </span>
                        <span
                          class="min-w-0 truncate text-sm text-muted"
                          title={row.entry.alertIds.join(', ')}
                        >
                          {alertSummary(row.entry)}
                        </span>
                        <Show when={formatAttemptCount(row.entry)}>
                          {(attempts) => (
                            <span class="shrink-0 text-xs tabular-nums text-muted">
                              {attempts()}
                            </span>
                          )}
                        </Show>
                        <time
                          class="ml-auto shrink-0 text-xs tabular-nums text-muted"
                          dateTime={row.entry.timestamp}
                          title={formatRelativeTime(row.entry.timestamp, { now: now() })}
                        >
                          {absoluteTimestamp(row.entry.timestamp)}
                        </time>
                      </div>
                      <Show
                        when={
                          !row.entry.success && (row.entry.failureClass || row.entry.errorMessage)
                        }
                      >
                        <p class="text-xs leading-5 text-red-700 dark:text-red-300">
                          <Show when={row.entry.failureClass}>
                            {(failureClass) => (
                              <span class="font-medium">
                                {getAlertDeliveryLogFailureClassLabel(failureClass())}
                                {row.entry.errorMessage ? ': ' : ''}
                              </span>
                            )}
                          </Show>
                          {row.entry.errorMessage}
                        </p>
                      </Show>
                    </li>
                  ) : (
                    <li class="flex flex-col gap-1 py-2" title={row.event.message}>
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
                        <span
                          class={`inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                            row.event.type === 'notification_deferred'
                              ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300'
                              : 'bg-surface-alt text-muted'
                          }`}
                        >
                          {row.event.type === 'notification_deferred' ? 'Deferred' : 'Held'}
                        </span>
                        <span class="min-w-0 truncate text-sm font-medium text-base-content">
                          {heldResourceLabel(row.event)}
                        </span>
                        <span class="min-w-0 truncate text-sm text-muted">
                          {describeAlertEventReason(row.event.reason)}
                        </span>
                        <time
                          class="ml-auto shrink-0 text-xs tabular-nums text-muted"
                          dateTime={row.event.occurredAt}
                          title={formatRelativeTime(row.event.occurredAt, { now: now() })}
                        >
                          {absoluteTimestamp(row.event.occurredAt)}
                        </time>
                      </div>
                      <Show when={heldDetailMessage(row.event)}>
                        {(detail) => <p class="text-xs leading-5 text-muted">{detail()}</p>}
                      </Show>
                    </li>
                  )
                }
              </For>
            </ul>
          </Show>
        </Show>
      </div>
    </Card>
  );
}
