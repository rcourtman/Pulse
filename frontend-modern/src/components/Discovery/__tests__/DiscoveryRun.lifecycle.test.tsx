import { createSignal } from 'solid-js';
import { cleanup, render, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ResourceDiscovery } from '@/types/discovery';
import { eventBus } from '@/stores/events';
import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import * as api from '@/api/discovery';
import { useDiscoveryTabState } from '../useDiscoveryTabState';

vi.mock('@/api/discovery', () => ({
  getDiscovery: vi.fn(async () => null),
  getDiscoveryInfo: vi.fn(async () => ({
    ai_provider: { provider: 'test', model: 'test', is_local: true, label: 'Local' },
  })),
  getConnectedAgents: vi.fn(async () => ({ count: 1, agents: [{ agent_id: 'node-agent' }] })),
  triggerDiscovery: vi.fn(),
  updateDiscoveryNotes: vi.fn(async () => undefined),
}));

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

const saved = (id = '100'): ResourceDiscovery => ({
  id: `vm:node-agent:${id}`,
  resource_type: 'vm',
  resource_id: id,
  target_id: 'node-agent',
  hostname: `guest-${id}`,
  service_type: 'home-assistant',
  service_name: `Saved service ${id}`,
  service_version: 'fixture',
  category: 'home_automation',
  cli_access: '',
  user_secrets: {},
  ai_reasoning: 'Synthetic saved evidence',
  scan_duration: 0,
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  ports: [],
  user_notes: '',
  discovered_at: '2026-10-03T12:00:00Z',
  updated_at: '2026-10-03T12:00:00Z',
  confidence: 0.9,
});

async function mount(initialAgent = 'node-agent') {
  const [id, setId] = createSignal('100');
  const [agent, setAgent] = createSignal(initialAgent);
  let state!: ReturnType<typeof useDiscoveryTabState>;
  const view = render(() => {
    state = useDiscoveryTabState({
      resourceType: 'vm',
      get agentId() {
        return agent();
      },
      get resourceId() {
        return id();
      },
      get hostname() {
        return `guest-${id()}`;
      },
    });
    return <span>Discovery lifecycle</span>;
  });
  await waitFor(() => expect(state.discoveryInfo.loading).toBe(false));
  await waitFor(() => expect(state.discovery.loading).toBe(false));
  return { state, setId, setAgent, dispose: view.unmount };
}

beforeEach(() => {
  vi.resetAllMocks();
  resetAIRuntimeState();
  syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<typeof syncAIRuntimeSettings>[0]);
  vi.mocked(api.getDiscovery).mockImplementation(async (_type, _agent, id) => saved(id));
  vi.mocked(api.getDiscoveryInfo).mockResolvedValue({
    ai_provider: { provider: 'test', model: 'test', is_local: true, label: 'Local' },
  });
  vi.mocked(api.getConnectedAgents).mockResolvedValue({ count: 0, agents: [] });
  vi.mocked(api.triggerDiscovery).mockResolvedValue(saved());
});
afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.useRealTimers();
});

