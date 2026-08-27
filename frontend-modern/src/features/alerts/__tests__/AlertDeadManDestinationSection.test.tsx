import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AlertsAPI } from '@/api/alerts';

import { AlertDeadManDestinationSection } from '../AlertDeadManDestinationSection';

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
});
