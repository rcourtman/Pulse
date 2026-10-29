/**
 * InvestigationMessages
 *
 * Collapsible chat thread for an investigation.
 * Lazy-loads messages when rendered.
 */

import { Component, createEffect, createMemo, on, Show, For } from 'solid-js';
import { getInvestigationMessages, formatTimestamp, type ChatMessage } from '@/api/patrol';
import { withAPIErrorStatusNull } from '@/api/responseUtils';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { Button } from '@/components/shared/Button';
import { InlineNotice } from '@/components/shared/InlineNotice';
import { createNonSuspendingQuery } from '@/hooks/createNonSuspendingQuery';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { getInvestigationMessagesState } from '@/utils/patrolEmptyStatePresentation';
import { getAPIReadAccessErrorMessage } from '@/utils/apiAccessError';
import { renderMarkdown } from '@/components/AI/aiChatUtils';
import { ToolExecutionBlock } from './ToolExecutionBlock';

// Compact markdown styling, scaled for the investigation thread's text-xs
// bubbles.
const investigationMarkdownClass =
  'text-xs prose prose-slate prose-sm dark:prose-invert max-w-none wrap-break-word prose-p:leading-relaxed prose-p:my-1.5 prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-md prose-pre:text-[10px] prose-pre:border prose-pre:border-slate-800 prose-code:text-blue-700 dark:prose-code:text-blue-300 prose-code:bg-blue-50 dark:prose-code:bg-blue-900/25 prose-code:px-1 prose-code:py-0.5 prose-code:rounded-sm prose-code:font-mono prose-code:text-[0.9em] prose-code:before:content-none prose-code:after:content-none prose-headings:font-semibold prose-headings:my-2 prose-ul:my-1.5 prose-ol:my-1.5 prose-li:my-0.5';

interface InvestigationMessagesProps {
  findingId: string;
  // The run whose thread this is: a new run reads its own thread instead of
  // showing the last one's.
  sessionId?: string;
  // While the run is active, the thread is read again as it grows.
  live?: boolean;
}

const LIVE_THREAD_POLL_MS = 5_000;
const FAILED_THREAD_RETRY_MS = 30_000;

/** Returns true if a message has no displayable content */
function isEmptyMessage(msg: ChatMessage): boolean {
  return !msg.content && !msg.reasoning_content && !msg.tool_calls?.length && !msg.tool_result;
}

// Each read parses new objects. A message that comes back unchanged keeps its
// object, so its row, and any tool output expanded in it, stays as it was.
function reuseIdenticalMessages(previous: ChatMessage[], next: ChatMessage[]): ChatMessage[] {
  const previousById = new Map(previous.map((message) => [message.id, message]));
  return next.map((message) => {
    const prior = previousById.get(message.id);
    return prior && JSON.stringify(prior) === JSON.stringify(message) ? prior : message;
  });
}

