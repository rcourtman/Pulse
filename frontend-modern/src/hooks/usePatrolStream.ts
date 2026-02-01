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
    /** True while the EventSource is connected. */
    isStreaming: Accessor<boolean>;
    /** Error message from the last error event, or empty. */
    errorMessage: Accessor<string>;
}

const MAX_RECONNECT_ATTEMPTS = 5;
const INITIAL_RECONNECT_DELAY_MS = 1000;
const MAX_RECONNECT_DELAY_MS = 15000;

export function usePatrolStream(opts: UsePatrolStreamOptions): PatrolStreamState {
    const [phase, setPhase] = createSignal('');
    const [currentTool, setCurrentTool] = createSignal('');
    const [tokens, setTokens] = createSignal(0);
    const [isStreaming, setIsStreaming] = createSignal(false);
    const [errorMessage, setErrorMessage] = createSignal('');

    let es: EventSource | undefined;
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
    let reconnectAttempts = 0;
    // Whether the close was intentional (complete/error event or running=false)
    let intentionalClose = false;

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
            es.close();
            es = undefined;
        }
        setIsStreaming(false);
    }

    function scheduleReconnect() {
        if (intentionalClose || !opts.running()) {
            return;
        }
        if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
            logger.warn(`[PatrolStream] Max reconnect attempts (${MAX_RECONNECT_ATTEMPTS}) reached, giving up`);
            opts.onError?.();
            return;
        }
        const delay = Math.min(
            INITIAL_RECONNECT_DELAY_MS * Math.pow(2, reconnectAttempts),
            MAX_RECONNECT_DELAY_MS,
        );
        reconnectAttempts++;
        logger.info(`[PatrolStream] Reconnecting in ${delay}ms (attempt ${reconnectAttempts}/${MAX_RECONNECT_ATTEMPTS})`);
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
            es.close();
            es = undefined;
        }
        setIsStreaming(false);
        intentionalClose = false;

        try {
            es = new EventSource('/api/ai/patrol/stream');

            es.onopen = () => {
                logger.info('[PatrolStream] SSE connected');
                setIsStreaming(true);
                reconnectAttempts = 0; // Reset on successful connection
            };

            es.onmessage = (msg) => {
                try {
                    const event: PatrolStreamEvent = JSON.parse(msg.data);

                    switch (event.type) {
                        case 'start':
                            setPhase('Starting patrol…');
                            setCurrentTool('');
                            setTokens(0);
                            setErrorMessage('');
                            break;
                        case 'phase':
                            if (event.phase) setPhase(event.phase);
                            break;
                        case 'content':
                            // Content events carry incremental text; phase stays as-is
                            break;
                        case 'tool_start':
                            if (event.tool_name) setCurrentTool(event.tool_name);
                            break;
                        case 'tool_end':
                            setCurrentTool('');
                            break;
                        case 'complete':
                            setPhase('');
                            setCurrentTool('');
                            setErrorMessage('');
                            if (event.tokens) setTokens(event.tokens);
                            close();
                            opts.onComplete?.();
                            break;
                        case 'error':
                            setPhase('');
                            setCurrentTool('');
                            setErrorMessage(event.content || 'Patrol encountered an error');
                            close();
                            opts.onError?.();
                            break;
                    }

                    if (event.tokens && event.type !== 'complete') {
                        setTokens(event.tokens);
                    }
                } catch {
                    logger.warn('[PatrolStream] Failed to parse SSE event');
                }
            };

            es.onerror = () => {
                logger.warn('[PatrolStream] SSE connection error');
                if (es) {
                    es.close();
                    es = undefined;
                }
                setIsStreaming(false);
                // Only reconnect if the close wasn't intentional and patrol is still running
                if (!intentionalClose && opts.running()) {
                    scheduleReconnect();
                } else {
                    opts.onError?.();
                }
            };
        } catch {
            // EventSource constructor can throw in some environments
            if (es) {
                es.close();
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
        if (opts.running()) {
            reconnectAttempts = 0;
            open();
        } else {
            close();
            // Reset signals when patrol stops
            setPhase('');
            setCurrentTool('');
            setTokens(0);
        }
    });

    onCleanup(() => close());

    return { phase, currentTool, tokens, isStreaming, errorMessage };
}
