import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesAlertsTable } from '../KubernetesAlertsTable';
import { buildKubernetesIncidentRows } from '../kubernetesPageModel';

const makeResource = ({
  id,
  type = 'pod',
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id'>): Resource => ({
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

describe('KubernetesAlertsTable', () => {
  it('renders incident rows critical → warning → info with severity-coloured dots', () => {
    const incidents = buildKubernetesIncidentRows([
      makeResource({
        id: 'pod-payments',
        type: 'pod',
        name: 'payments-worker-abc',
        kubernetes: {
          clusterName: 'prod-eu',
          namespace: 'services',
          ownerKind: 'Deployment',
          ownerName: 'payments-worker',
        },
        incidents: [
          { code: 'k8s_pod_crashloop', severity: 'critical', summary: 'CrashLoopBackOff' },
        ],
      }),
      makeResource({
        id: 'dep-checkout',
        type: 'k8s-deployment',
        name: 'checkout-api',
        kubernetes: { clusterName: 'prod-eu', namespace: 'apps' },
        incidents: [
          {
            code: 'k8s_deployment_under_replicated',
            severity: 'warning',
            summary: '1 / 3 ready',
          },
        ],
      }),
      makeResource({
        id: 'node-stale',
        type: 'k8s-node',
        name: 'edge-pop-lax-01',
        kubernetes: { clusterName: 'edge' },
        incidents: [{ code: 'k8s_node_version_drift', severity: 'info', summary: 'kubelet drift' }],
      }),
    ]);

    render(() => (
      <KubernetesAlertsTable
        incidents={incidents}
        emptyIcon={<span />}
        emptyTitle="No K8s alerts"
        emptyDescription="No K8s alerts"
        showToolbar={false}
      />
    ));

    const rows = Array.from(document.querySelectorAll('[data-kubernetes-alert-row]')).map((row) =>
      row.getAttribute('data-kubernetes-alert-row'),
    );
    expect(rows[0]).toContain('pod-payments:incident:k8s_pod_crashloop');
    expect(rows[1]).toContain('dep-checkout:incident:k8s_deployment_under_replicated');
    expect(rows[2]).toContain('node-stale:incident:k8s_node_version_drift');

    expect(screen.getAllByTitle('Critical')[0]).toHaveClass('bg-red-500');
    expect(screen.getAllByTitle('Warning')[0]).toHaveClass('bg-amber-500');
    expect(screen.getAllByTitle('Info')[0]).toHaveClass('bg-slate-400');

    expect(screen.getByText('CrashLoopBackOff')).toBeInTheDocument();
    expect(screen.getByText('1 / 3 ready')).toBeInTheDocument();
    expect(screen.getByText('prod-eu/services')).toBeInTheDocument();
  });

  it('renders the empty-state fallback when given no incidents', () => {
    render(() => (
      <KubernetesAlertsTable
        incidents={[]}
        emptyIcon={<span data-testid="empty-icon" />}
        emptyTitle="No K8s alerts"
        emptyDescription="Active alerts appear here."
        showToolbar={false}
      />
    ));
    expect(screen.getByText('No K8s alerts')).toBeInTheDocument();
    expect(document.querySelector('[data-kubernetes-alert-row]')).toBeNull();
  });
});
