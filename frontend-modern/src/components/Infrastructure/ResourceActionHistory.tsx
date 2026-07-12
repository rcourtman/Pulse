import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import { InfoCardFrame } from '@/components/shared/InfoCardFrame';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { formatRelativeTime } from '@/utils/format';
import {
  formatActionApprovalPolicyLabel,
  formatActionCapabilityLabel,
  getActionAuditRecordStatePresentation,
  getActionAuditResultPresentation,
  getActionAuditVerification,
  getActionAuditVerificationOutcomePresentation,
  shouldRenderActionAuditVerification,
} from '@/utils/actionAuditPresentation';
import { getAPTActionPresentation } from '@/features/actions/aptActionPresentation';

interface ResourceActionHistoryProps {
  audits: ActionAuditRecord[];
  count: number;
  loadingLabel: string;
  error: string;
  onRetry: () => void | Promise<unknown>;
}

const ActionHistoryRow: Component<{ audit: ActionAuditRecord }> = (props) => {
  const state = () => getActionAuditRecordStatePresentation(props.audit);
  const preflight = () => props.audit.plan?.preflight;
  const resultPresentation = () => getActionAuditResultPresentation(props.audit);
  const verificationOutcome = () => getActionAuditVerificationOutcomePresentation(props.audit);
  const verification = () => getActionAuditVerification(props.audit);
  const apt = () => getAPTActionPresentation(props.audit);
  const compensation = () => props.audit.result?.actionResultV2?.compensation;

  return (
    <div class="rounded border border-border bg-surface-hover px-2 py-1.5 text-[10px]">
      <div class="flex items-start justify-between gap-3">
        <div class="min-w-0">
          <div class="font-medium text-base-content">
            {formatActionCapabilityLabel(props.audit.request?.capabilityName)}
          </div>
          <div class="mt-0.5 text-muted">
            {formatRelativeTime(props.audit.updatedAt || props.audit.createdAt)}
            <Show when={props.audit.request?.requestedBy}>
              <span class="mx-1">•</span>
              <span>{props.audit.request.requestedBy}</span>
            </Show>
          </div>
        </div>
        <span class={`shrink-0 rounded border px-1.5 py-0.5 font-medium ${state().className}`}>
          {state().label}
        </span>
      </div>

      <div class="mt-1 space-y-1">
        <Show when={props.audit.request?.reason}>
          <div class="text-base-content">{props.audit.request.reason}</div>
        </Show>
        <div class="flex items-center justify-between gap-2">
          <span class="text-muted">Approval</span>
          <span class="font-medium text-base-content">
            {formatActionApprovalPolicyLabel(props.audit.plan?.approvalPolicy)}
          </span>
        </div>
        <Show when={preflight()}>
          <div class="flex items-center justify-between gap-2">
            <span class="text-muted">Dry run</span>
            <span class="font-medium text-base-content">
              {preflight()?.dryRunAvailable ? 'Available' : 'Not available'}
            </span>
          </div>
        </Show>
        <Show when={preflight()?.intendedChange}>
          <div class="flex items-start justify-between gap-2">
            <span class="text-muted">Intent</span>
            <span class="max-w-[70%] text-right font-medium text-base-content">
              {preflight()?.intendedChange}
            </span>
          </div>
        </Show>
        <Show when={(preflight()?.safetyChecks || []).length > 0}>
          <div class="space-y-1">
            <div class="text-muted">Safety checks</div>
            <ul class="space-y-0.5 pl-3 text-base-content">
              <For each={preflight()?.safetyChecks || []}>
                {(check) => <li class="list-disc">{check}</li>}
              </For>
            </ul>
          </div>
        </Show>
        <Show when={resultPresentation()}>
          {(() => {
            const presentation = resultPresentation()!;
            return (
              <div class={`rounded border px-2 py-1 text-[10px] ${presentation.className}`}>
                <div class="font-medium">{presentation.label}</div>
                <Show when={presentation.kind === 'refusal'}>
                  <div class="mt-0.5 font-medium">Refused before dispatch</div>
                </Show>
                <Show when={presentation.reasonLabel}>
                  <div class="mt-0.5 font-medium">{presentation.reasonLabel}</div>
                </Show>
                <Show when={presentation.detail && !apt()}>
                  <div class="mt-0.5 opacity-80">{presentation.detail}</div>
                </Show>
                <Show when={presentation.recordedDetail}>
                  <div class="mt-0.5 opacity-80">
                    <span class="font-medium">Recorded detail: </span>
                    {presentation.recordedDetail}
                  </div>
                </Show>
              </div>
            );
          })()}
        </Show>
        <Show when={apt()?.facts.length}>
          <dl data-testid="resource-apt-action-facts" class="grid gap-1 rounded border border-border bg-surface px-2 py-1.5 sm:grid-cols-2">
            <For each={apt()?.facts ?? []}>{(fact) => <div class="flex items-start justify-between gap-2"><dt class="text-muted">{fact.label}</dt><dd class="text-right font-medium text-base-content">{fact.value}</dd></div>}</For>
          </dl>
        </Show>
        <Show when={verificationOutcome()}>
          {(() => {
            const outcome = verificationOutcome()!;
            return (
              <div class={`rounded border px-2 py-1 text-[10px] ${outcome.className}`}>
                <div class="font-medium">{outcome.label}</div>
                <div class="mt-0.5 opacity-80">{outcome.detail}</div>
                <Show when={outcome.evidenceSummary}>
                  <div class="mt-0.5 opacity-80">
                    <span class="font-medium">Evidence: </span>
                    {outcome.evidenceSummary}
                  </div>
                </Show>
              </div>
            );
          })()}
        </Show>
        <Show when={compensation()}>
          {(recovery) => (
            <div data-testid="resource-action-recovery-truth" class="rounded border border-border bg-surface px-2 py-1 text-[10px] text-base-content">
              <div class="font-medium">Recovery: {formatActionCapabilityLabel(recovery().status)}</div>
              <div class="mt-0.5 text-muted">Support: {formatActionCapabilityLabel(recovery().support)}</div>
              <Show when={recovery().summary}><div class="mt-0.5">{recovery().summary}</div></Show>
            </div>
          )}
        </Show>
        <Show when={apt()}>{(presentation) => <div data-testid="resource-apt-action-next-step" class="rounded border border-blue-200 bg-blue-50 px-2 py-1 text-[10px] text-blue-900 dark:border-blue-800 dark:bg-blue-950/40 dark:text-blue-200"><span class="font-medium">Next: </span>{presentation().nextStep}</div>}</Show>
        <Show when={shouldRenderActionAuditVerification(props.audit)}>
          {(() => {
            const v = verification()!;
            const toneClass = v.success
              ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300'
              : 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-300';
            return (
              <div class={`rounded border px-2 py-1 text-[10px] ${toneClass}`}>
                <div class="font-medium">
                  {v.success ? 'Legacy check passed (source unclassified)' : 'Legacy check failed (source unclassified)'}
                </div>
                <Show when={v.command}>
                  <div class="mt-0.5 font-mono text-[10px] opacity-80">{v.command}</div>
                </Show>
                <Show when={v.output}>
                  <div class="mt-0.5 whitespace-pre-wrap break-words">{v.output}</div>
                </Show>
                <Show when={v.note}>
                  <div class="mt-0.5 italic">{v.note}</div>
                </Show>
              </div>
            );
          })()}
        </Show>
      </div>
    </div>
  );
};

