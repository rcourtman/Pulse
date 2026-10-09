// Real Backups server table/drawer/CSS; synthetic, credential-free resources only.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { ProxmoxBackupServersTable } from '../src/features/proxmox/ProxmoxBackupServersTable';
import type { Resource } from '../src/types/resource';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import '../src/index.css';

// Explicit offline context, not a live WebSocket or automatic discovery probe.
syncAIRuntimeSettings({ discovery_enabled: false });
const server = (id: string, name: string, customUrl?: string): Resource => ({
  id,
  type: 'pbs',
  name,
  displayName: name,
  platformId: id,
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  sources: ['pbs'],
  status: 'online',
  lastSeen: Date.now(),
  customUrl,
  pbs: {
    instanceId: id,
    nodeName: 'host.example',
    linkedAgentId: 'agent-host',
    connectionHealth: 'healthy',
    version: '3.2.1',
    datastores: [
      { name: 'archive', total: 1000, used: 400, available: 600, status: 'available' },
      { name: 'main', total: 1000, used: 200, available: 800, status: 'available' },
    ],
  },
});
const initial = [
  server('pbs-east', 'pbs-main', 'https://east.example:8007/proxy/'),
  server('pbs-west', 'pbs-main', 'http://[2001:db8::2]:8007/'),
  server(
    'pbs-long',
    'backup-server-with-a-very-long-readable-identity',
    'https://long.example:8007',
  ),
  server('pbs-invalid', 'unsafe-url', 'javascript:alert(1)'),
  server('pbs-unset', 'no-url'),
];
const host: Resource = {
  id: 'host',
  type: 'agent',
  name: 'pbs-main',
  displayName: 'pbs-main',
  platformId: 'host',
  platformType: 'proxmox-pbs',
  sourceType: 'agent',
  status: 'online',
  lastSeen: Date.now(),
  agent: { agentId: 'agent-host', hostname: 'host.example' },
  customUrl: 'https://WRONG-HOST.example:9000',
};
function Fixture() {
  const [servers, setServers] = createSignal([...initial, host]);
  (window as any).__pbsWeb = {
    update: (id: string, url: string) =>
      setServers((current) => current.map((r) => (r.id === id ? { ...r, customUrl: url } : r))),
    reverse: () =>
      setServers((current) =>
        [...current].reverse().map((r) => ({
          ...r,
          pbs: r.pbs ? { ...r.pbs, datastores: [...r.pbs.datastores!].reverse() } : undefined,
        })),
      ),
  };
  return (
    <main class="min-h-screen bg-surface p-3 text-base-content">
      <h1 class="mb-3 text-base font-semibold">Backup servers</h1>
      <ProxmoxBackupServersTable servers={servers()} />
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
