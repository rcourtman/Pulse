import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@solidjs/testing-library';
import { InfrastructureDetailsDrawer } from '@/components/shared/InfrastructureDetailsDrawer';
import type { Node, Agent } from '@/types/api';

const discoveryTabMock = vi.fn();
const webInterfaceUrlFieldMock = vi.fn();

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: (props: unknown) => {
    discoveryTabMock(props);
    return <div data-testid="discovery-tab">discovery</div>;
  },
}));

vi.mock('@/components/shared/WebInterfaceUrlField', () => ({
  WebInterfaceUrlField: (props: unknown) => {
    webInterfaceUrlFieldMock(props);
    return <div data-testid="web-interface-url-field">url-field</div>;
  },
}));

vi.mock('@/components/shared/cards/SystemInfoCard', () => ({
  SystemInfoCard: () => <div data-testid="system-info-card">system</div>,
}));

vi.mock('@/components/shared/cards/HardwareCard', () => ({
  HardwareCard: () => <div data-testid="hardware-card">hardware</div>,
}));

vi.mock('@/components/shared/cards/RootDiskCard', () => ({
  RootDiskCard: () => <div data-testid="root-disk-card">disk</div>,
}));

vi.mock('@/components/shared/cards/NetworkInterfacesCard', () => ({
  NetworkInterfacesCard: () => <div data-testid="network-interfaces-card">net</div>,
}));

vi.mock('@/components/shared/cards/DisksCard', () => ({
  DisksCard: () => <div data-testid="disks-card">disks</div>,
}));

const makeNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'node-1',
  name: 'pve1',
  displayName: 'pve1',
  instance: 'cluster-a',
  host: 'pve1',
  status: 'online',
  type: 'node',
  cpu: 0.2,
  memory: { total: 8, used: 4, free: 4, usage: 50 },
  disk: { total: 100, used: 30, free: 70, usage: 30 },
  uptime: 123,
  loadAverage: [0.1, 0.2, 0.3],
  kernelVersion: '6.8.12',
  pveVersion: 'pve-manager/8.3.0',
  cpuInfo: { model: 'EPYC', cores: 8, sockets: 1 },
  lastSeen: new Date().toISOString(),
  connectionHealth: 'online',
  linkedAgentId: undefined,
  ...overrides,
});

const makeAgent = (overrides: Partial<Agent> = {}): Agent => ({
  id: 'agent-1',
  hostname: 'pve1.local',
  displayName: 'PVE Agent',
  memory: { total: 8, used: 4, free: 4, usage: 50 },
  status: 'online',
  lastSeen: Date.now(),
  ...overrides,
});

describe('InfrastructureDetailsDrawer', () => {
  it('uses linkedAgentId when no agent object is provided', async () => {
    discoveryTabMock.mockClear();
    webInterfaceUrlFieldMock.mockClear();

    render(() => (
      <InfrastructureDetailsDrawer node={makeNode({ linkedAgentId: 'agent-host-1' })} />
    ));

    expect(webInterfaceUrlFieldMock).toHaveBeenCalledWith(
      expect.objectContaining({ metadataId: 'agent-host-1' }),
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Discovery' }));

    expect(discoveryTabMock).toHaveBeenCalledWith(
      expect.objectContaining({
        agentId: 'agent-host-1',
        resourceId: 'agent-host-1',
      }),
    );
  });

  it('prefers explicit agent identity over linkedAgentId', async () => {
    discoveryTabMock.mockClear();
    webInterfaceUrlFieldMock.mockClear();

    render(() => (
      <InfrastructureDetailsDrawer
        node={makeNode({ linkedAgentId: 'agent-host-1' })}
        agent={makeAgent({ id: 'agent-explicit-1', hostname: 'pve1.explicit' })}
      />
    ));

    expect(webInterfaceUrlFieldMock).toHaveBeenCalledWith(
      expect.objectContaining({ metadataId: 'agent-explicit-1' }),
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Discovery' }));

    expect(discoveryTabMock).toHaveBeenCalledWith(
      expect.objectContaining({
        agentId: 'agent-explicit-1',
        resourceId: 'agent-explicit-1',
        hostname: 'pve1.explicit',
      }),
    );
  });

  it('falls back to canonical agent metadata ids when the agent id is not the best identifier', async () => {
    discoveryTabMock.mockClear();
    webInterfaceUrlFieldMock.mockClear();

    render(() => (
      <InfrastructureDetailsDrawer
        node={makeNode({ linkedAgentId: 'agent-linked-1' })}
        agent={
          makeAgent({
            id: 'agent-explicit-1',
            hostname: 'pve1.explicit',
            platformData: {
              linkedAgentId: 'agent-linked-1',
            },
          }) as Agent
        }
      />
    ));

    expect(webInterfaceUrlFieldMock).toHaveBeenCalledWith(
      expect.objectContaining({ metadataId: 'agent-explicit-1' }),
    );

    await fireEvent.click(screen.getByRole('button', { name: 'Discovery' }));

    expect(discoveryTabMock).toHaveBeenCalledWith(
      expect.objectContaining({
        agentId: 'agent-explicit-1',
        resourceId: 'agent-explicit-1',
        hostname: 'pve1.explicit',
      }),
    );
  });
});