export const ResourceActionHistory: Component<ResourceActionHistoryProps> = (props) => (
  <InfoCardFrame data-testid="resource-action-history-section" class="w-full">
    <div class="flex items-center justify-between gap-3">
      <div>
        <div class="text-[11px] font-medium uppercase tracking-wide text-base-content">
          Action history
        </div>
        <Show when={props.count > 0}>
          <div class="mt-1 text-[10px] text-muted">Actions {props.count}</div>
        </Show>
      </div>
      <div class="text-right text-[10px] text-muted">{props.loadingLabel}</div>
    </div>

    <Show when={props.error}>
      <div class="mt-2 rounded border border-amber-200 bg-amber-50 px-2 py-1.5 text-[10px] text-amber-700 dark:border-amber-700 dark:bg-amber-900 dark:text-amber-200">
        <div class="flex items-start justify-between gap-2">
          <span>{props.error}</span>
          <button
            type="button"
            class="shrink-0 font-medium text-amber-700 underline dark:text-amber-200"
            onClick={() => void props.onRetry()}
          >
            Retry
          </button>
        </div>
      </div>
    </Show>

    <Show
      when={props.audits.length > 0}
      fallback={
        <div class="mt-3 rounded border border-dashed border-border bg-surface-hover px-2 py-2 text-[10px] text-muted">
          No actions yet.
        </div>
      }
    >
      <div class="mt-3 space-y-2">
        <For each={props.audits}>{(audit) => <ActionHistoryRow audit={audit} />}</For>
      </div>
    </Show>
  </InfoCardFrame>
);
