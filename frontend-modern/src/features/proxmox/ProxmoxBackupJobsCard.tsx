import {
  createMemo,
  createSignal,
  createUniqueId,
  For,
  onCleanup,
  Show,
  type Component,
} from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { supportsHoverTooltips } from '@/components/shared/hoverCapability';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { getRecoveryPrettyDateLabel } from '@/utils/recoveryDatePresentation';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';
import {
  backupJobsDayObserved,
  backupJobsHasHistory,
  backupJobsTotal,
  backupJobsUnavailable,
  fetchBackupJobs,
  type BackupJobsCounts,
  type BackupJobsDay,
  type BackupJobsFailure,
  type BackupJobsPayload,
} from './proxmoxBackupJobs';

// "Did last night's backups run, and is that normal?" in the platform
// overview's vocabulary: last-24-hour outcome pills in the header strip and
// one stacked bar per day for the last 14 days. Days Pulse did not fully
// observe are outlined, never drawn as clean empty days. Each window is named
// where it is shown, so the pills never read as the chart's totals: the pills
// say "Last 24h", and the chart's axis runs from its first day to Today.

type Outcome = keyof BackupJobsCounts;

const PILL_BASE =
  'inline-flex items-center whitespace-nowrap rounded-md px-1 sm:px-1.5 py-0.5 text-[10px] sm:text-[11px] font-semibold tabular-nums';

const OUTCOMES: readonly {
  key: Outcome;
  label: string;
  pill: string;
  segment: string;
}[] = [
  {
    key: 'failed',
    label: 'failed',
    pill: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300',
    segment: 'bg-metric-critical-bg',
  },
  {
    key: 'warning',
    label: 'warnings',
    pill: 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
    segment: 'bg-metric-warning-bg',
  },
  {
    key: 'running',
    label: 'running',
    pill: 'bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300',
    segment: 'bg-sky-400',
  },
  {
    key: 'unknown',
    label: 'unknown',
    pill: 'bg-surface-alt text-muted',
    segment: 'bg-slate-400',
  },
  {
    key: 'success',
    label: 'ok',
    pill: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    segment: 'bg-metric-normal-bg',
  },
];

const OUTCOME_NAME_LIMIT = 8;
const NAMED_OUTCOMES = [
  { key: 'failed', label: 'Guests with failures' },
  { key: 'warning', label: 'Guests with warnings' },
] as const;
type NamedOutcome = (typeof NAMED_OUTCOMES)[number];

const dayEmptyLabel = (day: BackupJobsDay): string => {
  if (day.historyState === 'unavailable') return 'Counts unavailable';
  return backupJobsDayObserved(day) ? 'No backups' : 'No outcomes observed';
};

const dayTitle = (day: BackupJobsDay): string => {
  const date = getRecoveryPrettyDateLabel(day.date);
  const observed = backupJobsDayObserved(day) ? '' : ' (not fully observed)';
  const counts = OUTCOMES.filter(({ key }) => day.counts[key] > 0)
    .reverse()
    .map(({ key, label }) => `${day.counts[key]} ${label}`)
    .join(', ');
  return `${date}: ${counts || dayEmptyLabel(day).toLowerCase()}${observed}`;
};

const failureNameList = (failures: readonly BackupJobsFailure[], outcome: string): string[] => [
  ...new Set(
    failures
      .filter((failure) => failure.outcome === outcome)
      .map((failure) => failure.workloadName)
      .filter((name) => name.trim() !== ''),
  ),
];

const failureNames = (failures: readonly BackupJobsFailure[], outcome: string): string => {
  const names = failureNameList(failures, outcome);
  const shown = names.slice(0, OUTCOME_NAME_LIMIT);
  const more = names.length - shown.length;
  return more > 0 ? `${shown.join(', ')} and ${more} more` : shown.join(', ');
};

const dayOutcomeNames = (day: BackupJobsDay, outcome: NamedOutcome) =>
  day.counts[outcome.key] > 0 ? failureNameList(day.failures, outcome.key) : [];

