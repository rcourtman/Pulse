import { Route, Router } from '@solidjs/router';
import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { AvailabilityChecksTable } from '../AvailabilityChecksTable';

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

const renderTable = (resources: Resource[]) =>
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

  it('links the empty state back to the availability check add flow', () => {
    renderTable([]);

    expect(screen.getByRole('link', { name: 'Add service/device check' })).toHaveAttribute(
      'href',
      '/settings/monitoring/availability?add=target&targetKind=service',
    );
  });
});
