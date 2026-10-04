import { createSignal, Suspense } from 'solid-js';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { GuestDrawer } from '../GuestDrawer';
import { guestDiskDeferrals } from '../__fixtures__/guestDiskDeferrals';
import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
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
vi.mock('@/api/charts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/charts')>()),
  ChartsAPI: {
    getMetricsHistory: vi.fn(async () => ({ metrics: {}, start: 1, end: 2, source: 'store' })),
  },
}));
vi.mock('@/stores/license', () => ({
  isRangeLocked: () => false,
  loadRuntimeCapabilities: vi.fn(),
  maxHistoryDays: () => 90,
}));

const guest = (patch: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  ({
    id: 'inst:node:100',
    instance: 'inst',
    node: 'node',
    vmid: 100,
    name: 'Backup guest',
    type: 'qemu',
    status: 'running',
    cpu: 0.25,
    cpus: 2,
    memory: { total: 1024, used: 512, free: 512, usage: 50 },
    disk: { total: 1024, used: 512, free: 512, usage: 50 },
    networkIn: 1,
    networkOut: 2,
    diskRead: 3,
    diskWrite: 4,
    uptime: 100,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: '2026-10-04T12:00:00Z',
    guestAgentStatus: 'available',
    discoveryTarget: { resourceType: 'vm', agentId: 'node-agent', resourceId: '100' },
    ...patch,
  }) as WorkloadGuest;

beforeEach(() => {
  resetAIRuntimeState();
  syncAIRuntimeSettings({ discovery_enabled: true } as Parameters<typeof syncAIRuntimeSettings>[0]);
  resetCreateNonSuspendingQueryCacheForTest();
});
afterEach(() => {
  cleanup();
  resetAIRuntimeState();
  vi.clearAllMocks();
});

describe('guest drawer and real Discovery safety wiring', () => {
  it.each([...guestDiskDeferrals.flatMap(([reason]) => [reason, `prev-${reason}`])])(
    'does not offer a live scan for %s, even with an assigned node agent and available metadata',
    async (reason) => {
      render(() => (
        <Suspense>
          <GuestDrawer guest={guest({ diskStatusReason: reason })} onClose={vi.fn()} />
        </Suspense>
      ));
      fireEvent.click(await screen.findByRole('tab', { name: 'Discovery' }));
      const button = await screen.findByRole('button', { name: 'Run Discovery' });
      await waitFor(() => expect(button).toBeDisabled());
      fireEvent.click(button);
      expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
      expect(screen.getByTestId('guest-read-precaution')).toBeVisible();
    },
  );

  it.each([{ lock: 'backup' }, { backupInProgress: true }, { guestAgentStatus: 'deferred' }])(
    'guards the native operation signal %j without fabricating a disk read reason',
    async (patch) => {
      render(() => (
        <Suspense>
          <GuestDrawer guest={guest(patch)} onClose={vi.fn()} />
        </Suspense>
      ));
      fireEvent.click(await screen.findByRole('tab', { name: 'Discovery' }));
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Run Discovery' })).toBeDisabled(),
      );
      expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    },
  );

  it('updates the existing same-guest button without remounting the tab or automatically scanning', async () => {
    const [value, setValue] = createSignal(guest({ lock: 'backup' }));
    render(() => (
      <Suspense>
        <GuestDrawer guest={value()} onClose={vi.fn()} />
      </Suspense>
    ));
    fireEvent.click(await screen.findByRole('tab', { name: 'Discovery' }));
    const button = await screen.findByRole('button', { name: 'Run Discovery' });
    await waitFor(() => expect(button).toBeDisabled());
    setValue(guest());
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.getByRole('button', { name: 'Run Discovery' })).toBe(button);
    expect(screen.getByRole('tab', { name: 'Discovery' })).toHaveAttribute('aria-selected', 'true');
    expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    setValue(guest({ diskStatusReason: 'prev-agent-timeout' }));
    await waitFor(() => expect(button).toBeDisabled());
    expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
  });

  it.each([
    { type: 'qemu' },
    {
      type: 'lxc',
      lock: 'backup',
      diskStatusReason: 'prev-agent-timeout',
      discoveryTarget: {
        resourceType: 'system-container',
        agentId: 'node-agent',
        resourceId: '100',
      },
    },
    {
      type: 'vm',
      platformType: 'vmware',
      guestAgentStatus: '',
      lock: '',
      discoveryTarget: { resourceType: 'vm', agentId: 'node-agent', resourceId: '100' },
    },
  ] as const)(
    'preserves ordinary manual scans outside the Proxmox guest safety condition %j',
    async (patch) => {
      render(() => (
        <Suspense>
          <GuestDrawer guest={guest(patch)} onClose={vi.fn()} />
        </Suspense>
      ));
      fireEvent.click(await screen.findByRole('tab', { name: 'Discovery' }));
      const button = await screen.findByRole('button', { name: 'Run Discovery' });
      await waitFor(() => expect(button).toBeEnabled());
      fireEvent.click(button);
      await waitFor(() => expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1));
    },
  );
});
