import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesConfigTable } from '../KubernetesConfigTable';

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

describe('KubernetesConfigTable', () => {
  it('renders API-native config fields without exposing metadata-only payload keys', () => {
    render(() => (
      <KubernetesConfigTable
        resources={[
          makeResource({
            id: 'payments',
            type: 'k8s-namespace',
            kubernetes: {
              clusterId: 'cluster-1',
              resourceKind: 'Namespace',
              phase: 'Terminating',
            },
            tags: ['team:payments'],
          }),
          makeResource({
            id: 'api-config',
            type: 'k8s-configmap',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'ConfigMap',
              dataKeys: ['app.yaml'],
              binaryDataKeys: ['logo.png'],
              immutable: true,
              metadataOnly: true,
            },
          }),
          makeResource({
            id: 'api-secret',
            type: 'k8s-secret',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'Secret',
              secretType: 'Opaque',
              dataKeys: ['db-password'],
              immutable: true,
              metadataOnly: true,
            },
          }),
          makeResource({
            id: 'checkout',
            type: 'k8s-serviceaccount',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'ServiceAccount',
              automountServiceAccountToken: false,
              secretCount: 2,
              imagePullSecrets: ['registry-pull'],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No config"
        emptyDescription="No config"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Lifecycle / trust')).toBeInTheDocument();
    expect(screen.getByText('Data shape')).toBeInTheDocument();
    expect(screen.getByText('Token refs')).toBeInTheDocument();
    expect(screen.getByText('Terminating')).toBeInTheDocument();
    expect(screen.getByText('Metadata-only · Immutable')).toBeInTheDocument();
    expect(screen.getByText('Metadata-only · Opaque · Immutable')).toBeInTheDocument();
    expect(screen.getAllByText('Payload omitted')).toHaveLength(2);
    expect(screen.getByText('No auto token')).toBeInTheDocument();
    expect(screen.getByText('2 secrets · pull: registry-pull')).toBeInTheDocument();
    expect(document.body.textContent).not.toContain('app.yaml');
    expect(document.body.textContent).not.toContain('logo.png');
    expect(document.body.textContent).not.toContain('db-password');
    expect(document.body.textContent).not.toContain('Mutable');
  });
});
