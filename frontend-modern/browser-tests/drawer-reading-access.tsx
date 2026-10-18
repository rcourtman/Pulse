// Existing production API/row/drawers/help/Docs, supplied only synthetic observations.
// No guest-agent, backup, update, permission or native recovery operation is made.
import { createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { NodeDrawer } from '../src/components/Workloads/NodeDrawer';
import Docs from '../src/pages/Docs';
import {
  GUEST_COLUMNS,
  getGuestColumnWidthStyle,
  getWorkloadTableLayoutModeForContainer,
  getWorkloadVisibleColumnsForLayout,
} from '../src/components/Workloads/guestRowModel';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { Node } from '../src/types/api';
import '../src/index.css';

const makeNode = (): Node => ({
  id: 'lab-pve-a',
  name: 'pve-a',
  instance: 'lab',
  status: 'online',
  type: 'node',
  cpu: 0.1,
  memory: { total: 1024, used: 256, free: 768, usage: 25 },
  disk: { total: 1024, used: 256, free: 768, usage: 25 },
  uptime: 3600,
  loadAverage: [0.1, 0.2, 0.1],
  kernelVersion: '6.8.12',
  pveVersion: 'pve-manager/9.2.21',
  cpuInfo: { model: 'Synthetic CPU', cores: 4, sockets: 1, mhz: '2400' },
  lastSeen: '2026-10-12T22:00:00Z',
  connectionHealth: 'healthy',
  pendingUpdatesStatus: 'unavailable',
  pendingUpdatesReason: 'permission_denied',
});
function Fixture() {
  const state = useWorkloads(() => true);
  const [expanded, setExpanded] = createSignal(false);
  const [node, setNode] = createSignal(makeNode());
  const primary = () => state.workloads()[0]!;
  const layout = getWorkloadTableLayoutModeForContainer(window.innerWidth - 24);
  const requested = new Set(['name', 'cpu', 'memory', 'disk', 'uptime']);
  const columns = getWorkloadVisibleColumnsForLayout(
    GUEST_COLUMNS.filter((c) => requested.has(c.id)),
    layout,
  );
  const ids = columns.map((c) => c.id);
  (window as any).__drawerReading = {
    refetch: state.refetch,
    node: (change: Partial<Node>) => setNode((prior) => ({ ...prior, ...change })),
    evidence: () => ({
      id: primary().id,
      type: primary().type,
      osName: primary().osName,
      osVersion: primary().osVersion,
    }),
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Drawer evidence access</h1>
      <p class="text-xs text-muted">Synthetic readings; no guest commands or update actions.</p>
      <Show when={primary()}>
        <div class="table-scroll-shell overflow-x-auto">
          <table class="w-full platform-table workload-table table-fixed">
            <colgroup>
              <For each={ids}>
                {(id) => (
                  <col
                    style={getGuestColumnWidthStyle(id, window.innerWidth <= 768, layout, ids)}
                  />
                )}
              </For>
            </colgroup>
            <thead>
              <tr>
                <For each={columns}>{(column) => <th scope="col">{column.label}</th>}</For>
              </tr>
            </thead>
            <tbody>
              <GuestRow
                guest={primary()}
                visibleColumnIds={ids}
                workloadTableLayoutMode={layout}
                isExpanded={expanded()}
                onClick={() => setExpanded((prior) => !prior)}
              />
            </tbody>
          </table>
        </div>
        <Show when={expanded()}>
          <section aria-label="Guest details">
            <GuestDrawer guest={primary()} onClose={() => setExpanded(false)} />
          </section>
        </Show>
      </Show>
      <section aria-label="Node details">
        <NodeDrawer node={node()} />
      </section>
    </main>
  );
}
const WithDarkMode = (props: { children: any }) => (
  <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
    {props.children}
  </DarkModeContext.Provider>
);
render(
  () => (
    <Router>
      <Route
        path="/docs/*docPath"
        component={() => (
          <WithDarkMode>
            <main class="min-h-screen bg-surface p-3 text-base-content">
              <Docs />
            </main>
          </WithDarkMode>
        )}
      />
      <Route
        path="*"
        component={() => (
          <WithDarkMode>
            <Fixture />
          </WithDarkMode>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
