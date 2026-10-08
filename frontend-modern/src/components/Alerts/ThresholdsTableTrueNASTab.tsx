import { Show, type JSX } from 'solid-js';
import Database from 'lucide-solid/icons/database';
import HardDrive from 'lucide-solid/icons/hard-drive';
import Server from 'lucide-solid/icons/server';

import { ResourceTable } from './ResourceTable';
import { CollapsibleSection } from './Thresholds/sections/CollapsibleSection';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';
import type { ThresholdsTableSectionProps } from '@/features/alerts/thresholds/thresholdsTableSectionProps';
import type { Resource } from '@/features/alerts/thresholds/tableTypes';
import { getDiskTemperatureByTypeItems } from '@/features/alerts/thresholds/trueNASDiskTemperature';
import { FACTORY_AGENT_DEFAULTS } from '@/utils/alertThresholdDefaults';
import type { AlertResourceGlobalDefaultFallback } from './alertResourceTableModel';
import { getAlertThresholdsDefaultsSummary } from '@/utils/alertThresholdsSectionPresentation';

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
const TRUENAS_DISK_COLUMN_TOOLTIPS = { 'Temp °C': 'Disk temperature that raises an alert.' };

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
    defaultsSummary?: string;
    globalDefaultFallbacks?: Record<string, AlertResourceGlobalDefaultFallback>;
    columnTooltips?: Record<string, string>;
    note?: string;
  },
) {
  const { state, tableProps } = props;

  return (
    <Show when={state.hasSection(props.id)}>
      <CollapsibleSection
        id={props.id}
        title={props.title}
        defaultsSummary={
          props.defaultsSummary ?? getAlertThresholdsDefaultsSummary(props.columns, props.defaults)
        }
        resourceCount={props.resources().length}
        collapsed={state.isCollapsed(props.id)}
        onToggle={() => state.toggleSection(props.id)}
        icon={props.icon}
        isGloballyDisabled={tableProps.disableAllTrueNAS()}
        emptyMessage="No TrueNAS alert targets match the current filters."
      >
        <div ref={state.registerSection(props.id)} class="scroll-mt-24">
          <Show when={props.note}>
            <p class="mb-3 text-xs text-muted">{props.note}</p>
          </Show>
          <ResourceTable
            title=""
            onConfigureResourceIntent={tableProps.onConfigureResourceIntent}
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
            globalDefaultFallbacks={props.globalDefaultFallbacks}
            columnTooltips={props.columnTooltips}
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
  const diskTemperatureByType = () =>
    getDiskTemperatureByTypeItems({
      agentDiskTemperature: props.tableProps.agentDefaults.diskTemperature,
      diskTempByType: props.tableProps.diskTempByType,
    });
  // Unset, TrueNAS disks follow Disk temperature by type, so the header names
  // the per-type triggers rather than one number.
  const trueNASDiskDefaultsSummary = () => {
    if (trueNASDiskDefaults().temperature !== undefined) return undefined;
    return getAlertThresholdsDefaultsSummary(
      TRUENAS_DISK_COLUMNS,
      diskTemperatureByType().length > 0 ? {} : { temperature: 0 },
      { ruleItems: diskTemperatureByType() },
    );
  };
  // When the agent Disk Temp default is off, Disk temperature by type is off
  // too, so the unset cell reads Off and switching it on stages a TrueNAS-wide
  // value at the factory disk temperature trigger.
  const trueNASDiskTemperatureFallback = (): Record<string, AlertResourceGlobalDefaultFallback> => {
    const items = diskTemperatureByType();
    return {
      temperature: {
        label: 'By type',
        title:
          items.length > 0
            ? `Follows Disk temperature by type under Agents: ${items.join(', ')}. Enter a value to use one temperature for every TrueNAS disk.`
            : 'Off because the agent Disk Temp default under Agents is off. Turn it on here to alert on TrueNAS disks anyway.',
        off: items.length === 0,
        enableValue: FACTORY_AGENT_DEFAULTS.diskTemperature,
      },
    };
  };

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
        defaultsSummary={trueNASDiskDefaultsSummary()}
        globalDefaultFallbacks={trueNASDiskTemperatureFallback()}
        columnTooltips={TRUENAS_DISK_COLUMN_TOOLTIPS}
        note="Unless set here, each disk alerts at Disk temperature by type, set under Agents."
        factoryDefaults={props.tableProps.factoryTrueNASDiskDefaults}
        setGlobalDefaults={props.tableProps.setTrueNASDiskDefaults}
        onResetDefaults={props.tableProps.resetTrueNASDiskDefaults}
      />
    </>
  );
}
