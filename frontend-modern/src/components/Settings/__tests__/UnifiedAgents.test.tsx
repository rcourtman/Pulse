import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup, within } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { Router, Route } from '@solidjs/router';
import { UnifiedAgents } from '../UnifiedAgents';
import type {
  Agent,
  ConnectedInfrastructureItem,
  DockerRuntime,
  KubernetesCluster,
  State,
} from '@/types/api';

let mockWsStore: {
  state: Pick<State, 'connectedInfrastructure'>;
  connected: () => boolean;
  reconnecting: () => boolean;
  activeAlerts: unknown[];
};

const lookupMock = vi.fn();
const createTokenMock = vi.fn();
const deleteAgentMock = vi.fn();
const allowHostAgentReenrollMock = vi.fn();
const updateAgentConfigMock = vi.fn();
const deleteDockerRuntimeMock = vi.fn();
const allowDockerRuntimeReenrollMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const notificationInfoMock = vi.fn();
const clipboardSpy = vi.fn();
const fetchMock = vi.fn();
const listProfilesMock = vi.fn();
const listAssignmentsMock = vi.fn();
const assignProfileMock = vi.fn();
const unassignProfileMock = vi.fn();
const trackAgentInstallTokenGeneratedMock = vi.fn();
const trackAgentInstallCommandCopiedMock = vi.fn();
const trackAgentInstallProfileSelectedMock = vi.fn();
const refetchResourcesMock = vi.fn();
const [mockResources, setMockResources] = createSignal<any[]>([]);
let securityStatusResponse = { requiresAuth: true, apiTokenConfigured: false };

vi.mock('@/App', () => ({
  useWebSocket: () => mockWsStore,
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    lookupAgent: (...args: unknown[]) => lookupMock(...args),
    deleteAgent: (...args: unknown[]) => deleteAgentMock(...args),
    allowHostAgentReenroll: (...args: unknown[]) => allowHostAgentReenrollMock(...args),
    updateAgentConfig: (...args: unknown[]) => updateAgentConfigMock(...args),
    deleteDockerRuntime: (...args: unknown[]) => deleteDockerRuntimeMock(...args),
    allowDockerRuntimeReenroll: (...args: unknown[]) => allowDockerRuntimeReenrollMock(...args),
  },
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    createToken: (...args: unknown[]) => createTokenMock(...args),
    getStatus: () => Promise.resolve(securityStatusResponse),
  },
}));

vi.mock('@/api/agentProfiles', () => ({
  AgentProfilesAPI: {
    listProfiles: (...args: unknown[]) => listProfilesMock(...args),
    listAssignments: (...args: unknown[]) => listAssignmentsMock(...args),
    assignProfile: (...args: unknown[]) => assignProfileMock(...args),
    unassignProfile: (...args: unknown[]) => unassignProfileMock(...args),
  },
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE:
    'Selected profile no longer exists. Refresh and choose another profile.',
}));

// Build mock resources from the WebSocket store data.
// The component uses resources() for agentResources (filtered by r.agent != null || r.type === 'docker-host')
// and byType('k8s-cluster') for kubernetes clusters.
const toAgentResource = (h: any) => ({
  id: h.id,
  type: 'agent' as const,
  platformType: 'agent' as const,
  sourceType: 'agent' as const,
  name: h.hostname || h.id,
  displayName: h.displayName,
  status: h.status || 'unknown',
  lastSeen: h.lastSeen,
  identity: { hostname: h.hostname },
  discoveryTarget: {
    resourceType: 'agent' as const,
    agentId: h.id,
    resourceId: h.id,
  },
  agent: {
    agentId: h.id,
    agentVersion: h.agentVersion,
    commandsEnabled: h.commandsEnabled,
    tokenName: h.tokenName,
    platform: h.platform,
    osName: h.osName,
  },
  platformData: {
    agent: {
      agentId: h.id,
      agentVersion: h.agentVersion,
      commandsEnabled: h.commandsEnabled,
      tokenName: h.tokenName,
      platform: h.platform,
      osName: h.osName,
    },
    agentVersion: h.agentVersion,
    isLegacy: h.isLegacy,
    linkedNodeId: h.linkedNodeId,
    commandsEnabled: h.commandsEnabled,
    agentId: h.id,
    platform: h.platform,
    osName: h.osName,
  },
});

const toDockerRuntimeResource = (d: any) => ({
  id: d.id,
  type: 'docker-host' as const,
  platformType: 'docker' as const,
  sourceType: 'agent' as const,
  name: d.hostname || d.id,
  displayName: d.displayName,
  status: d.status || 'unknown',
  lastSeen: d.lastSeen,
  identity: { hostname: d.hostname },
  discoveryTarget: {
    resourceType: 'app-container' as const,
    agentId: d.agentId || d.id,
    resourceId: d.id,
  },
  platformData: {
    agent: {
      agentId: d.agentId || d.id,
      agentVersion: d.agentVersion,
    },
    docker: {
      hostSourceId: d.id,
      agentVersion: d.agentVersion,
      dockerVersion: d.dockerVersion,
    },
    agentId: d.agentId || d.id,
    agentVersion: d.agentVersion,
    dockerVersion: d.dockerVersion,
    isLegacy: d.isLegacy,
  },
});

const toK8sClusterResource = (k: any) => ({
  id: k.id,
  type: 'k8s-cluster' as const,
  platformType: 'kubernetes' as const,
  sourceType: 'agent' as const,
  name: k.name || k.id,
  displayName: k.customDisplayName || k.displayName || k.name || k.id,
  status: k.status || 'unknown',
  lastSeen: k.lastSeen,
  discoveryTarget: {
    resourceType: 'k8s' as const,
    agentId: k.agentId || k.id,
    resourceId: k.id,
  },
  platformData: {
    kubernetes: {
      clusterId: k.id,
      clusterName: k.name,
      context: k.context,
      server: k.server,
      agentId: k.agentId,
    },
    agent: {
      agentId: k.agentId,
      agentVersion: k.agentVersion,
      tokenName: k.tokenName,
    },
  },
});

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    byType: (type: string) => mockResources().filter((resource) => resource.type === type),
    resources: () => mockResources(),
    mutate: (value: any[] | ((prev: any[]) => any[])) => {
      const next = typeof value === 'function' ? value(mockResources()) : value;
      setMockResources(next);
      return next;
    },
    refetch: (...args: unknown[]) => refetchResourcesMock(...args),
  }),
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
    info: (...args: unknown[]) => notificationInfoMock(...args),
  },
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackAgentInstallTokenGenerated: (...args: unknown[]) =>
    trackAgentInstallTokenGeneratedMock(...args),
  trackAgentInstallCommandCopied: (...args: unknown[]) =>
    trackAgentInstallCommandCopiedMock(...args),
  trackAgentInstallProfileSelected: (...args: unknown[]) =>
    trackAgentInstallProfileSelectedMock(...args),
}));

