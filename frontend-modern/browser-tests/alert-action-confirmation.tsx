// Actual overview/cards, state owners, HTTP adapter, translated toasts and CSS.
// Only observations and server responses are synthetic. No real alert is changed.
import { Route, Router } from '@solidjs/router';
import { createMemo, createSignal } from 'solid-js';
import { createStore } from 'solid-js/store';
import { render } from 'solid-js/web';
import { ToastContainer } from '../src/components/Toast/Toast';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { aiChatStore } from '../src/stores/aiChat';
import { loadLocaleCatalog, setLocalePreference, t } from '../src/i18n';
import type { Alert } from '../src/types/api';
import '../src/index.css';

aiChatStore.setEnabled(false);
const params = new URLSearchParams(location.search);
const locale = await loadLocaleCatalog(params.get('locale'));
setLocalePreference(locale);
document.documentElement.lang = locale;
const action = params.get('action');
const alert = (id: string): Alert => ({
  id,
  type: 'connection',
  level: 'critical',
  resourceId: `resource-${id}`,
  resourceName: 'Database',
  node: 'synthetic-pve',
  instance: 'synthetic',
  message: 'Synthetic current connection failure. Acknowledgement or snooze is not recovery.',
  value: 0,
  threshold: 0,
  startTime: '2026-10-21T11:55:00Z',
  acknowledged: false,
});
const target = alert('proof-target');
if (action === 'unacknowledge') target.acknowledged = true;
if (action === 'unsnooze')
  target.operationalRecord = {
    id: 'alert:proof-target',
    canonicalSpecId: 'connection',
    subjectResourceId: target.resourceId,
    state: 'suppressed',
    severity: 'critical',
    firstObservedAt: target.startTime,
    lastObservedAt: target.startTime,
    stateChangedAt: target.startTime,
    suppression: {
      at: target.startTime,
      by: 'synthetic',
      reason: 'user_snooze',
      expiresAt: '2026-10-21T14:00:00Z',
    },
    evidenceIds: [],
    causeKey: 'proof-target',
    relatedResourceIds: [],
  };
function Fixture() {
  const [rows, setRows] = createStore<Record<string, Alert>>({
    'proof-target': target,
    'proof-neighbour': alert('proof-neighbour'),
  });
  const alerts = createMemo(() => Object.fromEntries(Object.entries(rows)));
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  Object.assign(window, {
    __actionProof: {
      ready: true,
      locale,
      snapshot: () => JSON.parse(JSON.stringify(rows)),
      resample: () => {
        for (const [id, row] of Object.entries(rows))
          setRows(id, { ...row, message: row.message + ' Resampled.' });
      },
      labels: {
        acknowledge: t('alerts.overview.action.acknowledge'),
        restore: t('alerts.overview.action.unacknowledge'),
        snooze: t('alerts.overview.action.snooze'),
        resume: t('alerts.overview.action.resume'),
        snoozeTitle: t('alerts.overview.snooze.title'),
        oneHour: t('alerts.overview.snooze.option.oneHour'),
        error: t(
          `alerts.overview.notification.${action === 'unacknowledge' ? 'restoreFailed' : action === 'unsnooze' ? 'resumeFailed' : action === 'snooze' ? 'snoozeFailed' : 'acknowledgeFailed'}`,
        ),
      },
    },
  });
  return (
    <>
      <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
        <h1 class="text-xl font-semibold">Alerts Overview</h1>
        <p class="text-sm text-muted">Synthetic observations and local fixture actions only.</p>
        <OverviewTab
          activeAlerts={alerts()}
          overrides={[]}
          updateAlert={(id, changes) => {
            if (!rows[id]) throw new Error('Unrequested fixture alert');
            setRows(id, { ...rows[id], ...changes });
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
