// Production readers, org event/client and styles; all observations are synthetic.
// A stable resource ID across organisations must not preserve fetched History.
import { For } from 'solid-js';
import { render } from 'solid-js/web';
import { GuestDrawerHistory } from '../src/components/Workloads/GuestDrawerHistory';
import { HistoryChart, HistoryChartHoverGroup } from '../src/components/shared/HistoryChart';
import { HOST_METRICS_HISTORY_GROUPS } from '../src/components/shared/hostMetricsHistoryModel';
import { eventBus } from '../src/stores/events';
import { setOrgID } from '../src/utils/apiClient';
import '../src/index.css';

setOrgID('org-a');

function Fixture() {
  const switchOrg = (org: string) => {
    // Same ordering as useAppRuntimeState.handleOrgSwitch, with no authentication bypass.
    setOrgID(org);
    eventBus.emit('org_switched', org);
  };
  return (
    <main class="mx-auto min-h-screen max-w-5xl space-y-5 bg-surface p-4 text-base-content">
      <h1 class="text-lg font-semibold">History organisation boundary verification</h1>
      <div class="flex flex-wrap gap-2">
        <For each={['org-b', 'org-c', 'org-d']}>
          {(org) => (
            <button
              class="min-h-11 rounded border border-border px-3"
              onClick={() => switchOrg(org)}
            >
              Switch to {org}
            </button>
          )}
        </For>
      </div>
      <HistoryChartHoverGroup>
        <section data-testid="disk" class="rounded border border-border p-3">
          <h2 class="mb-3 font-semibold">Disk History</h2>
          <HistoryChart
            resourceType="disk"
            resourceId="disk:nas:sda"
            metric="smart_temp"
            label="Temperature"
            unit="C"
            range="1h"
            height={160}
            hideSelector
            compact
          />
        </section>
        <section data-testid="pool" class="rounded border border-border p-3">
          <h2 class="mb-3 font-semibold">Pool History</h2>
          <HistoryChart
            resourceType="storage"
            resourceId="storage:nas:pool"
            metric="disk"
            label="Usage"
            unit="%"
            range="1h"
            height={160}
            hideSelector
            compact
          />
        </section>
      </HistoryChartHoverGroup>
      <section data-testid="service" class="space-y-3">
        <h2 class="font-semibold">PBS service History (existing org-safe batch control)</h2>
        <GuestDrawerHistory
          target={{ resourceType: 'agent', resourceId: 'pbs-service' }}
          range="1h"
          groups={HOST_METRICS_HISTORY_GROUPS}
        />
      </section>
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
