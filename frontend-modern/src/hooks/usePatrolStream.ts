/**
 * SolidJS hook for consuming live patrol SSE events.
 *
 * Opens an EventSource to /api/ai/patrol/stream while the patrol is running,
 * exposing reactive signals for the current phase, active tool, and token count.
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

export function usePatrolStream(opts: UsePatrolStreamOptions): PatrolStreamState {
    const [phase, setPhase] = createSignal('');
    const [currentTool, setCurrentTool] = createSignal('');
    const [tokens, setTokens] = createSignal(0);
    const [isStreaming, setIsStreaming] = createSignal(false);
    const [errorMessage, setErrorMessage] = createSignal('');

    let es: EventSource | undefined;

    function close() {
        if (es) {
            es.close();
            es = undefined;
        }
        setIsStreaming(false);
    }

    function open() {
        close();
        try {
            es = new EventSource('/api/ai/patrol/stream');

            es.onopen = () => {
                logger.info('[PatrolStream] SSE connected');
                setIsStreaming(true);
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
                    // Ignore malformed events
                }
            };

            es.onerror = () => {
                // Silently close on error — polling continues as fallback
                logger.warn('[PatrolStream] SSE error, closing');
                close();
                opts.onError?.();
            };
        } catch {
            // EventSource constructor can throw in some environments
            close();
        }
    }

    // Reactively open/close based on running state
    createEffect(() => {
        if (opts.running()) {
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
