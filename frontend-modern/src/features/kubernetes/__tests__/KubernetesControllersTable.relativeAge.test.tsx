import { cleanup, render, screen } from '@solidjs/testing-library';
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

import type { Resource } from '@/types/resource';
import { RELATIVE_TIME_TICK_MS } from '@/utils/relativeTimeClock';
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
  vi.useRealTimers();
});

describe('KubernetesControllersTable relative ages', () => {
  it('keeps Job and CronJob detail ages moving while the rows stay mounted', () => {
    vi.useFakeTimers({ now: new Date('2026-05-24T13:31:00Z') });

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
              succeeded: 1,
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
              schedule: '0 * * * *',
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

    expect(screen.getByText('Completed 31m ago')).toBeInTheDocument();
    expect(screen.getByText('Last success 36m ago')).toBeInTheDocument();

    // No data changes: the timestamps stay put and only the clock moves. A
    // detail computed once at render would still read "31m ago" here.
    vi.setSystemTime(Date.parse('2026-05-24T16:31:00Z'));
    vi.advanceTimersByTime(RELATIVE_TIME_TICK_MS);

    expect(screen.getByText('Completed 3h ago')).toBeInTheDocument();
    expect(screen.getByText('Last success 3h ago')).toBeInTheDocument();
    expect(screen.queryByText('Completed 31m ago')).not.toBeInTheDocument();
  });
});
