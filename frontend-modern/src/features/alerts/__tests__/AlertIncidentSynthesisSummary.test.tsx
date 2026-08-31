import { fireEvent, render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import type { Alert } from '@/types/api';
import { AlertIncidentSynthesisSummary } from '../AlertIncidentSynthesisSummary';

function makeAlert(inference: 'supported-cause' | 'observation-set'): Alert {
  return {
    id: 'host-offline',
    type: 'offline',
    level: 'critical',
    resourceId: 'agent:edge-1',
    resourceName: 'Edge host',
    node: '',
    instance: '',
    message: 'Edge host is offline',
    value: 0,
    threshold: 0,
    startTime: '2026-08-30T18:00:00Z',
    acknowledged: false,
    correlation: {
      key: 'infrastructure:host-offline',
      kind: 'infrastructure-incident',
      role: 'primary',
      reason:
        inference === 'supported-cause'
          ? 'Runtime failure on Edge host is connected to one delivery symptom.'
          : 'The observations share context, but causality is not established.',
      failureClass: 'runtime',
      inference,
      primaryAlertId: 'host-offline',
      primaryResourceId: 'agent:edge-1',
      affectedResourceIds: ['availability:checkout'],
      observations: [
        {
          alertId: 'host-offline',
          resourceId: 'agent:edge-1',
          resourceName: 'Edge host',
          failureClass: 'runtime',
          level: 'critical',
          observedAt: '2026-08-30T18:01:00Z',
        },
        {
          alertId: 'checkout-unreachable',
          resourceId: 'availability:checkout',
          resourceName: 'Checkout',
          failureClass: 'network-path',
          level: 'critical',
          observedAt: '2026-08-30T18:01:30Z',
          evidenceIds: ['evidence_checkout'],
        },
      ],
    },
  };
}

describe('AlertIncidentSynthesisSummary', () => {
  it('shows the supported cause and lets the operator inspect every observation', async () => {
    render(() => <AlertIncidentSynthesisSummary alert={makeAlert('supported-cause')} />);

    expect(screen.getByText('Supported infrastructure cause')).toBeInTheDocument();
    expect(screen.getByText('1 affected resources · 2 observations')).toBeInTheDocument();
    await fireEvent.click(screen.getByText('Review synthesis evidence'));
    expect(screen.getByText('Checkout')).toBeInTheDocument();
    expect(screen.getByText('evidence_checkout')).toBeInTheDocument();
    expect(screen.getByText(/compare its timing before accepting this cause/i)).toBeInTheDocument();
  });

  it('labels contradictory correlation as an observation set without claiming root cause', () => {
    render(() => <AlertIncidentSynthesisSummary alert={makeAlert('observation-set')} />);

    expect(screen.getByText('Related observation set')).toBeInTheDocument();
    expect(screen.getByText(/causality is not established/i)).toBeInTheDocument();
  });
});
