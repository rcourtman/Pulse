import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import infrastructureSelectorModelSource from '@/components/shared/infrastructureSelectorModel.ts?raw';
import infrastructureSelectorStateSource from '@/components/shared/useInfrastructureSelectorState.ts?raw';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { buildInfrastructureSelectorAgents } from '@/components/shared/infrastructureSelectorModel';
import type { Resource } from '@/types/resource';

const useUnifiedResourcesMock = vi.fn(
  (options?: { enabled?: () => boolean }) => ({
    resources: () =>
      options?.enabled?.() === false
        ? []
        : [
            {
              id: 'node-1',
              type: 'agent',
              name: 'pve1',
              status: 'online',
              instance: 'main',
              uptime: 120,
              platformData: {
                type: 'node',
                node: 'pve1',
                instance: 'main',
                pveVersion: '8.0',
                cpu: 0.2,
                memory: { total: 8, used: 4, free: 4, usage: 50 },
                disk: { total: 100, used: 25, free: 75, usage: 25 },
                loadAverage: [0.1, 0.2, 0.3],
              },
              memory: { total: 8, used: 4, free: 4, current: 50 },
              agent: {
                platform: 'linux',
                memory: { total: 8, used: 4, free: 4, usage: 50 },
                disks: [],
                networkInterfaces: [],
                raid: [],
              },
            },
          ],
    refetch: vi.fn(),
    mutate: vi.fn(),
    loading: () => false,
    error: () => null,
  }),
);

const useRecoveryRollupsMock = vi.fn(
  (_query?: (() => unknown) | undefined) => ({
    rollups: () => [],
  }),
);

const infrastructureSummaryTableMock = vi.fn(
  (props: { onNodeClick: (nodeId: string, nodeType: 'pve' | 'pbs') => void; selectedNode: string | null }) => (
    <button
      type="button"
      data-testid="infrastructure-selector-table"
      onClick={() => props.onNodeClick('node-1', 'pve')}
    >
      {props.selectedNode ?? 'none'}
    </button>
  ),
);

vi.mock('@/components/shared/InfrastructureSummaryTable', () => ({
  InfrastructureSummaryTable: (props: {
    onNodeClick: (nodeId: string, nodeType: 'pve' | 'pbs') => void;
    selectedNode: string | null;
  }) => infrastructureSummaryTableMock(props),
}));

vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: (options?: { enabled?: () => boolean }) => useUnifiedResourcesMock(options),
}));

vi.mock('@/hooks/useRecoveryRollups', () => ({
  useRecoveryRollups: (query?: (() => unknown) | undefined) => useRecoveryRollupsMock(query),
}));

describe('InfrastructureSelector', () => {
  it('keeps the selector on shell, runtime, and model owners', () => {
    expect(infrastructureSelectorSource).toContain('useInfrastructureSelectorState');
    expect(infrastructureSelectorSource).toContain('InfrastructureSummaryTable');
    expect(infrastructureSelectorSource).not.toContain('useResources');
    expect(infrastructureSelectorSource).not.toContain('createSignal');
    expect(infrastructureSelectorSource).not.toContain("resource.type === 'truenas'");

    expect(infrastructureSelectorStateSource).toContain('useUnifiedResources');
    expect(infrastructureSelectorStateSource).toContain('enabled: showNodeSummary');
    expect(infrastructureSelectorStateSource).toContain('useRecoveryRollups');
    expect(infrastructureSelectorStateSource).toContain('createSignal');
    expect(infrastructureSelectorStateSource).toContain('document.addEventListener');
    expect(infrastructureSelectorStateSource).toContain(
      'export function useInfrastructureSelectorState',
    );

    expect(infrastructureSelectorModelSource).toContain(
      'buildInfrastructureSelectorAgents',
    );
    expect(infrastructureSelectorModelSource).toContain(
      'buildInfrastructureSelectorBackupCounts',
    );
    expect(infrastructureSelectorModelSource).toContain(
      'buildInfrastructureSelectorUnifiedNodes',
    );
    expect(infrastructureSelectorModelSource).toContain('isAgentFacetInfrastructureResource');
  });

  it('disables selector data hooks when the node summary is hidden', () => {
    render(() => <InfrastructureSelector currentTab="dashboard" showNodeSummary={false} />);

    expect(screen.queryByTestId('infrastructure-selector-table')).not.toBeInTheDocument();
    expect(useUnifiedResourcesMock).toHaveBeenCalledTimes(1);
    expect(useRecoveryRollupsMock).toHaveBeenCalledTimes(1);

    const unifiedOptions = useUnifiedResourcesMock.mock.calls[0]?.[0] as
      | { enabled?: () => boolean; query?: string; cacheKey?: string }
      | undefined;
    expect(unifiedOptions?.query).toBe('');
    expect(unifiedOptions?.cacheKey).toBe('all-resources');
    expect(unifiedOptions?.enabled?.()).toBe(false);

    const recoveryQuery = useRecoveryRollupsMock.mock.calls[0]?.[0] as
      | (() => unknown)
      | undefined;
    expect(recoveryQuery?.()).toBeNull();
  });

  it('toggles node selection and clears it on escape', async () => {
    const onNodeSelect = vi.fn();

    render(() => (
      <InfrastructureSelector currentTab="dashboard" showNodeSummary onNodeSelect={onNodeSelect} />
    ));

    const table = screen.getByTestId('infrastructure-selector-table');
    expect(table).toHaveTextContent('none');

    fireEvent.click(table);
    expect(onNodeSelect).toHaveBeenLastCalledWith('node-1', 'pve');
    await waitFor(() => expect(table).toHaveTextContent('node-1'));

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(onNodeSelect).toHaveBeenLastCalledWith(null, null);
    await waitFor(() => expect(table).toHaveTextContent('none'));
  });

  it('keeps shared selector agent labels on canonical local infrastructure identity', () => {
    const resources = [
      {
        id: 'pbs-sensitive',
        type: 'pbs',
        name: 'redacted-pbs',
        displayName: 'PBS Main',
        status: 'online',
        platformId: 'pbs-main',
        platformType: 'proxmox-pbs',
        sourceType: 'api',
        lastSeen: Date.now(),
        policy: {
          sensitivity: 'restricted',
          routing: { scope: 'local-only', redact: ['hostname'] },
        },
        platformData: {
          agent: {
            hostname: 'pbs.internal',
            memory: { total: 16, used: 8, free: 8, usage: 50 },
            disks: [],
            networkInterfaces: [],
            raid: [],
          },
        },
        agent: {
          memory: { total: 16, used: 8, free: 8, usage: 50 },
          disks: [],
          networkInterfaces: [],
          raid: [],
        },
      },
    ] as unknown as Resource[];

    expect(buildInfrastructureSelectorAgents(resources)).toEqual([
      expect.objectContaining({
        id: 'pbs-sensitive',
        displayName: 'PBS Main',
      }),
    ]);
  });
});
