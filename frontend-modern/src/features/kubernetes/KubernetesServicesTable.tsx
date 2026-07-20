import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  summarizePlatformTableValues,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  filterKubernetesResources,
  kubernetesScopeLabel,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// Services have a distinct API contract from Ingress and EndpointSlice:
// operators need the Service type, virtual IP, exposed ports, node ports, and
// selector together rather than in a generic networking/detail table.

const serviceName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const portSummary = (resource: Resource): { label: string; title: string } => {
  const ports = resource.kubernetes?.servicePorts ?? [];
  const labels = ports.map((port) => {
    if (!port.port) return undefined;
    const protocol = port.protocol ? `/${port.protocol.toLowerCase()}` : '';
    const target = port.targetPort ? `:${port.targetPort}` : '';
    const nodePort = port.nodePort ? ` node:${port.nodePort}` : '';
    return `${port.port}${target}${protocol}${nodePort}`;
  });
  return summarizePlatformTableValues(labels);
};

const selectorSummary = (resource: Resource): { label: string; title: string } => {
  const selector = resource.kubernetes?.selector;
  if (!selector || Object.keys(selector).length === 0) return { label: '—', title: '' };
  const pairs = Object.entries(selector)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => `${key}=${value}`);
  return summarizePlatformTableValues(pairs);
};

const externalIpSummary = (resource: Resource): { label: string; title: string } =>
  summarizePlatformTableValues(resource.kubernetes?.externalIps);

// The multi-value summary columns (External IPs, Ports, Selector) carry no
// single scalar to order on, so they stay non-sortable.
const KUBERNETES_SERVICE_SORT_KEYS = ['service', 'scope', 'type', 'clusterIp'] as const;

type KubernetesServiceSortKey = (typeof KUBERNETES_SERVICE_SORT_KEYS)[number];

const getKubernetesServiceSortValue = (
  resource: Resource,
  key: KubernetesServiceSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'service':
      return serviceName(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'type':
      return asTrimmedString(resource.kubernetes?.serviceType) || null;
    case 'clusterIp':
      return asTrimmedString(resource.kubernetes?.clusterIp) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesServicesTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
  externalSearch?: () => string;
  externalStatus?: () => KubernetesResourceStatusFilter;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-service-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesServices',
    sortKeys: KUBERNETES_SERVICE_SORT_KEYS,
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getKubernetesServiceSortValue),
  );

  return (
    <Show
      when={props.resources.length > 0}
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
            searchPlaceholder="Search services"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="services"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No services match current filters"
              description="Adjust the search or status filter to see more Kubernetes services."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Services'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="service"
                  class="md:w-[19%]"
                >
                  Service
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="hidden md:table-cell md:w-[15%]"
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="type"
                  class="md:w-[11%]"
                >
                  Type
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="clusterIp"
                  class="md:w-[13%]"
                >
                  Cluster IP
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[13%]"
                >
                  External IPs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="md:w-[16%]">
                  Ports
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[13%]"
                >
                  Selector
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => serviceName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const serviceType = () =>
                      formatPlatformTableTextValue(resource.kubernetes?.serviceType);
                    const clusterIp = () =>
                      formatPlatformTableTextValue(resource.kubernetes?.clusterIp);
                    const externalIps = () => externalIpSummary(resource);
                    const ports = () => portSummary(resource);
                    const selector = () => selectorSummary(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-service-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={resource.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="truncate font-semibold text-base-content" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={scope()}>
                              {scope()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {serviceType()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {clusterIp()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={externalIps().title}
                            >
                              {externalIps().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="inline-block max-w-[15rem] truncate" title={ports().title}>
                              {ports().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[13rem] truncate"
                              title={selector().title}
                            >
                              {selector().label}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(resource)}
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

export default KubernetesServicesTable;
