import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesNodesTable } from '../KubernetesNodesTable';

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: [] }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({ detectionEnabled: () => true }),
}));

const makeNodeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'k8s:prod-west:node:worker-01',
  name: 'worker-01',
  displayName: 'worker-01',
  platformId: 'prod-west',
  platformType: 'kubernetes',
  sourceType: 'hybrid',
  status: 'online',
  type: 'k8s-node',
  lastSeen: 1_700_000_000_000,
  uptime: 86_400,
  cpu: { current: 42 },
  memory: { total: 32_000, used: 20_000, free: 12_000, current: 62.5 },
  kubernetes: {
    clusterId: 'prod-west',
    clusterName: 'prod-west',
    nodeName: 'worker-01',
    ready: true,
    roles: ['worker'],
    kubeletVersion: 'v1.31.3',
    containerRuntimeVersion: 'containerd://1.7.20',
    capacityCpuCores: 8,
    capacityMemoryBytes: 32_000,
    capacityPods: 110,
  },
  ...overrides,
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('KubernetesNodesTable', () => {
  it('keeps node identity inert and launches its web interface without expanding the row', () => {
    const node = makeNodeResource({ customUrl: 'https://worker-01.internal' });

    const { container } = render(() => (
      <KubernetesNodesTable
        resources={[node]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    const row = container.querySelector('[data-kubernetes-node-row]') as HTMLElement;
    const launchLink = screen.getByRole('link', {
      name: 'Open web interface for worker-01',
    });

    expect(screen.getByText('worker-01').closest('a')).toBeNull();
    expect(launchLink).toHaveAttribute('href', 'https://worker-01.internal');
    expect(launchLink).toHaveAttribute('target', '_blank');
    expect(launchLink).toHaveAttribute('rel', 'noopener noreferrer');
    expect(row).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(launchLink);
    expect(row).toHaveAttribute('aria-expanded', 'false');
  });
});
