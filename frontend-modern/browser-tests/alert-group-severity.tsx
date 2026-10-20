// Real overview, grouping state, cards, router and CSS; only observations
// and local read-only API replies are synthetic. No monitored system is used.
import { Router, Route } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { aiChatStore } from '../src/stores/aiChat';
import type { Alert } from '../src/types/api';
import '../src/index.css';

aiChatStore.setEnabled(false);
const criticalReason =
  'Database storage is unavailable. The current observation cannot confirm a usable filesystem; check the affected storage before treating this service as recovered.';
function alert(id: string, overrides: Partial<Alert> = {}): Alert {
  return {
    id,
    type: 'connection',
    level: 'warning',
    resourceId: `resource-${id}`,
    resourceName: `Lab ${id}`,
    node: '',
    instance: 'synthetic',
    message: `Current synthetic observation for ${id}.`,
    value: 0,
    threshold: 0,
    startTime: '2026-10-20T11:55:00Z',
    acknowledged: false,
    ...overrides,
  };
}
const correlation = (role: 'primary' | 'supporting') => ({
  key: 'network:lab',
  kind: 'shared-system' as const,
  role,
  reason: 'Synthetic explicit membership, not name matching.',
});
const initial: Record<string, Alert> = {
  primary: alert('primary', { resourceName: 'Lab network', correlation: correlation('primary') }),
  critical: alert('critical', {
    resourceName: 'Database storage',
    message: criticalReason,
    level: 'critical',
    correlation: correlation('supporting'),
  }),
  warning: alert('warning', { correlation: correlation('supporting') }),
  info: alert('info', { level: 'info', correlation: correlation('supporting') }),
  acknowledged: alert('acknowledged', {
    level: 'critical',
    acknowledged: true,
    correlation: correlation('supporting'),
  }),
  otherPrimary: alert('other-primary', { resourceId: 'other', resourceName: 'Lab network' }),
  otherInfo: alert('other-info', { resourceId: 'other', level: 'info' }),
};

function Fixture() {
  const [alerts, setAlerts] = createSignal(initial);
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  Object.assign(window, {
    __alertGroupProof: {
      criticalReason,
      recover: () => setAlerts(({ critical: _removed, ...rest }) => rest),
      acknowledgeInfo: () =>
        setAlerts((previous) => ({ ...previous, info: { ...previous.info, acknowledged: true } })),
      clearRelated: () =>
        setAlerts((previous) => ({
          primary: previous.primary,
          otherPrimary: previous.otherPrimary,
          otherInfo: previous.otherInfo,
        })),
    },
  });
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-xl font-semibold">Alerts Overview</h1>
      <p class="text-sm text-muted">Synthetic observations; no notifications are sent.</p>
      <OverviewTab
        activeAlerts={alerts()}
        overrides={[]}
        updateAlert={() => {
          throw new Error('This read-only fixture must not mutate alerts.');
        }}
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