const hasNamedOutcomes = (day: BackupJobsDay) =>
  NAMED_OUTCOMES.some((outcome) => dayOutcomeNames(day, outcome).length > 0);

// A keyboard activation or touch on a hybrid device has not seen a mouse
// tooltip, even when that device's primary pointer supports hover.
const canPreviewNames = (event: MouseEvent) =>
  event.detail > 0 &&
  (!('pointerType' in event) || !event.pointerType || event.pointerType === 'mouse') &&
  supportsHoverTooltips();

const SelectedBackupJobNames: Component<{ day: BackupJobsDay; outcome: NamedOutcome }> = (
  props,
) => {
  const names = createMemo(() => dayOutcomeNames(props.day, props.outcome));
  const [expandedDate, setExpandedDate] = createSignal<string | null>(null);
  const expanded = () => expandedDate() === props.day.date;
  const namesId = createUniqueId();
  const toggleLabel = () =>
    expanded() ? 'Show fewer' : `Show ${names().length - OUTCOME_NAME_LIMIT} more`;
  return (
    <Show when={names().length > 0}>
      <p class="wrap-anywhere">
        <span class="font-medium">{props.outcome.label}: </span>
        <span id={namesId}>
          {(expanded() ? names() : names().slice(0, OUTCOME_NAME_LIMIT)).join(', ')}
        </span>
        <Show when={names().length > OUTCOME_NAME_LIMIT}>
          {' '}
          <button
            type="button"
            class="font-medium text-blue-600 hover:underline dark:text-blue-300"
            aria-controls={namesId}
            aria-expanded={expanded()}
            aria-label={`${toggleLabel()} ${props.outcome.label.toLowerCase()}`}
            onClick={() => setExpandedDate(expanded() ? null : props.day.date)}
          >
            {toggleLabel()}
          </button>
        </Show>
      </p>
    </Show>
  );
};

export interface BackupJobsDaySelection {
  selectedDay: () => string | null;
  // `reveal` brings the restore points into view. Keep the chart in view
  // when selecting named outcomes that the operator could not preview.
  onSelectDay: (date: string, options: { reveal: boolean }) => void;
  onRevealRestorePoints: () => void;
}

