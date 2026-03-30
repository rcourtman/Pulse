import { fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { describe, expect, it, vi } from 'vitest';
import infrastructureSelectorSource from '@/components/shared/InfrastructureSelector.tsx?raw';
import infrastructureSelectorModelSource from '@/components/shared/infrastructureSelectorModel.ts?raw';
import infrastructureSelectorStateSource from '@/components/shared/useInfrastructureSelectorState.ts?raw';
import { InfrastructureSelector } from '@/components/shared/InfrastructureSelector';
import { buildInfrastructureSelectorAgents } from '@/components/shared/infrastructureSelectorModel';
import type { Resource } from '@/types/resource';

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

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => [
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
  }),
}));

vi.mock('@/hooks/useRecoveryRollups', () => ({
  useRecoveryRollups: () => ({
    rollups: () => [],
  }),
}));

describe('InfrastructureSelector', () => {
  it('keeps the selector on shell, runtime, and model owners', () => {
    expect(infrastructureSelectorSource).toContain('useInfrastructureSelectorState');
    expect(infrastructureSelectorSource).toContain('InfrastructureSummaryTable');
    expect(infrastructureSelectorSource).not.toContain('useResources');
    expect(infrastructureSelectorSource).not.toContain('createSignal');
    expect(infrastructureSelectorSource).not.toContain("resource.type === 'truenas'");

    expect(infrastructureSelectorStateSource).toContain('useResources');
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
        lastSeen: Date.now(),
        policy: {
          display: {
            mode: 'governed',
            summary: 'backup server resource; status online; sources pbs',
          },
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
    ] as Resource[];

    expect(buildInfrastructureSelectorAgents(resources)).toEqual([
      expect.objectContaining({
        id: 'pbs-sensitive',
        displayName: 'PBS Main',
      }),
    ]);
  });
});
