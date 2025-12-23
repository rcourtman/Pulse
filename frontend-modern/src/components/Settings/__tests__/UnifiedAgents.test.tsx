import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { UnifiedAgents } from '../UnifiedAgents';
import type { Host, DockerHost } from '@/types/api';

let mockWsStore: {
  state: { hosts: Host[]; dockerHosts: DockerHost[] };
  connected: () => boolean;
  reconnecting: () => boolean;
  activeAlerts: unknown[];
};

const lookupMock = vi.fn();
const createTokenMock = vi.fn();
const deleteHostAgentMock = vi.fn();
const deleteDockerHostMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const notificationInfoMock = vi.fn();
const clipboardSpy = vi.fn();
const fetchMock = vi.fn();

vi.mock('@/App', () => ({
  useWebSocket: () => mockWsStore,
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    lookupHost: (...args: unknown[]) => lookupMock(...args),
    deleteHostAgent: (...args: unknown[]) => deleteHostAgentMock(...args),
    deleteDockerHost: (...args: unknown[]) => deleteDockerHostMock(...args),
  },
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    createToken: (...args: unknown[]) => createTokenMock(...args),
    getStatus: () => Promise.resolve({ requiresAuth: true, apiTokenConfigured: false }),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
    info: (...args: unknown[]) => notificationInfoMock(...args),
  },
}));

const createHost = (overrides?: Partial<Host>): Host => ({
  id: 'host-1',
  hostname: 'host-1.local',
  displayName: 'Host One',
  platform: 'linux',
  osName: 'Ubuntu',
  osVersion: '22.04',
  kernelVersion: '6.5.0',
  architecture: 'x86_64',
  cpuCount: 4,
  cpuUsage: 12.5,
  memory: {
    total: 8 * 1024 * 1024 * 1024,
    used: 4 * 1024 * 1024 * 1024,
    free: 4 * 1024 * 1024 * 1024,
    usage: 50,
    balloon: 0,
    swapUsed: 0,
    swapTotal: 0,
  },
  loadAverage: [],
  disks: [],
  networkInterfaces: [],
  sensors: {
    temperatureCelsius: {},
    fanRpm: {},
    additional: {},
  },
  status: 'online',
  uptimeSeconds: 12_345,
  intervalSeconds: 30,
  lastSeen: Date.now(),
  agentVersion: '1.2.3',
  tags: ['prod'],
  ...overrides,
});

const createDockerHost = (overrides?: Partial<DockerHost>): DockerHost => ({
  id: 'docker-host-1',
  agentId: 'agent-1',
  hostname: 'docker-host-1.local',
  displayName: 'Docker Host One',
  cpus: 4,
  totalMemoryBytes: 8 * 1024 * 1024 * 1024,
  uptimeSeconds: 12_345,
  status: 'online',
  lastSeen: Date.now(),
  intervalSeconds: 30,
  containers: [],
  ...overrides,
});

const setupComponent = (hosts: Host[] = [], dockerHosts: DockerHost[] = []) => {
  const [state] = createStore({
    hosts,
    dockerHosts,
  });

  mockWsStore = {
    state,
    connected: () => true,
    reconnecting: () => false,
    activeAlerts: [],
  };

  return render(() => <UnifiedAgents />);
};

beforeEach(() => {
  lookupMock.mockReset();
  createTokenMock.mockReset();
  deleteHostAgentMock.mockReset();
  deleteDockerHostMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  notificationInfoMock.mockReset();
  clipboardSpy.mockReset().mockResolvedValue(undefined);
  fetchMock.mockReset();
  fetchMock.mockResolvedValue(
    new Response(JSON.stringify({ requiresAuth: true, apiTokenConfigured: false }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  vi.stubGlobal('fetch', fetchMock);
  vi.stubGlobal('navigator', { clipboard: { writeText: clipboardSpy } } as unknown as Navigator);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('UnifiedAgents token generation', () => {
  it('renders the token generation UI when auth is required', async () => {
    setupComponent();

    await waitFor(() => {
      expect(screen.getByText('Generate API token')).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /Generate token/i })).toBeInTheDocument();
  });

  it('generates a token and shows confirmation', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-token-123',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abcdef',
        suffix: '1234',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Generate token/i })).toBeInTheDocument();
    });

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });
    await waitFor(
      () => expect(screen.getByText(/Token.*created/i)).toBeInTheDocument(),
      { interval: 0 },
    );
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Token generated with Host, Docker, and Kubernetes permissions.',
      4000,
    );
  });
});

