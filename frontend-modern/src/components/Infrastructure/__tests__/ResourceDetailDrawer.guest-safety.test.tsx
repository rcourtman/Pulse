import { createSignal, Suspense } from 'solid-js';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Resource } from '@/types/resource';
import { ResourceDetailDrawer } from '../ResourceDetailDrawer';
import { guestDiskDeferrals } from '@/components/Workloads/__fixtures__/guestDiskDeferrals';
import { resetAIRuntimeState, syncAIRuntimeSettings } from '@/stores/aiRuntimeState';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import * as discoveryApi from '@/api/discovery';
import { getShippedDocUrl } from '@/utils/docsLinks';

vi.mock('@/contexts/appRuntime', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/contexts/appRuntime')>()),
  useWebSocket: () => ({ activeAlerts: {} }),
}));

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
vi.mock('@/api/resources', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/resources')>()),
  ResourceAPI: {
    getFacetBundle: vi.fn(async () => ({
      capabilities: [],
      relationships: [],
      recentChanges: [],
      counts: {},
    })),
  },
}));
vi.mock('@/api/ai', () => ({ AIAPI: { getResourceIntelligence: vi.fn(async () => null) } }));
vi.mock('@/api/actionAudit', () => ({
  ActionAuditAPI: {
    listActionAudits: vi.fn(async () => ({ audits: [], count: 0, available: false })),
  },
}));

const vm = (patch: Partial<Resource> = {}): Resource => ({
  id: 'fixture:pve1:101',
  type: 'vm',
  name: 'Backup guest',
  displayName: 'Backup guest',
  platformId: 'fixture',
  status: 'online',
  sourceType: 'api',
  platformType: 'proxmox-pve',
  lastSeen: 1,
  proxmox: { nodeName: 'pve1', vmid: 101, guestAgentStatus: 'available' },
  discoveryTarget: { resourceType: 'vm', agentId: 'node-agent', resourceId: '101' },
  ...patch,
});
const mount = (value: () => Resource) =>
  render(() => (
    <Suspense>
      <ResourceDetailDrawer resource={value()} initialShowAccessContext />
    </Suspense>
  ));
const openAnalysis = async () => {
  fireEvent.click(await screen.findByRole('tab', { name: 'Manage' }));
  fireEvent.click(await screen.findByRole('button', { name: 'Open analysis' }));
  return within(screen.getByTestId('resource-access-analysis')).findByRole('button', {
    name: 'Run Discovery',
  });
};

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

describe('canonical drawer real Discovery safety wiring', () => {
  it('blocks every retained/current deferral reactively without remounting or dispatching a scan', async () => {
    const [value, setValue] = createSignal(vm({ proxmox: { lock: 'backup' } }));
    mount(value);
    const button = await openAnalysis();
    await waitFor(() => expect(button).toBeDisabled());
    expect(screen.getByTestId('resource-guest-read-precaution')).toBeVisible();
    expect(
      within(screen.getByTestId('resource-guest-read-precaution')).getByRole('link', {
        name: 'Backup safety guidance',
      }),
    ).toHaveAttribute('href', getShippedDocUrl('VM_DISK_MONITORING.md'));
    for (const [reason] of guestDiskDeferrals) {
      for (const diskStatusReason of [reason, `prev-${reason}`]) {
        setValue(vm({ proxmox: { diskStatusReason, guestAgentStatus: 'available' } }));
        await waitFor(() => expect(button).toBeDisabled());
        fireEvent.click(button);
      }
    }
    setValue(vm({ platformData: { proxmox: { backupInProgress: true } } }));
    await waitFor(() => expect(button).toBeDisabled());
    setValue(vm());
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.queryByTestId('resource-guest-read-precaution')).not.toBeInTheDocument();
    expect(
      within(screen.getByTestId('resource-access-analysis')).getByRole('button', {
        name: 'Run Discovery',
      }),
    ).toBe(button);
    expect(discoveryApi.triggerDiscovery).not.toHaveBeenCalled();
    fireEvent.click(button);
    await waitFor(() => expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1));
    expect(discoveryApi.triggerDiscovery).toHaveBeenCalledWith('vm', 'node-agent', '101', {
      force: true,
      hostname: 'Backup guest',
    });
  });

  it('keeps ordinary agent Discovery tab actions usable despite host-side PVE locks', async () => {
    mount(() =>
      vm({
        id: 'node-agent',
        type: 'agent',
        sourceType: 'agent',
        platformType: 'agent',
        sources: ['agent'],
        proxmox: { lock: 'backup' },
        discoveryTarget: { resourceType: 'agent', agentId: 'node-agent', resourceId: 'node-agent' },
      }),
    );
    fireEvent.click(await screen.findByRole('tab', { name: 'Discovery' }));
    const button = await screen.findByRole('button', { name: 'Run Discovery' });
    await waitFor(() => expect(button).toBeEnabled());
    expect(screen.queryByTestId('resource-guest-read-precaution')).not.toBeInTheDocument();
    fireEvent.click(button);
    await waitFor(() => expect(discoveryApi.triggerDiscovery).toHaveBeenCalledTimes(1));
  });
});
