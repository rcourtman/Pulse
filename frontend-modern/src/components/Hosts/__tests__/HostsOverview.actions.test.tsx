import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, cleanup } from '@solidjs/testing-library';
import { Router, Route } from '@solidjs/router';
import type { Host } from '@/types/api';
import { HostsOverview } from '@/components/Hosts/HostsOverview';

let currentHosts: Host[] = [];

const deleteHostAgentMock = vi.fn();
const getSecurityStatusMock = vi.fn();
const getAllHostMetadataMock = vi.fn();
const deleteHostMetadataMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();

vi.mock('@/App', () => ({
  useWebSocket: () => ({
    connected: () => true,
    reconnecting: () => false,
    reconnect: vi.fn(),
    state: {},
    activeAlerts: [],
  }),
}));

vi.mock('@/hooks/useResources', () => ({
  useResourcesAsLegacy: () => ({
    asHosts: () => currentHosts,
  }),
}));

vi.mock('@/hooks/useBreakpoint', () => ({
  useBreakpoint: () => ({
    isMobile: () => false,
  }),
}));

vi.mock('@/hooks/useColumnVisibility', () => ({
  useColumnVisibility: (_storageKey: string, columns: unknown[]) => ({
    visibleColumns: () => columns,
    availableToggles: () => columns,
    isHiddenByUser: () => false,
    toggle: vi.fn(),
    resetToDefaults: vi.fn(),
  }),
}));

vi.mock('@/stores/alertsActivation', () => ({
  useAlertsActivation: () => ({
    getTemperatureThreshold: () => 80,
  }),
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    getStatus: (...args: unknown[]) => getSecurityStatusMock(...args),
  },
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    deleteHostAgent: (...args: unknown[]) => deleteHostAgentMock(...args),
  },
}));

