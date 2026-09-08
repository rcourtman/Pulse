import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { APP_BOOTSTRAP_SLOW_DELAY_MS, AppBootstrapStatus } from '@/components/AppBootstrapStatus';

describe('AppBootstrapStatus', () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('names and announces the initial connection work', () => {
    render(() => <AppBootstrapStatus />);

    expect(screen.getByRole('main')).toHaveAccessibleName('Connecting to Pulse');
    expect(screen.getByRole('status')).toHaveTextContent(
      'Checking your session and preparing the workspace.',
    );
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();
  });

  it('keeps waiting while making a slow bootstrap understandable and retryable', async () => {
    vi.useFakeTimers();
    const onRetry = vi.fn();
    render(() => <AppBootstrapStatus onRetry={onRetry} />);

    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS);

    expect(screen.getByRole('status')).toHaveTextContent(
      'Pulse is taking longer than expected to respond.',
    );
    expect(screen.getByText('You can keep waiting, or retry the connection.')).toBeVisible();

    const retry = screen.getByRole('button', { name: 'Retry connection' });
    fireEvent.click(retry);
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it('cancels a completed bootstrap timer and gives a fresh attempt its full waiting period', async () => {
    vi.useFakeTimers();
    const first = render(() => <AppBootstrapStatus />);
    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS - 1);
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();

    first.unmount();
    expect(vi.getTimerCount()).toBe(0);

    render(() => <AppBootstrapStatus />);
    await vi.advanceTimersByTimeAsync(1);
    expect(screen.getByRole('status')).toHaveTextContent(
      'Checking your session and preparing the workspace.',
    );
    expect(screen.queryByRole('button', { name: 'Retry connection' })).toBeNull();

    await vi.advanceTimersByTimeAsync(APP_BOOTSTRAP_SLOW_DELAY_MS - 1);
    expect(screen.getByRole('button', { name: 'Retry connection' })).toBeVisible();
  });
});
