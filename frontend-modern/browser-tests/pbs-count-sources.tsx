// Production Backups page, reader and row producer. Resource observations are
// synthetic; PBS entries arrive through the real HTTP inventory decoder.
import { createMemo, createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import type { Resource } from '../src/types/resource';
import { canonicalizeRealtimeResource } from '../src/utils/resourceStateAdapters';
import { ProxmoxBackupsTable } from '../src/features/proxmox/ProxmoxBackupsTable';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import '../src/index.css';

const server = (name: string, instanceId = `pbs-${name}`): Resource => ({
  id: instanceId,
  type: 'pbs',
  name,
  displayName: name,
  platformId: instanceId,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.parse('2026-10-19T12:00:00Z'),
  pbs: {
    instanceId,
    connectionHealth: 'healthy',
    version: '4.0.0',
    datastores: [
      { name: 'tank', total: 10000, used: 4000, available: 6000, status: 'available' },
      { name: 'offsite', total: 20000, used: 2000, available: 18000, status: 'available' },
    ],
  },
});
function Fixture() {
  const [ambiguous, setAmbiguous] = createSignal(false);
  Object.assign(window, { __pbsCountSources: { setAmbiguous } });
  const canonical = new URLSearchParams(location.search).get('transport') === 'canonical';
  const servers = createMemo(() => {
    const resources = ambiguous()
      ? [server('duplicate', 'pbs-one'), server('duplicate', 'pbs-two')]
      : [server('backup'), server('pbs-backup'), server('archive', 'legacy-archive')];
    return resources.map((resource) => {
      if (!canonical) return resource;
      const { pbs, ...base } = resource;
      return canonicalizeRealtimeResource({ ...base, platformData: { sources: ['pbs'], pbs } });
    });
  });
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="mb-3 text-base font-semibold">Backups</h1>
      <ProxmoxBackupsTable emptyIcon={<span />} workloads={[]} servers={servers()} />
    </main>
  );
}
render(
  () => (
    <WebSocketContext.Provider
      value={{ state: { pmg: [] }, activeAlerts: {}, connected: () => false } as any}
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <Router>
          <Route path="/*" component={Fixture} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
