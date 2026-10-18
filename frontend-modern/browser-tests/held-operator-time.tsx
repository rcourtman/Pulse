// Existing production controls/drawer/CSS with synthetic, local-only inputs.
// No maintenance, guest, update-apply or rollback action is executed.
import { createSignal, Show } from 'solid-js';
import { render } from 'solid-js/web';
import { Route, Router } from '@solidjs/router';
import { ResourceOperatorStateSection } from '../src/components/Infrastructure/ResourceOperatorStateSection';
import { UpdatesSettingsPanel } from '../src/components/Settings/UpdatesSettingsPanel';
import { GuestDrawer } from '../src/components/Workloads/GuestDrawer';
import { DarkModeContext } from '../src/contexts/appRuntime';
import { updateStore } from '../src/stores/updates';
import { METRIC_ALERT_STATUS_STALE_MS } from '../src/features/alerts/metricAlertPresentation';
import type { Alert } from '../src/types/api';
import type { WorkloadGuest } from '../src/types/workloads';
import '../src/index.css';

const start = Date.parse('2026-10-11T04:00:00Z');
const iso = (offset: number) => new Date(start + offset).toISOString();
const surface = new URLSearchParams(location.search).get('surface') ?? 'once';

const initialGuest: WorkloadGuest = {
  id: 'fixture:pve-a:101',
  name: 'Held evidence VM',
  instance: 'fixture',
  node: 'pve-a',
  vmid: 101,
  type: 'qemu',
  status: 'running',
  cpu: 0.1,
  cpus: 2,
  memory: {
    total: 4 * 1024 ** 3,
    used: 3 * 1024 ** 3,
    free: 1024 ** 3,
    usage: 75,
    observation: { state: 'current', source: 'status-mem', observedAt: iso(-60_000) },
  },
  disk: { total: 10 * 1024 ** 3, used: 5 * 1024 ** 3, usage: 50 },
  tags: [],
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 3600,
  template: false,
  lastBackup: 0,
  lastSeen: iso(0),
};
const alert: Alert = {
  id: 'held-cpu-alert',
  resourceId: initialGuest.id,
  resourceName: initialGuest.name,
  type: 'cpu',
  level: 'warning',
  message: 'CPU above threshold',
  value: 92,
  threshold: 80,
  startTime: iso(-60_000),
  acknowledged: false,
  node: 'pve-a',
  metricStatus: {
    phase: 'breaching',
    value: 91,
    unit: '%',
    observedAt: iso(-METRIC_ALERT_STATUS_STALE_MS + 5_000),
    trigger: 80,
    recovery: 75,
  },
};

function Fixture() {
  const [resourceId, setResourceId] = createSignal(surface);
  const [guest, setGuest] = createSignal(initialGuest);
  const [alerts, setAlerts] = createSignal([alert]);
  const [channel, setChannel] = createSignal<'stable' | 'rc'>('stable');
  const [auto, setAuto] = createSignal(false);
  const [dirty, setDirty] = createSignal(false);
  const [sourceBuild, setSourceBuild] = createSignal(false);
  let checks = 0,
    installs = 0;
  if (surface === 'updates') void updateStore.checkForUpdates(true);

  (window as any).__heldOperator = {
    ready: () => surface !== 'updates' || updateStore.lastCheckedAt() !== null,
    controls: () => ({ checks, installs, dirty: dirty(), channel: channel(), auto: auto() }),
    sourceBuild: () => setSourceBuild(true),
    withdrawWindow: () => setResourceId('absent'),
    memory: (source: string, state = 'current') =>
      setGuest((current) => ({
        ...current,
        memory: {
          ...current.memory,
          usageUnavailable: state === 'unavailable',
          observation: { source, state, observedAt: iso(-60_000) },
        },
      })),
    withdrawAlert: () => setAlerts([]),
    guest: () => guest(),
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-3 bg-surface p-3 text-base-content">
      <h1 class="text-base font-semibold">Held operational evidence</h1>
      <p class="text-xs text-muted">Synthetic local inputs. No operation is dispatched.</p>
      <Show when={['once', 'server', 'recurring'].includes(surface)}>
        <ResourceOperatorStateSection resourceId={resourceId()} />
      </Show>
      <Show when={surface === 'updates'}>
        <UpdatesSettingsPanel
          versionInfo={() =>
            updateStore.versionInfo() && {
              ...updateStore.versionInfo()!,
              isSourceBuild: sourceBuild(),
            }
          }
          updateInfo={updateStore.updateInfo}
          checkingForUpdates={updateStore.isChecking}
          updateChannel={channel}
          setUpdateChannel={setChannel}
          autoUpdateEnabled={auto}
          setAutoUpdateEnabled={setAuto}
          setHasUnsavedChanges={setDirty}
          checkForUpdates={async () => {
            checks++;
          }}
          updatePlan={() => null}
          onInstallUpdate={() => {
            installs++;
          }}
          isInstalling={() => false}
        />
      </Show>
      <Show when={surface === 'guest'}>
        <GuestDrawer guest={guest()} alerts={alerts()} onClose={() => {}} />
      </Show>
    </main>
  );
}
render(
  () => (
    <DarkModeContext.Provider value={() => document.documentElement.classList.contains('dark')}>
      <Router>
        <Route path="/*" component={Fixture} />
      </Router>
    </DarkModeContext.Provider>
  ),
  document.getElementById('root')!,
);
