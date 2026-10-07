import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { createSignal, type JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { MobileAlertHistoryInvestigationDialog } from '../MobileAlertHistoryInvestigationDialog';
import { AlertResourceIncidentsPanel } from '../AlertResourceIncidentsPanel';
import { AlertsAPI } from '@/api/alerts';
import type { Incident } from '@/types/api';
import type { AlertHistoryState } from '../useAlertHistoryState';
import { useAlertResourceIncidentsState } from '../useAlertResourceIncidentsState';
import { aiChatStore } from '@/stores/aiChat';

vi.mock('@solidjs/router', () => ({
  A: (props: { href: string; children?: JSX.Element; [key: string]: unknown }) => (
    <a href={props.href} aria-label={props['aria-label'] as string}>
      {props.children}
    </a>
  ),
}));

describe('AlertResourceIncidentsPanel', () => {
  afterEach(() => {
    aiChatStore.close();
    aiChatStore.clearAllContext();
    aiChatStore.setEnabled(false);
    cleanup();
    vi.restoreAllMocks();
  });

  it.each(['inline', 'mobile drawer'] as const)(
    'retries through the real hook in the %s and retains cached history after a refresh failure',
    async (surface) => {
      let state!: ReturnType<typeof useAlertResourceIncidentsState>;
      const read = vi.spyOn(AlertsAPI, 'getIncidentsForResource');
      read.mockRejectedValueOnce(new Error('history unavailable'));
      render(() => {
        state = useAlertResourceIncidentsState();
        return surface === 'inline' ? (
          <AlertResourceIncidentsPanel state={state as unknown as AlertHistoryState} />
        ) : (
          <MobileAlertHistoryInvestigationDialog
            investigation={{
              kind: 'resource',
              rowKey: 'row-1',
              alert: { resourceName: 'Resource' } as Parameters<
                typeof MobileAlertHistoryInvestigationDialog
              >[0]['investigation']['alert'],
            }}
            state={state as unknown as AlertHistoryState}
            onClose={() => state.setResourceIncidentPanel(null)}
          />
        );
      });

      await state.openResourceIncidentPanel('resource-1', 'Resource', 'row-1');
      expect(screen.getByRole('alert')).toHaveTextContent('Use Refresh');
      expect(screen.queryByText('No incidents recorded for this resource yet.')).toBeNull();

      let resolve!: (incidents: Incident[]) => void;
      read.mockImplementationOnce(
        () =>
          new Promise((done) => {
            resolve = done;
          }),
      );
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.getByRole('button', { name: /Refresh/ })).toBeDisabled();
      resolve([
        {
          id: 'retained',
          alertIdentifier: 'resource-1::connectivity',
          alertType: 'connectivity',
          level: 'critical',
          resourceId: 'resource-1',
          resourceName: 'Resource',
          status: 'resolved',
          acknowledged: false,
          events: [],
          openedAt: '2026-09-08T00:00:00Z',
          message: 'Retained connection incident',
        },
      ]);
      await waitFor(() => expect(screen.getByText('Retained connection incident')).toBeVisible());
      expect(read).toHaveBeenLastCalledWith('resource-1', 10);

      read.mockRejectedValueOnce(new Error('refresh unavailable'));
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Use Refresh'));
      expect(screen.getByText('Retained connection incident')).toBeVisible();
      expect(screen.queryByText('No incidents recorded for this resource yet.')).toBeNull();

      read.mockResolvedValueOnce([]);
      fireEvent.click(screen.getByRole('button', { name: 'Refresh' }));
      await waitFor(() =>
        expect(screen.getByText('No incidents recorded for this resource yet.')).toBeVisible(),
      );
      expect(screen.queryByRole('alert')).toBeNull();
      expect(screen.queryByText('Retained connection incident')).toBeNull();
    },
  );

  it('shows persistent read failure without claiming an empty history', () => {
    const [failed, setFailed] = createSignal(true);
    render(() => (
      <AlertResourceIncidentsPanel
        state={
          {
            resourceIncidentPanel: () => ({ resourceId: 'resource-1', resourceName: 'Resource' }),
            resourceIncidents: () => ({}),
            resourceIncidentLoading: () => ({}),
            resourceIncidentError: () => ({ 'resource-1': failed() }),
            refreshResourceIncidentPanel: vi.fn(),
          } as any
        }
      />
    ));
    expect(screen.getByRole('alert').textContent).toContain('Use Refresh');
    expect(screen.queryByText('No incidents recorded for this resource yet.')).toBeNull();
    setFailed(false);
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('does not surface broad cross-jump links from incident details', () => {
    // Surface link chips into /infrastructure and broad aggregate workspaces
    // were retired with the platform-first migration.
    render(() => (
      <AlertResourceIncidentsPanel
        state={
          {
            resourceIncidentPanel: () => ({
              resourceId: 'truenas-main',
              resourceName: 'TrueNAS Main',
            }),
            resourceIncidents: () => ({
              'truenas-main': [
                {
                  id: 'incident-1',
                  alertType: 'Storage Health',
                  level: 'critical',
                  status: 'open',
                  acknowledged: false,
                  openedAt: '2026-03-30T09:00:00Z',
                  message: 'Pool tank is DEGRADED',
                  events: [],
                },
              ],
            }),
            resourceIncidentError: () => ({}),
            resourceIncidentLoading: () => ({ 'truenas-main': false }),
            expandedResourceIncidentIds: () => new Set<string>(),
            resourceIncidentEventFilters: () => new Set<string>(['opened']),
            setResourceIncidentEventFilters: vi.fn(),
            refreshResourceIncidentPanel: vi.fn(),
            setResourceIncidentPanel: vi.fn(),
            toggleResourceIncidentDetails: vi.fn(),
          } as any
        }
        getResource={(resourceId) =>
          resourceId === 'truenas-main'
            ? ({
                id: 'truenas-main',
                type: 'agent',
                name: 'truenas-main',
                displayName: 'TrueNAS Main',
                platformId: 'truenas-main',
                platformType: 'truenas',
                sourceType: 'hybrid',
                status: 'online',
                lastSeen: Date.now(),
                platformData: { sources: ['truenas'] },
              } as any)
            : undefined
        }
      />
    ));

    expect(
      screen.queryByRole('link', { name: 'Open related infrastructure for TrueNAS Main' }),
    ).toBeNull();
    expect(
      screen.queryByRole('link', { name: 'Open related workloads for TrueNAS Main' }),
    ).toBeNull();
    expect(
      screen.queryByRole('link', { name: 'Open related storage for TrueNAS Main' }),
    ).toBeNull();
    expect(
      screen.queryByRole('link', { name: 'Open related recovery for TrueNAS Main' }),
    ).toBeNull();
  });

  it('describes only the open occurrence from its live alert', () => {
    // The recorded message is the breach that opened the occurrence; a
    // threshold alert held below its trigger reads lower now.
    const alertId = 'homelab-minipc::metric-threshold:temperature';
    const occurrence = (overrides: Partial<Incident>): Incident => ({
      id: 'occurrence',
      alertIdentifier: alertId,
      alertType: 'temperature',
      level: 'warning',
      resourceId: 'homelab-minipc',
      resourceName: 'minipc',
      status: 'open',
      acknowledged: false,
      openedAt: '2026-10-06T07:57:44Z',
      message: 'Node temperature at 80.0°C',
      events: [],
      ...overrides,
    });
    const now = Date.now();
    render(() => (
      <AlertResourceIncidentsPanel
        state={
          {
            resourceIncidentPanel: () => ({ resourceId: 'homelab-minipc', resourceName: 'minipc' }),
            resourceIncidents: () => ({
              'homelab-minipc': [
                occurrence({ id: 'open' }),
                // A cached row still marked open from an earlier start is a
                // different occurrence and must not take this alert's reading.
                occurrence({
                  id: 'cached-earlier',
                  openedAt: '2026-10-04T09:00:00Z',
                  message: 'Node temperature at 81.0°C',
                }),
                occurrence({
                  id: 'closed',
                  status: 'resolved',
                  openedAt: '2026-10-05T07:00:00Z',
                  closedAt: '2026-10-05T08:00:00Z',
                  message: 'Node temperature at 83.0°C',
                }),
              ],
            }),
            activeAlerts: () => ({
              [alertId]: {
                id: alertId,
                type: 'temperature',
                level: 'warning',
                resourceId: 'homelab-minipc',
                resourceName: 'minipc',
                node: 'minipc',
                instance: 'homelab',
                message: 'Node temperature at 80.0°C',
                value: 80,
                threshold: 80,
                startTime: '2026-10-06T07:57:44Z',
                lastSeen: new Date(now - 5 * 60_000).toISOString(),
                acknowledged: false,
                metricStatus: {
                  phase: 'latched',
                  value: 76,
                  unit: '°C',
                  observedAt: new Date(now).toISOString(),
                  trigger: 80,
                  recovery: 75,
                  recoveryDelaySeconds: 300,
                },
              },
            }),
            resourceIncidentError: () => ({}),
            resourceIncidentLoading: () => ({ 'homelab-minipc': false }),
            expandedResourceIncidentIds: () => new Set<string>(),
            resourceIncidentEventFilters: () => new Set<string>(['opened']),
            setResourceIncidentEventFilters: vi.fn(),
            refreshResourceIncidentPanel: vi.fn(),
            setResourceIncidentPanel: vi.fn(),
            toggleResourceIncidentDetails: vi.fn(),
          } as any
        }
      />
    ));

    const live = screen.getByText(
      'Temperature 76°C now, back under the 80°C alert level. Stays open until it reaches 75°C or lower and stays there for 5 minutes.',
    );
    expect(live).toHaveAttribute(
      'title',
      expect.stringMatching(/^Last reading at or above 80°C: 80°C, /),
    );
    expect(screen.queryByText('Node temperature at 80.0°C')).toBeNull();
    expect(screen.getByText('Node temperature at 83.0°C')).toBeInTheDocument();
    expect(screen.getByText('Node temperature at 81.0°C')).toBeInTheDocument();
    expect(screen.getAllByText(/now, back under the 80°C alert level/)).toHaveLength(1);
  });

  it('opens Assistant from a resource incident without carrying raw command details', () => {
    const openSpy = vi.spyOn(aiChatStore, 'open');
    aiChatStore.setEnabled(true);

    render(() => (
      <AlertResourceIncidentsPanel
        state={
          {
            resourceIncidentPanel: () => ({
              resourceId: 'truenas-main',
              resourceName: 'TrueNAS Main',
            }),
            resourceIncidents: () => ({
              'truenas-main': [
                {
                  id: 'incident-1',
                  alertIdentifier: 'storage:tank::zfs-pool-state',
                  alertType: 'zfs-pool-state',
                  level: 'critical',
                  resourceId: 'storage:tank',
                  resourceName: 'tank',
                  resourceType: 'storage',
                  status: 'open',
                  acknowledged: false,
                  openedAt: '2026-03-30T09:00:00Z',
                  message: 'Pool tank is DEGRADED',
                  events: [
                    {
                      id: 'event-1',
                      type: 'command',
                      timestamp: '2026-03-30T09:01:00Z',
                      summary: 'zpool clear tank',
                      details: {
                        command: 'zpool clear tank',
                        output_excerpt: 'secret-output',
                      },
                    },
                  ],
                },
              ],
            }),
            resourceIncidentError: () => ({}),
            resourceIncidentLoading: () => ({ 'truenas-main': false }),
            expandedResourceIncidentIds: () => new Set<string>(),
            resourceIncidentEventFilters: () => new Set<string>(['command']),
            setResourceIncidentEventFilters: vi.fn(),
            refreshResourceIncidentPanel: vi.fn(),
            setResourceIncidentPanel: vi.fn(),
            toggleResourceIncidentDetails: vi.fn(),
          } as any
        }
        getResource={() => undefined}
      />
    ));

    fireEvent.click(
      screen.getByRole('button', {
        name: 'Discuss incident incident-1 with Pulse Assistant',
      }),
    );

    expect(openSpy).toHaveBeenCalledTimes(1);
    const [context] = openSpy.mock.calls[0] as [Record<string, unknown>];
    expect(context).toMatchObject({
      targetType: 'storage',
      targetId: 'storage:tank',
      autonomousMode: false,
      briefing: {
        sourceLabel: 'Pulse Alerts',
        title: 'Incident timeline attached',
      },
    });
    expect(JSON.stringify(context)).not.toContain('zpool clear tank');
    expect(JSON.stringify(context)).not.toContain('secret-output');
  });

  it('reactively reveals and hides the selected resource incident timeline', async () => {
    const [expandedIncidentIds, setExpandedIncidentIds] = createSignal(new Set<string>());
    const [eventFilters, setEventFilters] = createSignal(new Set<string>(['opened', 'resolved']));
    const toggleResourceIncidentDetails = vi.fn((incidentId: string) => {
      setExpandedIncidentIds((current) => {
        const next = new Set(current);
        if (next.has(incidentId)) next.delete(incidentId);
        else next.add(incidentId);
        return next;
      });
    });

    render(() => (
      <AlertResourceIncidentsPanel
        state={
          {
            resourceIncidentPanel: () => ({
              resourceId: 'node-1',
              resourceName: 'Production node',
              rowKey: 'alert-1-row',
            }),
            resourceIncidents: () => ({
              'node-1': [
                {
                  id: 'incident-1',
                  alertType: 'connectivity',
                  level: 'critical',
                  status: 'resolved',
                  acknowledged: false,
                  openedAt: '2026-08-21T09:00:00Z',
                  closedAt: '2026-08-21T09:05:00Z',
                  message: 'Connection lost',
                  events: [
                    {
                      id: 'event-1',
                      type: 'opened',
                      timestamp: '2026-08-21T09:00:00Z',
                      summary: 'Connectivity incident opened',
                    },
                    {
                      id: 'event-2',
                      type: 'resolved',
                      timestamp: '2026-08-21T09:05:00Z',
                      summary: 'Connectivity restored',
                    },
                  ],
                },
              ],
            }),
            resourceIncidentError: () => ({}),
            resourceIncidentLoading: () => ({ 'node-1': false }),
            expandedResourceIncidentIds: expandedIncidentIds,
            resourceIncidentEventFilters: eventFilters,
            setResourceIncidentEventFilters: setEventFilters,
            refreshResourceIncidentPanel: vi.fn(),
            setResourceIncidentPanel: vi.fn(),
            toggleResourceIncidentDetails,
          } as any
        }
        getResource={() => undefined}
      />
    ));

    expect(screen.queryByText('Connectivity incident opened')).not.toBeInTheDocument();

    await fireEvent.click(screen.getByRole('button', { name: 'Events (2)' }));

    expect(toggleResourceIncidentDetails).toHaveBeenCalledWith('incident-1');
    expect(screen.getByRole('button', { name: 'Hide events' })).toBeInTheDocument();
    expect(screen.getByText('Connectivity incident opened')).toBeVisible();

    setEventFilters(new Set<string>(['resolved']));
    expect(screen.queryByText('Connectivity incident opened')).not.toBeInTheDocument();
    expect(screen.getByText('Connectivity restored')).toBeVisible();

    await fireEvent.click(screen.getByRole('button', { name: 'Hide events' }));
    expect(screen.queryByText('Connectivity incident opened')).not.toBeInTheDocument();
  });
});
