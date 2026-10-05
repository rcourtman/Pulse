import {
  createEffect,
  createMemo,
  createSignal,
  For,
  onCleanup,
  onMount,
  Show,
  untrack,
} from 'solid-js';
import {
  deleteManualSuppressionRule,
  getSuppressionRules,
  isManualSuppressionRule,
  type PatrolSuppressionRule,
} from '@/api/patrol';
import { Button } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';

export const PATROL_SUPPRESSION_RULES_PATH = '/patrol/activity#patrol-suppression-rules';

const ruleScope = (rule: PatrolSuppressionRule) => ({
  resource: rule.resource_id ? rule.resource_name || rule.resource_id : 'All resources',
  category: rule.category || 'All categories',
});
const sameRule = (left: PatrolSuppressionRule, right: PatrolSuppressionRule) =>
  [
    'id',
    'resource_id',
    'resource_name',
    'category',
    'description',
    'created_at',
    'created_from',
    'finding_id',
  ].every(
    (key) => left[key as keyof PatrolSuppressionRule] === right[key as keyof PatrolSuppressionRule],
  );

/** A reversible manual-rule control inside Patrol's existing Activity flow. */
export function PatrolSuppressionRules(props: { openForLink?: boolean } = {}) {
  const [open, setOpen] = createSignal(false);
  const [rules, setRules] = createSignal<PatrolSuppressionRule[]>();
  const [selected, setSelected] = createSignal<PatrolSuppressionRule>();
  const [busy, setBusy] = createSignal(false);
  const [error, setError] = createSignal('');
  const [notice, setNotice] = createSignal('');
  const manualRules = createMemo(() => rules()?.filter(isManualSuppressionRule) ?? []);
  const otherCount = createMemo(() => (rules()?.length ?? 0) - manualRules().length);
  let loadedOrg = '';
  let generation = 0;
  let request: AbortController | undefined;
  let summary: HTMLElement | undefined;
  let trigger: HTMLElement | undefined;
  const currentOrg = () => getOrgID() || 'default';
  const begin = () => {
    request?.abort();
    request = new AbortController();
    return { generation: ++generation, controller: request, org: currentOrg() };
  };
  const isCurrent = (run: ReturnType<typeof begin>) =>
    run.generation === generation && !run.controller.signal.aborted && run.org === currentOrg();
  const invalidate = () => {
    ++generation;
    request?.abort();
    setRules(undefined);
    setSelected(undefined);
    setBusy(false);
    setNotice('');
    setError('Organisation or access changed. Reload rules before making changes.');
  };
  const offOrg = eventBus.on('org_switched', invalidate);
  const offMembership = eventBus.on('organizations_changed', invalidate);
  onCleanup(() => {
    ++generation;
    request?.abort();
    offOrg();
    offMembership();
  });

  const load = async () => {
    if (busy()) return;
    const run = begin();
    setBusy(true);
    setSelected(undefined);
    setRules(undefined);
    setError('');
    setNotice('');
    try {
      const data = await getSuppressionRules(run.org, run.controller.signal);
      if (!isCurrent(run)) return;
      loadedOrg = run.org;
      setRules(data);
    } catch {
      if (isCurrent(run))
        setError('Rules could not be loaded. Check your access and reload rules.');
    } finally {
      if (run.generation === generation) {
        if (run.org !== currentOrg()) invalidate();
        else setBusy(false);
      }
    }
  };
  const choose = (rule: PatrolSuppressionRule, button: HTMLElement) => {
    if (busy() || !rules() || !isManualSuppressionRule(rule)) return;
    if (loadedOrg !== currentOrg()) return invalidate();
    trigger = button;
    setNotice('');
    setSelected({ ...rule });
  };
  const remove = async () => {
    const rule = selected();
    if (!rule || busy() || !rules()) return;
    if (loadedOrg !== currentOrg()) return invalidate();
    const run = begin();
    setBusy(true);
    setError('');
    try {
      // A fresh authorised read verifies the exact displayed scope/origin.
      // Do not act on a stale, edited, missing or cross-tenant confirmation.
      const before = await getSuppressionRules(run.org, run.controller.signal);
      if (!isCurrent(run)) return;
      const current = before.find((entry) => entry.id === rule.id);
      if (!current || !isManualSuppressionRule(current) || !sameRule(current, rule)) {
        throw new Error('The confirmed rule is no longer current.');
      }
      await deleteManualSuppressionRule(current, run.org, run.controller.signal);
      if (!isCurrent(run)) return;
      const after = await getSuppressionRules(run.org, run.controller.signal);
      if (!isCurrent(run)) return;
      if (after.some((entry) => entry.id === rule.id)) throw new Error('Rule still present.');
      // The deleted row is gone; return focus to a stable control, not its
      // detached button. Cancellation returns to the selected row instead.
      trigger = summary;
      setRules(after);
      setSelected(undefined);
      setNotice(
        'Rule removed. Future matching findings can appear. Dismissed findings and history were not reopened or removed; other rules may still cover the same scope.',
      );
    } catch {
      if (!isCurrent(run)) return;
      trigger = summary;
      setSelected(undefined);
      setRules(undefined);
      setError('Removal could not be confirmed. Reload rules before trying again.');
    } finally {
      if (run.generation === generation) {
        if (run.org !== currentOrg()) invalidate();
        else setBusy(false);
      }
    }
  };
  const openFromLink = () => {
    setOpen(true);
    if (!rules() && !busy()) void load();
    queueMicrotask(() => summary?.focus());
  };
  const openFromHash = () => {
    if (window.location.hash === '#patrol-suppression-rules') openFromLink();
  };
  onMount(() => {
    openFromHash();
    window.addEventListener('hashchange', openFromHash);
    onCleanup(() => window.removeEventListener('hashchange', openFromHash));
  });
  createEffect(() => {
    // Router state can update before pushState changes window.location. The
    // confirmed route hash supplied by the owner is already authoritative.
    if (props.openForLink) untrack(openFromLink);
  });

  return (
    <details
      id="patrol-suppression-rules"
      class="rounded-lg border border-border bg-surface"
      open={open()}
      onToggle={(event) => {
        const next = event.currentTarget.open;
        setOpen(next);
        if (next && !rules() && !busy() && !error()) void load();
      }}
    >
      <summary
        ref={summary}
        class="min-h-11 cursor-pointer rounded-lg px-4 py-3 text-sm font-semibold text-base-content focus-visible:outline-2 focus-visible:outline-blue-500 sm:px-5"
      >
        Suppression rules
      </summary>
      <Show when={open()}>
        <div class="space-y-3 border-t border-border p-4 sm:p-5">
          <p class="text-sm leading-5 text-muted">
            Review permanent rules created with Create rule. Removing one allows future matching
            Patrol findings; it does not reopen dismissed findings or erase history. Use Reopen
            finding in Finding options and history to undo an individual dismissal.
          </p>
          <Button
            variant="secondary"
            size="sm"
            class="min-h-11"
            disabled={busy()}
            onClick={() => void load()}
          >
            {busy() ? 'Checking rules…' : 'Reload rules'}
          </Button>
          <Show when={error()}>
            <p role="alert" class="text-sm text-red-700 dark:text-red-300">
              {error()}
            </p>
          </Show>
          <Show when={notice()}>
            <p role="status" class="text-sm text-base-content">
              {notice()}
            </p>
          </Show>
          <Show when={rules()}>
            <Show
              when={manualRules().length > 0}
              fallback={<p class="text-sm text-muted">No manually created suppression rules.</p>}
            >
              <ul class="space-y-3" aria-label="Manually created suppression rules">
                <For each={manualRules()}>
                  {(rule) => (
                    <li class="rounded-md border border-border p-3">
                      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                        <div class="min-w-0 space-y-1 text-sm text-base-content">
                          <p class="break-words font-medium">
                            {ruleScope(rule).resource} · {ruleScope(rule).category}
                          </p>
                          <Show when={!rule.resource_id || !rule.category}>
                            <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                              Broad rule: matches{' '}
                              {!rule.resource_id ? 'all resources' : 'this resource'} in{' '}
                              {!rule.category ? 'all categories' : rule.category}.
                            </p>
                          </Show>
                          <p class="whitespace-pre-wrap break-words">
                            {rule.description || 'No reason recorded.'}
                          </p>
                          <Show when={rule.resource_id}>
                            <p class="break-all text-xs text-muted">
                              Resource ID: {rule.resource_id}
                            </p>
                          </Show>
                          <p class="break-all text-xs text-muted">Rule ID: {rule.id}</p>
                        </div>
                        <Button
                          variant="secondary"
                          size="sm"
                          class="min-h-11 shrink-0 self-start"
                          disabled={busy()}
                          aria-label={`Remove rule for ${ruleScope(rule).resource}, ${ruleScope(rule).category}`}
                          onClick={(event) => choose(rule, event.currentTarget)}
                        >
                          Remove rule
                        </Button>
                      </div>
                    </li>
                  )}
                </For>
              </ul>
            </Show>
            <Show when={otherCount() > 0}>
              <p class="text-xs leading-5 text-muted">
                {otherCount()} other remembered{' '}
                {otherCount() === 1 ? 'decision is' : 'decisions are'} not manual rules and cannot
                be removed here. Finding dismissals remain in Finding options and history.
              </p>
            </Show>
          </Show>
        </div>
      </Show>
      <Dialog
        isOpen={Boolean(selected())}
        ariaLabel="Remove suppression rule?"
        onClose={() => {
          if (!busy()) setSelected(undefined);
        }}
        returnFocus={() => trigger}
      >
        <Show when={selected()}>
          {(rule) => (
            <div class="flex min-h-0 flex-col">
              <div class="shrink-0 space-y-3 px-5 pt-5">
                <h2 class="text-base font-semibold text-base-content">Remove suppression rule?</h2>
                <p class="text-sm leading-5 text-muted">
                  Only this exact manual rule will be removed. Future matching findings can appear;
                  existing dismissals and history stay unchanged.
                </p>
              </div>
              <div
                role="region"
                aria-label="Rule scope and reason"
                tabindex="0"
                class="min-h-0 space-y-4 overflow-y-auto px-5 py-4 focus-visible:outline-2 focus-visible:outline-blue-500"
              >
                <dl class="space-y-2 text-sm text-base-content">
                  <div>
                    <dt class="font-medium">Resource</dt>
                    <dd class="break-words">{ruleScope(rule()).resource}</dd>
                  </div>
                  <Show when={rule().resource_id}>
                    <div>
                      <dt class="font-medium">Resource ID</dt>
                      <dd class="break-all">{rule().resource_id}</dd>
                    </div>
                  </Show>
                  <div>
                    <dt class="font-medium">Category</dt>
                    <dd>{ruleScope(rule()).category}</dd>
                  </div>
                  <div>
                    <dt class="font-medium">Rule ID</dt>
                    <dd class="break-all">{rule().id}</dd>
                  </div>
                  <div>
                    <dt class="font-medium">Reason</dt>
                    <dd class="whitespace-pre-wrap break-words">
                      {rule().description || 'No reason recorded.'}
                    </dd>
                  </div>
                </dl>
                <Show when={!rule().resource_id || !rule().category}>
                  <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
                    This broad rule covers {!rule().resource_id ? 'all resources' : 'this resource'}{' '}
                    in {!rule().category ? 'all categories' : rule().category}.
                  </p>
                </Show>
              </div>
              <div class="flex shrink-0 flex-wrap gap-2 border-t border-border px-5 py-4">
                <Button
                  variant="secondary"
                  class="min-h-11"
                  autofocus
                  disabled={busy()}
                  onClick={() => setSelected(undefined)}
                >
                  Cancel
                </Button>
                <Button
                  variant="danger"
                  class="min-h-11"
                  disabled={busy()}
                  onClick={() => void remove()}
                >
                  {busy() ? 'Verifying removal…' : 'Remove this rule'}
                </Button>
              </div>
            </div>
          )}
        </Show>
      </Dialog>
    </details>
  );
}