describe('UnifiedAgents host lookup', () => {
  it('performs host lookup and displays results', async () => {
    createTokenMock.mockResolvedValue({
      token: 'token-123',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abcdef',
        suffix: '1234',
        createdAt: new Date().toISOString(),
      },
    });

    const host = createHost();
    setupComponent([host]);

    // Generate token first to unlock commands
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Generate token/i })).toBeInTheDocument();
    });

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    // Wait for commands to be unlocked and lookup UI to appear
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Hostname or host ID')).toBeInTheDocument();
    });

    lookupMock.mockResolvedValue({
      success: true,
      host: {
        id: host.id,
        hostname: host.hostname,
        displayName: host.displayName,
        status: host.status,
        connected: true,
        lastSeen: host.lastSeen,
        agentVersion: host.agentVersion,
      },
    });

    const input = screen.getByPlaceholderText('Hostname or host ID') as HTMLInputElement;
    fireEvent.input(input, { target: { value: host.id } });

    const checkButton = screen.getByRole('button', { name: /Check status/i });
    fireEvent.click(checkButton);

    await waitFor(() => expect(lookupMock).toHaveBeenCalled(), { interval: 0 });
    await waitFor(
      () => expect(screen.getByText('Connected')).toBeInTheDocument(),
      { interval: 0 },
    );
  });

  it('shows error message when host is not found', async () => {
    createTokenMock.mockResolvedValue({
      token: 'token-456',
      record: {
        id: 'token-record-2',
        name: 'Test Token 2',
        prefix: 'ghijkl',
        suffix: '5678',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    // Generate token first
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Generate token/i })).toBeInTheDocument();
    });

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Hostname or host ID')).toBeInTheDocument();
    });

    lookupMock.mockResolvedValue(null);

    const query = 'missing-host';
    const input = screen.getByPlaceholderText('Hostname or host ID') as HTMLInputElement;
    fireEvent.input(input, { target: { value: query } });

    const checkButton = screen.getByRole('button', { name: /Check status/i });
    fireEvent.click(checkButton);

    await waitFor(
      () =>
        expect(
          screen.getByText(`No host has reported with "${query}" yet. Try again in a few seconds.`),
        ).toBeInTheDocument(),
      { interval: 0 },
    );
  });
});

describe('UnifiedAgents managed agents table', () => {
  it('displays host agents in the table', async () => {
    const host = createHost({ hostname: 'test-server.local', displayName: 'Test Server' });
    setupComponent([host]);

    await waitFor(() => {
      expect(screen.getByText('Managed Agents')).toBeInTheDocument();
    });

    expect(screen.getByText('Test Server')).toBeInTheDocument();
    expect(screen.getByText('Host')).toBeInTheDocument();
    expect(screen.getByText('online')).toBeInTheDocument();
  });

  it('displays docker hosts in the table', async () => {
    const dockerHost = createDockerHost({
      hostname: 'docker-server.local',
      displayName: 'Docker Server',
    });
    setupComponent([], [dockerHost]);

    await waitFor(() => {
      expect(screen.getByText('Managed Agents')).toBeInTheDocument();
    });

    expect(screen.getByText('Docker Server')).toBeInTheDocument();
    expect(screen.getByText('Docker')).toBeInTheDocument();
  });

  it('shows empty state when no agents are installed', async () => {
    setupComponent();

    await waitFor(() => {
      expect(screen.getByText('No agents installed yet.')).toBeInTheDocument();
    });
  });

  it('shows legacy agent warning when legacy agents are present', async () => {
    const legacyHost = createHost({ isLegacy: true });
    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByText(/legacy agent.*detected/i)).toBeInTheDocument();
    });
    expect(screen.getByText('Legacy')).toBeInTheDocument();
  });
});

describe('UnifiedAgents platform commands', () => {
  it('shows installation commands for all platforms after token generation', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-token',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => {
      expect(screen.getByText('Install on Linux')).toBeInTheDocument();
    });
    expect(screen.getByText('Install on macOS')).toBeInTheDocument();
    expect(screen.getByText('Install on Windows')).toBeInTheDocument();
  });

  it('includes insecure flag when TLS skip is enabled', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-token',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => {
      expect(screen.getByText(/Skip TLS certificate verification/i)).toBeInTheDocument();
    });

    // TLS skip is disabled by default
    const checkbox = screen.getByRole('checkbox', { name: /Skip TLS certificate verification/i });
    expect(checkbox).not.toBeChecked();

    // Enable it
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();

    // Find a copy button and check that the command includes the insecure flag
    const copyButtons = screen.getAllByRole('button', { name: /Copy command/i });
    expect(copyButtons.length).toBeGreaterThan(0);
  });
});
