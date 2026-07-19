import { For, Show, createMemo, type Component } from 'solid-js';
import ChevronDownIcon from 'lucide-solid/icons/chevron-down';
import type { ActionAuditRecord, ActionDetailResponse } from '@/types/actionAudit';
import {
  formatActionName,
  formatEvidenceClass,
  formatPolicyAuthority,
  formatPolicyReason,
  getActionResourcePresentation,
  verificationTruthLabel,
} from './actionPresentation';
import { getAPTActionPresentation } from './aptActionPresentation';

export const ActionDecisionPacket: Component<{
  audit: ActionAuditRecord;
  detail?: ActionDetailResponse;
}> = (props) => {
  const policy = () => props.audit.plan.policyDecision;
  const result = () => props.audit.result?.actionResultV2;
  const apt = () => getAPTActionPresentation(props.audit);
  const firstEvidence = () => result()?.verification.evidence?.[0];
  const resource = createMemo(() =>
    getActionResourcePresentation(props.audit.request.resourceId, props.audit.resource),
  );
  const expiry = createMemo(() => {
    const value = new Date(props.audit.plan.expiresAt ?? '');
    return Number.isNaN(value.valueOf()) ? 'Not recorded' : value.toLocaleString();
  });
  // Prefer read-time names for blast-radius entries; fall back to the raw
  // plan IDs so nothing is hidden when a resource cannot be resolved.
  const blastRadiusEntries = createMemo(() => {
    const ids = props.audit.plan.predictedBlastRadius ?? [];
    const named = new Map((props.audit.blastRadius ?? []).map((entry) => [entry.id, entry.name]));
    return ids.map((id) => ({ id, name: named.get(id)?.trim() || '' }));
  });

  return (
    <div class="space-y-4" data-testid="action-decision-packet">
      <section
        aria-labelledby="action-intent-heading"
        class="rounded-lg border border-border bg-surface p-4"
      >
        <h3 id="action-intent-heading" class="text-sm font-semibold text-base-content">
          What will happen
        </h3>
        <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt class="text-muted">Action</dt>
            <dd class="font-medium">{formatActionName(props.audit.request.capabilityName)}</dd>
          </div>
          <div>
            <dt class="text-muted">Resource</dt>
            <dd class="font-medium">{resource().label}</dd>
            <dd class="break-all text-xs text-muted">{props.audit.request.resourceId}</dd>
          </div>
          <div class="sm:col-span-2">
            <dt class="text-muted">Reason</dt>
            <dd>{props.audit.request.reason}</dd>
          </div>
          <Show when={props.audit.plan.preflight?.currentState}>
            <div>
              <dt class="text-muted">Current state</dt>
              <dd>{props.audit.plan.preflight?.currentState}</dd>
            </div>
          </Show>
          <Show when={props.audit.plan.preflight?.intendedChange}>
            <div>
              <dt class="text-muted">Intended change</dt>
              <dd>{props.audit.plan.preflight?.intendedChange}</dd>
            </div>
          </Show>
          <div>
            <dt class="text-muted">Approval expires</dt>
            <dd>{expiry()}</dd>
          </div>
          <div>
            <dt class="text-muted">Rollback declared</dt>
            <dd>{props.audit.plan.rollbackAvailable ? 'Yes' : 'No'}</dd>
          </div>
        </dl>
        <Show when={blastRadiusEntries().length > 0}>
          <div class="mt-3">
            <div class="text-sm text-muted">Also affected</div>
            <ul class="mt-1 list-disc pl-5 text-sm">
              <For each={blastRadiusEntries()}>
                {(entry) => (
                  <li>
                    <Show when={entry.name} fallback={<span class="break-all">{entry.id}</span>}>
                      {entry.name}
                      <span class="ml-1 break-all text-xs text-muted">({entry.id})</span>
                    </Show>
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>
      </section>

      <Show when={apt()}>
        {(presentation) => (
          <section
            aria-labelledby="action-safety-heading"
            data-testid="apt-action-safety"
            class="rounded-lg border border-border bg-surface p-4"
          >
            <h3 id="action-safety-heading" class="text-sm font-semibold text-base-content">
              Safety and authority
            </h3>
            <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
              <div>
                <dt class="text-muted">Risk posture</dt>
                <dd class="font-medium">{presentation().safetyPosture}</dd>
              </div>
              <div>
                <dt class="text-muted">Decision required</dt>
                <dd class="font-medium">{presentation().approvalPosture}</dd>
              </div>
              <div class="sm:col-span-2">
                <dt class="text-muted">Operator-selected parameters</dt>
                <dd>{presentation().parameterAuthority}</dd>
              </div>
              <div class="sm:col-span-2">
                <dt class="text-muted">What this action can do</dt>
                <dd>{presentation().authorityBoundary}</dd>
              </div>
            </dl>
          </section>
        )}
      </Show>

      <Show
        when={policy()?.status === 'resolved'}
        fallback={
          <section
            aria-labelledby="action-policy-heading"
            class="rounded-lg border border-amber-300 bg-amber-50 p-4 dark:bg-amber-950/40"
          >
            <h3 id="action-policy-heading" class="text-sm font-semibold text-base-content">
              Policy evidence unavailable
            </h3>
            <p class="mt-2 text-sm text-amber-700 dark:text-amber-200">
              This older action has no server-recorded policy provenance. Re-plan it before acting.
            </p>
          </section>
        }
      >
        <details
          data-testid="action-policy-provenance"
          class="group rounded-lg border border-border bg-surface"
        >
          <summary class="flex cursor-pointer list-none items-center justify-between gap-4 p-4 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500">
            <span>
              <span
                id="action-policy-heading"
                class="block text-sm font-semibold text-base-content"
              >
                Policy evidence
              </span>
              <span class="mt-1 block text-xs text-muted">
                {policy()?.authorities.length ?? 0}{' '}
                {(policy()?.authorities.length ?? 0) === 1 ? 'authority' : 'authorities'} checked at
                planning. Pulse checks current authority again before execution.
              </span>
            </span>
            <ChevronDownIcon
              class="h-4 w-4 shrink-0 text-muted transition-transform group-open:rotate-180"
              aria-hidden="true"
            />
          </summary>
          <div class="border-t border-border-subtle px-4 pb-4 pt-3">
            <p class="text-xs text-muted">Server decision {policy()?.decisionId}</p>
            <div class="mt-3 space-y-2">
              <For each={policy()?.authorities ?? []}>
                {(authority) => (
                  <div class="rounded border border-border-subtle bg-surface-hover p-3">
                    <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
                      <span class="font-medium">{formatPolicyAuthority(authority)}</span>
                      <span class="text-muted">
                        {authority.status === 'consulted'
                          ? 'Consulted'
                          : formatActionName(authority.status)}
                      </span>
                    </div>
                    <div class="mt-1 text-xs text-muted">
                      {authority.sourceId}
                      <Show when={authority.revision}> · {authority.revision}</Show>
                    </div>
                    <ul class="mt-2 list-disc pl-5 text-xs text-base-content">
                      <For each={authority.reasonCodes}>
                        {(reason) => <li>{formatPolicyReason(reason)}</li>}
                      </For>
                    </ul>
                  </div>
                )}
              </For>
            </div>
            <p class="mt-3 text-xs text-muted">
              This is the immutable planning-time policy record. It does not authorize execution by
              itself.
            </p>
          </div>
        </details>
      </Show>

      <Show when={result()}>
        {(truth) => (
          <section aria-labelledby="action-result-heading" class="space-y-3">
            <h3 id="action-result-heading" class="text-sm font-semibold text-base-content">
              Recorded outcome
            </h3>
            <Show when={apt()?.facts.length}>
              <div
                data-testid="apt-action-facts"
                class="rounded-lg border border-border bg-surface p-4"
              >
                <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                  Agent-reported facts
                </div>
                <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
                  <For each={apt()?.facts ?? []}>
                    {(fact) => (
                      <div>
                        <dt class="text-muted">{fact.label}</dt>
                        <dd class="font-medium">{fact.value}</dd>
                      </div>
                    )}
                  </For>
                </dl>
              </div>
            </Show>
            <div
              data-testid="action-execution-truth"
              class="rounded-lg border border-border bg-surface p-4"
            >
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Execution</div>
              <div class="mt-1 font-semibold">
                {truth().execution.status === 'not_run'
                  ? 'Did not run'
                  : formatActionName(truth().execution.status)}
              </div>
              <Show when={truth().execution.reasonCode}>
                <div class="mt-1 text-xs text-muted">
                  Reason: {formatActionName(truth().execution.reasonCode!)}
                </div>
              </Show>
              <Show when={truth().execution.summary && !apt()}>
                <p class="mt-2 text-sm">{truth().execution.summary}</p>
              </Show>
            </div>
            <div
              data-testid="action-verification-truth"
              class="rounded-lg border border-border bg-surface p-4"
            >
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">
                Verification
              </div>
              <div class="mt-1 font-semibold">
                {verificationTruthLabel(
                  truth().verification.status,
                  truth().verification.evidenceClass,
                )}
              </div>
              <div class="mt-1 text-sm text-muted">
                Source: {formatEvidenceClass(truth().verification.evidenceClass)}
              </div>
              <Show when={truth().verification.reasonCode}>
                <div class="mt-1 text-xs text-muted">
                  Reason: {formatActionName(truth().verification.reasonCode!)}
                </div>
              </Show>
              <Show
                when={
                  truth().verification.summary &&
                  (!apt() || truth().verification.summary !== truth().execution.summary)
                }
              >
                <p class="mt-2 text-sm">{truth().verification.summary}</p>
              </Show>
              <Show when={(truth().verification.evidence ?? []).length > 0}>
                <details class="mt-3 rounded border border-border-subtle p-3 text-xs">
                  <summary class="cursor-pointer font-medium">Evidence details</summary>
                  <ul class="mt-2 space-y-2">
                    <For each={truth().verification.evidence}>
                      {(evidence) => (
                        <li>
                          <div>
                            {apt()
                              ? 'Typed read-after-write observation'
                              : evidence.summary || evidence.method}
                          </div>
                          <div class="text-muted">
                            Observed by {evidence.observerId} · {evidence.observerTrustDomain}
                          </div>
                          <div class="text-muted">
                            Agent observed {new Date(evidence.observedAt).toLocaleString()} · Pulse
                            received {new Date(evidence.receivedAt).toLocaleString()}
                          </div>
                        </li>
                      )}
                    </For>
                  </ul>
                </details>
              </Show>
            </div>
            <div
              data-testid="action-compensation-truth"
              class="rounded-lg border border-border bg-surface p-4"
            >
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Recovery</div>
              <div class="mt-1 font-semibold">{formatActionName(truth().compensation.status)}</div>
              <div class="mt-1 text-sm text-muted">
                Support: {formatActionName(truth().compensation.support)}
              </div>
              <Show when={truth().compensation.strategy}>
                <div class="mt-1 text-sm">Strategy: {truth().compensation.strategy}</div>
              </Show>
              <Show when={truth().compensation.summary}>
                <p class="mt-2 text-sm">{truth().compensation.summary}</p>
              </Show>
            </div>
            <Show when={apt()}>
              {(presentation) => (
                <div
                  data-testid="apt-action-next-step"
                  class="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200"
                >
                  <div class="font-semibold">What to do next</div>
                  <p class="mt-1">{presentation().nextStep}</p>
                </div>
              )}
            </Show>
          </section>
        )}
      </Show>

      <Show when={props.detail?.attempt || props.detail?.receipt}>
        <section
          aria-labelledby="action-delivery-heading"
          data-testid="action-delivery-truth"
          class="rounded-lg border border-border bg-surface p-4"
        >
          <h3 id="action-delivery-heading" class="text-sm font-semibold text-base-content">
            Durable delivery record
          </h3>
          <p class="mt-2 text-sm font-medium">
            {props.detail?.receipt
              ? 'One agent receipt is recorded for this action.'
              : props.detail?.attempt?.state === 'receipt_pending'
                ? 'The action was sent once and Pulse is waiting for the durable agent receipt.'
                : 'Pulse recorded the delivery attempt before sending it.'}
          </p>
          <p class="mt-1 text-sm text-muted">
            Refreshing or reconnecting re-reads this action record; it does not create another
            action.
          </p>
          <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
            <Show when={firstEvidence()?.observedAt}>
              <div>
                <dt class="text-muted">Agent observation</dt>
                <dd>{new Date(firstEvidence()!.observedAt).toLocaleString()}</dd>
              </div>
            </Show>
            <Show when={props.detail?.receipt?.receivedAt}>
              <div>
                <dt class="text-muted">Receipt recorded by Pulse</dt>
                <dd>{new Date(props.detail!.receipt!.receivedAt).toLocaleString()}</dd>
              </div>
            </Show>
          </dl>
          <details class="mt-3 rounded border border-border-subtle p-3 text-xs">
            <summary class="cursor-pointer font-medium">Delivery identifiers</summary>
            <div class="mt-2 break-all text-muted">
              Action {props.audit.id}
              <Show when={props.detail?.attempt?.id}> · Attempt {props.detail?.attempt?.id}</Show>
              <Show when={props.detail?.receipt?.transportRequestId}>
                {' '}
                · Transport {props.detail?.receipt?.transportRequestId}
              </Show>
            </div>
          </details>
        </section>
      </Show>
    </div>
  );
};
