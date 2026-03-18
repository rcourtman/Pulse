import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@solidjs/testing-library';
import { AgentProfilesPanel } from '../AgentProfilesPanel';
import { notificationStore } from '@/stores/notifications';
import type { ConnectedInfrastructureItem, State } from '@/types/api';
import type { Resource } from '@/types/resource';

let mockResources: Resource[] = [];
let mockWsStore: {
  state: Pick<State, 'connectedInfrastructure'>;
};

const listProfilesMock = vi.fn();
const listAssignmentsMock = vi.fn();
const assignProfileMock = vi.fn();
const unassignProfileMock = vi.fn();
const createProfileMock = vi.fn();
const updateProfileMock = vi.fn();
const deleteProfileMock = vi.fn();

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    resources: () => mockResources,
  }),
}));

vi.mock('@/App', () => ({
  useWebSocket: () => mockWsStore,
}));

vi.mock('@/stores/license', () => ({
  getUpgradeActionUrlOrFallback: () => '/pricing',
  hasFeature: () => true,
  licenseLoaded: () => true,
  loadLicenseStatus: () => Promise.resolve(),
  licenseLoading: () => false,
}));

vi.mock('@/api/agentProfiles', () => ({
  MISSING_AGENT_PROFILE_ASSIGNMENT_MESSAGE:
    'Selected profile no longer exists. Refresh and choose another profile.',
  AgentProfilesAPI: {
    listProfiles: (...args: unknown[]) => listProfilesMock(...args),
    listAssignments: (...args: unknown[]) => listAssignmentsMock(...args),
    assignProfile: (...args: unknown[]) => assignProfileMock(...args),
    unassignProfile: (...args: unknown[]) => unassignProfileMock(...args),
    createProfile: (...args: unknown[]) => createProfileMock(...args),
    updateProfile: (...args: unknown[]) => updateProfileMock(...args),
    deleteProfile: (...args: unknown[]) => deleteProfileMock(...args),
  },
}));

vi.mock('@/api/ai', () => ({
  AIAPI: {
    getSettings: () => Promise.resolve({ enabled: false, configured: false }),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    debug: vi.fn(),
    warn: vi.fn(),
  },
}));

vi.mock('@/utils/upgradeMetrics', () => ({
  trackPaywallViewed: vi.fn(),
  trackUpgradeClicked: vi.fn(),
}));

const makeAgentResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'hash-agent-resource-id',
  type: 'agent',
  name: 'agent-one',
  displayName: 'Agent One',
  platformId: 'agent-one',
  platformType: 'agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'agent-one' },
  agent: { agentId: 'agent-123' },
  ...overrides,
});

const makeNodeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'node-resource-id',
  type: 'agent',
  name: 'pve-node-1',
  displayName: 'PVE Node One',
  platformId: 'pve-node-1',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'pve-node-1' },
  agent: { agentId: 'node-agent-1' },
  ...overrides,
});

const makeDockerRuntimeResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'docker-runtime-resource-id',
  type: 'docker-host',
  name: 'docker-runtime-1',
  displayName: 'Docker Runtime One',
  platformId: 'docker-runtime-1',
  platformType: 'docker',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'docker-runtime-1' },
  platformData: {
    agent: { agentId: 'agent-123' },
    docker: { hostSourceId: 'docker-runtime-1' },
  },
  ...overrides,
});

const makeKubernetesClusterResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'k8s-cluster-resource-id',
  type: 'k8s-cluster',
  name: 'cluster-1',
  displayName: 'Cluster One',
  platformId: 'cluster-1',
  platformType: 'kubernetes',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'cluster-1' },
  kubernetes: {
    clusterId: 'cluster-1',
    agentId: 'cluster-agent-1',
  },
  platformData: {
    kubernetes: { clusterId: 'cluster-1', agentId: 'cluster-agent-1' },
  },
  ...overrides,
});

const makeHostInfrastructureItem = (
  overrides: Partial<ConnectedInfrastructureItem> = {},
): ConnectedInfrastructureItem => ({
  id: 'hash-agent-resource-id',
  name: 'Agent One',
  displayName: 'Agent One',
  hostname: 'agent-one',
  status: 'active',
  healthStatus: 'online',
  lastSeen: Date.now(),
  scopeAgentId: 'agent-123',
  surfaces: [
    {
      id: 'agent:agent-123',
      kind: 'agent',
      label: 'Host telemetry',
      controlId: 'agent-123',
    },
  ],
  ...overrides,
});

const makeDockerInfrastructureItem = (
  overrides: Partial<ConnectedInfrastructureItem> = {},
): ConnectedInfrastructureItem => ({
  id: 'docker-runtime-resource-id',
  name: 'Docker Runtime One',
  displayName: 'Docker Runtime One',
  hostname: 'docker-runtime-1',
  status: 'active',
  healthStatus: 'online',
  lastSeen: Date.now(),
  scopeAgentId: 'agent-123',
  surfaces: [
    {
      id: 'docker:docker-runtime-1',
      kind: 'docker',
      label: 'Docker runtime data',
      controlId: 'docker-runtime-1',
    },
  ],
  ...overrides,
});

