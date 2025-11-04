import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { DockerAgents } from '../DockerAgents';
import type { DockerHost, RemovedDockerHost } from '@/types/api';

let mockWsStore: {
  state: { dockerHosts: DockerHost[]; removedDockerHosts: RemovedDockerHost[] };
  connected: () => boolean;
  reconnecting: () => boolean;
  activeAlerts: unknown[];
};

const allowDockerHostReenrollMock = vi.fn();
const deleteDockerHostMock = vi.fn();
const unhideDockerHostMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const fetchMock = vi.fn();

vi.mock('@/App', () => ({
  useWebSocket: () => mockWsStore,
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    allowDockerHostReenroll: (...args: unknown[]) => allowDockerHostReenrollMock(...args),
    deleteDockerHost: (...args: unknown[]) => deleteDockerHostMock(...args),
    unhideDockerHost: (...args: unknown[]) => unhideDockerHostMock(...args),
  },
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    createToken: vi.fn(),
  },
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: (...args: unknown[]) => notificationSuccessMock(...args),
    error: (...args: unknown[]) => notificationErrorMock(...args),
  },
}));

const createDockerHost = (overrides?: Partial<DockerHost>): DockerHost => ({
  id: 'host-1',
  agentId: 'agent-1',
  hostname: 'host-1.local',
  displayName: 'Host One',
  cpus: 4,
  totalMemoryBytes: 8 * 1024 * 1024 * 1024,
  uptimeSeconds: 12_345,
  status: 'online',
  lastSeen: Date.now(),
  intervalSeconds: 30,
  containers: [],
  ...overrides,
});

const createRemovedHost = (overrides?: Partial<RemovedDockerHost>): RemovedDockerHost => ({
  id: 'host-removed',
  hostname: 'retired-node.local',
  displayName: 'Retired Node',
  removedAt: Date.now() - 60_000,
  ...overrides,
});

const setupComponent = (hosts: DockerHost[], removedHosts: RemovedDockerHost[] = []) => {
  const [state] = createStore({
    dockerHosts: hosts,
    removedDockerHosts: removedHosts,
  });

  mockWsStore = {
    state,
    connected: () => true,
    reconnecting: () => false,
    activeAlerts: [],
  };

  return render(() => <DockerAgents />);
};

beforeEach(() => {
  allowDockerHostReenrollMock.mockReset();
  deleteDockerHostMock.mockReset();
  unhideDockerHostMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  fetchMock.mockReset();
  fetchMock.mockResolvedValue(
    new Response(JSON.stringify({ requiresAuth: true, apiTokenConfigured: false }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
  vi.stubGlobal('fetch', fetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('DockerAgents removed hosts', () => {
  it('renders removed hosts card and triggers allow/copy actions', async () => {
    allowDockerHostReenrollMock.mockResolvedValue(undefined);
    const clipboardSpy = vi.fn().mockResolvedValue(undefined);

    vi.stubGlobal('navigator', { clipboard: { writeText: clipboardSpy } } as unknown as Navigator);

    setupComponent([], [createRemovedHost()]);

    expect(screen.getByText('Recently removed container hosts')).toBeInTheDocument();
    const allowButton = screen.getByRole('button', { name: 'Allow re-enroll' });
    fireEvent.click(allowButton);

    await waitFor(() => expect(allowDockerHostReenrollMock).toHaveBeenCalledWith('host-removed'), { interval: 0 });

    const copyButton = screen.getByRole('button', { name: 'Copy curl command' });
    fireEvent.click(copyButton);

    await waitFor(() => expect(clipboardSpy).toHaveBeenCalledTimes(1), { interval: 0 });
    const copiedCommand = clipboardSpy.mock.calls.at(-1)?.[0];
    expect(typeof copiedCommand).toBe('string');
    expect(copiedCommand).toContain('/api/agents/docker/hosts/host-removed/allow-reenroll');

    expect(notificationSuccessMock).toHaveBeenCalled();
    expect(notificationErrorMock).not.toHaveBeenCalled();
  });

  it('shows progress stepper for an in-progress removal command', async () => {
    const now = Date.now();
    const host = createDockerHost({
      command: {
        id: 'cmd-1',
        type: 'stop',
        status: 'acknowledged',
        createdAt: now - 60_000,
        updatedAt: now - 20_000,
        dispatchedAt: now - 40_000,
        acknowledgedAt: now - 10_000,
      },
    });

    setupComponent([host]);

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    await screen.findByText('Remove container host "Host One"');
    expect(screen.getByText('acknowledged')).toBeInTheDocument();

    const progressHeading = screen.getByText('Progress');
    const progressCard = progressHeading.parentElement?.parentElement;
    if (!progressCard) {
      throw new Error('Progress card not found');
    }

    const steps = Array.from(progressCard.querySelectorAll('li'));
    expect(steps).toHaveLength(4);

    expect(steps[0]).toHaveTextContent('Stop command queued');
    expect(steps[1]).toHaveTextContent('Instruction delivered to the agent');
    expect(steps[2]).toHaveTextContent('Agent acknowledged the stop request');
    expect(steps[3]).toHaveTextContent('Agent disabled the service and removed autostart');

    const indicators = steps.map((step) => step.querySelector('span'));
    expect(indicators[0]?.className).toContain('bg-blue-500');
    expect(indicators[1]?.className).toContain('bg-blue-500');
    expect(indicators[2]?.className).toContain('animate-pulse');
    expect(indicators[3]?.className).toContain('bg-gray-300');
  });

  it('shows waiting message once the agent has already stopped', async () => {
    const host = createDockerHost({
      status: 'online',
      pendingUninstall: true,
      command: undefined,
      lastSeen: Date.now() - 45_000,
    });

    setupComponent([host]);

    const viewButton = await screen.findByRole('button', { name: 'View progress' });
    fireEvent.click(viewButton);

    await screen.findByText('Pulse is waiting for', { exact: false });

    const stopButton = screen.getByRole('button', { name: 'Waiting for host…' });
    expect(stopButton).toBeDisabled();
    expect(screen.queryByText('Progress')).not.toBeInTheDocument();
  });

  it('shows confirmation while waiting for the agent to acknowledge the stop command', async () => {
    deleteDockerHostMock.mockResolvedValue({});
    const host = createDockerHost();

    setupComponent([host]);

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    await screen.findByText('Remove container host "Host One"');

    const stopButton = screen.getByRole('button', { name: 'Stop agent now' });
    fireEvent.click(stopButton);

    await waitFor(() => expect(deleteDockerHostMock).toHaveBeenCalledWith('host-1'), { interval: 0 });

    const waitingButton = await screen.findByRole('button', { name: 'Waiting for agent…' });
    expect(waitingButton).toBeDisabled();

    expect(screen.getByText('Stop command sent.')).toBeInTheDocument();
    expect(notificationSuccessMock).toHaveBeenCalledWith('Stop command sent to Host One', 3500);
  });
});
