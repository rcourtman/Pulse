import { Component, Show, For, Switch, Match, createMemo } from 'solid-js';
import { renderMarkdown } from '../aiChatUtils';
import { ThinkingBlock } from './ThinkingBlock';
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
        <div class="max-w-[85%] px-4 py-2.5 rounded-2xl rounded-br-md bg-blue-600 text-white shadow-sm">
          <p class="text-sm whitespace-pre-wrap">{props.message.content}</p>
        </div>
      </Show>

      {/* Assistant message - full width, terminal-like */}
      <Show when={!isUser()}>
        <div class="w-full">
          {/* Assistant indicator */}
          <div class="flex items-center gap-2 mb-2 text-xs text-slate-500 dark:text-slate-400">
            <div class="w-5 h-5 rounded-md bg-blue-600 flex items-center justify-center">
              <svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082" />
              </svg>
            </div>
            <span class="font-medium">Assistant</span>
            <Show when={props.message.model && !props.message.isStreaming}>
              <span class="text-[10px] text-slate-400 dark:text-slate-500">
                · {props.message.model}
              </span>
            </Show>
          </div>

          {/* Main content area */}
          <div class="pl-7">
            {/* Stream events - chronological display */}
            <Show when={hasStreamEvents()}>
              <For each={groupedEvents()}>
                {(evt) => (
                  <Switch>
                    {/* Thinking block - collapsed by default */}
                    <Match when={evt.type === 'thinking' && evt.thinking}>
                      <ThinkingBlock
                        content={evt.thinking || ''}
                        isStreaming={props.message.isStreaming}
                      />
                    </Match>

                    {/* Pending tool - hidden, we only show completed tools */}
                    <Match when={evt.type === 'pending_tool' && evt.pendingTool}>
                      <></>
                    </Match>

                    {/* Completed tool execution block */}
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
                        class="text-sm prose prose-slate prose-sm dark:prose-invert max-w-none overflow-x-auto
                               prose-p:leading-relaxed prose-p:my-2
                               prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-lg prose-pre:text-xs prose-pre:overflow-x-auto prose-pre:max-w-full
                               prose-code:text-blue-600 dark:prose-code:text-blue-400
                               prose-code:bg-blue-50 dark:prose-code:bg-blue-900/30
                               prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded prose-code:break-all
                               prose-code:before:content-none prose-code:after:content-none
                               prose-headings:text-slate-900 dark:prose-headings:text-slate-100
                               prose-strong:text-slate-900 dark:prose-strong:text-slate-100
                               prose-ul:my-2 prose-ol:my-2 prose-li:my-0.5"
                        // eslint-disable-next-line solid/no-innerhtml
                        innerHTML={renderMarkdown(evt.content || '')}
                      />
                    </Match>

                    {/* Approval card - inline in stream */}
                    <Match when={evt.type === 'approval' && evt.approval}>
                      <div class="my-3">
                        <ApprovalCard
                          approval={evt.approval!}
                          onApprove={() => props.onApprove(evt.approval!)}
                          onSkip={() => props.onSkip(evt.approval!.toolId)}
                        />
                      </div>
                    </Match>

                    {/* Question card - inline in stream */}
                    <Match when={evt.type === 'question' && evt.question}>
                      <div class="my-3">
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

            {/* Fallback: show content if no stream events */}
            <Show when={props.message.content && !hasStreamEvents()}>
              <div
                class="text-sm prose prose-slate prose-sm dark:prose-invert max-w-none overflow-x-auto
                       prose-p:leading-relaxed prose-p:my-2
                       prose-pre:bg-slate-900 prose-pre:text-slate-100 prose-pre:rounded-lg prose-pre:text-xs prose-pre:overflow-x-auto prose-pre:max-w-full
                       prose-code:text-blue-600 dark:prose-code:text-blue-400
                       prose-code:bg-blue-50 dark:prose-code:bg-blue-900/30
                       prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded prose-code:break-all
                       prose-code:before:content-none prose-code:after:content-none
                       prose-headings:text-slate-900 dark:prose-headings:text-slate-100
                       prose-strong:text-slate-900 dark:prose-strong:text-slate-100
                       prose-ul:my-2 prose-ol:my-2 prose-li:my-0.5"
                // eslint-disable-next-line solid/no-innerhtml
                innerHTML={renderMarkdown(props.message.content)}
              />
            </Show>

            {/* Streaming text indicator */}
            <Show when={isStreamingText()}>
              <span class="inline-block w-2 h-4 ml-0.5 bg-blue-500 dark:bg-blue-400 animate-pulse rounded-sm" />
            </Show>

            <Show when={!props.message.isStreaming && contextTools().length > 0}>
              <div class="mt-3 text-[10px] text-slate-400 dark:text-slate-500">
                Context used: {contextTools().map((name) => formatToolName(name)).join(', ')}
              </div>
            </Show>

            {/* Token count footer */}
            <Show when={props.message.tokens && !props.message.isStreaming}>
              <div class="mt-3 pt-2 border-t border-slate-100 dark:border-slate-800 text-[10px] text-slate-400 dark:text-slate-500">
                {props.message.tokens!.input + props.message.tokens!.output} tokens
                <span class="mx-1">·</span>
                {props.message.tokens!.input} in / {props.message.tokens!.output} out
              </div>
            </Show>
          </div>
        </div>
      </Show>
    </div>
  );
};
