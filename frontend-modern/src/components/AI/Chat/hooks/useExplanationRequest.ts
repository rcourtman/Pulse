import { createEffect, onCleanup, untrack, type Accessor } from 'solid-js';
import { aiChatStore, type AIChatContext } from '@/stores/aiChat';
import type { SendMessageOptions } from './useChat';

export const EXPLAIN_SELECTED_ISSUE_PROMPT =
  'Explain this issue using current evidence. What needs my attention, and what is the next useful step? Do not make changes.';

export function explanationSendOptions(context: AIChatContext): SendMessageOptions {
  return {
    autonomousMode: false,
    handoffContext: context.handoffContext,
    handoffResources: context.handoffResources,
    handoffActions: context.handoffActions,
    handoffMetadata: context.handoffMetadata,
  };
}

// Use the normal send/queue/retry path. A contextual request must not erase an
// unsent draft, stop another response, or bypass action approval.
export function useExplanationRequest(options: {
  isOpen: Accessor<boolean>;
  prepare: () => Promise<void>;
  send: (
    prompt: string,
    findingId: string | undefined,
    options: SendMessageOptions,
  ) => Promise<boolean>;
  onError: (error: unknown) => void;
}) {
  let disposed = false;
  onCleanup(() => {
    disposed = true;
  });
  createEffect(() => {
    const request = aiChatStore.explanationRequestSignal?.();
    if (!request || !options.isOpen()) return;
    aiChatStore.ackExplanationRequest(request.id);
    untrack(() => {
      void options
        .prepare()
        .then(() =>
          disposed || request.signal.aborted
            ? false
            : options.send(
                EXPLAIN_SELECTED_ISSUE_PROMPT,
                request.context.findingId,
                explanationSendOptions(request.context),
              ),
        )
        .then((accepted) => {
          if (accepted) aiChatStore.clearRequestHandoffPayload(request.context);
        })
        .catch(options.onError);
    });
  });
}
