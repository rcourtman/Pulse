import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { getAnomaliesMock } = vi.hoisted(() => ({
  getAnomaliesMock: vi.fn(),
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getAnomalies: getAnomaliesMock,
  },
}));

import { useAllAnomalies } from '@/hooks/useAnomalies';

const flushMicrotasks = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useAnomalies', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.clearAllMocks();
    getAnomaliesMock.mockResolvedValue({ anomalies: [] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('keeps refresh interval alive until the last consumer unmounts', async () => {
    let disposeA!: () => void;
    let disposeB!: () => void;

    createRoot((dispose) => {
      disposeA = dispose;
      useAllAnomalies();
    });
    createRoot((dispose) => {
      disposeB = dispose;
      useAllAnomalies();
    });

    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(30_000);
    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(2);

    disposeA();
    vi.advanceTimersByTime(30_000);
    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);

    disposeB();
    vi.advanceTimersByTime(30_000);
    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);
  });

  it('restarts polling when a new consumer mounts after full teardown', async () => {
    let dispose!: () => void;

    createRoot((rootDispose) => {
      dispose = rootDispose;
      useAllAnomalies();
    });

    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    dispose();
    vi.advanceTimersByTime(30_000);
    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    createRoot((rootDispose) => {
      dispose = rootDispose;
      useAllAnomalies();
    });

    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(2);

    dispose();
  });
});
