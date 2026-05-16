import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { JSX } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { AlertResourceIncidentsPanel } from '../AlertResourceIncidentsPanel';
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

  it('does not surface cross-jump links to the retired top-level routes', () => {
    // Surface link chips to /infrastructure?resource=...,
    // /workloads?type=...&platform=..., /storage?source=..., and
    // /recovery?platform=... were retired with the platform-first migration.
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
});
