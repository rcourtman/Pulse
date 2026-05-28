import { Show, type JSX } from 'solid-js';
import Cpu from 'lucide-solid/icons/cpu';
import Database from 'lucide-solid/icons/database';
import Monitor from 'lucide-solid/icons/monitor';
import Network from 'lucide-solid/icons/network';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';

const VMWARE_HOST_COLUMNS = [
  'CPU %',
  'Memory %',
  'Disk R MB/s',
  'Disk W MB/s',
  'Net In MB/s',
  'Net Out MB/s',
];
const VMWARE_VM_COLUMNS = [
  'CPU %',
  'Memory %',
  'Disk %',
  'Disk R MB/s',
  'Disk W MB/s',
  'Net In MB/s',
  'Net Out MB/s',
];
const VMWARE_DATASTORE_COLUMNS = ['Usage %'];

function VMwareResourceSection(
  props: ThresholdsTableSectionProps & {
    id: string;
    title: string;
    resources: () => Resource[];
    columns: string[];
    icon: JSX.Element;
    typeKey: 'vmware-host' | 'vmware-vm' | 'vmware-datastore' | 'vmware-network';
    defaults?: Record<string, number | undefined>;
    factoryDefaults?: Record<string, number | undefined>;
    setGlobalDefaults?: (
      value:
        | Record<string, number | undefined>
        | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
    ) => void;
    onResetDefaults?: () => void;
  },
) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection(props.id)}>
      <CollapsibleSection
        id={props.id}
        title={props.title}
        resourceCount={props.resources().length}
        collapsed={state.isCollapsed(props.id)}
        onToggle={() => state.toggleSection(props.id)}
        icon={props.icon}
        isGloballyDisabled={tableProps.disableAllVMware()}
        emptyMessage="No vSphere alert targets match the current filters."
      >
        <div ref={state.registerSection(props.id)} class="scroll-mt-24">
          <ResourceTable
            title=""
            resources={props.resources()}
            columns={props.columns}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage="No vSphere alert targets match the current filters."
            onEdit={state.startEditing}
            onSaveEdit={state.saveEdit}
            onCancelEdit={state.cancelEdit}
            onRemoveOverride={state.removeOverride}
            onToggleDisabled={state.toggleDisabled}
            showOfflineAlertsColumn={false}
            editingId={state.editingId}
            editingThresholds={state.editingThresholds}
            setEditingThresholds={state.setEditingThresholds}
            editingNote={state.editingNote}
            setEditingNote={state.setEditingNote}
            onBulkEdit={(ids) => state.handleBulkEdit(ids, props.columns)}
            formatMetricValue={formatMetricValue}
            hasActiveAlert={state.hasActiveAlert}
            globalDefaults={props.defaults}
            setGlobalDefaults={props.setGlobalDefaults}
            setHasUnsavedChanges={tableProps.setHasUnsavedChanges}
            globalDisableFlag={tableProps.disableAllVMware}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllVMware(!tableProps.disableAllVMware())
            }
            showDelayColumn={props.columns.length > 0}
            globalDelaySeconds={tableProps.timeThresholds()[props.typeKey]}
            metricDelaySeconds={tableProps.metricTimeThresholds()[props.typeKey] ?? {}}
            onMetricDelayChange={(metric, value) =>
              state.updateMetricDelay(props.typeKey, metric, value)
            }
            factoryDefaults={props.factoryDefaults}
            onResetDefaults={props.onResetDefaults}
          />
        </div>
      </CollapsibleSection>
    </Show>
  );
}

export function ThresholdsTableVMwareTab(props: ThresholdsTableSectionProps) {
  const vmwareDefaults = () => props.tableProps.vmwareDefaults ?? {};
  const datastoreDefaults = () => ({ usage: vmwareDefaults().usage ?? 85 });

  const setDatastoreDefaults = (
    value:
      | Record<string, number | undefined>
      | ((prev: Record<string, number | undefined>) => Record<string, number | undefined>),
  ) => {
    if (!props.tableProps.setVMwareDefaults) return;
    props.tableProps.setVMwareDefaults((prev) => {
      const next = typeof value === 'function' ? value({ usage: prev.usage }) : value;
      return { ...prev, usage: next.usage ?? 85 };
    });
  };

  return (
    <>
      <VMwareResourceSection
        {...props}
        id="vmwareHosts"
        title="Hosts"
        resources={props.state.vmwareHostsWithOverrides}
        columns={VMWARE_HOST_COLUMNS}
        icon={<Cpu class="w-5 h-5" />}
        typeKey="vmware-host"
        defaults={vmwareDefaults()}
        factoryDefaults={props.tableProps.factoryVMwareDefaults}
        setGlobalDefaults={props.tableProps.setVMwareDefaults}
        onResetDefaults={props.tableProps.resetVMwareDefaults}
      />
      <VMwareResourceSection
        {...props}
        id="vmwareVMs"
        title="Virtual Machines"
        resources={props.state.vmwareVMsWithOverrides}
        columns={VMWARE_VM_COLUMNS}
        icon={<Monitor class="w-5 h-5" />}
        typeKey="vmware-vm"
        defaults={vmwareDefaults()}
        factoryDefaults={props.tableProps.factoryVMwareDefaults}
        setGlobalDefaults={props.tableProps.setVMwareDefaults}
        onResetDefaults={props.tableProps.resetVMwareDefaults}
      />
      <VMwareResourceSection
        {...props}
        id="vmwareDatastores"
        title="Datastores"
        resources={props.state.vmwareDatastoresWithOverrides}
        columns={VMWARE_DATASTORE_COLUMNS}
        icon={<Database class="w-5 h-5" />}
        typeKey="vmware-datastore"
        defaults={datastoreDefaults()}
        factoryDefaults={
          props.tableProps.factoryVMwareDefaults?.usage !== undefined
            ? { usage: props.tableProps.factoryVMwareDefaults.usage }
            : undefined
        }
        setGlobalDefaults={setDatastoreDefaults}
        onResetDefaults={props.tableProps.resetVMwareDefaults}
      />
      <VMwareResourceSection
        {...props}
        id="vmwareNetworks"
        title="Networks"
        resources={props.state.vmwareNetworksWithOverrides}
        columns={[]}
        icon={<Network class="w-5 h-5" />}
        typeKey="vmware-network"
      />
    </>
  );
}
