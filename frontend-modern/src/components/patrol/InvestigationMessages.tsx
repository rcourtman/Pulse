/**
 * InvestigationMessages
 *
 * Collapsible chat thread for an investigation.
 * Lazy-loads messages when rendered.
 */

import { Component, createResource, Show, For } from 'solid-js';
import { getInvestigationMessages, formatTimestamp, type ChatMessage } from '@/api/patrol';

interface InvestigationMessagesProps {
  findingId: string;
}

/** Returns true if a message has no displayable content */
function isEmptyMessage(msg: ChatMessage): boolean {
  return !msg.content && !msg.reasoning_content && !msg.tool_calls?.length && !msg.tool_result;
}

export const InvestigationMessages: Component<InvestigationMessagesProps> = (props) => {
  const [messages] = createResource(
    () => props.findingId,
    async (findingId) => {
      try {
        const result = await getInvestigationMessages(findingId);
        return result.messages || [];
      } catch {
        return [];
      }
    }
  );

  return (
    <div class="mt-2">
      <Show when={messages.loading}>
        <div class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400 py-2">
          <span class="h-3 w-3 border-2 border-current border-t-transparent rounded-full animate-spin" />
          Loading messages...
        </div>
      </Show>

      <Show when={!messages.loading && (!messages() || messages()!.length === 0)}>
        <p class="text-xs text-slate-500 dark:text-slate-400 py-1">No investigation messages available.</p>
      </Show>

      <Show when={!messages.loading && messages() && messages()!.length > 0}>
        <div class="space-y-2 max-h-80 overflow-y-auto rounded border border-slate-200 dark:border-slate-700 p-2 bg-slate-50 dark:bg-slate-800">
          <For each={messages()}>
            {(msg: ChatMessage) => {
              if (isEmptyMessage(msg)) return null;

              return (
                <div class={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  <div class={`max-w-[85%] rounded-md px-3 py-2 ${
                    msg.role === 'user'
                      ? 'bg-blue-100 dark:bg-blue-900 text-blue-900 dark:text-blue-100'
                      : msg.role === 'system'
                      ? 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 text-xs'
                      : 'bg-white dark:bg-slate-800 text-slate-800 dark:text-slate-200'
                  }`}>
                    {/* Reasoning content (extended thinking) */}
                    <Show when={msg.reasoning_content}>
                      <details class="mb-1">
                        <summary class="text-[10px] text-purple-600 dark:text-purple-400 cursor-pointer hover:underline">
                          Show reasoning
                        </summary>
                        <div class="mt-1 text-[10px] text-slate-500 dark:text-slate-400 whitespace-pre-wrap break-words border-l-2 border-purple-200 dark:border-purple-800 pl-2">
                          {msg.reasoning_content}
                        </div>
                      </details>
                    </Show>

                    {/* Text content */}
                    <Show when={msg.content}>
                      <div class="text-xs whitespace-pre-wrap break-words">{msg.content}</div>
                    </Show>

                    {/* Tool calls (assistant requesting tool use) */}
                    <Show when={msg.tool_calls && msg.tool_calls.length > 0}>
                      <div class="space-y-1">
                        <For each={msg.tool_calls}>
                          {(tc) => (
                            <div class="text-xs rounded border border-indigo-200 dark:border-indigo-800 bg-indigo-50 dark:bg-indigo-900 px-2 py-1">
                              <span class="font-semibold text-indigo-700 dark:text-indigo-300">{tc.name}</span>
                              <Show when={tc.input && Object.keys(tc.input).length > 0}>
                                <pre class="mt-1 text-[10px] text-slate-600 dark:text-slate-400 overflow-x-auto max-h-24 overflow-y-auto">
                                  {JSON.stringify(tc.input, null, 2)}
                                </pre>
                              </Show>
                            </div>
                          )}
                        </For>
                      </div>
                    </Show>

                    {/* Tool result (tool output returned to assistant) */}
                    <Show when={msg.tool_result}>
                      <div class={`text-xs rounded border px-2 py-1 ${
                        msg.tool_result!.is_error
                          ? 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900'
                          : 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900'
                      }`}>
                        <span class={`font-semibold text-[10px] ${
                          msg.tool_result!.is_error
                            ? 'text-red-700 dark:text-red-300'
                            : 'text-green-700 dark:text-green-300'
                        }`}>
                          {msg.tool_result!.is_error ? 'Error' : 'Result'}
                        </span>
                        <pre class="mt-1 text-[10px] text-slate-600 dark:text-slate-400 overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap break-words">
                          {msg.tool_result!.content}
                        </pre>
                      </div>
                    </Show>

                    <div class="text-[10px] text-slate-500 dark:text-slate-500 mt-1">
                      {formatTimestamp(msg.timestamp)}
                    </div>
                  </div>
                </div>
              );
            }}
          </For>
        </div>
      </Show>
    </div>
  );
};

export default InvestigationMessages;
