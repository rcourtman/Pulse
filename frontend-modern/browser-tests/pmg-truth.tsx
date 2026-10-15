import { render } from 'solid-js/web';
import { createMemo, createSignal } from 'solid-js';
import { createStore, reconcile } from 'solid-js/store';
import { Router, Route } from '@solidjs/router';
import '@/index.css';
import { DEFAULT_LOCALE, setActiveLocale } from '@/i18n';
import { WebSocketContext, DarkModeContext } from '@/contexts/appRuntime';
import { InfrastructureSourceManager } from '@/components/Settings/InfrastructureSourceManager';
import { connectionToRow } from '@/components/Settings/useConnectionsLedger';
import { buildProxmoxPageModel } from '@/features/proxmox/proxmoxPageModel';
import { ProxmoxMailGatewayTable } from '@/features/proxmox/ProxmoxMailGatewayTable';
import { AlertOverviewActiveAlertsSection } from '@/features/alerts/AlertOverviewActiveAlertsSection';
import { useAlertOverviewState } from '@/features/alerts/useAlertOverviewState';
import { useAlertIncidentTimelineState } from '@/features/alerts/useAlertIncidentTimelineState';
// The browser supplies the unchanged retained producer bytes. No live PMG.
const projections = await fetch('/__fixture/phases').then((response) => response.json());
setActiveLocale(DEFAULT_LOCALE);
// Store reconciliation mutates its target. Keep it separate from the retained
// producer phases so returning to an earlier phase still reads the original
// response, including the alert's nested evidence.
const alertMap = (p: any) =>
  Object.fromEntries((p.alerts || []).map((a: any) => [a.id, structuredClone(a)]));
function Probe() {
  const [phase, setPhase] = createSignal(projections[0]);
  const [activeAlerts, setActiveAlerts] = createStore(alertMap(phase()));
  const [showAcknowledged, setShowAcknowledged] = createSignal(true);
  const model = createMemo(() => buildProxmoxPageModel(phase().resources as any));
  const rows = createMemo(() => phase().connections.map(connectionToRow));
  const overview = useAlertOverviewState({
    activeAlerts: () => activeAlerts,
    overrides: () => [],
    showAcknowledged,
    updateAlert: () => {
      throw new Error('Mutation excluded from source fixture');
    },
  });
  const timeline = useAlertIncidentTimelineState();
  const apply = (name: string) => {
    const next = projections.find((p) => p.name === name);
    if (!next) throw new Error(name);
    setPhase(next);
    setActiveAlerts(reconcile(alertMap(next)));
  };
  (window as any).__pmg = { apply, phase, rows, model, activeAlerts, projections };
  return (
    <DarkModeContext.Provider value={() => true}>
      <WebSocketContext.Provider value={{ activeAlerts } as any}>
        <main class="min-h-screen bg-app text-base-content p-3 sm:p-6 space-y-6">
          <header>
            <h1 class="text-xl font-semibold">PMG health presentation verification</h1>
            <p class="text-xs text-muted">
              Retained synthetic producer responses through existing consumers. No live appliance or
              notification delivery.
            </p>
          </header>
          <section id="connections" aria-label="Infrastructure connections">
            <h2 class="text-lg font-semibold mb-2">Infrastructure</h2>
            <InfrastructureSourceManager
              rows={rows}
              discoveredNodes={() => []}
              discoveryEnabled={false}
              discoveryScanStatus={() => ({ scanning: false })}
              readOnly={true}
            />
          </section>
          <section id="gateways" aria-label="Mail Gateway readings">
            <h2 class="text-lg font-semibold mb-2">Mail Gateway</h2>
            <ProxmoxMailGatewayTable
              resources={model().pmg}
              emptyTitle="No gateways"
              emptyDescription="No gateways"
            />
          </section>
          <section id="alerts" aria-label="Alert overview">
            <AlertOverviewActiveAlertsSection
              state={overview}
              timelineState={timeline}
              activeAlerts={activeAlerts}
              alertsDisabled={false}
              showAcknowledged={showAcknowledged()}
              setShowAcknowledged={setShowAcknowledged}
            />
          </section>
        </main>
      </WebSocketContext.Provider>
    </DarkModeContext.Provider>
  );
}
render(
  () => (
    <Router>
      <Route path="/*" component={Probe} />
    </Router>
  ),
  document.getElementById('root')!,
);
