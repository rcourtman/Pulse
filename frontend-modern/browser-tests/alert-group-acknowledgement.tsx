// Real overview, keyed alert state, HTTP adapter, toast consumer and CSS.
// Only observations and responses are synthetic; no notification is sent.
import { Route, Router } from '@solidjs/router';
import { createMemo, createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { ToastContainer } from '../src/components/Toast/Toast';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { aiChatStore } from '../src/stores/aiChat';
import type { Alert } from '../src/types/api';
import '../src/index.css';

aiChatStore.setEnabled(false);
function alert(id: string, overrides: Partial<Alert> = {}): Alert {
  return {
    id,
    type: 'connection',
    level: 'warning',
    resourceId: `resource-${id}`,
    resourceName: 'Lab network',
    node: '',
    instance: 'synthetic',
    message: `Synthetic current observation for ${id}.`,
    value: 0,
    threshold: 0,
    startTime: '2026-10-20T11:55:00Z',
    acknowledged: false,
    ...overrides,
  };
}
const correlation = (role: 'primary' | 'supporting') => ({
  key: 'synthetic-network',
  kind: 'shared-system' as const,
  role,
  reason: 'Explicit synthetic membership, not display-name matching.',
});
const initial: Record<string, Alert> = {
  primary: alert('primary', { correlation: correlation('primary') }),
  critical: alert('critical', {
    level: 'critical',
    resourceName: 'Database storage',
    message:
      'Storage is still unavailable. Acknowledgement is not recovery; this failed acknowledgement must leave the current alert active.',
    correlation: correlation('supporting'),
  }),
  acknowledged: alert('acknowledged', {
    acknowledged: true,
    level: 'info',
    correlation: correlation('supporting'),
  }),
  otherPrimary: alert('other-primary', { resourceId: 'independent-network' }),
  otherRelated: alert('other-related', { level: 'info', resourceId: 'independent-network' }),
};
function Fixture() {
  const [alertsById, setAlertsById] = createStore(initial);
  const alerts = createMemo(() =>
    Object.fromEntries(Object.values(alertsById).map((entry) => [entry.id, entry])),
  );
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  Object.assign(window, {
    __alertAckProof: { snapshot: () => JSON.parse(JSON.stringify(alertsById)) },
  });
  return (
    <>
      <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
        <h1 class="text-xl font-semibold">Alerts Overview</h1>
        <p class="text-sm text-muted">
          Synthetic observations; only local fixture acknowledgements are allowed.
        </p>
        <OverviewTab
          activeAlerts={alerts()}
          overrides={[]}
          updateAlert={(id, updates) => {
            const current = Object.values(alertsById).find((entry) => entry.id === id);
            if (!current) throw new Error(`Unrequested synthetic alert ${id}`);
            const key = Object.keys(alertsById).find((key) => alertsById[key].id === id)!;
            setAlertsById(key, { ...current, ...updates });
          }}
          showAcknowledged={showAcknowledged}
          setShowAcknowledged={setShowAcknowledged}
          showQuickTip={() => false}
          dismissQuickTip={() => {}}
          alertsDisabled={() => false}
        />
      </main>
      <ToastContainer />
    </>
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
