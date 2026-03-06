import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@solidjs/testing-library';
import { AgentProfilesPanel } from '../AgentProfilesPanel';
import type { Resource } from '@/types/resource';

let mockResources: Resource[] = [];
let mockWsStore: {
  state: {
    removedDockerHosts?: Array<{ id: string; removedAt: number }>;
    removedKubernetesClusters?: Array<{ id: string; removedAt: number }>;
  };
};

const listProfilesMock = vi.fn();
const listAssignmentsMock = vi.fn();
const assignProfileMock = vi.fn();
const unassignProfileMock = vi.fn();

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
  AgentProfilesAPI: {
    listProfiles: (...args: unknown[]) => listProfilesMock(...args),
    listAssignments: (...args: unknown[]) => listAssignmentsMock(...args),
    assignProfile: (...args: unknown[]) => assignProfileMock(...args),
    unassignProfile: (...args: unknown[]) => unassignProfileMock(...args),
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
  platformType: 'k8s',
  sourceType: 'agent',
  status: 'healthy',
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

beforeEach(() => {
  mockResources = [makeAgentResource()];
  mockWsStore = {
    state: {
      removedDockerHosts: [],
      removedKubernetesClusters: [],
    },
  };
  listProfilesMock.mockReset();
  listAssignmentsMock.mockReset();
  assignProfileMock.mockReset();
  unassignProfileMock.mockReset();

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

    render(() => <AgentProfilesPanel />);

    await screen.findByText('PVE Node One');
    expect(screen.queryByText('Docker Runtime One')).not.toBeInTheDocument();
    expect(screen.getAllByRole('combobox')).toHaveLength(1);
  });

  it('excludes offline assignable resources from agent assignments', async () => {
    mockResources = [makeAgentResource({ displayName: 'Offline Agent', status: 'offline' })];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Offline Agent')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('excludes monitoring-stopped docker runtimes from agent assignments', async () => {
    mockResources = [makeDockerRuntimeResource()];
    mockWsStore.state.removedDockerHosts = [
      { id: 'docker-runtime-1', removedAt: Date.now() - 1_000 },
    ];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Docker Runtime One')).not.toBeInTheDocument();
    });
    expect(screen.queryAllByRole('combobox')).toHaveLength(0);
  });

  it('excludes monitoring-stopped kubernetes clusters from agent assignments', async () => {
    mockResources = [makeKubernetesClusterResource()];
    mockWsStore.state.removedKubernetesClusters = [{ id: 'cluster-1', removedAt: Date.now() }];

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.queryByText('Cluster One')).not.toBeInTheDocument();
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

    render(() => <AgentProfilesPanel />);

    await waitFor(() => {
      expect(screen.getByText('platform-host.local')).toBeInTheDocument();
    });
  });
});
