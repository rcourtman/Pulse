// Reads captured production resource HTTP bytes through the ordinary decoder,
// TrueNAS systems table, its inline drawer and the ordinary chart HTTP client.
// Only the local server data is synthetic; no appliance operation is performed.
import { createMemo } from 'solid-js';
import { useUnifiedResources } from '../src/hooks/useUnifiedResources';
import { TrueNASSystemsTable } from '../src/features/truenas/TrueNASSystemsTable';
import { renderInAppRuntime } from './fixtureRuntime';
import '../src/index.css';

function Fixture() {
  const state = useUnifiedResources({
    query: 'source=truenas&type=agent,storage,physical_disk',
    realtimeEnabled: () => false,
  });
  const systems = createMemo(() => state.resources.filter((resource) => resource.type === 'agent'));
  return (
    <main class="mx-auto max-w-6xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-lg font-semibold">CORE reporting consumer verification</h1>
      <p class="text-xs text-muted">Synthetic CORE REST observations. No appliance or guest is contacted.</p>
      <TrueNASSystemsTable
        systems={systems()}
        scope={state.resources}
        emptyIcon={<span />}
        emptyTitle="Loading CORE fixture"
        emptyDescription="Reading the captured canonical resource response."
        showToolbar={false}
      />
    </main>
  );
}
renderInAppRuntime(Fixture);
