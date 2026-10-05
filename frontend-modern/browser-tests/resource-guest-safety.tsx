// Real canonical drawer/state/Discovery clients; synthetic provider snapshots.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource, ResourceProxmoxMeta } from '../src/types/resource';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: true });
const healthy: ResourceProxmoxMeta = {
  nodeName: 'pve-a',
  vmid: 101,
  guestAgentStatus: 'available',
  lock: '',
  diskStatusReason: '',
};
const resource = (
  evidence: ResourceProxmoxMeta,
  legacy = false,
  type: Resource['type'] = 'vm',
): Resource => ({
  id: 'fixture:pve-a:101',
  type,
  name: 'Backup guest',
  displayName: 'Backup guest',
  platformId: 'fixture',
  platformType: type === 'agent' ? 'agent' : 'proxmox-pve',
  platformScopes: type === 'agent' ? ['agent'] : ['proxmox-pve'],
  sources: type === 'agent' ? ['agent'] : ['proxmox'],
  sourceType: type === 'agent' ? 'agent' : 'api',
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 10 },
  memory: { current: 25, used: 1024, total: 4096 },
  discoveryTarget: {
    resourceType: type === 'agent' ? 'agent' : 'vm',
    agentId: 'fixture-node-agent',
    resourceId: '101',
  },
  proxmox: legacy ? healthy : { ...healthy, ...evidence },
  platformData: legacy ? { proxmox: evidence } : undefined,
});
function Fixture() {
  const [value, setValue] = createSignal(resource({ lock: 'backup' }));
  (window as any).__resourceGuestSafety = {
    update: (evidence: ResourceProxmoxMeta, legacy = false, type: Resource['type'] = 'vm') =>
      setValue(resource(evidence, legacy, type)),
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Canonical guest safety evidence</h1>
      <p class="text-xs text-muted">Synthetic local responses. No guest check or backup is sent.</p>
      <ResourceDetailDrawer resource={value()} initialShowAccessContext />
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Fixture />
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
