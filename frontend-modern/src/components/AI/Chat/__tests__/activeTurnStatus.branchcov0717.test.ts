import { describe, expect, it } from 'vitest';

import {
  formatAssistantWorkflowStatus,
  getAssistantActiveTurnStatus,
} from '@/components/AI/Chat/activeTurnStatus';
import type {
  ChatMessage,
  PendingTool,
  StreamDisplayEvent,
} from '@/components/AI/Chat/types';

// Fixture builders — mirror the conventions of the sibling branch-coverage tests.
const assistantMessage = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: 'assistant-1',
  role: 'assistant',
  content: '',
  timestamp: new Date(1_000),
  ...overrides,
});

const activeTurn = (overrides: Partial<ChatMessage> = {}, now?: number) =>
  getAssistantActiveTurnStatus([assistantMessage(overrides)], true, now);

const readToolInput = (command: string): string =>
  JSON.stringify({ action: 'exec', command, target_host: 'current_resource' });

describe('activeTurnStatus branch coverage (supplemental)', () => {
  // -------------------------------------------------------------------------
  // sanitizeWorkflowStatusMessage / formatWorkflowToolName — the `|| toolLabel`
  // fallback arm when the tool identifier is not in WORKFLOW_TOOL_LABELS.
  // -------------------------------------------------------------------------

  describe('formatWorkflowToolName fallback', () => {
    it('falls back to formatIdentifierLabel for a tool name absent from WORKFLOW_TOOL_LABELS', () => {
      // `WORKFLOW_TOOL_LABELS[identifier] || toolLabel` right arm: the
      // internalWorkflowToolIdentifiers set includes the custom normalized name
      // which has no map entry, so the friendly identifier label is used.
      expect(formatAssistantWorkflowStatus({ message: 'Working', tool: 'custom_thing' })).toBe(
        'Working · custom thing',
      );
    });
  });

  // -------------------------------------------------------------------------
  // remainingRetryDelay — the `startedAt`-missing return arm (now is a valid
  // number but startedAt is not), which returns retryAfterMs verbatim.
  // -------------------------------------------------------------------------

  describe('remainingRetryDelay startedAt-missing arm', () => {
    it('returns the full retryAfterMs when now is finite but startedAt is absent', () => {
      // Line 146: `typeof startedAt !== 'number'` is true -> return retryAfterMs
      // unchanged, so the countdown is not decremented.
      expect(
        formatAssistantWorkflowStatus({ message: 'Working', attempt: 1, retryAfterMs: 500 }, 1_000),
      ).toBe('Working · attempt 1 · retrying in 500ms');
    });
  });

  // -------------------------------------------------------------------------
  // formatPendingToolStatus — the ternary false arm `: inputSummary` when the
  // parsed input is a real (non-placeholder) summary and there is no command.
  // -------------------------------------------------------------------------

  describe('formatPendingToolStatus input-summary arm', () => {
    it('renders a parsed query summary when no command preview is available', () => {
      // toolActivity = commandPreview('') || (isPlaceholder('list resources') ? '' : 'list resources')
      //  -> the ternary's false arm yields the real summary.
      expect(
        activeTurn({
          pendingTools: [
            { id: 't1', name: 'pulse_query', input: '{"action":"list"}', status: 'running' },
          ],
        }),
      ).toEqual({ type: 'tool', text: 'Running list resources' });
    });
  });

  // -------------------------------------------------------------------------
  // activePendingToolFromEvents / latestStreamActivityStatus — the `if (!event)
  // continue` guard hit by a malformed (undefined) event entry.
  // -------------------------------------------------------------------------

  describe('undefined event guard', () => {
    it('skips an undefined streamEvents entry and still reports generating output', () => {
      // `content` is set so hasVisibleAssistantOutput short-circuits before it
      // would otherwise iterate the undefined element (which would throw).
      // Both event loops hit their `if (!event) continue` guard.
      expect(
        activeTurn({
          content: 'partial answer',
          streamEvents: [undefined as unknown as StreamDisplayEvent],
        }),
      ).toEqual({ type: 'generating', text: 'Generating response' });
    });
  });

  // -------------------------------------------------------------------------
  // isSelectedModelRouteEvent — the `event.failedModel?.trim()` access arm
  // (failedModel present, so .trim() is evaluated rather than short-circuited).
  // -------------------------------------------------------------------------

  describe('isSelectedModelRouteEvent failedModel access', () => {
    it('clears the placeholder flag when a selected route also carries a failed model', () => {
      // failedModel is a non-nullish string, so `?.trim()` is actually invoked;
      // the trimmed value is truthy so the placeholder flag becomes false.
      expect(
        activeTurn({
          streamEvents: [
            {
              type: 'model_switch',
              model: 'openrouter:qwen/qwen3.7-plus',
              modelEvent: 'selected',
              failedModel: 'openrouter:openai/gpt-4o-mini',
              startedAt: 2_000,
              updatedAt: 2_000,
            },
          ],
        }),
      ).toEqual({
        type: 'thinking',
        text: 'Using Qwen: Qwen3.7 Plus via OpenRouter',
        startedAt: 2_000,
      });
    });
  });

  // -------------------------------------------------------------------------
  // activePendingToolCandidateFromState — the `if (!candidate) return current`
  // arm, hit when a pendingTools entry yields no status text.
  // -------------------------------------------------------------------------

  describe('activePendingToolCandidateFromState null-candidate arm', () => {
    it('skips a pendingTools entry that produces no status text', () => {
      // pendingToolCandidate(undefined) -> formatPendingToolStatus('') -> null,
      // so the reducer returns its current accumulator unchanged.
      expect(
        activeTurn({ pendingTools: [undefined as unknown as PendingTool] }),
      ).toEqual({ type: 'thinking', text: 'Sending prompt.', startedAt: 1_000 });
    });
  });

  // -------------------------------------------------------------------------
  // eventActivityAt (workflow_status arm) — each `||` fall-through level.
  // -------------------------------------------------------------------------

  describe('eventActivityAt workflow_status fall-through arms', () => {
    it('falls through to event.updatedAt when workflowStatus.startedAt is absent', () => {
      // `workflowStatus?.startedAt || event.updatedAt || ...` -> updatedAt wins.
      expect(
        activeTurn({
          streamEvents: [
            {
              type: 'workflow_status',
              workflowStatus: { message: 'Working on it.' },
              updatedAt: 5_000,
              startedAt: 3_000,
            },
          ],
        }),
      ).toEqual({ type: 'thinking', text: 'Working on it.', startedAt: 3_000 });
    });

    it('falls through to event.startedAt when updatedAt is also absent', () => {
      // `... || event.updatedAt || event.startedAt || undefined` -> startedAt.
      expect(
        activeTurn({
          streamEvents: [
            {
              type: 'workflow_status',
              workflowStatus: { message: 'Working on it.' },
              startedAt: 7_000,
            },
          ],
        }),
      ).toEqual({ type: 'thinking', text: 'Working on it.', startedAt: 7_000 });
    });

    it('returns undefined when no timestamp source is present at all', () => {
      // The final `|| undefined` arm: activityAt is undefined.
      expect(
        activeTurn({
          streamEvents: [{ type: 'workflow_status', workflowStatus: { message: 'Working on it.' } }],
        }),
      ).toEqual({ type: 'thinking', text: 'Working on it.' });
    });
  });

  // -------------------------------------------------------------------------
  // isFresherStatusCandidate — the `candidate.genericContent &&
  // current.currentWorkflow` false arm (an older content row vs a newer
  // workflow row keeps the workflow candidate).
  // -------------------------------------------------------------------------

  describe('isFresherStatusCandidate genericContent-vs-currentWorkflow arm', () => {
    it('keeps the newer workflow row over an earlier generic content row', () => {
      // Iteration is end-to-start: the later workflow_status becomes current
      // (currentWorkflow), then the earlier content is the candidate
      // (genericContent) -> returns false, so the workflow row is retained.
      expect(
        activeTurn(
          {
            streamEvents: [
              { type: 'content', content: 'partial', startedAt: 1_000, updatedAt: 1_000 },
              {
                type: 'workflow_status',
                workflowStatus: {
                  phase: 'stream_idle',
                  message: 'Still working.',
                  startedAt: 2_000,
                },
                startedAt: 2_000,
                updatedAt: 2_000,
              },
            ],
          },
          3_000,
        ),
      ).toEqual({ type: 'thinking', text: 'Still working.', startedAt: 2_000 });
    });
  });

  // -------------------------------------------------------------------------
  // isFresherStatusCandidate — the `candidateTime === undefined &&
  // currentTime !== undefined` false arm.
  // -------------------------------------------------------------------------

  describe('isFresherStatusCandidate undefined-candidateTime arm', () => {
    it('keeps the row that has a timestamp over one without', () => {
      // The earlier thinking event has no startedAt/updatedAt (activityAt
      // undefined); the later content event has updatedAt 500. The candidate
      // with an undefined time loses to the current with a defined time.
      expect(
        activeTurn({
          streamEvents: [
            { type: 'thinking', thinking: '**Alpha**' },
            { type: 'content', content: 'hi', startedAt: 500, updatedAt: 500 },
          ],
        }),
      ).toEqual({ type: 'generating', text: 'Generating response', startedAt: 500 });
    });
  });

  // -------------------------------------------------------------------------
  // isGenericToolProgress — the uncovered equality arms.
  // -------------------------------------------------------------------------

  describe('isGenericToolProgress equality arms', () => {
    it.each([
      'Running command',
      'Running read-only command',
      'Running read-only command.',
    ])('treats %q generic progress as generic when a command preview exists', (progress) => {
      // A real command preview plus a generic progress string forces
      // isGenericToolProgress to be evaluated (commandPreview truthy ->
      // !commandPreview is false -> !isGenericToolProgress(progress) runs).
      expect(
        activeTurn({
          pendingTools: [
            {
              id: 't1',
              name: 'pulse_read',
              input: readToolInput('uptime'),
              progress,
              status: 'running',
            },
          ],
        }),
      ).toEqual({ type: 'tool', text: 'Running $ uptime' });
    });
  });

  // -------------------------------------------------------------------------
  // modelSwitchStatusText — the `if (!model) return ''` arm.
  // -------------------------------------------------------------------------

  describe('modelSwitchStatusText empty-model arm', () => {
    it('produces no candidate for a model_switch event with no model', () => {
      // model is undefined -> text '' -> no candidate -> falls back to the
      // initial request status.
      expect(activeTurn({ streamEvents: [{ type: 'model_switch' }] })).toEqual({
        type: 'thinking',
        text: 'Sending prompt.',
        startedAt: 1_000,
      });
    });
  });

  // -------------------------------------------------------------------------
  // hasVisibleAssistantOutput — the `message.streamEvents || []` short-circuit
  // arm, reached when an assistant turn carries no streamEvents at all.
  // -------------------------------------------------------------------------

  describe('hasVisibleAssistantOutput missing-streamEvents arm', () => {
    it('falls back to the initial request status when the turn has no events or output', () => {
      // message.streamEvents is undefined -> `|| []` is evaluated, so the
      // .some() callback never runs and hasVisibleAssistantOutput is false.
      expect(activeTurn({})).toEqual({
        type: 'thinking',
        text: 'Sending prompt.',
        startedAt: 1_000,
      });
    });
  });

  // -------------------------------------------------------------------------
  // messageTimestampMs — the `Number.isFinite(value) ? value : undefined`
  // false arm for an invalid Date.
  // -------------------------------------------------------------------------

  describe('messageTimestampMs invalid-date arm', () => {
    it('drops the startedAt when the assistant message timestamp is an invalid Date', () => {
      // getTime() returns NaN -> !Number.isFinite -> return undefined, so the
      // initial request status is emitted without a startedAt.
      expect(activeTurn({ timestamp: new Date(NaN) })).toEqual({
        type: 'thinking',
        text: 'Sending prompt.',
      });
    });
  });
});
