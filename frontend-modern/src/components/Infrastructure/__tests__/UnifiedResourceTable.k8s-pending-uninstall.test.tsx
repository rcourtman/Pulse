import { describe, expect, it, vi } from 'vitest';
import { render } from '@solidjs/testing-library';
import type { Resource } from '@/types/resource';
import { UnifiedResourceTable } from '@/components/Infrastructure/UnifiedResourceTable';

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
    isVisible: () => true,
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

describe('UnifiedResourceTable k8s pending uninstall indicator', () => {
  it('adds a pending uninstall suffix for k8s clusters flagged for uninstall', () => {
    const resources: Resource[] = [
      baseResource({
        id: 'k8s-cluster-1',
        type: 'k8s-cluster',
        name: 'cluster-a',
        displayName: 'cluster-a',
        platformType: 'kubernetes',
        platformData: {
          sources: ['kubernetes'],
          kubernetes: {
            pendingUninstall: true,
          },
        },
      }),
    ];

    const { getByText } = render(() => (
      <UnifiedResourceTable
        resources={resources}
        expandedResourceId={null}
        onExpandedResourceChange={() => {}}
        groupingMode="flat"
      />
    ));

    expect(getByText('cluster-a')).toBeInTheDocument();
    expect(getByText('(pending uninstall)')).toBeInTheDocument();
  });
});