describe('Discovery outcome ownership', () => {
  it.each(['success', 'failure'] as const)(
    'does not apply an old target HTTP %s to the new guest or its in-flight run',
    async (outcome) => {
      const old = deferred<ResourceDiscovery>();
      const current = deferred<ResourceDiscovery>();
      vi.mocked(api.triggerDiscovery)
        .mockReturnValueOnce(old.promise)
        .mockReturnValueOnce(current.promise);
      const { state, setId } = await mount();
      const first = state.handleTriggerDiscovery(true);
      setId('101');
      await waitFor(() => expect(state.discovery()?.resource_id).toBe('101'));
      const second = state.handleTriggerDiscovery(true);
      if (outcome === 'success') old.resolve(saved());
      else old.reject(new Error('Old guest execution paused'));
      await first;
      expect(state.discovery()?.resource_id).toBe('101');
      expect(state.isScanning()).toBe(true);
      expect(state.scanError()).toBeNull();
      expect(state.scanSuccess()).toBe(false);
      current.resolve(saved('101'));
      await second;
      expect(state.scanSuccess()).toBe(true);
      expect(api.triggerDiscovery).toHaveBeenNthCalledWith(2, 'vm', 'node-agent', '101', {
        force: true,
        hostname: 'guest-101',
      });
    },
  );

  it('invalidates an old run even when the operator leaves and returns to the same target', async () => {
    const old = deferred<ResourceDiscovery>();
    vi.mocked(api.triggerDiscovery).mockReturnValueOnce(old.promise);
    const { state, setId } = await mount();
    const first = state.handleTriggerDiscovery(true);
    setId('101');
    await waitFor(() => expect(state.discovery()?.resource_id).toBe('101'));
    setId('100');
    await waitFor(() => expect(state.discovery()?.resource_id).toBe('100'));
    old.resolve({ ...saved(), service_name: 'Old run result' });
    await first;
    expect(state.discovery()?.service_name).toBe('Saved service 100');
    expect(state.scanSuccess()).toBe(false);
  });

  it('does not resurrect a run after Discovery was disabled and re-enabled', async () => {
    const old = deferred<ResourceDiscovery>();
    vi.mocked(api.triggerDiscovery).mockReturnValueOnce(old.promise);
    const { state } = await mount();
    const first = state.handleTriggerDiscovery(true);
    syncAIRuntimeSettings({ discovery_enabled: false } as Parameters<
      typeof syncAIRuntimeSettings
    >[0]);
    await waitFor(() => expect(state.canTriggerDiscovery()).toBe(false));
    syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<
      typeof syncAIRuntimeSettings
    >[0]);
    await waitFor(() => expect(state.canTriggerDiscovery()).toBe(true));
    old.resolve({ ...saved(), service_name: 'Disabled run result' });
    await first;
    expect(state.discovery()?.service_name).toBe('Saved service 100');
    expect(state.scanSuccess()).toBe(false);
  });

  it('ignores HTTP completion after disposal without scheduling another success timer', async () => {
    const old = deferred<ResourceDiscovery>();
    vi.mocked(api.triggerDiscovery).mockReturnValueOnce(old.promise);
    const { state, dispose } = await mount();
    vi.useFakeTimers();
    const first = state.handleTriggerDiscovery(true);
    dispose();
    old.resolve(saved());
    await first;
    expect(state.scanSuccess()).toBe(false);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not leave a spinner running when a retained handler has no target agent', async () => {
    const { state } = await mount('');
    await state.handleTriggerDiscovery(true);
    expect(api.triggerDiscovery).not.toHaveBeenCalled();
    expect(state.scanError()).toBe('Agent identifier unavailable for discovery');
    expect(state.isScanning()).toBe(false);
    expect(state.scanSuccess()).toBe(false);
  });

  it('does not let an earlier success timer clear the success of a later explicit run', async () => {
    const { state } = await mount();
    vi.useFakeTimers();
    await state.handleTriggerDiscovery(true);
    await vi.advanceTimersByTimeAsync(1500);
    await state.handleTriggerDiscovery(true);
    await vi.advanceTimersByTimeAsync(600);
    expect(state.scanSuccess()).toBe(true);
    await vi.advanceTimersByTimeAsync(1400);
    expect(state.scanSuccess()).toBe(false);
  });

  it('does not leave the previous HTTP success visible forever after a later completion event', async () => {
    const { state } = await mount();
    vi.useFakeTimers();
    await state.handleTriggerDiscovery(true);
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'completed',
    });
    await vi.advanceTimersByTimeAsync(2500);
    expect(state.scanSuccess()).toBe(false);
    expect(state.discovery()?.resource_id).toBe('100');
  });

  it.each(['failed', 'completed'] as const)(
    'retains a background %s-with-error outcome and the saved evidence, not silent success',
    async (status) => {
      const { state } = await mount();
      const before = vi.mocked(api.getDiscovery).mock.calls.length;
      vi.useFakeTimers();
      eventBus.emit('ai_discovery_progress', {
        resource_id: 'vm:node-agent:100',
        status,
        error: 'Guest execution paused: backup lock observed',
        current_step: 'Guest execution paused',
      });
      await vi.advanceTimersByTimeAsync(600);
      expect(state.scanError()).toBe('Guest execution paused: backup lock observed');
      expect(state.scanSuccess()).toBe(false);
      expect(state.discovery()?.service_name).toBe('Saved service 100');
      expect(api.getDiscovery).toHaveBeenCalledTimes(before);
      expect(api.triggerDiscovery).not.toHaveBeenCalled();
    },
  );

  it('shows background progress without claiming completion of a manual HTTP request', async () => {
    const http = deferred<ResourceDiscovery>();
    vi.mocked(api.triggerDiscovery).mockReturnValueOnce(http.promise);
    const { state } = await mount();
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'running',
      current_step: 'Collecting guest evidence',
    });
    expect(state.isScanning()).toBe(true);
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'completed',
    });
    expect(state.isScanning()).toBe(false);
    const first = state.handleTriggerDiscovery(true);
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'completed',
    });
    expect(state.isScanning()).toBe(true);
    expect(state.scanSuccess()).toBe(false);
    http.resolve(saved());
    await first;
    expect(state.scanSuccess()).toBe(true);
  });

  it('drops a delayed background refresh when its target changed before dispatch', async () => {
    const { state, setId } = await mount();
    vi.useFakeTimers();
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'completed',
    });
    setId('101');
    await Promise.resolve();
    const before = vi.mocked(api.getDiscovery).mock.calls.length;
    await vi.advanceTimersByTimeAsync(600);
    expect(api.getDiscovery).toHaveBeenCalledTimes(before);
    expect(state.discovery()?.resource_id).toBe('101');
    expect(api.triggerDiscovery).not.toHaveBeenCalled();
  });

  it('does not apply a delayed background read to a newer manual result on the same guest', async () => {
    const refresh = deferred<ResourceDiscovery | null>();
    const { state } = await mount();
    vi.mocked(api.getDiscovery).mockReturnValueOnce(refresh.promise);
    vi.useFakeTimers();
    eventBus.emit('ai_discovery_progress', {
      resource_id: 'vm:node-agent:100',
      status: 'completed',
    });
    await vi.advanceTimersByTimeAsync(500);
    vi.mocked(api.triggerDiscovery).mockResolvedValueOnce({
      ...saved(),
      service_name: 'New manual result',
    });
    await state.handleTriggerDiscovery(true);
    refresh.resolve({ ...saved(), service_name: 'Old background result' });
    await Promise.resolve();
    expect(state.discovery()?.service_name).toBe('New manual result');
    expect(state.scanSuccess()).toBe(true);
  });

  it.each([401, 403])(
    'withdraws saved evidence when a completion refresh is denied with %s',
    async (status) => {
      const { state } = await mount();
      vi.mocked(api.getDiscovery).mockRejectedValueOnce(
        Object.assign(new Error('Discovery details unavailable.'), { status }),
      );
      vi.useFakeTimers();
      eventBus.emit('ai_discovery_progress', {
        resource_id: 'vm:node-agent:100',
        status: 'completed',
      });
      await vi.advanceTimersByTimeAsync(500);
      expect(state.discovery()).toBeNull();
      expect(state.scanSuccess()).toBe(false);
      expect(api.triggerDiscovery).not.toHaveBeenCalled();
    },
  );

  it('keeps saved evidence without success when HTTP supplies no result', async () => {
    vi.mocked(api.triggerDiscovery).mockResolvedValueOnce(null as unknown as ResourceDiscovery);
    const { state } = await mount();
    await state.handleTriggerDiscovery(true);
    expect(state.scanError()).toBe('Discovery returned no saved result.');
    expect(state.scanSuccess()).toBe(false);
    expect(state.discovery()?.service_name).toBe('Saved service 100');
    expect(state.isScanning()).toBe(false);
  });

  it('does not close another guest’s notes editor when an old save finishes', async () => {
    const save = deferred<ResourceDiscovery>();
    vi.mocked(api.updateDiscoveryNotes).mockReturnValueOnce(save.promise);
    const { state, setId } = await mount();
    state.startEditingNotes();
    state.setNotesText('Original guest notes');
    const first = state.handleSaveNotes();
    setId('101');
    await waitFor(() => expect(state.discovery()?.resource_id).toBe('101'));
    state.startEditingNotes();
    state.setNotesText('New guest draft');
    save.resolve(saved());
    await first;
    expect(state.editingNotes()).toBe(true);
    expect(state.notesText()).toBe('New guest draft');
    expect(api.updateDiscoveryNotes).toHaveBeenCalledWith('vm', 'node-agent', '100', {
      user_notes: 'Original guest notes',
    });
  });
});