const makeKubernetesInfrastructureItem = (
  overrides: Partial<ConnectedInfrastructureItem> = {},
): ConnectedInfrastructureItem => ({
  id: 'k8s-cluster-resource-id',
  name: 'Cluster One',
  displayName: 'Cluster One',
  hostname: 'cluster-1',
  status: 'active',
  healthStatus: 'online',
  lastSeen: Date.now(),
  scopeAgentId: 'cluster-agent-1',
  surfaces: [
    {
      id: 'kubernetes:cluster-1',
      kind: 'kubernetes',
      label: 'Kubernetes cluster data',
      controlId: 'cluster-1',
    },
  ],
  ...overrides,
});

beforeEach(() => {
  mockResources = [makeAgentResource()];
  mockWsStore = {
    state: {
      connectedInfrastructure: [makeHostInfrastructureItem()],
    },
  };
  listProfilesMock.mockReset();
  listAssignmentsMock.mockReset();
  assignProfileMock.mockReset();
  unassignProfileMock.mockReset();
  createProfileMock.mockReset();
  updateProfileMock.mockReset();
  deleteProfileMock.mockReset();

  listProfilesMock.mockResolvedValue([
    {
      id: 'profile-a',
      name: 'Profile A',
      config: {},
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ]);
  listAssignmentsMock.mockResolvedValue([]);
});

afterEach(() => {
  cleanup();
});

describe('AgentProfilesPanel V6 agent ID handling', () => {
  it('maps assignments using actionable V6 agent ID instead of resource hash ID', async () => {
    listAssignmentsMock.mockResolvedValue([
      { agent_id: 'agent-123', profile_id: 'profile-a', updated_at: new Date().toISOString() },
    ]);

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.getByText('Agent Assignments')).toBeInTheDocument();
    });

    const agentCell = await screen.findByText('Agent One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(assignmentSelect.value).toBe('profile-a');
    });
  });

  it('keeps missing assigned profiles visible instead of showing a false empty state', async () => {
    listAssignmentsMock.mockResolvedValue([
      {
        agent_id: 'agent-123',
        profile_id: 'missing-profile',
        updated_at: new Date().toISOString(),
      },
    ]);

    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('Agent One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(assignmentSelect.value).toBe('missing-profile');
    });
    expect(
      within(assignmentSelect).getByRole('option', {
        name: 'Missing profile (missing-profile)',
      }),
    ).toBeInTheDocument();
    expect(within(assignmentSelect).getByRole('option', { name: 'Profile A' })).toBeInTheDocument();
  });

  it('sends actionable V6 agent ID when assigning a profile', async () => {
    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('Agent One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(
        within(assignmentSelect).getByRole('option', { name: 'Profile A' }),
      ).toBeInTheDocument();
    });
    fireEvent.change(assignmentSelect, { target: { value: 'profile-a' } });

    await waitFor(() => {
      expect(assignProfileMock).toHaveBeenCalledWith('agent-123', 'profile-a');
    });
  });

  it('reloads and surfaces the canonical missing-profile message when assignment races a deleted profile', async () => {
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
    assignProfileMock.mockRejectedValueOnce(
      new Error('Selected profile no longer exists. Refresh and choose another profile.'),
    );

    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('Agent One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(
        within(assignmentSelect).getByRole('option', { name: 'Profile A' }),
      ).toBeInTheDocument();
    });

    fireEvent.change(assignmentSelect, { target: { value: 'profile-a' } });

    await waitFor(() => {
      expect(listProfilesMock).toHaveBeenCalledTimes(2);
    });
    await waitFor(() => {
      expect(notificationStore.error).toHaveBeenCalledWith(
        'Selected profile no longer exists. Refresh and choose another profile.',
      );
    });
    await waitFor(() => {
      expect(
        within(assignmentSelect).queryByRole('option', { name: 'Profile A' }),
      ).not.toBeInTheDocument();
    });
  });

  it('sends actionable V6 agent ID when unassigning a profile', async () => {
    listAssignmentsMock.mockResolvedValue([
      { agent_id: 'agent-123', profile_id: 'profile-a', updated_at: new Date().toISOString() },
    ]);

    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('Agent One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(assignmentSelect.value).toBe('profile-a');
    });

    fireEvent.change(assignmentSelect, { target: { value: '' } });

    await waitFor(() => {
      expect(unassignProfileMock).toHaveBeenCalledWith('agent-123');
    });
  });

  it('lists assignable v6 agent resources (e.g. node agents)', async () => {
    mockResources = [makeNodeResource()];
    mockWsStore.state.connectedInfrastructure = [
      makeHostInfrastructureItem({
        id: 'node-resource-id',
        name: 'PVE Node One',
        displayName: 'PVE Node One',
        hostname: 'pve-node-1',
        scopeAgentId: 'node-agent-1',
        surfaces: [
          {
            id: 'agent:node-agent-1',
            kind: 'agent',
            label: 'Host telemetry',
            controlId: 'node-agent-1',
          },
        ],
      }),
    ];

    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('PVE Node One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(
        within(assignmentSelect).getByRole('option', { name: 'Profile A' }),
      ).toBeInTheDocument();
    });
    fireEvent.change(assignmentSelect, { target: { value: 'profile-a' } });

    await waitFor(() => {
      expect(assignProfileMock).toHaveBeenCalledWith('node-agent-1', 'profile-a');
    });
  });

  it('deduplicates resources that share the same actionable agent ID', async () => {
    mockResources = [
      makeDockerRuntimeResource(),
      makeNodeResource({ agent: { agentId: 'agent-123' } }),
    ];
    mockWsStore.state.connectedInfrastructure = [
      makeDockerInfrastructureItem(),
      makeHostInfrastructureItem({
        id: 'node-resource-id',
        name: 'PVE Node One',
        displayName: 'PVE Node One',
        hostname: 'pve-node-1',
      }),
    ];

    render(() => <AgentProfilesPanel />);

    await screen.findByText('PVE Node One');
    expect(screen.queryByText('Docker Runtime One')).not.toBeInTheDocument();
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
  });

  it('excludes offline assignable resources from agent assignments', async () => {
    mockResources = [makeAgentResource({ displayName: 'Offline Agent', status: 'offline' })];
    mockWsStore.state.connectedInfrastructure = [
      makeHostInfrastructureItem({
        name: 'Offline Agent',
        displayName: 'Offline Agent',
        healthStatus: 'offline',
      }),
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Offline Agent')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('excludes monitoring-stopped docker runtimes from agent assignments', async () => {
    mockResources = [makeDockerRuntimeResource()];
    mockWsStore.state.connectedInfrastructure = [
      {
        id: 'ignored:docker:docker-runtime-1',
        name: 'Docker Runtime One',
        displayName: 'Docker Runtime One',
        hostname: 'docker-runtime-1',
        status: 'ignored',
        removedAt: Date.now() - 1_000,
        surfaces: [
          {
            id: 'docker:docker-runtime-1',
            kind: 'docker',
            label: 'Docker runtime data',
            controlId: 'docker-runtime-1',
            action: 'allow-reconnect',
          },
        ],
      },
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Docker Runtime One')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('excludes monitoring-stopped kubernetes clusters from agent assignments', async () => {
    mockResources = [makeKubernetesClusterResource()];
    mockWsStore.state.connectedInfrastructure = [
      makeKubernetesInfrastructureItem({
        id: 'ignored:kubernetes:cluster-1',
        status: 'ignored',
        removedAt: Date.now(),
        surfaces: [
          {
            id: 'kubernetes:cluster-1',
            kind: 'kubernetes',
            label: 'Kubernetes cluster data',
            controlId: 'cluster-1',
            action: 'allow-reconnect',
          },
        ],
      }),
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Cluster One')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('excludes ignored host telemetry from assignments via connected infrastructure state', async () => {
    mockResources = [makeAgentResource()];
    mockWsStore.state.connectedInfrastructure = [
      {
        id: 'ignored:agent:agent-123',
        name: 'Agent One',
        displayName: 'Agent One',
        hostname: 'agent-one',
        status: 'ignored',
        removedAt: Date.now(),
        surfaces: [
          {
            id: 'agent:agent-123',
            kind: 'agent',
            label: 'Host telemetry',
            controlId: 'agent-123',
            action: 'allow-reconnect',
          },
        ],
      },
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Agent One')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('uses shared hostname fallback when identity hostname is absent', async () => {
    mockResources = [
      makeAgentResource({
        displayName: '',
        identity: undefined,
        name: '',
        platformId: 'platform-fallback',
        platformData: {
          agent: { hostname: 'platform-host.local' },
        },
      }),
    ];
    mockWsStore.state.connectedInfrastructure = [
      makeHostInfrastructureItem({
        id: 'hash-agent-resource-id',
        name: 'platform-host.local',
        displayName: '',
        hostname: 'platform-host.local',
      }),
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.getByText('platform-host.local')).toBeInTheDocument();
    });
  });

  it('surfaces malformed profile list responses instead of failing open to an empty state', async () => {
    listProfilesMock.mockRejectedValueOnce(new Error('Invalid agent profile list response from Pulse.'));

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(notificationStore.error).toHaveBeenCalledWith(
        'Invalid agent profile list response from Pulse.',
      );
    });
  });

  it('surfaces canonical profile response errors when creating a profile fails closed', async () => {
    createProfileMock.mockRejectedValueOnce(
      new Error('Invalid agent profile response from Pulse.'),
    );

    render(() => <AgentProfilesPanel />);

    fireEvent.click(await screen.findByRole('button', { name: /new profile/i }));
    fireEvent.input(screen.getByPlaceholderText('e.g., Production Servers'), {
      target: { value: 'Broken profile' },
    });
    fireEvent.click(screen.getByRole('button', { name: /^create profile$/i }));

    await waitFor(() => {
      expect(notificationStore.error).toHaveBeenCalledWith(
        'Invalid agent profile response from Pulse.',
      );
    });
  });
});
