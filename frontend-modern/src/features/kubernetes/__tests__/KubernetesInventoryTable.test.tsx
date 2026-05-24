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
  it('renders remaining policy inventory fields in the generic table', () => {
    render(() => (
      <KubernetesInventoryTable
        resources={[
          makeResource({
            id: 'default-deny',
            type: 'k8s-network-policy',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'NetworkPolicy',
              policyTypes: ['Ingress', 'Egress'],
              ingressRuleCount: 1,
              egressRuleCount: 2,
            },
          }),
        ]}
        variant="policy"
        emptyIcon={<span />}
        emptyTitle="No policy"
        emptyDescription="No policy"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Spec')).toBeInTheDocument();
    expect(screen.getByText('Detail')).toBeInTheDocument();
    expect(screen.getByText('Ingress, Egress')).toBeInTheDocument();
    expect(screen.getByText('1 ingress, 2 egress')).toBeInTheDocument();
  });
});