export const ProxmoxBackupJobsCard: Component<{
  jobs: BackupJobsPayload;
  selectedDay?: string | null;
  onSelectDay?: BackupJobsDaySelection['onSelectDay'];
  onRevealRestorePoints?: BackupJobsDaySelection['onRevealRestorePoints'];
  onRetry?: () => void;
  retrying?: boolean;
  accessMessage?: string | null;
}> = (props) => {
  let card: HTMLDivElement | undefined;
  const retainRetryFocus = (button: HTMLButtonElement) => {
    onCleanup(() => {
      if (document.activeElement !== button) return;
      // A confirmed empty result legitimately omits the jobs card. Keep the
      // operator in the existing inspection rather than focusing a retired
      // card or leaving focus on body. A departed surface is never revived.
      const inspection = card?.closest<HTMLElement>(
        '[role="region"][aria-label="Backup inspection"]',
      );
      queueMicrotask(() => {
        if (document.activeElement !== document.body) return;
        const target = card?.isConnected ? card : inspection;
        if (target?.isConnected) {
          target.focus({ preventScroll: true });
        }
      });
    });
  };
  const last = () => props.jobs.last24h;
  const maxTotal = () => Math.max(1, ...props.jobs.days.map((day) => backupJobsTotal(day.counts)));
  const pills = () => {
    const counts = last().counts;
    const shown = OUTCOMES.filter(({ key }) => counts[key] > 0);
    if (shown.length > 0) return shown.reverse();
    return last().historyState === 'complete' ? [OUTCOMES[OUTCOMES.length - 1]] : [];
  };
  const namesFor = (outcome: string) => failureNames(last().failures, outcome);
  const chosenDay = createMemo(
    () => props.jobs.days.find((day) => day.date === props.selectedDay) ?? null,
  );
  const chosenDayHasNames = createMemo(() => {
    const day = chosenDay();
    return day !== null && hasNamedOutcomes(day);
  });
  const chosenDayNamesId = createUniqueId();
  const [hover, setHover] = createSignal<{ date: string; x: number; y: number } | null>(null);
  // Read through the payload, so a poll updates an open tooltip and a day that
  // has left the window withdraws it.
  const hoveredDay = createMemo(() => {
    const target = hover();
    return target ? (props.jobs.days.find((day) => day.date === target.date) ?? null) : null;
  });
  // The button fills the column, so anchor on the bar's own top rather than a
  // point far above a short bar.
  const showDay = (target: HTMLElement, date: string) => {
    const rect = target.getBoundingClientRect();
    const tops = [...target.querySelectorAll('[aria-hidden="true"]')].map(
      (segment) => segment.getBoundingClientRect().top,
    );
    setHover({
      date,
      x: rect.left + rect.width / 2,
      y: tops.length > 0 ? Math.min(...tops) : rect.bottom,
    });
  };
  const hideDay = () => setHover(null);
  const unavailable = () => props.jobs.historyState === 'unavailable';
  const hasUnavailableCounts = () =>
    unavailable() ||
    last().historyState === 'unavailable' ||
    props.jobs.days.some((day) => day.historyState === 'unavailable');
  // The selected date is the operator's filter, not evidence from this
  // history response. The table owns the independent restore-points jump.
  const restorePointsButton = () => (
    <button
      type="button"
      class="whitespace-nowrap font-medium text-blue-600 hover:underline dark:text-blue-300"
      onClick={() => props.onRevealRestorePoints?.()}
    >
      View restore points
    </button>
  );

  return (
    <TableCard
      ref={card}
      role="region"
      aria-label="Backup jobs"
      tabIndex={-1}
      class="flex h-full flex-col focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500"
      data-backup-jobs={props.jobs.historyState}
    >
      <TableCardHeader
        title={<span class="whitespace-nowrap">Backup jobs</span>}
        actions={
          <Show
            when={!unavailable()}
            fallback={<span class="text-[11px] text-muted">History unavailable</span>}
          >
            <span
              class="inline-flex flex-wrap items-center justify-end gap-1 sm:gap-1.5"
              data-backup-jobs-last24h
            >
              <Show
                when={last().historyState !== 'unavailable'}
                fallback={<span class="text-[11px] text-muted">Last 24h unavailable</span>}
              >
                {/* The window is named at every width; phones get the short form. */}
                <span class="text-[10px] font-medium text-muted">
                  <span class="sm:hidden">24h</span>
                  <span class="hidden sm:inline">Last 24h</span>
                </span>
                <For
                  each={pills()}
                  fallback={<span class="text-[11px] text-muted">No outcomes observed</span>}
                >
                  {(outcome) => (
                    <span
                      class={`${PILL_BASE} ${outcome.pill}`}
                      title={
                        outcome.key === 'failed'
                          ? namesFor('failed')
                          : outcome.key === 'warning'
                            ? namesFor('warning')
                            : undefined
                      }
                      data-backup-jobs-pill={outcome.key}
                    >
                      {last().counts[outcome.key]} {outcome.label}
                    </span>
                  )}
                </For>
              </Show>
              <Show
                when={last().historyState !== 'complete' && last().historyState !== 'unavailable'}
              >
                <span
                  class="text-[10px] font-medium text-muted"
                  title="Pulse did not see the whole last 24 hours, so these counts may be low"
                  data-backup-jobs-incomplete
                >
                  incomplete data
                </span>
              </Show>
            </span>
          </Show>
        }
      />
      <Show when={hasUnavailableCounts()}>
        <InlineNotice layout="banner" tone="warning">
          <p role="status">
            {unavailable()
              ? 'Backup job counts are unknown. Restore points and protection are shown separately.'
              : 'Some backup job counts are unknown.'}
            <Show when={props.accessMessage}> {props.accessMessage}</Show>
          </p>
          <div class="mt-2 flex flex-wrap items-center gap-2">
            <Show when={props.onRetry}>
              <button
                ref={retainRetryFocus}
                type="button"
                class="min-h-10 rounded-md border border-current px-3 font-medium hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500 aria-disabled:opacity-50"
                aria-label="Retry backup jobs"
                aria-disabled={props.retrying}
                onClick={() => {
                  if (!props.retrying) props.onRetry?.();
                }}
              >
                {props.retrying ? 'Retrying…' : 'Retry backup jobs'}
              </button>
            </Show>
            <Show when={props.selectedDay && !chosenDay() && props.onRevealRestorePoints}>
              {restorePointsButton()}
            </Show>
          </div>
        </InlineNotice>
      </Show>
      <Show when={props.jobs.days.length > 0}>
        <div class="flex flex-1 flex-col px-3 pb-2 pt-3">
          <ol
            class="flex min-h-14 flex-1 items-end gap-1"
            aria-label={`Backups per day, last ${props.jobs.days.length} days`}
          >
            <For each={props.jobs.days}>
              {(day) => {
                const total = () => backupJobsTotal(day.counts);
                const observed = () => backupJobsDayObserved(day);
                const chosen = () => props.selectedDay === day.date;
                return (
                  <li
                    class={`flex h-full min-w-0 flex-1 rounded-xs ${
                      observed() ? '' : 'border border-dashed border-border'
                    }`}
                    data-backup-jobs-day={observed() ? 'observed' : day.historyState}
                  >
                    <button
                      type="button"
                      class={`flex h-full w-full cursor-pointer flex-col-reverse rounded-xs focus-visible:outline-2 focus-visible:outline-blue-500 ${
                        chosen()
                          ? 'outline-2 outline-offset-1 outline-blue-500'
                          : `hover:outline-1 hover:outline-border ${
                              chosenDay() ? 'opacity-40 hover:opacity-100' : ''
                            }`
                      }`}
                      aria-pressed={chosen()}
                      aria-describedby={
                        chosen() && chosenDayHasNames() ? chosenDayNamesId : undefined
                      }
                      onClick={(event) =>
                        props.onSelectDay?.(day.date, {
                          reveal: !hasNamedOutcomes(day) || canPreviewNames(event),
                        })
                      }
                      onMouseEnter={(event) => showDay(event.currentTarget, day.date)}
                      onMouseLeave={hideDay}
                      onFocus={(event) => showDay(event.currentTarget, day.date)}
                      onBlur={hideDay}
                    >
                      <span class="sr-only">{dayTitle(day)}</span>
                      <Show
                        when={total() > 0}
                        fallback={
                          <Show when={observed()}>
                            <span
                              class="h-0.5 w-full rounded-xs bg-surface-hover"
                              aria-hidden="true"
                            />
                          </Show>
                        }
                      >
                        <For each={[...OUTCOMES].reverse()}>
                          {(outcome) => (
                            <Show when={day.counts[outcome.key] > 0}>
                              <span
                                class={`w-full first:rounded-b-xs last:rounded-t-xs ${outcome.segment}`}
                                style={{
                                  height: `max(2px, ${(day.counts[outcome.key] / maxTotal()) * 100}%)`,
                                }}
                                aria-hidden="true"
                              />
                            </Show>
                          )}
                        </For>
                      </Show>
                    </button>
                  </li>
                );
              }}
            </For>
          </ol>
          <div class="mt-1 flex justify-between text-[10px] text-muted" aria-hidden="true">
            <span>{getRecoveryPrettyDateLabel(props.jobs.days[0]?.date ?? '')}</span>
            <span>Today</span>
          </div>
          <Show when={chosenDay()}>
            {(day) => (
              <div class="mt-1 text-[11px] text-base-content" data-backup-jobs-chosen-day>
                <p>{dayTitle(day())}</p>
                <div id={chosenDayNamesId}>
                  <For each={NAMED_OUTCOMES}>
                    {(outcome) => <SelectedBackupJobNames day={day()} outcome={outcome} />}
                  </For>
                </div>
                <Show
                  when={
                    props.onRevealRestorePoints &&
                    (chosenDayHasNames() || day().historyState === 'unavailable')
                  }
                >
                  {restorePointsButton()}
                </Show>
              </div>
            )}
          </Show>
        </div>
      </Show>
      <TooltipPortal
        when={hoveredDay() !== null}
        x={hover()?.x ?? 0}
        y={hover()?.y ?? 0}
        align="center"
        direction="up"
        maxWidth={240}
      >
        <Show when={hoveredDay()}>
          {(day) => (
            <div class="min-w-[160px]" data-backup-jobs-tooltip>
              <div class="border-b border-border pb-1 text-[11px] font-semibold text-base-content">
                {getRecoveryPrettyDateLabel(day().date)}
                <Show when={!backupJobsDayObserved(day())}>
                  <span class="ml-1 font-normal text-muted">not fully observed</span>
                </Show>
              </div>
              <Show
                when={backupJobsTotal(day().counts) > 0}
                fallback={<div class="mt-1.5 text-muted">{dayEmptyLabel(day())}</div>}
              >
                <ul class="mt-1.5 space-y-1">
                  <For each={OUTCOMES.filter(({ key }) => day().counts[key] > 0).reverse()}>
                    {(outcome) => (
                      <li>
                        <div class="flex items-center justify-between gap-4 text-base-content">
                          <span class="flex min-w-0 items-center gap-1.5">
                            <span class={`h-2 w-2 shrink-0 rounded-xs ${outcome.segment}`} />
                            <span class="truncate">{outcome.label}</span>
                          </span>
                          <span class="shrink-0 font-mono tabular-nums">
                            {day().counts[outcome.key]}
                          </span>
                        </div>
                        <Show when={outcome.key === 'failed' || outcome.key === 'warning'}>
                          <div class="mt-0.5 text-muted">
                            {failureNames(day().failures, outcome.key)}
                          </div>
                        </Show>
                      </li>
                    )}
                  </For>
                </ul>
              </Show>
            </div>
          )}
        </Show>
      </TooltipPortal>
    </TableCard>
  );
};

