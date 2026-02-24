/**
 * SolidJS hook for consuming live patrol SSE events.
 *
 * Opens an EventSource to /api/ai/patrol/stream while the patrol is running,
 * exposing reactive signals for the current phase, active tool, and token count.
 * Automatically reconnects with exponential backoff on connection drops.
 */

import { createSignal, createEffect, onCleanup, type Accessor } from 'solid-js';
import type { PatrolStreamEvent } from '@/api/patrol';
import { logger } from '@/utils/logger';

export interface UsePatrolStreamOptions {
  /** Reactive boolean – stream is only open while this returns true. */
  running: Accessor<boolean>;
  /** Called when the SSE connection opens successfully. */
  onStart?: () => void;
  /** Called when a 'complete' event is received. */
  onComplete?: () => void;
  /** Called when an 'error' event is received or the SSE connection fails. */
  onError?: () => void;
}

export interface PatrolStreamState {
  /** Current phase text (e.g. "Analyzing resources…"). */
  phase: Accessor<string>;
  /** Name of the tool currently executing, or empty. */
  currentTool: Accessor<string>;
  /** Cumulative token count reported by the backend. */
  tokens: Accessor<number>;
  /** True if we had to resync via snapshot due to reconnect/buffer rotation. */
  resynced: Accessor<boolean>;
  /** Why we were resynced (if any). */
  resyncReason: Accessor<string>;
  /** Buffered seq window advertised by the backend (snapshot only). */
  bufferStartSeq: Accessor<number>;
  bufferEndSeq: Accessor<number>;
  /** True when the snapshot output was truncated by the backend tail buffer. */
  outputTruncated: Accessor<boolean>;
  /** Count of successful reconnects during the current run. */
  reconnectCount: Accessor<number>;
  /** True while the EventSource is connected. */
  isStreaming: Accessor<boolean>;
  /** Error message from the last error event, or empty. */
  errorMessage: Accessor<string>;
}

const MAX_RECONNECT_ATTEMPTS = 5;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 15000;
const MAX_SSE_EVENT_DATA_CHARS = 64 * 1024;
const MAX_STREAM_TEXT_CHARS = 4096;
const MAX_RUN_ID_CHARS = 256;
const MAX_LAST_EVENT_ID_CHARS = 20;
const MAX_TOKEN_COUNT = 10_000_000;

const clampStreamText = (value: unknown): string | undefined => {
  if (typeof value !== 'string') {
    return undefined;
  }
  if (value.length <= MAX_STREAM_TEXT_CHARS) {
    return value;
  }
  return value.slice(0, MAX_STREAM_TEXT_CHARS);
};

const normalizeRunId = (value: unknown): string => {
  if (typeof value !== 'string') {
    return '';
  }
  if (value.length === 0 || value.length > MAX_RUN_ID_CHARS) {
    return '';
  }
  return value;
};

const normalizeTokenCount = (value: unknown): number | undefined => {
  if (typeof value !== 'number' || !Number.isFinite(value) || value < 0) {
    return undefined;
  }
  return Math.min(Math.floor(value), MAX_TOKEN_COUNT);
};

const normalizeResyncReason = (
  value: unknown,
): '' | 'late_joiner' | 'stale_last_event_id' | 'buffer_rotated' => {
  if (value === 'late_joiner' || value === 'stale_last_event_id' || value === 'buffer_rotated') {
    return value;
  }
  return '';
};

