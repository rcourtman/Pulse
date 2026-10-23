// Real active-alert adapter and Overview, fed only by the Go manager's frames.
import { createSignal, onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { AlertsAPI } from '../src/api/alerts';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { aiAvailabilityStore } from '../src/stores/aiAvailability';
import type { Alert } from '../src/types/api';
import '../src/index.css';

aiAvailabilityStore.setEnabled(false);

function Fixture() {
  const [active, setActive] = createSignal<Record<string, Alert>>({});
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  const [generation, setGeneration] = createSignal(0);
  const reload = async () => {
    const alerts = await AlertsAPI.getActive();
    setActive(Object.fromEntries(alerts.map((alert) => [alert.id, alert])));
    setGeneration((value) => value + 1);
  };
  onMount(() => void reload());
  Object.assign(window, { __backupDate: { reload, active, generation } });
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Backup-age observations</h1>
      <p class="text-xs text-muted">Synthetic monitoring evidence. No backup or guest operation.</p>
      <button
        class="min-h-11 rounded-sm border border-border px-3 text-sm"
        onClick={() => void reload()}
      >
        Refresh monitoring evidence
      </button>
      <OverviewTab
        overrides={[]}
        activeAlerts={active()}
        updateAlert={(id, updates) =>
          setActive((previous) => ({ ...previous, [id]: { ...previous[id], ...updates } }))
        }
        showQuickTip={() => false}
        dismissQuickTip={() => {}}
        showAcknowledged={showAcknowledged}
        setShowAcknowledged={setShowAcknowledged}
        alertsDisabled={() => false}
      />
    </main>
  );
}

render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