const createAgent = (overrides?: Partial<Agent>): Agent => ({
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

const createDockerHost = (overrides?: Partial<DockerRuntime>): DockerRuntime => ({
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

const createKubernetesCluster = (overrides?: Partial<KubernetesCluster>): KubernetesCluster => ({
  id: 'cluster-1',
  agentId: 'cluster-agent-1',
  name: 'cluster-1',
  displayName: 'Cluster One',
  status: 'online',
  lastSeen: Date.now(),
  intervalSeconds: 30,
  ...overrides,
});

const buildConnectedInfrastructureFromFixtures = ({
  hosts = [],
  dockerHosts = [],
  kubernetesClusters = [],
  ignoredItems = [],
}: {
  hosts?: Agent[];
  dockerHosts?: DockerRuntime[];
  kubernetesClusters?: KubernetesCluster[];
  ignoredItems?: ConnectedInfrastructureItem[];
}): ConnectedInfrastructureItem[] => {
  const items: ConnectedInfrastructureItem[] = [];

  hosts.forEach((host) => {
    items.push({
      id: host.id,
      name: host.displayName || host.hostname || host.id,
      displayName: host.displayName,
      hostname: host.hostname,
      status: 'active',
      healthStatus: host.status,
      lastSeen: host.lastSeen,
      version: host.agentVersion,
      isOutdatedBinary: host.isLegacy,
      linkedNodeId: host.linkedNodeId,
      commandsEnabled: host.commandsEnabled,
      scopeAgentId: host.id,
      uninstallAgentId: host.id,
      uninstallHostname: host.hostname,
      upgradePlatform: host.platform?.toLowerCase().includes('windows') ? 'windows' : 'linux',
      surfaces: [
        {
          id: `agent:${host.id}`,
          kind: 'agent',
          label: 'Host telemetry',
          detail: 'System health, inventory, and Pulse command connectivity.',
          controlId: host.id,
          action: 'stop-monitoring',
          idLabel: 'Agent ID',
          idValue: host.id,
        },
      ],
    });
  });

  dockerHosts.forEach((runtime) => {
    items.push({
      id: runtime.id,
      name: runtime.displayName || runtime.hostname || runtime.id,
      displayName: runtime.displayName,
      hostname: runtime.hostname,
      status: 'active',
      healthStatus: runtime.status,
      lastSeen: runtime.lastSeen,
      version: runtime.agentVersion || runtime.dockerVersion,
      isOutdatedBinary: runtime.isLegacy,
      scopeAgentId: runtime.agentId,
      upgradePlatform: 'linux',
      surfaces: [
        {
          id: `docker:${runtime.id}`,
          kind: 'docker',
          label: 'Docker runtime data',
          detail: 'Container runtime coverage reported from this machine.',
          controlId: runtime.id,
          action: 'stop-monitoring',
          idLabel: 'Docker runtime ID',
          idValue: runtime.id,
        },
      ],
    });
  });

  kubernetesClusters.forEach((cluster) => {
    items.push({
      id: cluster.id,
      name: cluster.displayName || cluster.name || cluster.id,
      displayName: cluster.displayName,
      status: 'active',
      healthStatus: cluster.status,
      lastSeen: cluster.lastSeen,
      version: cluster.agentVersion || cluster.version,
      scopeAgentId: cluster.agentId,
      upgradePlatform: 'linux',
      surfaces: [
        {
          id: `kubernetes:${cluster.id}`,
          kind: 'kubernetes',
          label: 'Kubernetes cluster data',
          detail: 'Cluster inventory and Kubernetes telemetry reported through Pulse.',
          controlId: cluster.id,
          action: 'stop-monitoring',
          idLabel: 'Cluster ID',
          idValue: cluster.id,
        },
      ],
    });
  });

  return [...items, ...ignoredItems];
};

const createIgnoredHostItem = (
  overrides?: Partial<ConnectedInfrastructureItem>,
): ConnectedInfrastructureItem => ({
  id: 'ignored:agent:removed-host-1',
  name: 'old-host.local',
  hostname: 'old-host.local',
  status: 'ignored',
  removedAt: Date.now() - 60_000,
  upgradePlatform: 'linux',
  surfaces: [
    {
      id: 'agent:removed-host-1',
      kind: 'agent',
      label: 'Host telemetry',
      detail: 'Pulse is blocking host telemetry from this machine.',
      controlId: 'removed-host-1',
      action: 'allow-reconnect',
      idLabel: 'Agent ID',
      idValue: 'removed-host-1',
    },
  ],
  ...overrides,
});

const createIgnoredDockerItem = (
  overrides?: Partial<ConnectedInfrastructureItem>,
): ConnectedInfrastructureItem => ({
  id: 'ignored:docker:removed-docker-1',
  name: 'old-docker.local',
  hostname: 'old-docker.local',
  status: 'ignored',
  removedAt: Date.now() - 60_000,
  upgradePlatform: 'linux',
  surfaces: [
    {
      id: 'docker:removed-docker-1',
      kind: 'docker',
      label: 'Docker runtime data',
      detail: 'Pulse is blocking Docker runtime reports from this machine.',
      controlId: 'removed-docker-1',
      action: 'allow-reconnect',
      idLabel: 'Docker runtime ID',
      idValue: 'removed-docker-1',
    },
  ],
  ...overrides,
});

const setupComponent = (
  hosts: Agent[] = [],
  dockerHosts: DockerRuntime[] = [],
  kubernetesClusters: KubernetesCluster[] = [],
  ignoredItems: ConnectedInfrastructureItem[] = [],
) => {
  setMockResources([
    ...hosts.map(toAgentResource),
    ...dockerHosts.map(toDockerRuntimeResource),
    ...kubernetesClusters.map(toK8sClusterResource),
  ]);

  const [state] = createStore<Pick<State, 'connectedInfrastructure'>>({
    connectedInfrastructure: buildConnectedInfrastructureFromFixtures({
      hosts,
      dockerHosts,
      kubernetesClusters,
      ignoredItems,
    }),
  });

  mockWsStore = {
    state,
    connected: () => true,
    reconnecting: () => false,
    activeAlerts: [],
  };

  return render(() => (
    <Router>
      <Route path="/" component={() => <UnifiedAgents />} />
    </Router>
  ));
};

const setupWithResources = (resources: any[], connectedInfrastructure: ConnectedInfrastructureItem[]) => {
  setMockResources(resources);

  const [state] = createStore<Pick<State, 'connectedInfrastructure'>>({
    connectedInfrastructure,
  });

  mockWsStore = {
    state,
    connected: () => true,
    reconnecting: () => false,
    activeAlerts: [],
  };

  return render(() => (
    <Router>
      <Route path="/" component={() => <UnifiedAgents />} />
    </Router>
  ));
};

const getConnectionUrlInput = (): HTMLInputElement => {
  const urlBlock = screen.getByText(/Connection URL \(Agent → Pulse\)/i).closest('div');
  expect(urlBlock).not.toBeNull();
  const urlInput = urlBlock?.querySelector('input');
  expect(urlInput).not.toBeNull();
  return urlInput as HTMLInputElement;
};

beforeEach(() => {
  securityStatusResponse = { requiresAuth: true, apiTokenConfigured: false };
  lookupMock.mockReset();
  createTokenMock.mockReset();
  deleteAgentMock.mockReset();
  allowHostAgentReenrollMock.mockReset();
  updateAgentConfigMock.mockReset();
  deleteDockerRuntimeMock.mockReset();
  allowDockerRuntimeReenrollMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  notificationInfoMock.mockReset();
  listProfilesMock.mockReset();
  listAssignmentsMock.mockReset();
  assignProfileMock.mockReset();
  unassignProfileMock.mockReset();
  trackAgentInstallTokenGeneratedMock.mockReset();
  trackAgentInstallCommandCopiedMock.mockReset();
  trackAgentInstallProfileSelectedMock.mockReset();
  refetchResourcesMock.mockReset();
  refetchResourcesMock.mockResolvedValue(mockResources());
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

  listProfilesMock.mockResolvedValue([]);
  listAssignmentsMock.mockResolvedValue([]);
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
    await waitFor(() => expect(screen.getByText(/Token.*created/i)).toBeInTheDocument(), {
      interval: 0,
    });
    expect(trackAgentInstallTokenGeneratedMock).toHaveBeenCalledWith(
      'settings_unified_agents',
      'manual',
    );
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Token generated with Agent config + reporting, Docker, and Kubernetes permissions.',
      4000,
    );
  });
});

describe('UnifiedAgents agent lookup', () => {
  it('performs agent lookup and displays results', async () => {
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

    const host = createAgent();
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
      expect(screen.getByPlaceholderText('Hostname or agent ID')).toBeInTheDocument();
    });

    lookupMock.mockResolvedValue({
      success: true,
      agent: {
        id: host.id,
        hostname: host.hostname,
        displayName: host.displayName,
        status: host.status,
        connected: true,
        lastSeen: host.lastSeen,
        agentVersion: host.agentVersion,
      },
    });

    const input = screen.getByPlaceholderText('Hostname or agent ID') as HTMLInputElement;
    fireEvent.input(input, { target: { value: host.id } });

    const checkButton = screen.getByRole('button', { name: /Check status/i });
    fireEvent.click(checkButton);

    await waitFor(() => expect(lookupMock).toHaveBeenCalled(), { interval: 0 });
    await waitFor(() => expect(screen.getByText('Connected')).toBeInTheDocument(), { interval: 0 });
  });

  it('shows error message when agent is not found', async () => {
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
      expect(screen.getByPlaceholderText('Hostname or agent ID')).toBeInTheDocument();
    });

    lookupMock.mockResolvedValue(null);

    const query = 'missing-host';
    const input = screen.getByPlaceholderText('Hostname or agent ID') as HTMLInputElement;
    fireEvent.input(input, { target: { value: query } });

    const checkButton = screen.getByRole('button', { name: /Check status/i });
    fireEvent.click(checkButton);

    await waitFor(
      () =>
        expect(
          screen.getByText(
            `No agent has reported with "${query}" yet. Try again in a few seconds.`,
          ),
        ).toBeInTheDocument(),
      { interval: 0 },
    );
  });
});

