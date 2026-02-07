import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/Infrastructure/ResourceDetailDrawer', () => ({
  ResourceDetailDrawer: () => <div data-testid="resource-drawer">drawer</div>,
}));

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'host',
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
  it('renders workloads links for supported resource types and prevents row toggle on link click', async () => {
    const onExpandedResourceChange = vi.fn();
    const resources: Resource[] = [
      baseResource({
        id: 'node-1',
        type: 'node',
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
    expect(hrefs).toContain('/workloads?host=pve1');
    expect(hrefs).toContain('/workloads?type=k8s&context=cluster-a');

    const hostLink = links.find((link) => link.getAttribute('href') === '/workloads?host=pve1');
    expect(hostLink).toBeDefined();
    hostLink!.addEventListener('click', (event) => event.preventDefault());
    await fireEvent.click(hostLink!);
    expect(onExpandedResourceChange).not.toHaveBeenCalled();
  });
});
