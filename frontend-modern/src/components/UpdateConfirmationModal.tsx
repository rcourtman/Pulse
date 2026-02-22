import { createEffect, createSignal, Show, For } from 'solid-js';
import type { UpdatePlan } from '@/api/updates';

interface UpdateConfirmationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  currentVersion: string;
  latestVersion: string;
  plan: UpdatePlan;
  isApplying: boolean;
}

export function UpdateConfirmationModal(props: UpdateConfirmationModalProps) {
  const [acknowledged, setAcknowledged] = createSignal(false);

  createEffect(() => {
    if (!props.isOpen) {
      setAcknowledged(false);
    }
  });

  const handleConfirm = () => {
    if (acknowledged() && !props.isApplying) {
      props.onConfirm();
    }
  };

  return (
    <Show when={props.isOpen}>
      <div class="fixed inset-0 bg-black flex items-center justify-center z-50 p-4">
        <div class="bg-surface rounded-md shadow-sm max-w-2xl w-full max-h-[90vh] overflow-y-auto">
          {/* Header */}
          <div class="px-6 py-4 border-b border-border">
            <div class="flex items-center justify-between">
              <h2 class="text-xl font-semibold text-base-content">
                Confirm Update
              </h2>
              <button
                onClick={props.onClose}
                class="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
                disabled={props.isApplying}
              >
                <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          {/* Body */}
          <div class="px-6 py-4 space-y-4">
            {/* Version Jump */}
            <div class="bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-4">
              <div class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
                Version Update
              </div>
              <div class="flex items-center gap-3 text-blue-800 dark:text-blue-200">
                <span class="font-mono text-sm">{props.currentVersion}</span>
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7l5 5m0 0l-5 5m5-5H6" />
                </svg>
                <span class="font-mono text-sm font-semibold">{props.latestVersion}</span>
              </div>
            </div>

            {/* Estimated Time */}
            <Show when={props.plan.estimatedTime}>
              <div class="flex items-center gap-2 text-sm text-muted">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span>Estimated time: {props.plan.estimatedTime}</span>
              </div>
            </Show>

            {/* Prerequisites */}
            <Show when={props.plan.prerequisites && props.plan.prerequisites.length > 0}>
              <div>
                <div class="text-sm font-medium text-base-content mb-2">
                  Prerequisites
                </div>
                <ul class="space-y-2">
                  <For each={props.plan.prerequisites}>
                    {(prerequisite) => (
                      <li class="flex items-start gap-2 text-sm text-slate-700 dark:text-slate-300">
                        <svg class="w-4 h-4 mt-0.5 flex-shrink-0 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                        </svg>
                        <span>{prerequisite}</span>
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            </Show>

            {/* Root Required Warning */}
            <Show when={props.plan.requiresRoot}>
              <div class="bg-yellow-50 dark:bg-yellow-900 border border-yellow-200 dark:border-yellow-800 rounded-md p-3">
                <div class="flex items-start gap-2">
                  <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                  </svg>
                  <div class="text-sm text-yellow-800 dark:text-yellow-200">
                    <div class="font-medium">Root access required</div>
                    <div class="text-yellow-700 dark:text-yellow-300 mt-1">
                      This update requires elevated privileges to modify system files.
                    </div>
                  </div>
                </div>
              </div>
            </Show>

            {/* Rollback Support */}
            <Show when={props.plan.rollbackSupport}>
              <div class="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span>Automatic backup will be created</span>
              </div>
            </Show>

            {/* Acknowledgement Checkbox */}
            <div class="pt-4 border-t border-border">
              <label class="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  checked={acknowledged()}
                  onChange={(e) => setAcknowledged(e.currentTarget.checked)}
                  class="mt-1 w-4 h-4 text-blue-600 bg-slate-100 border-slate-300 rounded focus:ring-blue-500 focus:ring-2"
                  disabled={props.isApplying}
                />
                <span class="text-sm text-slate-700 dark:text-slate-300">
                  I understand that Pulse will be temporarily unavailable during the update process.
                  {props.plan.rollbackSupport && ' A backup will be created automatically.'}
                </span>
              </label>
            </div>
          </div>

          {/* Footer */}
          <div class="px-6 py-4 bg-slate-50 dark:bg-slate-800 border-t border-border flex items-center justify-end gap-3">
            <button
              onClick={props.onClose}
              disabled={props.isApplying}
              class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Cancel
            </button>
            <button
              onClick={handleConfirm}
              disabled={!acknowledged() || props.isApplying}
              class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              <Show when={props.isApplying}>
                <svg class="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                  <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
              </Show>
              <span>{props.isApplying ? 'Starting...' : 'Start Update'}</span>
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