describe('UnifiedAgents managed agents table', () => {
  it('displays agents in the table', async () => {
    const host = createAgent({ hostname: 'test-server.local', displayName: 'Test Server' });
    setupComponent([host]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });
    expect(screen.getByText(/Hosts and runtimes currently checking in to Pulse\./i)).toBeInTheDocument();
    expect(screen.getByText('Reporting surfaces')).toBeInTheDocument();

    expect(screen.getAllByText('Test Server').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Host telemetry').length).toBeGreaterThan(0);
    expect(screen.getByText('online')).toBeInTheDocument();

    const toggle = screen.getByRole('button', { name: /details for Test Server/i });
    fireEvent.click(toggle);

    const detailsRow = document.getElementById('agent-details-agent-host-1');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(screen.getByText('Browse reporting items')).toBeInTheDocument();
    expect(screen.getByText('Select a reporting item to open its details drawer.')).toBeInTheDocument();
    expect(details.getByText('Selected reporting item')).toBeInTheDocument();
    expect(details.getByText('Machine overview')).toBeInTheDocument();
    expect(details.getByText('Surface controls')).toBeInTheDocument();
  });

  it('displays docker hosts in the table', async () => {
    const dockerHost = createDockerHost({
      hostname: 'docker-server.local',
      displayName: 'Docker Server',
    });
    setupComponent([], [dockerHost]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('Docker Server').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Docker runtime data').length).toBeGreaterThan(0);

    const toggle = screen.getByRole('button', { name: /details for Docker Server/i });
    fireEvent.click(toggle);

    const detailsRow = document.getElementById('agent-details-agent-docker-host-1');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(details.getByText('Machine actions')).toBeInTheDocument();
    expect(details.getByText('Surface controls')).toBeInTheDocument();
  });

  it('summarizes mixed machine coverage in the active row', async () => {
    setupWithResources(
      [
        {
          id: 'delly-resource',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          name: 'delly',
          displayName: 'delly',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'delly' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'delly-agent',
            resourceId: 'delly-resource',
          },
          agent: {
            agentId: 'delly-agent',
            agentVersion: '1.2.3',
            commandsEnabled: true,
            platform: 'linux',
            osName: 'Ubuntu',
          },
          platformData: {
            agent: {
              agentId: 'delly-agent',
              agentVersion: '1.2.3',
              commandsEnabled: true,
              platform: 'linux',
              osName: 'Ubuntu',
            },
            docker: {
              hostSourceId: 'delly-docker',
              dockerVersion: '26.0.0',
            },
            agentId: 'delly-agent',
          },
          proxmox: {
            node: 'delly',
            type: 'pve',
          },
        },
      ],
      [
        {
          id: 'delly-resource',
          name: 'delly',
          displayName: 'delly',
          hostname: 'delly',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          commandsEnabled: true,
          scopeAgentId: 'delly-agent',
          uninstallAgentId: 'delly-agent',
          uninstallHostname: 'delly',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:delly-agent',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'delly-agent',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'delly-agent',
            },
            {
              id: 'docker:delly-docker',
              kind: 'docker',
              label: 'Docker runtime data',
              detail: 'Container runtime coverage reported from this machine.',
              controlId: 'delly-docker',
              action: 'stop-monitoring',
              idLabel: 'Docker runtime ID',
              idValue: 'delly-docker',
            },
            {
              id: 'proxmox:delly-resource',
              kind: 'proxmox',
              label: 'Proxmox data',
              detail: 'Proxmox node telemetry linked to this machine.',
              idLabel: 'Node ID',
              idValue: 'delly-resource',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(
      screen.getByText(
        'Pulse is receiving host telemetry, Docker runtime data, and Proxmox data from this item.',
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /details for delly/i }));

    const detailsRow = document.getElementById('agent-details-agent-delly-resource');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(screen.getByText('Browse reporting items')).toBeInTheDocument();
    expect(details.getByText('Selected reporting item')).toBeInTheDocument();
    expect(details.getByText('Machine overview')).toBeInTheDocument();
    expect(details.getByText('Surface controls')).toBeInTheDocument();
    expect(details.getByText('Machine actions')).toBeInTheDocument();
    expect(details.getByText('Surface')).toBeInTheDocument();
    expect(details.getByText('What Pulse receives')).toBeInTheDocument();
    expect(details.getByText('ID')).toBeInTheDocument();
    expect(details.getByText('Control')).toBeInTheDocument();
    expect(
      details.getByText(
        'Use surface controls to stop specific reporting without removing the machine.',
      ),
    ).toBeInTheDocument();
    expect(details.getByText('Host telemetry')).toBeInTheDocument();
    expect(details.getByText('Docker runtime data')).toBeInTheDocument();
    expect(details.getByText('Proxmox data')).toBeInTheDocument();
    expect(
      details.getByText('System health, inventory, and Pulse command connectivity.'),
    ).toBeInTheDocument();
    expect(
      details.getByText('Container runtime coverage reported from this machine.'),
    ).toBeInTheDocument();
    expect(
      details.getByText('Proxmox node telemetry linked to this machine.'),
    ).toBeInTheDocument();
    expect(details.getByText('Agent ID')).toBeInTheDocument();
    expect(details.getByText('Docker runtime ID')).toBeInTheDocument();
    expect(details.getByText('Node ID')).toBeInTheDocument();
    expect(details.getAllByText('delly-resource').length).toBeGreaterThan(0);
    expect(details.getByText('Managed with host telemetry')).toBeInTheDocument();
    expect(details.getAllByText('delly-agent').length).toBeGreaterThan(0);
    expect(details.getAllByText('delly-docker').length).toBeGreaterThan(0);
  });

  it('distinguishes PBS coverage from Proxmox nodes on non-Proxmox hosts', async () => {
    setupWithResources(
      [
        {
          id: 'tower-resource',
          type: 'agent',
          platformType: 'agent',
          sourceType: 'hybrid',
          name: 'Tower',
          displayName: 'Tower',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'tower.local' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'tower-agent',
            resourceId: 'tower-resource',
          },
          agent: {
            agentId: 'tower-agent',
            agentVersion: '1.2.3',
            commandsEnabled: true,
            platform: 'linux',
            osName: 'Unraid',
          },
          platformData: {
            agent: {
              agentId: 'tower-agent',
              agentVersion: '1.2.3',
              commandsEnabled: true,
              platform: 'linux',
              osName: 'Unraid',
            },
            docker: {
              hostSourceId: 'tower-docker',
              dockerVersion: '26.0.0',
            },
            pbs: {
              version: '3.4.1',
            },
            agentId: 'tower-agent',
          },
        },
      ],
      [
        {
          id: 'tower-resource',
          name: 'Tower',
          displayName: 'Tower',
          hostname: 'tower.local',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          commandsEnabled: true,
          scopeAgentId: 'tower-agent',
          uninstallAgentId: 'tower-agent',
          uninstallHostname: 'tower.local',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:tower-agent',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'tower-agent',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'tower-agent',
            },
            {
              id: 'docker:tower-docker',
              kind: 'docker',
              label: 'Docker runtime data',
              detail: 'Container runtime coverage reported from this machine.',
              controlId: 'tower-docker',
              action: 'stop-monitoring',
              idLabel: 'Docker runtime ID',
              idValue: 'tower-docker',
            },
            {
              id: 'pbs:tower-resource',
              kind: 'pbs',
              label: 'PBS data',
              detail: 'Proxmox Backup Server inventory and backup telemetry.',
              idLabel: 'PBS ID',
              idValue: 'tower-resource',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(
      screen.getByText(
        'Pulse is receiving host telemetry, Docker runtime data, and PBS data from this item.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Proxmox data')).not.toBeInTheDocument();
    expect(
      screen.getByText('Pulse is currently receiving live reports from 1 host, 1 Docker runtime, and 1 PBS server.'),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /details for Tower/i }));

    const detailsRow = document.getElementById('agent-details-agent-tower-resource');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(details.getByText('PBS data')).toBeInTheDocument();
    expect(
      details.getByText('Proxmox Backup Server inventory and backup telemetry.'),
    ).toBeInTheDocument();
    expect(details.queryByText('Proxmox data')).not.toBeInTheDocument();
  });

  it('can stop only the docker surface for a mixed host without removing the parent machine row', async () => {
    deleteDockerRuntimeMock.mockResolvedValue({});

    setupWithResources(
      [
        {
          id: 'delly-resource',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          name: 'delly',
          displayName: 'delly',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'delly' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'delly-agent',
            resourceId: 'delly-resource',
          },
          agent: {
            agentId: 'delly-agent',
            agentVersion: '1.2.3',
            commandsEnabled: true,
            platform: 'linux',
            osName: 'Ubuntu',
          },
          platformData: {
            agent: {
              agentId: 'delly-agent',
              agentVersion: '1.2.3',
              commandsEnabled: true,
              platform: 'linux',
              osName: 'Ubuntu',
            },
            docker: {
              hostSourceId: 'delly-docker',
              dockerVersion: '26.0.0',
            },
            agentId: 'delly-agent',
          },
          proxmox: {
            node: 'delly',
            type: 'pve',
          },
        },
      ],
      [
        {
          id: 'delly-resource',
          name: 'delly',
          displayName: 'delly',
          hostname: 'delly',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          commandsEnabled: true,
          scopeAgentId: 'delly-agent',
          uninstallAgentId: 'delly-agent',
          uninstallHostname: 'delly',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:delly-agent',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'delly-agent',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'delly-agent',
            },
            {
              id: 'docker:delly-docker',
              kind: 'docker',
              label: 'Docker runtime data',
              detail: 'Container runtime coverage reported from this machine.',
              controlId: 'delly-docker',
              action: 'stop-monitoring',
              idLabel: 'Docker runtime ID',
              idValue: 'delly-docker',
            },
            {
              id: 'proxmox:delly-resource',
              kind: 'proxmox',
              label: 'Proxmox data',
              detail: 'Proxmox node telemetry linked to this machine.',
              idLabel: 'Node ID',
              idValue: 'delly-resource',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('delly').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: /details for delly/i }));

    const detailsRow = document.getElementById('agent-details-agent-delly-resource');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    const dockerSurfaceRow = details.getByText('Docker runtime data').closest('.grid');
    expect(dockerSurfaceRow).not.toBeNull();

    fireEvent.click(
      within(dockerSurfaceRow as HTMLElement).getByRole('button', { name: 'Stop this surface' }),
    );

    expect(screen.getByText('Stop monitoring?')).toBeInTheDocument();
    expect(screen.getByText(/Docker runtime data will stop in Pulse/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(
      () => expect(deleteDockerRuntimeMock).toHaveBeenCalledWith('delly-docker', { force: true }),
      { interval: 0 },
    );
    expect(deleteAgentMock).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(screen.getAllByText('delly').length).toBeGreaterThan(0);
    });
    expect(
      screen.queryByText('Pulse is receiving host telemetry, Docker runtime data, and Proxmox data from this item.'),
    ).not.toBeInTheDocument();
  });

  it('lists affected reporting surfaces in the stop-monitoring dialog for mixed hosts', async () => {
    setupWithResources(
      [
        {
          id: 'delly-resource',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          name: 'delly',
          displayName: 'delly',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'delly' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'delly-agent',
            resourceId: 'delly-resource',
          },
          agent: {
            agentId: 'delly-agent',
            agentVersion: '1.2.3',
            commandsEnabled: true,
            platform: 'linux',
            osName: 'Ubuntu',
          },
          platformData: {
            agent: {
              agentId: 'delly-agent',
              agentVersion: '1.2.3',
              commandsEnabled: true,
              platform: 'linux',
              osName: 'Ubuntu',
            },
            docker: {
              hostSourceId: 'delly-docker',
              dockerVersion: '26.0.0',
            },
            agentId: 'delly-agent',
          },
          proxmox: {
            node: 'delly',
            type: 'pve',
          },
        },
      ],
      [
        {
          id: 'delly-resource',
          name: 'delly',
          displayName: 'delly',
          hostname: 'delly',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          commandsEnabled: true,
          scopeAgentId: 'delly-agent',
          uninstallAgentId: 'delly-agent',
          uninstallHostname: 'delly',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:delly-agent',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'delly-agent',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'delly-agent',
            },
            {
              id: 'docker:delly-docker',
              kind: 'docker',
              label: 'Docker runtime data',
              detail: 'Container runtime coverage reported from this machine.',
              controlId: 'delly-docker',
              action: 'stop-monitoring',
              idLabel: 'Docker runtime ID',
              idValue: 'delly-docker',
            },
            {
              id: 'proxmox:delly-resource',
              kind: 'proxmox',
              label: 'Proxmox data',
              detail: 'Proxmox node telemetry linked to this machine.',
              idLabel: 'Node ID',
              idValue: 'delly-resource',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('delly').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));

    expect(screen.getByText('Stop monitoring?')).toBeInTheDocument();
    expect(
      screen.getByText(/Host telemetry, Docker runtime data, and Proxmox data will stop in Pulse/i),
    ).toBeInTheDocument();
    expect(screen.getByText('Pulse will stop these reporting surfaces')).toBeInTheDocument();
    expect(screen.getAllByText('Host telemetry').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Docker runtime data').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Proxmox data').length).toBeGreaterThan(0);
    expect(screen.getAllByText('delly-agent').length).toBeGreaterThan(0);
    expect(screen.getAllByText('delly-docker').length).toBeGreaterThan(0);
  });

  it('explains that stop monitoring moves items into the ignored-by-pulse list', async () => {
    setupComponent([createAgent({ displayName: 'Tower' })]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(
      screen.getByText(
        /Stop monitoring removes an item from active reporting and moves it into the Ignored by Pulse list/i,
      ),
    ).toBeInTheDocument();
  });

  it('includes canonical agent-facet resources without a legacy top-level agent payload', async () => {
    setupWithResources(
      [
        {
          id: 'resource-agent-platform-only',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          name: 'tower',
          displayName: 'Tower',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'tower.local' },
          platformData: {
            agent: {
              agentId: 'agent-platform-only',
              agentVersion: '1.2.3',
            },
            agentId: 'agent-platform-only',
          },
        },
      ],
      [
        {
          id: 'resource-agent-platform-only',
          name: 'Tower',
          displayName: 'Tower',
          hostname: 'tower.local',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          scopeAgentId: 'agent-platform-only',
          uninstallAgentId: 'agent-platform-only',
          uninstallHostname: 'tower.local',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:agent-platform-only',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'agent-platform-only',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'agent-platform-only',
            },
            {
              id: 'proxmox:resource-agent-platform-only',
              kind: 'proxmox',
              label: 'Proxmox data',
              detail: 'Proxmox node telemetry linked to this machine.',
              idLabel: 'Node ID',
              idValue: 'resource-agent-platform-only',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);

    const toggle = screen.getByRole('button', { name: /details for Tower/i });
    fireEvent.click(toggle);

    const detailsRow = document.getElementById('agent-details-agent-resource-agent-platform-only');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(details.getByText('Machine overview')).toBeInTheDocument();
  });

  it('shows empty state when no agents are installed', async () => {
    setupComponent();

    await waitFor(() => {
      expect(screen.getByText('Nothing is actively reporting to Pulse yet.')).toBeInTheDocument();
    });
  });

  it('shows outdated agent warning when older agent binaries are present', async () => {
    const legacyHost = createAgent({ isLegacy: true });
    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByText(/outdated agent binar(y|ies).*detected/i)).toBeInTheDocument();
    });

    const toggle = screen.getByRole('button', { name: /details for Host One/i });
    fireEvent.click(toggle);

    const detailsRow = document.getElementById('agent-details-agent-host-1');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    expect(details.getByText('Outdated')).toBeInTheDocument();
  });

  it('preserves docker-only flags in copied upgrade commands for outdated docker agents', async () => {
    const dockerHost = createDockerHost({
      id: 'docker-host-legacy',
      agentId: 'docker-agent-legacy',
      displayName: 'Docker Legacy',
      hostname: 'docker-legacy.local',
      isLegacy: true,
      agentVersion: '1.2.3',
    });

    setupComponent([], [dockerHost]);

    await waitFor(() => {
      expect(screen.getByText(/outdated agent binar(y|ies).*detected/i)).toBeInTheDocument();
    });

    const toggle = screen.getByRole('button', { name: /details for Docker Legacy/i });
    fireEvent.click(toggle);

    const copyUpgradeButton = screen.getByRole('button', { name: /Copy upgrade command/i });
    fireEvent.click(copyUpgradeButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('--enable-docker');
    expect(copied).toContain('--disable-host');
    expect(copied).toContain("--agent-id 'docker-agent-legacy'");
    expect(copied).toContain("--hostname 'docker-legacy.local'");
  });

  it('uses PowerShell upgrade commands for outdated Windows agents', async () => {
    const windowsHost = createAgent({
      id: 'windows-host-1',
      hostname: 'windows-host.local',
      displayName: 'Windows Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
      isLegacy: true,
    });

    setupComponent([windowsHost]);

    await waitFor(() => {
      expect(screen.getByText(/outdated agent binar(y|ies).*detected/i)).toBeInTheDocument();
    });

    const toggle = screen.getByRole('button', { name: /details for Windows Host/i });
    fireEvent.click(toggle);

    const copyUpgradeButton = screen.getByRole('button', { name: /Copy upgrade command/i });
    fireEvent.click(copyUpgradeButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('install.ps1');
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN=');
    expect(copied).toContain('$env:PULSE_AGENT_ID="windows-host-1"');
    expect(copied).toContain('$env:PULSE_HOSTNAME="windows-host.local"');
    expect(copied).not.toContain('| bash -s --');
  });

  it('uses PowerShell uninstall commands for Windows agents', async () => {
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

    const windowsHost = createAgent({
      id: 'windows-host-2',
      hostname: 'windows-uninstall.local',
      displayName: 'Windows Uninstall Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });
    const toggle = screen.getByRole('button', { name: /details for Windows Uninstall Host/i });
    fireEvent.click(toggle);

    const copyUninstallButton = screen.getByRole('button', { name: /Copy uninstall command/i });
    fireEvent.click(copyUninstallButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_UNINSTALL="true"');
    expect(copied).toContain('install.ps1');
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN="test-token"');
    expect(copied).toContain('$env:PULSE_AGENT_ID="windows-host-2"');
    expect(copied).toContain('$env:PULSE_HOSTNAME="windows-uninstall.local"');
    expect(copied).not.toContain('| bash -s --');
    expect(copied).not.toContain('token-record');
  });

  it('preserves PULSE_URL in token-optional Windows uninstall commands', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const windowsHost = createAgent({
      id: 'windows-host-3',
      hostname: 'windows-no-token.local',
      displayName: 'Windows No Token Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));

    const toggle = screen.getByRole('button', { name: /details for Windows No Token Host/i });
    fireEvent.click(toggle);

    const copyUninstallButton = screen.getByRole('button', { name: /Copy uninstall command/i });
    fireEvent.click(copyUninstallButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_UNINSTALL="true"');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
    expect(copied).not.toContain('$env:PULSE_TOKEN=');
  });

  it('fails closed for required-auth Windows uninstall commands without a token', async () => {
    const windowsHost = createAgent({
      id: 'windows-host-required-token',
      hostname: 'windows-required-token.local',
      displayName: 'Windows Required Token Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    fireEvent.click(
      screen.getByRole('button', { name: /details for Windows Required Token Host/i }),
    );
    fireEvent.click(screen.getByRole('button', { name: /Copy uninstall command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN="<api-token>"');
    expect(copied).toContain('$env:PULSE_UNINSTALL="true"');
  });

  it('PowerShell-escapes canonical identity in copied Windows uninstall commands', async () => {
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

    const windowsHost = createAgent({
      id: 'windows-$agent`"id',
      hostname: 'windows-$host`"name.local',
      displayName: 'Windows Escaped Uninstall Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });
    fireEvent.click(
      screen.getByRole('button', { name: /details for Windows Escaped Uninstall Host/i }),
    );
    fireEvent.click(screen.getByRole('button', { name: /Copy uninstall command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_AGENT_ID="windows-`$agent```"id"');
    expect(copied).toContain('$env:PULSE_HOSTNAME="windows-`$host```"name.local"');
  });

  it('quotes copied Windows uninstall script URLs', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-$token`"value',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    const windowsHost = createAgent({
      id: 'windows-uninstall-url-host',
      hostname: 'windows-uninstall-url.local',
      displayName: 'Windows Uninstall URL Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/base path/$url`"value' },
    });

    fireEvent.click(
      screen.getByRole('button', { name: /details for Windows Uninstall URL Host/i }),
    );
    fireEvent.click(screen.getByRole('button', { name: /Copy uninstall command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL="https://pulse.example.com/base path/`$url```"value"');
    expect(copied).toContain('$env:PULSE_TOKEN="test-`$token```"value"');
    expect(copied).toContain(
      '$pulseScriptUrl="https://pulse.example.com/base path/`$url```"value/install.ps1"',
    );
    expect(copied).toContain('irm $pulseScriptUrl');
  });

  it('preserves insecure TLS mode in Windows uninstall commands', async () => {
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

    const windowsHost = createAgent({
      id: 'windows-host-4',
      hostname: 'windows-self-signed.local',
      displayName: 'Windows Self-Signed Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
    });

    setupComponent([windowsHost]);

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => {
      expect(
        screen.getByRole('checkbox', { name: /Skip TLS certificate verification/i }),
      ).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('checkbox', { name: /Skip TLS certificate verification/i }));

    const toggle = screen.getByRole('button', { name: /details for Windows Self-Signed Host/i });
    fireEvent.click(toggle);

    const copyUninstallButton = screen.getByRole('button', { name: /Copy uninstall command/i });
    fireEvent.click(copyUninstallButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(copied).toContain('$env:PULSE_TOKEN="test-token"');
    expect(copied).toContain('$env:PULSE_UNINSTALL="true"');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
    expect(copied).toContain(
      '[System.Net.ServicePointManager]::ServerCertificateValidationCallback',
    );
  });

  it('does not fall back to token record IDs in shell uninstall commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'linux-secret-token',
      record: {
        id: 'token-record-id',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent([
      createAgent({
        id: 'linux-host-2',
        hostname: 'linux-uninstall.local',
        displayName: 'Linux Uninstall Host',
        platform: 'linux',
        osName: 'Ubuntu',
      }),
    ]);

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });
    const toggle = screen.getByRole('button', { name: /details for Linux Uninstall Host/i });
    fireEvent.click(toggle);

    const copyUninstallButton = screen.getByRole('button', { name: /Copy uninstall command/i });
    fireEvent.click(copyUninstallButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('--uninstall');
    expect(copied).toContain("--token 'linux-secret-token'");
    expect(copied).toContain("--agent-id 'linux-host-2'");
    expect(copied).toContain("--hostname 'linux-uninstall.local'");
    expect(copied).not.toContain('token-record-id');
  });

  it('fails closed for required-auth shell uninstall commands without a token', async () => {
    setupComponent([
      createAgent({
        id: 'linux-required-token',
        hostname: 'linux-required-token.local',
        displayName: 'Linux Required Token Host',
      }),
    ]);

    fireEvent.click(screen.getByRole('button', { name: /details for Linux Required Token Host/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy uninstall command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('--uninstall');
    expect(copied).toContain("--token '<api-token>'");
    expect(copied).toContain("--agent-id 'linux-required-token'");
    expect(copied).toContain("--hostname 'linux-required-token.local'");
  });

  it('shell-escapes canonical identity in copied Unix uninstall commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'shell-secret-token',
      record: {
        id: 'token-record-id',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent([
      createAgent({
        id: "agent id's canonical",
        hostname: "host name's canonical",
        displayName: 'Escaped Linux Uninstall Host',
        platform: 'linux',
        osName: 'Ubuntu',
      }),
    ]);

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });
    fireEvent.click(
      screen.getByRole('button', { name: /details for Escaped Linux Uninstall Host/i }),
    );
    fireEvent.click(screen.getByRole('button', { name: /Copy uninstall command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("--agent-id 'agent id'\"'\"'s canonical'");
    expect(copied).toContain("--hostname 'host name'\"'\"'s canonical'");
  });

  it('separates active and ignored items into dedicated sections', async () => {
    const host = createAgent({ displayName: 'Active Host' });
    const ignoredDocker = createIgnoredDockerItem();
    setupComponent([host], [], [], [ignoredDocker]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(
      screen.getByText(
        '1 item is actively reporting right now, and 1 item is currently ignored by Pulse. Stopping monitoring in Pulse does not uninstall software on the remote system.',
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Ignored by Pulse').length).toBeGreaterThan(0);
    expect(screen.getByText('Active reporting')).toBeInTheDocument();
    expect(screen.getByText('Showing 1 of 1 active records.')).toBeInTheDocument();
    expect(screen.getAllByText('Ignored by Pulse').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Active Host').length).toBeGreaterThan(0);
    expect(screen.getAllByText('old-docker.local').length).toBeGreaterThan(0);
    expect(screen.getByText('Docker runtime')).toBeInTheDocument();
    expect(screen.getByText('1 item(s) are currently ignored by Pulse.')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Pulse is currently receiving live reports from 1 host.',
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText('Missing expected coverage')).not.toBeInTheDocument();
    expect(screen.queryByText('No Kubernetes reporter is currently connected.')).not.toBeInTheDocument();
    expect(
      screen.getByText(
        /Items you explicitly told Pulse to ignore stay out of live reporting until reconnect is allowed\./i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/This workspace does not list every asset Pulse has discovered\./i),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Select an ignored item to open its recovery drawer.'),
    ).toBeInTheDocument();
  });

  it('renders the canonical ignored docker entry without an overlapping active row', async () => {
    const ignoredDocker = createIgnoredDockerItem({
      id: 'ignored:docker:tower-docker',
      hostname: 'tower.local',
      name: 'Tower',
      displayName: 'Tower',
      surfaces: [
        {
          id: 'docker:tower-docker',
          kind: 'docker',
          label: 'Docker runtime data',
          detail: 'Pulse is blocking Docker runtime reports from this machine.',
          controlId: 'tower-docker',
          action: 'allow-reconnect',
          idLabel: 'Docker runtime ID',
          idValue: 'tower-docker',
        },
      ],
    });
    setupComponent([], [], [], [ignoredDocker]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('Ignored by Pulse').length).toBeGreaterThan(0);
    const ignoredTowerButton = screen.getByText('Select to review').closest('button');
    expect(ignoredTowerButton).not.toBeNull();
    fireEvent.click(ignoredTowerButton as HTMLButtonElement);
    expect(screen.getAllByText('Ignored surface').length).toBeGreaterThan(0);
    expect(screen.getAllByText('What Pulse is ignoring').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Recovery').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Docker runtime data').length).toBeGreaterThan(0);
    const ignoredDrawer = screen.getByRole('dialog', { name: 'Ignored item details' });
    const ignoredDetails = within(ignoredDrawer);
    expect(ignoredDetails.getByRole('button', { name: /Allow Docker reconnect/i })).toBeInTheDocument();
    expect(ignoredDetails.queryByRole('button', { name: 'Stop monitoring' })).not.toBeInTheDocument();
    expect(screen.getByText('Showing 0 of 0 active records.')).toBeInTheDocument();
    expect(screen.getByText('1 item(s) are currently ignored by Pulse.')).toBeInTheDocument();
  });

  it('allows reconnect for removed host agents from the ignored-by-pulse list', async () => {
    const ignoredHost = createIgnoredHostItem({
      id: 'ignored:agent:tower-agent',
      hostname: 'tower.local',
      name: 'Tower',
      displayName: 'Tower',
      surfaces: [
        {
          id: 'agent:tower-agent',
          kind: 'agent',
          label: 'Host telemetry',
          detail: 'Pulse is blocking host telemetry from this machine.',
          controlId: 'tower-agent',
          action: 'allow-reconnect',
          idLabel: 'Agent ID',
          idValue: 'tower-agent',
        },
      ],
    });
    allowHostAgentReenrollMock.mockResolvedValue(undefined);

    setupComponent([], [], [], [ignoredHost]);

    await waitFor(() => {
      expect(screen.getAllByText('Ignored by Pulse').length).toBeGreaterThan(0);
    });

    const ignoredTowerButton = screen.getByText('Select to review').closest('button');
    expect(ignoredTowerButton).not.toBeNull();
    fireEvent.click(ignoredTowerButton as HTMLButtonElement);
    expect(screen.getAllByText('Host telemetry').length).toBeGreaterThan(0);
    expect(screen.getByRole('button', { name: /Allow host reconnect/i })).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Allow host reconnect/i }));

    await waitFor(() => expect(allowHostAgentReenrollMock).toHaveBeenCalledWith('tower-agent'), {
      interval: 0,
    });
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Reconnect allowed for Tower. Pulse will accept reports from it again.',
    );
  });

  it('shows Kubernetes clusters in the unified table', async () => {
    const cluster = createKubernetesCluster({ displayName: 'K8s Alpha' });
    setupComponent([], [], [cluster]);

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('K8s Alpha').length).toBeGreaterThan(0);

    const capSelect = screen.getByLabelText('Capability');
    fireEvent.change(capSelect, { target: { value: 'kubernetes' } });

    expect(screen.getAllByText('K8s Alpha').length).toBeGreaterThan(0);
  });

  it('removes an agent row immediately after successful deletion', async () => {
    deleteAgentMock.mockResolvedValue(undefined);

    const host = createAgent({ displayName: 'Tower' });
    setupComponent([host]);

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));
    expect(screen.getByText('Stop monitoring?')).toBeInTheDocument();
    expect(screen.getByText(/Host telemetry.*will stop in Pulse/i)).toBeInTheDocument();
    expect(screen.getByText('Pulse will stop these reporting surfaces')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(() => expect(deleteAgentMock).toHaveBeenCalledWith('host-1'), { interval: 0 });
    await waitFor(
      () =>
        expect(screen.queryByRole('button', { name: 'Stop monitoring' })).not.toBeInTheDocument(),
      {
        interval: 0,
      },
    );
    expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByText('Tower')[0]);
    expect(screen.getAllByRole('button', { name: 'Allow host reconnect' }).length).toBeGreaterThan(0);
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Monitoring stopped for Tower. Pulse will ignore future reports until reconnect is allowed.',
    );
    expect(refetchResourcesMock).toHaveBeenCalled();
    expect(screen.getByText('Monitoring stopped for Tower')).toBeInTheDocument();
    expect(
      screen.getByText(/Pulse removed it from active reporting and will ignore new reports/i),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'View ignored items' })).toBeInTheDocument();
  });

  it('shows row-level progress while stop monitoring is in flight', async () => {
    let resolveDelete: (() => void) | undefined;
    deleteAgentMock.mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveDelete = resolve;
        }),
    );

    setupComponent([createAgent({ displayName: 'Tower' })]);

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));
    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: 'Stopping…' }).length).toBeGreaterThan(0);
    });

    resolveDelete?.();

    await waitFor(() => expect(deleteAgentMock).toHaveBeenCalledWith('host-1'), { interval: 0 });
    await waitFor(
      () =>
        expect(screen.queryByRole('button', { name: 'Stop monitoring' })).not.toBeInTheDocument(),
      {
        interval: 0,
      },
    );
    expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    expect(screen.getByText('Monitoring stopped for Tower')).toBeInTheDocument();
  });

  it('moves a stopped host into the ignored-by-pulse list immediately after deletion succeeds', async () => {
    deleteAgentMock.mockResolvedValue(undefined);

    setupComponent([createAgent({ displayName: 'Tower' })]);

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));
    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(() => expect(deleteAgentMock).toHaveBeenCalledWith('host-1'), { interval: 0 });

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('Ignored by Pulse').length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByText('Tower')[0]);
    expect(screen.getAllByRole('button', { name: 'Allow host reconnect' }).length).toBeGreaterThan(0);
  });

  it('force-removes docker runtimes from Pulse inventory', async () => {
    deleteDockerRuntimeMock.mockResolvedValue({});

    const dockerHost = createDockerHost({ displayName: 'Tower', status: 'online' });
    setupComponent([], [dockerHost]);

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));
    expect(screen.getByText('Stop monitoring?')).toBeInTheDocument();
    expect(screen.getByText(/will stop in Pulse/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(
      () => expect(deleteDockerRuntimeMock).toHaveBeenCalledWith('docker-host-1', { force: true }),
      { interval: 0 },
    );
    await waitFor(
      () =>
        expect(screen.queryByRole('button', { name: 'Stop monitoring' })).not.toBeInTheDocument(),
      {
        interval: 0,
      },
    );
    fireEvent.click(screen.getAllByText('Tower')[0]);
    expect(screen.getAllByRole('button', { name: 'Allow Docker reconnect' }).length).toBeGreaterThan(0);
    expect(notificationSuccessMock).toHaveBeenCalledWith(
      'Monitoring stopped for Tower. Pulse will ignore future reports until reconnect is allowed.',
    );
  });

  it('uses the canonical docker machine id for active runtime stop-monitoring actions', async () => {
    deleteDockerRuntimeMock.mockResolvedValue({});

    setupWithResources(
      [
        {
          id: 'agent-2de76498a76c287b',
          type: 'docker-host',
          name: 'Tower',
          displayName: 'Tower',
          platformId: 'Tower',
          platformType: 'docker',
          sourceType: 'hybrid',
          status: 'online',
          lastSeen: Date.now(),
          identity: {
            hostname: 'Tower',
            machineId: 'de6d3fee-2595-6c2b-6b08-43db6b0ab427',
          },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-2de76498a76c287b',
            resourceId: 'agent-2de76498a76c287b',
          },
          platformData: {
            agentId: 'Tower',
            dockerVersion: '27.5.1',
          },
        },
      ],
      [
        {
          id: 'agent-2de76498a76c287b',
          name: 'Tower',
          displayName: 'Tower',
          hostname: 'Tower',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '27.5.1',
          scopeAgentId: 'Tower',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'docker:de6d3fee-2595-6c2b-6b08-43db6b0ab427',
              kind: 'docker',
              label: 'Docker runtime data',
              detail: 'Container runtime coverage reported from this machine.',
              controlId: 'de6d3fee-2595-6c2b-6b08-43db6b0ab427',
              action: 'stop-monitoring',
              idLabel: 'Docker runtime ID',
              idValue: 'de6d3fee-2595-6c2b-6b08-43db6b0ab427',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Tower').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: 'Stop monitoring' }));
    fireEvent.click(screen.getByRole('button', { name: 'Confirm stop monitoring' }));

    await waitFor(
      () =>
        expect(deleteDockerRuntimeMock).toHaveBeenCalledWith(
          'de6d3fee-2595-6c2b-6b08-43db6b0ab427',
          { force: true },
        ),
      { interval: 0 },
    );
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

  it('applies Proxmox PBS target profile flags to shell install commands', async () => {
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
      expect(screen.getByLabelText('Target profile (optional)')).toBeInTheDocument();
    });

    const profileSelect = screen.getByLabelText('Target profile (optional)');
    fireEvent.change(profileSelect, { target: { value: 'proxmox-pbs' } });

    expect(trackAgentInstallProfileSelectedMock).toHaveBeenCalledWith(
      'settings_unified_agents',
      'proxmox-pbs',
    );
    await waitFor(() => {
      const commandBlocks = screen.getAllByText(/--proxmox-type pbs/i);
      expect(commandBlocks.length).toBeGreaterThan(0);
    });
  });

  it('applies Proxmox PBS target profile env to copied Windows install commands', async () => {
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
      expect(screen.getByLabelText('Target profile (optional)')).toBeInTheDocument();
    });

    const profileSelect = screen.getByLabelText('Target profile (optional)');
    fireEvent.change(profileSelect, { target: { value: 'proxmox-pbs' } });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[copyButtons.length - 1]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_ENABLE_PROXMOX="true"');
    expect(copied).toContain('$env:PULSE_PROXMOX_TYPE="pbs"');
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('install.ps1');
  });

  it('applies Docker target profile flags to shell install commands', async () => {
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
      expect(screen.getByLabelText('Target profile (optional)')).toBeInTheDocument();
    });

    const profileSelect = screen.getByLabelText('Target profile (optional)');
    fireEvent.change(profileSelect, { target: { value: 'docker' } });

    expect(trackAgentInstallProfileSelectedMock).toHaveBeenCalledWith(
      'settings_unified_agents',
      'docker',
    );
    await waitFor(() => {
      const dockerBlocks = screen.getAllByText(/--enable-docker/i);
      expect(dockerBlocks.length).toBeGreaterThan(0);
    });
    await waitFor(() => {
      const hostDisableBlocks = screen.getAllByText(/--disable-host/i);
      expect(hostDisableBlocks.length).toBeGreaterThan(0);
    });
    await waitFor(() => {
      const dockerEnvBlocks = screen.getAllByText(/\$env:PULSE_ENABLE_DOCKER="true"/i);
      expect(dockerEnvBlocks.length).toBeGreaterThan(0);
    });
    await waitFor(() => {
      const hostDisableEnvBlocks = screen.getAllByText(/\$env:PULSE_ENABLE_HOST="false"/i);
      expect(hostDisableEnvBlocks.length).toBeGreaterThan(0);
    });
  });

  it('tracks install command copies', async () => {
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
    const copyButton = screen.getAllByRole('button', { name: /Copy command/i })[0];
    fireEvent.click(copyButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    await waitFor(() => expect(trackAgentInstallCommandCopiedMock).toHaveBeenCalled(), {
      interval: 0,
    });
    const [surface, capability] = trackAgentInstallCommandCopiedMock.mock.calls[0] as [
      string,
      string,
    ];
    expect(surface).toBe('settings_unified_agents');
    expect(capability).toContain(':auto:');
  });

  it('omits token arguments from copied install commands when tokens are optional', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    setupComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));

    await waitFor(() => {
      expect(screen.getByText('Install on Linux')).toBeInTheDocument();
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('--url ');
    expect(copied).not.toContain('--token');
    expect(copied).not.toContain('disabled');
    expect(copied).not.toContain('PULSE_TOKEN');
  });

  it('preserves generated tokens in copied install commands when auth is optional', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };
    createTokenMock.mockResolvedValue({
      token: 'optional-token',
      record: {
        id: 'token-record',
        name: 'Optional Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Generate token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN="optional-token"');
    expect(copied).not.toContain('<api-token>');
  });

  it('preserves installer arguments when wrapping copied shell commands with privilege escalation', async () => {
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
      expect(screen.getByLabelText('Target profile (optional)')).toBeInTheDocument();
    });

    const profileSelect = screen.getByLabelText('Target profile (optional)');
    fireEvent.change(profileSelect, { target: { value: 'docker' } });

    const enableCommandsCheckbox = screen.getByRole('checkbox', {
      name: /Enable Pulse command execution/i,
    });
    fireEvent.click(enableCommandsCheckbox);

    const copyButton = screen.getAllByRole('button', { name: /Copy command/i })[0];
    fireEvent.click(copyButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('sudo bash -s -- --url ');
    expect(copied).toContain("--token 'test-token'");
    expect(copied).toContain('--enable-docker');
    expect(copied).toContain('--disable-host');
    expect(copied).toContain('--enable-commands');
    expect(copied).not.toContain('bash -s -- $1');
  });

  it('shell-escapes copied Unix install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: "token'withquote",
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

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: "https://pulse.example.com/base path/agent's" },
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain(
      "curl -fsSL 'https://pulse.example.com/base path/agent'\"'\"'s/install.sh'",
    );
    expect(copied).toContain("--url 'https://pulse.example.com/base path/agent'\"'\"'s'");
    expect(copied).toContain("--token 'token'\"'\"'withquote'");
  });

  it('preserves plain-http insecure continuity in copied Unix install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'http-token',
      record: {
        id: 'token-record',
        name: 'HTTP Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'http://pulse.example:7655' },
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("--url 'http://pulse.example:7655'");
    expect(copied).toContain("--token 'http-token'");
    expect(copied).toContain('--insecure');
    expect(copied).not.toContain('curl -kfsSL');
  });

  it('preserves custom CA transport in copied Unix install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'custom-ca-token',
      record: {
        id: 'token-record',
        name: 'Custom CA Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    fireEvent.input(screen.getByLabelText(/Custom CA certificate path \(optional\)/i), {
      target: { value: '/etc/pulse/custom-ca.pem' },
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("curl -fsSL --cacert '/etc/pulse/custom-ca.pem'");
    expect(copied).toContain("--cacert '/etc/pulse/custom-ca.pem'");
  });

  it('renders installer snippets with the same Unix transport and flags that copy uses', async () => {
    createTokenMock.mockResolvedValue({
      token: 'rendered-token',
      record: {
        id: 'token-record',
        name: 'Rendered Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'http://pulse.example:7655/' },
    });
    fireEvent.input(screen.getByLabelText(/Custom CA certificate path \(optional\)/i), {
      target: { value: '/etc/pulse/custom-ca.pem' },
    });
    fireEvent.click(screen.getByRole('checkbox', { name: /Enable Pulse command execution/i }));
    fireEvent.change(screen.getByLabelText('Target profile (optional)'), {
      target: { value: 'docker' },
    });

    const linuxHeading = screen.getByText('Install on Linux');
    const linuxSection = linuxHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(linuxSection).not.toBeNull();

    const rendered = within(linuxSection as HTMLElement).getByText((content) =>
      content.includes('curl -fsSL'),
    );
    expect(rendered.textContent).toContain(
      "curl -fsSL --cacert '/etc/pulse/custom-ca.pem' 'http://pulse.example:7655/install.sh'",
    );
    expect(rendered.textContent).toContain("--url 'http://pulse.example:7655'");
    expect(rendered.textContent).toContain("--token 'rendered-token'");
    expect(rendered.textContent).toContain('--enable-docker --disable-host --enable-commands');
    expect(rendered.textContent).toContain('--insecure');
  });

  it('renders installer snippets with the same Windows env continuity that copy uses', async () => {
    createTokenMock.mockResolvedValue({
      token: 'windows-rendered-token',
      record: {
        id: 'token-record',
        name: 'Windows Rendered Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    fireEvent.input(screen.getByLabelText(/Custom CA certificate path \(optional\)/i), {
      target: { value: 'C:\\Pulse\\custom-ca.cer' },
    });
    fireEvent.click(screen.getByRole('checkbox', { name: /Skip TLS certificate verification/i }));
    fireEvent.click(screen.getByRole('checkbox', { name: /Enable Pulse command execution/i }));
    fireEvent.change(screen.getByLabelText('Target profile (optional)'), {
      target: { value: 'proxmox-pbs' },
    });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const rendered = within(windowsSection as HTMLElement).getAllByText((content) =>
      content.includes('$pulseScriptUrl='),
    )[0];
    expect(rendered.textContent).toContain('$env:PULSE_URL="http://localhost:3000"');
    expect(rendered.textContent).toContain('$env:PULSE_TOKEN="windows-rendered-token"');
    expect(rendered.textContent).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(rendered.textContent).toContain('$env:PULSE_CACERT="C:\\Pulse\\custom-ca.cer"');
    expect(rendered.textContent).toContain('$env:PULSE_ENABLE_PROXMOX="true"');
    expect(rendered.textContent).toContain('$env:PULSE_PROXMOX_TYPE="pbs"');
    expect(rendered.textContent).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
  });

  it('normalizes trailing slashes in copied install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'trimmed-token',
      record: {
        id: 'token-record',
        name: 'Trimmed Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/base/' },
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("curl -fsSL 'https://pulse.example.com/base/install.sh'");
    expect(copied).toContain("--url 'https://pulse.example.com/base'");
    expect(copied).not.toContain('//install.sh');
    expect(copied).not.toContain("--url 'https://pulse.example.com/base/'");
  });

  it('falls back to the canonical endpoint when the custom install URL is only whitespace', async () => {
    createTokenMock.mockResolvedValue({
      token: 'fallback-token',
      record: {
        id: 'token-record',
        name: 'Fallback Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: '   ' },
    });

    fireEvent.click(screen.getAllByRole('button', { name: /Copy command/i })[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("curl -fsSL 'http://localhost:3000/install.sh'");
    expect(copied).toContain("--url 'http://localhost:3000'");
  });

  it('preserves insecure and command-execution env in copied Windows install commands', async () => {
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

    const insecureCheckbox = screen.getByRole('checkbox', {
      name: /Skip TLS certificate verification/i,
    });
    fireEvent.click(insecureCheckbox);

    const enableCommandsCheckbox = screen.getByRole('checkbox', {
      name: /Enable Pulse command execution/i,
    });
    fireEvent.click(enableCommandsCheckbox);

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[copyButtons.length - 1]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
    expect(copied).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN="test-token"');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
    expect(copied).toContain(
      '[System.Net.ServicePointManager]::ServerCertificateValidationCallback',
    );
  });

  it('PowerShell-escapes copied Windows install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-$token`"value',
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

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/$url`"value' },
    });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[copyButtons.length - 1]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL="https://pulse.example.com/`$url```"value"');
    expect(copied).toContain('$env:PULSE_TOKEN="test-`$token```"value"');
    expect(copied).toContain(
      '$pulseScriptUrl="https://pulse.example.com/`$url```"value/install.ps1"',
    );
    expect(copied).toContain('irm $pulseScriptUrl');
  });

  it('preserves PULSE_URL in copied interactive Windows install commands', async () => {
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

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/base path/interactive' },
    });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL="https://pulse.example.com/base path/interactive"');
    expect(copied).toContain('$env:PULSE_TOKEN="test-token"');
    expect(copied).toContain(
      '$pulseScriptUrl="https://pulse.example.com/base path/interactive/install.ps1"',
    );
    expect(copied).toContain('irm $pulseScriptUrl');
  });

  it('normalizes trailing slashes in copied interactive Windows install commands', async () => {
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

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/base/' },
    });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL="https://pulse.example.com/base"');
    expect(copied).toContain('$pulseScriptUrl="https://pulse.example.com/base/install.ps1"');
    expect(copied).not.toContain('//install.ps1');
    expect(copied).not.toContain('$env:PULSE_URL="https://pulse.example.com/base/"');
  });

  it('uses the selected token instead of a placeholder in copied interactive Windows install commands', async () => {
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

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN="test-token"');
    expect(copied).not.toContain('<api-token>');
  });

  it('omits token transport in copied interactive Windows install commands when auth is optional', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    setupComponent();

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[0]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
    expect(copied).not.toContain('$env:PULSE_TOKEN=');
    expect(copied).not.toContain('<api-token>');
  });

  it('preserves custom CA transport in copied Windows install commands', async () => {
    createTokenMock.mockResolvedValue({
      token: 'windows-ca-token',
      record: {
        id: 'token-record',
        name: 'Windows CA Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    setupComponent();

    fireEvent.click(screen.getByRole('button', { name: /Generate token/i }));
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    fireEvent.input(screen.getByLabelText(/Custom CA certificate path \(optional\)/i), {
      target: { value: 'C:\\Pulse\\custom-ca.cer' },
    });

    const windowsHeading = screen.getByText('Install on Windows');
    const windowsSection = windowsHeading.closest('.space-y-3.rounded-md.border.border-border.p-4');
    expect(windowsSection).not.toBeNull();

    const copyButtons = within(windowsSection as HTMLElement).getAllByRole('button', {
      name: /Copy command/i,
    });
    fireEvent.click(copyButtons[copyButtons.length - 1]);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_CACERT="C:\\Pulse\\custom-ca.cer"');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
    expect(copied).toContain(
      '$pulseCustomCaBytes = [System.IO.File]::ReadAllBytes($env:PULSE_CACERT)',
    );
    expect(copied).toContain('$pulseCustomCaText.Contains("-----BEGIN CERTIFICATE-----")');
    expect(copied).toContain(
      '[System.Security.Cryptography.X509Certificates.X509Certificate2]::CreateFromPem($pulseCustomCaText)',
    );
  });

  it('preserves insecure and command-execution env in Windows upgrade commands', async () => {
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

    const windowsHost = createAgent({
      id: 'windows-host-upgrade-env',
      hostname: 'windows-upgrade-env.local',
      displayName: 'Windows Upgrade Env',
      platform: 'windows',
      osName: 'Windows Server 2022',
      isLegacy: true,
    });

    setupComponent([windowsHost]);

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const insecureCheckbox = screen.getByRole('checkbox', {
      name: /Skip TLS certificate verification/i,
    });
    fireEvent.click(insecureCheckbox);

    const enableCommandsCheckbox = screen.getByRole('checkbox', {
      name: /Enable Pulse command execution/i,
    });
    fireEvent.click(enableCommandsCheckbox);

    fireEvent.click(screen.getByRole('button', { name: /details for Windows Upgrade Env/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_ENABLE_COMMANDS="true"');
    expect(copied).toContain('$env:PULSE_INSECURE_SKIP_VERIFY="true"');
    expect(copied).toContain('$env:PULSE_URL=');
    expect(copied).toContain('$env:PULSE_TOKEN=');
    expect(copied).toContain('$env:PULSE_AGENT_ID="windows-host-upgrade-env"');
    expect(copied).toContain('$env:PULSE_HOSTNAME="windows-upgrade-env.local"');
    expect(copied).toContain('$pulseScriptUrl="http://localhost:3000/install.ps1"');
  });

  it('PowerShell-escapes canonical identity in copied Windows upgrade commands', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const windowsHost = createAgent({
      id: 'windows-$upgrade`"id',
      hostname: 'windows-$upgrade`"host.local',
      displayName: 'Windows Escaped Upgrade Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
      isLegacy: true,
    });

    setupComponent([windowsHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));
    fireEvent.click(
      screen.getByRole('button', { name: /details for Windows Escaped Upgrade Host/i }),
    );
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_AGENT_ID="windows-`$upgrade```"id"');
    expect(copied).toContain('$env:PULSE_HOSTNAME="windows-`$upgrade```"host.local"');
  });

  it('PowerShell-quotes copied Windows upgrade script URLs', async () => {
    createTokenMock.mockResolvedValue({
      token: 'test-$token`"value',
      record: {
        id: 'token-record',
        name: 'Test Token',
        prefix: 'abc',
        suffix: '123',
        createdAt: new Date().toISOString(),
      },
    });

    const windowsHost = createAgent({
      id: 'windows-upgrade-url-host',
      hostname: 'windows-upgrade-url.local',
      displayName: 'Windows Upgrade URL Host',
      platform: 'windows',
      osName: 'Windows Server 2022',
      isLegacy: true,
    });

    setupComponent([windowsHost]);

    const generateButton = screen.getByRole('button', { name: /Generate token/i });
    fireEvent.click(generateButton);

    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'https://pulse.example.com/base path/$url`"value' },
    });

    fireEvent.click(screen.getByRole('button', { name: /details for Windows Upgrade URL Host/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('$env:PULSE_URL="https://pulse.example.com/base path/`$url```"value"');
    expect(copied).toContain('$env:PULSE_TOKEN="test-`$token```"value"');
    expect(copied).toContain(
      '$pulseScriptUrl="https://pulse.example.com/base path/`$url```"value/install.ps1"',
    );
    expect(copied).toContain('irm $pulseScriptUrl');
  });

  it('omits token arguments from copied upgrade commands when tokens are optional', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const legacyHost = createAgent({
      id: 'legacy-host-no-token',
      hostname: 'legacy-no-token.local',
      displayName: 'Legacy No Token',
      isLegacy: true,
    });

    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));

    await waitFor(() => {
      expect(screen.getByText(/outdated agent binar(y|ies).*detected/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /details for Legacy No Token/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain('--url ');
    expect(copied).not.toContain('--token');
    expect(copied).not.toContain('disabled');
    expect(copied).toContain("--agent-id 'legacy-host-no-token'");
    expect(copied).toContain("--hostname 'legacy-no-token.local'");
  });

  it('shell-escapes canonical identity in copied Unix upgrade commands', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const legacyHost = createAgent({
      id: "legacy host's canonical",
      hostname: "legacy name's canonical",
      displayName: 'Escaped Legacy Host',
      isLegacy: true,
    });

    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));
    fireEvent.click(screen.getByRole('button', { name: /details for Escaped Legacy Host/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("--agent-id 'legacy host'\"'\"'s canonical'");
    expect(copied).toContain("--hostname 'legacy name'\"'\"'s canonical'");
  });

  it('preserves plain-http insecure continuity in copied Unix upgrade commands', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const legacyHost = createAgent({
      id: 'legacy-http-upgrade',
      hostname: 'legacy-http-upgrade.local',
      displayName: 'Legacy HTTP Upgrade',
      isLegacy: true,
    });

    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));

    await waitFor(() => {
      expect(screen.getByText(/Connection URL \(Agent → Pulse\)/i)).toBeInTheDocument();
    });

    const urlInput = getConnectionUrlInput();
    fireEvent.input(urlInput, {
      target: { value: 'http://pulse.example:7655' },
    });

    fireEvent.click(screen.getByRole('button', { name: /details for Legacy HTTP Upgrade/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("--url 'http://pulse.example:7655'");
    expect(copied).toContain('--insecure');
    expect(copied).not.toContain('curl -kfsSL');
  });

  it('preserves custom CA transport in copied Unix upgrade commands', async () => {
    securityStatusResponse = { requiresAuth: false, apiTokenConfigured: false };

    const legacyHost = createAgent({
      id: 'legacy-upgrade-ca',
      hostname: 'legacy-upgrade-ca.local',
      displayName: 'Legacy Upgrade CA',
      isLegacy: true,
    });

    setupComponent([legacyHost]);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Confirm without token/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: /Confirm without token/i }));
    fireEvent.input(screen.getByLabelText(/Custom CA certificate path \(optional\)/i), {
      target: { value: '/etc/pulse/upgrade-ca.pem' },
    });

    fireEvent.click(screen.getByRole('button', { name: /details for Legacy Upgrade CA/i }));
    fireEvent.click(screen.getByRole('button', { name: /Copy upgrade command/i }));

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });
    const copied = clipboardSpy.mock.calls.at(-1)?.[0] as string;
    expect(copied).toContain("curl -fsSL --cacert '/etc/pulse/upgrade-ca.pem'");
    expect(copied).toContain("--cacert '/etc/pulse/upgrade-ca.pem'");
  });

  it('keeps missing assigned profiles visible instead of collapsing scope to default', async () => {
    listProfilesMock.mockResolvedValue([
      {
        id: 'profile-a',
        name: 'Profile A',
        config: {},
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      },
    ]);
    listAssignmentsMock.mockResolvedValue([
      {
        agent_id: 'agent-platform-only',
        profile_id: 'missing-profile',
        updated_at: new Date().toISOString(),
      },
    ]);

    setupWithResources(
      [
        {
          id: 'resource-agent-platform-only',
          type: 'agent',
          platformType: 'proxmox-pve',
          sourceType: 'hybrid',
          name: 'tower',
          displayName: 'Tower',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'tower.local' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-platform-only',
            resourceId: 'resource-agent-platform-only',
          },
          agent: {
            agentId: 'agent-platform-only',
            agentVersion: '1.2.3',
          },
          platformData: {
            agent: {
              agentId: 'agent-platform-only',
              agentVersion: '1.2.3',
            },
            agentId: 'agent-platform-only',
          },
        },
      ],
      [
        {
          id: 'resource-agent-platform-only',
          name: 'Tower',
          displayName: 'Tower',
          hostname: 'tower.local',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          scopeAgentId: 'agent-platform-only',
          uninstallAgentId: 'agent-platform-only',
          uninstallHostname: 'tower.local',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:agent-platform-only',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'agent-platform-only',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'agent-platform-only',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    const toggle = screen.getByRole('button', { name: /details for Tower/i });
    fireEvent.click(toggle);

    const detailsRow = document.getElementById('agent-details-agent-resource-agent-platform-only');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    await waitFor(() => {
      expect(details.getByRole('combobox')).toBeInTheDocument();
    });
    const select = details.getByRole('combobox') as HTMLSelectElement;

    await waitFor(() => {
      expect(select.value).toBe('missing-profile');
    });
    expect(
      within(select).getByRole('option', { name: 'Missing profile (missing-profile)' }),
    ).toBeInTheDocument();
    expect(within(select).getByRole('option', { name: 'Profile A' })).toBeInTheDocument();
  });

  it('resyncs scope options after a missing-profile assignment rejection', async () => {
    listProfilesMock
      .mockResolvedValueOnce([
        {
          id: 'profile-a',
          name: 'Profile A',
          config: {},
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      ])
      .mockResolvedValueOnce([]);
    listAssignmentsMock.mockResolvedValueOnce([]).mockResolvedValueOnce([]);
    assignProfileMock.mockRejectedValueOnce(
      new Error('Selected profile no longer exists. Refresh and choose another profile.'),
    );

    setupWithResources(
      [
        {
          id: 'resource-agent-profile-resync',
          type: 'agent',
          platformType: 'agent',
          sourceType: 'agent',
          name: 'tower',
          displayName: 'Tower',
          status: 'online',
          lastSeen: Date.now(),
          identity: { hostname: 'tower.local' },
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'agent-profile-resync',
            resourceId: 'resource-agent-profile-resync',
          },
          agent: {
            agentId: 'agent-profile-resync',
            agentVersion: '1.2.3',
          },
          platformData: {
            agent: {
              agentId: 'agent-profile-resync',
              agentVersion: '1.2.3',
            },
            agentId: 'agent-profile-resync',
          },
        },
      ],
      [
        {
          id: 'resource-agent-profile-resync',
          name: 'Tower',
          displayName: 'Tower',
          hostname: 'tower.local',
          status: 'active',
          healthStatus: 'online',
          lastSeen: Date.now(),
          version: '1.2.3',
          scopeAgentId: 'agent-profile-resync',
          uninstallAgentId: 'agent-profile-resync',
          uninstallHostname: 'tower.local',
          upgradePlatform: 'linux',
          surfaces: [
            {
              id: 'agent:agent-profile-resync',
              kind: 'agent',
              label: 'Host telemetry',
              detail: 'System health, inventory, and Pulse command connectivity.',
              controlId: 'agent-profile-resync',
              action: 'stop-monitoring',
              idLabel: 'Agent ID',
              idValue: 'agent-profile-resync',
            },
          ],
        },
      ],
    );

    await waitFor(() => {
      expect(screen.getAllByText('Reporting now').length).toBeGreaterThan(0);
    });

    fireEvent.click(screen.getByRole('button', { name: /details for Tower/i }));

    const detailsRow = document.getElementById('agent-details-agent-resource-agent-profile-resync');
    expect(detailsRow).not.toBeNull();
    const details = within(detailsRow as HTMLElement);
    await waitFor(() => {
      expect(details.getByRole('combobox')).toBeInTheDocument();
    });

    const select = details.getByRole('combobox') as HTMLSelectElement;
    fireEvent.change(select, { target: { value: 'profile-a' } });

    await waitFor(() => {
      expect(assignProfileMock).toHaveBeenCalledWith('agent-profile-resync', 'profile-a');
    });
    await waitFor(() => {
      expect(listProfilesMock).toHaveBeenCalledTimes(2);
      expect(listAssignmentsMock).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Selected profile no longer exists. Refresh and choose another profile.',
      );
    });
    await waitFor(() => {
      expect(details.queryByRole('combobox')).not.toBeInTheDocument();
    });
    expect(details.getByText('Default')).toBeInTheDocument();
  });

  it('surfaces malformed profile list responses instead of failing open to empty profile state', async () => {
    listProfilesMock.mockRejectedValueOnce(
      new Error('Invalid agent profile list response from Pulse.'),
    );

    setupWithResources([], []);

    await waitFor(() => {
      expect(notificationErrorMock).toHaveBeenCalledWith(
        'Invalid agent profile list response from Pulse.',
      );
    });
  });
});
