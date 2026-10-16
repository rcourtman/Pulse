// Actual page, queries, reader, tab admission and table; only HTTP observations
// are synthetic. No replication job, guest command or recovery action is run.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import { ProxmoxPageSurface } from '../src/features/proxmox/ProxmoxPageSurface';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { getGlobalWebSocketStore } from '../src/stores/websocket-global';
import '../src/index.css';

window.history.replaceState(null, '', '/proxmox/replication');

render(
  () => (
    <WebSocketContext.Provider value={getGlobalWebSocketStore()}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="min-h-screen bg-surface p-3 text-base-content">
          <h1 class="mb-3 text-base font-semibold">Replication inventory read verification</h1>
          <Router>
            <Route path="/proxmox/*" component={ProxmoxPageSurface} />
          </Router>
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
