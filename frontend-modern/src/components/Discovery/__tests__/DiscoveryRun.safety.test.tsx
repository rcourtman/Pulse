import { createSignal, Suspense } from 'solid-js';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { ResourceDiscovery } from '@/types/discovery';
import { DiscoveryTab } from '../DiscoveryTab';
import { useDiscoveryTabState } from '../useDiscoveryTabState';
import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import * as discoveryApi from '@/api/discovery';

vi.mock('@/api/discovery', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/discovery')>()),
  getDiscovery: vi.fn(async () => null),
  getDiscoveryInfo: vi.fn(async () => ({
    ai_provider: { provider: 'test', model: 'test', is_local: true, label: 'Local' },
    commands: [],
    command_categories: [],
  })),
  getConnectedAgents: vi.fn(async () => ({ count: 1, agents: [{ agent_id: 'node-agent' }] })),
  triggerDiscovery: vi.fn(async () => null),
}));

const reason = 'Discovery paused for this guest. Saved results remain available.';
const target = {
  resourceType: 'vm' as const,
  agentId: 'node-agent',
  resourceId: '100',
  hostname: 'guest',
};
const saved = {
  id: 'vm:node-agent:100',
  resource_type: 'vm',
  resource_id: '100',
  target_id: 'node-agent',
  hostname: 'guest',
  service_type: 'home-assistant',
  service_name: 'Saved service',
  category: 'home_automation',
  facts: [],
  config_paths: [],
  data_paths: [],
  log_paths: [],
  ports: [],
  user_notes: '',
  discovered_at: '2026-10-03T12:00:00Z',
  updated_at: '2026-10-03T12:00:00Z',
  confidence: 0.9,
} as ResourceDiscovery;

beforeEach(() => {
  resetAIRuntimeState();
  syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<typeof syncAIRuntimeSettings>[0]);
  vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(null);
  vi.mocked(discoveryApi.triggerDiscovery).mockResolvedValue(saved);
});
afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('manual discovery safety', () => {
  it.each([
    ['loading', () => new Promise<ResourceDiscovery | null>(() => undefined)],
    ['absent', async () => null],
    ['empty', async () => ({ ...saved, service_type: '', service_name: '', confidence: 0 })],
    ['saved', async () => saved],
  ] as const)(
    'blocks all run affordances with %s discovery while retaining passive reads',
    async (_name, lookup) => {
      vi.mocked(discoveryApi.getDiscovery).mockImplementation(lookup);
      const props = { ...target, runBlockReason: reason };
      render(() => <DiscoveryTab {...props} />);
      await waitFor(() => expect(discoveryApi.getDiscoveryInfo).toHaveBeenCalled());
      await waitFor(() => {
        const buttons = screen.getAllByRole('button', { name: /^(Run|Re-scan|Update) Discovery/ });
        for (const button of buttons) expect(button).toBeDisabled();
      });
      expect(screen.getByTestId('discovery-run-block')).toHaveTextContent(reason);
      expect(discoveryApi.getDiscovery).toHaveBeenCalledWith('vm', 'node-agent', '100');
      expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    },
  );

  it('keeps saved evidence mounted and restores only the explicit run action, not an automatic scan', async () => {
    vi.mocked(discoveryApi.getDiscovery).mockResolvedValue(saved);
    const [block, setBlock] = createSignal<string | null>(reason);
    const props = {
      ...target,
      get runBlockReason() {
        return block();
      },
      showManualRunAction: true,
    };
    render(() => (
      <Suspense>
        <DiscoveryTab {...props} />
      </Suspense>
    ));
    const button = await screen.findByRole('button', { name: 'Run Discovery' });
    await waitFor(() => expect(button).toBeDisabled());
    const evidence = await screen.findByText('Saved service');
    expect(evidence).toBeVisible();
    setBlock(null);
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.getByText('Saved service')).toBe(evidence);
    expect(screen.queryByTestId('discovery-run-block')).toBeNull();
    expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    fireEvent.click(button);
    await waitFor(() => expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1));
  });

  it('checks the current block inside the handler even when a caller retained the old enabled verdict', async () => {
    const [block, setBlock] = createSignal<string | null>(null);
    const props = {
      ...target,
      get runBlockReason() {
        return block();
      },
    };
    let state!: ReturnType<typeof useDiscoveryTabState>;
    render(() => {
      state = useDiscoveryTabState(props);
      return <span>state</span>;
    });
    await waitFor(() => expect(state.canTriggerDiscovery()).toBe(true));
    setBlock(reason);
    await state.handleTriggerDiscovery(true);
    expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    expect(state.isScanning()).toBe(false);
    expect(state.scanSuccess()).toBe(false);
  });

  it('does not cancel or hide an in-flight scan when a new snapshot blocks further runs', async () => {
    const finishes: Array<(value: ResourceDiscovery) => void> = [];
    vi.mocked(discoveryApi.triggerDiscovery).mockImplementation(
      () =>
        new Promise((resolve) => {
          finishes.push(resolve);
        }),
    );
    const [block, setBlock] = createSignal<string | null>(null);
    const props = {
      ...target,
      get runBlockReason() {
        return block();
      },
    };
    let state!: ReturnType<typeof useDiscoveryTabState>;
    render(() => {
      state = useDiscoveryTabState(props);
      return <span>state</span>;
    });
    await waitFor(() => expect(state.canTriggerDiscovery()).toBe(true));
    const original = state.handleTriggerDiscovery(true);
    expect(state.isScanning()).toBe(true);
    setBlock(reason);
    const second = state.handleTriggerDiscovery(true);
    try {
      expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1);
      expect(state.isScanning()).toBe(true);
    } finally {
      for (const finish of finishes) finish(saved);
      await Promise.all([original, second]);
    }
    expect(state.isScanning()).toBe(false);
    expect(state.discovery()).toBe(saved);
    expect(state.scanSuccess()).toBe(true);
    setBlock(null);
    expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1);
  });
});
