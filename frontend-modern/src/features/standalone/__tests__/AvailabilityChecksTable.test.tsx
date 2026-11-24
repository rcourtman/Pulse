import { Route, Router } from '@solidjs/router';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import type { ProbeAgentOption } from '@/utils/availabilityProbeAgents';
import { AvailabilityHistoryAPI } from '@/api/availabilityHistory';
import { AvailabilityChecksTable, resolveAvailabilityChecksView } from '../AvailabilityChecksTable';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

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

const renderTable = (
  resources: Resource[],
  probeAgentOptions?: ProbeAgentOption[],
  focusCheckId?: string,
) =>
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
            focusCheckId={focusCheckId}
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

  it.each(['filtered', 'empty'] as const)(
    'moves focus to the existing control when a live snapshot leaves the fleet %s',
    async (state) => {
      const check = availabilityResource();
      const other = availabilityResource({
        id: 'availability:other',
        name: 'Other check',
        displayName: 'Other check',
        platformId: 'other',
        availability: { ...check.availability!, targetId: 'other', address: 'other.example.test' },
      });
      const [resources, setResources] = createSignal(
        state === 'filtered' ? [check, other] : [check],
      );
      render(() => (
        <Router>
          <Route
            path="/"
            component={() => (
              <AvailabilityChecksTable
                resources={resources()}
                view="fleet"
                externalSearch={state === 'filtered' ? () => 'MQTT' : undefined}
                emptyIcon={<span />}
                emptyTitle="No checks"
                emptyDescription="Add checks"
              />
            )}
          />
        </Router>
      ));
      const search = screen.getByRole('textbox', { name: 'Search availability checks' });
      fireEvent.click(screen.getByRole('button', { name: 'Open details for MQTT power meter' }));
      screen.getByRole('button', { name: 'Collapse MQTT power meter details' }).focus();

      setResources(state === 'filtered' ? [other] : []);
      await Promise.resolve();
      expect(screen.queryByTestId('availability-fleet-detail')).not.toBeInTheDocument();
      if (state === 'filtered') {
        expect(screen.getByText('No checks match current filters')).toBeInTheDocument();
        expect(search).toHaveValue('MQTT');
        expect(search).toHaveFocus();
      } else {
        expect(screen.getByRole('link', { name: 'Add service/device check' })).toHaveFocus();
      }
      setResources([check, other]);
      expect(screen.queryByTestId('availability-fleet-detail')).not.toBeInTheDocument();
    },
  );

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

  it('opens the detail of the check a link asked for, and no other', () => {
    const asked = availabilityResource();
    const other = availabilityResource({ id: 'availability:other', name: 'Other check' });
    const { container } = renderTable([other, asked], undefined, asked.id);

    const open = [...container.querySelectorAll('[data-inline-platform-resource-detail-for]')];
    expect(
      open.map((node) => node.getAttribute('data-inline-platform-resource-detail-for')),
    ).toEqual([asked.id]);
    // The owner can close it: the link opens it once and does not hold it open.
    fireEvent.click(screen.getByRole('button', { name: `Collapse details for ${asked.name}` }));
    expect(container.querySelector('[data-inline-platform-resource-detail-for]')).toBeNull();
  });

  it('opens nothing for a link that names no check on the page', () => {
    const { container } = renderTable([availabilityResource()], undefined, 'availability:gone');
    expect(container.querySelector('[data-inline-platform-resource-detail-for]')).toBeNull();
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

  it('keeps the result whole on one line beside a multi-location source', () => {
    renderTable([
      availabilityResource({
        availability: {
          targetId: 'mock-availability-frontend',
          protocol: 'https',
          address: 'frontend.demo.pulse.local',
          enabled: true,
          available: false,
          lastChecked: 1_700_000_300_000,
          reportingLocations: 2,
          expectedLocations: 2,
          locations: [
            { locationId: 'pulse', kind: 'pulse' },
            { locationId: 'agent:host-edge-01', kind: 'agent', probeAgentId: 'host-edge-01' },
          ],
        } as unknown as Resource['availability'],
      }),
    ]);

    // Settings words the same fact the same way.
    const source = screen.getByText('2/2 locations reporting');
    // One line (rows are windowed at a single height): the source truncates
    // while the result keeps its full width at the cell's right edge.
    expect(source).toHaveClass('truncate');
    const line = source.closest('div');
    expect(line?.className).toContain('flex');
    const result = line?.lastElementChild as HTMLElement;
    expect(result).toHaveClass('shrink-0');
    expect(result.textContent).not.toBe('');
  });

  it('gives the check name the room on phones by hiding method and target', () => {
    renderTable([availabilityResource()]);
    const headers = screen.getAllByRole('columnheader');
    const method = headers.find((th) => th.textContent?.trim() === 'Method');
    const target = headers.find((th) => th.textContent?.trim() === 'Target');
    expect(method).toHaveClass('platform-table-phone-hidden');
    expect(target).toHaveClass('platform-table-phone-hidden');
    const row = screen.getByText('MQTT power meter').closest('tr')!;
    expect(row.children[1]).toHaveClass('platform-table-phone-hidden');
    expect(row.children[2]).toHaveClass('platform-table-phone-hidden');
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

  it('switches between 24h, 7d, 30d, and 1y history windows and updates batch query, column header, and coverage labels', async () => {
    const batchMock = vi.mocked(AvailabilityHistoryAPI.batch);
    batchMock.mockImplementation(async (_ids, range) => {
      if (range === '30d') {
        return {
          start: '2026-10-22T00:00:00Z',
          end: '2026-11-21T00:00:00Z',
          targets: [
            {
              targetId: 'mock-availability-mqtt-meter',
              summary: {
                reachableSeconds: 4 * 86_400,
                unreachableSeconds: 0,
                indeterminateSeconds: 0,
                unknownSeconds: 26 * 86_400,
                coveragePercent: 13.3,
                availabilityPercent: 100,
              },
            },
          ],
        };
      }
      if (range === '365d') {
        return {
          start: '2025-11-21T00:00:00Z',
          end: '2026-11-21T00:00:00Z',
          targets: [
            {
              targetId: 'mock-availability-mqtt-meter',
              summary: {
                reachableSeconds: 364 * 86_400,
                unreachableSeconds: 86_400,
                indeterminateSeconds: 0,
                unknownSeconds: 0,
                coveragePercent: 100,
                availabilityPercent: 99.73,
              },
            },
          ],
        };
      }
      return {
        start: '2026-11-20T00:00:00Z',
        end: '2026-11-21T00:00:00Z',
        targets: [
          {
            targetId: 'mock-availability-mqtt-meter',
            summary: {
              reachableSeconds: 86_400,
              unreachableSeconds: 0,
              indeterminateSeconds: 0,
              unknownSeconds: 0,
              coveragePercent: 100,
              availabilityPercent: 100,
            },
          },
        ],
      };
    });

    const { container } = renderTable([availabilityResource()]);
    expect(await screen.findByText('100%')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Uptime 24h' })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '30d' }));
    await vi.waitFor(() => {
      expect(batchMock).toHaveBeenCalledWith(['mock-availability-mqtt-meter'], '30d');
      expect(screen.getByRole('columnheader', { name: 'Uptime 30d' })).toBeInTheDocument();
      expect(container.querySelector('[data-availability-uptime]')).toHaveAttribute(
        'title',
        'Not enough history yet: 4d observed in the last 30 days',
      );
    });

    fireEvent.click(screen.getByRole('button', { name: '1y' }));
    await vi.waitFor(() => {
      expect(batchMock).toHaveBeenCalledWith(['mock-availability-mqtt-meter'], '365d');
      expect(screen.getByRole('columnheader', { name: 'Uptime 1y' })).toBeInTheDocument();
      expect(screen.getByText('99.73%')).toBeInTheDocument();
    });
  });
  it('shows the Table and Fleet presentation switch directly in the toolbar and keeps the Add check CTA visible in Fleet view', () => {
    const onViewChange = vi.fn();
    render(() => (
      <Router>
        <Route
          path="/"
          component={() => (
            <AvailabilityChecksTable
              resources={[availabilityResource()]}
              view="fleet"
              onViewChange={onViewChange}
              emptyIcon={<span />}
              emptyTitle="No checks"
              emptyDescription="Add checks"
            />
          )}
        />
      </Router>
    ));

    // Table | Fleet switch is directly accessible without opening the View menu
    const tableBtn = screen.getByRole('button', { name: 'Table' });
    const fleetBtn = screen.getByRole('button', { name: 'Fleet' });
    expect(tableBtn).toBeInTheDocument();
    expect(fleetBtn).toBeInTheDocument();
    fireEvent.click(tableBtn);
    expect(onViewChange).toHaveBeenCalledWith('table');

    // + Add service/device check remains visible in Fleet view
    expect(screen.getByRole('link', { name: /Add service\/device check/i })).toBeInTheDocument();
  });
});
