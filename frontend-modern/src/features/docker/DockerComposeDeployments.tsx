import { A } from '@solidjs/router';
import { For, Show, createEffect, createSignal, onCleanup } from 'solid-js';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { Button } from '@/components/shared/Button';
import { buildActionReviewPath } from '@/features/actions/actionRouting';
import { eventBus } from '@/stores/events';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { getOrgID } from '@/utils/apiClient';
import { formatAbsoluteTime } from '@/utils/format';

const imageLabels = {
  waiting: 'Waiting for fresh container and image inventory',
  different: 'The service runs a different image digest',
  not_running: 'The service container is stopped',
  matched: 'The running image matches the reviewed digest',
};
const endpointLabels = {
  waiting: 'Waiting for a fresh HTTP check on the reviewed deployment',
  unconfigured: 'No linked HTTP check. Add one under Service checks.',
  failed: 'An HTTP check failed on the reviewed deployment',
  passed: 'The linked HTTP checks passed on the reviewed deployment',
};

export function DockerComposeDeployments(props: { runtime: string; project: string }) {
  const [open, setOpen] = createSignal(false);
  const [revision, setRevision] = createSignal(0);
  const [records, setRecords] = createSignal<ActionAuditRecord[]>([]);
  const [error, setError] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const refresh = () => setRevision((n) => n + 1);
  const offOrg = eventBus.on('org_switched', refresh);
  const offMembership = eventBus.on('organizations_changed', refresh);
  onCleanup(() => {
    offOrg();
    offMembership();
  });
  createEffect(() => {
    const runtime = props.runtime,
      project = props.project;
    revision();
    setRecords([]);
    setError('');
    setLoading(false);
    if (!open()) return;
    const controller = new AbortController();
    onCleanup(() => controller.abort());
    setLoading(true);
    void ResourceActionsAPI.listComposeDeployments(
      runtime,
      project,
      getOrgID() || 'default',
      controller.signal,
    )
      .then((response) => {
        if (!controller.signal.aborted) setRecords(response.actions);
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setError('Could not read deployment history for this application.');
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
  });
  return (
    <details
      class="rounded-lg border border-border bg-surface p-4 text-sm"
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary class="cursor-pointer font-semibold">Service deployments</summary>
      <div class="mt-2">
        <Button size="mdCompact" onClick={refresh} disabled={loading()}>
          Refresh observations
        </Button>
      </div>
      <Show when={loading()}>
        <p role="status">Reading deployment history…</p>
      </Show>
      <Show when={error()}>
        <p role="alert">{error()}</p>
      </Show>
      <Show when={!loading() && !error()}>
        <ol class="divide-y divide-border">
          <For
            each={records()}
            fallback={
              <li class="mt-2 text-muted">No reviewed service updates for this application.</li>
            }
          >
            {(record) => (
              <li class="space-y-1 py-3">
                <A
                  class="font-medium text-primary hover:underline"
                  href={buildActionReviewPath(record.id)}
                >
                  {String(record.request.params?.composeService || 'Service update')} ·{' '}
                  {record.state.replaceAll('_', ' ')}
                </A>
                <time class="ml-2 text-xs text-muted" dateTime={record.updatedAt}>
                  {formatAbsoluteTime(Date.parse(record.updatedAt))}
                </time>
                <p class="break-all">{String(record.request.params?.afterImage || '')}</p>
                <Show when={record.result?.actionResultV2?.execution.summary}>
                  {(summary) => <p>{summary()}</p>}
                </Show>
                <Show when={record.composeObservation}>
                  {(observation) => (
                    <div>
                      <p>{imageLabels[observation().image]}</p>
                      <p>{endpointLabels[observation().endpoint]}</p>
                    </div>
                  )}
                </Show>
                <Show when={record.result?.actionResultV2?.compensation.summary}>
                  {(summary) => (
                    <details>
                      <summary class="cursor-pointer">Recovery</summary>
                      <p>{summary()}</p>
                    </details>
                  )}
                </Show>
              </li>
            )}
          </For>
        </ol>
      </Show>
    </details>
  );
}
