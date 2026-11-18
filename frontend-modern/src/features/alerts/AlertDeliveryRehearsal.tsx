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
import { CalloutCard } from '@/components/shared/CalloutCard';
import { StatusIndicatorBadge } from '@/components/shared/StatusIndicatorBadge';
import { TechnicalDetailsDisclosure } from '@/components/shared/TechnicalDetailsDisclosure';
import type { StatusIndicatorVariant } from '@/utils/status';
import { FormSelect } from '@/components/shared/FormSelect';
import { formControl, formField, formLabel } from '@/components/shared/Form';
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
const outcomeVariants: Record<RehearsalStatus, StatusIndicatorVariant> = {
  not_started: 'muted',
  pending: 'info',
  deferred: 'info',
  passed: 'success',
  failed: 'danger',
  suppressed: 'muted',
  unavailable: 'danger',
};

function OutcomeBadge(props: { outcome: RehearsalOutcome }) {
  return (
    <StatusIndicatorBadge
      variant={outcomeVariants[props.outcome.status]}
      label={outcomeLabels[props.outcome.status]}
      title={props.outcome.message}
      dot
    />
  );
}

function OutcomeDetails(props: {
  title: string;
  outcome: RehearsalOutcome;
  hideMessage?: boolean;
}) {
  return (
    <div class="min-w-0 space-y-2">
      <h5 class="text-sm font-medium text-base-content">{props.title}</h5>
      <Show when={!props.hideMessage}>
        <p class="break-words text-sm text-muted">{props.outcome.message}</p>
      </Show>
      <dl class="space-y-1 text-xs text-muted">
        <Show when={props.outcome.attempts > 0}>
          <div class="flex gap-2">
            <dt>Attempts</dt>
            <dd>{props.outcome.attempts}</dd>
          </div>
        </Show>
        <Show when={props.outcome.nextAt}>
          <div>
            <dt class="inline">Next eligible check: </dt>
            <dd class="inline">{new Date(props.outcome.nextAt!).toLocaleString()}</dd>
          </div>
        </Show>
        <Show when={props.outcome.failureClass}>
          <div>
            <dt class="inline">Failure category: </dt>
            <dd class="inline">{props.outcome.failureClass!.replaceAll('_', ' ')}</dd>
          </div>
        </Show>
      </dl>
      <Show when={props.outcome.diagnostic}>
        <p class="break-words rounded-sm bg-surface-alt p-2 text-xs leading-relaxed text-muted">
          {props.outcome.diagnostic}
        </p>
      </Show>
    </div>
  );
}

function DestinationResult(props: {
  destination: DeliveryRehearsal['destinations'][number];
  policy: RehearsalOutcome;
  id: string;
}) {
  const note = () => {
    const { firing, recovery } = props.destination;
    if (
      firing.status !== 'passed' &&
      firing.status !== 'not_started' &&
      firing.message !== props.policy.message
    )
      return { phase: 'firing', label: 'Alert', outcome: firing };
    if (
      recovery.status !== 'passed' &&
      recovery.status !== 'not_started' &&
      recovery.message !== props.policy.message
    )
      return { phase: 'recovery', label: 'Recovery', outcome: recovery };
    return undefined;
  };
  return (
    <li class="min-w-0 py-4" aria-labelledby={props.id}>
      <div class="grid min-w-0 grid-cols-2 items-start gap-x-4 gap-y-2 md:grid-cols-[minmax(0,1fr)_7rem_7rem]">
        <div class="col-span-2 min-w-0 md:col-span-1">
          <h4 id={props.id} class="break-words text-sm font-medium text-base-content">
            {props.destination.name}
          </h4>
          <Show when={note()}>
            {(reason) => (
              <p class="mt-1 break-words text-xs leading-relaxed text-muted">
                <span class="font-medium">{reason().label}: </span>
                {reason().outcome.message}
              </p>
            )}
          </Show>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs text-muted md:sr-only">Alert</span>
          <OutcomeBadge outcome={props.destination.firing} />
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-xs text-muted md:sr-only">Recovery</span>
          <OutcomeBadge outcome={props.destination.recovery} />
        </div>
      </div>
      <TechnicalDetailsDisclosure
        title="Delivery details"
        subtitle=""
        dataTestId={`${props.id}-details`}
        class="mt-2 text-muted"
        contentClass="mt-3 grid min-w-0 gap-4 border-t border-border pt-3 md:grid-cols-2"
        keepMounted
      >
        <OutcomeDetails
          title="Firing notification"
          outcome={props.destination.firing}
          hideMessage={note()?.phase === 'firing'}
        />
        <OutcomeDetails
          title="Recovery notification"
          outcome={props.destination.recovery}
          hideMessage={note()?.phase === 'recovery'}
        />
      </TechnicalDetailsDisclosure>
    </li>
  );
}

