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
  it('renders Kubernetes Event fields with severity, newest-first ordering, and message context', () => {
    render(() => (
      <KubernetesEventsTable
        resources={[
          makeResource({
            id: 'normal-event',
            type: 'k8s-event',
            status: 'degraded',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'Event',
              eventType: 'Normal',
              reason: 'Scheduled',
              involvedKind: 'Pod',
              involvedName: 'checkout-web-456',
              count: 1,
              eventTime: '2026-05-24T12:00:00Z',
              message: 'Successfully assigned apps/checkout-web-456 to prod-k8s-01',
            },
          }),
          makeResource({
            id: 'warning-event',
            type: 'k8s-event',
            status: 'online',
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
    expect(screen.getByTitle('Warning')).toHaveClass('bg-amber-500');
    expect(screen.getByTitle('Normal')).toHaveClass('bg-slate-400');
    expect(screen.getByText('Warning')).toBeInTheDocument();
    expect(screen.getByText('FailedScheduling')).toBeInTheDocument();
    expect(screen.getByText('Pod/checkout-api-123')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('2026-05-24T13:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('0/3 nodes are available')).toBeInTheDocument();

    expect(
      Array.from(document.querySelectorAll('[data-kubernetes-event-row]')).map((row) =>
        row.getAttribute('data-kubernetes-event-row'),
      ),
    ).toEqual(['warning-event', 'normal-event']);
  });
});
