import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCard } from '@/components/shared/TableCard';
import { TableCardHeader } from '@/components/shared/TableCardHeader';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  filterPlatformResources,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import type { Resource, ResourceKubernetesPodContainerStatus } from '@/types/resource';
import { compareKubernetesPods, mapKubernetesPodStatus } from './kubernetesPageModel';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const podName = (resource: Resource): string =>
  asTrimmedString(resource.kubernetes?.podName) ||
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  resource.id;

const podScope = (resource: Resource): string => {
  const cluster =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  if (namespace) return cluster ? `${cluster}/${namespace}` : namespace;
  return cluster || 'Cluster';
};

const podPhase = (resource: Resource): string =>
  textValue(resource.kubernetes?.podPhase || resource.kubernetes?.phase);

const podContainers = (resource: Resource): ResourceKubernetesPodContainerStatus[] =>
  resource.kubernetes?.podContainers ?? [];

const readySummary = (resource: Resource): string => {
  const containers = podContainers(resource);
  if (containers.length === 0) return '—';
  const ready = containers.filter((container) => container.ready).length;
  return `${ready}/${containers.length}`;
};

const restartCount = (resource: Resource): number | undefined => {
  if (typeof resource.kubernetes?.restarts === 'number') return resource.kubernetes.restarts;
  const containers = podContainers(resource);
  if (containers.length === 0) return undefined;
  return containers.reduce((total, container) => total + (container.restartCount ?? 0), 0);
};

const ownerValue = (resource: Resource): string => {
  const kind = asTrimmedString(resource.kubernetes?.ownerKind);
  const name = asTrimmedString(resource.kubernetes?.ownerName);
  if (kind && name) return `${kind}/${name}`;
  return kind || name || '—';
};

const imageValue = (resource: Resource): string =>
  textValue(resource.kubernetes?.image || podContainers(resource)[0]?.image);

const ageValue = (resource: Resource): string => {
  const seconds = resource.kubernetes?.uptimeSeconds ?? resource.uptime;
  if (typeof seconds !== 'number' || seconds < 0) return '—';
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3_600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
};

const numericValue = (value: number | undefined): JSX.Element =>
  typeof value === 'number' ? <span class="tabular-nums">{value}</span> : <span>—</span>;

export const KubernetesPodsTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const sortedRows = createMemo(() => [...tableState.filtered()].sort(compareKubernetesPods));

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
            searchPlaceholder="Search pods"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="pods"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No pods match current filters"
              description="Adjust the search or status filter to see more Kubernetes pods."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Pods'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1240px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                    Pod
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Node
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[8%]`}>
                    Phase
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[7%]`}
                  >
                    Ready
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}
                  >
                    Restarts
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Owner
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Image
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[4%]`}
                  >
                    Age
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => mapKubernetesPodStatus(resource);
                    const name = () => podName(resource);
                    const scope = () => podScope(resource);
                    const node = () => textValue(resource.kubernetes?.nodeName);
                    const phase = () => podPhase(resource);
                    const owner = () => ownerValue(resource);
                    const image = () => imageValue(resource);
                    const age = () => ageValue(resource);

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-kubernetes-pod-row={resource.id}
                      >
                        <TableCell class={getPlatformTableCellClassForKind('name')}>
                          <div class="flex min-w-0 items-center gap-2">
                            <StatusDot
                              size="sm"
                              variant={indicator().variant}
                              title={indicator().label}
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
                          <span class="block max-w-full truncate" title={scope()}>
                            {scope()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={node()}>
                            {node()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {phase()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {readySummary(resource)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {numericValue(restartCount(resource))}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={owner()}>
                            {owner()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={image()}>
                            {image()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {age()}
                        </TableCell>
                      </TableRow>
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

export default KubernetesPodsTable;
