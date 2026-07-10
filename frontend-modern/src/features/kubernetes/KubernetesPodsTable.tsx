import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableNumberValue,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
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
import type { Resource, ResourceKubernetesPodContainerStatus } from '@/types/resource';
import {
  compareKubernetesPods,
  filterKubernetesResources,
  kubernetesScopeLabel,
  mapKubernetesPodStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

const podName = (resource: Resource): string =>
  asTrimmedString(resource.kubernetes?.podName) ||
  asTrimmedString(resource.displayName) ||
  asTrimmedString(resource.name) ||
  resource.id;

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
  formatPlatformTableTextValue(resource.kubernetes?.image || podContainers(resource)[0]?.image);

const ageValue = (resource: Resource): string => {
  const seconds = resource.kubernetes?.uptimeSeconds ?? resource.uptime;
  if (typeof seconds !== 'number' || seconds < 0) return '—';
  if (seconds < 60) return `${Math.floor(seconds)}s`;
  if (seconds < 3_600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3_600)}h`;
  return `${Math.floor(seconds / 86_400)}d`;
};

const KUBERNETES_POD_SORT_KEYS = [
  'pod',
  'scope',
  'node',
  'status',
  'ready',
  'restarts',
  'owner',
  'image',
  'age',
] as const;

type KubernetesPodSortKey = (typeof KUBERNETES_POD_SORT_KEYS)[number];

// Scalar per column that user-controlled sorting orders on. Ready sorts by
// the ready fraction (ascending-first, so the least-ready pods surface on
// the first click); '—' style empty renders map to null so rows without
// the datum sink to the bottom.
const getKubernetesPodSortValue = (
  resource: Resource,
  key: KubernetesPodSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'pod':
      return podName(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'node':
      return asTrimmedString(resource.kubernetes?.nodeName) || null;
    case 'status':
      return mapKubernetesPodStatus(resource).label;
    case 'ready': {
      const containers = podContainers(resource);
      if (containers.length === 0) return null;
      return containers.filter((container) => container.ready).length / containers.length;
    }
    case 'restarts':
      return restartCount(resource) ?? null;
    case 'owner': {
      const owner = ownerValue(resource);
      return owner === '—' ? null : owner;
    }
    case 'image': {
      const image = imageValue(resource);
      return image === '—' ? null : image;
    }
    case 'age': {
      const seconds = resource.kubernetes?.uptimeSeconds ?? resource.uptime;
      return typeof seconds === 'number' && seconds >= 0 ? seconds : null;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesPodsTable: Component<{
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
  // User-controlled sorting layered over the attention-first default: rows
  // are pre-sorted by the status compare, so a user sort keeps that order
  // for ties and the table falls straight back to it when the sort clears.
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesPods',
    sortKeys: KUBERNETES_POD_SORT_KEYS,
    descendingFirst: ['restarts', 'age'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows([...tableState.filtered()].sort(compareKubernetesPods), getKubernetesPodSortValue),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-pod-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);

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
          <PlatformTableShell
            title={props.title ?? 'Pods'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1240px]"
            header={
              <>
                <PlatformSortableTableHead kind="name" sort={sort} sortKey="pod" class="md:w-[20%]">
                  Pod
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="hidden md:table-cell md:w-[13%]"
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="node"
                  class="hidden md:table-cell md:w-[13%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} sortKey="status" class="md:w-[8%]">
                  Status
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="ready"
                  class="md:w-[7%]"
                >
                  Ready
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="restarts"
                  class="md:w-[8%]"
                >
                  Restarts
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="owner"
                  class="hidden md:table-cell md:w-[13%]"
                >
                  Owner
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="image"
                  class="hidden md:table-cell md:w-[14%]"
                >
                  Image
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="age"
                  class="hidden md:table-cell md:w-[4%]"
                >
                  Age
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => mapKubernetesPodStatus(resource);
                    const name = () => podName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const node = () => formatPlatformTableTextValue(resource.kubernetes?.nodeName);
                    const owner = () => ownerValue(resource);
                    const image = () => imageValue(resource);
                    const age = () => ageValue(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-pod-row={resource.id}
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
                            {/* The mapped label carries the container failure reason
                              (CrashLoopBackOff, ImagePullBackOff, ...) where the raw
                              phase would still read "Running". */}
                            <span class="block max-w-full truncate" title={indicator().label}>
                              {indicator().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {readySummary(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue value={restartCount(resource)} />
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
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={9}
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

export default KubernetesPodsTable;
