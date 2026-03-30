import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';

import { AlertResourceIncidentsPanel } from '../AlertResourceIncidentsPanel';

vi.mock('@solidjs/router', () => ({
  A: (props: { href: string; children: unknown; [key: string]: unknown }) => (
    <a href={props.href} aria-label={props['aria-label'] as string}>
      {props.children}
    </a>
  ),
}));

describe('AlertResourceIncidentsPanel', () => {
  it('surfaces canonical investigation handoff links for TrueNAS resources', () => {
    render(() => (
      <AlertResourceIncidentsPanel
        state={{
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
        } as any}
        getResource={(resourceId) =>
          resourceId === 'truenas-main'
            ? ({
                id: 'truenas-main',
                type: 'truenas',
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
      screen.getByRole('link', { name: 'Open related infrastructure for TrueNAS Main' }),
    ).toHaveAttribute('href', '/infrastructure?resource=truenas-main');
    expect(
      screen.getByRole('link', { name: 'Open related workloads for TrueNAS Main' }),
    ).toHaveAttribute(
      'href',
      '/workloads?type=app-container&platform=truenas&agent=truenas-main',
    );
    expect(
      screen.getByRole('link', { name: 'Open related storage for TrueNAS Main' }),
    ).toHaveAttribute('href', '/storage?source=truenas&node=truenas-main');
    expect(
      screen.getByRole('link', { name: 'Open related recovery for TrueNAS Main' }),
    ).toHaveAttribute('href', '/recovery?platform=truenas&node=truenas-main');
  });
});
