import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DiscoveryTab } from '../src/components/Discovery/DiscoveryTab';
import { eventBus } from '../src/stores/events';
import { syncAIRuntimeSettings } from '../src/stores/aiRuntimeState';
import type { DiscoveryProgress } from '../src/types/discovery';
import '../src/index.css';

declare global {
  interface Window {
    discoveryOwnership: {
      target: (id: string) => void;
      progress: (progress: DiscoveryProgress) => void;
      enabled: (enabled: boolean) => void;
    };
  }
}

function Fixture() {
  const [id, setId] = createSignal('101');
  const enabled = (discovery_enabled: boolean) =>
    syncAIRuntimeSettings({ discovery_enabled } as Parameters<typeof syncAIRuntimeSettings>[0]);
  enabled(true);
  window.discoveryOwnership = {
    target: setId,
    progress: (progress) => eventBus.emit('ai_discovery_progress', progress),
    enabled,
  };
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Discovery outcome ownership evidence</h1>
      <p class="text-xs text-muted">Synthetic guest responses. No guest command is sent.</p>
      <h2 class="text-sm font-medium">Guest {id()}</h2>
      <DiscoveryTab
        resourceType="vm"
        agentId="fixture-node-agent"
        resourceId={id()}
        canonicalResourceId={`fixture-pve1-${id()}`}
        hostname={`guest-${id()}`}
        showManualRunAction
      />
    </main>
  );
}

render(
  () => (
    <Router>
      <Route path="/*" component={Fixture} />
    </Router>
  ),
  document.getElementById('root')!,
);
