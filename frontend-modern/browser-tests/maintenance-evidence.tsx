// The full production resource drawer, transport/query, shared clock and CSS.
// Only snapshots and HTTP responses are synthetic. No real operation runs.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

syncAIRuntimeSettings({ discovery_enabled: false });
const [resource, setResource] = createSignal<Resource>({
  id: 'pbs-a',
  name: 'Recorded backup server',
  displayName: 'Recorded backup server',
  type: 'pbs',
  platformId: 'pbs-a',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  cpu: { current: 12 },
  lastSeen: Date.parse('2026-10-11T04:00:00Z'),
});
(window as any).__maintenanceEvidence = {
  withdraw: () =>
    setResource((current) => ({ ...current, id: 'empty-resource', name: 'No reports' })),
  select: (id: string) =>
    setResource((current) => ({
      ...current,
      id,
      name: 'Different resource',
      displayName: 'Different resource',
    })),
};

render(
  () => (
    <WebSocketContext.Provider
      value={{ state: { pmg: [] }, activeAlerts: {}, connected: () => true } as any}
    >
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="mx-auto min-h-screen max-w-5xl space-y-3 bg-surface p-3 text-base-content">
          <h1 class="text-base font-semibold">Recorded maintenance evidence</h1>
          <p class="text-xs text-muted">
            Synthetic local responses. No operation reaches a server.
          </p>
          <ResourceDetailDrawer resource={resource()} />
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
