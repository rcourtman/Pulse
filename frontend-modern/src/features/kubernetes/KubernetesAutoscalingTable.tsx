import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
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

const autoscalerName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const targetRef = (resource: Resource): string =>
  formatPlatformTableTextValue(
    [resource.kubernetes?.targetKind, resource.kubernetes?.targetName].filter(Boolean).join('/'),
  );

const metricSources = (resource: Resource): { label: string; title: string } => {
  const sources = summarizePlatformTableValues(resource.kubernetes?.metricTypes, { maxVisible: 3 });
  if (sources.values.length === 0)
    return { label: 'Default CPU', title: 'Default CPU utilization' };
  return sources;
};

const labelSummary = (resource: Resource): { label: string; title: string } => {
  return summarizePlatformTableValues(resource.tags);
};

const bounds = (resource: Resource): string => {
  const min = resource.kubernetes?.minReplicas;
  const max = resource.kubernetes?.maxReplicas;
  if (typeof min !== 'number' && typeof max !== 'number') return '—';
  return `${min ?? '—'}-${max ?? '—'}`;
};

// Bounds ("1-10"), Metrics, and Labels are composite summaries with no single
// scalar to order on, so they stay non-sortable.
const KUBERNETES_AUTOSCALING_SORT_KEYS = [
  'autoscaler',
  'scope',
  'target',
  'current',
  'desired',
] as const;

type KubernetesAutoscalingSortKey = (typeof KUBERNETES_AUTOSCALING_SORT_KEYS)[number];

const getKubernetesAutoscalingSortValue = (
  resource: Resource,
  key: KubernetesAutoscalingSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'autoscaler':
      return autoscalerName(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'target': {
      const target = targetRef(resource);
      return target === '—' ? null : target;
    }
    case 'current':
      return resource.kubernetes?.currentReplicas ?? null;
    case 'desired':
      return resource.kubernetes?.desiredReplicas ?? null;
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesAutoscalingTable: Component<{
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-autoscaling-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesAutoscaling',
    sortKeys: KUBERNETES_AUTOSCALING_SORT_KEYS,
    descendingFirst: ['current', 'desired'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getKubernetesAutoscalingSortValue),
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
            searchPlaceholder="Search autoscalers"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="autoscalers"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No autoscalers match current filters"
              description="Adjust the search or status filter to see more Kubernetes autoscalers."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'HorizontalPodAutoscalers'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1080px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="autoscaler"
                  class="md:w-[20%]"
                >
                  Autoscaler
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="hidden md:table-cell md:w-[16%]"
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="target"
                  class="md:w-[18%]"
                >
                  Scale target
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="md:w-[10%]">
                  Bounds
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="current"
                  class="md:w-[9%]"
                >
                  Current
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="desired"
                  class="md:w-[9%]"
                >
                  Desired
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="md:w-[11%]">
                  Metrics
                </PlatformSortableTableHead>
                <PlatformSortableTableHead kind="text" sort={sort} class="hidden md:table-cell md:w-[7%]">
                  Labels
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => autoscalerName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const target = () => targetRef(resource);
                    const metrics = () => metricSources(resource);
                    const labels = () => labelSummary(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-autoscaling-row={resource.id}
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
                            <span class="inline-block max-w-[14rem] truncate" title={target()}>
                              {target()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {bounds(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={resource.kubernetes?.currentReplicas}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={resource.kubernetes?.desiredReplicas}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[10rem] truncate"
                              title={metrics().title}
                            >
                              {metrics().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[8rem] truncate" title={labels().title}>
                              {labels().label}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
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

export default KubernetesAutoscalingTable;
