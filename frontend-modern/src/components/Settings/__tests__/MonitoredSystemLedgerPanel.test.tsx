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
      expect(screen.getByText('Monitored system usage is temporarily unavailable.')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('recovers from error when Retry is clicked and API succeeds', async () => {
    getLedgerMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('Monitored system usage is temporarily unavailable.')).toBeInTheDocument();
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
          explanation: {
            summary:
              'Counts as one monitored system because Pulse sees one top-level host view from agent.',
            reasons: [
              {
                kind: 'standalone',
                signal: 'single-top-level-view',
                summary: 'No overlapping top-level source matched this system.',
              },
            ],
            surfaces: [{ name: 'host-1', type: 'host', source: 'agent' }],
          },
        },
      ],
      total: 1,
      limit: 5,
    });

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

    await waitFor(() => {
      expect(screen.getByText('host-1')).toBeInTheDocument();
    });

    expect(
      screen.queryByText('Monitored system usage is temporarily unavailable.'),
    ).not.toBeInTheDocument();
  });

  it('keeps Retry button visible on repeated failures', async () => {
    getLedgerMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('Monitored system usage is temporarily unavailable.')).toBeInTheDocument();
    });

    const callsBefore = getLedgerMock.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

    await waitFor(() => {
      expect(screen.getByText('Monitored system usage is temporarily unavailable.')).toBeInTheDocument();
    });

    expect(getLedgerMock.mock.calls.length).toBeGreaterThan(callsBefore);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
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
          explanation: {
            summary:
              'Counts as one monitored system because Pulse sees one top-level host view from agent.',
            reasons: [
              {
                kind: 'standalone',
                signal: 'single-top-level-view',
                summary: 'No overlapping top-level source matched this system.',
              },
            ],
            surfaces: [{ name: 'server-a', type: 'host', source: 'agent' }],
          },
        },
        {
          name: 'server-b',
          type: 'pbs-server',
          status: 'offline',
          last_seen: '2026-01-02T00:00:00Z',
          source: 'pbs',
          explanation: {
            summary:
              'Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.',
            reasons: [
              {
                kind: 'shared-identity',
                signal: 'machine-id',
                summary: 'Merged host and PBS server views using shared machine identity.',
              },
            ],
            surfaces: [
              { name: 'server-b', type: 'pbs-server', source: 'pbs' },
              { name: 'server-b host', type: 'host', source: 'agent' },
            ],
          },
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
        'Review the monitored systems currently counted against your Pulse Pro plan limit.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View counting rules' })).toBeInTheDocument();
    expect(
      screen.queryByText(/a monitored system is a top-level machine or cluster pulse actively monitors/i),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View counting rules' }));

    expect(
      screen.getByText(/a monitored system is a top-level machine or cluster pulse actively monitors/i),
    ).toBeInTheDocument();
    expect(screen.getByText('server-b')).toBeInTheDocument();
    expect(screen.getByText('2 / 10')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'View counting details' })).toHaveLength(2);
    fireEvent.click(screen.getAllByRole('button', { name: 'View counting details' })[1]!);
    expect(
      screen.getByText(
        'Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Included collection paths')).toBeInTheDocument();
    expect(screen.getByText('server-b host (host, agent)')).toBeInTheDocument();
    expect(
      screen.queryByText('Monitored system usage is temporarily unavailable.'),
    ).not.toBeInTheDocument();
  });

  it('does not crash when explanation payload is missing', async () => {
    getLedgerMock.mockResolvedValue({
      systems: [
        {
          name: 'server-a',
          type: 'host',
          status: 'online',
          last_seen: '2026-01-01T00:00:00Z',
          source: 'agent',
        },
      ],
      total: 1,
      limit: 10,
    } as MonitoredSystemLedgerResponse);

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-a')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'View counting details' }));

    expect(
      screen.getByText('Pulse counts this top-level collection path as one monitored system.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Included collection paths')).not.toBeInTheDocument();
  });
});
