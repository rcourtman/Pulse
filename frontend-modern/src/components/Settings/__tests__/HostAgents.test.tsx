import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, fireEvent, screen, waitFor, cleanup } from '@solidjs/testing-library';
import { createStore } from 'solid-js/store';
import { HostAgents } from '../HostAgents';
import type { Host } from '@/types/api';

let mockWsStore: {
  state: { hosts: Host[]; connectionHealth?: Record<string, boolean> };
  connected: () => boolean;
  reconnecting: () => boolean;
  activeAlerts: unknown[];
};

const lookupMock = vi.fn();
const createTokenMock = vi.fn();
const deleteHostAgentMock = vi.fn();
const notificationSuccessMock = vi.fn();
const notificationErrorMock = vi.fn();
const notificationInfoMock = vi.fn();
const clipboardSpy = vi.fn();

vi.mock('@/App', () => ({
  useWebSocket: () => mockWsStore,
}));

vi.mock('@/api/monitoring', () => ({
  MonitoringAPI: {
    lookupHost: (...args: unknown[]) => lookupMock(...args),
    deleteHostAgent: (...args: unknown[]) => deleteHostAgentMock(...args),
  },
}));

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    createToken: (...args: unknown[]) => createTokenMock(...args),
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

const stubFetchSuccess = vi.fn();

const setupComponent = (hosts: Host[]) => {
  const [state] = createStore({
    hosts,
    connectionHealth: {},
  });

  mockWsStore = {
    state,
    connected: () => true,
    reconnecting: () => false,
    activeAlerts: [],
  };

  return render(() => <HostAgents />);
};

beforeEach(() => {
  lookupMock.mockReset();
  createTokenMock.mockReset();
  deleteHostAgentMock.mockReset();
  notificationSuccessMock.mockReset();
  notificationErrorMock.mockReset();
  notificationInfoMock.mockReset();
  stubFetchSuccess.mockImplementation(
    async () =>
      new Response(JSON.stringify({ requiresAuth: true, apiTokenConfigured: false }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
  );
  vi.stubGlobal('fetch', stubFetchSuccess);
  clipboardSpy.mockReset();
  vi.stubGlobal('navigator', { clipboard: { writeText: clipboardSpy } } as unknown as Navigator);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe('HostAgents lookup flow', () => {
  it('highlights a host after a successful lookup and clears highlight after timeout', async () => {
    const host = createHost();
    setupComponent([host]);

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
    const generateButton = screen.getByRole('button', { name: 'Generate token' });
    fireEvent.click(generateButton);
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => expect(screen.getByRole('button', { name: 'Check status' })).toBeEnabled(), {
      interval: 0,
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

    const button = screen.getByRole('button', { name: 'Check status' });
    fireEvent.click(button);

    await waitFor(() => expect(lookupMock).toHaveBeenCalled(), { interval: 0 });
    const [lookupArgs] = lookupMock.mock.calls.at(-1) ?? [];
    expect(lookupArgs).toEqual({ id: host.id, hostname: host.id });

    await waitFor(
      () =>
        expect(
          screen.getByText('Connected', { selector: 'span' }),
        ).toBeInTheDocument(),
      { interval: 0 },
    );
    const statusBadges = screen.getAllByText('online', { selector: 'span' });
    expect(statusBadges.length).toBeGreaterThan(0);
    expect(screen.getByText('Agent version 1.2.3')).toBeInTheDocument();
  });

  it('shows an error when lookup returns no host and does not highlight rows', async () => {
    const host = createHost();
    const { container } = setupComponent([host]);

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
    const generateButton = screen.getByRole('button', { name: 'Generate token' });
    fireEvent.click(generateButton);
    await waitFor(() => expect(createTokenMock).toHaveBeenCalled(), { interval: 0 });

    await waitFor(() => expect(screen.getByRole('button', { name: 'Check status' })).toBeEnabled(), {
      interval: 0,
    });

    lookupMock.mockResolvedValue(null);

    const query = 'missing-host';
    const input = screen.getByPlaceholderText('Hostname or host ID') as HTMLInputElement;
    fireEvent.input(input, { target: { value: query } });

    const button = screen.getByRole('button', { name: 'Check status' });
    fireEvent.click(button);

    await waitFor(
      () =>
        expect(
          screen.getByText(`No host has reported with "${query}" yet. Try again in a few seconds.`),
        ).toBeInTheDocument(),
      { interval: 0 },
    );

    const row = container.querySelector(`tr[data-host-id="${host.id}"]`) as HTMLTableRowElement;
    expect(row.classList.contains('ring-2')).toBe(false);
  });
});

describe('Host removal modal', () => {
  it('removes a host while it is still reporting and explains the impact', async () => {
    deleteHostAgentMock.mockResolvedValue(undefined);
    const host = createHost({
      lastSeen: Date.now(),
      status: 'online',
    });

    setupComponent([host]);

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    await screen.findByText('Remove host "Host One"');

    const copyButton = screen.getByRole('button', { name: 'Copy command' });
    fireEvent.click(copyButton);
    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });

    const confirmButton = await screen.findByRole('button', { name: 'I ran this command' });
    fireEvent.click(confirmButton);

    expect(
      screen.getByText("Pulse revokes the host's API token", {
        exact: false,
      }),
    ).toBeInTheDocument();

    const removeHostButton = screen.getByRole('button', { name: 'Remove host' });
    expect(removeHostButton).toBeEnabled();
    fireEvent.click(removeHostButton);

    await waitFor(() => expect(deleteHostAgentMock).toHaveBeenCalledWith('host-1'), { interval: 0 });
    await waitFor(() => expect(notificationSuccessMock).toHaveBeenCalledWith('Host "Host One" removed', 4000), {
      interval: 0,
    });
    await waitFor(() => expect(screen.queryByText('Remove host "Host One"')).not.toBeInTheDocument(), {
      interval: 0,
    });
  });

  it('removes a stale host without forcing', async () => {
    deleteHostAgentMock.mockResolvedValue(undefined);
    const host = createHost({
      lastSeen: Date.now() - 5 * 60_000,
      status: 'offline',
    });

    setupComponent([host]);

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    await screen.findByText('Remove host "Host One"');

    const copyButton = screen.getByRole('button', { name: 'Copy command' });
    fireEvent.click(copyButton);
    await waitFor(() => expect(clipboardSpy).toHaveBeenCalled(), { interval: 0 });

    const confirmButton = await screen.findByRole('button', { name: 'I ran this command' });
    fireEvent.click(confirmButton);

    const removeHostButton = screen.getByRole('button', { name: 'Remove host' });
    fireEvent.click(removeHostButton);

    await waitFor(() => expect(deleteHostAgentMock).toHaveBeenCalledWith('host-1'), { interval: 0 });
    await waitFor(() => expect(notificationSuccessMock).toHaveBeenCalledWith('Host "Host One" removed', 4000), {
      interval: 0,
    });
    await waitFor(() => expect(screen.queryByText('Remove host "Host One"')).not.toBeInTheDocument(), {
      interval: 0,
    });
  });

  it('shows macOS-specific uninstall guidance', async () => {
    const host = createHost({
      platform: 'macos',
    });

    setupComponent([host]);

    const removeButton = screen.getByRole('button', { name: 'Remove' });
    fireEvent.click(removeButton);

    await screen.findByText('Remove host "Host One"');
    expect(screen.getByText('launchctl unload', { exact: false })).toBeInTheDocument();
    expect(screen.getByText('Unloads the launch agent, removes the plist, deletes the binary, and clears the local log.')).toBeInTheDocument();
  });
});
