import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { AvailabilityProbeStatusCard } from '@/components/Infrastructure/AvailabilityProbeStatusCard';

afterEach(cleanup);

describe('AvailabilityProbeStatusCard', () => {
  it('does not turn an unobserved check into a confirmed failure', () => {
    render(() => (
      <AvailabilityProbeStatusCard
        availability={{
          targetId: 'probe-new',
          address: 'api.example.test',
          protocol: 'https',
          enabled: true,
        }}
      />
    ));

    expect(screen.getByText('Not checked')).toBeInTheDocument();
    expect(screen.queryByText('Down')).not.toBeInTheDocument();
    expect(screen.getByText('freshness unknown')).toBeInTheDocument();
  });

  it('shows stale evidence and an unresolved canonical resource link', () => {
    render(() => (
      <AvailabilityProbeStatusCard
        availability={{
          targetId: 'probe-api',
          address: 'api.example.test',
          protocol: 'https',
          enabled: true,
          available: true,
          latencyMillis: 12,
          lastChecked: '2026-01-01T00:00:00Z',
          correlationState: 'unresolved',
          evidence: {
            id: 'evidence-probe-api',
            source: { provider: 'availability', collector: 'availability-poller' },
            subject: { resourceId: 'network-endpoint:probe-api' },
            observedAt: '2026-01-01T00:00:00Z',
            ingestedAt: '2026-01-01T00:00:00Z',
            validUntil: '2026-01-01T00:02:00Z',
            completeness: 'complete',
            confidence: 'confirmed',
            permissions: 'sufficient',
          },
        }}
      />
    ));

    expect(screen.getByText('Stale')).toBeInTheDocument();
    expect(screen.queryByText('Up')).not.toBeInTheDocument();
    expect(screen.getByText('stale')).toBeInTheDocument();
    expect(screen.getByText('Resource link is unresolved')).toBeInTheDocument();
    expect(screen.queryByText('Responding normally')).not.toBeInTheDocument();
  });
});
