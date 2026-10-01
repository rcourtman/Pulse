import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { UpdateStatus } from '@/api/updates';
import {
  MAX_SAME_VERSION_HEALTHY_ATTEMPTS,
  UPDATE_PROGRESS_STALL_TIMEOUT_MS,
  UPDATE_STATUS_POLL_INTERVAL_MS,
  UPDATE_STREAM_SILENCE_FALLBACK_MS,
} from '@/components/updateReadinessModel';

const getUpdateStatusMock = vi.hoisted(() => vi.fn());
const apiFetchMock = vi.hoisted(() => vi.fn());

vi.mock('@/api/updates', () => ({
  UpdatesAPI: { getUpdateStatus: getUpdateStatusMock },
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

vi.mock('@/stores/updates', async () => {
  const { createSignal } = await import('solid-js');
  const [versionInfo, setVersionInfo] = createSignal<{ version: string } | null>({
    version: '6.4.5-rc.5',
  });
  return { updateStore: { versionInfo }, __setStoreVersionInfo: setVersionInfo };
});

vi.mock('@/utils/logger', () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

import { UpdateProgressModal } from '@/components/UpdateProgressModal';
import * as updatesStoreModule from '@/stores/updates';

const setStoreVersionInfo = (
  updatesStoreModule as unknown as {
    __setStoreVersionInfo: (value: { version: string } | null) => void;
  }
).__setStoreVersionInfo;

class MockEventSource {
  static instances: MockEventSource[] = [];

  readonly url: string;
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readyState = 0;
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

  emitError() {
    this.onerror?.(new Event('error'));
  }

  emitStatus(status: Partial<UpdateStatus>) {
    this.onmessage?.({
      data: JSON.stringify({ message: '', updatedAt: '', ...status }),
    } as MessageEvent);
  }
}

const statusOf = (status: string, progress: number, message = ''): UpdateStatus => ({
  status,
  progress,
  message,
  updatedAt: new Date().toISOString(),
});

const versionResponse = (version: string) => ({
  ok: true,
  json: async () => ({ version }),
});

const renderModal = () =>
  render(() => (
    <UpdateProgressModal
      isOpen={true}
      onClose={() => undefined}
      onViewHistory={() => undefined}
      connected={() => true}
      reconnecting={() => false}
    />
  ));

describe('UpdateProgressModal progress feed', () => {
  const originalEventSource = globalThis.EventSource;
  const originalLocation = window.location;
  let reloadMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    MockEventSource.instances = [];
    (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
      MockEventSource as unknown as typeof EventSource;
    reloadMock = vi.fn();
    // jsdom marks location.reload as non-configurable, so replace the whole object
    Object.defineProperty(window, 'location', {
      value: { ...originalLocation, reload: reloadMock },
      writable: true,
      configurable: true,
    });
    getUpdateStatusMock.mockReset();
    apiFetchMock.mockReset();
    setStoreVersionInfo({ version: '6.4.5-rc.5' });
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    (globalThis as unknown as { EventSource: typeof EventSource }).EventSource =
      originalEventSource;
    Object.defineProperty(window, 'location', {
      value: originalLocation,
      writable: true,
      configurable: true,
    });
  });

  it('polls when the stream opens and then goes silent, and reloads once the new version serves', async () => {
    // The stream delivers the first stage and then nothing, without erroring:
    // the "stuck at Downloading 10%" report.
    const polled = [
      statusOf('applying', 80, 'Applying update...'),
      statusOf('restarting', 95, 'Restarting service...'),
    ];
    getUpdateStatusMock.mockImplementation(async () => polled.shift() ?? polled[0]);
    apiFetchMock.mockResolvedValue(versionResponse('6.4.5'));

    renderModal();
    const stream = MockEventSource.instances[0];
    expect(stream.url).toBe('/api/updates/stream');
    stream.emitOpen();
    stream.emitStatus({ status: 'downloading', progress: 10, message: 'Downloading update...' });

    expect(await screen.findByText('Downloading update...')).toBeInTheDocument();
    expect(screen.getByText('10%')).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(UPDATE_STREAM_SILENCE_FALLBACK_MS - 1);
    expect(getUpdateStatusMock).not.toHaveBeenCalled();

    // The EventSource is still open and never errored; silence alone starts polling.
    await vi.advanceTimersByTimeAsync(1);
    expect(stream.closed).toBe(false);
    expect(getUpdateStatusMock).toHaveBeenCalledTimes(1);
    expect(await screen.findByText('Applying update...')).toBeInTheDocument();
    expect(screen.getByText('80%')).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
    expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();
    expect(stream.closed).toBe(true);

    // The first health check is scheduled immediately (fake timers treat a
    // zero delay scheduled mid-tick as 1ms).
    await vi.advanceTimersByTimeAsync(1);
    expect(apiFetchMock).toHaveBeenCalledWith('/api/version', { cache: 'no-store' });
    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  it('keeps probing through the restart and reloads only when a different version is served', async () => {
    const versions = [
      Promise.resolve(versionResponse('6.4.5-rc.5')),
      Promise.reject(new Error('connection refused')),
      Promise.resolve(versionResponse('6.4.5')),
    ];
    // Avoid unhandled-rejection noise for the pre-built rejected promise.
    versions[1].catch(() => undefined);
    apiFetchMock.mockImplementation(() => versions.shift() ?? versions[versions.length - 1]);

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({
      status: 'completed',
      progress: 100,
      message: 'Update completed, restarting...',
    });

    // First probe still answers with the old process's version: wait.
    await vi.advanceTimersByTimeAsync(0);
    expect(reloadMock).not.toHaveBeenCalled();
    expect(await screen.findByText('Pulse is restarting...')).toBeInTheDocument();

    // Health check during the restart window fails, then the new version answers.
    await vi.advanceTimersByTimeAsync(0);
    expect(reloadMock).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(2000);
    expect(reloadMock).toHaveBeenCalledTimes(1);
    expect(getUpdateStatusMock).not.toHaveBeenCalled();
  });

  it('offers a reload instead of an endless spinner when progress stops moving', async () => {
    getUpdateStatusMock.mockResolvedValue(statusOf('downloading', 10, 'Downloading update...'));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'downloading', progress: 10, message: 'Downloading update...' });
    expect(
      screen.getByText('Please do not close this window or refresh the page during the update.'),
    ).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(UPDATE_PROGRESS_STALL_TIMEOUT_MS - 1);
    expect(screen.queryByText('No progress reported for a while')).not.toBeInTheDocument();
    // Polling kept echoing the same stage; that is not progress.
    expect(getUpdateStatusMock.mock.calls.length).toBeGreaterThan(1);

    await vi.advanceTimersByTimeAsync(1);
    expect(screen.getByText('No progress reported for a while')).toBeInTheDocument();
    expect(
      screen.queryByText('Please do not close this window or refresh the page during the update.'),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Reload to check' }));
    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  it('ignores stale polled stages that would move progress backwards', async () => {
    getUpdateStatusMock.mockResolvedValue(statusOf('downloading', 10, 'Downloading update...'));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'extracting', progress: 40, message: 'Extracting update...' });

    await vi.advanceTimersByTimeAsync(UPDATE_STREAM_SILENCE_FALLBACK_MS);
    expect(getUpdateStatusMock).toHaveBeenCalled();
    expect(screen.getByText('Extracting update...')).toBeInTheDocument();
    expect(screen.queryByText('Downloading update...')).not.toBeInTheDocument();
  });
  it('rides out a transient poll failure during a download without reloading', async () => {
    const polled: Array<UpdateStatus | Error> = [
      new Error('Request failed with status 500'),
      statusOf('downloading', 10, 'Downloading update...'),
      statusOf('extracting', 40, 'Extracting update...'),
    ];
    getUpdateStatusMock.mockImplementation(async () => {
      const next = polled.shift() ?? statusOf('extracting', 40, 'Extracting update...');
      if (next instanceof Error) {
        throw next;
      }
      return next;
    });
    apiFetchMock.mockResolvedValue(versionResponse('6.4.5-rc.5'));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'downloading', progress: 10, message: 'Downloading update...' });

    // Silence starts polling; the first poll fails with a 500.
    await vi.advanceTimersByTimeAsync(UPDATE_STREAM_SILENCE_FALLBACK_MS);
    expect(getUpdateStatusMock).toHaveBeenCalledTimes(1);
    expect(stream.closed).toBe(false);
    expect(screen.queryByText('Pulse is restarting...')).not.toBeInTheDocument();
    expect(screen.getByText('Downloading update...')).toBeInTheDocument();

    // Polling carries on and progress keeps moving.
    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS * 2);
    expect(screen.getByText('Extracting update...')).toBeInTheDocument();
    expect(screen.getByText('40%')).toBeInTheDocument();

    // The stream is still live and later stages from it are still honoured.
    stream.emitStatus({ status: 'backing-up', progress: 60, message: 'Creating backup...' });
    expect(screen.getByText('Creating backup...')).toBeInTheDocument();
    expect(stream.closed).toBe(false);
    expect(apiFetchMock).not.toHaveBeenCalled();
    expect(reloadMock).not.toHaveBeenCalled();
  });

  it('keeps polling through repeated failures early in the update, even with the stream closed', async () => {
    let failing = true;
    getUpdateStatusMock.mockImplementation(async () => {
      if (failing) {
        throw new Error('network error');
      }
      return statusOf('verifying', 25, 'Verifying signature...');
    });
    apiFetchMock.mockResolvedValue(versionResponse('6.4.5-rc.5'));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'downloading', progress: 10, message: 'Downloading update...' });
    stream.emitError();
    expect(stream.closed).toBe(true);

    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS * 6);
    expect(getUpdateStatusMock.mock.calls.length).toBeGreaterThanOrEqual(6);
    expect(screen.queryByText('Pulse is restarting...')).not.toBeInTheDocument();
    expect(apiFetchMock).not.toHaveBeenCalled();

    failing = false;
    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
    expect(screen.getByText('Verifying signature...')).toBeInTheDocument();
    expect(reloadMock).not.toHaveBeenCalled();
  });

  it('never reloads on an unchanged version when the restart was only inferred', async () => {
    getUpdateStatusMock.mockRejectedValue(new Error('connection refused'));
    let serving = '6.4.5-rc.5';
    apiFetchMock.mockImplementation(async () => versionResponse(serving));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'applying', progress: 80, message: 'Applying update...' });
    stream.emitError();

    // Two failed polls late in the update, with the stream gone: probe for
    // the restarted backend, but the restart is not confirmed.
    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
    expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();

    // Far more same-version answers than the confirmed-restart fallback allows.
    await vi.advanceTimersByTimeAsync(15000 * (MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 4));
    expect(apiFetchMock.mock.calls.length).toBeGreaterThan(MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 1);
    expect(reloadMock).not.toHaveBeenCalled();
    // The user gets the manual check instead.
    expect(screen.getByText('No progress reported for a while')).toBeInTheDocument();

    // Real evidence (the new version serving) still reloads.
    serving = '6.4.5';
    await vi.advanceTimersByTimeAsync(15000);
    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  it('lets a late failure correct an inferred restart instead of reloading', async () => {
    let pollFails = true;
    getUpdateStatusMock.mockImplementation(async () => {
      if (pollFails) {
        throw new Error('connection refused');
      }
      return { ...statusOf('error', 80, 'Failed to apply update'), error: 'disk full' };
    });
    apiFetchMock.mockResolvedValue(versionResponse('6.4.5-rc.5'));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'applying', progress: 80, message: 'Applying update...' });
    stream.emitError();

    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
    expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();

    pollFails = false;
    await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
    expect(screen.getByText('Update Failed')).toBeInTheDocument();
    expect(screen.getByText('disk full')).toBeInTheDocument();

    await vi.advanceTimersByTimeAsync(60000);
    expect(reloadMock).not.toHaveBeenCalled();
  });

  it('reloads after a confirmed restart only once the new version serves', async () => {
    getUpdateStatusMock.mockResolvedValue(statusOf('downloading', 10, 'Downloading update...'));
    let serving = '6.4.5-rc.5';
    apiFetchMock.mockImplementation(async () => versionResponse(serving));

    renderModal();
    const stream = MockEventSource.instances[0];
    stream.emitOpen();
    stream.emitStatus({ status: 'restarting', progress: 95, message: 'Restarting service...' });
    expect(stream.closed).toBe(true);

    await vi.advanceTimersByTimeAsync(2000);
    expect(apiFetchMock).toHaveBeenCalled();
    expect(reloadMock).not.toHaveBeenCalled();
    // A stale status from a poll cannot drag a confirmed restart back.
    expect(getUpdateStatusMock).not.toHaveBeenCalled();

    serving = '6.4.5';
    await vi.advanceTimersByTimeAsync(15000);
    expect(reloadMock).toHaveBeenCalledTimes(1);
  });

  describe('when the modal opens before the running version is known', () => {
    beforeEach(() => {
      setStoreVersionInfo(null);
    });

    it('fetches the pre-update version on open and uses it for restart detection', async () => {
      let serving = '6.4.5-rc.5';
      apiFetchMock.mockImplementation(async () => versionResponse(serving));

      renderModal();
      expect(apiFetchMock).toHaveBeenCalledWith('/api/version', { cache: 'no-store' });
      await vi.advanceTimersByTimeAsync(0);

      const stream = MockEventSource.instances[0];
      stream.emitOpen();
      stream.emitStatus({ status: 'completed', progress: 100, message: 'Update completed' });

      // The old process still answers with the captured version: wait.
      await vi.advanceTimersByTimeAsync(4000);
      expect(reloadMock).not.toHaveBeenCalled();

      serving = '6.4.5';
      await vi.advanceTimersByTimeAsync(15000);
      expect(reloadMock).toHaveBeenCalledTimes(1);
    });

    it('takes the baseline from the store once it loads', async () => {
      getUpdateStatusMock.mockRejectedValue(new Error('connection refused'));
      let serving = '6.4.5-rc.5';
      let baselineCall = true;
      apiFetchMock.mockImplementation(async () => {
        if (baselineCall) {
          baselineCall = false;
          throw new Error('network blip');
        }
        return versionResponse(serving);
      });

      renderModal();
      const stream = MockEventSource.instances[0];
      stream.emitOpen();
      stream.emitStatus({ status: 'downloading', progress: 10, message: 'Downloading update...' });
      await vi.advanceTimersByTimeAsync(0);

      setStoreVersionInfo({ version: '6.4.5-rc.5' });
      stream.emitStatus({ status: 'applying', progress: 80, message: 'Applying update...' });
      stream.emitError();

      // Suspected restart; old-version answers must not reload.
      await vi.advanceTimersByTimeAsync(60000);
      expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();
      expect(reloadMock).not.toHaveBeenCalled();

      serving = '6.4.5';
      await vi.advanceTimersByTimeAsync(15000);
      expect(reloadMock).toHaveBeenCalledTimes(1);
    });

    it('never reloads without a baseline while completion is unconfirmed', async () => {
      getUpdateStatusMock.mockRejectedValue(new Error('connection refused'));
      let baselineCall = true;
      apiFetchMock.mockImplementation(async () => {
        if (baselineCall) {
          baselineCall = false;
          throw new Error('network blip');
        }
        // The old process, still healthy.
        return versionResponse('6.4.5-rc.5');
      });

      renderModal();
      const stream = MockEventSource.instances[0];
      stream.emitOpen();
      stream.emitStatus({ status: 'applying', progress: 80, message: 'Applying update...' });
      stream.emitError();

      await vi.advanceTimersByTimeAsync(UPDATE_STATUS_POLL_INTERVAL_MS);
      expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();

      await vi.advanceTimersByTimeAsync(15000 * (MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 4));
      expect(apiFetchMock.mock.calls.length).toBeGreaterThan(MAX_SAME_VERSION_HEALTHY_ATTEMPTS + 1);
      expect(reloadMock).not.toHaveBeenCalled();
      expect(screen.getByText('No progress reported for a while')).toBeInTheDocument();
    });

    it('after confirmed completion without a baseline, reloads only once the restart was seen', async () => {
      const answers: Array<'blip' | 'down' | string> = [
        'blip', // baseline fetch on open fails
        '6.4.5-rc.5', // old process still answering after 'completed'
        '6.4.5-rc.5',
        'down', // old process exits
        '6.4.5', // new process
      ];
      apiFetchMock.mockImplementation(async () => {
        const next = answers.length > 1 ? answers.shift()! : answers[0];
        if (next === 'blip' || next === 'down') {
          throw new Error(next);
        }
        return versionResponse(next);
      });

      renderModal();
      const stream = MockEventSource.instances[0];
      stream.emitOpen();
      await vi.advanceTimersByTimeAsync(0);
      stream.emitStatus({ status: 'completed', progress: 100, message: 'Update completed' });

      // Two healthy answers from the old process right after 'completed', then
      // the probe at 2s finds it gone.
      await vi.advanceTimersByTimeAsync(2000);
      expect(apiFetchMock).toHaveBeenCalledTimes(4);
      expect(reloadMock).not.toHaveBeenCalled();
      expect(screen.getByText('Pulse is restarting...')).toBeInTheDocument();

      // The next healthy answer after the restart was seen reloads.
      await vi.advanceTimersByTimeAsync(4000);
      expect(apiFetchMock).toHaveBeenCalledTimes(5);
      expect(reloadMock).toHaveBeenCalledTimes(1);
    });
  });
});
