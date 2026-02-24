import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent, cleanup, within } from '@solidjs/testing-library';
import { AgentProfilesPanel } from '../AgentProfilesPanel';
import type { Resource } from '@/types/resource';

let mockHostResources: Resource[] = [];

const listProfilesMock = vi.fn();
const listAssignmentsMock = vi.fn();
const assignProfileMock = vi.fn();
const unassignProfileMock = vi.fn();

vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({
    byType: (type: string) => (type === 'host' ? mockHostResources : []),
  }),
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

const makeHostResource = (overrides: Partial<Resource> = {}): Resource => ({
  id: 'hash-host-resource-id',
  type: 'host',
  name: 'host-one',
  displayName: 'Host One',
  platformId: 'host-one',
  platformType: 'host-agent',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  identity: { hostname: 'host-one' },
  agent: { agentId: 'agent-123' },
  ...overrides,
});

beforeEach(() => {
  mockHostResources = [makeHostResource()];
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

    const agentCell = await screen.findByText('Host One');
    const row = agentCell.closest('tr') as HTMLElement;
    const assignmentSelect = within(row).getByRole('combobox') as HTMLSelectElement;
    await waitFor(() => {
      expect(assignmentSelect.value).toBe('profile-a');
    });
  });

  it('sends actionable V6 agent ID when assigning a profile', async () => {
    render(() => <AgentProfilesPanel />);

    const agentCell = await screen.findByText('Host One');
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

    const agentCell = await screen.findByText('Host One');
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
});
