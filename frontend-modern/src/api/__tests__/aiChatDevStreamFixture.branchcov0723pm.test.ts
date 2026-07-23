import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  maybeRunAIChatDevStreamFixture,
  type AIChatDevStreamFixtureOptions,
} from '@/api/aiChatDevStreamFixture';
import type { AIChatStreamEvent } from '@/api/generated/aiChatEvents';

const flushMicrotasks = async () => {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    await Promise.resolve();
  }
};

const STEP_DELAY_MS = 20;

type TimelineEntry = {
  ms: number;
  type: string;
  phase?: string;
};

const workflowPhase = (event: AIChatStreamEvent): string | undefined =>
  event.type === 'workflow_state' ? (event.data as { phase?: string }).phase : undefined;

const collectPacedTimeline = async (prompt: string, model?: string): Promise<TimelineEntry[]> => {
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  const timeline: TimelineEntry[] = [];
  let elapsedMs = 0;
  const options: AIChatDevStreamFixtureOptions = {
    prompt,
    model,
    stepDelayMs: STEP_DELAY_MS,
    onEvent: (event) => {
      timeline.push({ ms: elapsedMs, type: event.type, phase: workflowPhase(event) });
    },
  };

  const streamPromise = maybeRunAIChatDevStreamFixture(options);
  await flushMicrotasks();

  // Advance the fake clock 1ms at a time so each recorded ms-offset reflects the
  // exact hold delay applied after the preceding event. Increment before
  // advancing so the offset is correct at the moment the timer callback fires.
  let guard = 0;
  while (vi.getTimerCount() > 0 && guard < 20000) {
    guard += 1;
    elapsedMs += 1;
    await vi.advanceTimersByTimeAsync(1);
  }

  await expect(streamPromise).resolves.toBe(true);
  return timeline;
};

const gapAfterFirstMatching = (
  timeline: TimelineEntry[],
  predicate: (entry: TimelineEntry) => boolean,
): number => {
  const index = timeline.findIndex(predicate);
  expect(index).toBeGreaterThanOrEqual(0);
  expect(index).toBeLessThan(timeline.length - 1);
  return timeline[index + 1].ms - timeline[index].ms;
};

