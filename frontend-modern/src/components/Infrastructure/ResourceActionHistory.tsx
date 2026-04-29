import { For, Show } from 'solid-js';
import type { Component } from 'solid-js';
import type { ActionAuditRecord } from '@/types/actionAudit';
import { formatRelativeTime } from '@/utils/format';
import {
  formatActionApprovalPolicyLabel,
  formatActionCapabilityLabel,
  getActionAuditStatePresentation,
} from '@/utils/actionAuditPresentation';

interface ResourceActionHistoryProps {
  audits: ActionAuditRecord[];
  count: number;
  loadingLabel: string;
  error: string;
  onRetry: () => void | Promise<unknown>;
}

const ActionHistoryRow: Component<{ audit: ActionAuditRecord }> = (props) => {
  const state = () => getActionAuditStatePresentation(props.audit.state);
  const preflight = () => props.audit.plan?.preflight;
  const result = () => props.audit.result;

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
        <Show when={result()}>
          <div class="rounded border border-border bg-surface px-2 py-1 text-[10px]">
            <div class="font-medium text-base-content">
              {result()?.success ? 'Result' : 'Failure'}
            </div>
            <Show when={result()?.output || result()?.errorMessage}>
              <div class="mt-0.5 text-muted">{result()?.output || result()?.errorMessage}</div>
            </Show>
          </div>
        </Show>
      </div>
    </div>
  );
};

export const ResourceActionHistory: Component<ResourceActionHistoryProps> = (props) => (
  <div
    data-testid="resource-action-history-section"
    class="w-full rounded border border-border bg-surface p-3 shadow-sm"
  >
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
  </div>
);
