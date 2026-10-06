import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

vi.mock('@/api/resources', () => ({
  ResourceAPI: {
    getFacetBundle: vi.fn().mockResolvedValue({
      capabilities: [],
      relationships: [],
      recentChanges: [],
    }),
  },
}));

import { formatPlatformTableDateTimeValue } from '@/features/platformPage/sharedPlatformPage';
import type { Resource } from '@/types/resource';
import { KubernetesControllersTable } from '../KubernetesControllersTable';

// Generated Kubernetes names render as a truncating head and a kept tail, so
// the full name is the text of the name wrapper rather than of one text node.
const kubernetesName =
  (name: string) =>
  (_content: string, element: Element | null): boolean =>
    element?.hasAttribute('data-kubernetes-name') === true && element.textContent === name;

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
  vi.useRealTimers();
});

const absoluteTime = (timestamp: string): string =>
  formatPlatformTableDateTimeValue(timestamp, { dateTimeFormat: { year: 'numeric' } });

describe('KubernetesControllersTable', () => {
  it('renders native controller fields for ReplicaSet, StatefulSet, DaemonSet, Job, and CronJob rows', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

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
    // Job and CronJob timestamps arrive as RFC 3339 strings; the cell shows
    // how long ago and keeps the absolute time on hover.
    const completed = screen.getByText('Completed 31m ago');
    expect(completed).toHaveAttribute(
      'title',
      `Completed: ${absoluteTime('2026-05-24T13:00:00Z')}`,
    );

    expect(screen.getByText('CronJob')).toBeInTheDocument();
    expect(screen.getByText('*/5 * * * *')).toBeInTheDocument();
    expect(screen.getByText('Suspended')).toBeInTheDocument();
    const lastSuccess = screen.getByText('Last success 36m ago');
    expect(lastSuccess).toHaveAttribute(
      'title',
      `Last success: ${absoluteTime('2026-05-24T12:55:00Z')}`,
    );
    expect(screen.queryByText(/2026-05-24T1[23]/)).not.toBeInTheDocument();
  });

  it('keeps controller, kind, ready and issues on the phone row and moves target to the expansion', () => {
    render(() => (
      <KubernetesControllersTable
        resources={[
          makeResource({
            id: 'node-exporter',
            type: 'k8s-daemonset',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'observability',
              resourceKind: 'DaemonSet',
              desiredNumberScheduled: 6,
              numberReady: 6,
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No controllers"
        emptyDescription="No controllers"
        showToolbar={false}
      />
    ));

    // "DaemonSet" and "StatefulSet" clipped in a 15% kind track at 390px, and
    // "1 completions" clipped in the target track, so the phone row widens kind
    // and issues and demotes target to the row expansion.
    expect(screen.getByText('Kind').closest('th')).toHaveClass('platform-table-mobile-w-25');
    expect(screen.getByText('Target').closest('th')).toHaveClass('platform-table-phone-hidden');
    expect(screen.getByText('6 nodes').closest('td')).toHaveClass('platform-table-phone-hidden');
    expect(screen.getByText('Exceptions').closest('th')).toHaveClass('platform-table-mobile-w-30');
    expect(screen.getByText('Controller').closest('th')).toHaveClass('platform-table-mobile-w-30');
    expect(screen.getByText('Ready/Done').closest('th')).toHaveClass('platform-table-mobile-w-15');
    expect(screen.getByText('Kind').closest('th')).not.toHaveClass('platform-table-phone-hidden');
  });

  it('shows the absolute Job and CronJob times in the row expansion', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

    render(() => (
      <KubernetesControllersTable
        resources={[
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
              startTime: '2026-05-24T12:55:00Z',
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
              lastScheduleTime: '2026-05-24T13:25:00Z',
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

    // Tapping the row opens the expansion; the Detail column is hidden below a
    // large container, so this is where a phone or half-width pane reads the
    // completion time.
    fireEvent.click(screen.getByText(kubernetesName('nightly-import')));
    const job = screen.getByTestId('resource-kubernetes-controller-section');
    expect(within(job).getByText('Started')).toBeInTheDocument();
    expect(
      within(job).getByText(`${absoluteTime('2026-05-24T12:55:00Z')} (36m ago)`),
    ).toBeInTheDocument();
    expect(within(job).getByText('Completed')).toBeInTheDocument();
    expect(
      within(job).getByText(`${absoluteTime('2026-05-24T13:00:00Z')} (31m ago)`),
    ).toBeInTheDocument();
    expect(within(job).getByText('Duration')).toBeInTheDocument();
    expect(within(job).getByText('5m')).toBeInTheDocument();
    expect(within(job).getByText('Target')).toBeInTheDocument();
    expect(within(job).getByText('10 completions')).toBeInTheDocument();
    expect(within(job).getByText('Failed').closest('tr')).toHaveTextContent('2');

    fireEvent.click(screen.getByText(kubernetesName('billing-rollup')));
    const cron = screen.getByTestId('resource-kubernetes-controller-section');
    expect(within(cron).getByText('Schedule')).toBeInTheDocument();
    expect(within(cron).getByText('*/5 * * * *')).toBeInTheDocument();
    expect(within(cron).getByText('Last run')).toBeInTheDocument();
    expect(
      within(cron).getByText(`${absoluteTime('2026-05-24T13:25:00Z')} (6m ago)`),
    ).toBeInTheDocument();
    expect(within(cron).getByText('Last success')).toBeInTheDocument();
    expect(
      within(cron).getByText(`${absoluteTime('2026-05-24T12:55:00Z')} (36m ago)`),
    ).toBeInTheDocument();
    expect(within(cron).getByText('Namespace').closest('tr')).toHaveTextContent('batch');
  });

  it('falls back to start and last-run times when a Job or CronJob has not completed', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-05-24T13:31:00Z'));

    render(() => (
      <KubernetesControllersTable
        resources={[
          makeResource({
            id: 'nightly-import',
            type: 'k8s-job',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'batch',
              resourceKind: 'Job',
              desiredReplicas: 1,
              active: 1,
              startTime: '2026-05-24T13:29:30Z',
            },
          }),
          makeResource({
            id: 'billing-rollup',
            type: 'k8s-cronjob',
            kubernetes: {
              clusterName: 'prod',
              namespace: 'batch',
              resourceKind: 'CronJob',
              schedule: '0 2 * * *',
              lastScheduleTime: '2026-05-22T02:00:00Z',
            },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No controllers"
        emptyDescription="No controllers"
        showToolbar={false}
      />
    ));

    // A single desired completion reads as "1 completion", matching the
    // expansion's Target row.
    expect(screen.getByText('1 completion')).toBeInTheDocument();
    expect(screen.getByText('Started 1m ago')).toHaveAttribute(
      'title',
      `Started: ${absoluteTime('2026-05-24T13:29:30Z')}`,
    );
    expect(screen.getByText('Last run 2d ago')).toHaveAttribute(
      'title',
      `Last run: ${absoluteTime('2026-05-22T02:00:00Z')}`,
    );
  });
});
