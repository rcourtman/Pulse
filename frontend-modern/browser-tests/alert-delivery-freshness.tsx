// Production overview, state, API reader, router and CSS. Only collected
// observations are synthetic; no notification or monitoring action executes.
import { Router, Route } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { setActiveLocale } from '../src/i18n';
import { aiChatStore } from '../src/stores/aiChat';
import type { Alert } from '../src/types/api';
import '../src/index.css';

aiChatStore.setEnabled(false);
function makeAlert(id: string, name: string): Alert {
  return {
    id,
    type: 'cpu',
    level: 'warning',
    resourceId: id,
    resourceName: name,
    node: 'lab-node',
    instance: 'lab',
    message: `CPU high on ${name}`,
    value: 91,
    threshold: 80,
    startTime: '2026-10-20T11:55:00Z',
    lastSeen: '2026-10-20T12:00:00Z',
    acknowledged: false,
    metadata: { resourceType: 'vm' },
  };
}
function Fixture() {
  const initial = {
    'vm-a': makeAlert('vm-a', 'Database'),
    'vm-b': makeAlert('vm-b', 'Web server'),
  };
  const [alerts, setAlerts] = createSignal<Record<string, Alert>>(initial);
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  Object.assign(window, { __deliveryFreshness: { setAlerts, initial, setActiveLocale } });
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-xl font-semibold">Alerts Overview</h1>
      <p class="text-sm text-muted">Synthetic observations; no notifications are sent.</p>
      <OverviewTab
        activeAlerts={alerts()}
        overrides={[]}
        updateAlert={(id, updates) =>
          setAlerts((previous) => ({ ...previous, [id]: { ...previous[id], ...updates } }))
        }
        showAcknowledged={showAcknowledged}
        setShowAcknowledged={setShowAcknowledged}
        showQuickTip={() => false}
        dismissQuickTip={() => {}}
        alertsDisabled={() => false}
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
