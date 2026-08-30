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
    expect(screen.getByText('Latency').parentElement).toHaveClass(
      'justify-between',
      'lg:grid',
      'lg:grid-cols-[7rem_minmax(0,1fr)]',
    );
    expect(screen.getByText('freshness unknown')).toHaveClass('text-right', 'lg:text-left');
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

  it('shows certificate trust, expiry, hostname, issuer, and fingerprint', () => {
    render(() => (
      <AvailabilityProbeStatusCard
        availability={{
          targetId: 'probe-pulse',
          address: 'pulse.example.test',
          protocol: 'https',
          enabled: true,
          available: true,
          lastChecked: new Date().toISOString(),
          certificateExpiryWarningDays: 30,
          certificate: {
            subject: 'pulse.example.test',
            issuer: 'Example CA',
            dnsNames: ['pulse.example.test'],
            notBefore: '2026-01-01T00:00:00Z',
            notAfter: '2027-01-01T00:00:00Z',
            fingerprintSha256: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
            chainValid: true,
            hostnameValid: true,
            selfSigned: false,
            trustStatus: 'trusted',
            observedAt: '2026-08-06T12:00:00Z',
          },
        }}
      />
    ));

    expect(screen.getByText('Trusted')).toBeInTheDocument();
    expect(screen.getAllByText('pulse.example.test')).toHaveLength(2);
    expect(screen.getByText('Example CA')).toBeInTheDocument();
    expect(screen.getByText('Matches')).toBeInTheDocument();
    expect(screen.getByText('SHA-256')).toBeInTheDocument();
    expect(screen.getByTitle(/0123456789abcdef/)).toHaveTextContent('0123456789abcdef…');
    expect(screen.getByTitle('Warning window: 30 days')).toHaveTextContent('2027');
  });

  it('separates endpoint reachability from application correctness', () => {
    render(() => (
      <AvailabilityProbeStatusCard
        availability={{
          targetId: 'probe-orders',
          address: 'orders.example.test',
          protocol: 'https',
          enabled: true,
          available: false,
          lastChecked: new Date().toISOString(),
          transportOutcome: 'reachable',
          applicationOutcome: 'failed',
          applicationStatusCode: 503,
          applicationFailureCode: 'status_mismatch',
          lastError: 'http response status 503 was outside the expected 200-299 range',
        }}
      />
    ));

    expect(screen.getByText('Endpoint answered')).toBeInTheDocument();
    expect(screen.getByText('Contract failed · HTTP 503')).toBeInTheDocument();
  });
});
