import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import type { ProbeAgentOption } from '@/utils/availabilityProbeAgents';
import { AvailabilityHistoryAPI } from '@/api/availabilityHistory';
import { AvailabilityChecksTable, resolveAvailabilityChecksView } from '../AvailabilityChecksTable';

vi.mock('@/api/availabilityHistory', () => ({
  AvailabilityHistoryAPI: { batch: vi.fn(async () => ({ targets: [] })) },
}));

const availabilityResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'availability:mock-availability-mqtt-meter',
    name: 'MQTT power meter',
    displayName: 'MQTT power meter',
    type: 'network-endpoint',
    platformId: 'mock-availability-mqtt-meter',
    platformType: 'availability',
    sourceType: 'api',
    sources: ['availability'],
    status: 'online',
    lastSeen: 1_700_000_000_000,
    availability: {
      targetId: 'mock-availability-mqtt-meter',
      protocol: 'tcp',
      address: 'power-meter-01.lab.local',
      port: 1883,
      enabled: true,
      available: true,
      latencyMillis: 7,
      lastChecked: 1_700_000_300_000,
      lastSuccess: 1_700_000_000_000,
      pollIntervalSeconds: 90,
      failureThreshold: 2,
    },
    ...overrides,
  }) as Resource;

const renderTable = (resources: Resource[], probeAgentOptions?: ProbeAgentOption[]) =>
  render(() => (
    <Router>
      <Route
        path="/"
        component={() => (
          <AvailabilityChecksTable
            resources={resources}
            emptyIcon={<span />}
            emptyTitle="No checks"
            emptyDescription="Add checks"
            probeAgentOptions={probeAgentOptions}
          />
        )}
      />
    </Router>
  ));

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe('AvailabilityChecksTable', () => {
  it('uses the fleet default only for estate-sized inventories without an explicit view', () => {
    expect(resolveAvailabilityChecksView(undefined, 19)).toBe('table');
    expect(resolveAvailabilityChecksView(undefined, 20)).toBe('fleet');
    expect(resolveAvailabilityChecksView('table', 50)).toBe('table');
    expect(resolveAvailabilityChecksView('fleet', 1)).toBe('fleet');
    expect(resolveAvailabilityChecksView(['fleet'], 1)).toBe('fleet');
  });

  it('renders agentless check status from unified network endpoint resources', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_600_000);

    renderTable([availabilityResource()]);

    expect(screen.getByText('Availability checks')).toBeInTheDocument();
    expect(screen.getByText('MQTT power meter')).toBeInTheDocument();
    expect(screen.getByText('TCP 1883')).toBeInTheDocument();
    expect(screen.getByText('power-meter-01.lab.local:1883')).toBeInTheDocument();
    expect(screen.getByText('5m ago')).toBeInTheDocument();
    expect(screen.getByText('10m ago')).toBeInTheDocument();
    expect(screen.getByText('1m 30s')).toBeInTheDocument();
    expect(screen.getByText('7 ms')).toBeInTheDocument();
  });

  it('keeps five high-value columns visible in the shared phone layout', () => {
    const { container } = renderTable([availabilityResource()]);
    const headers = [...container.querySelectorAll('thead th')];

    expect(headers).toHaveLength(9);
    expect(headers[0]).toHaveClass('platform-table-name-column', 'platform-table-mobile-w-30');
    expect(headers[1]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[1]).not.toHaveClass('hidden');
    expect(headers[2]).toHaveClass('platform-table-mobile-w-25');
    expect(headers[2]).not.toHaveClass('hidden');
    expect(headers[3]).toHaveClass('platform-table-mobile-w-15');
    // Uptime 24h joins from lg up, beside the latest result.
    expect(headers[4]).toHaveTextContent('Uptime 24h');
    expect(headers[4]).toHaveClass('hidden', 'lg:table-cell');
    expect(headers[5]).toHaveClass('platform-table-mobile-w-15');
    expect(headers[5]).not.toHaveClass('hidden');
    // Interval is configuration; only the widest tables carry it.
    expect(headers[8]).toHaveTextContent('Interval');
    expect(headers[8]).toHaveClass('hidden', '2xl:table-cell');
  });

  it('exposes complete availability details from the compact summary row', () => {
    const { container } = renderTable([availabilityResource()]);
    const row = container.querySelector('[data-availability-check-row]');

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    fireEvent.click(screen.getByRole('button', { name: 'Expand details for MQTT power meter' }));

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    expect(
      container.querySelector(
        '[data-inline-platform-resource-detail-for="availability:mock-availability-mqtt-meter"]',
      ),
    ).not.toBeNull();
  });

  it('places failing checks before healthy checks by default', () => {
    const view = renderTable([
      availabilityResource({ id: 'healthy', name: 'Healthy', displayName: 'Healthy' }),
      availabilityResource({
        id: 'offline',
        name: 'Offline',
        displayName: 'Offline',
        status: 'offline',
        availability: {
          targetId: 'offline',
          protocol: 'icmp',
          address: 'offline.lab.local',
          enabled: true,
          available: false,
          lastChecked: '2023-11-14T22:18:20.000Z',
          lastSuccess: '2023-11-14T21:56:40.000Z',
          consecutiveFailures: 4,
          failureThreshold: 2,
        },
      }),
    ]);

    const rows = [...view.container.querySelectorAll('[data-availability-check-row]')];
    expect(rows.map((row) => row.getAttribute('data-availability-check-row'))).toEqual([
      'offline',
      'healthy',
    ]);
  });

  it('chips probe-reported results with the agent host that produced them', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_600_000);

    renderTable(
      [
        availabilityResource({
          availability: {
            targetId: 'mock-availability-mqtt-meter',
            protocol: 'tcp',
            address: 'power-meter-01.lab.local',
            port: 1883,
            enabled: true,
            available: true,
            latencyMillis: 7,
            lastChecked: '2023-11-14T22:18:20.000Z',
            pollIntervalSeconds: 90,
            probeAgentId: 'host-edge-01',
          },
        }),
      ],
      [{ id: 'host-edge-01', label: 'Edge 01' }],
    );

    expect(screen.getByText('via Edge 01')).toBeInTheDocument();
  });

  it('falls back to the raw agent id when the host list does not contain it', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_700_000_600_000);

    renderTable([
      availabilityResource({
        availability: {
          targetId: 'mock-availability-mqtt-meter',
          protocol: 'tcp',
          address: 'power-meter-01.lab.local',
          enabled: true,
          available: true,
          lastChecked: '2023-11-14T22:18:20.000Z',
          probeAgentId: 'host-gone',
        },
      }),
    ]);

    expect(screen.getByText('via host-gone')).toBeInTheDocument();
  });

  it('shows no source chip for locally executed checks', () => {
    renderTable([availabilityResource()]);

    expect(screen.queryByText(/^via /)).not.toBeInTheDocument();
  });

  it('links the empty state back to the availability check add flow', () => {
    renderTable([]);

    expect(screen.getByRole('link', { name: 'Add service/device check' })).toHaveAttribute(
      'href',
      '/settings/monitoring/availability?add=target&targetKind=service',
    );
  });

  it('shows 24h uptime from history and says what consecutive failures mean', async () => {
    vi.mocked(AvailabilityHistoryAPI.batch).mockResolvedValueOnce({
      targets: [
        {
          targetId: 'mock-availability-mqtt-meter',
          summary: {
            reachableSeconds: 85_000,
            unreachableSeconds: 1_400,
            indeterminateSeconds: 0,
            unknownSeconds: 0,
            coveragePercent: 100,
            availabilityPercent: 98.4,
          },
        },
      ],
    } as never);

    const { container } = renderTable([
      availabilityResource({
        availability: {
          ...availabilityResource().availability!,
          available: false,
          consecutiveFailures: 3,
          failureThreshold: 2,
        },
      }),
    ]);

    const uptime = await screen.findByText('98.4%');
    expect(uptime).toHaveAttribute('data-availability-uptime');
    expect(uptime.className).toContain('text-amber-700');
    const failures = screen.getByText('3 in a row');
    expect(failures).toHaveAttribute(
      'title',
      '3 failed checks in a row. It counts as offline after 2.',
    );
    expect(container.textContent).not.toContain('3/2');
  });

  it('shows no uptime figure when most of the window was inconclusive', async () => {
    vi.mocked(AvailabilityHistoryAPI.batch).mockResolvedValueOnce({
      targets: [
        {
          targetId: 'mock-availability-mqtt-meter',
          summary: {
            reachableSeconds: 60,
            unreachableSeconds: 0,
            indeterminateSeconds: 86_340,
            unknownSeconds: 0,
            coveragePercent: 100,
            availabilityPercent: 100,
          },
        },
      ],
    } as never);

    const { container } = renderTable([availabilityResource()]);

    await vi.waitFor(() =>
      expect(container.querySelector('[data-availability-uptime]')).toHaveAttribute(
        'title',
        'Most results in the last 24h were inconclusive, so there is no uptime figure',
      ),
    );
    expect(container.querySelector('[data-availability-uptime]')).toHaveTextContent('—');
  });
});
