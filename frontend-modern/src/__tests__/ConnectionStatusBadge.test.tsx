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
          detail:
            'Live connection open and monitoring state received. Individual readings may still be stale or unavailable.',
          tone: 'healthy',
        })}
      />
    ));

    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(
      screen.getByRole('status', {
        name: 'Live connection open and monitoring state received. Individual readings may still be stale or unavailable.',
      }),
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
});
