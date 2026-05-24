import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesControllersTable } from '../KubernetesControllersTable';

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

describe('KubernetesControllersTable', () => {
  it('renders native controller fields for ReplicaSet, StatefulSet, DaemonSet, Job, and CronJob rows', () => {
    render(() => (
      <KubernetesControllersTable
        resources={[
          makeResource({
            id: 'checkout-api-replicaset',
            type: 'k8s-replicaset',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'ReplicaSet',
              desiredReplicas: 4,
              currentReplicas: 4,
              readyReplicas: 3,
              availableReplicas: 3,
              fullyLabeledReplicas: 4,
            },
          }),
          makeResource({
            id: 'checkout-api-stateful',
            type: 'k8s-statefulset',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'apps',
              resourceKind: 'StatefulSet',
              desiredReplicas: 3,
              currentReplicas: 3,
              readyReplicas: 2,
              availableReplicas: 2,
              serviceName: 'checkout-headless',
            },
          }),
          makeResource({
            id: 'node-exporter',
            type: 'k8s-daemonset',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'observability',
              resourceKind: 'DaemonSet',
              desiredNumberScheduled: 6,
              currentNumberScheduled: 6,
              numberReady: 5,
              numberAvailable: 5,
              numberUnavailable: 1,
              numberMisscheduled: 1,
              updatedReplicas: 5,
            },
          }),
          makeResource({
            id: 'nightly-import',
            type: 'k8s-job',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'batch',
              resourceKind: 'Job',
              desiredReplicas: 10,
              active: 1,
              succeeded: 8,
              failed: 2,
              completionTime: '2026-05-24T13:00:00Z',
            },
          }),
          makeResource({
            id: 'billing-rollup',
            type: 'k8s-cronjob',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'batch',
              resourceKind: 'CronJob',
              schedule: '*/5 * * * *',
              active: 2,
              suspend: true,
              lastSuccessfulTime: '2026-05-24T12:55:00Z',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No controllers"
        emptyDescription="No controllers"
        showToolbar={false}
      />
    ));

    expect(screen.getByText('Controller')).toBeInTheDocument();
    expect(screen.getByText('Target')).toBeInTheDocument();
    expect(screen.getByText('Ready/Done')).toBeInTheDocument();
    expect(screen.getByText('Exceptions')).toBeInTheDocument();
    expect(screen.getByText('Detail')).toBeInTheDocument();

    expect(screen.getByText('ReplicaSet')).toBeInTheDocument();
    expect(screen.getByText('4 pods')).toBeInTheDocument();
    expect(screen.getAllByText('1 not ready')).toHaveLength(2);
    expect(screen.getByText('Fully labeled: 4')).toBeInTheDocument();

    expect(screen.getByText('StatefulSet')).toBeInTheDocument();
    expect(screen.getByText('3 pods')).toBeInTheDocument();
    expect(screen.getByText('Service: checkout-headless')).toBeInTheDocument();

    expect(screen.getByText('DaemonSet')).toBeInTheDocument();
    expect(screen.getByText('6 nodes')).toBeInTheDocument();
    expect(screen.getByText('Unavailable: 1 / Misscheduled: 1')).toBeInTheDocument();
    expect(screen.getByText('Updated: 5')).toBeInTheDocument();

    expect(screen.getByText('Job')).toBeInTheDocument();
    expect(screen.getByText('10 completions')).toBeInTheDocument();
    expect(screen.getByText('Failed: 2')).toBeInTheDocument();
    expect(screen.getByText('Completed: 2026-05-24T13:00:00Z')).toBeInTheDocument();

    expect(screen.getByText('CronJob')).toBeInTheDocument();
    expect(screen.getByText('*/5 * * * *')).toBeInTheDocument();
    expect(screen.getByText('Suspended')).toBeInTheDocument();
    expect(screen.getByText('Last success: 2026-05-24T12:55:00Z')).toBeInTheDocument();
  });
});
