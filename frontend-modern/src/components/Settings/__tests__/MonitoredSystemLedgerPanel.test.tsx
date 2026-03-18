import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';

import type { MonitoredSystemLedgerResponse } from '@/api/monitoredSystemLedger';

const getLedgerMock = vi.fn<() => Promise<MonitoredSystemLedgerResponse>>();

vi.mock('@/api/monitoredSystemLedger', () => ({
  MonitoredSystemLedgerAPI: {
    getLedger: () => getLedgerMock(),
  },
}));

vi.mock('@/components/shared/SettingsPanel', () => ({
  default: (props: { children?: JSX.Element; title?: string; description?: string }) => (
    <section data-testid="settings-panel">
      <h2>{props.title}</h2>
      <p>{props.description}</p>
      {props.children}
    </section>
  ),
}));

vi.mock('@/components/shared/StatusDot', () => ({
  StatusDot: () => <span data-testid="status-dot" />,
}));

vi.mock('@/components/shared/Table', () => ({
  Table: (props: { children?: JSX.Element }) => <table>{props.children}</table>,
  TableHeader: (props: { children?: JSX.Element }) => <thead>{props.children}</thead>,
  TableBody: (props: { children?: JSX.Element }) => <tbody>{props.children}</tbody>,
  TableRow: (props: { children?: JSX.Element }) => <tr>{props.children}</tr>,
  TableHead: (props: { children?: JSX.Element }) => <th>{props.children}</th>,
  TableCell: (props: { children?: JSX.Element }) => <td>{props.children}</td>,
}));

vi.mock('@/utils/format', () => ({
  formatRelativeTime: (v: string) => v,
}));

import { MonitoredSystemLedgerPanel } from '../MonitoredSystemLedgerPanel';

describe('MonitoredSystemLedgerPanel', () => {
  afterEach(() => {
    cleanup();
    getLedgerMock.mockReset();
  });

  it('shows error message with Retry button when API fails', async () => {
    getLedgerMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load monitored system ledger.')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('recovers from error when Retry is clicked and API succeeds', async () => {
    getLedgerMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load monitored system ledger.')).toBeInTheDocument();
    });

    // Now make API succeed on retry
    getLedgerMock.mockResolvedValue({
      systems: [
        {
          name: 'host-1',
          type: 'host',
          status: 'online',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
      ],
      total: 1,
      limit: 5,
    });

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => {
      expect(screen.getByText('host-1')).toBeInTheDocument();
    });

    expect(screen.queryByText('Failed to load monitored system ledger.')).not.toBeInTheDocument();
  });

  it('keeps Retry button visible on repeated failures', async () => {
    getLedgerMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('Failed to load monitored system ledger.')).toBeInTheDocument();
    });

    const callsBefore = getLedgerMock.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }));

    await waitFor(() => {
      expect(screen.getByText('Failed to load monitored system ledger.')).toBeInTheDocument();
    });

    expect(getLedgerMock.mock.calls.length).toBeGreaterThan(callsBefore);
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  });

  it('renders ledger data on successful load', async () => {
    getLedgerMock.mockResolvedValue({
      systems: [
        {
          name: 'server-a',
          type: 'host',
          status: 'online',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
        {
          name: 'server-b',
          type: 'pbs-server',
          status: 'offline',
          last_seen: '2026-01-02T00:00:00Z',
          source: 'pbs',
        },
      ],
      total: 2,
      limit: 10,
    });

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-a')).toBeInTheDocument();
    });

    expect(screen.getByText('Monitored System Ledger')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Review the monitored systems currently counting toward your Pulse Pro allocation.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('server-b')).toBeInTheDocument();
    expect(screen.getByText('2 / 10')).toBeInTheDocument();
    expect(screen.queryByText('Failed to load monitored system ledger.')).not.toBeInTheDocument();
  });
});
