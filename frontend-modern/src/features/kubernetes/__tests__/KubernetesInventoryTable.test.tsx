import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesInventoryTable } from '../KubernetesInventoryTable';

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
  status: 'online',
  type,
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

afterEach(() => {
  cleanup();
});

describe('KubernetesInventoryTable', () => {
  it('renders event inventory fields in the remaining generic table', () => {
    render(() => (
      <KubernetesInventoryTable
        resources={[
          makeResource({
            id: 'event-1',
            type: 'k8s-event',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'Event',
              reason: 'FailedScheduling',
              involvedKind: 'Pod',
              involvedName: 'checkout-api-123',
              count: 3,
              message: '0/3 nodes are available',
            },
          }),
        ]}
        variant="events"
        emptyIcon={<span />}
        emptyTitle="No events"
        emptyDescription="No events"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Reason')).toBeInTheDocument();
    expect(screen.getByText('Object')).toBeInTheDocument();
    expect(screen.getByText('Count')).toBeInTheDocument();
    expect(screen.getByText('Message')).toBeInTheDocument();
    expect(screen.getByText('FailedScheduling')).toBeInTheDocument();
    expect(screen.getByText('Pod/checkout-api-123')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
    expect(screen.getByText('0/3 nodes are available')).toBeInTheDocument();
  });
});
