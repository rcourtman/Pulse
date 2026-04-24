import { Component, For, Show } from 'solid-js';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import CheckIcon from 'lucide-solid/icons/check';
import ClipboardCheckIcon from 'lucide-solid/icons/clipboard-check';
import LoaderCircleIcon from 'lucide-solid/icons/loader-circle';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import XIcon from 'lucide-solid/icons/x';
import type { PendingApproval } from './types';

interface ApprovalCardProps {
  approval: PendingApproval;
  onApprove: () => void;
  onSkip: () => void;
}

const riskLabel = (risk?: string) => {
  const normalized = risk?.trim();
  return normalized ? normalized.toUpperCase() : 'REVIEW';
};

const confidenceLabel = (level?: string) => {
  const normalized = level?.trim();
  return normalized ? normalized.replace(/_/g, ' ').toUpperCase() : 'UNKNOWN';
};

const confidenceClasses = (level?: string) => {
  switch (level) {
    case 'verified':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-200';
    case 'partial':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-900 dark:text-sky-200';
    case 'inferred':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200';
    default:
      return 'bg-base-200 text-base-content';
  }
};

const formatPlanHash = (hash?: string) => {
  const trimmed = hash?.trim();
  if (!trimmed) return '';
  return trimmed.length > 12 ? trimmed.slice(0, 12) : trimmed;
};

const formatExpiry = (expiresAt?: string) => {
  if (!expiresAt) return '';
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
};

