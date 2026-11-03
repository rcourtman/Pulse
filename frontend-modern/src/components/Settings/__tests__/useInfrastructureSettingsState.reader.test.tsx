import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { VMwareAPI, type VMwareConnection } from '@/api/vmware';
import { eventBus } from '@/stores/events';
import { useInfrastructureSettingsState } from '../useInfrastructureSettingsState';

// Keep the containing hook and VMware state real: a lower-level reader test
// cannot detect a missing currentUser at the production call site.
vi.mock('../useInfrastructureConfiguredNodesState', () => ({
  useInfrastructureConfiguredNodesState: () => ({
    nodes: () => [],
    showNodeModal: () => false,
    loadNodes: vi.fn().mockResolvedValue(undefined),
    clearLoadNodesRetry: vi.fn(),
  }),
}));
vi.mock('../useInfrastructureDiscoveryRuntimeState', () => ({
  useInfrastructureDiscoveryRuntimeState: () => ({
    loadDiscoveredNodes: vi.fn().mockResolvedValue(undefined),
  }),
}));
vi.mock('../useTrueNASSettingsPanelState', () => ({ useTrueNASSettingsPanelState: () => ({}) }));
vi.mock('@/api/vmware', () => ({
  VMwareAPI: { listConnections: vi.fn() },
  isRedactedVMwareSecret: (value?: string) => value === '********',
}));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => 'team-a' }));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const connection = (): VMwareConnection => ({
  id: 'vc-1',
  name: 'Synthetic vCenter',
  host: 'vc.example.invalid',
  password: '********',
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: false,
  monitorDatastores: true,
});

describe('Infrastructure settings VMware reader wiring', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.mocked(VMwareAPI.listConnections).mockImplementation(async () => [connection()]);
  });

  it('withdraws the actual VMware draft when the containing settings user changes', async () => {
    const [currentUser, setUser] = createSignal<string | undefined>('reader-a');
    const noop = () => undefined;
    const { result } = renderHook(() => {
      const [enabled, setEnabled] = createSignal(false);
      const [text, setText] = createSignal('');
      const [mode, setMode] = createSignal<'auto' | 'custom'>('auto');
      return useInfrastructureSettingsState({
        eventBus,
        canReadInfrastructure: () => true,
        currentUser,
        discoveryEnabled: enabled,
        setDiscoveryEnabled: setEnabled,
        discoverySubnet: text,
        discoveryMode: mode,
        setDiscoveryMode: setMode,
        discoverySubnetDraft: text,
        setDiscoverySubnetDraft: setText,
        lastCustomSubnet: text,
        setLastCustomSubnet: setText,
        setDiscoverySubnetError: noop,
        savingDiscoverySettings: enabled,
        setSavingDiscoverySettings: setEnabled,
        envOverrides: () => ({}),
        normalizeSubnetList: (value) => value,
        isValidCIDR: () => false,
        applySavedDiscoverySubnet: noop,
        temperatureMonitoringEnabled: enabled,
        savingTemperatureSetting: enabled,
        setSavingTemperatureSetting: setEnabled,
        loadSecurityStatus: async () => undefined,
        initializeSystemSettingsState: async () => undefined,
      });
    });
    await waitFor(() => expect(result.initialLoadComplete()).toBe(true));
    const state = result.vmwareSettings;
    const old = state.connections()[0];
    state.openEditDialog(old);
    state.updateForm({ password: 'synthetic-draft' });
    let release!: (value: VMwareConnection[]) => void;
    vi.mocked(VMwareAPI.listConnections).mockReturnValueOnce(
      new Promise((resolve) => {
        release = resolve;
      }),
    );
    const reading = state.loadConnections();
    const pending = vi.mocked(VMwareAPI.listConnections).mock.calls[1][0]!;
    setUser('reader-b');
    await waitFor(() => expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(3));
    expect(pending.signal?.aborted).toBe(true);
    expect(state.dialogOpen()).toBe(false);
    expect(state.form().password).toBe('');
    release([old]);
    await reading;
    await waitFor(() => expect(state.loading()).toBe(false));
    state.openEditDialog(old);
    expect(state.dialogOpen()).toBe(false);
    state.openEditDialog(state.connections()[0]);
    expect(state.form()).toMatchObject({
      hasStoredPassword: true,
      password: '',
      insecureSkipVerify: false,
      monitorVms: true,
      monitorHosts: false,
      monitorDatastores: true,
    });
  });
});
