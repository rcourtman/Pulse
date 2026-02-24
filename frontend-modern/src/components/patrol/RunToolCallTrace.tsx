/**
 * RunToolCallTrace
 *
 * Collapsible tool call list for a patrol run.
 * Lazy-loads tool call details when expanded.
 */

import { Component, createSignal, createResource, Show, For } from 'solid-js';
import { getPatrolRunHistoryWithToolCalls, type ToolCallRecord } from '@/api/patrol';

interface RunToolCallTraceProps {
  runId: string;
  toolCallCount: number;
}

export const RunToolCallTrace: Component<RunToolCallTraceProps> = (props) => {
  const [expanded, setExpanded] = createSignal(false);
  const [expandedCall, setExpandedCall] = createSignal<string | null>(null);

  // Lazy-load tool calls when expanded — fetch small batch since the run is likely recent
  const [toolCalls] = createResource(
    () => (expanded() ? props.runId : null),
    async (runId) => {
      if (!runId) return [];
      try {
        // Start with a small fetch; if not found, widen the search
        let runs = await getPatrolRunHistoryWithToolCalls(10);
        let run = runs.find((r) => r.id === runId);
        if (!run) {
          runs = await getPatrolRunHistoryWithToolCalls(50);
          run = runs.find((r) => r.id === runId);
        }
        return run?.tool_calls || [];
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
              Loading tool calls...
            </div>
          </Show>

          <Show when={!toolCalls.loading && toolCalls() && toolCalls()!.length > 0}>
            <div class="mt-2 space-y-1">
              <For each={toolCalls()}>
                {(call: ToolCallRecord, index) => (
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
                          class={`px-1.5 py-0.5 rounded text-[10px] font-medium ${
                            call.success
                              ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300'
                              : 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                          }`}
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
                      </div>
                    </Show>
                  </div>
                )}
              </For>
            </div>
          </Show>

          <Show when={!toolCalls.loading && (!toolCalls() || toolCalls()!.length === 0)}>
            <p class="text-xs text-muted mt-2">Tool call details not available for this run.</p>
          </Show>
        </Show>
      </div>
    </Show>
  );
};

export default RunToolCallTrace;
