import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import ExternalLinkIcon from 'lucide-solid/icons/external-link';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import MoreHorizontalIcon from 'lucide-solid/icons/more-horizontal';
import PlusIcon from 'lucide-solid/icons/plus';
import Trash2Icon from 'lucide-solid/icons/trash-2';
import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onCleanup,
  type Component,
  type JSX,
} from 'solid-js';
import { Portal } from 'solid-js/web';
import { AgentMetadataAPI, type AgentMetadata } from '@/api/agentMetadata';
import { MonitoringAPI } from '@/api/monitoring';
import { EnhancedCPUBar } from '@/components/Workloads/EnhancedCPUBar';
import { StackedDiskBar } from '@/components/Workloads/StackedDiskBar';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { ColumnPicker } from '@/components/shared/ColumnPicker';
import { StatusDot } from '@/components/shared/StatusDot';
import { TemperatureGauge } from '@/components/shared/TemperatureGauge';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import { TooltipPortal } from '@/components/shared/TooltipPortal';
import { hostOverrideIdCandidates } from '@/features/alerts/alertOverridesModel';
import {
  compareAgentVersions,
  formatAgentVersionDisplay,
  hostAgentVersion,
} from '@/features/platformPage/agentVersion';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableResetFiltersButton,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import { useColumnVisibility } from '@/hooks/useColumnVisibility';
import type { Disk } from '@/types/api';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { getActionableAgentIdFromResource } from '@/utils/agentResources';
import { formatBytes, formatSpeed, normalizeDiskArray } from '@/utils/format';
import { STORAGE_KEYS } from '@/utils/localStorage';
import { useAlertsActivation } from '@/stores/alertsActivation';
import { notificationStore } from '@/stores/notifications';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  getRaidDeviceBadgeClass,
  getRaidStateTextClass,
  getRaidStateVariant,
} from '@/utils/raidPresentation';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  RESOURCE_METADATA_CHANGED_EVENT,
  type ResourceMetadataChangedDetail,
} from '@/utils/resourceMetadataEvents';
import { getWorkloadsGuestNetworkEmptyState } from '@/utils/workloadGuestPresentation';
import {
  AGENT_MACHINE_COLUMNS,
  getAgentMachineCpuPercent,
  getAgentMachineDiskIODetails,
  getAgentMachineDiskIOTotal,
  getAgentMachineIpValues,
  matchesAgentMachineSearch,
  getAgentMachineNetworkInterfaceDetails,
  getAgentMachineNetworkTotal,
  getAgentMachinePrimaryIp,
  getAgentMachineRaidArrayDetails,
  getAgentMachineRaidSummary,
  getAgentMachineTemperatureCelsius,
  getAgentMachineTemperatureDetailSections,
  getAgentMachineTemperatureTitle,
  getNextAgentMachineSortState,
  sortAgentMachines,
  timestampMillisFrom,
  type AgentMachineColumn,
  type AgentMachineColumnId,
  type AgentMachineDiskIODetail,
  type AgentMachineNetworkInterfaceDetail,
  type AgentMachineRaidArrayDetail,
  type AgentMachineSortKey,
  type AgentMachineTemperatureDetailSection,
} from './agentMachineTableModel';

const formatUptime = (seconds: number | undefined): string => {
  if (!seconds || seconds <= 0) return '—';
  const days = Math.floor(seconds / 86_400);
  if (days > 0) return `${days}d`;
  const hours = Math.floor(seconds / 3_600);
  if (hours > 0) return `${hours}h`;
  const mins = Math.floor(seconds / 60);
  return `${mins}m`;
};

