// Production table, inline drawer and alert store. Synthetic API/config only;
// no appliance, guest-agent, alert delivery or installed acceptance.
import { onMount } from 'solid-js';
import { render } from 'solid-js/web';
import { Router, Route } from '@solidjs/router';
import { TrueNASStorageTopologyTable } from '../src/features/truenas/TrueNASStorageTopologyTable';
import { DarkModeContext, WebSocketContext } from '../src/contexts/appRuntime';
import { getGlobalWebSocketStore } from '../src/stores/websocket-global';
import { useAlertsActivation } from '../src/stores/alertsActivation';
import { resolveMetricDisplayThresholds } from '../src/utils/metricThresholds';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const disk = (id: string, temperature: number, retained = false): Resource =>
  ({
    id,
    type: 'physical_disk',
    name: id,
    displayName: id,
    status: 'online',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    sourceType: 'api',
    physicalDisk: {
      devPath: `/dev/${id}`,
      serial: `fixture-${id}`,
      diskType: 'nvme',
      temperature,
      health: 'PASSED',
      ...(retained
        ? { collection: { temperature: { state: 'unavailable', reason: 'Fixture standby' } } }
        : {}),
    },
  }) as Resource;

function Fixture() {
  const store = useAlertsActivation();
  onMount(() => {
    (window as any).__thresholdFixture = {
      refresh: store.refreshConfig,
      thresholds: (id: string) => store.getTrueNASDiskTemperatureThresholds(disk(id, 72)),
      guestControl: () =>
        resolveMetricDisplayThresholds(store.config(), 'guest', 'memory', 'guest', [
          'pulse-relaxed',
        ]),
    };
  });
  const resources = [
    disk('nvme-wide', 64),
    disk('nvme-raised', 72),
    disk('nvme-retained', 95, true),
  ];
  return (
    <main class="min-h-screen space-y-4 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">TrueNAS disk temperature</h1>
      <p class="text-xs text-muted">
        Synthetic readings and configuration; no appliance operations.
      </p>
      <TrueNASStorageTopologyTable
        resources={resources}
        scope={resources}
        emptyIcon={<span />}
        emptyTitle="No storage"
        emptyDescription="No storage"
      />
    </main>
  );
}

render(
  () => (
    <Router>
      <Route
        path="*"
        component={() => (
          <DarkModeContext.Provider
            value={() => document.documentElement.classList.contains('dark')}
          >
            <WebSocketContext.Provider value={getGlobalWebSocketStore()}>
              <Fixture />
            </WebSocketContext.Provider>
          </DarkModeContext.Provider>
        )}
      />
    </Router>
  ),
  document.getElementById('root')!,
);