export const InvestigationMessages: Component<InvestigationMessagesProps> = (props) => {
  // The thread is read again only when its run changes, so a finished thread
  // keeps its rows, and any expanded tool output, across re-reads of the run.
  const thread = createMemo(
    () => ({ findingId: props.findingId, sessionId: props.sessionId ?? '' }),
    undefined,
    { equals: (a, b) => a.findingId === b.findingId && a.sessionId === b.sessionId },
  );
  // As in InvestigationSection, reads stay out of the app's route Suspense
  // boundary, whose fallback replaces the page and loses the reader's place.
  // The open thread is no longer remounted by findings refreshes, so it reads
  // itself again every 5s while its run is live, and 30s after a failed read.
  const messagesQuery = createNonSuspendingQuery<
    ChatMessage[],
    { findingId: string; sessionId: string }
  >({
    source: thread,
    cacheKey: ({ findingId, sessionId }) =>
      `patrol-investigation-messages:${findingId}:${sessionId}`,
    retainPreviousValueOnSourceChange: false,
    fetcher: async ({ findingId }): Promise<ChatMessage[]> => {
      const read = await withAPIErrorStatusNull(getInvestigationMessages(findingId), 404);
      // Read after the await: outside the query's source effect, so the
      // thread on screen is not tracked as a reason to read again.
      return reuseIdenticalMessages(messagesQuery.value(), read?.messages || []);
    },
    initialValue: [],
    pollMs: (): number | undefined =>
      props.live ? LIVE_THREAD_POLL_MS : messagesQuery.error() ? FAILED_THREAD_RETRY_MS : undefined,
  });
  // A run that finishes between two live reads gets one more, so the thread
  // ends with the run's last messages.
  createEffect(
    on(
      () => Boolean(props.live),
      (live, wasLive) => {
        if (wasLive && !live) void messagesQuery.refetch({ background: true });
      },
    ),
  );
  const messages = messagesQuery.value;
  const messagesState = createMemo(() =>
    getInvestigationMessagesState(!messagesQuery.resolvedOnce(), messages().length > 0),
  );
  // The thread stays mounted across refreshes, so its ages re-read on each
  // tick of the shared clock to keep moving.
  const now = useRelativeTimeNow();
  const formatAge = (timestamp: string) => {
    now();
    return formatTimestamp(timestamp);
  };

  return (
    <div class="mt-2">
      <Show when={messagesQuery.error()}>
        <InlineNotice
          role="alert"
          tone="warning"
          aria-label="Investigation thread read failed"
          class="mb-2"
        >
          <p class="font-medium">Could not load investigation messages.</p>
          <p>
            {getAPIReadAccessErrorMessage(messagesQuery.error()) ??
              (messages().length > 0
                ? 'Showing previously loaded messages. Newer messages may be missing.'
                : 'The saved thread could not be read. This does not mean it is empty.')}
          </p>
          <Button
            size="sm"
            disabled={messagesQuery.loading()}
            onClick={(e) => {
              e.stopPropagation();
              void messagesQuery.refetch();
            }}
          >
            {messagesQuery.loading() ? 'Retrying messages...' : 'Retry messages'}
          </Button>
        </InlineNotice>
      </Show>
      <Show when={!messagesQuery.error() && !messagesState().empty && messagesState().text}>
        <div class="flex items-center gap-2 text-xs text-muted py-2">
          <LoadingSpinner size="sm" />
          {messagesState().text}
        </div>
      </Show>

      <Show when={!messagesQuery.error() && messagesState().empty}>
        <p class="text-xs text-muted py-1">{messagesState().text}</p>
      </Show>

      <Show when={messages().length > 0}>
        <div class="space-y-2 max-h-80 overflow-y-auto rounded-sm border border-border p-2 bg-surface-alt">
          <For each={messages()}>
            {(msg: ChatMessage) => {
              if (isEmptyMessage(msg)) return null;

              return (
                <div class={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  <div
                    class={`max-w-[85%] rounded-md px-3 py-2 ${
                      msg.role === 'user'
                        ? 'bg-blue-100 dark:bg-blue-900/25 text-blue-900 dark:text-blue-100'
                        : msg.role === 'system'
                          ? 'bg-surface-alt text-muted text-xs'
                          : 'bg-surface text-base-content'
                    }`}
                  >
                    {/* Reasoning content (extended thinking) */}
                    <Show when={msg.reasoning_content}>
                      <details class="mb-1">
                        <summary class="text-[10px] text-muted cursor-pointer hover:text-base-content hover:underline">
                          Show reasoning
                        </summary>
                        <div class="mt-1 text-[10px] text-muted whitespace-pre-wrap wrap-break-word border-l-2 border-border pl-2">
                          {msg.reasoning_content}
                        </div>
                      </details>
                    </Show>

                    {/* Text content: assistant turns carry model markdown;
                        render it through the shared sanitizing renderer
                        instead of raw text. */}
                    <Show when={msg.content}>
                      <Show
                        when={msg.role === 'assistant'}
                        fallback={
                          <div class="text-xs whitespace-pre-wrap wrap-break-word">
                            {msg.content}
                          </div>
                        }
                      >
                        <div
                          class={investigationMarkdownClass}
                          // eslint-disable-next-line solid/no-innerhtml
                          innerHTML={renderMarkdown(msg.content)}
                        />
                      </Show>
                    </Show>

                    {/* Tool calls (assistant requesting tool use) */}
                    <Show when={msg.tool_calls && msg.tool_calls.length > 0}>
                      <div class="space-y-1">
                        <For each={msg.tool_calls}>
                          {(tc) => (
                            <Show
                              when={typeof tc.success === 'boolean'}
                              fallback={
                                <div class="text-xs rounded-sm border border-indigo-200 dark:border-indigo-800 bg-indigo-50 dark:bg-indigo-900/25 px-2 py-1">
                                  <span class="font-semibold text-indigo-700 dark:text-indigo-300">
                                    {tc.name}
                                  </span>
                                  <Show when={tc.input && Object.keys(tc.input).length > 0}>
                                    <pre class="mt-1 text-[10px] text-muted overflow-x-auto max-h-24 overflow-y-auto">
                                      {JSON.stringify(tc.input, null, 2)}
                                    </pre>
                                  </Show>
                                  <Show when={tc.output}>
                                    <pre class="mt-1 text-[10px] text-muted overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap wrap-break-word">
                                      {tc.output}
                                    </pre>
                                  </Show>
                                </div>
                              }
                            >
                              <ToolExecutionBlock
                                tool={{
                                  name: tc.name,
                                  input: JSON.stringify(tc.input),
                                  output: tc.output ?? '',
                                  success: tc.success!,
                                }}
                              />
                            </Show>
                          )}
                        </For>
                      </div>
                    </Show>

                    {/* Tool result (tool output returned to assistant) */}
                    <Show when={msg.tool_result}>
                      <div
                        class={`text-xs rounded border px-2 py-1 ${
                          msg.tool_result!.is_error
                            ? 'border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/25'
                            : 'border-green-200 dark:border-green-800 bg-green-50 dark:bg-green-900/25'
                        }`}
                      >
                        <span
                          class={`font-semibold text-[10px] ${
                            msg.tool_result!.is_error
                              ? 'text-red-700 dark:text-red-300'
                              : 'text-green-700 dark:text-green-300'
                          }`}
                        >
                          {msg.tool_result!.is_error ? 'Error' : 'Result'}
                        </span>
                        <pre class="mt-1 text-[10px] text-muted overflow-x-auto max-h-32 overflow-y-auto whitespace-pre-wrap wrap-break-word">
                          {msg.tool_result!.content}
                        </pre>
                      </div>
                    </Show>

                    <div class="text-[10px] text-muted mt-1">{formatAge(msg.timestamp)}</div>
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
