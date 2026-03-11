/**
 * RemediationStatus
 *
 * Post-execution display: shows success/failure status, command output,
 * exit code, error details, and server message.
 */

import { Component, Show, createSignal } from 'solid-js';
import type { ApprovalExecutionResult } from '@/api/ai';
import { getRemediationPresentation } from '@/utils/remediationPresentation';

interface RemediationStatusProps {
  result: ApprovalExecutionResult;
}

export const RemediationStatus: Component<RemediationStatusProps> = (props) => {
  const [showOutput, setShowOutput] = createSignal(false);
  const presentation = () => getRemediationPresentation(props.result.success);

  return (
    <div class={`mt-2 p-2 rounded text-xs ${presentation().panelClass}`}>
      <div class="flex items-center gap-2">
        <Show when={props.result.success}>
          <svg
            class={`w-4 h-4 flex-shrink-0 ${presentation().iconClass}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <span class={presentation().messageClass}>{presentation().message}</span>
        </Show>
        <Show when={!props.result.success}>
          <svg
            class={`w-4 h-4 flex-shrink-0 ${presentation().iconClass}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <span class={presentation().messageClass}>{presentation().message}</span>
        </Show>
        <Show when={props.result.exit_code !== undefined}>
          <span class="text-muted">exit code: {props.result.exit_code}</span>
        </Show>
      </div>

      <Show when={props.result.error}>
        <div class={`mt-1 ${presentation().errorClass}`}>{props.result.error}</div>
      </Show>

      <Show when={props.result.message && props.result.message !== props.result.error}>
        <div class="text-muted mt-1">{props.result.message}</div>
      </Show>

      <Show when={props.result.output}>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            setShowOutput(!showOutput());
          }}
          class="text-[10px] text-muted hover:underline mt-1"
        >
          {showOutput() ? 'Hide output' : 'Show output'}
        </button>
        <Show when={showOutput()}>
          <div class="bg-surface rounded p-2 font-mono mt-1 max-h-32 overflow-auto whitespace-pre-wrap text-[11px] text-base-content">
            {props.result.output}
          </div>
        </Show>
      </Show>
    </div>
  );
};

export default RemediationStatus;
