import {
  batch,
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  For,
  onCleanup,
  onMount,
  Show,
  untrack,
} from 'solid-js';
import { createStore, reconcile, unwrap } from 'solid-js/store';
import {
  NotificationsAPI,
  type DeliveryRehearsal,
  type RehearsalOutcome,
  type RehearsalStatus,
} from '@/api/notifications';
import { Button } from '@/components/shared/Button';
import { SettingsPanel } from '@/components/shared/SettingsPanel';
import { getOrgID } from '@/utils/apiClient';

const outcomeLabels: Record<RehearsalStatus, string> = {
  not_started: 'Not started',
  pending: 'Pending',
  deferred: 'Deferred',
  passed: 'Passed',
  failed: 'Failed',
  suppressed: 'Suppressed',
  unavailable: 'Unavailable',
};
const outcomeColors: Record<RehearsalStatus, string> = {
  not_started: 'text-muted',
  pending: 'text-blue-700 dark:text-blue-300',
  deferred: 'text-amber-800 dark:text-amber-200',
  passed: 'text-emerald-700 dark:text-emerald-300',
  failed: 'text-red-700 dark:text-red-300',
  suppressed: 'text-amber-800 dark:text-amber-200',
  unavailable: 'text-red-700 dark:text-red-300',
};
function Outcome(props: { title: string; outcome: RehearsalOutcome }) {
  return (
    <div class="min-w-0 space-y-1 rounded-md border border-border p-3">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <h4 class="font-medium">{props.title}</h4>
        <span class={`text-sm font-semibold ${outcomeColors[props.outcome.status]}`}>
          {outcomeLabels[props.outcome.status]}
        </span>
      </div>
      <p class="break-words text-sm text-muted">{props.outcome.message}</p>
      <Show when={props.outcome.nextAt}>
        <p class="text-xs text-muted">
          Next eligible check: {new Date(props.outcome.nextAt!).toLocaleString()}
        </p>
      </Show>
      <Show when={props.outcome.attempts > 0}>
        <p class="text-xs text-muted">Recorded attempts: {props.outcome.attempts}</p>
      </Show>
      <Show when={props.outcome.diagnostic || props.outcome.failureClass}>
        <details class="text-xs text-muted">
          <summary class="cursor-pointer rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2">
            Delivery details
          </summary>
          <Show when={props.outcome.failureClass}>
            <p class="mt-2">Failure category: {props.outcome.failureClass!.replaceAll('_', ' ')}</p>
          </Show>
          <Show when={props.outcome.diagnostic}>
            <p class="mt-2 break-words">{props.outcome.diagnostic}</p>
          </Show>
        </details>
      </Show>
    </div>
  );
}

