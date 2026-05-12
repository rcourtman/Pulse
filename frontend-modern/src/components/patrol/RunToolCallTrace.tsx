/**
 * RunToolCallTrace
 *
 * Collapsible tool call list for a patrol run.
 * Lazy-loads tool call details when expanded.
 */

import { Component, createSignal, createResource, Show, For } from 'solid-js';
import {
  getPatrolRunWithToolCalls,
  type ToolCallRecord,
  type ToolCallVerification,
  type ToolCallVerificationStatus,
} from '@/api/patrol';
import {
  getToolCallResultBadgeClass,
  getToolCallsLoadingState,
  getToolCallsUnavailableState,
} from '@/utils/patrolRunPresentation';

interface RunToolCallTraceProps {
  runId: string;
  toolCallCount: number;
}

const VERIFIED_LABELS: Record<ToolCallVerificationStatus, string> = {
  unknown: 'Verification not run',
  verified: 'Verified',
  unverified: 'Verification inconclusive',
  failed: 'Verification failed',
};

const verifiedStatus = (v?: ToolCallVerification): ToolCallVerificationStatus =>
  v?.status ?? 'unknown';

const verifiedTooltip = (v?: ToolCallVerification): string => {
  const status = verifiedStatus(v);
  const base = VERIFIED_LABELS[status];
  const summary = v?.evidenceSummary?.trim();
  return summary ? `${base}: ${summary}` : base;
};

const verifiedIconClass = (status: ToolCallVerificationStatus): string => {
  switch (status) {
    case 'verified':
      return 'text-green-600 dark:text-green-400';
    case 'failed':
      return 'text-red-600 dark:text-red-400';
    default:
      return 'text-muted';
  }
};

const VerifiedIcon: Component<{ status: ToolCallVerificationStatus }> = (p) => {
  if (p.status === 'verified') {
    return (
      <svg
        class={`w-3 h-3 ${verifiedIconClass(p.status)}`}
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        aria-hidden="true"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M5 13l4 4L19 7" />
      </svg>
    );
  }
  if (p.status === 'failed') {
    return (
      <svg
        class={`w-3 h-3 ${verifiedIconClass(p.status)}`}
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 9v3m0 3h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
        />
      </svg>
    );
  }
  return (
    <span class={`inline-block w-3 text-center font-mono text-xs ${verifiedIconClass(p.status)}`} aria-hidden="true">
      –
    </span>
  );
};

export const RunToolCallTrace: Component<RunToolCallTraceProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const [expandedCall, setExpandedCall] = createSignal<string | null>(null);

  // Lazy-load tool calls when expanded — fetch small batch since the run is likely recent
  const [toolCalls] = createResource(
    () => (expanded() ? props.runId : null),
    async (runId) => {
      if (!runId) return [];
      try {
        const run = await getPatrolRunWithToolCalls(runId);
        return run?.tool_calls ?? [];
      } catch {
        return [];
      }
    },
  );

  const truncate = (text: string, max: number = 200) =>
    text.length <= max ? text : text.slice(0, max - 1) + '...';

  return (
    <Show when={props.toolCallCount > 0}>
      <div class="mt-3 pt-3 border-t border-border">
        <button
          type="button"
          onClick={() => setExpanded(!expanded())}
          class="flex items-center gap-2 text-xs font-medium text-base-content hover:text-base-content"
        >
          <svg
            class={`w-3 h-3 transition-transform ${expanded() ? 'rotate-90' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 5l7 7-7 7"
            />
          </svg>
          Tool calls ({props.toolCallCount})
        </button>

        <Show when={expanded()}>
          <Show when={toolCalls.loading}>
            <div class="flex items-center gap-2 text-xs text-muted py-2 mt-2">
              <span class="h-3 w-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
              {getToolCallsLoadingState()}
            </div>
          </Show>

          <Show when={!toolCalls.loading && toolCalls() && toolCalls()!.length > 0}>
            <div class="mt-2 space-y-1">
              <For each={toolCalls()}>
                {(call: ToolCallRecord, index) => {
                  const status = verifiedStatus(call.verification);
                  return (
                    <div class="border border-border rounded">
                      <button
                        type="button"
                        onClick={() => setExpandedCall(expandedCall() === call.id ? null : call.id)}
                        class="w-full flex items-center justify-between gap-2 px-2 py-1.5 text-xs hover:bg-surface-hover"
                      >
                        <div class="flex items-center gap-2 min-w-0">
                          <span class="text-muted font-mono w-5 text-right flex-shrink-0">
                            {index() + 1}.
                          </span>
                          <span class="font-medium text-base-content font-mono truncate">
                            {call.tool_name}
                          </span>
                        </div>
                        <div class="flex items-center gap-2 flex-shrink-0">
                          <span
                            class="inline-flex items-center justify-center w-4"
                            title={verifiedTooltip(call.verification)}
                            aria-label={verifiedTooltip(call.verification)}
                            data-testid={`tool-call-verified-${call.id}`}
                            data-verification-status={status}
                          >
                            <VerifiedIcon status={status} />
                          </span>
                          <span
                            class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getToolCallResultBadgeClass(call.success)}`}
                          >
                            {call.success ? 'success' : 'failed'}
                          </span>
                          <span class="text-muted font-mono">{call.duration_ms}ms</span>
                        </div>
                      </button>

                      <Show when={expandedCall() === call.id}>
                        <div class="border-t border-border p-2 space-y-2">
                          <Show when={call.input}>
                            <div>
                              <div class="text-[10px] font-medium text-muted mb-1">Input</div>
                              <pre class="text-[11px] font-mono bg-base rounded p-2 max-h-32 overflow-auto whitespace-pre-wrap text-base-content">
                                {truncate(call.input, 500)}
                              </pre>
                            </div>
                          </Show>
                          <Show when={call.output}>
                            <div>
                              <div class="text-[10px] font-medium text-muted mb-1">Output</div>
                              <pre class="text-[11px] font-mono bg-base rounded p-2 max-h-32 overflow-auto whitespace-pre-wrap text-base-content">
                                {truncate(call.output, 500)}
                              </pre>
                            </div>
                          </Show>
                          <Show when={call.verification?.evidenceSummary}>
                            <div>
                              <div class="text-[10px] font-medium text-muted mb-1">
                                {VERIFIED_LABELS[status]}
                              </div>
                              <p class="text-[11px] text-base-content whitespace-pre-wrap">
                                {call.verification!.evidenceSummary}
                              </p>
                            </div>
                          </Show>
                        </div>
                      </Show>
                    </div>
                  );
                }}
              </For>
            </div>
          </Show>

          <Show when={!toolCalls.loading && (!toolCalls() || toolCalls()!.length === 0)}>
            <p class="text-xs text-muted mt-2">{getToolCallsUnavailableState()}</p>
          </Show>
        </Show>
      </div>
    </Show>
  );
};

export default RunToolCallTrace;
