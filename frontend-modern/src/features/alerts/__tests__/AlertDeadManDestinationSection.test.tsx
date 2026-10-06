import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';

import {
  AlertDeadManDestinationSection,
  DEAD_MAN_STATUS_POLL_MS,
} from '../AlertDeadManDestinationSection';

vi.mock('@/api/alerts', () => ({
  AlertsAPI: {
    getDeadManStatus: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: { error: vi.fn() },
}));

describe('AlertDeadManDestinationSection', () => {
  beforeEach(() => {
    vi.mocked(AlertsAPI.getDeadManStatus).mockReset();
    vi.mocked(AlertsAPI.getDeadManStatus).mockResolvedValue({
      configured: true,
      state: 'healthy',
      heartbeatIntervalSeconds: 60,
      recommendedGraceSeconds: 180,
      lastMonitoringProgress: '2026-08-27T11:59:55Z',
      lastSuccessAt: '2026-08-27T12:00:00Z',
      consecutiveFailures: 0,
      lastInterruption: {
        from: '2026-08-27T11:55:00Z',
        to: '2026-08-27T12:00:00Z',
        durationSeconds: 300,
        cleanShutdown: false,
      },
    });
  });

  afterEach(cleanup);

  it('never places the stored credential in the DOM and makes removal explicit', async () => {
    const [pingUrl, setPingUrl] = createSignal('***REDACTED***');
    const setHasUnsavedChanges = vi.fn();
    const { container } = render(() => (
      <AlertDeadManDestinationSection
        pingUrl={pingUrl}
        setPingUrl={setPingUrl}
        setHasUnsavedChanges={setHasUnsavedChanges}
      />
    ));

    expect(container.textContent).not.toContain('credential-token');
    const input = screen.getByLabelText('Healthchecks-compatible success ping URL');
    expect(input).toHaveAttribute('type', 'password');
    expect(input).toHaveValue('');
    expect(input).toHaveAttribute('placeholder', 'Configured — enter a new URL to replace');

    await waitFor(() => expect(screen.getByText('Heartbeat healthy')).toBeInTheDocument());
    expect(screen.getByText('5 min (unexpected stop)')).toBeInTheDocument();
    expect(screen.getByText('0')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));
    expect(pingUrl()).toBe('');
    expect(setHasUnsavedChanges).toHaveBeenCalledWith(true);
  });

  it('supports replacing and revealing only a newly entered URL', async () => {
    const [pingUrl, setPingUrl] = createSignal('***REDACTED***');
    const { container } = render(() => (
      <AlertDeadManDestinationSection
        pingUrl={pingUrl}
        setPingUrl={setPingUrl}
        setHasUnsavedChanges={vi.fn()}
      />
    ));
    const input = screen.getByLabelText('Healthchecks-compatible success ping URL');
    fireEvent.input(input, {
      target: { value: 'https://watchdog.example.test/ping/new-token' },
    });

    expect(input).toHaveValue('https://watchdog.example.test/ping/new-token');
    expect(container.textContent).not.toContain('***REDACTED***');
    // The section's actions are the shared outline Buttons, which give a
    // phone a 44px touch target.
    expect(screen.getByRole('button', { name: 'Show' })).toHaveClass('min-h-11', 'sm:min-h-9');
    expect(screen.getByRole('button', { name: /^Refresh/ })).toHaveClass('min-h-11', 'sm:min-h-0');
    fireEvent.click(screen.getByRole('button', { name: 'Show' }));
    expect(input).toHaveAttribute('type', 'text');
    fireEvent.click(screen.getByRole('button', { name: 'Hide' }));
    expect(input).toHaveAttribute('type', 'password');
  });

  it('surfaces encrypted configuration failures as an actionable state', async () => {
    vi.mocked(AlertsAPI.getDeadManStatus).mockResolvedValue({
      configured: true,
      state: 'configuration_unavailable',
      heartbeatIntervalSeconds: 60,
      recommendedGraceSeconds: 180,
      consecutiveFailures: 0,
      lastError: 'Saved external watchdog configuration could not be read',
    });
    const [pingUrl, setPingUrl] = createSignal('***REDACTED***');
    render(() => (
      <AlertDeadManDestinationSection
        pingUrl={pingUrl}
        setPingUrl={setPingUrl}
        setHasUnsavedChanges={vi.fn()}
      />
    ));

    await waitFor(() => expect(screen.getByText('Configuration unavailable')).toBeInTheDocument());
    expect(
      screen.getByText('Saved external watchdog configuration could not be read'),
    ).toBeInTheDocument();
  });

  it('keeps Last success aging and re-reads the status while the panel stays open', async () => {
    const start = Date.parse('2026-08-27T12:00:30Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    const [pingUrl, setPingUrl] = createSignal('***REDACTED***');

    try {
      render(() => (
        <AlertDeadManDestinationSection
          pingUrl={pingUrl}
          setPingUrl={setPingUrl}
          setHasUnsavedChanges={vi.fn()}
        />
      ));
      const lastSuccess = () => screen.getByText('Last success').nextElementSibling;
      await waitFor(() => expect(lastSuccess()).toHaveTextContent('30s ago'));
      expect(AlertsAPI.getDeadManStatus).toHaveBeenCalledTimes(1);

      // The watchdog stops being reached: every background re-read returns the
      // same last success, and its age keeps moving on the clock.
      for (let poll = 2; poll <= 5; poll += 1) {
        vi.advanceTimersByTime(DEAD_MAN_STATUS_POLL_MS);
        await waitFor(() => expect(AlertsAPI.getDeadManStatus).toHaveBeenCalledTimes(poll));
      }
      expect(lastSuccess()).toHaveTextContent('2 mins ago');
      expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled();

      // A later heartbeat lands: the next background re-read picks it up.
      vi.mocked(AlertsAPI.getDeadManStatus).mockResolvedValue({
        ...(await AlertsAPI.getDeadManStatus()),
        lastSuccessAt: '2026-08-27T12:02:30Z',
      });
      vi.advanceTimersByTime(DEAD_MAN_STATUS_POLL_MS);
      await waitFor(() => expect(lastSuccess()).toHaveTextContent('30s ago'));
    } finally {
      vi.useRealTimers();
    }
  });

  it('keeps Refresh working and ignores a stale answer when a background read hangs', async () => {
    const start = Date.parse('2026-08-27T12:00:30Z');
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'], now: start });
    const base = await AlertsAPI.getDeadManStatus();
    vi.mocked(AlertsAPI.getDeadManStatus).mockClear();
    let releaseHungRead: (value: typeof base) => void = () => undefined;
    vi.mocked(AlertsAPI.getDeadManStatus)
      .mockResolvedValueOnce(base)
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            releaseHungRead = resolve;
          }),
      )
      .mockResolvedValue({ ...base, lastSuccessAt: '2026-08-27T12:00:55Z' });
    const [pingUrl, setPingUrl] = createSignal('***REDACTED***');

    try {
      render(() => (
        <AlertDeadManDestinationSection
          pingUrl={pingUrl}
          setPingUrl={setPingUrl}
          setHasUnsavedChanges={vi.fn()}
        />
      ));
      const lastSuccess = () => screen.getByText('Last success').nextElementSibling;
      await waitFor(() => expect(lastSuccess()).toHaveTextContent('30s ago'));

      // The first background read never settles.
      vi.advanceTimersByTime(DEAD_MAN_STATUS_POLL_MS);
      await waitFor(() => expect(AlertsAPI.getDeadManStatus).toHaveBeenCalledTimes(2));

      // Refresh still reads and applies the newer status.
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      await waitFor(() => expect(AlertsAPI.getDeadManStatus).toHaveBeenCalledTimes(3));
      await waitFor(() => expect(lastSuccess()).toHaveTextContent('5s ago'));
      expect(screen.getByRole('button', { name: 'Refresh' })).not.toBeDisabled();

      // The hung read finally answers with the older status; it must not win.
      releaseHungRead(base);
      await Promise.resolve();
      await Promise.resolve();
      expect(lastSuccess()).toHaveTextContent('5s ago');

      // Background reads keep going after the hang.
      vi.advanceTimersByTime(DEAD_MAN_STATUS_POLL_MS);
      await waitFor(() => expect(AlertsAPI.getDeadManStatus).toHaveBeenCalledTimes(4));
    } finally {
      vi.useRealTimers();
    }
  });
});
