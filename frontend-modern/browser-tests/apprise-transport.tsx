// Synthetic HTTP → real API reader/normalizer → the production Notifications tab.
// The driver refuses every write and all off-origin requests. No delivery test.
import { createSignal, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { DestinationsTab } from '../src/features/alerts/tabs/DestinationsTab';
import { useAlertDestinationsState } from '../src/features/alerts/useAlertDestinationsState';
import '../src/index.css';

function Fixture() {
  const state = useAlertDestinationsState({ activeTab: () => 'destinations' });
  const [dirty, setDirty] = createSignal(false);
  onMount(() => void state.loadDestinations({ indicateLoading: true }));
  return (
    <main class="mx-auto max-w-4xl p-3 text-base-content">
      <h1 class="mb-4 text-xl font-semibold">Notifications</h1>
      <div class="mb-4 flex flex-wrap items-center gap-3">
        <button
          class="min-h-11 rounded-md border border-border px-3"
          onClick={() => void state.loadDestinations({ indicateLoading: true })}
        >
          Reload fixture configuration
        </button>
        <output aria-label="Fixture draft state">{dirty() ? 'Edited' : 'Unchanged'}</output>
      </div>
      <DestinationsTab
        emailConfig={state.emailConfig}
        setEmailConfig={state.setEmailConfig}
        appriseConfig={state.appriseConfig}
        setAppriseConfig={state.setAppriseConfig}
        setHasUnsavedChanges={setDirty}
        configLoadError={state.destConfigLoadError}
        isRetrying={() => false}
        isLoadingDestinations={state.isLoadingDestinations}
        onRetryLoad={() => void state.loadDestinations()}
        webhooks={state.webhooks}
        setWebhooks={state.setWebhooks}
        deadManPingUrl={state.deadManPingUrl}
        setDeadManPingUrl={state.setDeadManPingUrl}
        pushMinimumSeverity={state.pushMinimumSeverity}
        setPushMinimumSeverity={state.setPushMinimumSeverity}
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
