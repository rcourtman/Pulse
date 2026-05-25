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

  it('renders RBAC Role / ClusterRole / RoleBinding / ClusterRoleBinding inventory', () => {
    render(() => (
      <KubernetesConfigTable
        resources={[
          makeResource({
            id: 'role-checkout',
            type: 'k8s-role',
            name: 'checkout-api-runtime',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'Role',
              ruleCount: 4,
            },
          }),
          makeResource({
            id: 'clusterrole-mon',
            type: 'k8s-cluster-role',
            name: 'pulse-demo-monitoring',
            kubernetes: {
              clusterId: 'cluster-1',
              resourceKind: 'ClusterRole',
              ruleCount: 12,
              aggregationLabels: { 'rbac.authorization.k8s.io/aggregate-to-admin': 'true' },
            },
          }),
          makeResource({
            id: 'rb-checkout',
            type: 'k8s-role-binding',
            name: 'checkout-api-runtime',
            kubernetes: {
              clusterId: 'cluster-1',
              namespace: 'apps',
              resourceKind: 'RoleBinding',
              roleKind: 'Role',
              roleName: 'checkout-api-runtime',
              subjectCount: 2,
              subjectKinds: ['Group', 'ServiceAccount'],
            },
          }),
          makeResource({
            id: 'crb-mon',
            type: 'k8s-cluster-role-binding',
            name: 'pulse-demo-monitoring',
            kubernetes: {
              clusterId: 'cluster-1',
              resourceKind: 'ClusterRoleBinding',
              roleKind: 'ClusterRole',
              roleName: 'pulse-demo-monitoring',
              subjectCount: 3,
              subjectKinds: ['Group', 'ServiceAccount', 'User'],
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No RBAC"
        emptyDescription="No RBAC"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Role')).toBeInTheDocument();
    expect(screen.getByText('ClusterRole')).toBeInTheDocument();
    expect(screen.getByText('RoleBinding')).toBeInTheDocument();
    expect(screen.getByText('ClusterRoleBinding')).toBeInTheDocument();
    expect(screen.getByText('4 rules')).toBeInTheDocument();
    expect(screen.getByText('12 rules · Aggregated')).toBeInTheDocument();
    expect(screen.getByText('Role/checkout-api-runtime')).toBeInTheDocument();
    expect(screen.getByText('ClusterRole/pulse-demo-monitoring')).toBeInTheDocument();
    expect(screen.getByText('2 subjects · Group, ServiceAccount')).toBeInTheDocument();
    expect(screen.getByText('3 subjects · Group, ServiceAccount +1')).toBeInTheDocument();
  });
});
