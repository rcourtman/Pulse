// Real monitor-produced projections, production Workloads adapter, row and
// drawer. Only the upstream HTTP estate and phase controls are synthetic.
import { createSignal, For, onMount, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { mergeCanonicalResourceSnapshot } from '../src/utils/resourceStateAdapters';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const phases = [
  ['suppressed', 'Before migration'],
  ['migrated', 'After migration'],
  ['refreshed', 'Refresh network/version'],
  ['returned', 'Return migration'],
];

function Fixture() {
  const [projections, setProjections] = createSignal<Record<string, Resource[]>>({});
  const [snapshot, setSnapshot] = createSignal<Resource[]>([]);
  const [phase, setPhase] = createSignal('suppressed');
  const [expanded, setExpanded] = createSignal(false);
  const state = useWorkloads(() => true, { resourceSnapshot: snapshot });
  const guest = () => state.workloads()[0]!;
  const apply = (next: string) => {
    setSnapshot((previous) => mergeCanonicalResourceSnapshot(projections()[next], previous));
    setPhase(next);
  };
  onMount(async () => {
    const response = await fetch('/__fixture/guest-metadata-migration.json');
    if (!response.ok) throw new Error('Missing actual monitor projections');
    setProjections(await response.json());
    apply('suppressed');
  });
  (window as any).__guestMetadataMigration = { phase, guest, snapshot };
  return (
    <main class="mx-auto min-h-screen max-w-6xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Guest metadata across migration</h1>
      <p class="text-xs text-muted">Controlled source proof. No native guest or backup is operated.</p>
      <div class="flex flex-wrap gap-2" aria-label="Fixture phases">
        <For each={phases}>
          {([key, label]) => (
            <button
              class="rounded border border-border px-2 py-1 text-xs"
              disabled={!projections()[key]}
              aria-pressed={phase() === key}
              onClick={() => apply(key)}
            >
              {label}
            </button>
          )}
        </For>
      </div>
      <Show when={guest()}>
        <table class="w-full table-fixed">
          <tbody>
            <GuestRow
              guest={guest()}
              visibleColumnIds={window.innerWidth <= 768 ? ['name', 'node', 'disk'] : ['name', 'node', 'cpu', 'memory', 'disk', 'ip', 'os']}
              isExpanded={expanded()}
              onClick={() => setExpanded(!expanded())}
              workloadTableLayoutMode={window.innerWidth <= 768 ? 'phone' : 'wide'}
            />
          </tbody>
        </table>
        <Show when={expanded()}>
          <section aria-label="Guest details">
            <GuestDrawer guest={guest()} onClose={() => setExpanded(false)} />
          </section>
        </Show>
      </Show>
    </main>
  );
}

render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router><Route path="/*" component={Fixture} /></Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
