import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesEventsTable } from '../KubernetesEventsTable';

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  name: id,
  displayName: id,
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  sources: ['kubernetes'],
  status: 'degraded',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesEventsTable', () => {
  it('renders Kubernetes Event fields with object, count, time, and message context', () => {
    render(() => (
      <KubernetesEventsTable
        resources={[
          makeResource({
            id: 'event-1',
            type: 'k8s-event',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'Event',
              eventType: 'Warning',
              reason: 'FailedScheduling',
              involvedKind: 'Pod',
              involvedName: 'checkout-api-123',
              count: 3,
              eventTime: '2026-05-24T13:00:00Z',
              message: '0/3 nodes are available',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No events"
        emptyDescription="No events"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Event')).toBeInTheDocument();
    expect(screen.getByText('Type')).toBeInTheDocument();
    expect(screen.getByText('Reason')).toBeInTheDocument();
    expect(screen.getByText('Object')).toBeInTheDocument();
    expect(screen.getByText('Observed')).toBeInTheDocument();
    expect(screen.getByText('Warning')).toBeInTheDocument();
    expect(screen.getByText('FailedScheduling')).toBeInTheDocument();
    expect(screen.getByText('Pod/checkout-api-123')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('2026-05-24T13:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('0/3 nodes are available')).toBeInTheDocument();
  });
});
