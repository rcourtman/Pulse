// Safe local HTTP responses, real recovery reader and the existing Protection
// consumer. No appliance request, backup, replication or recovery is run.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { TrueNASProtectionTable } from '../src/features/truenas/TrueNASProtectionTable';
import { useRecoveryPoints } from '../src/hooks/useRecoveryPoints';
import '../src/index.css';

function Protection() {
  const state = useRecoveryPoints(() => ({ platform: 'truenas', page: 1, limit: 200 }));
  return (
    <section aria-label="TrueNAS protection activity">
      <button
        type="button"
        class="mb-3 min-h-11 rounded-md border border-border px-3 disabled:opacity-60"
        disabled={state.response.loading}
        onClick={() => void state.refetch()}
      >
        Refresh fixture read
      </button>
      <TrueNASProtectionTable
        points={state.points()}
        loading={state.response.loading}
        error={state.response.error}
        onRefresh={() => void state.refetch()}
        emptyIcon={<span />}
        emptyTitle="No TrueNAS protection activity"
        emptyDescription="ZFS snapshots and replication tasks appear here once the TrueNAS API reports snapshot or replication activity."
      />
    </section>
  );
}

function Fixture() {
  const [mounted, setMounted] = createSignal(true);
  return (
    <main class="mx-auto max-w-6xl space-y-4 p-3 text-base-content">
      <h1 class="text-xl font-semibold">TrueNAS · Protection</h1>
      <button
        type="button"
        class="min-h-11 rounded-md border border-border px-3"
        onClick={() => setMounted(!mounted())}
      >
        {mounted() ? 'Leave fixture tab' : 'Open fixture tab'}
      </button>
      <Show when={mounted()}>
        <Protection />
      </Show>
    </main>
  );
}

render(() => <Fixture />, document.getElementById('root')!);