function firingSummary(run: DeliveryRehearsal) {
  return (
    ['passed', 'failed', 'pending', 'deferred', 'suppressed', 'unavailable', 'not_started'] as const
  )
    .map((status) => ({
      status,
      count: run.destinations.filter((destination) => destination.firing.status === status).length,
    }))
    .filter(({ count }) => count > 0)
    .map(({ status, count }) => `${count} ${outcomeLabels[status].toLowerCase()}`)
    .join(' · ');
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
  const focusReport = (id: string, version: number, force = false) => {
    if (!force && focusedLinkID === id) return;
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
        focusReport(reconciled.id, selectionVersion);
      } else if (version === selectionVersion) {
        setSelected(wanted ?? runs[0] ?? null);
        linkedRequestID = undefined;
        if (requestedLink) focusReport(requestedLink, version);
      }
    } catch {
      if (current() && sequence === readSequence) {
        setUnavailable(true);
        setError('Rehearsal evidence is unavailable. Refresh before taking another action.');
        if (requestedLink) focusReport(requestedLink, version);
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
      focusReport(run.id, selectionVersion);
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
      focusReport(result.id, selectionVersion, true);
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
      description="Check a sample alert and recovery through your saved destinations."
      bodyClass="space-y-4"
      action={
        <Button
          size="sm"
          aria-label="Refresh rehearsal"
          disabled={busy()}
          onClick={() => void refresh()}
        >
          Refresh
        </Button>
      }
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
        <div class="mt-4 space-y-5">
          <Show when={props.hasUnsavedChanges?.()}>
            <CalloutCard
              tone="warning"
              scale="compact"
              padding="sm"
              role="status"
              description="Save or discard your settings changes before rehearsing. Only saved settings are used."
            />
          </Show>
          <Show when={error()}>
            <CalloutCard
              tone="danger"
              scale="compact"
              padding="sm"
              role="alert"
              description={error()}
            />
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
          <Show when={!active() || uncertainStart()}>
            <div class="flex flex-wrap items-end gap-3">
              <FormSelect
                id={`${inputID}-level`}
                label="Rehearsal severity"
                fieldClass="min-w-32 flex-1 sm:max-w-40"
                value={level()}
                disabled={busy() || !!active() || uncertainStart()}
                onChange={(event) =>
                  setLevel(event.currentTarget.value as DeliveryRehearsal['level'])
                }
              >
                <option value="info">Info</option>
                <option value="warning">Warning</option>
                <option value="critical">Critical</option>
              </FormSelect>
              <label class={`${formField} min-w-40 flex-[2]`} for={`${inputID}-tags`}>
                <span class={formLabel}>
                  Routing tags{' '}
                  <span class="text-xs font-normal text-muted">optional, comma-separated</span>
                </span>
                <input
                  id={`${inputID}-tags`}
                  class={formControl}
                  value={tags()}
                  maxLength={2080}
                  disabled={busy() || !!active() || uncertainStart()}
                  onInput={(event) => setTags(event.currentTarget.value)}
                />
              </label>
              <Button
                variant={active() ? 'secondary' : 'primary'}
                isLoading={busy()}
                disabled={loading() || unavailable() || !!active() || !!props.hasUnsavedChanges?.()}
                onClick={() => void start()}
              >
                {uncertainStart() ? 'Retry same rehearsal request' : 'Send rehearsal alert'}
              </Button>
            </div>
          </Show>
          <Show when={active() && selected()?.id !== active()?.id}>
            <Button
              variant="primary"
              onClick={() => {
                linkedRequestID = undefined;
                ++selectionVersion;
                setSelected(active()!);
              }}
            >
              Resume current rehearsal
            </Button>
          </Show>
          <Show when={loading()}>
            <p role="status" class="text-sm text-muted">
              Loading rehearsal evidence…
            </p>
          </Show>
          <Show when={!loading() && !selected() && !unavailable()}>
            <p class="py-4 text-sm text-muted">
              No rehearsal in the last seven days. Start with a warning to check your saved routes.
            </p>
          </Show>
          <Show when={selected()}>
            {(run) => (
              <div
                class="space-y-4"
                aria-live="polite"
                aria-busy={busy()}
                data-testid="rehearsal-report"
              >
                <div class="flex flex-wrap items-start justify-between gap-3 border-t border-border pt-4">
                  <div class="min-w-0 space-y-1">
                    <h3
                      ref={reportHeading}
                      tabIndex={-1}
                      class="text-base font-semibold"
                      aria-label={`Delivery results. Rehearsal started ${new Date(run().createdAt).toLocaleString()}`}
                    >
                      Delivery results
                    </h3>
                    <p class="text-xs text-muted">{new Date(run().createdAt).toLocaleString()}</p>
                    <p class="break-words text-xs text-muted">
                      {run().level} ·{' '}
                      {run().tags.length ? run().tags.join(', ') : 'No routing tags'}
                    </p>
                  </div>
                  <Show
                    when={!run().recoveredAt}
                    fallback={<span class="text-xs text-muted">Recovery requested</span>}
                  >
                    <Button
                      variant="primary"
                      aria-label="Send recovery and finish rehearsal"
                      disabled={busy() || unavailable() || !!props.hasUnsavedChanges?.()}
                      onClick={() => void recover()}
                    >
                      Send recovery and finish
                    </Button>
                  </Show>
                </div>
                <Show when={unavailable()}>
                  <p class="text-sm text-red-700 dark:text-red-300">
                    The results below are from the last successful read. Current outcomes are
                    unknown.
                  </p>
                </Show>
                <Show when={run().policy.status !== 'passed'}>
                  <CalloutCard
                    tone={
                      run().policy.status === 'unavailable' || run().policy.status === 'failed'
                        ? 'danger'
                        : 'warning'
                    }
                    scale="compact"
                    padding="sm"
                    title={outcomeLabels[run().policy.status]}
                    description={run().policy.message}
                  />
                </Show>
                <Show
                  when={run().destinations.length > 0}
                  fallback={
                    <CalloutCard
                      tone="warning"
                      scale="compact"
                      padding="sm"
                      description="No saved destinations were available. Configure and save a destination for your next rehearsal."
                    />
                  }
                >
                  <p class="text-sm text-base-content">
                    <span class="text-muted">Alert delivery: </span>
                    {firingSummary(run())}
                  </p>
                  <div>
                    <div
                      aria-hidden="true"
                      class="hidden grid-cols-[minmax(0,1fr)_7rem_7rem] gap-4 border-b border-border pb-2 text-xs font-medium text-muted md:grid"
                    >
                      <span>Destination</span>
                      <span>Alert</span>
                      <span>Recovery</span>
                    </div>
                    <ul class="divide-y divide-border" aria-label="Destination delivery results">
                      <For each={run().destinations}>
                        {(destination) => (
                          <DestinationResult
                            destination={destination}
                            policy={run().policy}
                            id={`${inputID}-${encodeURIComponent(destination.id)}`}
                          />
                        )}
                      </For>
                    </ul>
                  </div>
                </Show>
                <div class="space-y-1 text-xs leading-relaxed text-muted">
                  <p>
                    A passed delivery means the destination accepted it. Confirm receipt in your
                    inbox or service.
                  </p>
                  <Show
                    when={!run().recoveredAt}
                    fallback={
                      <p>
                        Recovery requested {new Date(run().recoveredAt!).toLocaleString()}. Pending
                        deliveries continue under your saved policies.
                      </p>
                    }
                  >
                    <p>
                      Finishing cancels pending alert sends. Only destinations that accepted the
                      alert can receive recovery.
                    </p>
                  </Show>
                </div>
              </div>
            )}
          </Show>
          <div class="space-y-3 border-t border-border pt-3">
            <Show when={history().length > 1 || selectedOutsideHistory()}>
              <TechnicalDetailsDisclosure
                title="Recent rehearsals"
                subtitle="Saved for seven days"
                dataTestId={`${inputID}-history-disclosure`}
                class="text-muted"
                keepMounted
              >
                <FormSelect
                  id={`${inputID}-history`}
                  label={
                    selectedOutsideHistory()
                      ? 'Recent rehearsals and linked result'
                      : 'Recent rehearsals (up to 20, retained for seven days)'
                  }
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
                        {run.recoveredAt ? ' · recovery requested' : ' · current'}
                      </option>
                    )}
                  </For>
                </FormSelect>
              </TechnicalDetailsDisclosure>
            </Show>
            <TechnicalDetailsDisclosure
              title="How this works"
              subtitle="Routes, policies and recovery"
              dataTestId={`${inputID}-help`}
              class="text-muted"
              keepMounted
            >
              <div class="space-y-3 text-sm leading-relaxed text-muted">
                <p>
                  Sends clearly labelled messages to saved email, webhook and Apprise destinations
                  through real routing, grouping, quiet hours and retries. It creates no
                  infrastructure incident.
                </p>
                <p>
                  This sample follows the performance-alert quiet-hours category, including your
                  saved setting for suppressing critical performance alerts.
                </p>
                <p>
                  Metric detection, resource maintenance, push notifications and the external
                  watchdog are separate checks. Rehearsals share ordinary destination rate limits
                  and delivery health.
                </p>
                <p>
                  Use Recent delivery activity for individual attempts and transport diagnostics.
                </p>
                <Show when={selected()?.policy.status === 'passed'}>
                  <p>{selected()!.policy.message}</p>
                </Show>
              </div>
            </TechnicalDetailsDisclosure>
          </div>
        </div>
      </details>
    </SettingsPanel>
  );
}