vi.mock('@/api/hostMetadata', () => ({
  HostMetadataAPI: {
    getAllMetadata: (...args: unknown[]) => getAllHostMetadataMock(...args),
    deleteMetadata: (...args: unknown[]) => deleteHostMetadataMock(...args),
    updateMetadata: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('@/utils/toast', () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
}));

vi.mock('@/utils/url', () => ({
  isKioskMode: () => false,
  subscribeToKioskMode: () => () => undefined,
}));

vi.mock('@/components/shared/Card', () => ({
  Card: (props: any) => <div>{props.children}</div>,
}));

vi.mock('@/components/shared/EmptyState', () => ({
  EmptyState: (props: { title?: string; description?: string }) => (
    <div>
      <div>{props.title}</div>
      <div>{props.description}</div>
    </div>
  ),
}));

vi.mock('@/components/shared/ScrollableTable', () => ({
  ScrollableTable: (props: any) => <div>{props.children}</div>,
}));

vi.mock('@/components/Hosts/HostsFilter', () => ({
  HostsFilter: () => <div data-testid="hosts-filter" />,
}));

vi.mock('@/components/Hosts/HostDrawer', () => ({
  HostDrawer: () => <div data-testid="host-drawer" />,
}));

vi.mock('@/components/shared/StatusDot', () => ({
  StatusDot: () => <span data-testid="status-dot" />,
}));

vi.mock('@/components/Dashboard/EnhancedCPUBar', () => ({
  EnhancedCPUBar: () => <div data-testid="cpu-bar" />,
}));

vi.mock('@/components/Dashboard/StackedMemoryBar', () => ({
  StackedMemoryBar: () => <div data-testid="memory-bar" />,
}));

vi.mock('@/components/Dashboard/StackedDiskBar', () => ({
  StackedDiskBar: () => <div data-testid="disk-bar" />,
}));

const createHost = (overrides: Partial<Host> = {}): Host => ({
  id: 'host-1',
  hostname: 'host-1.local',
  displayName: 'Host One',
  platform: 'linux',
  osName: 'Ubuntu',
  osVersion: '24.04',
  kernelVersion: '6.8.0',
  architecture: 'x86_64',
  cpuCount: 8,
  cpuUsage: 10,
  loadAverage: [0.2],
  memory: {
    total: 16 * 1024 * 1024 * 1024,
    used: 8 * 1024 * 1024 * 1024,
    free: 8 * 1024 * 1024 * 1024,
    usage: 50,
    balloon: 0,
    swapUsed: 0,
    swapTotal: 0,
  },
  disks: [],
  networkInterfaces: [],
  sensors: {
    temperatureCelsius: {},
    fanRpm: {},
    additional: {},
  },
  raid: [],
  status: 'online',
  uptimeSeconds: 12345,
  lastSeen: Date.now(),
  intervalSeconds: 30,
  agentVersion: '1.0.0',
  ...overrides,
});

const renderComponent = () =>
  render(() => (
    <Router>
      <Route path="/" component={() => <HostsOverview />} />
    </Router>
  ));

describe('HostsOverview row actions menu', () => {
  beforeEach(() => {
    currentHosts = [createHost()];

    deleteHostAgentMock.mockReset().mockResolvedValue(undefined);
    getSecurityStatusMock.mockReset().mockResolvedValue({ tokenScopes: ['settings:write'] });
    getAllHostMetadataMock.mockReset().mockResolvedValue({});
    deleteHostMetadataMock.mockReset().mockResolvedValue(undefined);
    showSuccessMock.mockReset();
    showErrorMock.mockReset();
  });

  afterEach(() => {
    cleanup();
    vi.restoreAllMocks();
  });

  it('opens and closes the row actions menu via outside click', async () => {
    renderComponent();

    const trigger = await screen.findByTitle('Host actions');
    fireEvent.click(trigger);

    expect(await screen.findByRole('button', { name: /remove host from pulse/i })).toBeInTheDocument();

    fireEvent.mouseDown(document.body);

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /remove host from pulse/i })).not.toBeInTheDocument();
    });
  });

  it('closes the row actions menu when Escape is pressed', async () => {
    renderComponent();

    const trigger = await screen.findByTitle('Host actions');
    fireEvent.click(trigger);
    expect(await screen.findByRole('button', { name: /remove host from pulse/i })).toBeInTheDocument();

    fireEvent.keyDown(document, { key: 'Escape' });

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /remove host from pulse/i })).not.toBeInTheDocument();
    });
  });

  it('removes a host from the row actions menu', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    renderComponent();

    const trigger = await screen.findByTitle('Host actions');
    fireEvent.click(trigger);

    const removeButton = await screen.findByRole('button', { name: /remove host from pulse/i });
    fireEvent.click(removeButton);

    await waitFor(() => {
      expect(deleteHostAgentMock).toHaveBeenCalledWith('host-1');
    });
    expect(deleteHostMetadataMock).toHaveBeenCalledWith('host-1');
    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalledWith('Host One removed from Pulse');
    });
    expect(confirmSpy).toHaveBeenCalled();
  });

  it('hides row actions for read-only scoped tokens', async () => {
    getSecurityStatusMock.mockResolvedValue({ tokenScopes: ['monitoring:read'] });
    renderComponent();

    await waitFor(() => expect(getSecurityStatusMock).toHaveBeenCalled());

    expect(screen.queryByTitle('Host actions')).not.toBeInTheDocument();
  });

  it('positions menu within viewport bounds near the trigger', async () => {
    Object.defineProperty(window, 'visualViewport', {
      configurable: true,
      value: {
        width: 320,
        height: 200,
        offsetTop: 0,
      },
    });

    renderComponent();

    const trigger = await screen.findByTitle('Host actions');
    vi.spyOn(trigger, 'getBoundingClientRect').mockReturnValue({
      x: 302,
      y: 180,
      top: 180,
      left: 302,
      bottom: 196,
      right: 318,
      width: 16,
      height: 16,
      toJSON: () => ({}),
    } as DOMRect);

    fireEvent.click(trigger);

    const removeButton = await screen.findByRole('button', { name: /remove host from pulse/i });
    const menu = removeButton.closest('[data-host-actions-menu]') as HTMLElement;

    expect(menu).toBeInTheDocument();
    expect(menu.style.top).toBe('94px');
    expect(menu.style.left).toBe('112px');
  });
});
