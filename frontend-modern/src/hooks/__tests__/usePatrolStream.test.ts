import { createRoot, createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { loggerMock } = vi.hoisted(() => ({
    loggerMock: {
        info: vi.fn(),
        warn: vi.fn(),
    },
}));
vi.mock('@/utils/logger', () => ({ logger: loggerMock }));

import { usePatrolStream } from '@/hooks/usePatrolStream';

class MockEventSource {
    static instances: MockEventSource[] = [];

    readonly url: string;
    onopen: ((event: Event) => void) | null = null;
    onmessage: ((event: MessageEvent) => void) | null = null;
    onerror: ((event: Event) => void) | null = null;
    readyState = 0;
    withCredentials = false;
    closed = false;

    constructor(url: string) {
        this.url = url;
        MockEventSource.instances.push(this);
    }

    close() {
        this.closed = true;
        this.readyState = 2;
    }

    emitOpen() {
        this.readyState = 1;
        this.onopen?.(new Event('open'));
    }

    emitMessage(payload: unknown, lastEventId = '') {
        const evt = { data: JSON.stringify(payload), lastEventId } as MessageEvent;
        this.onmessage?.(evt);
    }

    emitRawMessage(data: unknown, lastEventId = '') {
        const evt = { data, lastEventId } as MessageEvent;
        this.onmessage?.(evt);
    }

    emitError() {
        this.onerror?.(new Event('error'));
    }
}

describe('usePatrolStream', () => {
    const originalEventSource = globalThis.EventSource;

    beforeEach(() => {
        vi.useFakeTimers();
        vi.clearAllMocks();
        MockEventSource.instances = [];
        (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
            MockEventSource as unknown as typeof EventSource;
    });

    afterEach(() => {
        vi.useRealTimers();
        (globalThis as unknown as { EventSource: typeof EventSource }).EventSource = originalEventSource;
    });

    it('opens stream and reconnects with last_event_id for active run', () => {
        let dispose!: () => void;
        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            usePatrolStream({ running });
        });

        expect(MockEventSource.instances).toHaveLength(1);
        expect(MockEventSource.instances[0].url).toBe('/api/ai/patrol/stream');

        const first = MockEventSource.instances[0];
        first.emitOpen();
        first.emitMessage({ type: 'start', run_id: 'run-1', seq: 1 }, '1');
        first.emitError();

        vi.advanceTimersByTime(1000);
        expect(MockEventSource.instances.length).toBeGreaterThanOrEqual(2);
        expect(
            MockEventSource.instances.some((es) =>
                es.url.includes('/api/ai/patrol/stream?last_event_id=1'),
            ),
        ).toBe(true);

        dispose();
    });

    it('does not include last_event_id when run_id is unknown', () => {
        let dispose!: () => void;
        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            usePatrolStream({ running });
        });

        const first = MockEventSource.instances[0];
        first.emitOpen();
        // seq received, but no run_id has ever been established.
        first.emitMessage({ type: 'content', seq: 7 }, '7');
        first.emitError();

        vi.advanceTimersByTime(1000);
        expect(MockEventSource.instances.length).toBeGreaterThanOrEqual(2);
        expect(
            MockEventSource.instances.some((es) =>
                es.url.includes('/api/ai/patrol/stream?last_event_id='),
            ),
        ).toBe(false);

        dispose();
    });

    it('maps snapshot resync metadata into state', () => {
        let dispose!: () => void;
        let state!: ReturnType<typeof usePatrolStream>;

        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            state = usePatrolStream({ running });
        });

        const stream = MockEventSource.instances[0];
        stream.emitOpen();
        stream.emitMessage({
            type: 'snapshot',
            run_id: 'run-2',
            resync_reason: 'buffer_rotated',
            buffer_start_seq: 120,
            buffer_end_seq: 320,
            content_truncated: true,
        });

        expect(state.resynced()).toBe(true);
        expect(state.resyncReason()).toBe('buffer_rotated');
        expect(state.bufferStartSeq()).toBe(120);
        expect(state.bufferEndSeq()).toBe(320);
        expect(state.outputTruncated()).toBe(true);

        dispose();
    });

    it('drops oversized SSE message payloads', () => {
        let dispose!: () => void;
        let state!: ReturnType<typeof usePatrolStream>;

        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            state = usePatrolStream({ running });
        });

        const stream = MockEventSource.instances[0];
        stream.emitOpen();
        stream.emitMessage({ type: 'phase', phase: 'initial-phase' });
        expect(state.phase()).toBe('initial-phase');

        stream.emitMessage({ type: 'phase', phase: 'x'.repeat(70 * 1024) });

        expect(state.phase()).toBe('initial-phase');
        expect(loggerMock.warn).toHaveBeenCalledWith('[PatrolStream] Dropping oversized SSE event payload');

        dispose();
    });

    it('clamps stream text fields and token counters from SSE input', () => {
        let dispose!: () => void;
        let state!: ReturnType<typeof usePatrolStream>;

        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            state = usePatrolStream({ running });
        });

        const stream = MockEventSource.instances[0];
        stream.emitOpen();
        stream.emitMessage({
            type: 'snapshot',
            phase: 'p'.repeat(5000),
            tokens: 999999999,
            tool_name: 't'.repeat(5000),
        });

        expect(state.phase().length).toBe(4096);
        expect(state.currentTool().length).toBe(4096);
        expect(state.tokens()).toBe(10_000_000);

        stream.emitMessage({ type: 'error', content: 'e'.repeat(5000) });
        expect(state.errorMessage().length).toBe(4096);

        dispose();
    });

    it('ignores unknown stream event types', () => {
        let dispose!: () => void;
        let state!: ReturnType<typeof usePatrolStream>;

        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            state = usePatrolStream({ running });
        });

        const stream = MockEventSource.instances[0];
        stream.emitOpen();
        stream.emitMessage({ type: 'start', run_id: 'run-1', tokens: 2 });
        expect(state.tokens()).toBe(2);

        stream.emitRawMessage(JSON.stringify({ type: 'unexpected_type', tokens: 999 }));

        expect(state.tokens()).toBe(2);
        expect(loggerMock.warn).toHaveBeenCalledWith('[PatrolStream] Ignoring unknown SSE event type');

        dispose();
    });

    it('stops reconnecting after max attempts and calls onError', () => {
        const onError = vi.fn();
        let dispose!: () => void;

        createRoot((d) => {
            dispose = d;
            const [running] = createSignal(true);
            usePatrolStream({ running, onError });
        });

        const backoffMs = [1000, 2000, 4000, 8000, 15000];
        for (const delay of backoffMs) {
            const current = MockEventSource.instances[MockEventSource.instances.length - 1];
            current.emitError();
            vi.advanceTimersByTime(delay);
        }

        expect(MockEventSource.instances).toHaveLength(6); // initial + 5 reconnect attempts

        const last = MockEventSource.instances[MockEventSource.instances.length - 1];
        last.emitError(); // exceeds max attempts
        vi.advanceTimersByTime(60000);

        expect(onError).toHaveBeenCalledTimes(1);
        expect(MockEventSource.instances).toHaveLength(6);

        dispose();
    });

    it('does not reopen stream when dependencies change but running remains true', () => {
        let dispose!: () => void;
        let setChurn!: (value: number) => void;

        createRoot((d) => {
            dispose = d;
            const [baseRunning] = createSignal(true);
            const [churn, _setChurn] = createSignal(0);
            setChurn = _setChurn;

            usePatrolStream({
                // churn() changes trigger effect reruns, but this expression stays true.
                running: () => baseRunning() && churn() >= 0,
            });
        });

        expect(MockEventSource.instances).toHaveLength(1);
        setChurn(1);
        setChurn(2);
        setChurn(3);
        expect(MockEventSource.instances).toHaveLength(1);

        dispose();
    });

    it('closes stream immediately when running switches off mid-run', () => {
        let dispose!: () => void;
        let setEnabled!: (value: boolean) => void;
        let setBackendRunning!: (value: boolean) => void;

        createRoot((d) => {
            dispose = d;
            const [enabled, _setEnabled] = createSignal(true);
            const [backendRunning, _setBackendRunning] = createSignal(true);
            setEnabled = _setEnabled;
            setBackendRunning = _setBackendRunning;

            usePatrolStream({
                running: () => enabled() && backendRunning(),
            });
        });

        expect(MockEventSource.instances).toHaveLength(1);
        const first = MockEventSource.instances[0];
        first.emitOpen();
        expect(first.closed).toBe(false);

        // User disables patrol while backend still reports running.
        setEnabled(false);
        expect(first.closed).toBe(true);
        expect(MockEventSource.instances).toHaveLength(1);

        // Backend state churn should not reopen until enabled again.
        setBackendRunning(false);
        setBackendRunning(true);
        expect(MockEventSource.instances).toHaveLength(1);

        dispose();
    });
});
