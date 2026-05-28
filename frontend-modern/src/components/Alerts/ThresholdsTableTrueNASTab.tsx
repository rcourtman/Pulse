import { Show, type JSX } from 'solid-js';
import Database from 'lucide-solid/icons/database';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Server from 'lucide-solid/icons/server';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';

const TRUENAS_SYSTEM_COLUMNS = [
  'CPU %',
  'Memory %',
  'Disk %',
  'Temp °C',
  'Disk R MB/s',
  'Disk W MB/s',
  'Net In MB/s',
  'Net Out MB/s',
];
const TRUENAS_STORAGE_COLUMNS = ['Usage %'];
const TRUENAS_DISK_COLUMNS = ['Temp °C'];

function TrueNASResourceSection(
  props: ThresholdsTableSectionProps & {
    id: string;
    title: string;
    resources: () => Resource[];
    columns: string[];
    icon: JSX.Element;
    typeKey: 'truenas-system' | 'truenas-pool' | 'truenas-dataset' | 'truenas-disk';
    defaults: Record<string, number | undefined>;
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
        isGloballyDisabled={tableProps.disableAllTrueNAS()}
        emptyMessage="No TrueNAS alert targets match the current filters."
      >
        <div ref={state.registerSection(props.id)} class="scroll-mt-24">
          <ResourceTable
            title=""
            resources={props.resources()}
            columns={props.columns}
            activeAlerts={tableProps.activeAlerts}
            emptyMessage="No TrueNAS alert targets match the current filters."
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
            globalDisableFlag={tableProps.disableAllTrueNAS}
            onToggleGlobalDisable={() =>
              tableProps.setDisableAllTrueNAS(!tableProps.disableAllTrueNAS())
            }
            showDelayColumn={true}
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

export function ThresholdsTableTrueNASTab(props: ThresholdsTableSectionProps) {
  const trueNASDefaults = () => props.tableProps.trueNASDefaults ?? {};
  const trueNASStorageDefaults = () => ({ usage: trueNASDefaults().usage ?? 85 });
  const trueNASDiskDefaults = () => props.tableProps.trueNASDiskDefaults ?? {};

  return (
    <>
      <TrueNASResourceSection
        {...props}
        id="trueNASSystems"
        title="Systems"
        resources={props.state.trueNASSystemsWithOverrides}
        columns={TRUENAS_SYSTEM_COLUMNS}
        icon={<Server class="w-5 h-5" />}
        typeKey="truenas-system"
        defaults={trueNASDefaults()}
        factoryDefaults={props.tableProps.factoryTrueNASDefaults}
        setGlobalDefaults={props.tableProps.setTrueNASDefaults}
        onResetDefaults={props.tableProps.resetTrueNASDefaults}
      />
      <TrueNASResourceSection
        {...props}
        id="trueNASPools"
        title="Pools"
        resources={props.state.trueNASPoolsWithOverrides}
        columns={TRUENAS_STORAGE_COLUMNS}
        icon={<Database class="w-5 h-5" />}
        typeKey="truenas-pool"
        defaults={trueNASStorageDefaults()}
        factoryDefaults={
          props.tableProps.factoryTrueNASDefaults?.usage !== undefined
            ? { usage: props.tableProps.factoryTrueNASDefaults.usage }
            : undefined
        }
        setGlobalDefaults={(value) => {
          if (!props.tableProps.setTrueNASDefaults) return;
          props.tableProps.setTrueNASDefaults((prev) => {
            const next = typeof value === 'function' ? value({ usage: prev.usage }) : value;
            return { ...prev, usage: next.usage ?? 85 };
          });
        }}
        onResetDefaults={props.tableProps.resetTrueNASDefaults}
      />
      <TrueNASResourceSection
        {...props}
        id="trueNASDatasets"
        title="Datasets"
        resources={props.state.trueNASDatasetsWithOverrides}
        columns={TRUENAS_STORAGE_COLUMNS}
        icon={<Database class="w-5 h-5" />}
        typeKey="truenas-dataset"
        defaults={trueNASStorageDefaults()}
        factoryDefaults={
          props.tableProps.factoryTrueNASDefaults?.usage !== undefined
            ? { usage: props.tableProps.factoryTrueNASDefaults.usage }
            : undefined
        }
        setGlobalDefaults={(value) => {
          if (!props.tableProps.setTrueNASDefaults) return;
          props.tableProps.setTrueNASDefaults((prev) => {
            const next = typeof value === 'function' ? value({ usage: prev.usage }) : value;
            return { ...prev, usage: next.usage ?? 85 };
          });
        }}
        onResetDefaults={props.tableProps.resetTrueNASDefaults}
      />
      <TrueNASResourceSection
        {...props}
        id="trueNASDisks"
        title="Disks"
        resources={props.state.trueNASDisksWithOverrides}
        columns={TRUENAS_DISK_COLUMNS}
        icon={<HardDrive class="w-5 h-5" />}
        typeKey="truenas-disk"
        defaults={trueNASDiskDefaults()}
        factoryDefaults={props.tableProps.factoryTrueNASDiskDefaults}
        setGlobalDefaults={props.tableProps.setTrueNASDiskDefaults}
        onResetDefaults={props.tableProps.resetTrueNASDiskDefaults}
      />
    </>
  );
}
