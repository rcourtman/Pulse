import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';

import type { Resource } from '@/types/resource';
import { KubernetesClustersTable } from '@/features/kubernetes/KubernetesClustersTable';
import { KubernetesNodesTable } from '@/features/kubernetes/KubernetesNodesTable';
import { TrueNASSystemsTable } from '@/features/truenas/TrueNASSystemsTable';
import { VsphereHostsTable } from '@/features/vmware/VsphereHostsTable';

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: (props: {
    resource: { id: string };
    presentation?: string;
    onClose?: () => void;
  }) => (
    <div
      data-testid="resource-detail-drawer"
      data-resource-id={props.resource.id}
      data-presentation={props.presentation ?? 'full'}
    >
      <button type="button" aria-label="Close resource drawer" onClick={() => props.onClose?.()} />
    </div>
  ),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="responsive-metric-cell" />,
}));

vi.mock('@/components/Workloads/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar" />,
}));

const makeResource = ({
  id,
  type,
  ...overrides
}: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  id,
  type,
  name: id,
  displayName: id,
  platformId: 'homelab',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  cpu: { current: 42 },
  memory: { total: 32_000, used: 16_000, free: 16_000, current: 50 },
  disk: { total: 80_000, used: 40_000, free: 40_000, current: 50 },
  ...overrides,
});

const expectRowOpensResourceDrawer = async (row: HTMLTableRowElement, resourceId: string) => {
  expect(row).toHaveAttribute('aria-expanded', 'false');
  expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();

  await fireEvent.click(row);

  expect(row).toHaveAttribute('aria-expanded', 'true');
  expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
    'data-resource-id',
    resourceId,
  );
  expect(screen.getByTestId('resource-detail-drawer')).toHaveAttribute(
    'data-presentation',
    'table-row',
  );

  await fireEvent.click(screen.getByRole('button', { name: 'Close resource drawer' }));

  expect(row).toHaveAttribute('aria-expanded', 'false');
  expect(screen.queryByTestId('resource-detail-drawer')).not.toBeInTheDocument();
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('platform resource table drawers', () => {
  it('opens canonical resource details from TrueNAS system rows', async () => {
    const system = makeResource({
      id: 'agent:truenas-main',
      type: 'agent',
      name: 'truenas-main',
      platformType: 'truenas',
    });

    render(() => (
      <TrueNASSystemsTable
        systems={[system]}
        scope={[system]}
        emptyIcon={<span />}
        emptyTitle="No systems"
        emptyDescription="No systems"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('truenas-main').closest('tr');
    expect(row).toBeTruthy();
    await expectRowOpensResourceDrawer(row!, system.id);
  });

  it('opens canonical resource details from vSphere host rows', async () => {
    const host = makeResource({
      id: 'vmware-host:esxi-01',
      type: 'agent',
      name: 'esxi-01',
      platformType: 'vmware-vsphere',
      vmware: {
        managedObjectId: 'host-12',
        datacenterName: 'DC1',
        clusterName: 'Cluster A',
        powerState: 'POWERED_ON',
        vcenterHost: 'vcenter.local',
      },
    });
    const vm = makeResource({
      id: 'vmware-vm:app-01',
      type: 'vm',
      name: 'app-01',
      platformType: 'vmware-vsphere',
      vmware: {
        runtimeHostId: 'host-12',
      },
    });

    render(() => (
      <VsphereHostsTable
        hosts={[host]}
        scope={[host, vm]}
        emptyIcon={<span />}
        emptyTitle="No hosts"
        emptyDescription="No hosts"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('esxi-01').closest('tr');
    expect(row).toBeTruthy();
    await expectRowOpensResourceDrawer(row!, host.id);
  });

  it('opens canonical resource details from Kubernetes node rows', async () => {
    const node = makeResource({
      id: 'k8s-node:worker-01',
      type: 'k8s-node',
      name: 'worker-01',
      platformType: 'kubernetes',
      sourceType: 'hybrid',
      kubernetes: {
        clusterId: 'prod-west',
        clusterName: 'prod-west',
        nodeName: 'worker-01',
        kubeletVersion: 'v1.31.3',
        containerRuntimeVersion: 'containerd://1.7.22',
      },
    });

    render(() => (
      <KubernetesNodesTable
        resources={[node]}
        emptyIcon={<span />}
        emptyTitle="No nodes"
        emptyDescription="No nodes"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('worker-01').closest('tr');
    expect(row).toBeTruthy();
    await expectRowOpensResourceDrawer(row!, node.id);
  });

  it('opens canonical resource details from Kubernetes cluster rows', async () => {
    const cluster = makeResource({
      id: 'k8s-cluster:prod-west',
      type: 'k8s-cluster',
      name: 'prod-west',
      platformType: 'kubernetes',
      sourceType: 'hybrid',
      kubernetes: {
        clusterId: 'prod-west',
        clusterName: 'prod-west',
        context: 'prod-west-admin',
        version: 'v1.31.3',
      },
    });

    render(() => (
      <KubernetesClustersTable
        clusters={[cluster]}
        scope={[cluster]}
        emptyIcon={<span />}
        emptyTitle="No clusters"
        emptyDescription="No clusters"
        showToolbar={false}
      />
    ));

    const row = screen.getByText('prod-west').closest('tr');
    expect(row).toBeTruthy();
    await expectRowOpensResourceDrawer(row!, cluster.id);
  });
});
