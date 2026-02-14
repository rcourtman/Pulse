import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseAnomaliesModule = typeof import('@/hooks/useAnomalies');

const { getAnomaliesMock } = vi.hoisted(() => ({
  getAnomaliesMock: vi.fn(),
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getAnomalies: getAnomaliesMock,
  },
}));

const flushMicrotasks = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useAnomalies', () => {
  let getAnomaliesMock: ReturnType<typeof vi.fn>;
  let useAllAnomalies: UseAnomaliesModule['useAllAnomalies'];

  beforeEach(async () => {
    vi.useFakeTimers();
    vi.resetModules();

    getAnomaliesMock = vi.fn().mockResolvedValue({ anomalies: [] });

    vi.doMock('@/api/ai', () => ({
      AIAPI: {
        getAnomalies: getAnomaliesMock,
      },
    }));

    ({
      useAllAnomalies,
    } = await import('@/hooks/useAnomalies'));
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

    vi.advanceTimersByTime(30_000);
    await flushMicrotasks();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);

    dispose();
  });
});
