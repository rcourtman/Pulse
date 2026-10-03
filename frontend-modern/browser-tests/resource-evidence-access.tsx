import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { ResourceDetailDrawer } from '../src/components/Infrastructure/ResourceDetailDrawer';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

// Synthetic snapshots and HTTP responses; production drawer, query/client and CSS.
syncAIRuntimeSettings({ discovery_enabled: false });
const resource: Resource = {
  id: 'pbs-a',
  name: 'PBS A',
  displayName: 'PBS A',
  type: 'pbs',
  platformId: 'pbs-a',
  platformType: 'proxmox-pbs',
  sourceType: 'api',
  status: 'online',
  lastSeen: Date.now(),
  cpu: { current: 12 },
  recentChanges: [
    {
      id: 'embedded-event',
      resourceId: 'pbs-a',
      observedAt: '2026-10-03T03:00:00Z',
      kind: 'restart',
      sourceType: 'platform_event',
      confidence: 'high',
      actor: 'Embedded snapshot actor',
    },
  ],
  facetCounts: { recentChanges: 7 },
};
const [mounted, setMounted] = createSignal(true);
render(
  () => (
    <WebSocketContext.Provider value={{ state: { pmg: [] }, connected: () => true } as any}>
      <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
        <main class="min-h-screen space-y-4 bg-surface p-4 text-base-content">
          <h1>Resource evidence access verification</h1>
          <p>
            Synthetic PBS evidence; no appliance, release or installed authorisation acceptance.
          </p>
          <button
            class="min-h-11 rounded border border-border px-3"
            onClick={() => {
              setMounted(false);
              setTimeout(() => setMounted(true), 0);
            }}
          >
            Remount details
          </button>
          <Show when={mounted()}>
            <ResourceDetailDrawer resource={resource} />
          </Show>
        </main>
      </DarkModeContext.Provider>
    </WebSocketContext.Provider>
  ),
  document.getElementById('root')!,
);
