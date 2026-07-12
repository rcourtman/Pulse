import { For, Show, createMemo, type Component } from 'solid-js';
import type { ActionAuditRecord } from '@/types/actionAudit';
import {
  formatActionName,
  formatEvidenceClass,
  formatPolicyAuthority,
  formatPolicyReason,
  verificationTruthLabel,
} from './actionPresentation';

export const ActionDecisionPacket: Component<{ audit: ActionAuditRecord }> = (props) => {
  const policy = () => props.audit.plan.policyDecision;
  const result = () => props.audit.result?.actionResultV2;
  const expiry = createMemo(() => {
    const value = new Date(props.audit.plan.expiresAt ?? '');
    return Number.isNaN(value.valueOf()) ? 'Not recorded' : value.toLocaleString();
  });

  return (
    <div class="space-y-4" data-testid="action-decision-packet">
      <section aria-labelledby="action-intent-heading" class="rounded-lg border border-border bg-surface p-4">
        <h3 id="action-intent-heading" class="text-sm font-semibold text-base-content">What will happen</h3>
        <dl class="mt-3 grid gap-3 text-sm sm:grid-cols-2">
          <div><dt class="text-muted">Action</dt><dd class="font-medium">{formatActionName(props.audit.request.capabilityName)}</dd></div>
          <div><dt class="text-muted">Resource</dt><dd class="break-all font-medium">{props.audit.request.resourceId}</dd></div>
          <div class="sm:col-span-2"><dt class="text-muted">Reason</dt><dd>{props.audit.request.reason}</dd></div>
          <Show when={props.audit.plan.preflight?.currentState}><div><dt class="text-muted">Current state</dt><dd>{props.audit.plan.preflight?.currentState}</dd></div></Show>
          <Show when={props.audit.plan.preflight?.intendedChange}><div><dt class="text-muted">Intended change</dt><dd>{props.audit.plan.preflight?.intendedChange}</dd></div></Show>
          <div><dt class="text-muted">Approval expires</dt><dd>{expiry()}</dd></div>
          <div><dt class="text-muted">Rollback declared</dt><dd>{props.audit.plan.rollbackAvailable ? 'Yes' : 'No'}</dd></div>
        </dl>
        <Show when={(props.audit.plan.predictedBlastRadius ?? []).length > 0}>
          <div class="mt-3"><div class="text-sm text-muted">Also affected</div><ul class="mt-1 list-disc pl-5 text-sm"><For each={props.audit.plan.predictedBlastRadius}>{(resource) => <li>{resource}</li>}</For></ul></div>
        </Show>
      </section>

      <section aria-labelledby="action-policy-heading" class="rounded-lg border border-border bg-surface p-4">
        <h3 id="action-policy-heading" class="text-sm font-semibold text-base-content">Why Pulse allows this review</h3>
        <Show when={policy()?.status === 'resolved'} fallback={<p class="mt-2 text-sm text-amber-700">This older action has no server-recorded policy provenance. Re-plan it before acting.</p>}>
          <p class="mt-1 text-xs text-muted">Server decision {policy()?.decisionId}</p>
          <div class="mt-3 space-y-2">
            <For each={policy()?.authorities ?? []}>
              {(authority) => (
                <div class="rounded border border-border-subtle bg-surface-hover p-3">
                  <div class="flex flex-wrap items-center justify-between gap-2 text-sm">
                    <span class="font-medium">{formatPolicyAuthority(authority)}</span>
                    <span class="text-muted">{authority.status === 'consulted' ? 'Consulted' : formatActionName(authority.status)}</span>
                  </div>
                  <div class="mt-1 text-xs text-muted">{authority.sourceId}<Show when={authority.revision}> · {authority.revision}</Show></div>
                  <ul class="mt-2 list-disc pl-5 text-xs text-base-content"><For each={authority.reasonCodes}>{(reason) => <li>{formatPolicyReason(reason)}</li>}</For></ul>
                </div>
              )}
            </For>
          </div>
          <p class="mt-3 text-xs text-muted">This records planning-time policy evidence. Pulse checks current authority again before execution.</p>
        </Show>
      </section>

      <Show when={result()}>
        {(truth) => (
          <section aria-labelledby="action-result-heading" class="space-y-3">
            <h3 id="action-result-heading" class="text-sm font-semibold text-base-content">Recorded outcome</h3>
            <div data-testid="action-execution-truth" class="rounded-lg border border-border bg-surface p-4">
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Execution</div>
              <div class="mt-1 font-semibold">{truth().execution.status === 'not_run' ? 'Did not run' : formatActionName(truth().execution.status)}</div>
              <Show when={truth().execution.reasonCode}><div class="mt-1 text-xs text-muted">Reason: {formatActionName(truth().execution.reasonCode!)}</div></Show>
              <Show when={truth().execution.summary}><p class="mt-2 text-sm">{truth().execution.summary}</p></Show>
            </div>
            <div data-testid="action-verification-truth" class="rounded-lg border border-border bg-surface p-4">
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Verification</div>
              <div class="mt-1 font-semibold">{verificationTruthLabel(truth().verification.status, truth().verification.evidenceClass)}</div>
              <div class="mt-1 text-sm text-muted">Source: {formatEvidenceClass(truth().verification.evidenceClass)}</div>
              <Show when={truth().verification.reasonCode}><div class="mt-1 text-xs text-muted">Reason: {formatActionName(truth().verification.reasonCode!)}</div></Show>
              <Show when={truth().verification.summary}><p class="mt-2 text-sm">{truth().verification.summary}</p></Show>
              <Show when={(truth().verification.evidence ?? []).length > 0}>
                <details class="mt-3 rounded border border-border-subtle p-3 text-xs"><summary class="cursor-pointer font-medium">Evidence details</summary><ul class="mt-2 space-y-2"><For each={truth().verification.evidence}>{(evidence) => <li><div>{evidence.summary || evidence.method}</div><div class="text-muted">Observed by {evidence.observerId} · {evidence.observerTrustDomain}</div></li>}</For></ul></details>
              </Show>
            </div>
            <div data-testid="action-compensation-truth" class="rounded-lg border border-border bg-surface p-4">
              <div class="text-xs font-semibold uppercase tracking-wide text-muted">Recovery</div>
              <div class="mt-1 font-semibold">{formatActionName(truth().compensation.status)}</div>
              <div class="mt-1 text-sm text-muted">Support: {formatActionName(truth().compensation.support)}</div>
              <Show when={truth().compensation.strategy}><div class="mt-1 text-sm">Strategy: {truth().compensation.strategy}</div></Show>
              <Show when={truth().compensation.summary}><p class="mt-2 text-sm">{truth().compensation.summary}</p></Show>
            </div>
          </section>
        )}
      </Show>
    </div>
  );
};
