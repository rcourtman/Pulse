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
    expect(screen.getByText('1m 30s')).toBeInTheDocument();
    expect(screen.getByText('7 ms')).toBeInTheDocument();
  });

  it('links the empty state back to the availability check add flow', () => {
    renderTable([]);

    expect(screen.getByRole('link', { name: 'Add service/device check' })).toHaveAttribute(
      'href',
      '/settings/monitoring/availability?add=target&targetKind=service',
    );
  });
});
