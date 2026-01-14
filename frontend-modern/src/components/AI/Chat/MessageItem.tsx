import { Component, Show, For, Switch, Match } from 'solid-js';
import { renderMarkdown } from '../aiChatUtils';
import { ThinkingBlock } from './ThinkingBlock';
import { ToolExecutionBlock, PendingToolBlock } from './ToolExecutionBlock';
import { ApprovalCard } from './ApprovalCard';
import type { ChatMessage, PendingApproval } from './types';

interface MessageItemProps {
  message: ChatMessage;
  onApprove: (approval: PendingApproval) => void;
  onSkip: (toolId: string) => void;
}

export const MessageItem: Component<MessageItemProps> = (props) => {
  const isUser = () => props.message.role === 'user';
  const hasStreamEvents = () =>
    props.message.streamEvents && props.message.streamEvents.length > 0;

  return (
    <div class={`flex ${isUser() ? 'justify-end' : 'justify-start'}`}>
      <div
        class={`max-w-[90%] rounded-2xl overflow-hidden transition-all ${
          isUser()
            ? 'bg-gradient-to-br from-purple-600 to-violet-600 text-white shadow-lg'
            : 'bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 shadow-md border border-gray-100 dark:border-gray-700'
        }`}
      >
        <div class="px-4 py-3">
          {/* User messages */}
          <Show when={isUser()}>
            <p class="text-sm whitespace-pre-wrap">{props.message.content}</p>
          </Show>

          {/* Assistant messages with stream events */}
          <Show when={!isUser()}>
            {/* Stream events (chronological order) */}
            <Show when={hasStreamEvents()}>
              <div class="space-y-3">
                <For each={props.message.streamEvents}>
                  {(evt) => (
                    <Switch>
                      <Match when={evt.type === 'thinking' && evt.thinking}>
                        <ThinkingBlock content={evt.thinking!} />
                      </Match>
                      <Match when={evt.type === 'tool' && evt.tool}>
                        <ToolExecutionBlock tool={evt.tool!} />
                      </Match>
                      <Match when={evt.type === 'content' && evt.content}>
                        <div
                          class="text-sm prose prose-sm dark:prose-invert max-w-none prose-pre:bg-gray-800 prose-pre:text-gray-100 prose-code:text-purple-600 dark:prose-code:text-purple-400 prose-code:before:content-none prose-code:after:content-none"
                          innerHTML={renderMarkdown(evt.content!)}
                        />
                      </Match>
                    </Switch>
                  )}
                </For>
              </div>
            </Show>

            {/* Pending tools (running) */}
            <Show when={props.message.pendingTools && props.message.pendingTools.length > 0}>
              <div class="mt-3 space-y-2">
                <For each={props.message.pendingTools}>
                  {(tool) => <PendingToolBlock tool={tool} />}
                </For>
              </div>
            </Show>

            {/* Fallback: show content if no stream events */}
            <Show when={props.message.content && !hasStreamEvents()}>
              <div
                class="text-sm prose prose-sm dark:prose-invert max-w-none prose-pre:bg-gray-800 prose-pre:text-gray-100 prose-code:text-purple-600 dark:prose-code:text-purple-400 prose-code:before:content-none prose-code:after:content-none"
                innerHTML={renderMarkdown(props.message.content)}
              />
            </Show>

            {/* Pending approvals */}
            <Show when={props.message.pendingApprovals && props.message.pendingApprovals.length > 0}>
              <div class="mt-4 space-y-3">
                <For each={props.message.pendingApprovals}>
                  {(approval) => (
                    <ApprovalCard
                      approval={approval}
                      onApprove={() => props.onApprove(approval)}
                      onSkip={() => props.onSkip(approval.toolId)}
                    />
                  )}
                </For>
              </div>
            </Show>

            {/* Streaming indicator */}
            <Show when={props.message.isStreaming && !props.message.pendingTools?.length}>
              <div class="mt-2 flex items-center gap-2 text-purple-500 dark:text-purple-400">
                <div class="flex gap-1">
                  <span class="w-1.5 h-1.5 bg-current rounded-full animate-bounce" style="animation-delay: 0ms" />
                  <span class="w-1.5 h-1.5 bg-current rounded-full animate-bounce" style="animation-delay: 150ms" />
                  <span class="w-1.5 h-1.5 bg-current rounded-full animate-bounce" style="animation-delay: 300ms" />
                </div>
              </div>
            </Show>
          </Show>
        </div>

        {/* Message metadata footer */}
        <Show when={!isUser() && props.message.model && !props.message.isStreaming}>
          <div class="px-4 py-1.5 bg-gray-50 dark:bg-gray-900/50 border-t border-gray-100 dark:border-gray-700 flex items-center justify-between text-[10px] text-gray-400 dark:text-gray-500">
            <span>{props.message.model}</span>
            <Show when={props.message.tokens}>
              <span>
                {props.message.tokens!.input + props.message.tokens!.output} tokens
              </span>
            </Show>
          </div>
        </Show>
      </div>
    </div>
  );
};
