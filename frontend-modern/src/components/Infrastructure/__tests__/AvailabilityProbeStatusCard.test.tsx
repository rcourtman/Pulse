import { render, screen, within } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { AvailabilityProbeStatusCard } from '../AvailabilityProbeStatusCard';

describe('AvailabilityProbeStatusCard', () => {
  it('shows disagreement and keeps every observation path visible', () => {
    render(() => (
      <AvailabilityProbeStatusCard
        availability={{
          targetId: 'customer-api',
          address: 'api.service.local',
          protocol: 'https',
          enabled: true,
          available: true,
          aggregateState: 'degraded',
          disagreement: true,
          expectedLocations: 3,
          reportingLocations: 2,
          locations: [
            {
              locationId: 'pulse:local',
              kind: 'pulse',
              outcome: 'reachable',
              available: true,
              latencyMillis: 8,
              lastChecked: '2026-08-30T19:00:00Z',
            },
            {
              locationId: 'agent:edge-a',
              kind: 'agent',
              probeAgentId: 'edge-a',
              outcome: 'unreachable',
              available: false,
              lastChecked: '2026-08-30T19:00:00Z',
            },
            {
              locationId: 'agent:edge-b',
              kind: 'agent',
              probeAgentId: 'edge-b',
              outcome: 'indeterminate',
              available: false,
              stale: true,
              lastChecked: '2026-08-30T18:00:00Z',
            },
          ],
        }}
      />
    ));

    expect(screen.getByText('Paths disagree')).toBeInTheDocument();
    const paths = screen.getByText('Observation paths').parentElement?.parentElement;
    expect(paths).not.toBeNull();
    const scope = within(paths!);
    expect(scope.getByText('2/3 reporting')).toBeInTheDocument();
    expect(scope.getByText('This Pulse server')).toBeInTheDocument();
    expect(scope.getByText('8 ms')).toBeInTheDocument();
    expect(scope.getByText('edge-a')).toBeInTheDocument();
    expect(scope.getByText('Unreachable')).toBeInTheDocument();
    expect(scope.getByText('edge-b')).toBeInTheDocument();
    expect(scope.getByText('No recent report')).toBeInTheDocument();
  });
});
