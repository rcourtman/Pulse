import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, waitFor } from '@solidjs/testing-library';
import { InfrastructureSummaryTable } from '@/components/shared/InfrastructureSummaryTable';
import type { Agent, Node } from '@/types/api';

const enhancedCpuBarMock = vi.fn();
const infrastructureDetailsDrawerMock = vi.fn();

if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver;
}

vi.mock('@/App', () => ({
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
  pveVersion: '8.0',
  status: 'online',
  uptime: 123,
  cpu: 0.21,
  memory: { total: 8, used: 4, free: 4, usage: 50 },
  disk: { total: 100, used: 25, free: 75, usage: 25 },
  linkedAgentId: undefined,
  ...overrides,
});

const makeAgent = (overrides: Partial<Agent> = {}): Agent => ({
  id: 'agent-1',
  hostname: 'pve1.local',
  displayName: 'Agent 1',
  status: 'online',
  lastSeen: Date.now(),
  ...overrides,
});

describe('InfrastructureSummaryTable', () => {
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

  it('matches drawer agents by shared linked-agent aliases', () => {
    infrastructureDetailsDrawerMock.mockClear();

    const { container } = render(() => (
      <InfrastructureSummaryTable
        nodes={[makeNode({ linkedAgentId: 'agent-linked' })]}
        agents={[
          makeAgent({
            id: 'agent-explicit',
            platformData: {
              linkedAgentId: 'agent-linked',
              agent: {
                hostname: 'pve1.internal',
              },
            },
          }) as Agent,
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
