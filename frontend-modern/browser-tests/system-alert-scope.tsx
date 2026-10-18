// Production alert components/state/router and real Assistant handoff store.
// Synthetic API evidence; no native service, model, diagnostic or command.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { AlertHistoryTableSection } from '../src/features/alerts/AlertHistoryTableSection';
import { useAlertHistoryState } from '../src/features/alerts/useAlertHistoryState';
import { makeSystemAlert } from '../src/features/alerts/__fixtures__/systemAlerts';
import { aiChatStore } from '../src/stores/aiChat';
import type { Alert } from '../src/types/api';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

aiChatStore.setEnabled(true);
const pulseVM = { id: 'vm-pulse', name: 'Pulse', type: 'vm' } as Resource;
const control = makeSystemAlert('cpu', {
  id: 'resource-cpu',
  resourceId: pulseVM.id,
  metadata: { resourceType: 'vm' },
  value: 92,
  threshold: 80,
  message: 'Synthetic CPU reading on the monitored VM called Pulse.',
});

function Fixture() {
  const initial = makeSystemAlert();
  const [alerts, setAlerts] = createSignal<Record<string, Alert>>({
    [initial.id]: initial,
    [control.id]: control,
  });
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  const history = useAlertHistoryState({
    activeAlerts: alerts,
    getResource: () => pulseVM,
    allResources: () => [pulseVM],
  });
  (window as any).__systemAlertScope = {
    updateSystem: (type: string, legacy = false) => {
      const next = makeSystemAlert(
        type,
        legacy
          ? {
              id: 'legacy-system-id',
              resourceId: 'vm-wrong',
              node: 'wrong-node',
              metadata: { systemAlert: true, resourceType: 'vm' },
              threshold: 80,
            }
          : {},
      );
      setAlerts({ [next.id]: next, [control.id]: control });
    },
    recover: () => setAlerts({ [control.id]: control }),
    alerts,
    historyRows: history.alertData,
    context: () => aiChatStore.context,
    closeExplanation: () => aiChatStore.close(),
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Pulse system-alert evidence</h1>
      <p class="text-xs text-muted">
        Synthetic observations. No workload, diagnostic or model is contacted.
      </p>
      <section aria-label="Existing alert overview">
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
      </section>
      <section aria-label="Existing alert history">
        <h2 class="mb-2 font-semibold">History</h2>
        <AlertHistoryTableSection state={history} />
      </section>
      <Show when={aiChatStore.isOpenSignal()}>
        <section
          aria-label="Captured explanation context"
          class="rounded-md border border-border p-3"
        >
          <p class="text-sm">
            Test capture of the real explanation store; no Assistant request is executed.
          </p>
          <pre class="whitespace-pre-wrap break-all text-xs">
            {JSON.stringify(aiChatStore.context, null, 2)}
          </pre>
        </section>
      </Show>
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
