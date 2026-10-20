import { createMemo, createSignal, For } from 'solid-js';
import { render } from 'solid-js/web';
import { StoragePoolsTable } from '../src/components/Storage/StoragePoolsTable';
import { useStorageAlertState } from '../src/components/Storage/useStorageAlertState';
import { buildStorageRecords } from '../src/features/storageBackups/storageAdapters';
import { groupStorageRecords } from '../src/features/storageBackups/storageModelCore';
import { useStorageExpansionState } from '../src/components/Storage/useStorageExpansionState';
import type { State } from '../src/types/api';
import type { Resource } from '../src/types/resource';
import frames from './unraid-storage-frames.json';
import '../src/index.css';

const modes = Object.keys(frames);
const [mode, setMode] = createSignal('legacy-17-empty');

const state = createMemo(() => (frames as Record<string, State>)[mode()]);
const resources = createMemo(() => state().resources as Resource[]);
const records = createMemo(() =>
  buildStorageRecords({ state: state(), resources: resources() }).filter(
    (r) => r.details?.type === 'unraid-array',
  ),
);
const disks = createMemo(() => resources().filter((r) => r.type === 'physical_disk'));
const { getRecordAlertState } = useStorageAlertState({
  records,
  activeAlerts: () => state().activeAlerts,
  alertsEnabled: () => true,
});
const groups = createMemo(() => groupStorageRecords(records(), 'none'));
// These independent isolation controls deliberately share the displayed array
// and host names with the producer row. Map each separately so the test holds
// display names constant without exercising unrelated duplicate-row folding.
const healthyState = (frames as unknown as Record<string, State>)['fresh-agent'];
const template = (healthyState.resources as Resource[]).find((r) => r.type === 'storage')!;
const controls = [
  {
    ...template,
    id: 'isolation-other-host',
    metricsTarget: { resourceType: 'storage', resourceId: 'other-machine/storage:unraid-array' },
  },
  {
    ...template,
    id: 'isolation-other-array',
    metricsTarget: { resourceType: 'storage', resourceId: 'unraid-machine/storage:other-array' },
  },
  { ...template, id: 'isolation-api', sourceType: 'api', metricsTarget: template.metricsTarget },
] as Resource[];
const Control = (props: { resource: Resource }) => {
  const isolatedRecords = () =>
    buildStorageRecords({ state: healthyState, resources: [props.resource] });
  const { getRecordAlertState: isolatedAlertState } = useStorageAlertState({
    records: isolatedRecords,
    activeAlerts: () => state().activeAlerts,
    alertsEnabled: () => true,
  });
  const isolatedGroups = () => groupStorageRecords(isolatedRecords(), 'none');
  const expansion = useStorageExpansionState({
    groupBy: () => 'none',
    groupedKeys: () => isolatedGroups().map((group) => group.key),
    view: () => 'pools',
  });
  return (
    <section aria-label={props.resource.id}>
      <StoragePoolsTable
        groupedRecords={isolatedGroups()}
        groupBy="none"
        expandedGroups={expansion.expandedGroups()}
        toggleGroup={expansion.toggleGroup}
        expandedPoolId={expansion.expandedPoolId()}
        setExpandedPoolId={expansion.setExpandedPoolId}
        storageGrowthBySeriesId={new Map()}
        storageGrowthColumnLabel="Growth (7d)"
        physicalDisks={[]}
        nodeOnlineByLabel={new Map()}
        highlightedRecordId={null}
        getRecordAlertState={isolatedAlertState}
        isLoading={false}
      />
    </section>
  );
};
const App = () => {
  const { expandedGroups, expandedPoolId, setExpandedPoolId, toggleGroup } =
    useStorageExpansionState({
      groupBy: () => 'none',
      groupedKeys: () => groups().map((group) => group.key),
      view: () => 'pools',
    });
  return (
    <main class="min-h-screen bg-surface p-4 text-base-content space-y-4">
      <h1 class="text-lg font-semibold">Unraid reported-topology verification</h1>
      <p>
        Production Storage table, alert binding and detail, using captured synthetic producer
        frames.
      </p>
      <nav aria-label="Captured frame" class="flex flex-wrap gap-2">
        <For each={modes}>
          {(value) => (
            <button
              class="min-h-11 rounded border border-border px-3"
              onClick={() => {
                setMode(value);
                setExpandedPoolId(null);
              }}
            >
              {value}
            </button>
          )}
        </For>
      </nav>
      <section aria-label="Storage consumer">
        <StoragePoolsTable
          groupedRecords={groups()}
          groupBy="none"
          expandedGroups={expandedGroups()}
          toggleGroup={toggleGroup}
          expandedPoolId={expandedPoolId()}
          setExpandedPoolId={setExpandedPoolId}
          storageGrowthBySeriesId={new Map()}
          storageGrowthColumnLabel="Growth (7d)"
          physicalDisks={disks()}
          nodeOnlineByLabel={new Map()}
          highlightedRecordId={null}
          getRecordAlertState={getRecordAlertState}
          isLoading={false}
        />
      </section>
      <h2>Identity isolation controls — same displayed array and host</h2>
      <For each={controls}>{(resource) => <Control resource={resource} />}</For>
    </main>
  );
};
render(App, document.getElementById('root')!);
