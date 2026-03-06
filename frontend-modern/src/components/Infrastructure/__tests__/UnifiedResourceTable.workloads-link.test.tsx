import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, within } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';

const responsiveMetricCellMock = vi.fn();

if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
    isVisible: () => true,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: (props: unknown) => {
    responsiveMetricCellMock(props);
    return <div data-testid="metric-cell">metric</div>;
  },
}));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="resource-drawer">drawer</div>,
}));

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'agent',
  name: 'pve1',
  displayName: 'pve1',
  platformId: 'pve1',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['proxmox'] },
  ...overrides,
});

describe('UnifiedResourceTable workloads links', () => {
  it('passes canonical metrics keys to metric cells for docker-host resources', async () => {
    responsiveMetricCellMock.mockClear();

    render(() => (
      <UnifiedResourceTable
        resources={[
          baseResource({
            id: 'hash-docker-resource',
            type: 'docker-host',
            name: 'Tower',
            displayName: 'Tower',
            platformType: 'docker',
            sourceType: 'agent',
            metricsTarget: {
              resourceType: 'docker-host',
              resourceId: 'docker-host-1',
            },
            platformData: {
              sources: ['docker'],
              docker: {
                hostSourceId: 'docker-host-1',
              },
            },
            cpu: { current: 0.21 },
            memory: { current: 0.52, total: 8, used: 4 },
            disk: { current: 0.33 },
          }),
        ]}
        expandedResourceId={null}
        onExpandedResourceChange={vi.fn()}
        groupingMode="flat"
      />
    ));

    expect(responsiveMetricCellMock).toHaveBeenCalled();
    expect(
      responsiveMetricCellMock.mock.calls.some(
        ([props]) => (props as { resourceId?: string }).resourceId === 'dockerHost:docker-host-1',
      ),
    ).toBe(true);
  });

  it('renders workloads links for supported resource types and prevents row toggle on link click', async () => {
    const onExpandedResourceChange = vi.fn();
    const resources: Resource[] = [
      baseResource({
        id: 'node-1',
        type: 'agent',
        platformData: {
          sources: ['proxmox'],
          proxmox: { nodeName: 'pve1' },
        },
      }),
      baseResource({
        id: 'k8s-cluster-1',
        type: 'k8s-cluster',
        name: 'cluster-a',
        displayName: 'cluster-a',
        clusterId: 'cluster-a',
        platformType: 'kubernetes',
        sourceType: 'api',
        platformData: {
          sources: ['kubernetes'],
          kubernetes: {
            clusterName: 'cluster-a',
          },
        },
      }),
      baseResource({
        id: 'pbs-1',
        type: 'pbs',
        name: 'pbs-main',
        displayName: 'pbs-main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        platformData: {
          sources: ['pbs'],
        },
      }),
    ];

    const { getAllByRole } = render(() => (
      <UnifiedResourceTable
        resources={resources}
        expandedResourceId={null}
        onExpandedResourceChange={onExpandedResourceChange}
        groupingMode="flat"
      />
    ));

    const links = getAllByRole('link', { name: /view workloads/i });
    expect(links).toHaveLength(2);
    const hrefs = links
      .map((link) => link.getAttribute('href'))
      .filter((href): href is string => typeof href === 'string');
    expect(hrefs).toContain('/workloads?agent=pve1');
    expect(hrefs).toContain('/workloads?type=pod&context=cluster-a');

    const hostLink = links.find((link) => link.getAttribute('href') === '/workloads?agent=pve1');
    expect(hostLink).toBeDefined();
    hostLink!.addEventListener('click', (event) => event.preventDefault());
    await fireEvent.click(hostLink!);
    expect(onExpandedResourceChange).not.toHaveBeenCalled();
  });

  it('renders PBS and PMG resources in a dedicated service table with service-native columns', async () => {
    const onExpandedResourceChange = vi.fn();
    const resources: Resource[] = [
      baseResource({
        id: 'pbs-1',
        type: 'pbs',
        name: 'pbs-main',
        displayName: 'pbs-main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        platformData: {
          sources: ['pbs'],
          pbs: {
            datastoreCount: 2,
            backupJobCount: 1,
          },
        },
      }),
      baseResource({
        id: 'pmg-1',
        type: 'pmg',
        name: 'pmg-main',
        displayName: 'pmg-main',
        platformType: 'proxmox-pmg',
        sourceType: 'api',
        platformData: {
          sources: ['pmg'],
          pmg: {
            queueTotal: 519,
            nodeCount: 1,
          },
        },
      }),
    ];

    const { getByText, getByRole, getAllByText } = render(() => (
      <UnifiedResourceTable
        resources={resources}
        expandedResourceId={null}
        onExpandedResourceChange={onExpandedResourceChange}
        groupingMode="flat"
      />
    ));

    expect(getByText('Service Infrastructure')).toBeInTheDocument();
    expect(getByText('PBS Services')).toBeInTheDocument();
    expect(getByText('PMG Services')).toBeInTheDocument();
    expect(getByText('Datastores')).toBeInTheDocument();
    expect(getByText('Jobs')).toBeInTheDocument();
    expect(getAllByText('Action').length).toBeGreaterThan(0);
    expect(getByText('Queue')).toBeInTheDocument();
    expect(getByText('Deferred')).toBeInTheDocument();
    expect(getByText('Hold')).toBeInTheDocument();
    expect(getByText('Nodes')).toBeInTheDocument();

    const pbsRow = getByText('pbs-main').closest('tr');
    expect(pbsRow).toBeTruthy();
    if (pbsRow) {
      const row = within(pbsRow);
      expect(row.getByText('2')).toBeInTheDocument();
      expect(row.getByText('1')).toBeInTheDocument();
    }

    const pmgRow = getByText('pmg-main').closest('tr');
    expect(pmgRow).toBeTruthy();
    if (pmgRow) {
      const row = within(pmgRow);
      expect(row.getByText('519')).toBeInTheDocument();
      expect(row.getByText('1')).toBeInTheDocument();
    }

    const pbsLink = getByRole('link', { name: /open pbs backups/i });
    expect(pbsLink).toHaveTextContent('Recovery');
    expect(pbsLink).toHaveAttribute('href', '/recovery?provider=proxmox-pbs&mode=remote');
    const pmgLink = getByRole('link', { name: /open pmg thresholds/i });
    expect(pmgLink).toHaveTextContent('Thresholds');
    expect(pmgLink).toHaveAttribute('href', '/alerts/thresholds/mail-gateway');
    pbsLink.addEventListener('click', (event) => event.preventDefault());
    await fireEvent.click(pbsLink);
    expect(onExpandedResourceChange).not.toHaveBeenCalled();
  });
});