describe('aiChatDevStreamFixture — branch coverage (0723pm)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  describe('abortError', () => {
    it('rejects with a DOM-shaped AbortError and emits nothing when the signal is already aborted', async () => {
      const controller = new AbortController();
      controller.abort();
      const onEvent = vi.fn();

      const streamPromise = maybeRunAIChatDevStreamFixture({
        prompt: '/fixture tool-burst',
        onEvent,
        signal: controller.signal,
      });

      await expect(streamPromise).rejects.toMatchObject({
        name: 'AbortError',
        message: 'The operation was aborted.',
      });
      // Early termination: the very first throwIfAborted at the top of the loop
      // fires before any event is dispatched to the consumer.
      expect(onEvent).not.toHaveBeenCalled();
    });

    it('catches an abort raised from inside onEvent at the next waitForFixtureStep gate (zero-delay pacing)', async () => {
      const controller = new AbortController();
      const seen: string[] = [];
      const onEvent = vi.fn((event: AIChatStreamEvent) => {
        seen.push(event.type);
        // Abort once the second event lands; the subsequent waitForFixtureStep
        // entry guard must surface the AbortError before event three.
        if (seen.length === 2) {
          controller.abort();
        }
      });

      const streamPromise = maybeRunAIChatDevStreamFixture({
        prompt: '/fixture send-hold',
        onEvent,
        signal: controller.signal,
      });

      await expect(streamPromise).rejects.toMatchObject({ name: 'AbortError' });
      expect(seen).toEqual(['session', 'workflow_state']);
    });
  });

  describe('handleAbort (mid-stream abort while a paced step is in flight)', () => {
    it('tears down the pending wait, rejects with AbortError, registers the listener once, and emits no further events', async () => {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
      const controller = new AbortController();
      const addSpy = vi.spyOn(controller.signal, 'addEventListener');
      const removeSpy = vi.spyOn(controller.signal, 'removeEventListener');
      const onEvent = vi.fn();

      const streamPromise = maybeRunAIChatDevStreamFixture({
        prompt: '/fixture tool-chain',
        stepDelayMs: 100,
        onEvent,
        signal: controller.signal,
      });

      // First event (session) is dispatched synchronously before the first
      // paced wait, so exactly one listener is now attached to a pending timer.
      await flushMicrotasks();
      expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['session']);
      expect(vi.getTimerCount()).toBe(1);
      expect(addSpy).toHaveBeenCalledTimes(1);
      expect(addSpy).toHaveBeenCalledWith('abort', expect.any(Function), { once: true });

      // Consumer aborts mid-wait: handleAbort must run, clear the timer,
      // detach its listener, and reject with a fresh abortError.
      controller.abort();

      expect(vi.getTimerCount()).toBe(0);
      expect(removeSpy).toHaveBeenCalledTimes(1);
      expect(removeSpy).toHaveBeenCalledWith('abort', expect.any(Function));

      await expect(streamPromise).rejects.toMatchObject({
        name: 'AbortError',
        message: 'The operation was aborted.',
      });
      // No further events after the abort.
      expect(onEvent.mock.calls.map(([event]) => event.type)).toEqual(['session']);
    });

    it('does not register an abort listener when no signal is supplied, even with pacing enabled', async () => {
      vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
      const onEvent = vi.fn();

      const streamPromise = maybeRunAIChatDevStreamFixture({
        prompt: '/fixture tool-burst',
        stepDelayMs: 5,
        onEvent,
      });

      await flushMicrotasks();
      // No signal -> the optional-chaining addEventListener arm is skipped, but
      // a timer is still scheduled for the paced wait.
      expect(vi.getTimerCount()).toBe(1);

      await vi.runAllTimersAsync();
      await expect(streamPromise).resolves.toBe(true);
      expect(onEvent.mock.calls[onEvent.mock.calls.length - 1][0].type).toBe('done');
    });
  });

  describe('command resolution early-outs', () => {
    it('returns false and emits nothing for a non-fixture prompt', async () => {
      const onEvent = vi.fn();
      const ran = await maybeRunAIChatDevStreamFixture({
        prompt: 'show me the latest alerts',
        onEvent,
      });

      expect(ran).toBe(false);
      expect(onEvent).not.toHaveBeenCalled();
    });

    it('treats a bare "/fixture" as a command but an unknown fixture (error arm)', async () => {
      const onEvent = vi.fn();
      const ran = await maybeRunAIChatDevStreamFixture({ prompt: '/fixture', onEvent });

      expect(ran).toBe(true);
      const typesList = onEvent.mock.calls.map(([event]) => event.type);
      expect(typesList).toEqual(['session', 'workflow_state', 'error']);
      const errorEvent = onEvent.mock.calls[2][0];
      expect(errorEvent.type).toBe('error');
      expect((errorEvent.data as { message: string }).message).toContain(
        'Unknown Assistant fixture',
      );
    });
  });

  describe('fixtureStepDelay pacing arms (uncovered sequencing holds)', () => {
    it('holds the provider_retry workflow_state for 1600ms (provider-retry fixture)', async () => {
      const timeline = await collectPacedTimeline('/fixture provider-retry');
      const gap = gapAfterFirstMatching(
        timeline,
        (entry) => entry.type === 'workflow_state' && entry.phase === 'provider_retry',
      );
      expect(gap).toBe(1600);
    });

    it('holds the pending-tool tool_progress for 2200ms and falls back to the default delay otherwise', async () => {
      const timeline = await collectPacedTimeline('/fixture pending-tool');
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'tool_progress')).toBe(2200);
      // Fallthrough arm: `if (normalizedPrompt === '/fixture pending-tool') return defaultDelayMs;`
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'session')).toBe(
        STEP_DELAY_MS,
      );
    });

    it('holds the send-hold session for 1400ms', async () => {
      const timeline = await collectPacedTimeline('/fixture send-hold');
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'session')).toBe(1400);
    });

    it('holds context-group tool_start for 900ms and tool_end for 180ms', async () => {
      const timeline = await collectPacedTimeline('/fixture context-group');
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'tool_start')).toBe(900);
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'tool_end')).toBe(180);
    });

    it('holds the stream-idle stream_idle workflow_state for 1800ms', async () => {
      const timeline = await collectPacedTimeline('/fixture stream-idle');
      expect(
        gapAfterFirstMatching(
          timeline,
          (entry) => entry.type === 'workflow_state' && entry.phase === 'stream_idle',
        ),
      ).toBe(1800);
    });

    it('holds the queue-hold stream_idle workflow_state for 10000ms', async () => {
      const timeline = await collectPacedTimeline('/fixture queue-hold');
      expect(
        gapAfterFirstMatching(
          timeline,
          (entry) => entry.type === 'workflow_state' && entry.phase === 'stream_idle',
        ),
      ).toBe(10000);
    });

    it('holds the workflow-burst provider_start workflow_state for 2400ms', async () => {
      const timeline = await collectPacedTimeline('/fixture workflow-burst');
      expect(
        gapAfterFirstMatching(
          timeline,
          (entry) => entry.type === 'workflow_state' && entry.phase === 'provider_start',
        ),
      ).toBe(2400);
    });

    it('holds the long-output content event for 1800ms', async () => {
      const timeline = await collectPacedTimeline('/fixture long-output');
      expect(gapAfterFirstMatching(timeline, (entry) => entry.type === 'content')).toBe(1800);
    });
  });

  describe('model/provider resolution fallbacks', () => {
    const collectEvents = async (prompt: string, model?: string) => {
      const onEvent = vi.fn();
      const ran = await maybeRunAIChatDevStreamFixture({ prompt, model, onEvent });
      expect(ran).toBe(true);
      return onEvent.mock.calls.map(([event]) => event) as AIChatStreamEvent[];
    };

    it('falls back to the "dev" provider when the fixture model has no provider separator', async () => {
      // providerFromFixtureModel: indexOf(':') <= 0 -> provider '' -> `|| 'dev'`.
      const events = await collectEvents('/fixture provider-retry', 'standalone-model');
      const retryState = events.find(
        (event) =>
          event.type === 'workflow_state' &&
          (event.data as { phase?: string }).phase === 'provider_retry',
      );
      expect(retryState).toBeDefined();
      expect((retryState!.data as { provider?: string }).provider).toBe('dev');
      const done = events.find((event) => event.type === 'done');
      expect((done!.data as { model?: string }).model).toBe('standalone-model');
    });

    it('falls back to the default fixture model when the supplied model is blank whitespace', async () => {
      // assistantFixtureModel: model?.trim() || 'dev:assistant-stream-fixture'.
      const events = await collectEvents('/fixture provider-retry', '   ');
      const done = events.find((event) => event.type === 'done');
      expect((done!.data as { model?: string }).model).toBe('dev:assistant-stream-fixture');
    });
  });
});
