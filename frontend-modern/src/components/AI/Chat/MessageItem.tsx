import { Component, Show, For, Switch, Match, createMemo } from 'solid-js';
import { renderMarkdown } from '../aiChatUtils';
import { ThinkingBlock } from './ThinkingBlock';
import { ExploreStatusBlock } from './ExploreStatusBlock';
import { ToolExecutionBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import { QuestionCard } from './QuestionCard';
import type { ChatMessage, PendingApproval, PendingQuestion, StreamDisplayEvent } from './types';

interface MessageItemProps {
  message: ChatMessage;
  onApprove: (approval: PendingApproval) => void;
  onSkip: (toolId: string) => void;
  onAnswerQuestion: (question: PendingQuestion, answers: Array<{ id: string; value: string }>) => void;
  onSkipQuestion: (questionId: string) => void;
}

/**
 * MessageItem - Renders a single message in the chat.
 * 
 * User messages: Compact, right-aligned bubble
 * Assistant messages: Full-width, terminal-like with clear sections
 */
export const MessageItem: Component<MessageItemProps> = (props) => {
  const isUser = () => props.message.role === 'user';

  const hasStreamEvents = () =>
    props.message.streamEvents && props.message.streamEvents.length > 0;

  // Group stream events for cleaner rendering
  // Combine consecutive content events, separate thinking, tools, and approvals
  const groupedEvents = createMemo(() => {
    const events = props.message.streamEvents || [];
    const grouped: StreamDisplayEvent[] = [];

    for (const evt of events) {
      // Thinking events are kept separate
      if (evt.type === 'thinking') {
        grouped.push(evt);
        continue;
      }

      // Explore status events are kept separate
      if (evt.type === 'explore_status') {
        grouped.push(evt);
        continue;
      }

      // Tool events are kept separate
      if (evt.type === 'tool') {
        grouped.push(evt);
        continue;
      }

      // Pending tool events are kept separate
      if (evt.type === 'pending_tool') {
        grouped.push(evt);
        continue;
      }

      // Approval events are kept separate
      if (evt.type === 'approval') {
        grouped.push(evt);
        continue;
      }

      // Question events are kept separate
      if (evt.type === 'question') {
        grouped.push(evt);
        continue;
      }

      // Content events can be merged with previous content
      if (evt.type === 'content' && evt.content) {
        const lastIdx = grouped.length - 1;
        if (lastIdx >= 0 && grouped[lastIdx].type === 'content') {
          grouped[lastIdx] = {
            ...grouped[lastIdx],
            content: (grouped[lastIdx].content || '') + evt.content,
          };
        } else {
          grouped.push(evt);
        }
      }
    }

    return grouped;
  });

  const contextTools = createMemo(() => {
    const events = props.message.streamEvents || [];
    const names = new Set<string>();

    for (const evt of events) {
      if (evt.type === 'tool' && evt.tool?.name) {
        names.add(evt.tool.name);
      }
    }

    return Array.from(names);
  });

  const formatToolName = (name: string) =>
    name.replace(/^pulse_/, '').replace(/_/g, ' ');

  // Check if currently streaming content (no tools pending, still streaming)
  const isStreamingText = () =>
    props.message.isStreaming &&
    (!props.message.pendingTools || props.message.pendingTools.length === 0);

  return (
    <div class={`${isUser() ? 'flex justify-end' : ''} mb-4`}>
      {/* User message - compact bubble */}
      <Show when={isUser()}>
        <div class="max-w-[85%] px-4 py-2.5 rounded-md rounded-br-sm bg-blue-600 text-white shadow-sm">
          <p class="text-sm whitespace-pre-wrap">{props.message.content}</p>
        </div>
      </Show>

      {/* Assistant message - card style */}
      <Show when={!isUser()}>
        <div class="w-full pl-2 pr-2">
          <div class="group relative bg-slate-50 dark:bg-slate-800 rounded-md border border-slate-200 dark:border-slate-700 p-5 shadow-sm transition-all hover:border-slate-300 dark:hover:border-slate-600">
            {/* Assistant indicator */}
            <div class="flex items-center gap-2.5 mb-3">
              <div class="w-6 h-6 rounded-md bg-white dark:bg-slate-700 border border-slate-100 dark:border-slate-600 shadow-sm flex items-center justify-center shrink-0">
                <svg class="w-3.5 h-3.5 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456zM16.894 20.567L16.5 21.75l-.394-1.183a2.25 2.25 0 00-1.423-1.423L13.5 18.75l1.183-.394a2.25 2.25 0 001.423-1.423l.394-1.183.394 1.183a2.25 2.25 0 001.423 1.423l1.183.394-1.183.394a2.25 2.25 0 00-1.423 1.423z" />
                </svg>
              </div>
              <div class="flex items-baseline gap-2">
                <span class="text-xs font-semibold text-slate-700 dark:text-slate-200">Assistant</span>
                <Show when={props.message.model && !props.message.isStreaming}>
                  <span class="text-[10px] text-slate-400 dark:text-slate-500 font-mono">
                    {props.message.model}
                  </span>
                </Show>
              </div>
            </div>

            {/* Main content area */}
            <div class="pl-1">
              {/* Stream events - chronological display */}
              <Show when={hasStreamEvents()}>
                <For each={groupedEvents()}>
                  {(evt) => (
                    <Switch>
                      {/* Thinking block */}
                      <Match when={evt.type === 'thinking' && evt.thinking}>
                        <ThinkingBlock
                          content={evt.thinking || ''}
                          isStreaming={props.message.isStreaming}
                        />
                      </Match>

                      <Match when={evt.type === 'explore_status' && evt.exploreStatus}>
                        <ExploreStatusBlock status={evt.exploreStatus!} />
                      </Match>

                      <Match when={evt.type === 'pending_tool' && evt.pendingTool}>
                        <></>
                      </Match>

                      <Match when={evt.type === 'tool' && evt.tool}>
                        <ToolExecutionBlock tool={{
                          name: evt.tool?.name || 'unknown',
                          input: evt.tool?.input || '{}',
                          output: evt.tool?.output || '',
                          success: evt.tool?.success ?? true,
                        }} />
                      </Match>

                      {/* Content/text block */}
                      <Match when={evt.type === 'content' && evt.content}>
                        <div
                          class="text-sm prose prose-slate prose-sm dark:prose-invert max-w-none prose-p:leading-relaxed prose-p:my-2 prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-md prose-pre:text-xs prose-pre:border prose-pre:border-slate-800 prose-code:text-blue-700 dark:prose-code:text-blue-300 prose-code:bg-blue-50 dark:prose-code:bg-blue-900 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-md prose-code:font-mono prose-code:text-[0.9em] prose-code:border prose-code:border-blue-100 dark:prose-code:border-blue-800 prose-code:before:content-none prose-code:after:content-none prose-headings:font-semibold prose-headings:tracking-tight prose-hr:border-slate-200 dark:prose-hr:border-slate-700 prose-ul:my-2 prose-ol:my-2 prose-li:my-1"
                          // eslint-disable-next-line solid/no-innerhtml
                          innerHTML={renderMarkdown(evt.content || '')}
                        />
                      </Match>

                      <Match when={evt.type === 'approval' && evt.approval}>
                        <div class="my-4">
                          <ApprovalCard
                            approval={evt.approval!}
                            onApprove={() => props.onApprove(evt.approval!)}
                            onSkip={() => props.onSkip(evt.approval!.toolId)}
                          />
                        </div>
                      </Match>

                      <Match when={evt.type === 'question' && evt.question}>
                        <div class="my-4">
                          <QuestionCard
                            question={evt.question!}
                            onAnswer={(answers) => props.onAnswerQuestion(evt.question!, answers)}
                            onSkip={() => props.onSkipQuestion(evt.question!.questionId)}
                          />
                        </div>
                      </Match>
                    </Switch>
                  )}
                </For>
              </Show>

              {/* Fallback */}
              <Show when={props.message.content && !hasStreamEvents()}>
                <div
                  class="text-sm prose prose-slate prose-sm dark:prose-invert max-w-none prose-p:leading-relaxed prose-p:my-2 prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-md prose-pre:text-xs prose-pre:border prose-pre:border-slate-800 prose-code:text-blue-700 dark:prose-code:text-blue-300 prose-code:bg-blue-50 dark:prose-code:bg-blue-900 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded-md prose-code:font-mono prose-code:text-[0.9em] prose-code:border prose-code:border-blue-100 dark:prose-code:border-blue-800 prose-code:before:content-none prose-code:after:content-none prose-headings:font-semibold prose-headings:tracking-tight prose-hr:border-slate-200 dark:prose-hr:border-slate-700 prose-ul:my-2 prose-ol:my-2 prose-li:my-1"
                  // eslint-disable-next-line solid/no-innerhtml
                  innerHTML={renderMarkdown(props.message.content)}
                />
              </Show>

              {/* Streaming cursor */}
              <Show when={isStreamingText()}>
                <span class="inline-block w-1.5 h-4 ml-0.5 align-middle bg-blue-500 dark:bg-blue-400 animate-pulse rounded-full" />
              </Show>

              <Show when={!props.message.isStreaming && contextTools().length > 0}>
                <div class="mt-4 pt-3 border-t border-slate-100 dark:border-slate-700 flex flex-wrap gap-2">
                  <span class="text-[10px] uppercase font-semibold text-slate-400 dark:text-slate-500 tracking-wider">Context used</span>
                  <div class="flex flex-wrap gap-1.5">
                    {contextTools().map((name) => (
                      <span class="px-1.5 py-0.5 rounded text-[10px] bg-slate-100 dark:bg-slate-700 text-slate-500 dark:text-slate-300 border border-slate-200 dark:border-slate-600 font-medium">
                        {formatToolName(name)}
                      </span>
                    ))}
                  </div>
                </div>
              </Show>

              <Show when={props.message.tokens && !props.message.isStreaming}>
                <div class="mt-1 flex justify-end">
                  <span class="text-[9px] text-slate-300 dark:text-slate-600 font-mono">
                    {props.message.tokens!.input} in · {props.message.tokens!.output} out
                  </span>
                </div>
              </Show>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
};
