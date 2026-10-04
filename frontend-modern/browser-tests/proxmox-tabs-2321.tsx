// Mount the production page and router; the browser runner replaces only the
// resource and replication API responses for #2321's PBS-only estate.
import { Route, Router } from '@solidjs/router';
import { render } from 'solid-js/web';
import { ProxmoxPageSurface } from '../src/features/proxmox/ProxmoxPageSurface';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { getGlobalWebSocketStore } from '../src/stores/websocket-global';
import '../src/index.css';

const requested = new URLSearchParams(window.location.search).get('tab');
const tab = requested === 'mail' || requested === 'backups' ? requested : 'overview';
window.history.replaceState(null, '', `/proxmox/${tab}`);

render(
  () => (
    <WebSocketContext.Provider value={getGlobalWebSocketStore()}>
      <DarkModeContext.Provider value={() => false}>
        <Router>
          <Route path="/proxmox/*" component={ProxmoxPageSurface} />
        </Router>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
