import { describe, expect, it, vi } from 'vitest';
import { render } from '@solidjs/testing-library';

import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '@/components/Infrastructure/ResourceDetailDrawer';

const wsState = vi.hoisted(() => ({ pmg: [] as any[] }));
const reconnectSpy = vi.hoisted(() => vi.fn());

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    state: wsState,
    connected: () => true,
    initialDataReceived: () => true,
    reconnecting: () => false,
    reconnect: reconnectSpy,
  }),
  useDarkMode: () => () => false,
}));

vi.mock('@/components/Discovery/DiscoveryTab', () => ({
  DiscoveryTab: () => <div data-testid="discovery-tab" />,
}));

class ResizeObserverMock {
  constructor(_callback: ResizeObserverCallback) {}
  observe() {}
  unobserve() {}
  disconnect() {}
}

if (typeof globalThis.ResizeObserver === 'undefined') {
  vi.stubGlobal('ResizeObserver', ResizeObserverMock);
}

const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'resource-1',
  type: 'host',
  name: 'host-1',
  displayName: 'host-1',
  platformId: 'host-1',
  platformType: 'host-agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: { sources: ['agent', 'proxmox'] },
  ...overrides,
});

describe('ResourceDetailDrawer runtime and identity cards', () => {
  it('renders runtime source summary and mode details', () => {
    const resource = baseResource({
      platformData: {
        sources: ['agent', 'proxmox'],
        sourceStatus: {
          agent: { status: 'online', lastSeen: new Date().toISOString() },
          proxmox: { status: 'online', lastSeen: new Date().toISOString() },
        },
      },
    });

    const { getByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Runtime')).toBeInTheDocument();
    expect(getByText('Sources')).toBeInTheDocument();
    expect(getByText('2/2 healthy')).toBeInTheDocument();
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('Hybrid')).toBeInTheDocument();
  });

  it('falls back to source list summary when per-source health is unavailable', () => {
    const resource = baseResource({
      sourceType: 'api',
      platformData: {
        sources: ['pmg'],
      },
    });

    const { getByText, getAllByText } = render(() => <ResourceDetailDrawer resource={resource} />);

    expect(getByText('Sources')).toBeInTheDocument();
    expect(getAllByText('PMG').length).toBeGreaterThan(0);
    expect(getByText('Mode')).toBeInTheDocument();
    expect(getByText('API')).toBeInTheDocument();
  });

  it('shows identity aliases and fallback message when identity metadata is sparse', () => {
    const resource = baseResource({
      id: 'pmg-main',
      type: 'pmg',
      name: 'pmg-main',
      displayName: 'PMG Main',
      platformId: 'pmg-main',
      platformType: 'proxmox-pmg',
      sourceType: 'api',
      identity: {
        hostname: 'pmg.local',
      },
      platformData: {
        sources: ['pmg'],
        pmg: {
          instanceId: 'pmg-main',
        },
      },
    });

    const { getByText, getAllByText, queryByText } = render(() => (
      <ResourceDetailDrawer resource={resource} />
    ));

    expect(getByText('Identity')).toBeInTheDocument();
    expect(getByText('Aliases')).toBeInTheDocument();
    expect(getAllByText('pmg-main').length).toBeGreaterThan(0);
    expect(queryByText('No enriched identity metadata yet.')).toBeNull();

    const sparse = baseResource({
      id: 'host-min',
      name: 'host-min',
      displayName: 'Host Min',
      platformId: 'host-min',
      sourceType: 'agent',
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
      platformData: { sources: ['agent'] },
    });

    const sparseRender = render(() => <ResourceDetailDrawer resource={sparse} />);
    expect(sparseRender.getByText('No enriched identity metadata yet.')).toBeInTheDocument();
  });

  it('renders aliases inline when few exist and collapses when many exist', () => {
    const inlineResource = baseResource({
      type: 'host',
      platformData: {
        sources: ['agent'],
        agent: {
          agentId: 'agent-inline-1',
          hostname: 'inline-host.local',
        },
      },
      identity: undefined,
      parentId: undefined,
      clusterId: undefined,
      discoveryTarget: undefined,
      tags: [],
    });

    const inlineRender = render(() => <ResourceDetailDrawer resource={inlineResource} />);
    expect(inlineRender.getByText('Aliases')).toBeInTheDocument();
    expect(inlineRender.container.querySelector('details')).toBeNull();
    expect(inlineRender.getByText('agent-inline-1')).toBeInTheDocument();
    expect(inlineRender.getAllByText('inline-host.local').length).toBeGreaterThan(0);

    const overflowResource = baseResource({
      type: 'host',
      platformData: {
        sources: ['agent', 'proxmox'],
        agent: {
          agentId: 'agent-overflow-1',
          hostname: 'overflow-host.local',
        },
        proxmox: {
          nodeName: 'overflow-node-1',
        },
      },
      discoveryTarget: {
        resourceType: 'host',
        hostId: 'overflow-host-id',
        resourceId: 'overflow-resource-id',
      },
      identity: {
        hostname: 'overflow-identity-host',
      },
      tags: [],
    });

    const overflowRender = render(() => <ResourceDetailDrawer resource={overflowResource} />);
    expect(overflowRender.getByText('Aliases')).toBeInTheDocument();
    expect(overflowRender.container.querySelector('details')).toBeTruthy();
  });
});