export function AlertDeliveryRehearsal(props: {
  hasUnsavedChanges?: () => boolean;
  requestedRehearsalID?: () => string | undefined;
}) {
  const inputID = `delivery-rehearsal-${createUniqueId()}`;
  const org = getOrgID() || 'default';
  const requestOptions = { expectedOrgID: org, retry: false } as const;
  const [historyRows, setHistoryRows] = createStore<DeliveryRehearsal[]>([]);
  const history = () => historyRows;
  const setHistory = (rows: DeliveryRehearsal[]) => setHistoryRows(reconcile(rows));
  const [selection, setSelection] = createStore<{ run: DeliveryRehearsal | null }>({ run: null });
  const selected = () => selection.run;
  const setSelected = (run: DeliveryRehearsal | null) => {
    // A selected report must not mutate a history row when its identity changes.
    // Reconcile only within one run so its native disclosures survive polling.
    const snapshot = run ? structuredClone(unwrap(run)) : null;
    batch(() => {
      if (snapshot?.id !== selection.run?.id) setSelection('run', null);
      setSelection('run', reconcile(snapshot));
    });
  };
  const [level, setLevel] = createSignal<DeliveryRehearsal['level']>('warning');
  const [tags, setTags] = createSignal('');
  const [loading, setLoading] = createSignal(true);
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  const [unavailable, setUnavailable] = createSignal(false);
  const [uncertainStart, setUncertainStart] = createSignal(false);
  const [expanded, setExpanded] = createSignal(false);
  let linkedRequestID: string | undefined;
  let lastRequestedID: string | undefined;
  let summaryElement: HTMLElement | undefined;
  let reportHeading: HTMLHeadingElement | undefined;
  let focusedLinkID: string | undefined;
  let pendingRequest: { id: string; level: DeliveryRehearsal['level']; tags: string[] } | undefined;
  let disposed = false;
  let readSequence = 0;
  let selectionVersion = 0;
  const current = () => !disposed && (getOrgID() || 'default') === org;
  const active = createMemo(() => history().find((run) => !run.recoveredAt));
  const focusLinkedResult = (id: string, version: number) => {
    if (focusedLinkID === id) return;
    queueMicrotask(() => {
      if (!current() || version !== selectionVersion) return;
      (reportHeading?.isConnected ? reportHeading : summaryElement)?.focus();
      focusedLinkID = id;
    });
  };
  const selectedOutsideHistory = () =>
    selected() && !history().some((row) => row.id === selected()!.id);
  const mergeRun = (run: DeliveryRehearsal) => {
    setHistory(
      [run, ...history().filter((row) => row.id !== run.id)]
        .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
        .slice(0, 20),
    );
    setSelected(run);
  };
  const refresh = async () => {
    if (!current()) return;
    const sequence = ++readSequence;
    const version = selectionVersion;
    const requestedLink = linkedRequestID;
    const wantedID = linkedRequestID || (!pendingRequest ? selected()?.id : undefined);
    try {
      const runs = await NotificationsAPI.listRehearsals(requestOptions);
      if (!current() || sequence !== readSequence) return;
      let wanted = runs.find((run) => run.id === wantedID);
      if (wantedID && !wanted)
        wanted = await NotificationsAPI.getRehearsal(wantedID, requestOptions);
      if (!current() || sequence !== readSequence) return;
      setHistory(runs);
      if (unavailable() && !uncertainStart()) setError('');
      setUnavailable(false);
      const reconciled = pendingRequest && runs.find((run) => run.id === pendingRequest!.id);
      if (reconciled) {
        pendingRequest = undefined;
        setUncertainStart(false);
        mergeRun(reconciled);
        setError('');
      } else if (version === selectionVersion) {
        setSelected(wanted ?? runs[0] ?? null);
        linkedRequestID = undefined;
        if (requestedLink) focusLinkedResult(requestedLink, version);
      }
    } catch {
      if (current() && sequence === readSequence) {
        setUnavailable(true);
        setError('Rehearsal evidence is unavailable. Refresh before taking another action.');
        if (requestedLink) focusLinkedResult(requestedLink, version);
      }
    } finally {
      if (current() && sequence === readSequence) setLoading(false);
    }
  };
  const start = async () => {
    if (!current() || busy() || unavailable() || props.hasUnsavedChanges?.() || active()) return;
    const routingTags = tags()
      .split(',')
      .map((tag) => tag.trim())
      .filter(Boolean);
    if (
      !pendingRequest &&
      (routingTags.length > 32 ||
        routingTags.some((tag) => new TextEncoder().encode(tag).length > 64))
    ) {
      setError('Use at most 32 routing tags, each no longer than 64 bytes.');
      return;
    }
    pendingRequest ??= { id: crypto.randomUUID(), level: level(), tags: routingTags };
    setBusy(true);
    setError('');
    ++readSequence;
    try {
      const run = await NotificationsAPI.startRehearsal(pendingRequest, requestOptions);
      if (!current()) return;
      pendingRequest = undefined;
      setUncertainStart(false);
      ++selectionVersion;
      mergeRun(run);
      setUnavailable(false);
    } catch {
      if (current()) {
        setUncertainStart(true);
        setError(
          'The request could not be confirmed. Refresh to look for its recorded result. Retrying this same request will not create a second rehearsal.',
        );
      }
    } finally {
      if (current()) setBusy(false);
    }
  };
  const recover = async () => {
    const run = selected();
    if (
      !current() ||
      !run ||
      run.recoveredAt ||
      busy() ||
      unavailable() ||
      props.hasUnsavedChanges?.()
    )
      return;
    setBusy(true);
    setError('');
    ++readSequence;
    try {
      const result = await NotificationsAPI.recoverRehearsal(run.id, requestOptions);
      if (!current()) return;
      ++selectionVersion;
      mergeRun(result);
    } catch {
      if (current()) {
        setUnavailable(true);
        setError(
          'The recovery request could not be confirmed. Refresh to reconcile its recorded state before trying again.',
        );
      }
    } finally {
      if (current()) setBusy(false);
    }
  };
  onMount(() => {
    void refresh();
    const timer = setInterval(() => {
      if (expanded() && !busy() && document.visibilityState === 'visible') void refresh();
    }, 5000);
    onCleanup(() => clearInterval(timer));
  });
  createEffect(() => {
    const id = props.requestedRehearsalID?.();
    if (id === lastRequestedID) return;
    lastRequestedID = id;
    linkedRequestID = id;
    focusedLinkID = undefined;
    if (!id) return;
    ++selectionVersion;
    setSelected(null);
    setLoading(true);
    setExpanded(true);
    void untrack(refresh);
  });
  onCleanup(() => {
    disposed = true;
    ++readSequence;
  });

  return (
    <SettingsPanel
      title="Delivery rehearsal"
      description="Exercise your saved notification routes, queue and recovery."
    >
      <details
        class="group"
        data-testid="delivery-rehearsal"
        open={expanded()}
        onToggle={(event) => {
          const wasExpanded = expanded();
          setExpanded(event.currentTarget.open);
          if (event.currentTarget.open && !wasExpanded) void refresh();
        }}
      >
        <summary
          ref={summaryElement}
          class="cursor-pointer rounded-md text-sm font-medium text-link focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-4"
        >
          Rehearse alert delivery
        </summary>
        <div class="mt-4 space-y-4">
          <p class="text-sm text-muted">
            Sends clearly labelled messages to saved email, webhook and Apprise destinations using
            real routing, grouping, quiet hours, retries and recovery receipts. It creates no
            infrastructure incident.
          </p>
          <p class="text-sm text-muted">
            This sample follows the performance-alert quiet-hours category. Critical severity
            follows your saved setting for suppressing critical performance alerts.
          </p>
          <p class="text-sm text-muted">
            A passed delivery means the destination accepted it. Check your inbox or service to
            confirm receipt. Metric detection, resource maintenance, push notifications and the
            external watchdog are separate checks.
          </p>
          <Show when={props.hasUnsavedChanges?.()}>
            <p role="status" class="text-sm text-amber-800 dark:text-amber-200">
              Save or discard your settings changes before rehearsing. Only saved settings are used.
            </p>
          </Show>
          <Show when={error()}>
            <p role="alert" class="break-words text-sm text-red-700 dark:text-red-300">
              {error()}
            </p>
          </Show>
          <Show when={unavailable() && !!linkedRequestID}>
            <Button
              onClick={() => {
                linkedRequestID = undefined;
                ++selectionVersion;
                setSelected(null);
                void refresh();
              }}
            >
              Show recent rehearsals
            </Button>
          </Show>
          <div class="flex flex-wrap items-end gap-3">
            <label class="min-w-36 flex-1 space-y-1 text-sm" for={`${inputID}-level`}>
              <span>Rehearsal severity</span>
              <select
                id={`${inputID}-level`}
                class="min-h-11 w-full rounded-md border border-border bg-surface px-3 text-base-content"
                value={level()}
                disabled={busy() || !!active() || uncertainStart()}
                onChange={(event) =>
                  setLevel(event.currentTarget.value as DeliveryRehearsal['level'])
                }
              >
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </select>
            </label>
            <label class="min-w-44 flex-[2] space-y-1 text-sm" for={`${inputID}-tags`}>
              <span>Routing tags (optional, comma-separated)</span>
              <input
                id={`${inputID}-tags`}
                class="min-h-11 w-full rounded-md border border-border bg-surface px-3 text-base-content"
                value={tags()}
                maxLength={2080}
                disabled={busy() || !!active() || uncertainStart()}
                onInput={(event) => setTags(event.currentTarget.value)}
              />
            </label>
          </div>
          <div class="flex flex-wrap gap-2">
            <Button
              variant="primary"
              isLoading={busy()}
              disabled={loading() || unavailable() || !!active() || !!props.hasUnsavedChanges?.()}
              onClick={() => void start()}
            >
              {uncertainStart() ? 'Retry same rehearsal request' : 'Send rehearsal alert'}
            </Button>
            <Button disabled={busy()} onClick={() => void refresh()}>
              Refresh rehearsal
            </Button>
            <Show when={active() && selected()?.id !== active()?.id}>
              <Button
                onClick={() => {
                  linkedRequestID = undefined;
                  ++selectionVersion;
                  setSelected(active()!);
                }}
              >
                Resume current rehearsal
              </Button>
            </Show>
          </div>
          <Show when={loading()}>
            <p role="status" class="text-sm text-muted">
              Loading rehearsal evidence…
            </p>
          </Show>
          <Show when={!loading() && !selected() && !unavailable()}>
            <p class="text-sm text-muted">
              No rehearsal in the last seven days. Start with a warning to check your saved routes.
            </p>
          </Show>
          <Show when={history().length > 1 || selectedOutsideHistory()}>
            <label class="block space-y-1 text-sm" for={`${inputID}-history`}>
              <span>
                {selectedOutsideHistory()
                  ? 'Recent rehearsals and linked result'
                  : 'Recent rehearsals (up to 20, retained for seven days)'}
              </span>
              <select
                id={`${inputID}-history`}
                class="min-h-11 w-full rounded-md border border-border bg-surface px-3 text-base-content"
                value={selected()?.id}
                disabled={busy()}
                onChange={(event) => {
                  linkedRequestID = undefined;
                  ++selectionVersion;
                  setSelected(
                    history().find((row) => row.id === event.currentTarget.value) ?? null,
                  );
                }}
              >
                <Show when={selectedOutsideHistory()}>
                  <option value={selected()!.id}>
                    Linked rehearsal · {new Date(selected()!.createdAt).toLocaleString()}
                  </option>
                </Show>
                <For each={history()}>
                  {(run) => (
                    <option value={run.id}>
                      {new Date(run.createdAt).toLocaleString()} · {run.level}
                      {run.recoveredAt ? ' · finished' : ' · current'}
                    </option>
                  )}
                </For>
              </select>
            </label>
          </Show>
          <Show when={selected()}>
            {(run) => (
              <div class="space-y-4" aria-live="polite" aria-busy={busy()}>
                <div class="space-y-1">
                  <h3 ref={reportHeading} tabIndex={-1} class="font-semibold">
                    Rehearsal started {new Date(run().createdAt).toLocaleString()}
                  </h3>
                  <p class="break-words text-xs text-muted">
                    {run().level} · {run().tags.length ? run().tags.join(', ') : 'No routing tags'}
                  </p>
                </div>
                <Show when={unavailable()}>
                  <p class="text-sm text-red-700 dark:text-red-300">
                    The results below are from the last successful read. Current outcomes are
                    unknown.
                  </p>
                </Show>
                <Outcome title="Notification admission" outcome={run().policy} />
                <Show when={run().destinations.length === 0}>
                  <p class="text-sm text-amber-800 dark:text-amber-200">
                    No saved destinations were available. Configure and save a destination for your
                    next rehearsal.
                  </p>
                </Show>
                <For each={run().destinations}>
                  {(destination) => (
                    <section class="min-w-0 space-y-2">
                      <h3 class="break-words font-semibold">{destination.name}</h3>
                      <div class="grid min-w-0 gap-2 md:grid-cols-2">
                        <Outcome title="Firing notification" outcome={destination.firing} />
                        <Outcome title="Recovery notification" outcome={destination.recovery} />
                      </div>
                    </section>
                  )}
                </For>
                <Show
                  when={!run().recoveredAt}
                  fallback={
                    <p class="text-sm text-muted">
                      Recovery was requested {new Date(run().recoveredAt!).toLocaleString()}.
                      Pending deliveries continue under the saved queue policy.
                    </p>
                  }
                >
                  <div class="space-y-2 border-t border-border pt-4">
                    <p class="text-sm text-muted">
                      When ready, finish this rehearsal. Pending firing messages will be cancelled.
                      Recovery is sent only where this occurrence has a successful firing receipt
                      and recovery notifications are enabled.
                    </p>
                    <Button
                      disabled={busy() || unavailable() || !!props.hasUnsavedChanges?.()}
                      onClick={() => void recover()}
                    >
                      Send recovery and finish rehearsal
                    </Button>
                  </div>
                </Show>
                <p class="text-xs text-muted">
                  Use Recent delivery activity for individual attempts. These messages share
                  ordinary destination rate limits, queue policy and delivery health.
                </p>
              </div>
            )}
          </Show>
        </div>
      </details>
    </SettingsPanel>
  );
}
