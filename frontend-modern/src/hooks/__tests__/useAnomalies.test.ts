import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

type UseAnomaliesModule = typeof import('@/hooks/useAnomalies');

const flushAsync = async () => {
  await Promise.resolve();
  await Promise.resolve();
};

describe('useAnomalies', () => {
  let getAnomaliesMock: ReturnType<typeof vi.fn>;
  let useAllAnomalies: UseAnomaliesModule['useAllAnomalies'];
  let useAnomalyForMetric: UseAnomaliesModule['useAnomalyForMetric'];

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
      useAnomalyForMetric,
    } = await import('@/hooks/useAnomalies'));
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
    vi.resetModules();
  });

  it('shares a single polling interval across consumers and stops on last dispose', async () => {
    let disposeFirst = () => {};
    createRoot((dispose) => {
      disposeFirst = dispose;
      useAllAnomalies();
    });

    let disposeSecond = () => {};
    createRoot((dispose) => {
      disposeSecond = dispose;
      useAnomalyForMetric(() => 'resource-1', () => 'cpu');
    });

    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(30_000);
    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(2);

    disposeFirst();

    vi.advanceTimersByTime(30_000);
    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);

    disposeSecond();

    vi.advanceTimersByTime(60_000);
    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);
  });

  it('restarts polling when a new consumer mounts after full cleanup', async () => {
    let dispose = () => {};
    createRoot((rootDispose) => {
      dispose = rootDispose;
      useAllAnomalies();
    });

    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    dispose();

    vi.advanceTimersByTime(30_000);
    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(1);

    createRoot((rootDispose) => {
      dispose = rootDispose;
      useAllAnomalies();
    });

    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(30_000);
    await flushAsync();
    expect(getAnomaliesMock).toHaveBeenCalledTimes(3);

    dispose();
  });
});