const formatLastSeen = (value: number | string | Date | undefined): string => {
  const timestampMillis = timestampMillisFrom(value);
  if (!timestampMillis) return '—';
  const ageSeconds = Math.max(0, Math.floor((Date.now() - timestampMillis) / 1000));
  if (ageSeconds < 60) return 'now';
  const minutes = Math.floor(ageSeconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.floor(hours / 24)}d`;
};

type MetricFallbackReason = {
  label: string;
  title: string;
};

const metricFallback = (reason?: MetricFallbackReason) => (
  <div class="flex justify-center">
    <span
      class={reason ? 'text-[9px] font-medium text-muted' : 'text-xs text-muted'}
      title={reason?.title}
      aria-label={reason?.title ?? 'No telemetry data'}
    >
      {reason?.label ?? '—'}
    </span>
  </div>
);

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const hasPositiveTemperature = (celsius: number | undefined): celsius is number =>
  typeof celsius === 'number' && Number.isFinite(celsius) && celsius > 0;

const AgentMachineMetricTooltip: Component<{
  triggerDataAttribute: string;
  tooltipDataAttribute: string;
  triggerClass: string;
  tooltipClass: string;
  enabled: boolean;
  ariaLabel?: string;
  title?: string;
  maxWidth: number;
  trigger: JSX.Element;
  children: JSX.Element;
}> = (props) => {
  const [visible, setVisible] = createSignal(false);
  const [position, setPosition] = createSignal({ x: 0, y: 0 });
  const triggerDataAttributes = () => ({ [props.triggerDataAttribute]: 'true' });
  const tooltipDataAttributes = () => ({ [props.tooltipDataAttribute]: 'true' });
  const enabled = () => props.enabled;
  const label = () => props.ariaLabel || props.title || undefined;
  const open = (event: MouseEvent | FocusEvent) => {
    if (!enabled()) return;
    const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
    setPosition({ x: rect.left + rect.width / 2, y: rect.top });
    setVisible(true);
  };
  const close = () => setVisible(false);

  return (
    <>
      <span
        {...triggerDataAttributes()}
        class={props.triggerClass}
        aria-label={label()}
        title={!enabled() ? props.title || undefined : undefined}
        tabIndex={enabled() ? 0 : undefined}
        onMouseEnter={open}
        onMouseOver={open}
        onMouseLeave={close}
        onFocus={open}
        onBlur={close}
        onClick={(event) => {
          event.stopPropagation();
          open(event);
        }}
      >
        {props.trigger}
      </span>
      <TooltipPortal
        when={visible() && enabled()}
        x={position().x}
        y={position().y}
        maxWidth={props.maxWidth}
      >
        <div {...tooltipDataAttributes()} class={props.tooltipClass}>
          {props.children}
        </div>
      </TooltipPortal>
    </>
  );
};

const AgentMachineTemperatureCell: Component<{
  celsius: number | undefined;
  sections: AgentMachineTemperatureDetailSection[];
  title: string;
}> = (props) => {
  const hasDetails = () => props.sections.length > 0;
  const positiveTemperature = () =>
    hasPositiveTemperature(props.celsius) ? props.celsius : undefined;

  return (
    <AgentMachineMetricTooltip
      triggerDataAttribute="data-agent-machine-temperature-trigger"
      tooltipDataAttribute="data-agent-machine-temperature-tooltip"
      triggerClass="inline-flex min-w-[2.25rem] justify-end text-xs tabular-nums"
      tooltipClass="min-w-[190px] max-w-[300px] space-y-2"
      enabled={hasDetails()}
      ariaLabel={props.title || undefined}
      maxWidth={320}
      trigger={
        <Show when={positiveTemperature()} fallback={<span class="text-muted">—</span>}>
          {(value) => <TemperatureGauge value={value()} />}
        </Show>
      }
    >
      <For each={props.sections}>
        {(section) => (
          <section>
            <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">
              {section.heading}
            </div>
            <div class="space-y-0.5">
              <For each={section.rows}>
                {(row) => (
                  <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
                    <span class="min-w-0 truncate text-muted" title={row.label}>
                      {row.label}
                    </span>
                    <span
                      classList={{
                        'text-base-content': !row.muted,
                        'text-muted': row.muted,
                      }}
                      class="font-mono tabular-nums"
                    >
                      {row.value}
                    </span>
                  </div>
                )}
              </For>
            </div>
          </section>
        )}
      </For>
    </AgentMachineMetricTooltip>
  );
};

const AgentMachineNetworkInterfacesList: Component<{
  interfaces: AgentMachineNetworkInterfaceDetail[];
  maxInterfaces?: number;
  maxAddressesPerInterface?: number;
}> = (props) => {
  const maxInterfaces = () => props.maxInterfaces ?? 8;
  const maxAddressesPerInterface = () => props.maxAddressesPerInterface ?? 4;
  const shownInterfaces = () => props.interfaces.slice(0, maxInterfaces());
  const hiddenInterfaceCount = () =>
    Math.max(0, props.interfaces.length - shownInterfaces().length);

  return (
    <div class="max-h-[280px] space-y-1.5 overflow-y-auto pr-1">
      <For each={shownInterfaces()}>
        {(iface, index) => (
          <div class="min-w-0" classList={{ 'border-t border-border pt-1.5': index() > 0 }}>
            <div class="flex min-w-0 items-center gap-2">
              <span class="min-w-0 truncate font-semibold text-base-content" title={iface.name}>
                {iface.name}
              </span>
              <Show when={iface.mac}>
                {(mac) => (
                  <span
                    class="max-w-[150px] shrink-0 truncate font-mono text-[9px] text-muted"
                    title={mac()}
                  >
                    {mac()}
                  </span>
                )}
              </Show>
            </div>
            <Show
              when={iface.addresses.length > 0}
              fallback={
                <div class="mt-0.5 text-[9px] text-muted">
                  {getWorkloadsGuestNetworkEmptyState()}
                </div>
              }
            >
              <div class="mt-1 flex flex-wrap gap-1">
                <For each={iface.addresses.slice(0, maxAddressesPerInterface())}>
                  {(address) => (
                    <span
                      class="max-w-full truncate rounded border border-border bg-surface-alt px-1.5 py-0.5 font-mono text-[9px] text-base-content"
                      title={address}
                    >
                      {address}
                    </span>
                  )}
                </For>
                <Show when={iface.addresses.length > maxAddressesPerInterface()}>
                  <span class="rounded border border-border px-1.5 py-0.5 text-[9px] text-muted">
                    +{iface.addresses.length - maxAddressesPerInterface()}
                  </span>
                </Show>
              </div>
            </Show>
            <Show
              when={
                iface.rxBytes !== undefined ||
                iface.txBytes !== undefined ||
                iface.speedMbps !== undefined
              }
            >
              <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[9px] text-muted">
                {/* Interface counters are cumulative totals, not rates — format as bytes. */}
                <Show when={iface.rxBytes !== undefined || iface.txBytes !== undefined}>
                  <span>
                    <span class="font-mono text-emerald-500">RX</span>{' '}
                    {formatBytes(iface.rxBytes ?? 0)}
                  </span>
                  <span>
                    <span class="font-mono text-orange-400">TX</span>{' '}
                    {formatBytes(iface.txBytes ?? 0)}
                  </span>
                </Show>
                <Show when={iface.speedMbps !== undefined}>
                  <span>{iface.speedMbps} Mbps</span>
                </Show>
              </div>
            </Show>
          </div>
        )}
      </For>
      <Show when={hiddenInterfaceCount() > 0}>
        <div class="border-t border-border pt-1 text-[9px] text-muted">
          +{hiddenInterfaceCount()} more interfaces
        </div>
      </Show>
    </div>
  );
};

const AgentMachineNetworkCell: Component<{
  network: Resource['network'] | undefined;
  interfaces: AgentMachineNetworkInterfaceDetail[];
  title: string;
}> = (props) => {
  const hasDetails = () => props.interfaces.length > 0;
  const ariaLabel = () => props.title.replace(/\n/g, ', ') || 'Network throughput';

  return (
    <AgentMachineMetricTooltip
      triggerDataAttribute="data-agent-machine-network-trigger"
      tooltipDataAttribute="data-agent-machine-network-tooltip"
      triggerClass="grid w-full grid-cols-[0.75rem_minmax(0,1fr)] grid-rows-2 items-center gap-x-1 gap-y-0.5 text-[11px] leading-tight tabular-nums"
      tooltipClass="min-w-[230px] max-w-[360px] space-y-2"
      enabled={hasDetails()}
      ariaLabel={ariaLabel()}
      title={props.title}
      maxWidth={380}
      trigger={
        <>
          <span class="inline-flex w-3 justify-center text-emerald-500">↓</span>
          <span class="min-w-0 truncate">{formatSpeed(props.network?.rxBytes ?? 0)}</span>
          <span class="inline-flex w-3 justify-center text-orange-400">↑</span>
          <span class="min-w-0 truncate">{formatSpeed(props.network?.txBytes ?? 0)}</span>
        </>
      }
    >
      <section>
        <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">
          Network Interfaces
        </div>
        <AgentMachineNetworkInterfacesList interfaces={props.interfaces} />
      </section>
    </AgentMachineMetricTooltip>
  );
};

const AgentMachineIpCell: Component<{
  primaryIp: string;
  ipValues: string[];
  interfaces: AgentMachineNetworkInterfaceDetail[];
}> = (props) => {
  const hasDetails = () => props.ipValues.length > 0 || props.interfaces.length > 0;
  const shownIps = () => props.ipValues.slice(0, 12);
  const hiddenIpCount = () => Math.max(0, props.ipValues.length - shownIps().length);
  const additionalIpCount = () => Math.max(0, props.ipValues.length - (props.primaryIp ? 1 : 0));
  const title = () => props.ipValues.join('\n') || props.primaryIp || '—';
  const ariaLabel = () =>
    hasDetails() ? `IP details: ${props.ipValues.join(', ') || props.primaryIp}` : 'IP unavailable';

  return (
    <AgentMachineMetricTooltip
      triggerDataAttribute="data-agent-machine-ip-trigger"
      tooltipDataAttribute="data-agent-machine-ip-tooltip"
      triggerClass="inline-flex max-w-full min-w-0 justify-start"
      tooltipClass="min-w-[230px] max-w-[380px] space-y-2"
      enabled={hasDetails()}
      ariaLabel={ariaLabel()}
      title={title()}
      maxWidth={400}
      trigger={
        <span class="inline-flex max-w-full min-w-0 items-center gap-1.5">
          <span
            class="min-w-0 truncate font-mono text-[11px]"
            classList={{
              'text-base-content': Boolean(props.primaryIp),
              'text-muted': !props.primaryIp,
            }}
          >
            {props.primaryIp || '—'}
          </span>
          <Show when={additionalIpCount() > 0}>
            <span class="shrink-0 rounded border border-border px-1 py-0.5 text-[9px] text-muted">
              +{additionalIpCount()}
            </span>
          </Show>
        </span>
      }
    >
      <section>
        <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">IP Addresses</div>
        <Show
          when={shownIps().length > 0}
          fallback={<div class="text-[9px] text-muted">{getWorkloadsGuestNetworkEmptyState()}</div>}
        >
          <div class="flex max-h-[120px] flex-wrap gap-1 overflow-y-auto pr-1">
            <For each={shownIps()}>
              {(address) => (
                <span
                  class="max-w-full truncate rounded border border-border bg-surface-alt px-1.5 py-0.5 font-mono text-[9px] text-base-content"
                  title={address}
                >
                  {address}
                </span>
              )}
            </For>
            <Show when={hiddenIpCount() > 0}>
              <span class="rounded border border-border px-1.5 py-0.5 text-[9px] text-muted">
                +{hiddenIpCount()}
              </span>
            </Show>
          </div>
        </Show>
      </section>
      <Show when={props.interfaces.length > 0}>
        <section>
          <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">
            Network Interfaces
          </div>
          <AgentMachineNetworkInterfacesList
            interfaces={props.interfaces}
            maxInterfaces={6}
            maxAddressesPerInterface={4}
          />
        </section>
      </Show>
    </AgentMachineMetricTooltip>
  );
};

const AgentMachineDiskIOCell: Component<{
  diskIO: Resource['diskIO'] | undefined;
  details: AgentMachineDiskIODetail[];
  title: string;
}> = (props) => {
  const hasDetails = () => props.details.length > 0;
  const shownDetails = () => props.details.slice(0, 8);
  const hiddenDetailCount = () => Math.max(0, props.details.length - shownDetails().length);
  const ariaLabel = () => props.title.replace(/\n/g, ', ') || 'Disk I/O throughput';
  const hasCounterValue = (disk: AgentMachineDiskIODetail) =>
    disk.readBytes !== undefined ||
    disk.writeBytes !== undefined ||
    disk.readOps !== undefined ||
    disk.writeOps !== undefined ||
    disk.readTimeMs !== undefined ||
    disk.writeTimeMs !== undefined ||
    disk.ioTimeMs !== undefined;

  return (
    <AgentMachineMetricTooltip
      triggerDataAttribute="data-agent-machine-diskio-trigger"
      tooltipDataAttribute="data-agent-machine-diskio-tooltip"
      triggerClass="grid w-full grid-cols-[0.75rem_minmax(0,1fr)] grid-rows-2 items-center gap-x-1 gap-y-0.5 text-[11px] leading-tight tabular-nums"
      tooltipClass="min-w-[230px] max-w-[360px] space-y-2"
      enabled={hasDetails()}
      ariaLabel={ariaLabel()}
      title={props.title}
      maxWidth={380}
      trigger={
        <>
          <span class="inline-flex w-3 justify-center font-mono text-blue-500">R</span>
          <span class="min-w-0 truncate">{formatSpeed(props.diskIO?.readRate ?? 0)}</span>
          <span class="inline-flex w-3 justify-center font-mono text-amber-500">W</span>
          <span class="min-w-0 truncate">{formatSpeed(props.diskIO?.writeRate ?? 0)}</span>
        </>
      }
    >
      <section>
        <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">Disk I/O</div>
        <div class="mb-1 grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-0.5 text-[9px]">
          <span class="font-mono text-blue-500">Read</span>
          <span class="min-w-0 truncate text-base-content">
            {formatSpeed(props.diskIO?.readRate ?? 0)}
          </span>
          <span class="font-mono text-amber-500">Write</span>
          <span class="min-w-0 truncate text-base-content">
            {formatSpeed(props.diskIO?.writeRate ?? 0)}
          </span>
        </div>
        <div class="max-h-[280px] space-y-1.5 overflow-y-auto pr-1">
          <For each={shownDetails()}>
            {(disk, index) => (
              <div class="min-w-0" classList={{ 'border-t border-border pt-1.5': index() > 0 }}>
                <div class="truncate font-semibold text-base-content" title={disk.device}>
                  {disk.device}
                </div>
                <Show
                  when={hasCounterValue(disk)}
                  fallback={<div class="mt-0.5 text-[9px] text-muted">No device counters</div>}
                >
                  <div class="mt-1 grid grid-cols-[auto_minmax(0,1fr)_auto_minmax(0,1fr)] gap-x-2 gap-y-0.5 text-[9px] text-muted">
                    <Show when={disk.readBytes !== undefined || disk.writeBytes !== undefined}>
                      <span class="font-mono text-blue-500">R</span>
                      <span class="min-w-0 truncate">{formatBytes(disk.readBytes ?? 0)}</span>
                      <span class="font-mono text-amber-500">W</span>
                      <span class="min-w-0 truncate">{formatBytes(disk.writeBytes ?? 0)}</span>
                    </Show>
                    <Show when={disk.readOps !== undefined || disk.writeOps !== undefined}>
                      <span class="font-mono text-blue-500">RO</span>
                      <span class="min-w-0 truncate">{Math.round(disk.readOps ?? 0)}</span>
                      <span class="font-mono text-amber-500">WO</span>
                      <span class="min-w-0 truncate">{Math.round(disk.writeOps ?? 0)}</span>
                    </Show>
                    <Show when={disk.ioTimeMs !== undefined}>
                      <span class="font-mono">IO</span>
                      <span class="min-w-0 truncate">{Math.round(disk.ioTimeMs ?? 0)} ms</span>
                    </Show>
                  </div>
                </Show>
              </div>
            )}
          </For>
          <Show when={hiddenDetailCount() > 0}>
            <div class="border-t border-border pt-1 text-[9px] text-muted">
              +{hiddenDetailCount()} more devices
            </div>
          </Show>
        </div>
      </section>
    </AgentMachineMetricTooltip>
  );
};

const AgentMachineRaidCell: Component<{
  arrays: AgentMachineRaidArrayDetail[];
  summary: string;
}> = (props) => {
  const hasDetails = () => props.arrays.length > 0;
  const shownArrays = () => props.arrays.slice(0, 6);
  const hiddenArrayCount = () => Math.max(0, props.arrays.length - shownArrays().length);
  const triggerLabel = () => props.summary || '—';
  const stateLabel = (state: string) => asTrimmedString(state) ?? 'unknown';
  const arrayLabel = (array: AgentMachineRaidArrayDetail) =>
    asTrimmedString(array.name) ?? asTrimmedString(array.device) ?? 'RAID array';
  const rebuildWidth = (percent: number) => `${Math.min(100, Math.max(0, percent))}%`;

  return (
    <AgentMachineMetricTooltip
      triggerDataAttribute="data-agent-machine-raid-trigger"
      tooltipDataAttribute="data-agent-machine-raid-tooltip"
      triggerClass="inline-flex max-w-full justify-start"
      tooltipClass="min-w-[250px] max-w-[380px] space-y-2"
      enabled={hasDetails()}
      ariaLabel={hasDetails() ? `RAID details: ${triggerLabel()}` : 'RAID unavailable'}
      title={triggerLabel()}
      maxWidth={400}
      trigger={
        <span
          class="max-w-full truncate text-[11px] font-medium"
          classList={{
            'text-base-content': hasDetails(),
            'text-muted': !hasDetails(),
          }}
        >
          {triggerLabel()}
        </span>
      }
    >
      <section>
        <div class="mb-1 border-b border-border pb-1 font-semibold text-muted">RAID Arrays</div>
        <div class="max-h-[300px] space-y-2 overflow-y-auto pr-1">
          <For each={shownArrays()}>
            {(array, index) => {
              const rebuilding = () =>
                Number.isFinite(array.rebuildPercent) && array.rebuildPercent > 0;

              return (
                <div class="min-w-0" classList={{ 'border-t border-border pt-2': index() > 0 }}>
                  <div class="flex min-w-0 items-start justify-between gap-2">
                    <div class="min-w-0">
                      <div
                        class="truncate font-semibold text-base-content"
                        title={arrayLabel(array)}
                      >
                        {arrayLabel(array)}
                      </div>
                      <div class="mt-0.5 flex flex-wrap gap-x-2 gap-y-0.5 text-[9px] text-muted">
                        <span class="font-mono uppercase">{array.level}</span>
                        <Show when={array.device && array.device !== arrayLabel(array)}>
                          <span class="font-mono">{array.device}</span>
                        </Show>
                      </div>
                    </div>
                    <div class="flex shrink-0 items-center gap-1.5" title={stateLabel(array.state)}>
                      <StatusDot variant={getRaidStateVariant(array.state)} size="xs" ariaHidden />
                      <span class={`text-[10px] font-medium ${getRaidStateTextClass(array.state)}`}>
                        {stateLabel(array.state)}
                      </span>
                    </div>
                  </div>

                  <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[9px] text-muted">
                    <span>
                      Active <span class="font-mono text-base-content">{array.activeDevices}</span>/
                      <span class="font-mono text-base-content">{array.totalDevices}</span>
                    </span>
                    <span>
                      Working{' '}
                      <span class="font-mono text-base-content">{array.workingDevices}</span>
                    </span>
                    <Show when={array.spareDevices > 0}>
                      <span>
                        Spare <span class="font-mono text-base-content">{array.spareDevices}</span>
                      </span>
                    </Show>
                    <Show when={array.failedDevices > 0}>
                      <span>
                        Failed{' '}
                        <span class="font-mono text-red-600 dark:text-red-400">
                          {array.failedDevices}
                        </span>
                      </span>
                    </Show>
                  </div>

                  <Show when={rebuilding()}>
                    <div class="mt-1.5">
                      <div class="mb-0.5 flex items-center justify-between gap-3 text-[9px]">
                        <span class="text-amber-600 dark:text-amber-400">Rebuilding</span>
                        <span class="font-mono text-base-content">
                          {Math.round(array.rebuildPercent)}%
                        </span>
                      </div>
                      <div class="h-1 overflow-hidden rounded-full bg-surface-alt">
                        <div
                          class="h-full rounded-full bg-amber-500"
                          style={{ width: rebuildWidth(array.rebuildPercent) }}
                        />
                      </div>
                      <Show when={array.rebuildSpeed}>
                        {(speed) => <div class="mt-0.5 text-[9px] text-muted">{speed()}</div>}
                      </Show>
                    </div>
                  </Show>

                  <Show when={array.devices.length > 0}>
                    <div class="mt-1.5 flex flex-wrap gap-1">
                      <For each={array.devices.slice(0, 12)}>
                        {(device) => (
                          <span
                            class={`inline-flex max-w-full items-center truncate rounded border px-1.5 py-0.5 text-[9px] font-medium ${getRaidDeviceBadgeClass(device)}`}
                            title={`slot ${device.slot} - ${device.state}`}
                          >
                            {device.device}
                          </span>
                        )}
                      </For>
                      <Show when={array.devices.length > 12}>
                        <span class="rounded border border-border px-1.5 py-0.5 text-[9px] text-muted">
                          +{array.devices.length - 12}
                        </span>
                      </Show>
                    </div>
                  </Show>
                </div>
              );
            }}
          </For>
          <Show when={hiddenArrayCount() > 0}>
            <div class="border-t border-border pt-1 text-[9px] text-muted">
              +{hiddenArrayCount()} more arrays
            </div>
          </Show>
        </div>
      </section>
    </AgentMachineMetricTooltip>
  );
};

const AgentMachineWebLinkCell: Component<{
  url: string;
  name: string;
  canConfigure: boolean;
  onConfigure: () => void;
}> = (props) => {
  const url = () => asTrimmedString(props.url) ?? '';
  const label = () => `Open web interface for ${props.name}`;
  const configureLabel = () => `Add web interface URL for ${props.name}`;

  return (
    <div class="flex justify-center">
      <Show
        when={url()}
        fallback={
          <Show
            when={props.canConfigure}
            fallback={
              <span class="inline-flex h-7 w-7 items-center justify-center text-xs text-muted">
                —
              </span>
            }
          >
            <button
              type="button"
              class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
              title="Add web interface URL"
              aria-label={configureLabel()}
              onClick={(event) => {
                event.stopPropagation();
                props.onConfigure();
              }}
              onKeyDown={(event) => event.stopPropagation()}
            >
              <PlusIcon class="h-3.5 w-3.5 opacity-60" />
            </button>
          </Show>
        }
      >
        {(href) => (
          <a
            data-agent-machine-web-link
            href={href()}
            target="_blank"
            rel="noopener noreferrer"
            class="inline-flex h-7 w-7 items-center justify-center rounded-md text-blue-600 transition-colors hover:bg-blue-50 hover:text-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 dark:text-blue-400 dark:hover:bg-blue-950 dark:hover:text-blue-300"
            title={href()}
            aria-label={label()}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => event.stopPropagation()}
          >
            <ExternalLinkIcon class="h-3.5 w-3.5" />
          </a>
        )}
      </Show>
    </div>
  );
};

const AgentMachineActionsCell: Component<{
  name: string;
  agentId: string;
  menuOpen: boolean;
  confirmingRemoval: boolean;
  removing: boolean;
  onToggleMenu: () => void;
  onRemove: () => void;
}> = (props) => {
  const removeLabel = () => `Remove ${props.name} from Pulse`;
  const confirmLabel = () => `Confirm remove ${props.name} from Pulse`;
  const [menuPosition, setMenuPosition] = createSignal({ left: 0, top: 0 });
  let triggerRef: HTMLButtonElement | undefined;
  const updateMenuPosition = () => {
    if (!triggerRef || typeof window === 'undefined') return;
    const rect = triggerRef.getBoundingClientRect();
    const menuWidth = 240;
    setMenuPosition({
      left: Math.max(8, Math.min(window.innerWidth - menuWidth - 8, rect.right - menuWidth)),
      top: rect.bottom + 4,
    });
  };

  createEffect(() => {
    if (!props.menuOpen || typeof window === 'undefined') return;
    updateMenuPosition();
    window.addEventListener('resize', updateMenuPosition);
    window.addEventListener('scroll', updateMenuPosition, true);
    onCleanup(() => {
      window.removeEventListener('resize', updateMenuPosition);
      window.removeEventListener('scroll', updateMenuPosition, true);
    });
  });

  return (
    <div
      data-agent-machine-actions-root
      class="relative flex justify-center"
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => {
        event.stopPropagation();
        if (event.key === 'Escape' && props.menuOpen) props.onToggleMenu();
      }}
    >
      <button
        ref={triggerRef}
        type="button"
        class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
        title="Machine actions"
        aria-label={`Machine actions for ${props.name}`}
        aria-haspopup="menu"
        aria-expanded={props.menuOpen ? 'true' : 'false'}
        onClick={(event) => {
          event.stopPropagation();
          updateMenuPosition();
          props.onToggleMenu();
        }}
      >
        <MoreHorizontalIcon class="h-3.5 w-3.5" />
      </button>

      <Show when={props.menuOpen}>
        <Portal mount={document.body}>
          <div
            data-agent-machine-actions-root
            data-agent-machine-actions-menu
            role="menu"
            class="fixed z-[9999] w-60 rounded-md border border-border bg-surface p-1 text-left shadow-lg"
            style={{
              left: `${menuPosition().left}px`,
              top: `${menuPosition().top}px`,
            }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={(event) => event.stopPropagation()}
            onKeyDown={(event) => {
              event.stopPropagation();
              if (event.key === 'Escape' && props.menuOpen) props.onToggleMenu();
            }}
          >
            <button
              type="button"
              role="menuitem"
              class="flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-xs font-medium text-red-600 transition-colors hover:bg-red-50 focus:outline-none focus-visible:ring-2 focus-visible:ring-red-500/60 disabled:cursor-not-allowed disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-950"
              aria-label={props.confirmingRemoval ? confirmLabel() : removeLabel()}
              disabled={props.removing || !props.agentId}
              onClick={(event) => {
                event.stopPropagation();
                props.onRemove();
              }}
            >
              <Show
                when={props.removing}
                fallback={<Trash2Icon class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />}
              >
                <Loader2Icon class="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />
              </Show>
              <span class="min-w-0 truncate">
                {props.removing
                  ? 'Removing...'
                  : props.confirmingRemoval
                    ? 'Click again to confirm'
                    : 'Remove from Pulse'}
              </span>
            </button>

            <Show when={props.confirmingRemoval}>
              <p class="px-2 pb-1.5 pt-1 text-[10px] leading-snug text-muted">
                Pulse will forget this machine. The agent stays installed until you run the
                uninstall command.
              </p>
            </Show>
          </div>
        </Portal>
      </Show>
    </div>
  );
};

const availabilityFor = (machine: Resource): ResourceAvailabilityMeta | undefined =>
  machine.availability ??
  (machine.platformData?.availability as ResourceAvailabilityMeta | undefined);

const isAgentlessMachine = (machine: Resource): boolean =>
  machine.type !== 'agent' &&
  machine.sourceType !== 'agent' &&
  machine.platformType !== 'agent' &&
  String(availabilityFor(machine)?.targetKind ?? '')
    .trim()
    .toLowerCase() === 'machine';

const availabilityAddressFor = (machine: Resource): string => {
  const availability = availabilityFor(machine);
  const address = asTrimmedString(availability?.address);
  if (address) return address;
  const identityWithIPAddresses = machine.identity as
    | (Resource['identity'] & { ipAddresses?: string[] })
    | undefined;
  const firstIP = asTrimmedString(
    identityWithIPAddresses?.ipAddresses?.[0] ?? machine.identity?.ips?.[0],
  );
  if (firstIP) return firstIP;
  return asTrimmedString(machine.identity?.hostname) ?? '';
};

const memoryTotalFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.total) ?? finiteMetric(machine.agent?.memory?.total) ?? 0;

const memoryUsedFor = (machine: Resource): number =>
  finiteMetric(machine.memory?.used) ?? finiteMetric(machine.agent?.memory?.used) ?? 0;

const memoryPercentOnlyFor = (machine: Resource): number | undefined => {
  if (memoryTotalFor(machine) > 0) return undefined;
  return finiteMetric(machine.memory?.current) ?? finiteMetric(machine.agent?.memory?.usage);
};

const memoryBalloonFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.balloon);

const memoryCacheFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.cache);

const memorySwapUsedFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.swapUsed);

const memorySwapTotalFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.memory?.swapTotal);

const cpuCoresFor = (machine: Resource): number | undefined => {
  const cores = finiteMetric(machine.agent?.cpuCount);
  return cores && cores > 0 ? cores : undefined;
};

const cpuLoadAverageFor = (machine: Resource): number | undefined =>
  finiteMetric(machine.agent?.loadAverage?.[0]);

const aggregateDiskFor = (machine: Resource): Disk | undefined => {
  if (!machine.disk) return undefined;
  const total = finiteMetric(machine.disk.total) ?? 0;
  const used = finiteMetric(machine.disk.used) ?? 0;
  const free = finiteMetric(machine.disk.free) ?? (total > 0 ? Math.max(0, total - used) : 0);
  const usage =
    total > 0 && used > 0 ? (used / total) * 100 : (finiteMetric(machine.disk.current) ?? 0);
  if (total <= 0 && usage <= 0) return undefined;
  return { total, used, free, usage };
};

const titleCase = (value: string): string =>
  value
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
    .join(' ');

const SYSTEM_NAME_OVERRIDES = new Map<string, string>([
  ['darwin', 'macOS'],
  ['freebsd', 'FreeBSD'],
  ['linux', 'Linux'],
  ['mac os', 'macOS'],
  ['macos', 'macOS'],
  ['microsoft windows', 'Windows'],
  ['unraid', 'Unraid'],
  ['win32', 'Windows'],
  ['windows', 'Windows'],
]);

const systemNameForDisplay = (value: string, titleCaseFallback = false): string => {
  const trimmed = value.trim();
  if (!trimmed) return '';

  const normalized = trimmed.toLowerCase().replace(/[_-]+/g, ' ').replace(/\s+/g, ' ');
  return (
    SYSTEM_NAME_OVERRIDES.get(normalized) ?? (titleCaseFallback ? titleCase(trimmed) : trimmed)
  );
};

const normalizedIdentityToken = (value: string | undefined): string =>
  (value ?? '').trim().toLowerCase();

const appendUniqueMachineSubtitlePart = (parts: string[], value: string | undefined) => {
  const normalized = normalizedIdentityToken(value);
  if (!normalized || parts.some((part) => normalizedIdentityToken(part) === normalized)) return;
  parts.push(value?.trim() ?? '');
};

const machineRowSubtitleFor = (
  name: string,
  hostname: string | undefined,
  primaryIp: string,
  lastSeen: string,
  includePrimaryIp: boolean,
  includeLastSeen: boolean,
): string => {
  const parts: string[] = [];

  if (normalizedIdentityToken(hostname) !== normalizedIdentityToken(name)) {
    appendUniqueMachineSubtitlePart(parts, hostname);
  }
  if (includePrimaryIp) {
    appendUniqueMachineSubtitlePart(parts, primaryIp);
  }
  if (includeLastSeen && lastSeen && lastSeen !== '—') {
    appendUniqueMachineSubtitlePart(parts, `seen ${lastSeen}`);
  }

  return parts.slice(0, 2).join(' | ');
};

const systemLabelFor = (machine: Resource): string => {
  if (isAgentlessMachine(machine)) {
    const protocol = (asTrimmedString(availabilityFor(machine)?.protocol) ?? '').toUpperCase();
    return protocol ? `${protocol} availability` : 'Agentless availability';
  }
  const osName = asTrimmedString(machine.agent?.osName);
  const osVersion = asTrimmedString(machine.agent?.osVersion);
  if (osName && osVersion) return `${systemNameForDisplay(osName)} ${osVersion}`;
  if (osName) return systemNameForDisplay(osName);
  return systemNameForDisplay(
    asTrimmedString(machine.agent?.platform) || asTrimmedString(machine.technology) || 'Agent',
    true,
  );
};

const agentVersionFor = (machine: Resource): string =>
  isAgentlessMachine(machine) ? 'Agentless' : asTrimmedString(machine.agent?.agentVersion) || '—';

const outdatedAgentTelemetryFallbackFor = (
  machine: Resource,
  targetAgentVersion: string | null | undefined,
): MetricFallbackReason | undefined => {
  if (isAgentlessMachine(machine)) return undefined;

  const agentVersion = hostAgentVersion(machine);
  const comparison = compareAgentVersions(agentVersion, targetAgentVersion);
  if (comparison === null || comparison >= 0) return undefined;

  const current = formatAgentVersionDisplay(agentVersion) || agentVersion || 'this version';
  const target = formatAgentVersionDisplay(targetAgentVersion) || targetAgentVersion;
  return {
    label: 'old agent',
    title: target
      ? `Update this agent from ${current} to ${target} for full machine telemetry.`
      : `Update this agent from ${current} for full machine telemetry.`,
  };
};

const filterAgentMachineResources = (
  resources: Resource[],
  search: string,
  status: PlatformResourceStatusFilter,
): Resource[] =>
  filterPlatformResources(resources, '', status).filter((machine) =>
    matchesAgentMachineSearch(machine, search, systemLabelFor, agentVersionFor),
  );

const AGENT_MACHINE_SEARCH_TIPS = {
  popoverId: 'machines-search-help',
  intro: 'Filter Pulse Agent machines by identity, platform, or reported inventory.',
  tips: [
    { code: 'mac-mini', description: 'Match hostnames and display names' },
    { code: '192.168.0.98', description: 'Find reported IP addresses' },
    { code: 'macos', description: 'Find operating system names or versions' },
    { code: 'arm64', description: 'Match CPU architecture' },
    { code: 'darwin', description: 'Find kernel or platform details' },
  ],
  footerHighlight: 'Tip',
  footerText: 'Hidden columns such as IP, RAID, Arch, and Kernel are still searchable.',
};

const networkTitleFor = (machine: Resource): string => {
  if (!machine.network) return '';
  return `In ${formatSpeed(machine.network.rxBytes)}\nOut ${formatSpeed(machine.network.txBytes)}`;
};

const diskIOTitleFor = (machine: Resource): string => {
  if (!machine.diskIO) return '';
  return `Read ${formatSpeed(machine.diskIO.readRate)}\nWrite ${formatSpeed(machine.diskIO.writeRate)}`;
};

const agentIdentityIdFor = (machine: Resource): string =>
  asTrimmedString(getActionableAgentIdFromResource(machine)) || asTrimmedString(machine.id) || '';

const agentMetadataIdFor = (machine: Resource): string =>
  isAgentlessMachine(machine) ? '' : agentIdentityIdFor(machine);

const agentRemovalIdFor = (machine: Resource): string => {
  if (isAgentlessMachine(machine)) return '';
  return agentIdentityIdFor(machine);
};

const machineColumnWidthClass = (columnId: AgentMachineColumnId): string => {
  switch (columnId) {
    case 'machine':
      return 'w-[28%] md:w-[15%]';
    case 'system':
      return 'hidden md:table-cell md:w-[12%]';
    case 'agent':
      return 'hidden md:table-cell md:w-[6%]';
    case 'cpu':
    case 'memory':
    case 'disk':
      return 'w-[20%] md:w-[8%]';
    case 'network':
    case 'diskio':
      return 'hidden lg:table-cell lg:w-[9%]';
    case 'uptime':
    case 'temp':
      return 'hidden md:table-cell md:w-[5%]';
    case 'lastSeen':
      return 'hidden lg:table-cell lg:w-[6%]';
    case 'web':
      return 'hidden xl:table-cell xl:w-[4%]';
    case 'ip':
      return 'hidden xl:table-cell xl:w-[8%]';
    case 'raid':
      return 'hidden xl:table-cell xl:w-[6%]';
    case 'arch':
      return 'hidden xl:table-cell xl:w-[5%]';
    case 'kernel':
      return 'hidden xl:table-cell xl:w-[10%]';
    case 'actions':
      return 'w-[12%] md:w-[4%]';
  }
};

const getSortIndicator = (
  activeKey: AgentMachineSortKey,
  direction: 'asc' | 'desc',
  key: AgentMachineSortKey | undefined,
): '▲' | '▼' | '' => {
  if (!key || activeKey !== key) return '';
  return direction === 'asc' ? '▲' : '▼';
};

const getCompactColumnLabel = (column: AgentMachineColumn): string => {
  switch (column.id) {
    case 'uptime':
      return 'Up';
    case 'lastSeen':
      return 'Seen';
    default:
      return column.label;
  }
};

const AgentMachineSortableHead: Component<{
  column: AgentMachineColumn;
  activeSort: AgentMachineSortKey;
  direction: 'asc' | 'desc';
  onSort: (key: AgentMachineSortKey) => void;
}> = (props) => {
  const sortIndicator = () =>
    getSortIndicator(props.activeSort, props.direction, props.column.sortKey);
  const kind = (): NonNullable<AgentMachineColumn['kind']> => props.column.kind ?? 'text';

  return (
    <TableHead
      class={`${getPlatformTableHeadClassForKind(kind())} ${machineColumnWidthClass(props.column.id)}`}
      aria-sort={
        props.column.sortKey && props.activeSort === props.column.sortKey
          ? props.direction === 'asc'
            ? 'ascending'
            : 'descending'
          : undefined
      }
    >
      <Show
        when={props.column.sortKey}
        fallback={
          <span class={props.column.id === 'actions' ? 'sr-only' : 'truncate'}>
            {props.column.label}
          </span>
        }
      >
        {(sortKey) => (
          <button
            type="button"
            class="inline-flex max-w-full items-center gap-1 truncate hover:text-base-content focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60"
            onClick={() => props.onSort(sortKey())}
            aria-label={`Sort by ${props.column.label}`}
          >
            <span class="truncate">
              <Show
                when={props.column.id === 'memory'}
                fallback={getCompactColumnLabel(props.column)}
              >
                <span class="md:hidden">Mem</span>
                <span class="hidden md:inline">Memory</span>
              </Show>
            </span>
            <span class="w-2 shrink-0 text-[9px]" aria-hidden="true">
              {sortIndicator()}
            </span>
          </button>
        )}
      </Show>
    </TableHead>
  );
};

export const AgentsMachinesTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  targetAgentVersion?: string | null;
}> = (props) => {
  const [locallyRemovedResourceIds, setLocallyRemovedResourceIds] = createSignal<
    Record<string, boolean>
  >({});
  const visibleMachineResources = createMemo(() =>
    props.resources.filter((machine) => !locallyRemovedResourceIds()[machine.id]),
  );
  const tableState = createPlatformTableFilterState({
    resources: visibleMachineResources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterAgentMachineResources,
  });
  const alertsActivation = useAlertsActivation();
  const [sortKey, setSortKey] = createSignal<AgentMachineSortKey>('name');
  const [sortDirection, setSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const columnVisibility = useColumnVisibility(
    'pulse:standalone:machines:columns:v3',
    AGENT_MACHINE_COLUMNS,
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'agents-machine-drawer' });
  const [agentMetadataById, setAgentMetadataById] = createSignal<Record<string, AgentMetadata>>({});
  const [accessTargetResourceId, setAccessTargetResourceId] = createSignal<string | null>(null);
  const [actionsMenuResourceId, setActionsMenuResourceId] = createSignal<string | null>(null);
  const [confirmingRemovalResourceId, setConfirmingRemovalResourceId] = createSignal<string | null>(
    null,
  );
  const [removingResourceIds, setRemovingResourceIds] = createSignal<Record<string, boolean>>({});
  const visibleColumns = createMemo(
    () => columnVisibility.visibleColumns() as AgentMachineColumn[],
  );
  const detailColspan = createMemo(() => visibleColumns().length);
  const agentMetadataTargetIds = createMemo(() => {
    const ids = new Set<string>();
    for (const machine of visibleMachineResources()) {
      const metadataId = agentMetadataIdFor(machine);
      if (metadataId) ids.add(metadataId);
    }
    return [...ids].sort();
  });
  const sortedMachines = createMemo(() =>
    sortAgentMachines(
      tableState.filtered(),
      sortKey(),
      sortDirection(),
      systemLabelFor,
      agentVersionFor,
    ),
  );
  const savedAgentCustomUrlFor = (metadataId: string): string =>
    asTrimmedString(agentMetadataById()[metadataId]?.customUrl) ?? '';
  const handleSort = (key: AgentMachineSortKey) => {
    const next = getNextAgentMachineSortState(sortKey(), sortDirection(), key);
    setSortKey(next.key);
    setSortDirection(next.direction);
  };
  const closeActionsMenu = () => {
    setActionsMenuResourceId(null);
    setConfirmingRemovalResourceId(null);
  };
  const toggleActionsMenu = (resourceId: string) => {
    setActionsMenuResourceId((current) => (current === resourceId ? null : resourceId));
    setConfirmingRemovalResourceId(null);
  };
  const isRemovingMachine = (resourceId: string): boolean =>
    Boolean(removingResourceIds()[resourceId]);
  const setMachineRemoving = (resourceId: string, removing: boolean) => {
    setRemovingResourceIds((current) => {
      if (removing) {
        return { ...current, [resourceId]: true };
      }
      const next = { ...current };
      delete next[resourceId];
      return next;
    });
  };
  const removeAgentMetadata = async (metadataId: string) => {
    if (!metadataId) return;
    try {
      await AgentMetadataAPI.deleteMetadata(metadataId);
    } catch (_error) {
      // The agent removal has already succeeded; stale metadata can be cleaned up later.
    }

    setAgentMetadataById((current) => {
      if (!current[metadataId]) return current;
      const next = { ...current };
      delete next[metadataId];
      return next;
    });
  };
  const handleRemoveMachine = async (machine: Resource, agentId: string, name: string) => {
    const resourceId = machine.id;
    if (!agentId) {
      notificationStore.error('Agent identifier unavailable.');
      closeActionsMenu();
      return;
    }

    if (confirmingRemovalResourceId() !== resourceId) {
      setActionsMenuResourceId(resourceId);
      setConfirmingRemovalResourceId(resourceId);
      return;
    }

    setMachineRemoving(resourceId, true);
    try {
      await MonitoringAPI.deleteAgent(agentId);
      await removeAgentMetadata(agentMetadataIdFor(machine));
      setLocallyRemovedResourceIds((current) => ({ ...current, [resourceId]: true }));
      drawer.close(machine);
      if (accessTargetResourceId() === resourceId) setAccessTargetResourceId(null);
      notificationStore.success(`${name} removed from Pulse`);
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to remove machine';
      notificationStore.error(message);
    } finally {
      setMachineRemoving(resourceId, false);
      closeActionsMenu();
    }
  };

  createEffect(() => {
    const targetIds = agentMetadataTargetIds();
    if (targetIds.length === 0) {
      setAgentMetadataById({});
      return;
    }

    let cancelled = false;
    void AgentMetadataAPI.getAllMetadata()
      .then((metadata) => {
        if (!cancelled) setAgentMetadataById(metadata ?? {});
      })
      .catch(() => {
        if (!cancelled) setAgentMetadataById({});
      });

    onCleanup(() => {
      cancelled = true;
    });
  });

  createEffect(() => {
    if (typeof window === 'undefined') return;

    const handleMetadataChanged = (event: Event) => {
      const detail = (event as CustomEvent<ResourceMetadataChangedDetail>).detail;
      if (!detail || detail.metadataKind !== 'agent') return;

      const metadataId = asTrimmedString(detail.metadataId);
      if (!metadataId) return;

      const customUrl = asTrimmedString(detail.customUrl) ?? '';
      setAgentMetadataById((current) => {
        const previous = current[metadataId] ?? { id: metadataId };
        if (previous.customUrl === customUrl) return current;
        return {
          ...current,
          [metadataId]: {
            ...previous,
            customUrl,
          },
        };
      });
    };

    window.addEventListener(RESOURCE_METADATA_CHANGED_EVENT, handleMetadataChanged);
    onCleanup(() => {
      window.removeEventListener(RESOURCE_METADATA_CHANGED_EVENT, handleMetadataChanged);
    });
  });

  createEffect(() => {
    if (typeof document === 'undefined' || !actionsMenuResourceId()) return;

    const handleMouseDown = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest('[data-agent-machine-actions-root]')) {
        return;
      }
      closeActionsMenu();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') closeActionsMenu();
    };

    document.addEventListener('mousedown', handleMouseDown);
    document.addEventListener('keydown', handleKeyDown);
    onCleanup(() => {
      document.removeEventListener('mousedown', handleMouseDown);
      document.removeEventListener('keydown', handleKeyDown);
    });
  });

  return (
    <Show
      when={visibleMachineResources().length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <div class="flex flex-wrap items-center gap-2">
          <div class="min-w-[280px] flex-1">
            <PlatformTableToolbar
              search={tableState.search}
              onSearchChange={tableState.setSearch}
              searchPlaceholder="Search machines"
              searchHistory={{
                storageKey: STORAGE_KEYS.MACHINES_SEARCH_HISTORY,
                emptyMessage: 'Machine searches you run will appear here.',
              }}
              searchTips={AGENT_MACHINE_SEARCH_TIPS}
              status={tableState.status()}
              onStatusChange={tableState.setStatus}
              statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
              visible={tableState.visible()}
              total={tableState.total()}
              rowNoun="machines"
              hasActiveFilters={tableState.hasActiveFilters()}
              onResetFilters={tableState.resetFilters}
            />
          </div>
          <ColumnPicker
            columns={columnVisibility.availableToggles()}
            isHidden={columnVisibility.isHiddenByUser}
            onToggle={columnVisibility.toggle}
            onReset={columnVisibility.resetToDefaults}
          />
        </div>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No machines match current filters"
              description="Adjust the search or status filter to see more Pulse Agent machines."
              actions={<PlatformTableResetFiltersButton onReset={tableState.resetFilters} />}
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Machines" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1210px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <For each={visibleColumns()}>
                    {(column) => (
                      <AgentMachineSortableHead
                        column={column}
                        activeSort={sortKey()}
                        direction={sortDirection()}
                        onSort={handleSort}
                      />
                    )}
                  </For>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedMachines()}>
                  {(machine) => {
                    const name = () => asTrimmedString(machine.name) || machine.id;
                    const hostname = () =>
                      isAgentlessMachine(machine)
                        ? availabilityAddressFor(machine)
                        : asTrimmedString(machine.agent?.hostname) ||
                          asTrimmedString(machine.identity?.hostname);
                    const systemLabel = () => systemLabelFor(machine);
                    const agentVersion = () => agentVersionFor(machine);
                    const indicator = () => getSimpleStatusIndicator(machine.status);
                    const canRenderMetrics = () => indicator().variant !== 'danger';
                    const telemetryFallback = () =>
                      canRenderMetrics()
                        ? outdatedAgentTelemetryFallbackFor(machine, props.targetAgentVersion)
                        : undefined;
                    const metricsKey = () => buildMetricKeyForUnifiedResource(machine);
                    const alertResourceIds = () => hostOverrideIdCandidates(machine);
                    const cpuThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'cpu', alertResourceIds());
                    const memoryThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'memory', alertResourceIds());
                    const diskThresholds = () =>
                      alertsActivation.getMetricThresholds('agent', 'disk', alertResourceIds());
                    const cpuPercent = () => getAgentMachineCpuPercent(machine);
                    const cpuCores = () => cpuCoresFor(machine);
                    const cpuLoadAverage = () => cpuLoadAverageFor(machine);
                    const memoryUsed = () => memoryUsedFor(machine);
                    const memoryTotal = () => memoryTotalFor(machine);
                    const memoryCache = () => memoryCacheFor(machine);
                    const memoryBalloon = () => memoryBalloonFor(machine);
                    const memorySwapUsed = () => memorySwapUsedFor(machine);
                    const memorySwapTotal = () => memorySwapTotalFor(machine);
                    const memoryPercentOnly = () => memoryPercentOnlyFor(machine);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const aggregateDisk = () => aggregateDiskFor(machine);
                    const disks = () => normalizeDiskArray(machine.agent?.disks);
                    const hasDiskMetric = () =>
                      aggregateDisk() !== undefined || (disks()?.length ?? 0) > 0;
                    const networkTotal = () => getAgentMachineNetworkTotal(machine);
                    const networkInterfaces = () => getAgentMachineNetworkInterfaceDetails(machine);
                    const diskIOTotal = () => getAgentMachineDiskIOTotal(machine);
                    const diskIODetails = () => getAgentMachineDiskIODetails(machine);
                    const primaryIp = () => getAgentMachinePrimaryIp(machine);
                    const lastSeenLabel = () =>
                      formatLastSeen(
                        isAgentlessMachine(machine)
                          ? availabilityFor(machine)?.lastChecked
                          : machine.lastSeen,
                      );
                    const machineSubtitle = () =>
                      machineRowSubtitleFor(
                        name(),
                        hostname(),
                        primaryIp(),
                        lastSeenLabel(),
                        !columnVisibility.isColumnVisible('ip'),
                        !columnVisibility.isColumnVisible('lastSeen'),
                      );
                    const ipValues = () => getAgentMachineIpValues(machine);
                    const raidArrays = () => getAgentMachineRaidArrayDetails(machine);
                    const raidSummary = () => getAgentMachineRaidSummary(machine);
                    const temperature = () => getAgentMachineTemperatureCelsius(machine);
                    const temperatureSections = () =>
                      getAgentMachineTemperatureDetailSections(machine);
                    const temperatureTitle = () => getAgentMachineTemperatureTitle(machine);
                    const isExpanded = () => drawer.isExpanded(machine);
                    const detailRowId = () => drawer.detailRowId(machine);
                    const agentMetadataId = () => agentMetadataIdFor(machine);
                    const agentRemovalId = () => agentRemovalIdFor(machine);
                    const savedWebInterfaceUrl = () => savedAgentCustomUrlFor(agentMetadataId());
                    const clearAccessTargetIfCurrent = () => {
                      if (accessTargetResourceId() === machine.id) setAccessTargetResourceId(null);
                    };
                    const toggleDetails = () => {
                      const wasExpanded = isExpanded();
                      drawer.toggle(machine);
                      if (wasExpanded) clearAccessTargetIfCurrent();
                    };
                    const handleDetailsActivationKey: JSX.EventHandler<
                      HTMLTableRowElement,
                      KeyboardEvent
                    > = (event) => {
                      if (event.key !== 'Enter' && event.key !== ' ') return;
                      event.preventDefault();
                      toggleDetails();
                    };

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-agents-machine-row={machine.id}
                          onClick={toggleDetails}
                          onKeyDown={handleDetailsActivationKey}
                          tabIndex={0}
                        >
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('name')} ${machineColumnWidthClass('machine')}`}
                          >
                            <div class="flex min-w-0 items-center gap-2">
                              <ChevronRightIcon
                                data-agent-machine-expand-icon
                                class="h-3.5 w-3.5 shrink-0 text-muted transition-transform duration-150"
                                classList={{ 'rotate-90': isExpanded() }}
                                aria-hidden="true"
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={machine.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                            <Show
                              when={machineSubtitle()}
                              fallback={
                                <span
                                  class="mt-0.5 block truncate pl-10 text-[9px] text-muted sm:text-[10px] md:hidden"
                                  title={systemLabel()}
                                >
                                  {systemLabel()}
                                </span>
                              }
                            >
                              {(subtitle) => (
                                <span
                                  class="mt-0.5 block truncate pl-10 text-[9px] text-muted sm:text-[10px]"
                                  title={subtitle()}
                                >
                                  {subtitle()}
                                </span>
                              )}
                            </Show>
                          </TableCell>
                          <Show when={columnVisibility.isColumnVisible('system')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('system')} text-base-content`}
                            >
                              <span class="truncate" title={systemLabel()}>
                                {systemLabel()}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('agent')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('agent')} font-mono text-[11px] text-base-content`}
                            >
                              <span class="truncate" title={agentVersion()}>
                                {agentVersion()}
                              </span>
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('cpu')}`}
                          >
                            <Show
                              when={canRenderMetrics() && cpuPercent() !== undefined}
                              fallback={metricFallback(telemetryFallback())}
                            >
                              <EnhancedCPUBar
                                usage={cpuPercent() ?? 0}
                                loadAverage={cpuLoadAverage()}
                                cores={cpuCores()}
                                thresholds={cpuThresholds()}
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('memory')}`}
                          >
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={metricFallback(telemetryFallback())}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                                cache={memoryCache()}
                                balloon={memoryBalloon()}
                                swapUsed={memorySwapUsed()}
                                swapTotal={memorySwapTotal()}
                                thresholds={memoryThresholds()}
                                resourceId={metricsKey()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} ${machineColumnWidthClass('disk')}`}
                          >
                            <Show
                              when={canRenderMetrics() && hasDiskMetric()}
                              fallback={metricFallback(telemetryFallback())}
                            >
                              <StackedDiskBar
                                mode={(disks()?.length ?? 0) > 1 ? 'vertical-bars' : undefined}
                                disks={disks()}
                                aggregateDisk={aggregateDisk()}
                                thresholds={diskThresholds()}
                              />
                            </Show>
                          </TableCell>
                          <Show when={columnVisibility.isColumnVisible('network')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('network')} text-base-content`}
                            >
                              <Show
                                when={canRenderMetrics() && networkTotal() !== undefined}
                                fallback={metricFallback(telemetryFallback())}
                              >
                                <AgentMachineNetworkCell
                                  network={machine.network}
                                  interfaces={networkInterfaces()}
                                  title={networkTitleFor(machine)}
                                />
                              </Show>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('diskio')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('diskio')} text-base-content`}
                            >
                              <Show
                                when={canRenderMetrics() && diskIOTotal() !== undefined}
                                fallback={metricFallback(telemetryFallback())}
                              >
                                <AgentMachineDiskIOCell
                                  diskIO={machine.diskIO}
                                  details={diskIODetails()}
                                  title={diskIOTitleFor(machine)}
                                />
                              </Show>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('uptime')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('uptime')} text-base-content`}
                            >
                              {formatUptime(machine.uptime ?? machine.agent?.uptimeSeconds)}
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('temp')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('temp')} text-base-content`}
                            >
                              <AgentMachineTemperatureCell
                                celsius={temperature()}
                                sections={temperatureSections()}
                                title={temperatureTitle()}
                              />
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('lastSeen')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('numeric-value')} ${machineColumnWidthClass('lastSeen')} text-base-content`}
                            >
                              {lastSeenLabel()}
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('web')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('badge')} ${machineColumnWidthClass('web')} text-base-content`}
                            >
                              <AgentMachineWebLinkCell
                                url={savedWebInterfaceUrl()}
                                name={name()}
                                canConfigure={Boolean(agentMetadataId())}
                                onConfigure={() => {
                                  setAccessTargetResourceId(machine.id);
                                  drawer.open(machine);
                                }}
                              />
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('ip')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('ip')} text-base-content`}
                            >
                              <AgentMachineIpCell
                                primaryIp={primaryIp()}
                                ipValues={ipValues()}
                                interfaces={networkInterfaces()}
                              />
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('raid')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('raid')} text-base-content`}
                            >
                              <AgentMachineRaidCell arrays={raidArrays()} summary={raidSummary()} />
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('arch')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('arch')} text-base-content`}
                            >
                              <span class="truncate" title={machine.agent?.architecture}>
                                {machine.agent?.architecture || '—'}
                              </span>
                            </TableCell>
                          </Show>
                          <Show when={columnVisibility.isColumnVisible('kernel')}>
                            <TableCell
                              class={`${getPlatformTableCellClassForKind('text')} ${machineColumnWidthClass('kernel')} text-base-content`}
                            >
                              <span class="truncate" title={machine.agent?.kernelVersion}>
                                {machine.agent?.kernelVersion || '—'}
                              </span>
                            </TableCell>
                          </Show>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('badge')} ${machineColumnWidthClass('actions')} text-base-content`}
                          >
                            <AgentMachineActionsCell
                              name={name()}
                              agentId={agentRemovalId()}
                              menuOpen={actionsMenuResourceId() === machine.id}
                              confirmingRemoval={confirmingRemovalResourceId() === machine.id}
                              removing={isRemovingMachine(machine.id)}
                              onToggleMenu={() => toggleActionsMenu(machine.id)}
                              onRemove={() =>
                                void handleRemoveMachine(machine, agentRemovalId(), name())
                              }
                            />
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={machine}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={detailColspan()}
                          initialShowAccessContext={accessTargetResourceId() === machine.id}
                          onClose={() => {
                            drawer.close(machine);
                            clearAccessTargetIfCurrent();
                          }}
                        />
                      </>
                    );
                  }}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default AgentsMachinesTable;
