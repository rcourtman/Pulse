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
  it('renders controller inventory fields in the remaining generic table', () => {
    render(() => (
      <KubernetesInventoryTable
        resources={[
          makeResource({
            id: 'checkout-api-rs',
            type: 'k8s-replicaset',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'ReplicaSet',
              desiredReplicas: 5,
              readyReplicas: 4,
              reason: 'ReplicaFailure',
            },
          }),
        ]}
        variant="controllers"
        emptyIcon={<span />}
        emptyTitle="No controllers"
        emptyDescription="No controllers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Desired')).toBeInTheDocument();
    expect(screen.getByText('Ready')).toBeInTheDocument();
    expect(screen.getByText('Detail')).toBeInTheDocument();
    expect(screen.getByText('ReplicaSet')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('4')).toBeInTheDocument();
    expect(screen.getByText('ReplicaFailure')).toBeInTheDocument();
  });
});
