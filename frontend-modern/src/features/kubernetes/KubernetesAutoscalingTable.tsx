import { createMemo, type Component, type JSX } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableNumberValue,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  summarizePlatformTableValues,
  PlatformTableShell,
  type PlatformTableSortValue,
} from '@/features/platformPage/sharedPlatformPage';
import {
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  filterKubernetesResources,
  kubernetesScopeLabel,
  type KubernetesResourceStatusFilter,
  isSingleKubernetesCluster,
} from './kubernetesPageModel';
import { KubernetesScopeText } from './KubernetesScopeText';
import {
  KubernetesDetailRow,
  KubernetesRowLead,
  KubernetesTableFrame,
  createKubernetesDetailState,
} from './KubernetesRecordDetail';

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
  const singleCluster = createMemo(() => isSingleKubernetesCluster(props.resources));
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const drawer = createKubernetesDetailState('kubernetes-autoscaling-drawer');
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
    <KubernetesTableFrame
      hasResources={props.resources.length > 0}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      rowNoun="autoscalers"
      filteredEmptyNoun="Kubernetes autoscalers"
      tableState={tableState}
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
              class="platform-table-mobile-w-30 md:w-[20%]"
            >
              Autoscaler
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="scope"
              class="hidden lg:table-cell md:w-[16%]"
            >
              Scope
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="target"
              class="platform-table-mobile-w-20 md:w-[18%]"
            >
              Target
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="platform-table-mobile-w-20 md:w-[10%]"
            >
              Bounds
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="current"
              class="platform-table-mobile-w-15 md:w-[9%]"
            >
              <PlatformResponsiveTableLabel compact="Now" full="Current" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="numeric-value"
              sort={sort}
              sortKey="desired"
              class="platform-table-mobile-w-15 md:w-[9%]"
            >
              <PlatformResponsiveTableLabel compact="Goal" full="Desired" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="hidden sm:table-cell md:w-[11%]"
            >
              Metrics
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="hidden lg:table-cell md:w-[7%]"
            >
              Labels
            </PlatformSortableTableHead>
          </>
        }
        body={
          <>
            <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
              {(resource) => {
                const indicator = () => getSimpleStatusIndicator(resource.status);
                const name = () => autoscalerName(resource);
                const target = () => targetRef(resource);
                const metrics = () => metricSources(resource);
                const labels = () => labelSummary(resource);
                const isExpanded = () => drawer.isExpanded(resource);

                return (
                  <>
                    <TableRow
                      class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                      data-kubernetes-autoscaling-row={resource.id}
                      onClick={() => drawer.toggle(resource)}
                    >
                      <TableCell class={getPlatformTableCellClassForKind('name')}>
                        <KubernetesRowLead
                          resource={resource}
                          drawer={drawer}
                          label={name()}
                          status={indicator()}
                          dotTitle={resource.status || 'unknown'}
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      >
                        <KubernetesScopeText
                          resource={resource}
                          singleCluster={singleCluster()}
                          class="block truncate"
                        />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span class="block truncate" title={target()}>
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
                        <PlatformTableNumberValue value={resource.kubernetes?.currentReplicas} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                      >
                        <PlatformTableNumberValue value={resource.kubernetes?.desiredReplicas} />
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                      >
                        <span class="block truncate" title={metrics().title}>
                          {metrics().label}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                      >
                        <span class="block truncate" title={labels().title}>
                          {labels().label}
                        </span>
                      </TableCell>
                    </TableRow>
                    <KubernetesDetailRow
                      resource={resource}
                      statusIndicator={indicator()}
                      drawer={drawer}
                      colSpan={8}
                      resolveResourceLabel={resolveResourceLabel}
                    />
                  </>
                );
              }}
            </PlatformWindowedRows>
          </>
        }
      />
    </KubernetesTableFrame>
  );
};

export default KubernetesAutoscalingTable;
