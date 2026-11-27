import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { ConnectionStatusBadge } from '@/AppLayout';

afterEach(() => {
  cleanup();
});

describe('ConnectionStatusBadge', () => {
  it('renders a healthy connected state', () => {
    render(() => (
      <ConnectionStatusBadge
        connectionStatus={() => ({
          kind: 'connected',
          label: 'Connected',
          detail: 'Backend and live data stream are connected.',
          tone: 'healthy',
        })}
      />
    ));

    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(
      screen.getByRole('status', { name: 'Backend and live data stream are connected.' }),
    ).toBeInTheDocument();
  });

  it('renders a degraded sync reconnecting state without marking the shell disconnected', () => {
    render(() => (
      <ConnectionStatusBadge
        connectionStatus={() => ({
          kind: 'sync-reconnecting',
          label: 'Sync reconnecting',
          detail: 'Backend is healthy. Live updates are reconnecting.',
          tone: 'warning',
        })}
      />
    ));

    expect(screen.getByText('Sync reconnecting')).toBeInTheDocument();
    expect(
      screen.getByRole('status', { name: 'Backend is healthy. Live updates are reconnecting.' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Disconnected')).not.toBeInTheDocument();
  });

  it('renders a disconnected state when both backend and stream are unavailable', () => {
    render(() => (
      <ConnectionStatusBadge
        connectionStatus={() => ({
          kind: 'disconnected',
          label: 'Disconnected',
          detail: 'Backend and live data stream are unavailable.',
          tone: 'offline',
        })}
      />
    ));

    expect(screen.getByText('Disconnected')).toBeInTheDocument();
    expect(
      screen.getByRole('status', { name: 'Backend and live data stream are unavailable.' }),
    ).toBeInTheDocument();
  });

  it('uses WCAG AA contrast token pairs for healthy, degraded, and reconnecting states', () => {
    const cases = [
      {
        status: {
          kind: 'connected' as const,
          label: 'Connected',
          detail: 'Connected detail',
          tone: 'healthy' as const,
        },
        classes: ['bg-green-200', 'dark:bg-green-900', 'text-green-900', 'dark:text-green-100'],
      },
      {
        status: {
          kind: 'sync-reconnecting' as const,
          label: 'Sync reconnecting',
          detail: 'Sync reconnecting detail',
          tone: 'warning' as const,
        },
        classes: ['bg-amber-200', 'dark:bg-amber-900', 'text-amber-900', 'dark:text-amber-100'],
      },
      {
        status: {
          kind: 'reconnecting' as const,
          label: 'Reconnecting',
          detail: 'Reconnecting detail',
          tone: 'offline' as const,
        },
        classes: ['bg-yellow-200', 'dark:bg-yellow-900', 'text-yellow-900', 'dark:text-yellow-100'],
      },
    ];

    for (const entry of cases) {
      cleanup();
      render(() => <ConnectionStatusBadge connectionStatus={() => entry.status} />);
      const badge = screen.getByRole('status', { name: entry.status.detail });
      for (const token of entry.classes) {
        expect(badge).toHaveClass(token);
      }
    }
  });
});
