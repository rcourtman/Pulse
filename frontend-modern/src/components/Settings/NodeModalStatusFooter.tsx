import { Component, For, Show } from 'solid-js';
import type { NodeModalProps } from '@/components/Settings/nodeModalModel';
import type { NodeModalState } from '@/components/Settings/useNodeModalState';
import { logger } from '@/utils/logger';

interface NodeModalStatusFooterProps {
  modalProps: NodeModalProps;
  state: NodeModalState;
}

export const NodeModalStatusFooter: Component<NodeModalStatusFooterProps> = (props) => {
  const { modalProps, state } = props;

  return (
    <>
      <Show when={state.testResult()}>
        {(() => {
          const result = state.testResult();
          logger.debug('Test result display', {
            status: result?.status,
            message: result?.message,
          });
          return null;
        })()}
        <div class={state.testResultPresentation().panelClass}>
          <div class="flex items-start gap-2">
            <Show when={state.testResultPresentation().icon === 'success'}>
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="flex-shrink-0 mt-0.5"
              >
                <path d="M9 12l2 2 4-4"></path>
                <circle cx="12" cy="12" r="10"></circle>
              </svg>
            </Show>
            <Show when={state.testResultPresentation().icon === 'warning'}>
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="flex-shrink-0 mt-0.5"
              >
                <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path>
              </svg>
            </Show>
            <Show when={state.testResultPresentation().icon === 'error'}>
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="flex-shrink-0 mt-0.5"
              >
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="15" y1="9" x2="9" y2="15"></line>
                <line x1="9" y1="9" x2="15" y2="15"></line>
              </svg>
            </Show>
            <div class={`flex-1 ${state.testResultPresentation().textClass}`}>
              <p>{state.testResult()?.message}</p>
              <Show when={state.testResult()?.isCluster}>
                <p class="mt-1 text-xs opacity-80">
                  ✨ Cluster detected! All cluster nodes will be automatically added.
                </p>
              </Show>
              <Show when={state.testResult()?.warnings && state.testResult()!.warnings!.length > 0}>
                <div class="mt-2 space-y-1">
                  <p class="text-xs font-semibold opacity-90">Warnings:</p>
                  <ul class="text-xs space-y-0.5 opacity-80">
                    <For each={state.testResult()?.warnings}>{(warning) => <li>• {warning}</li>}</For>
                  </ul>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>

      <Show when={state.hostLimitReached()}>
        <div class="mx-6 mb-2 rounded-md border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-900/30 px-4 py-3">
          <p class="text-sm font-medium text-amber-800 dark:text-amber-200">
            Agent limit reached — you'll need to remove an agent or upgrade to add more.
          </p>
          <div class="mt-2 flex items-center gap-3">
            <span class="text-xs text-amber-700 dark:text-amber-300">Need more agents?</span>
            <Show when={state.canStartTrial()}>
              <button
                type="button"
                class="text-xs font-semibold text-indigo-700 dark:text-indigo-300 hover:underline disabled:opacity-60"
                disabled={state.startingTrial()}
                onClick={state.handleStartTrial}
              >
                Start your free 14-day trial
              </button>
            </Show>
          </div>
        </div>
      </Show>

      <div class="flex items-center justify-between px-6 py-4 border-t border-border">
        <button
          type="button"
          onClick={state.handleTestConnection}
          disabled={state.isTesting()}
          class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {state.isTesting() ? 'Testing...' : 'Test Connection'}
        </button>

        <div class="flex items-center gap-3">
          <Show when={modalProps.showBackToDiscovery && modalProps.onBackToDiscovery}>
            <button
              type="button"
              onClick={() => {
                modalProps.onBackToDiscovery!();
                modalProps.onClose();
              }}
              class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors flex items-center gap-2"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
              >
                <line x1="19" y1="12" x2="5" y2="12"></line>
                <polyline points="12 19 5 12 12 5"></polyline>
              </svg>
              Back to Discovery
            </button>
          </Show>
          <button
            type="button"
            onClick={modalProps.onClose}
            class="px-4 py-2 text-sm border border-border text-base-content rounded-md hover:bg-surface-hover transition-colors"
          >
            Cancel
          </button>
          <button
            type="submit"
            class="px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors"
          >
            {state.isEditingExistingNode() ? 'Update' : 'Add'} Node
          </button>
        </div>
      </div>
    </>
  );
};