export const ApprovalCard: Component<ApprovalCardProps> = (props) => {
  return (
    <div class="rounded-md border border-amber-300 dark:border-amber-700 overflow-hidden shadow-sm">
      {/* Header */}
      <div class="px-3 py-2 text-xs font-medium flex items-center gap-2 bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200 border-b border-amber-200 dark:border-amber-800">
        <div class="p-1 rounded bg-amber-100 dark:bg-amber-800">
          <AlertTriangleIcon class="w-3.5 h-3.5" />
        </div>
        <span class="font-semibold">Approval Required</span>
        <span class="px-1.5 py-0.5 bg-amber-200 dark:bg-amber-800 rounded text-[10px] font-bold uppercase tracking-wider">
          {riskLabel(props.approval.risk)}
        </span>
        <Show when={props.approval.runOnHost}>
          <span class="px-1.5 py-0.5 bg-amber-200 dark:bg-amber-800 rounded text-[10px] font-bold uppercase tracking-wider">
            Agent
          </span>
        </Show>
        <Show when={props.approval.targetHost}>
          <span class="text-[10px] text-amber-600 dark:text-amber-400">
            → {props.approval.targetHost}
          </span>
        </Show>
      </div>

      {/* Command */}
      <div class="px-3 py-3 bg-amber-50 dark:bg-amber-900">
        <Show when={props.approval.description}>
          <p class="mb-3 text-xs leading-relaxed text-amber-900 dark:text-amber-100">
            {props.approval.description}
          </p>
        </Show>

        <div class="mb-3 grid grid-cols-1 sm:grid-cols-3 gap-2 text-[11px]">
          <div>
            <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">Tool</div>
            <div class="text-base-content break-all">{props.approval.toolName}</div>
          </div>
          <div>
            <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">Target</div>
            <div class="text-base-content break-all">
              {props.approval.targetHost || 'Pulse runtime'}
            </div>
          </div>
          <div>
            <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">Execution</div>
            <div class="text-base-content">
              {props.approval.runOnHost ? 'Agent routed' : 'Pulse API'}
            </div>
          </div>
        </div>

        <div class="mb-3 p-2 bg-surface rounded border border-amber-200 dark:border-amber-700">
          <code class="text-xs font-mono text-base-content break-all">
            {props.approval.command}
          </code>
        </div>

        <Show
          when={
            props.approval.plan ||
            props.approval.contextConfidence ||
            props.approval.preflight ||
            props.approval.auditId ||
            props.approval.targetType ||
            props.approval.targetId
          }
        >
          <div class="mb-3 rounded-md border border-amber-200 dark:border-amber-700 bg-white/70 dark:bg-black/10">
            <div class="px-2.5 py-2 flex items-center gap-2 border-b border-amber-200 dark:border-amber-700 text-[11px] font-semibold uppercase text-amber-700 dark:text-amber-300">
              <ClipboardCheckIcon class="w-3.5 h-3.5" />
              <span>Governed Plan</span>
            </div>
            <div class="p-2.5 space-y-2 text-xs">
              <Show when={props.approval.plan?.summary}>
                <p class="leading-relaxed text-base-content">{props.approval.plan?.summary}</p>
              </Show>

              <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px]">
                <div>
                  <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                    Approval
                  </div>
                  <div class="text-base-content break-words">
                    {props.approval.plan?.approval_policy || 'admin approval'}
                  </div>
                </div>
                <div>
                  <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                    Blast radius
                  </div>
                  <div class="text-base-content break-words">
                    {props.approval.plan?.blast_radius || 'target resource state'}
                  </div>
                </div>
                <div>
                  <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                    Rollback
                  </div>
                  <div class="text-base-content">
                    {props.approval.plan?.rollback_available ? 'Available' : 'Not declared'}
                  </div>
                </div>
                <Show when={formatExpiry(props.approval.plan?.expires_at)}>
                  <div>
                    <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                      Approval window
                    </div>
                    <div class="text-base-content">
                      Expires {formatExpiry(props.approval.plan?.expires_at)}
                    </div>
                  </div>
                </Show>
              </div>

              <Show when={props.approval.preflight}>
                <div class="pt-2 border-t border-amber-200 dark:border-amber-700">
                  <div class="flex items-center gap-2 mb-1">
                    <ShieldCheckIcon class="w-3.5 h-3.5 text-amber-700 dark:text-amber-300" />
                    <span class="text-[11px] font-semibold uppercase text-amber-700 dark:text-amber-300">
                      Preflight
                    </span>
                    <span class="px-1.5 py-0.5 rounded bg-base-200 text-[10px] font-bold uppercase text-base-content">
                      {props.approval.preflight?.dry_run_available ? 'Dry run' : 'No dry run'}
                    </span>
                  </div>

                  <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-[11px]">
                    <Show when={props.approval.preflight?.target}>
                      <div>
                        <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                          Target
                        </div>
                        <div class="text-base-content break-words">
                          {props.approval.preflight?.target}
                        </div>
                      </div>
                    </Show>
                    <Show when={props.approval.preflight?.intended_change}>
                      <div>
                        <div class="uppercase font-semibold text-amber-700 dark:text-amber-300">
                          Intended change
                        </div>
                        <div class="text-base-content break-words">
                          {props.approval.preflight?.intended_change}
                        </div>
                      </div>
                    </Show>
                  </div>

                  <Show when={props.approval.preflight?.current_state}>
                    <p class="mt-1.5 leading-relaxed text-base-content">
                      {props.approval.preflight?.current_state}
                    </p>
                  </Show>

                  <Show when={props.approval.preflight?.dry_run_summary}>
                    <p class="mt-1.5 leading-relaxed text-base-content">
                      {props.approval.preflight?.dry_run_summary}
                    </p>
                  </Show>

                  <Show when={(props.approval.preflight?.safety_checks || []).length > 0}>
                    <div class="mt-2">
                      <div class="text-[11px] font-semibold uppercase text-amber-700 dark:text-amber-300">
                        Safety checks
                      </div>
                      <ul class="mt-1 space-y-1 text-[11px] text-base-content/80">
                        <For each={props.approval.preflight?.safety_checks || []}>
                          {(item) => <li class="break-words">{item}</li>}
                        </For>
                      </ul>
                    </div>
                  </Show>

                  <Show when={(props.approval.preflight?.verification_steps || []).length > 0}>
                    <div class="mt-2">
                      <div class="text-[11px] font-semibold uppercase text-amber-700 dark:text-amber-300">
                        Verification
                      </div>
                      <ul class="mt-1 space-y-1 text-[11px] text-base-content/80">
                        <For each={props.approval.preflight?.verification_steps || []}>
                          {(item) => <li class="break-words">{item}</li>}
                        </For>
                      </ul>
                    </div>
                  </Show>
                </div>
              </Show>

              <Show when={props.approval.contextConfidence}>
                <div class="pt-2 border-t border-amber-200 dark:border-amber-700">
                  <div class="flex items-center gap-2 mb-1">
                    <ShieldCheckIcon class="w-3.5 h-3.5 text-amber-700 dark:text-amber-300" />
                    <span
                      class={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${confidenceClasses(
                        props.approval.contextConfidence?.level,
                      )}`}
                    >
                      {confidenceLabel(props.approval.contextConfidence?.level)}
                    </span>
                  </div>
                  <Show when={props.approval.contextConfidence?.summary}>
                    <p class="leading-relaxed text-base-content">
                      {props.approval.contextConfidence?.summary}
                    </p>
                  </Show>
                  <Show when={(props.approval.contextConfidence?.evidence || []).length > 0}>
                    <ul class="mt-1.5 space-y-1 text-[11px] text-base-content/80">
                      <For each={props.approval.contextConfidence?.evidence || []}>
                        {(item) => <li class="break-words">{item}</li>}
                      </For>
                    </ul>
                  </Show>
                </div>
              </Show>

              <Show when={props.approval.auditId || props.approval.plan?.plan_hash}>
                <div class="pt-2 border-t border-amber-200 dark:border-amber-700 text-[11px] text-base-content/80 break-all">
                  <Show when={props.approval.auditId}>
                    <span>Audit {props.approval.auditId}</span>
                  </Show>
                  <Show when={props.approval.plan?.plan_hash}>
                    <span class="ml-2">Plan {formatPlanHash(props.approval.plan?.plan_hash)}</span>
                  </Show>
                </div>
              </Show>
            </div>
          </div>
        </Show>

        {/* Actions */}
        <div class="flex gap-2">
          <button
            type="button"
            onClick={props.onApprove}
            disabled={props.approval.isExecuting}
            class={`flex-1 px-3 py-2 text-xs font-semibold rounded-md transition-all ${
              props.approval.isExecuting
                ? 'bg-green-400 text-white cursor-wait'
                : 'bg-green-500 hover:bg-green-600 text-white shadow-sm hover:shadow-sm'
            }`}
          >
            <Show
              when={!props.approval.isExecuting}
              fallback={
                <span class="flex items-center justify-center gap-1.5">
                  <LoaderCircleIcon class="w-3.5 h-3.5 animate-spin" />
                  Running...
                </span>
              }
            >
              <span class="flex items-center justify-center gap-1.5">
                <CheckIcon class="w-3.5 h-3.5" />
                Approve & Run
              </span>
            </Show>
          </button>
          <button
            type="button"
            onClick={props.onSkip}
            disabled={props.approval.isExecuting}
            class="flex-1 px-3 py-2 text-xs font-semibold hover:bg-surface-hover text-base-content rounded-md transition-colors disabled:opacity-50"
          >
            <span class="flex items-center justify-center gap-1.5">
              <XIcon class="w-3.5 h-3.5" />
              Skip
            </span>
          </button>
        </div>
      </div>
    </div>
  );
};
