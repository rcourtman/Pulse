import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@solidjs/testing-library';
import { InfrastructureSummaryTable } from '@/components/shared/InfrastructureSummaryTable';
import type { Agent, Node } from '@/types/api';
import infrastructureSummaryTableSource from '@/components/shared/InfrastructureSummaryTable.tsx?raw';
import infrastructureSummaryTableRowSource from '@/components/shared/InfrastructureSummaryTableRow.tsx?raw';
import infrastructureSummaryTableModelSource from '@/components/shared/infrastructureSummaryTableModel.ts?raw';
import infrastructureSummaryTableStateSource from '@/components/shared/useInfrastructureSummaryTableState.ts?raw';
import resourceIdentitySource from '@/utils/resourceIdentity.ts?raw';

const enhancedCpuBarMock = vi.fn();
const infrastructureDetailsDrawerMock = vi.fn();

if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    activeAlerts: [],
  }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    activationState: () => 'active',
    getTemperatureThreshold: () => 80,
  }),
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

vi.mock('@/components/shared/responsive', () => ({
  ResponsiveMetricCell: () => <div data-testid="metric-cell">metric</div>,
}));

vi.mock('@/components/shared/TemperatureGauge', () => ({
  TemperatureGauge: () => <div data-testid="temperature-gauge">temp</div>,
}));

vi.mock('@/components/Dashboard/EnhancedCPUBar', () => ({
  EnhancedCPUBar: (props: unknown) => {
    enhancedCpuBarMock(props);
    return <div data-testid="enhanced-cpu-bar">cpu</div>;
  },
}));

vi.mock('@/components/Dashboard/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="stacked-memory-bar">memory</div>,
}));

vi.mock('@/components/Dashboard/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="stacked-disk-bar">disk</div>,
}));

vi.mock('@/components/shared/InfrastructureDetailsDrawer', () => ({
  InfrastructureDetailsDrawer: (props: unknown) => {
    infrastructureDetailsDrawerMock(props);
    return <div data-testid="infra-details-drawer">drawer</div>;
  },
}));

const makeNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'node-1',
  name: 'pve1',
  instance: 'main',
  host: 'https://pve1:8006',
  pveVersion: '8.0',
  status: 'online',
  type: 'node',
  uptime: 123,
  cpu: 0.21,
  memory: { total: 8, used: 4, free: 4, usage: 50 },
  disk: { total: 100, used: 25, free: 75, usage: 25 },
  loadAverage: [0.1, 0.2, 0.3],
  kernelVersion: '6.8.0',
  cpuInfo: { model: 'Test CPU', cores: 8, sockets: 1, mhz: '3200' },
  lastSeen: '2026-01-01T00:00:00Z',
  connectionHealth: 'online',
  linkedAgentId: undefined,
  ...overrides,
});

const makeAgent = (overrides: Partial<Agent> = {}): Agent => ({
  id: 'agent-1',
  hostname: 'pve1.local',
  displayName: 'Agent 1',
  memory: { total: 8, used: 4, free: 4, usage: 50 },
  status: 'online',
  lastSeen: Date.now(),
  ...overrides,
});

describe('InfrastructureSummaryTable', () => {
  it('keeps the shared table on shell, runtime, and model owners', () => {
    expect(infrastructureSummaryTableSource).toContain('useInfrastructureSummaryTableState');
    expect(infrastructureSummaryTableSource).toContain('InfrastructureSummaryTableRow');
    expect(infrastructureSummaryTableSource).not.toContain('useWebSocket');
    expect(infrastructureSummaryTableSource).not.toContain('useAlertsActivation');
    expect(infrastructureSummaryTableStateSource).toContain('useWebSocket');
    expect(infrastructureSummaryTableStateSource).toContain('useAlertsActivation');
    expect(infrastructureSummaryTableModelSource).toContain(
      'resolveInfrastructureSummaryLinkedAgent',
    );
    expect(infrastructureSummaryTableModelSource).toContain('sortInfrastructureSummaryItems');
    expect(infrastructureSummaryTableRowSource).toContain('InfrastructureDetailsDrawer');
    expect(infrastructureSummaryTableRowSource).toContain('getAlertStyles');
  });

  it('keeps pending update badges inside the shared row primitive', () => {
    expect(infrastructureSummaryTableRowSource).toContain('pendingUpdates');
    expect(infrastructureSummaryTableRowSource).toContain('pending apt update');
  });

  it('uses the shared normalized identity lookup helper', () => {
    expect(resourceIdentitySource).toContain('getNormalizedIdentityLookupVariants');
    expect(resourceIdentitySource).not.toContain(
      'const asTrimmedString = (value: unknown): string | undefined => {',
    );
  });

  it('uses linked agent ids for agent-backed node metric keys', () => {
    enhancedCpuBarMock.mockClear();

    render(() => (
      <InfrastructureSummaryTable
        nodes={[makeNode({ linkedAgentId: 'agent-host-1' })]}
        selectedNode={null}
        currentTab="dashboard"
        onNodeClick={vi.fn()}
      />
    ));

    expect(
      enhancedCpuBarMock.mock.calls.some(
        ([props]) => (props as { resourceId?: string }).resourceId === 'agent:agent-host-1',
      ),
    ).toBe(true);
  });

  it('falls back to node metric keys when no linked agent id exists', () => {
    enhancedCpuBarMock.mockClear();

    render(() => (
      <InfrastructureSummaryTable
        nodes={[makeNode({ id: 'node-2', name: 'pve2' })]}
        selectedNode={null}
        currentTab="dashboard"
        onNodeClick={vi.fn()}
      />
    ));

    expect(
      enhancedCpuBarMock.mock.calls.some(
        ([props]) => (props as { resourceId?: string }).resourceId === 'node:node-2',
      ),
    ).toBe(true);
  });

  it('keeps the shared horizontal-scroll wrapper around the table shell', () => {
    const { container } = render(() => (
      <InfrastructureSummaryTable
        nodes={[makeNode()]}
        selectedNode={null}
        currentTab="dashboard"
        onNodeClick={vi.fn()}
      />
    ));

    expect(container.querySelector('div.overflow-x-auto > table')).toBeTruthy();
  });

  it('matches drawer agents by shared linked-agent aliases', () => {
    infrastructureDetailsDrawerMock.mockClear();

    const { container } = render(() => (
      <InfrastructureSummaryTable
        nodes={[makeNode({ linkedAgentId: 'agent-linked' })]}
        agents={[
          makeAgent({
            id: 'agent-explicit',
            linkedNodeId: 'node-1',
            platform: 'linux',
            memory: { total: 16, used: 8, free: 8, usage: 50 },
            platformData: {
              linkedAgentId: 'agent-linked',
              agent: {
                hostname: 'pve1.internal',
              },
            },
          } as Partial<Agent> & { platformData: { linkedAgentId: string; agent: { hostname: string } } }),
        ]}
        selectedNode="node-1"
        currentTab="dashboard"
        onNodeClick={vi.fn()}
      />
    ));

    const expandToggle = container.querySelector('div.cursor-pointer.transition-transform');
    expect(expandToggle).toBeTruthy();
    fireEvent.click(expandToggle!);

    return waitFor(() => {
      expect(infrastructureDetailsDrawerMock).toHaveBeenCalledWith(
        expect.objectContaining({
          agent: expect.objectContaining({ id: 'agent-explicit' }),
        }),
      );
    });
  });
});
