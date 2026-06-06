import type { StreamDisplayEvent } from './types';

// Group raw chronological stream events into display blocks.
//
// The streamed answer text and reasoning arrive as many small deltas. Reasoning
// models reached through gateways like OpenRouter INTERLEAVE reasoning and
// answer tokens (thinking, content, thinking, content, ...) rather than sending
// all reasoning first. If each interleaved content delta is rendered as its own
// markdown block, block-level whitespace trimming collapses every inter-word
// space and a markdown table split across blocks never parses, so the answer
// renders as unreadable run-on text.
//
// To avoid that, content deltas merge into a single content block and reasoning
// deltas merge into a single thinking block, even when they arrive interleaved.
// Tool / approval / question / pending-tool events are genuine sequence
// boundaries (the model narrates around an action), so they close the open
// content and thinking blocks: text before and after an action stays ordered.
export const groupStreamEventsForDisplay = (
  events: StreamDisplayEvent[],
): StreamDisplayEvent[] => {
  const grouped: StreamDisplayEvent[] = [];
  // Indices of the currently-open content and thinking blocks, or -1 when none
  // is open. Only a hard-boundary event resets these.
  let contentIdx = -1;
  let thinkingIdx = -1;

  for (const evt of events) {
    switch (evt.type) {
      case 'content': {
        if (!evt.content) break; // skip empty deltas
        if (contentIdx >= 0) {
          grouped[contentIdx] = {
            ...grouped[contentIdx],
            content: (grouped[contentIdx].content || '') + evt.content,
          };
        } else {
          grouped.push({ ...evt });
          contentIdx = grouped.length - 1;
        }
        break;
      }

      case 'thinking': {
        if (!evt.thinking) break; // skip empty deltas
        if (thinkingIdx >= 0) {
          const current = grouped[thinkingIdx];
          grouped[thinkingIdx] = {
            ...current,
            thinking: (current.thinking || '') + evt.thinking,
            startedAt: current.startedAt || evt.startedAt,
            updatedAt: evt.updatedAt || current.updatedAt,
          };
        } else {
          grouped.push({ ...evt });
          thinkingIdx = grouped.length - 1;
        }
        break;
      }

      // Hard boundaries: a tool call (or the surfaces it can spawn) closes the
      // open text and reasoning blocks so any following text starts fresh and
      // stays after the action in the transcript.
      default: {
        grouped.push(evt);
        contentIdx = -1;
        thinkingIdx = -1;
        break;
      }
    }
  }

  return grouped;
};
