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
    const badge = screen.getByRole('status', {
      name: 'Backend and live data stream are connected.',
    });
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('status', 'connected');
  });

  it('renders a waiting-for-data warning state with its visible degraded label', () => {
    render(() => (
      <ConnectionStatusBadge
        connectionStatus={() => ({
          kind: 'waiting-for-data',
          label: 'Waiting for data',
          detail: 'Live updates are connected. Waiting for the first dashboard snapshot.',
          tone: 'warning',
        })}
      />
    ));

    const label = screen.getByText('Waiting for data');
    expect(label).toBeInTheDocument();
    expect(label).toHaveClass('max-w-[170px]', 'opacity-100');
    const badge = screen.getByRole('status', {
      name: 'Live updates are connected. Waiting for the first dashboard snapshot.',
    });
    expect(badge).toHaveClass('status', 'degraded');
    expect(badge.querySelector('svg')).toBeNull();
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
    const badge = screen.getByRole('status', {
      name: 'Backend is healthy. Live updates are reconnecting.',
    });
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('status', 'degraded');
    expect(badge.querySelector('svg')).not.toBeNull();
    expect(screen.queryByText('Disconnected')).not.toBeInTheDocument();
  });

  it('renders a reconnecting state with its visible reconnecting label', () => {
    render(() => (
      <ConnectionStatusBadge
        connectionStatus={() => ({
          kind: 'reconnecting',
          label: 'Reconnecting...',
          detail: 'Attempting to reconnect to the backend and live data stream.',
          tone: 'offline',
        })}
      />
    ));

    const label = screen.getByText('Reconnecting...');
    expect(label).toBeInTheDocument();
    expect(label).toHaveClass('max-w-[170px]', 'opacity-100');
    const badge = screen.getByRole('status', {
      name: 'Attempting to reconnect to the backend and live data stream.',
    });
    expect(badge).toHaveClass('status', 'reconnecting');
    expect(badge.querySelector('svg')).not.toBeNull();
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
    const badge = screen.getByRole('status', {
      name: 'Backend and live data stream are unavailable.',
    });
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('status', 'disconnected');
  });
});
