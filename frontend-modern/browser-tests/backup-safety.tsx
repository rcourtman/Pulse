// Canonical Resources API -> useWorkloads -> production row and backup disclosure.
// Synthetic observations only: no backup, guest request, thaw or recovery action.
import { createSignal, For, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { useWorkloads } from '../src/hooks/useWorkloads';
import { GuestRow } from '../src/components/Workloads/GuestRow';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import Docs from '../src/pages/Docs';
import { DarkModeContext } from '../src/contexts/appRuntime';
import '../src/index.css';

function Fixture() {
  const state = useWorkloads(() => true);
  const [showBackup, setShowBackup] = createSignal(true);
  const [actions, setActions] = createSignal(0);
  const [expanded, setExpanded] = createSignal(false);
  const primary = () => state.workloads().find((guest) => guest.vmid === 101);
  const layout = window.innerWidth <= 768 ? 'phone' : 'wide';
  (window as any).__backupSafety = {
    refetch: state.refetch,
    evidence: () => primary(),
    actions,
    expanded,
  };
  return (
    <main class="min-h-screen space-y-3 bg-surface p-3 text-base-content pulse-wide-data-surface">
      <h1 class="text-base font-semibold">Backup completion and guest safety</h1>
      <p class="text-xs text-muted">Synthetic resource observations; no guest commands.</p>
      <button
        class="min-h-11 rounded-sm border border-border px-3 text-xs"
        onClick={() => setShowBackup(!showBackup())}
      >
        {showBackup() ? 'Hide Backup column' : 'Show Backup column'}
      </button>
      <For each={[101, 102, 103, 104]}>
        {(vmid) => {
          const guest = () => state.workloads().find((value) => value.vmid === vmid);
          return (
            <Show when={guest()}>
              <section aria-label={`Guest ${vmid} row`}>
                <table class="w-full table-fixed workload-table">
                  <tbody>
                    <GuestRow
                      guest={guest()!}
                      visibleColumnIds={showBackup() ? ['name', 'backup'] : ['name']}
                      workloadTableLayoutMode={layout}
                      isExpanded={vmid === 101 && expanded()}
                      onClick={() => {
                        setActions((value) => value + 1);
                        if (vmid === 101) setExpanded((value) => !value);
                      }}
                    />
                  </tbody>
                </table>
              </section>
            </Show>
          );
        }}
      </For>
      <Show when={expanded() && primary()}>
        <section aria-label="Guest details">
          <GuestDrawer guest={primary()!} onClose={() => setExpanded(false)} />
        </section>
      </Show>
      <p role="status" aria-label="Row action count">
        {actions()}
      </p>
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
