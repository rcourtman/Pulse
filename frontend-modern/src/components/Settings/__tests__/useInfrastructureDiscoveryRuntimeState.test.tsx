import { createRoot, createSignal } from 'solid-js';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useInfrastructureDiscoveryRuntimeState } from '../useInfrastructureDiscoveryRuntimeState';

const apiFetchMock = vi.hoisted(() => vi.fn());
const notificationStoreMock = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  info: vi.fn(),
}));

vi.mock('@/utils/apiClient', () => ({
  apiFetch: apiFetchMock,
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: notificationStoreMock,
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@/api/settings', () => ({
  SettingsAPI: {
    updateSystemSettings: vi.fn(),
  },
}));

const mountHook = () => {
  let dispose = () => {};
  let state!: ReturnType<typeof useInfrastructureDiscoveryRuntimeState>;

  createRoot((d) => {
    dispose = d;
    const [nodes] = createSignal([] as any[]);
    const [discoveryEnabled, setDiscoveryEnabled] = createSignal(false);
    const [discoverySubnet] = createSignal('auto');
    const [discoveryMode, setDiscoveryMode] = createSignal<'auto' | 'custom'>('auto');
    const [discoverySubnetDraft, setDiscoverySubnetDraft] = createSignal('');
    const [lastCustomSubnet, setLastCustomSubnet] = createSignal('');
    const [_discoverySubnetError, setDiscoverySubnetError] = createSignal<string | undefined>(
      undefined,
    );
    const [savingDiscoverySettings, setSavingDiscoverySettings] = createSignal(false);
    const [envOverrides] = createSignal<Record<string, boolean>>({});

    state = useInfrastructureDiscoveryRuntimeState({
      eventBus: {
        on: () => () => {},
      },
      nodes,
      discoveryEnabled,
      setDiscoveryEnabled,
      discoverySubnet,
      discoveryMode,
      setDiscoveryMode,
      discoverySubnetDraft,
      setDiscoverySubnetDraft,
      lastCustomSubnet,
      setLastCustomSubnet,
      setDiscoverySubnetError,
      savingDiscoverySettings,
      setSavingDiscoverySettings,
      envOverrides,
      normalizeSubnetList: (value) => value.trim(),
      isValidCIDR: () => true,
      applySavedDiscoverySubnet: () => {},
    });
  });

  return { dispose, state };
};

describe('useInfrastructureDiscoveryRuntimeState', () => {
  afterEach(() => {
    apiFetchMock.mockReset();
    notificationStoreMock.success.mockReset();
    notificationStoreMock.error.mockReset();
    notificationStoreMock.info.mockReset();
  });

  it('applies manual discovery results immediately from the explicit scan response', async () => {
    apiFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        servers: [
          {
            ip: '10.0.0.55',
            port: 8006,
            type: 'pve',
            version: '8.2.2',
            hostname: 'discovered-pve.lab',
          },
        ],
        errors: [],
        timestamp: 1_700_000_000_000,
      }),
    });

    const { dispose, state } = mountHook();

    await state.triggerDiscoveryScan();

    expect(state.discoveredNodes()).toEqual([
      {
        ip: '10.0.0.55',
        port: 8006,
        type: 'pve',
        version: '8.2.2',
        hostname: 'discovered-pve.lab',
        release: undefined,
      },
    ]);
    expect(state.discoveryScanStatus().scanning).toBe(false);
    expect(state.discoveryScanStatus().lastResultAt).toBe(1_700_000_000_000);
    expect(notificationStoreMock.success).toHaveBeenCalledWith('Discovery scan complete', 2000);

    dispose();
  });

  it('normalizes cached discovery updated timestamps into millisecond result times', async () => {
    apiFetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({
        servers: [
          {
            ip: '10.0.0.88',
            port: 8007,
            type: 'pbs',
            version: '3.2.1',
          },
        ],
        errors: [],
        updated: 1_700_000_010,
      }),
    });

    const { dispose, state } = mountHook();

    await state.loadDiscoveredNodes();

    expect(state.discoveryScanStatus().lastResultAt).toBe(1_700_000_010_000);
    expect(state.discoveredNodes()).toEqual([
      {
        ip: '10.0.0.88',
        port: 8007,
        type: 'pbs',
        version: '3.2.1',
        hostname: undefined,
        release: undefined,
      },
    ]);

    dispose();
  });
});
