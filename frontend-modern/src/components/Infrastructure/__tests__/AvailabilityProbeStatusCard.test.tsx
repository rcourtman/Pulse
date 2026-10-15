import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ResourceAvailabilityMeta } from '@/types/resource';

import { AvailabilityProbeStatusCard } from '../AvailabilityProbeStatusCard';

describe('AvailabilityProbeStatusCard', () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });
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

  describe('current health requires fresh evidence', () => {
    const start = Date.parse('2026-10-15T19:00:00Z');
    const successful: ResourceAvailabilityMeta = {
      available: true,
      aggregateState: 'healthy',
      latencyMillis: 8,
      transportOutcome: 'reachable',
      applicationOutcome: 'passed',
      applicationStatusCode: 200,
      locations: [
        {
          locationId: 'pulse:local',
          kind: 'pulse',
          outcome: 'reachable',
          latencyMillis: 8,
          lastChecked: new Date(start).toISOString(),
        },
      ],
      certificate: {
        observedAt: new Date(start).toISOString(),
        notBefore: '2026-09-01T00:00:00Z',
        notAfter: '2027-09-01T00:00:00Z',
        fingerprintSha256: 'a'.repeat(64),
        trustStatus: 'trusted',
        chainValid: true,
        hostnameValid: true,
        selfSigned: false,
      },
    };
    beforeEach(() => {
      vi.useFakeTimers();
      vi.setSystemTime(start);
    });
    const expectRetained = () => {
      const card = screen.getByTestId('availability-probe-status');
      expect(within(card).getByTestId('availability-retained-evidence')).toHaveTextContent(
        'These are last reported results, not evidence of current availability.',
      );
      expect(within(card).queryByText('Responding normally')).toBeNull();
      expect(within(card).queryByText('Up', { exact: true })).toBeNull();
      expect(card.querySelector('[class*="emerald"]')).toBeNull();
      // Retain useful observations rather than replacing them with zeroes.
      expect(within(card).getByText('8ms')).toBeInTheDocument();
      expect(within(card).getByText('Endpoint answered')).toBeInTheDocument();
      expect(within(card).getByText('Contract passed · HTTP 200')).toBeInTheDocument();
      expect(within(card).getByText('Trusted')).toBeInTheDocument();
      expect(within(card).getByText('Matches')).toBeInTheDocument();
    };
    it.each([
      {},
      { lastChecked: 'invalid', pollIntervalSeconds: 30 },
      { lastChecked: new Date(start).toISOString() },
      { lastChecked: new Date(start).toISOString(), pollIntervalSeconds: 0 },
    ])('does not call an undated or unleased result healthy: %j', (evidence) => {
      render(() => <AvailabilityProbeStatusCard availability={{ ...successful, ...evidence }} />);
      expect(screen.getByText('Freshness unknown', { exact: true })).toBeInTheDocument();
      expectRetained();
    });
    it('uses the explicit expired lease even beside a recent checked time', () => {
      render(() => (
        <AvailabilityProbeStatusCard
          availability={{
            ...successful,
            lastChecked: new Date(start).toISOString(),
            pollIntervalSeconds: 30,
            evidence: { validUntil: new Date(start - 1).toISOString() } as never,
          }}
        />
      ));
      expect(screen.getByText('Stale', { exact: true })).toBeInTheDocument();
      expectRetained();
    });
    it('ages and recovers the same mounted card without a probe, remount or lost results', () => {
      const [availability, replace] = createSignal<ResourceAvailabilityMeta>({
        ...successful,
        lastChecked: new Date(start).toISOString(),
        pollIntervalSeconds: 30,
      });
      render(() => <AvailabilityProbeStatusCard availability={availability()} />);
      const card = screen.getByTestId('availability-probe-status');
      expect(screen.getByText('Responding normally')).toBeInTheDocument();
      expect(screen.getByText('Up', { exact: true })).toBeInTheDocument();
      vi.advanceTimersByTime(90_000);
      expect(screen.getByText('Stale', { exact: true })).toBeInTheDocument();
      expectRetained();
      replace({
        ...successful,
        evidence: { validUntil: new Date(start + 120_000).toISOString() } as never,
      });
      expect(screen.getByTestId('availability-probe-status')).toBe(card);
      expect(screen.getByText('Responding normally')).toBeInTheDocument();
      expect(screen.queryByTestId('availability-retained-evidence')).toBeNull();
    });
    it.each([undefined, 'invalid'])(
      'does not colour a path with no usable checked time: %s',
      (lastChecked) => {
        render(() => (
          <AvailabilityProbeStatusCard
            availability={{
              ...successful,
              lastChecked: new Date(start).toISOString(),
              pollIntervalSeconds: 30,
              locations: [{ ...successful.locations![0], lastChecked }],
            }}
          />
        ));
        expect(screen.getByText('Awaiting evidence')).not.toHaveClass('text-emerald-600');
        expect(screen.getByText('Awaiting evidence')).not.toHaveClass('text-red-600');
      },
    );
  });
});
