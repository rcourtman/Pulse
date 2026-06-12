import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { ResponsiveMetricCell } from '@/components/shared/responsive';
import { StackedMemoryBar } from '@/components/Workloads/StackedMemoryBar';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import { formatUptime } from '@/utils/format';
import { formatVmwareClusterServices } from '@/utils/vmwareDisplay';
import {
  formatVmwarePowerState,
  getVmwareResourceDisplayStatus,
  getVmwarePowerStateVariant,
} from './vmwarePageModel';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';

// vSphere ESXi hosts are virtualization hypervisors managed by vCenter,
// not generic Pulse Agents. The generic infrastructure table renders
// dashes for Uptime / Temperature (vCenter inventory does not expose
// host uptime today) and lacks the columns that matter for the cluster
// operator: datacenter, cluster, power state, connection state,
// datastore count, and VM count alongside CPU / Memory utilisation.
// This bespoke table reuses canonical shared primitives and surfaces
// those ESXi-native columns — matching the Docker / Proxmox bar
// treatment so the Overview stack reads as one consistent surface.
// Per-host VM count is computed from the page scope client-side (no
// extra API calls).

const finiteMetric = (value: number | undefined): number | undefined =>
  typeof value === 'number' && Number.isFinite(value) ? value : undefined;

const metricFallback = () => (
  <div class="flex justify-center">
    <span class="text-xs text-muted" aria-hidden="true">
      —
    </span>
  </div>
);

export const VsphereHostsTable: Component<{
  hosts: Resource[];
  // Full vSphere scope so we can count VMs per host without spawning
  // additional fetches.
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.hosts,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: (resources, search, status) =>
      filterPlatformResources(resources, search, status, getVmwareResourceDisplayStatus),
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-host-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

  const vmCountByHost = createMemo(() => {
    const map = new Map<string, number>();
    for (const resource of props.scope) {
      if (resource.type !== 'vm') continue;
      const runtimeHost = asTrimmedString(resource.vmware?.runtimeHostId);
      if (!runtimeHost) continue;
      map.set(runtimeHost, (map.get(runtimeHost) ?? 0) + 1);
    }
    return map;
  });

  return (
    <Show
      when={props.hosts.length > 0}
      fallback={
        <PlatformTableEmptyState
          icon={props.emptyIcon}
          title={props.emptyTitle}
          description={props.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        <Show when={props.showToolbar !== false}>
          <PlatformTableToolbar
            search={tableState.search}
            onSearchChange={tableState.setSearch}
            searchPlaceholder="Search ESXi hosts"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="hosts"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No hosts match current filters"
              description="Adjust the search or status filter to see more hosts."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Hosts'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1240px]"
            header={
              <>
                {/*
                    Desktop widths give the Host FQDN room, balance the CPU
                    and Memory bars, and trim the Datacenter / Cluster /
                    Power / vCenter text columns and the Datastores / VMs
                    integer-count columns to what their content actually
                    needs. Mobile widths are unchanged.
                  */}
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[16%]`}>
                  Host
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[6%]`}
                >
                  Version
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                >
                  Datacenter
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                >
                  Cluster
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[7%]`}
                >
                  Power
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[12%]`}>
                  CPU
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('metric-bar')} md:w-[13%]`}>
                  Memory
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                >
                  Datastores
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[4%]`}>
                  VMs
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[6%]`}
                >
                  Uptime
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[9%]`}
                >
                  vCenter
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(host) => {
                    const meta = () => host.vmware;
                    const name = () => asTrimmedString(host.name) || host.id;
                    const datacenter = () => asTrimmedString(meta()?.datacenterName) || '—';
                    const cluster = () => asTrimmedString(meta()?.clusterName) || '—';
                    const clusterServices = () => formatVmwareClusterServices(meta());
                    const vcenter = () => asTrimmedString(meta()?.vcenterHost) || '—';
                    // ESXi version from agent.osVersion (canonical projection of
                    // HostSystem.config.product.fullName); uptime from the
                    // sys.uptime.latest PerformanceManager counter routed onto
                    // canonical Resource.Uptime.
                    const esxiVersion = () => asTrimmedString(host.agent?.osVersion) || '—';
                    const uptimeLabel = () =>
                      typeof host.uptime === 'number' && host.uptime > 0
                        ? formatUptime(host.uptime, true)
                        : '—';
                    const uptimeFull = () =>
                      typeof host.uptime === 'number' && host.uptime > 0
                        ? formatUptime(host.uptime)
                        : '';
                    const datastoreCount = () =>
                      meta()?.datastoreIds?.length ?? meta()?.datastoreNames?.length ?? 0;
                    const vmCount = () =>
                      vmCountByHost().get(asTrimmedString(meta()?.managedObjectId) || '') ?? 0;
                    const displayStatus = () => getVmwareResourceDisplayStatus(host);
                    const indicator = () => getSimpleStatusIndicator(displayStatus());
                    const metricsKey = () => buildMetricKeyForUnifiedResource(host);
                    const cpuPercent = () => finiteMetric(host.cpu?.current);
                    const memoryTotal = () => finiteMetric(host.memory?.total) ?? 0;
                    const memoryUsed = () => finiteMetric(host.memory?.used) ?? 0;
                    const memoryPercentOnly = () =>
                      memoryTotal() > 0 ? undefined : finiteMetric(host.memory?.current);
                    const hasMemoryMetric = () =>
                      memoryTotal() > 0 || memoryPercentOnly() !== undefined;
                    const canRenderMetrics = () => indicator().variant !== 'muted';
                    const detailRowId = () => drawer.detailRowId(host);
                    const isExpanded = () => drawer.isExpanded(host);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-vsphere-host-row={host.id}
                          onClick={() => drawer.toggle(host)}
                          onKeyDown={drawer.handleActivationKey(host)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={displayStatus() || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                            title={esxiVersion()}
                          >
                            <span class="block truncate">{esxiVersion()}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {datacenter()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={[cluster(), clusterServices()].filter(Boolean).join(' · ')}
                          >
                            <span class="block truncate">{cluster()}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden md:table-cell`}
                          >
                            <div class="flex items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={getVmwarePowerStateVariant(meta()?.powerState)}
                                title={meta()?.powerState || 'unknown'}
                                ariaHidden
                              />
                              <span class="text-base-content">
                                {formatVmwarePowerState(meta()?.powerState)}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <ResponsiveMetricCell
                              class="w-full"
                              value={cpuPercent() ?? 0}
                              type="cpu"
                              resourceId={metricsKey()}
                              isRunning={canRenderMetrics() && cpuPercent() !== undefined}
                              showMobile={false}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('metric-bar')} w-[20%] md:w-auto`}
                          >
                            <Show
                              when={canRenderMetrics() && hasMemoryMetric()}
                              fallback={metricFallback()}
                            >
                              <StackedMemoryBar
                                used={memoryUsed()}
                                total={memoryTotal()}
                                percentOnly={memoryPercentOnly()}
                              />
                            </Show>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {datastoreCount()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {vmCount()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell whitespace-nowrap`}
                            title={uptimeFull()}
                          >
                            {uptimeLabel()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden font-mono text-[11px] text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={vcenter()}>
                              {vcenter()}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={host}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={11}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(host)}
                        />
                      </>
                    );
                  }}
                </For>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default VsphereHostsTable;
