import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { createStore, reconcile } from 'solid-js/store';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import { KubernetesNodesTable } from '../KubernetesNodesTable';

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

// The websocket's activeAlerts is a Solid store, so rows must follow it live.
const activeAlertsRef = vi.hoisted(() => ({ current: {} as Record<string, Alert> }));

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({ activeAlerts: activeAlertsRef.current }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    detectionEnabled: () => true,
    getMetricThresholds: () => ({ warning: 80, critical: 85 }),
  }),
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

const [activeAlerts, setActiveAlerts] = createStore<Record<string, Alert>>({});
activeAlertsRef.current = activeAlerts;

afterEach(() => {
  cleanup();
  setActiveAlerts(reconcile({}));
  vi.clearAllMocks();
});

describe('KubernetesNodesTable', () => {
  it('keeps identity near one-third width while exposing cluster and capacity on phones', () => {
    const { container } = render(() => (
      <KubernetesNodesTable
        resources={[makeNodeResource()]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    const headers = [...container.querySelectorAll('thead th')];
    expect(headers.find((header) => header.textContent?.includes('Node'))).toHaveClass(
      'platform-table-mobile-w-30',
    );
    expect(headers.find((header) => header.textContent?.includes('Cluster'))).toHaveClass(
      'platform-table-mobile-w-15',
    );
    expect(headers.find((header) => header.textContent?.includes('Capacity'))).toHaveClass(
      'platform-table-mobile-w-10',
    );
    expect(headers.find((header) => header.textContent?.includes('Role'))).toHaveClass(
      'platform-table-phone-hidden',
    );
    // The dot carries the state on phones, so the Status column is desktop-only.
    expect(headers.find((header) => header.textContent?.includes('Status'))).toHaveClass(
      'platform-table-phone-hidden',
    );
  });

  it('says why a node is not Ready in its Status column and to assistive tech', () => {
    render(() => (
      <KubernetesNodesTable
        resources={[
          makeNodeResource({ kubernetes: { ...makeNodeResource().kubernetes, ready: false } }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    // Desktop: the Status column says it. Phones: the column is demoted and a
    // phone-only screen-reader label says it, so it is never announced twice.
    const labels = screen.getAllByText('NotReady');
    expect(labels).toHaveLength(2);
    expect(labels.some((label) => label.closest('.platform-table-phone-hidden'))).toBe(true);
    expect(
      labels.some((label) => label.classList.contains('platform-table-phone-only-inline')),
    ).toBe(true);
    expect(screen.queryByLabelText('NotReady')).toBeNull();
  });

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
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    fireEvent.click(launchLink);
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
  it('tints a node row for the open alerts its drawer lists and leads with it', () => {
    const agentAlert = (id: string, resourceId: string, overrides: Partial<Alert> = {}): Alert => ({
      id,
      type: 'memory',
      level: 'critical',
      resourceId,
      resourceName: 'Prod Euw1 K8s 02',
      node: 'prod-euw1-k8s-02',
      instance: '',
      message: 'Agent memory at 100.0%',
      value: 100,
      threshold: 90,
      startTime: '2026-10-06T10:00:00Z',
      acknowledged: false,
      ...overrides,
    });
    // A node that runs a Pulse agent is an agent row; its alerts are keyed
    // "agent:<agentId>" and carry the hostname, never the row id or name.
    const agentNode = (id: string, name: string, agentId: string, ready = true) =>
      makeNodeResource({
        id,
        name,
        displayName: name,
        type: 'agent',
        agent: { agentId },
        kubernetes: { ...makeNodeResource().kubernetes, nodeName: id, ready },
      } as Partial<Resource>);
    setActiveAlerts({
      memory: agentAlert('memory', 'agent:host-k8s-node-2'),
    });

    const { container } = render(() => (
      <KubernetesNodesTable
        resources={[
          agentNode('node-1', 'Prod Euw1 K8s 01', 'host-k8s-node-1'),
          agentNode('node-2', 'Prod Euw1 K8s 02', 'host-k8s-node-2'),
          makeNodeResource({
            id: 'node-3',
            name: 'Prod Euw1 K8s 03',
            kubernetes: { ...makeNodeResource().kubernetes, unschedulable: true },
          }),
        ]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    const row = (id: string) =>
      container.querySelector(`[data-kubernetes-node-row="${id}"]`) as HTMLElement;
    const order = () =>
      [...container.querySelectorAll('[data-kubernetes-node-row]')].map((element) =>
        element.getAttribute('data-kubernetes-node-row'),
      );
    expect(row('node-2')).toHaveClass('bg-red-50');
    expect(row('node-1')).not.toHaveClass('bg-red-50');
    // A Ready node with a critical alert leads, ahead of a cordoned node.
    expect(order()).toEqual(['node-2', 'node-3', 'node-1']);

    // The tint and the order follow the live alert store: acknowledging
    // clears both, and a new warning tints the row amber.
    setActiveAlerts('memory', 'acknowledged', true);
    expect(row('node-2')).not.toHaveClass('bg-red-50');
    expect(order()).toEqual(['node-3', 'node-1', 'node-2']);
    setActiveAlerts('memory', { acknowledged: false, level: 'warning' });
    expect(row('node-2')).toHaveClass('bg-yellow-50');
    // An expanded row drops the tint while its drawer lists the alert.
    fireEvent.click(row('node-2'));
    expect(row('node-2')).not.toHaveClass('bg-yellow-50');
    fireEvent.click(row('node-2'));
    expect(row('node-2')).toHaveClass('bg-yellow-50');
    setActiveAlerts(reconcile({}));
    expect(row('node-2')).not.toHaveClass('bg-yellow-50');
  });
});