export function usePatrolStream(opts: UsePatrolStreamOptions): PatrolStreamState {
  const [phase, setPhase] = createSignal('');
  const [currentTool, setCurrentTool] = createSignal('');
  const [tokens, setTokens] = createSignal(0);
  const [resynced, setResynced] = createSignal(false);
  const [resyncReason, setResyncReason] = createSignal('');
  const [bufferStartSeq, setBufferStartSeq] = createSignal(0);
  const [bufferEndSeq, setBufferEndSeq] = createSignal(0);
  const [outputTruncated, setOutputTruncated] = createSignal(false);
  const [reconnectCount, setReconnectCount] = createSignal(0);
  const [isStreaming, setIsStreaming] = createSignal(false);
  const [errorMessage, setErrorMessage] = createSignal('');
  const [lastEventId, setLastEventId] = createSignal(0);
  const [activeRunId, setActiveRunId] = createSignal<string>('');

  let es: EventSource | undefined;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
  let reconnectAttempts = 0;
  // Whether the close was intentional (complete/error event or running=false)
  let intentionalClose = false;
  let wasRunning = false;

  function disposeSource(source: EventSource | undefined) {
    if (!source) return;
    source.onopen = null;
    source.onmessage = null;
    source.onerror = null;
    source.close();
  }

  function clearReconnectTimer() {
    if (reconnectTimer !== undefined) {
      clearTimeout(reconnectTimer);
      reconnectTimer = undefined;
    }
  }

  function close() {
    intentionalClose = true;
    clearReconnectTimer();
    if (es) {
      disposeSource(es);
      es = undefined;
    }
    setIsStreaming(false);
  }

  function scheduleReconnect() {
    if (intentionalClose || !opts.running()) {
      return;
    }
    if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      logger.warn(
        `[PatrolStream] Max reconnect attempts (${MAX_RECONNECT_ATTEMPTS}) reached, giving up`,
      );
      opts.onError?.();
      return;
    }
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempts),
      MAX_RECONNECT_DELAY_MS,
    );
    reconnectAttempts++;
    logger.info(
      `[PatrolStream] Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`,
    );
    clearReconnectTimer();
    reconnectTimer = setTimeout(() => {
      if (opts.running()) {
        open();
      }
    }, delay);
  }

  function open() {
    // Close existing connection without triggering intentionalClose
    clearReconnectTimer();
    if (es) {
      disposeSource(es);
      es = undefined;
    }
    setIsStreaming(false);
    intentionalClose = false;

    try {
      const last = activeRunId() ? lastEventId() : 0;
      const url =
        last > 0
          ? `/api/ai/patrol/stream?last_event_id=${encodeURIComponent(String(last))}`
          : '/api/ai/patrol/stream';
      const source = new EventSource(url);
      es = source;

      source.onopen = () => {
        if (es !== source) return;
        logger.info('[PatrolStream] SSE connected');
        setIsStreaming(true);
        if (reconnectAttempts > 0) {
          setReconnectCount((c) => c + 1);
        }
        reconnectAttempts = 0; // Reset on successful connection
        opts.onStart?.();
      };

      source.onmessage = (msg) => {
        if (es !== source) return;
        try {
          const rawEventData = msg.data;
          if (typeof rawEventData !== 'string') {
            logger.warn('[PatrolStream] Ignoring non-string SSE event payload');
            return;
          }
          if (rawEventData.length > MAX_SSE_EVENT_DATA_CHARS) {
            logger.warn('[PatrolStream] Dropping oversized SSE event payload');
            return;
          }

          const parsed = JSON.parse(rawEventData) as unknown;
          if (!parsed || typeof parsed !== 'object') {
            logger.warn('[PatrolStream] Ignoring malformed SSE event payload');
            return;
          }

          const event = parsed as PatrolStreamEvent;
          const normalizedTokens = normalizeTokenCount(event.tokens);
          const hasTokens = normalizedTokens !== undefined;
          const runId = normalizeRunId(event.run_id);
          const isNewRun = !!runId && runId !== activeRunId();
          if (isNewRun) {
            // New run; reset reconnect/resync indicators.
            setActiveRunId(runId);
            setResynced(false);
            setResyncReason('');
            setReconnectCount(0);
            setLastEventId(0);
            setBufferStartSeq(0);
            setBufferEndSeq(0);
            setOutputTruncated(false);
          }

          // Update lastEventId after any new-run reset so it never leaks across runs.
          const msgLastId = (msg as MessageEvent).lastEventId;
          if (
            typeof msgLastId === 'string' &&
            msgLastId.trim() !== '' &&
            msgLastId.length <= MAX_LAST_EVENT_ID_CHARS
          ) {
            const parsed = Number.parseInt(msgLastId, 10);
            if (Number.isSafeInteger(parsed) && parsed > 0) setLastEventId(parsed);
          } else {
            const seq = event.seq;
            if (typeof seq === 'number' && Number.isSafeInteger(seq) && seq > 0) {
              setLastEventId(seq);
            }
          }

          switch (event.type) {
            case 'snapshot':
              // Late-joiner state: initialize phase/tokens (UI does not render content here).
              if (event.phase) setPhase(clampStreamText(event.phase) || '');
              if (hasTokens) setTokens(normalizedTokens!);
              if (event.tool_name) setCurrentTool(clampStreamText(event.tool_name) || '');
              setErrorMessage('');
              const nextResyncReason = normalizeResyncReason(event.resync_reason);
              setResyncReason(nextResyncReason);
              const bufferStartSeq = event.buffer_start_seq;
              if (
                typeof bufferStartSeq === 'number' &&
                Number.isSafeInteger(bufferStartSeq) &&
                bufferStartSeq >= 0
              ) {
                setBufferStartSeq(bufferStartSeq);
              }
              const bufferEndSeq = event.buffer_end_seq;
              if (
                typeof bufferEndSeq === 'number' &&
                Number.isSafeInteger(bufferEndSeq) &&
                bufferEndSeq >= 0
              ) {
                setBufferEndSeq(bufferEndSeq);
              }
              setOutputTruncated(event.content_truncated === true);
              if (nextResyncReason && nextResyncReason !== 'late_joiner') setResynced(true);
              break;
            case 'start':
              setPhase('Starting patrol…');
              setCurrentTool('');
              setTokens(0);
              setErrorMessage('');
              setResynced(false);
              setResyncReason('');
              if (runId) setActiveRunId(runId);
              setBufferStartSeq(0);
              setBufferEndSeq(0);
              setOutputTruncated(false);
              break;
            case 'phase':
              if (event.phase) setPhase(clampStreamText(event.phase) || '');
              break;
            case 'content':
              // Content events carry incremental text; phase stays as-is
              break;
            case 'thinking':
              // Intentionally ignored for now (kept for future UI enhancements).
              break;
            case 'tool_start':
              if (event.tool_name) setCurrentTool(clampStreamText(event.tool_name) || '');
              break;
            case 'tool_end':
              setCurrentTool('');
              break;
            case 'complete':
              setPhase('');
              setCurrentTool('');
              setErrorMessage('');
              if (hasTokens) setTokens(normalizedTokens!);
              setActiveRunId('');
              setLastEventId(0);
              setResyncReason('');
              setBufferStartSeq(0);
              setBufferEndSeq(0);
              setOutputTruncated(false);
              close();
              opts.onComplete?.();
              break;
            case 'error':
              setPhase('');
              setCurrentTool('');
              setErrorMessage(clampStreamText(event.content) || 'Patrol encountered an error');
              setActiveRunId('');
              setLastEventId(0);
              setResyncReason('');
              setBufferStartSeq(0);
              setBufferEndSeq(0);
              setOutputTruncated(false);
              close();
              opts.onError?.();
              break;
            default:
              logger.warn('[PatrolStream] Ignoring unknown SSE event type');
              return;
          }

          if (hasTokens && event.type !== 'complete') {
            setTokens(normalizedTokens!);
          }
        } catch {
          logger.warn('[PatrolStream] Failed to parse SSE event');
        }
      };

      source.onerror = () => {
        if (es !== source) return;
        logger.warn('[PatrolStream] SSE connection error');
        disposeSource(source);
        es = undefined;
        setIsStreaming(false);
        // Only reconnect if the close wasn't intentional and patrol is still running
        if (!intentionalClose && opts.running()) {
          scheduleReconnect();
        } else if (!intentionalClose) {
          opts.onError?.();
        }
        // If intentionalClose, do nothing — we already handled it
      };
    } catch {
      // EventSource constructor can throw in some environments
      if (es) {
        disposeSource(es);
        es = undefined;
      }
      setIsStreaming(false);
      if (!intentionalClose && opts.running()) {
        scheduleReconnect();
      }
    }
  }

  // Reactively open/close based on running state
  createEffect(() => {
    const running = opts.running();
    // Avoid tearing down and recreating EventSource when dependencies change
    // but the effective running state remains true.
    if (running === wasRunning) {
      return;
    }
    wasRunning = running;

    if (running) {
      reconnectAttempts = 0;
      open();
    } else {
      close();
      // Reset signals when patrol stops
      setPhase('');
      setCurrentTool('');
      setTokens(0);
      setResynced(false);
      setResyncReason('');
      setReconnectCount(0);
      setLastEventId(0);
      setActiveRunId('');
      setBufferStartSeq(0);
      setBufferEndSeq(0);
      setOutputTruncated(false);
    }
  });

  onCleanup(() => close());

  return {
    phase,
    currentTool,
    tokens,
    resynced,
    resyncReason,
    bufferStartSeq,
    bufferEndSeq,
    outputTruncated,
    reconnectCount,
    isStreaming,
    errorMessage,
  };
}