const BACKUP_JOBS_POLL_MS = 60_000;

// Owns the backup jobs read. Renders nothing until history exists, so a page
// without PVE backup jobs keeps Backup health alone at full width.
export const ProxmoxBackupJobsSection: Component<BackupJobsDaySelection> = (props) => {
  const query = createNonSuspendingQuery({
    source: () => 'jobs',
    fetcher: (_source, signal) => fetchBackupJobs(signal),
    initialValue: null as BackupJobsPayload | null,
    pollMs: BACKUP_JOBS_POLL_MS,
  });
  // The query already retains ordinary same-context observations and clears
  // them on organization/access withdrawal. A second component-local cache
  // must not revive a payload after that owner has discarded it.
  const shown = createMemo((): BackupJobsPayload | null => {
    if (query.error()) return backupJobsUnavailable(query.value());
    const jobs = query.value();
    return backupJobsHasHistory(jobs) ? jobs : null;
  });
  const accessMessage = () => {
    const error = query.error();
    if (error && typeof error === 'object' && 'status' in error && error.status === 403) {
      return 'Access denied. Check your monitoring permissions.';
    }
    return getAPIReadAccessErrorMessage(error);
  };
  return (
    <Show when={shown()}>
      {(jobs) => (
        <ProxmoxBackupJobsCard
          jobs={jobs()}
          selectedDay={props.selectedDay()}
          onSelectDay={props.onSelectDay}
          onRevealRestorePoints={props.onRevealRestorePoints}
          onRetry={() => {
            if (!query.loading()) void query.refetch();
          }}
          retrying={query.loading()}
          accessMessage={accessMessage()}
        />
      )}
    </Show>
  );
};
