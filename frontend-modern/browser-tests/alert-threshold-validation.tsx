// Production configuration, threshold tables, HTTP adapters and toast consumer.
// All resources and responses are synthetic; no real alert policy is changed.
import { Route, Router } from '@solidjs/router';
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { ToastContainer } from '../src/components/Toast/Toast';
import { AlertsConfigurationSurface } from '../src/features/alerts/AlertsConfigurationSurface';
import { ThresholdSlider } from '../src/components/Workloads/ThresholdSlider';
import type { Resource } from '../src/types/resource';
import '../src/index.css';

const resources: Resource[] = [
  {
    id: 'guest:fixture:101',
    type: 'vm',
    name: 'validation-guest',
    displayName: 'validation-guest',
    status: 'running',
    platformType: 'proxmox-pve',
    platformScopes: ['proxmox-pve'],
    sourceType: 'api',
    parentId: 'fixture-node',
  } as Resource,
];

function Fixture() {
  const [dirty, setDirty] = createSignal(false);
  return (
    <>
      <main class="mx-auto min-h-screen max-w-[2000px] space-y-4 bg-surface p-3 text-base-content">
        <h1 class="text-xl font-semibold">Alert thresholds</h1>
        <p class="text-sm text-muted">
          Synthetic configuration; no monitoring or notification operations.
        </p>
        <AlertsConfigurationSurface
          activeTab={() => 'thresholds'}
          allResources={() => resources}
          byType={(type) => resources.filter((resource) => resource.type === type)}
          children={() => []}
          activeAlerts={{}}
          removeAlerts={() => {}}
          setOverviewOverrides={() => {}}
          hasUnsavedChanges={dirty}
          setHasUnsavedChanges={setDirty}
          alertsActivationState={() => 'active'}
          alertsActivationConfig={() => ({ enabled: true })}
        />
        <section
          aria-label="Temperature threshold colour control"
          class="rounded-md border border-border p-4"
        >
          <h2 class="mb-4 text-sm">Temperature limit (synthetic control)</h2>
          <ThresholdSlider
            type="temperature"
            value={65}
            onChange={() => {}}
            ariaLabel="Temperature control limit"
          />
        </section>
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
