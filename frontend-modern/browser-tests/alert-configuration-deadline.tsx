// Actual policy/destination loaders, complete Notifications surface and saves.
// Only HTTP producer responses are synthetic; no installed policy is touched.
import { Route, Router } from '@solidjs/router';
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { AlertsConfigurationSurface } from '../src/features/alerts/AlertsConfigurationSurface';
import { ToastContainer } from '../src/components/Toast/Toast';
import { loadRuntimeCapabilities } from '../src/stores/license';
import '../src/index.css';

function Fixture() {
  const [dirty, setDirty] = createSignal(false);
  const [visible, setVisible] = createSignal(true);
  return (
    <main class="mx-auto min-h-screen max-w-4xl space-y-4 bg-surface p-3 text-base-content sm:p-6">
      <h1 class="text-lg font-semibold">Alert notification settings</h1>
      <p class="text-sm text-muted">Synthetic settings only. No real policy or destination is used.</p>
      <button onClick={() => setVisible(false)}>Retire synthetic settings view</button>
      <Show when={visible()}>
        <AlertsConfigurationSurface
          activeTab={() => 'destinations'} allResources={() => []} byType={() => []}
          children={() => []} activeAlerts={{}} removeAlerts={() => {}}
          setOverviewOverrides={() => {}} hasUnsavedChanges={dirty} setHasUnsavedChanges={setDirty}
          alertsActivationState={() => 'active'} alertsActivationConfig={() => ({ enabled: true })}
        />
      </Show>
      <ToastContainer />
    </main>
  );
}
// Keep existing subscribers' compatibility read in this recovery matrix.
await loadRuntimeCapabilities();
render(() => <Router><Route path="*" component={Fixture} /></Router>, document.getElementById('root')!);
