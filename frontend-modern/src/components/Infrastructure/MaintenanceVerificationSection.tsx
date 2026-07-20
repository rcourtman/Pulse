import { Component, For, Show, createMemo, createSignal } from 'solid-js';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { notificationStore } from '@/stores/notifications';
import {
  listMaintenanceVerificationsForResource,
  rerunMaintenanceVerification,
  reviewMaintenanceVerification,
  type MaintenanceVerificationReport,
  type MaintenanceVerificationStatus,
} from '@/api/maintenanceVerification';
import { formatRelativeTime } from '@/utils/format';

/**
 * MaintenanceVerificationSection surfaces Maintenance Verification
 * Reports in the resource detail drawer. Reports are durable records
 * the sentinel writes when a maintenance window ends for a resource,
 * summarizing whether the resource recovered cleanly.
 *
 * The section is empty (and hidden) until the first report is
 * written. Operators can:
 *   - Read the deterministic evidence (alerts, findings, failed
 *     actions, basic metric recovery summary)
 *   - Mark a report as reviewed (without changing the underlying
 *     status / evidence — those stay immutable)
 *   - Rerun verification now (writes a new -rerun-N report)
 *
 * The "open Assistant with this report context" action is intentionally
 * omitted from this first pass — see Status: missing-actions note at
 * the bottom. The other two actions (mark reviewed, rerun verification)
 * are sufficient for the MVP review loop.
 */
interface MaintenanceVerificationSectionProps {
  resourceId: string;
}

const STATUS_LABELS: Record<MaintenanceVerificationStatus, string> = {
  pending: 'Pending',
  healthy: 'Healthy',
  needs_review: 'Needs review',
  failed_verification: 'Failed verification',
};

const STATUS_CLASSES: Record<MaintenanceVerificationStatus, string> = {
  pending: 'bg-surface-hover text-base-content',
  healthy: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
  needs_review: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  failed_verification: 'bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300',
};

