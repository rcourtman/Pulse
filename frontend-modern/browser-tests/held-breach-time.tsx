// Production attention, alert/history, Assistant button/store and chat briefing.
// All observations are synthetic. The fixture consumes the explanation intent
// before mounting Chat, so rendering a briefing never calls a model or action.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { NodeDrawerOverview } from '../src/components/Workloads/NodeDrawerOverview';
import { OverviewTab } from '../src/features/alerts/OverviewTab';
import { AlertHistoryTableSection } from '../src/features/alerts/AlertHistoryTableSection';
import { useAlertHistoryState } from '../src/features/alerts/useAlertHistoryState';
import { InvestigateAlertButton } from '../src/components/Alerts/InvestigateAlertButton';
import { AIChat } from '../src/components/AI/Chat';
import { aiChatStore } from '../src/stores/aiChat';
import type { Alert, Node } from '../src/types/api';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const node = {
  id: 'lab-minipc',
  name: 'minipc',
  instance: 'lab',
  status: 'online',
  type: 'node',
  cpu: 0,
  memory: { total: 1024, used: 256, free: 768, usage: 25 },
  disk: { total: 1024, used: 256, free: 768, usage: 25 },
  uptime: 3600,
  loadAverage: [],
  cpuInfo: { model: 'Fixture CPU', cores: 4, sockets: 1, mhz: '2400' },
  lastSeen: '2026-10-06T10:51:50Z',
  connectionHealth: 'healthy',
} as Node;
const resource = { ...node, type: 'node', platform: 'proxmox' } as unknown as Resource;
function makeAlert(scenario: string): Alert {
  const alert: Alert = {
    id: 'lab-minipc::metric-threshold:temperature',
    type: 'temperature',
    level: 'warning',
    resourceId: node.id,
    resourceName: node.name,
    node: node.name,
    instance: 'lab',
    message: 'Node temperature at 80.0°C',
    value: 80,
    threshold: 80,
    startTime: '2026-10-06T07:57:44Z',
    acknowledged: false,
    // A newer legacy poll must not replace the evaluated breach date.
    lastSeen: '2026-10-06T10:51:59Z',
    metricStatus: {
      phase: 'latched',
      value: 76,
      unit: '°C',
      observedAt: '2026-10-06T10:51:50Z',
      lastBreachAt: '2026-10-06T11:31:59.123456789+01:00',
      trigger: 80,
      recovery: 75,
      recoveryDelaySeconds: 300,
    },
  };
  if (scenario === 'recovering')
    alert.metricStatus = {
      ...alert.metricStatus!,
      phase: 'recovering',
      value: 72,
      recoveryElapsedSeconds: 120,
    };
  if (scenario === 'legacy') {
    alert.metricStatus!.lastBreachAt = 'not-a-date';
    alert.lastSeen = '2026-10-06T10:32:00Z';
  }
  if (scenario === 'unknown') {
    alert.metricStatus!.lastBreachAt = '0001-01-01T00:00:00Z';
    alert.lastSeen = 'unknown';
  }
  if (scenario === 'stale') alert.metricStatus!.observedAt = '2026-10-06T10:40:00Z';
  if (scenario === 'restart') {
    alert.metricStatus = undefined;
    alert.lastSeen = undefined;
  }
  return alert;
}
function Fixture() {
  const initial = makeAlert(new URLSearchParams(location.search).get('case') || 'latched');
  const [alerts, setAlerts] = createSignal<Record<string, Alert>>({ [initial.id]: initial });
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  const [showChat, setShowChat] = createSignal(false);
  const history = useAlertHistoryState({
    activeAlerts: alerts,
    getResource: () => resource,
    allResources: () => [resource],
  });
  aiChatStore.setEnabled(true);
  const readBriefing = () => {
    const request = aiChatStore.explanationRequestSignal();
    if (request) aiChatStore.ackExplanationRequest(request.id);
    setShowChat(true);
  };
  const closeChat = () => {
    setShowChat(false);
    aiChatStore.close();
  };
  (window as any).__heldBreach = {
    setScenario: (scenario: string) => {
      closeChat();
      const next = makeAlert(scenario);
      setAlerts({ [next.id]: next });
    },
    context: () => aiChatStore.context,
    close: closeChat,
    readBriefing,
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Held-alert breach date evidence</h1>
      <p class="text-xs text-muted">
        Synthetic readings only. No model, diagnostic or action is executed.
      </p>
      <section aria-label="Existing node attention">
        <NodeDrawerOverview node={node} alerts={Object.values(alerts())} />
      </section>
      <section aria-label="Existing alert overview">
        <OverviewTab
          activeAlerts={alerts()}
          overrides={[]}
          updateAlert={(id, updates) =>
            setAlerts((old) => ({ ...old, [id]: { ...old[id], ...updates } }))
          }
          showAcknowledged={showAcknowledged}
          setShowAcknowledged={setShowAcknowledged}
          showQuickTip={() => false}
          dismissQuickTip={() => {}}
          alertsDisabled={() => false}
        />
      </section>
      <section aria-label="Existing alert history">
        <h2 class="font-semibold">History</h2>
        <AlertHistoryTableSection state={history} />
      </section>
      <section aria-label="Existing Assistant handoff">
        <InvestigateAlertButton
          alert={Object.values(alerts())[0]}
          resourceType="node"
          variant="full"
        />
      </section>
      <button type="button" onClick={readBriefing}>
        Render attached briefing without inference
      </button>
      <Show when={showChat()}>
        <AIChat onClose={closeChat} />
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
