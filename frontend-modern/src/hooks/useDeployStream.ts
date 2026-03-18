/**
 * SolidJS hook for consuming live deploy/preflight SSE events.
 *
 * Opens an EventSource when `eventsUrl()` is non-null, closes when null or on
 * component cleanup. Deduplicates events by ID (backend replays on reconnect).
 * Reconnects with exponential backoff: 1s -> 2s -> 4s -> 8s -> 15s, max 5 attempts.
 * Detects `job_complete` event type as terminal signal.
 */

import { createSignal, createEffect, onCleanup, type Accessor } from 'solid-js';
import type { DeployEvent } from '@/types/agentDeploy';
import { logger } from '@/utils/logger';

export interface UseDeployStreamOptions {
  /** Reactive URL — null = closed, string = open SSE connection. */
  eventsUrl: Accessor<string | null>;
  /** Called for each incoming deploy event. */
  onEvent?: (event: DeployEvent) => void;
  /** Called when a `job_complete` event is received, with the final job status. */
  onComplete?: (status: string) => void;
  /** Called on connection failure after max reconnect attempts. */
  onError?: (message: string) => void;
}

export interface DeployStreamState {
  isStreaming: Accessor<boolean>;
  events: Accessor<DeployEvent[]>;
  lastError: Accessor<string>;
}

const MAX_RECONNECT_ATTEMPTS = 5;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 15000;
const MAX_SSE_EVENT_DATA_CHARS = 64 * 1024;

export function useDeployStream(opts: UseDeployStreamOptions): DeployStreamState {
  const [isStreaming, setIsStreaming] = createSignal(false);
  const [events, setEvents] = createSignal<DeployEvent[]>([]);
  const [lastError, setLastError] = createSignal('');

  // Track seen event IDs to deduplicate replays on reconnect.
  const seenIds = new Set<string>();

  let es: EventSource | undefined;
  let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
  let reconnectAttempts = 0;
  let intentionalClose = false;
  let previousUrl: string | null = null;

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

  function reset() {
    close();
    seenIds.clear();
    setEvents([]);
    setLastError('');
    reconnectAttempts = 0;
    intentionalClose = false;
  }

  function scheduleReconnect(url: string) {
    if (intentionalClose || !opts.eventsUrl()) return;
    if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      logger.warn(`[DeployStream] Max reconnect attempts (${MAX_RECONNECT_ATTEMPTS}) reached`);
      setLastError('Connection lost after max retries');
      opts.onError?.('Connection lost after max retries');
      return;
    }
    const delay = Math.min(
      INITIAL_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempts),
      MAX_RECONNECT_DELAY_MS,
    );
    reconnectAttempts++;
    logger.info(
      `[DeployStream] Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`,
    );
    clearReconnectTimer();
    reconnectTimer = setTimeout(() => {
      if (opts.eventsUrl()) {
        open(url);
      }
    }, delay);
  }

  function open(url: string) {
    clearReconnectTimer();
    if (es) {
      disposeSource(es);
      es = undefined;
    }
    setIsStreaming(false);
    intentionalClose = false;

    try {
      const source = new EventSource(url);
      es = source;

      source.onopen = () => {
        if (es !== source) return;
        logger.info('[DeployStream] SSE connected');
        setIsStreaming(true);
        reconnectAttempts = 0;
      };

      source.onmessage = (msg) => {
        if (es !== source) return;
        try {
          const rawData = msg.data;
          if (typeof rawData !== 'string') return;
          if (rawData.length > MAX_SSE_EVENT_DATA_CHARS) {
            logger.warn('[DeployStream] Dropping oversized SSE event');
            return;
          }

          const parsed = JSON.parse(rawData) as unknown;
          if (!parsed || typeof parsed !== 'object') return;

          const event = parsed as DeployEvent;

          // Detect terminal `job_complete` event (synthetic from SSE handler).
          if (event.type === 'job_complete') {
            const status = (parsed as { status?: string }).status as string;
            close();
            opts.onComplete?.(status || 'unknown');
            return;
          }

          // Deduplicate by event ID.
          if (event.id && seenIds.has(event.id)) return;
          if (event.id) seenIds.add(event.id);

          setEvents((prev) => [...prev, event]);
          opts.onEvent?.(event);
        } catch {
          logger.warn('[DeployStream] Failed to parse SSE event');
        }
      };

      source.onerror = () => {
        if (es !== source) return;
        logger.warn('[DeployStream] SSE connection error');
        disposeSource(source);
        es = undefined;
        setIsStreaming(false);
        if (!intentionalClose && opts.eventsUrl()) {
          scheduleReconnect(url);
        } else if (!intentionalClose) {
          opts.onError?.('SSE connection error');
        }
      };
    } catch {
      if (es) {
        disposeSource(es);
        es = undefined;
      }
      setIsStreaming(false);
      if (!intentionalClose && opts.eventsUrl()) {
        scheduleReconnect(url);
      }
    }
  }

  // Reactively open/close based on eventsUrl signal.
  createEffect(() => {
    const url = opts.eventsUrl();
    if (url === previousUrl) return;
    previousUrl = url;

    if (url) {
      // New URL — reset state and open fresh connection.
      seenIds.clear();
      setEvents([]);
      setLastError('');
      reconnectAttempts = 0;
      intentionalClose = false;
      open(url);
    } else {
      close();
    }
  });

  onCleanup(() => reset());

  return { isStreaming, events, lastError };
}
