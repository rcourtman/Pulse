import { createMemo, For, Show, type Component } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { getRecoveryPrettyDateLabel } from '@/utils/recoveryDatePresentation';
import {
  backupJobsDayObserved,
  backupJobsHasHistory,
  backupJobsTotal,
  backupJobsUnavailable,
  fetchBackupJobs,
  type BackupJobsCounts,
  type BackupJobsDay,
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
  'inline-flex items-center whitespace-nowrap rounded-md px-1.5 py-0.5 text-[11px] font-semibold tabular-nums';

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

const FAILED_NAMES = 8;

const describe = (counts: BackupJobsCounts): string => {
  const parts = OUTCOMES.filter(({ key }) => counts[key] > 0)
    .reverse()
    .map(({ key, label }) => `${counts[key]} ${label}`);
  return parts.length > 0 ? parts.join(', ') : 'no backups';
};

const dayTitle = (day: BackupJobsDay): string => {
  const date = getRecoveryPrettyDateLabel(day.date);
  const observed = backupJobsDayObserved(day) ? '' : ' (not fully observed)';
  return `${date}: ${describe(day.counts)}${observed}`;
};

export const ProxmoxBackupJobsCard: Component<{ jobs: BackupJobsPayload }> = (props) => {
  const last = () => props.jobs.last24h;
  const maxTotal = () => Math.max(1, ...props.jobs.days.map((day) => backupJobsTotal(day.counts)));
  const pills = () => {
    const counts = last().counts;
    const shown = OUTCOMES.filter(({ key }) => counts[key] > 0);
    return shown.length > 0 ? shown.reverse() : [OUTCOMES[OUTCOMES.length - 1]];
  };
  const namesFor = (outcome: string) => {
    const names = [
      ...new Set(
        last()
          .failures.filter((failure) => failure.outcome === outcome)
          .map((failure) => failure.workloadName),
      ),
    ];
    const shown = names.slice(0, FAILED_NAMES);
    const more = names.length - shown.length;
    return more > 0 ? `${shown.join(', ')} and ${more} more` : shown.join(', ');
  };
  const unavailable = () => props.jobs.historyState === 'unavailable';

  return (
    <TableCard class="flex h-full flex-col" data-backup-jobs={props.jobs.historyState}>
      <TableCardHeader
        title={<span class="whitespace-nowrap">Backup jobs</span>}
        actions={
          <Show
            when={!unavailable()}
            fallback={<span class="text-[11px] text-muted">History unavailable</span>}
          >
            <span
              class="inline-flex flex-wrap items-center justify-end gap-1.5"
              data-backup-jobs-last24h
            >
              {/* The window is named at every width; phones get the short form. */}
              <span class="text-[10px] font-medium text-muted">
                <span class="sm:hidden">24h</span>
                <span class="hidden sm:inline">Last 24h</span>
              </span>
              <For each={pills()}>
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
              <Show when={last().historyState !== 'complete'}>
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
      <div class="flex flex-1 flex-col px-3 pb-2 pt-3">
        <ol
          class="flex min-h-14 flex-1 items-end gap-1"
          aria-label={`Backups per day, last ${props.jobs.days.length} days`}
        >
          <For each={props.jobs.days}>
            {(day) => {
              const total = () => backupJobsTotal(day.counts);
              const observed = () => backupJobsDayObserved(day);
              return (
                <li
                  class={`flex h-full min-w-0 flex-1 flex-col-reverse rounded-xs ${
                    observed() ? '' : 'border border-dashed border-border'
                  }`}
                  title={dayTitle(day)}
                  data-backup-jobs-day={observed() ? 'observed' : day.historyState}
                >
                  <span class="sr-only">{dayTitle(day)}</span>
                  <Show
                    when={total() > 0}
                    fallback={
                      <Show when={observed()}>
                        <span class="h-0.5 w-full rounded-xs bg-surface-hover" aria-hidden="true" />
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
                </li>
              );
            }}
          </For>
        </ol>
        <div class="mt-1 flex justify-between text-[10px] text-muted" aria-hidden="true">
          <span>{getRecoveryPrettyDateLabel(props.jobs.days[0]?.date ?? '')}</span>
          <span>Today</span>
        </div>
      </div>
    </TableCard>
  );
};

const BACKUP_JOBS_POLL_MS = 60_000;

// Owns the backup jobs read. Renders nothing until history exists, so a page
// without PVE backup jobs keeps Backup health alone at full width.
export const ProxmoxBackupJobsSection: Component = () => {
  const query = createNonSuspendingQuery({
    source: () => 'jobs',
    fetcher: (_source, signal) => fetchBackupJobs(signal),
    initialValue: null as BackupJobsPayload | null,
    pollMs: BACKUP_JOBS_POLL_MS,
  });
  // The last payload that was worth showing; a later failed read keeps the
  // card as unavailable rather than making history disappear.
  let lastShown: BackupJobsPayload | null = null;
  const shown = createMemo((): BackupJobsPayload | null => {
    if (query.error()) return lastShown ? backupJobsUnavailable(lastShown) : null;
    const jobs = query.value();
    const visible = jobs && jobs.days.length > 0 && backupJobsHasHistory(jobs) ? jobs : null;
    if (visible) lastShown = visible;
    return visible;
  });
  return <Show when={shown()}>{(jobs) => <ProxmoxBackupJobsCard jobs={jobs()} />}</Show>;
};
