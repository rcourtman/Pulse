import { Component, Show } from 'solid-js';
import type { PendingApproval } from './types';

interface ApprovalCardProps {
  approval: PendingApproval;
  onApprove: () => void;
  onSkip: () => void;
}

export const ApprovalCard: Component<ApprovalCardProps> = (props) => {
  return (
    <div class="rounded-md border border-amber-300 dark:border-amber-700 overflow-hidden shadow-sm">
      {/* Header */}
      <div class="px-3 py-2 text-xs font-medium flex items-center gap-2 bg-amber-50 dark:bg-amber-900 text-amber-800 dark:text-amber-200 border-b border-amber-200 dark:border-amber-800">
        <div class="p-1 rounded bg-amber-100 dark:bg-amber-800">
          <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
        </div>
        <span class="font-semibold">Approval Required</span>
        <Show when={props.approval.runOnHost}>
          <span class="px-1.5 py-0.5 bg-amber-200 dark:bg-amber-800 rounded text-[10px] font-bold uppercase tracking-wider">
            Host
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
        <div class="mb-3 p-2 bg-surface rounded border border-amber-200 dark:border-amber-700">
          <code class="text-xs font-mono text-base-content break-all">
            {props.approval.command}
          </code>
        </div>

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
                  <svg class="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                  </svg>
                  Running...
                </span>
              }
            >
              <span class="flex items-center justify-center gap-1.5">
                <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                Run Command
              </span>
            </Show>
          </button>
          <button
            type="button"
            onClick={props.onSkip}
            disabled={props.approval.isExecuting}
            class="flex-1 px-3 py-2 text-xs font-semibold bg-slate-100 hover:bg-slate-200 dark:bg-slate-700 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-md transition-colors disabled:opacity-50"
          >
            <span class="flex items-center justify-center gap-1.5">
              <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
              Skip
            </span>
          </button>
        </div>
      </div>
    </div>
  );
};
