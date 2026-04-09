import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';

import type {
  MonitoredSystemLedgerExplainResponse,
  MonitoredSystemLedgerResponse,
} from '@/api/monitoredSystemLedger';

const explainMock = vi.fn<() => Promise<MonitoredSystemLedgerExplainResponse>>();
const presentationPolicyHidesCommercialSurfacesMock = vi.fn(() => false);
const sessionPresentationPolicyResolvedMock = vi.fn(() => true);

const explainResponse = (
  ledger: MonitoredSystemLedgerResponse,
): MonitoredSystemLedgerExplainResponse => ({
  ledger,
  preview: null,
});

vi.mock('@/api/monitoredSystemLedger', () => ({
  MonitoredSystemLedgerAPI: {
    explain: () => explainMock(),
  },
}));

vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesCommercialSurfaces: () => presentationPolicyHidesCommercialSurfacesMock(),
  sessionPresentationPolicyResolved: () => sessionPresentationPolicyResolvedMock(),
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
  beforeEach(() => {
    presentationPolicyHidesCommercialSurfacesMock.mockReset();
    sessionPresentationPolicyResolvedMock.mockReset();
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(false);
    sessionPresentationPolicyResolvedMock.mockReturnValue(true);
  });

  afterEach(() => {
    cleanup();
    explainMock.mockReset();
  });

  it('waits for the session presentation policy before requesting ledger data', () => {
    sessionPresentationPolicyResolvedMock.mockReturnValue(false);

    render(() => <MonitoredSystemLedgerPanel />);

    expect(explainMock).not.toHaveBeenCalled();
    expect(screen.getByText('Checking monitored-system visibility')).toBeInTheDocument();
    expect(screen.getByText(/before loading usage or plan-limit data/i)).toBeInTheDocument();
  });

  it('hides monitored-system usage in demo mode without requesting the ledger', () => {
    presentationPolicyHidesCommercialSurfacesMock.mockReturnValue(true);

    render(() => (
      <MonitoredSystemLedgerPanel
        monitoredSystemLimit={{
          key: 'max_monitored_systems',
          limit: 5,
          current: 16,
          state: 'enforced',
        }}
      />
    ));

    expect(explainMock).not.toHaveBeenCalled();
    expect(screen.getByText('Monitored-system usage is hidden in demo mode')).toBeInTheDocument();
    expect(screen.getByText(/instead of creating a demo license/i)).toBeInTheDocument();
    expect(screen.queryByText('16 / 5')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Try again' })).not.toBeInTheDocument();
  });

  it('shows monitored-system loading copy while the ledger request is pending', () => {
    explainMock.mockReturnValue(new Promise<MonitoredSystemLedgerExplainResponse>(() => {}));

    render(() => <MonitoredSystemLedgerPanel />);

    expect(screen.getByText('Loading monitored system usage…')).toBeInTheDocument();
  });

  it('shows error message with Retry button when API fails', async () => {
    explainMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(
        screen.getByText('Monitored system usage is temporarily unavailable.'),
      ).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('recovers from error when Retry is clicked and API succeeds', async () => {
    explainMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(
        screen.getByText('Monitored system usage is temporarily unavailable.'),
      ).toBeInTheDocument();
    });

    explainMock.mockResolvedValue(
      explainResponse({
        systems: [
          {
            name: 'host-1',
            type: 'host',
            status: 'online',
            status_explanation: {
              summary: 'All included top-level collection paths currently report online status.',
              reasons: [],
            },
            latest_included_signal: {
              name: 'host-1',
              type: 'host',
              source: 'agent',
              at: '2026-01-01T00:00:00Z',
            },
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
      }),
    );

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

    await waitFor(() => {
      expect(screen.getByText('host-1')).toBeInTheDocument();
    });

    expect(
      screen.queryByText('Monitored system usage is temporarily unavailable.'),
    ).not.toBeInTheDocument();
  });

  it('keeps Retry button visible on repeated failures', async () => {
    explainMock.mockRejectedValue(new Error('network error'));

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(
        screen.getByText('Monitored system usage is temporarily unavailable.'),
      ).toBeInTheDocument();
    });

    const callsBefore = explainMock.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

    await waitFor(() => {
      expect(
        screen.getByText('Monitored system usage is temporarily unavailable.'),
      ).toBeInTheDocument();
    });

    expect(explainMock.mock.calls.length).toBeGreaterThan(callsBefore);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('shows a verification state when canonical monitored-system usage is not settled', async () => {
    explainMock.mockRejectedValue(
      Object.assign(new Error('usage unavailable'), {
        code: 'monitored_system_usage_unavailable',
        details: { reason: 'supplemental_inventory_unsettled' },
      }),
    );

    render(() => (
      <MonitoredSystemLedgerPanel
        monitoredSystemLimit={{
          key: 'max_monitored_systems',
          limit: 5,
          current: 16,
          current_available: false,
          current_unavailable_reason: 'supplemental_inventory_unsettled',
          state: 'enforced',
        }}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('Verifying monitored-system inventory')).toBeInTheDocument();
    });

    expect(screen.getByText('Verifying…')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('0 / 5')).not.toBeInTheDocument();
    expect(
      screen.queryByText('Monitored system usage is temporarily unavailable.'),
    ).not.toBeInTheDocument();
  });

  it('surfaces monitored-system continuity context from entitlements', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [],
        total: 7,
        limit: 12,
      }),
    );

    render(() => (
      <MonitoredSystemLedgerPanel
        monitoredSystemContinuity={{
          plan_limit: 5,
          grandfathered_floor: 12,
          effective_limit: 12,
          capture_pending: true,
        }}
      />
    ));

    await waitFor(() => {
      expect(screen.getByText('7 / 12')).toBeInTheDocument();
    });

    expect(screen.getByText('Plan continuity')).toBeInTheDocument();
    expect(screen.getByText('Plan limit')).toBeInTheDocument();
    expect(screen.getByText('Effective limit')).toBeInTheDocument();
    expect(screen.getByText('Grandfathered floor')).toBeInTheDocument();
    expect(screen.getByText('Continuity capture')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getAllByText('12')).toHaveLength(2);
    expect(screen.getByText('Pending')).toBeInTheDocument();
  });

  it('renders ledger data on successful load', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [
          {
            name: 'server-a',
            type: 'host',
            status: 'online',
            status_explanation: {
              summary: 'All included top-level collection paths currently report online status.',
              reasons: [],
            },
            latest_included_signal: {
              name: 'server-a',
              type: 'host',
              source: 'agent',
              at: '2026-01-01T00:00:00Z',
            },
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
            status_explanation: {
              summary:
                'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
              reasons: [
                {
                  kind: 'source-offline',
                  name: 'server-b',
                  type: 'pbs-server',
                  source: 'pbs',
                  status: 'offline',
                  reported_at: '2026-01-01T23:55:00Z',
                  summary:
                    'PBS data for server-b is offline or disconnected (last reported 2026-01-01T23:55:00Z).',
                },
              ],
            },
            latest_included_signal: {
              name: 'server-b',
              type: 'pbs-server',
              source: 'pbs',
              at: '2026-01-02T00:00:00Z',
            },
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
      }),
    );

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-a')).toBeInTheDocument();
    });

    expect(screen.getByText('Monitored System Ledger')).toBeInTheDocument();
    expect(screen.getByText('Latest Included Signal')).toBeInTheDocument();
    expect(screen.getByText('2026-01-02T00:00:00Z')).toBeInTheDocument();
    expect(screen.getAllByText('server-b (PBS Server via PBS)').length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        'Review the monitored systems currently counted against your Pulse Pro plan limit.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View counting rules' })).toBeInTheDocument();
    expect(
      screen.queryByText(
        /a monitored system is a top-level machine or cluster pulse actively monitors/i,
      ),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'View counting rules' }));

    expect(
      screen.getByText(
        /a monitored system is a top-level machine or cluster pulse actively monitors/i,
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('server-b')).toBeInTheDocument();
    expect(screen.getByText('2 / 10')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: 'View counting details' })).toHaveLength(2);
    fireEvent.click(screen.getAllByRole('button', { name: 'View counting details' })[1]!);
    expect(
      screen.getAllByText(
        'Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.',
      ),
    ).toHaveLength(2);
    expect(screen.getAllByText('Counts as 1 monitored system')).toHaveLength(2);
    expect(screen.getByText('2 grouped sources')).toBeInTheDocument();
    expect(screen.getByText('Current status')).toBeInTheDocument();
    expect(
      screen.getByText(
        'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Latest included signal: server-b (PBS Server via PBS), reported 2026-01-02T00:00:00Z.',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'PBS data for server-b is offline or disconnected (last reported 2026-01-01T23:55:00Z).',
      ),
    ).toBeInTheDocument();
    expect(screen.getByText('Why this counts')).toBeInTheDocument();
    expect(screen.getByText('Grouped sources')).toBeInTheDocument();
    expect(screen.getAllByText('server-b host (Host via Agent)').length).toBeGreaterThan(0);
    expect(
      screen.queryByText('Monitored system usage is temporarily unavailable.'),
    ).not.toBeInTheDocument();
  });

  it('does not crash when explanation payload is missing', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [
          {
            name: 'server-a',
            type: 'host',
            status: 'online',
            status_explanation: {
              summary: 'All included top-level collection paths currently report online status.',
              reasons: [],
            },
            explanation: {
              summary: 'Pulse counts this top-level collection path as one monitored system.',
              reasons: [],
              surfaces: [],
            },
            latest_included_signal: {
              name: 'server-a',
              type: 'host',
              source: 'agent',
              at: '2026-01-01T00:00:00Z',
            },
            source: 'agent',
          },
        ],
        total: 1,
        limit: 10,
      }),
    );

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-a')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'View counting details' }));

    expect(
      screen.getAllByText('Pulse counts this top-level collection path as one monitored system.'),
    ).toHaveLength(2);
    expect(
      screen.getByText('All included top-level collection paths currently report online status.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Grouped sources')).not.toBeInTheDocument();
  });

  it('shows a customer-facing fallback when no included signal is available', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [
          {
            name: 'server-c',
            type: 'host',
            status: 'unknown',
            status_explanation: {
              summary:
                'Pulse cannot determine a canonical runtime status for this monitored system yet.',
              reasons: [],
            },
            explanation: {
              summary: 'Pulse counts this top-level collection path as one monitored system.',
              reasons: [],
              surfaces: [],
            },
            latest_included_signal: {
              name: 'server-c',
              type: 'host',
              source: 'agent',
              at: '',
            },
            source: 'agent',
          },
        ],
        total: 1,
        limit: 10,
      }),
    );

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-c')).toBeInTheDocument();
    });

    expect(screen.getByText('No included signal yet.')).toBeInTheDocument();
  });

  it('falls back to the canonical status summary for the row status when status explanation is missing', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [
          {
            name: 'server-d',
            type: 'host',
            status: 'offline',
            status_explanation: {
              summary:
                'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
              reasons: [],
            },
            explanation: {
              summary: 'Pulse counts this top-level collection path as one monitored system.',
              reasons: [],
              surfaces: [],
            },
            latest_included_signal: {
              name: 'server-d',
              type: 'host',
              source: 'agent',
              at: '2026-01-01T00:00:00Z',
            },
            source: 'agent',
          },
        ],
        total: 1,
        limit: 10,
      }),
    );

    render(() => <MonitoredSystemLedgerPanel />);

    await waitFor(() => {
      expect(screen.getByText('server-d')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'View counting details' }));

    expect(
      screen.getByText(
        'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
      ),
    ).toBeInTheDocument();
  });

  it('opens counting rules by default for usage-focused billing arrivals', async () => {
    explainMock.mockResolvedValue(
      explainResponse({
        systems: [],
        total: 2,
        limit: 5,
      }),
    );

    render(() => <MonitoredSystemLedgerPanel showCountingRulesByDefault />);

    await waitFor(() => {
      expect(screen.getByText('2 / 5')).toBeInTheDocument();
    });

    expect(screen.getByRole('button', { name: 'Hide counting rules' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(
      screen.getByText(/a monitored system is a top-level machine or cluster/i),
    ).toBeInTheDocument();
  });
});
