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
  it('marks metadata-only config and secret rows without implying payload fields were read', () => {
    render(() => (
      <KubernetesInventoryTable
        resources={[
          makeResource({
            id: 'api-config',
            type: 'k8s-configmap',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'ConfigMap',
              dataKeys: ['app.yaml'],
              metadataOnly: true,
            },
          }),
          makeResource({
            id: 'api-secret',
            type: 'k8s-secret',
            kubernetes: {
              namespace: 'apps',
              resourceKind: 'Secret',
              dataKeys: ['token'],
              metadataOnly: true,
            },
          }),
        ]}
        variant="config"
        emptyIcon={<span />}
        emptyTitle="No config"
        emptyDescription="No config"
        showToolbar={false}
      />
    ));

    expect(screen.getAllByText('Metadata-only')).toHaveLength(2);
    expect(screen.getAllByText('Payload omitted')).toHaveLength(2);
    expect(screen.queryByText('app.yaml')).not.toBeInTheDocument();
    expect(screen.queryByText('token')).not.toBeInTheDocument();
    expect(screen.queryByText('Mutable')).not.toBeInTheDocument();
  });
});