export const MaintenanceVerificationSection: Component<MaintenanceVerificationSectionProps> = (
  props,
) => {
  const [refreshTick, setRefreshTick] = createSignal(0);

  const query = createNonSuspendingQuery<MaintenanceVerificationReport[], string>({
    source: () => (props.resourceId ? `${props.resourceId}:${refreshTick()}` : null),
    fetcher: async () => {
      try {
        const res = await listMaintenanceVerificationsForResource(props.resourceId);
        return res.data ?? [];
      } catch (err) {
        notificationStore.error(
          err instanceof Error ? err.message : 'Failed to load Maintenance Verification Reports',
        );
        return [];
      }
    },
    initialValue: [] as MaintenanceVerificationReport[],
    cacheKey: (key: string) => `maintenance-verifications:${key}`,
  });

  const reports = createMemo<MaintenanceVerificationReport[]>(() => query.value() ?? []);
  const hasReports = createMemo(() => reports().length > 0);

  const [rerunning, setRerunning] = createSignal(false);
  const [reviewingId, setReviewingId] = createSignal<string | null>(null);

  const refresh = () => setRefreshTick((n) => n + 1);

  const handleRerun = async () => {
    if (rerunning() || !props.resourceId) return;
    setRerunning(true);
    try {
      await rerunMaintenanceVerification(props.resourceId);
      notificationStore.success('Maintenance verification rerun complete');
      refresh();
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to rerun maintenance verification',
      );
    } finally {
      setRerunning(false);
    }
  };

  const handleReview = async (report: MaintenanceVerificationReport) => {
    if (reviewingId() || !report.id) return;
    setReviewingId(report.id);
    try {
      await reviewMaintenanceVerification(report.id);
      notificationStore.success('Marked report reviewed');
      refresh();
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : 'Failed to mark report reviewed',
      );
    } finally {
      setReviewingId(null);
    }
  };

  return (
    <section
      class="rounded border border-border bg-surface p-3 text-sm"
      data-testid="maintenance-verification-section"
      aria-labelledby="maintenance-verification-heading"
    >
      <header class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 id="maintenance-verification-heading" class="text-sm font-semibold text-base-content">
            Maintenance verification
          </h3>
          <p class="text-xs text-muted">
            Pulse runs deterministic checks each time a maintenance window ends and writes a
            Maintenance Verification Report. The result sticks here for review.
          </p>
        </div>
        <button
          type="button"
          class="rounded border border-border bg-surface-hover px-2 py-1 text-xs font-medium hover:bg-surface"
          disabled={rerunning() || !props.resourceId}
          onClick={handleRerun}
          data-testid="maintenance-verification-rerun"
        >
          {rerunning() ? 'Rerunning…' : 'Rerun verification'}
        </button>
      </header>

      <Show
        when={hasReports()}
        fallback={
          <div class="mt-3 rounded border border-dashed border-border bg-surface-hover p-3 text-xs text-muted">
            No verification reports yet. A report is written automatically the next time this
            resource exits a maintenance window.
          </div>
        }
      >
        <ul class="mt-3 space-y-2">
          <For each={reports()}>
            {(report) => (
              <li
                class="rounded border border-border bg-surface-hover p-2"
                data-testid="maintenance-verification-report"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      class={`inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium ${STATUS_CLASSES[report.status]}`}
                      data-testid={`maintenance-verification-status-${report.status}`}
                    >
                      {STATUS_LABELS[report.status]}
                    </span>
                    <span class="text-xs text-muted">
                      Window ended{' '}
                      {report.windowEndedAt ? formatRelativeTime(report.windowEndedAt) : 'unknown'}
                    </span>
                  </div>
                  <Show when={!report.userOutcome}>
                    <button
                      type="button"
                      class="rounded border border-border bg-surface px-2 py-1 text-[11px] font-medium hover:bg-surface-hover"
                      disabled={reviewingId() === report.id}
                      onClick={() => handleReview(report)}
                      data-testid="maintenance-verification-review"
                    >
                      {reviewingId() === report.id ? 'Marking…' : 'Mark reviewed'}
                    </button>
                  </Show>
                  <Show when={report.userOutcome === 'reviewed'}>
                    <span class="text-[11px] text-muted">
                      Reviewed {report.reviewedAt ? formatRelativeTime(report.reviewedAt) : ''}
                      {report.reviewedBy ? ` by ${report.reviewedBy}` : ''}
                    </span>
                  </Show>
                </div>

                <Show when={report.recommendation}>
                  <p class="mt-1 text-xs text-base-content">{report.recommendation}</p>
                </Show>

                <dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-[11px] text-muted sm:grid-cols-4">
                  <div>
                    <dt class="font-medium text-base-content">Critical alerts</dt>
                    <dd>{report.evidence.activeCriticalAlerts}</dd>
                  </div>
                  <div>
                    <dt class="font-medium text-base-content">Warning alerts</dt>
                    <dd>{report.evidence.activeWarningAlerts}</dd>
                  </div>
                  <div>
                    <dt class="font-medium text-base-content">Critical findings</dt>
                    <dd>{report.evidence.activeCriticalFindings}</dd>
                  </div>
                  <div>
                    <dt class="font-medium text-base-content">Warning findings</dt>
                    <dd>{report.evidence.activeWarningFindings}</dd>
                  </div>
                  <div>
                    <dt class="font-medium text-base-content">Failed actions</dt>
                    <dd>{report.evidence.failedActionsSinceWindowStart}</dd>
                  </div>
                  <Show when={report.evidence.metricRecovery}>
                    {(rec) => (
                      <>
                        <div>
                          <dt class="font-medium text-base-content">Metric samples</dt>
                          <dd>{rec().samplesAfterEnd}</dd>
                        </div>
                        <div>
                          <dt class="font-medium text-base-content">Metric trend</dt>
                          <dd>{rec().trend || 'unknown'}</dd>
                        </div>
                      </>
                    )}
                  </Show>
                </dl>

                <Show when={report.evidence.operatorStateSummary}>
                  <p class="mt-1 text-[11px] text-muted">{report.evidence.operatorStateSummary}</p>
                </Show>

                <Show when={report.evidence.patrolRunTodo}>
                  <p class="mt-1 text-[11px] italic text-muted">{report.evidence.patrolRunTodo}</p>
                </Show>

                <Show when={report.userOutcome === 'reviewed' && report.reviewNote}>
                  <p class="mt-1 text-[11px] text-base-content">
                    Reviewer note: {report.reviewNote}
                  </p>
                </Show>
              </li>
            )}
          </For>
        </ul>
      </Show>

      {/*
        MVP omission: "Open Assistant with this report context" is the
        third action called out in the product brief. Wiring it requires
        a stable Assistant deep-link contract for report payloads which
        is not yet implemented — left as a follow-up so the surface
        does not ship a button that does nothing.
      */}
    </section>
  );
};

export default MaintenanceVerificationSection;
