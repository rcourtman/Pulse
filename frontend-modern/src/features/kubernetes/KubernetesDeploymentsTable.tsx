import { For, Show, createMemo, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableRow } from '@/components/shared/Table';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformSortableTableHead,
  PlatformTableNumberValue,
  PlatformTableRelativeTimeValue,
  PlatformTableToolbar,
  PlatformTableEmptyState,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  getPlatformTableDateTimeSortValue,
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
  compareKubernetesDeployments,
  filterKubernetesResources,
  kubernetesClusterLabel,
  mapKubernetesDeploymentStatus,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// Kubernetes Deployments are scheduling abstractions over their controlled
// pods, so the generic infrastructure table's CPU / Memory / Disk I/O /
// Uptime / Temperature columns are conceptually N/A on these rows and
// render as dashes. This deployment-native table reuses canonical shared
// primitives (Card, Table, SearchInput, FilterButtonGroup, StatusDot) but
// surfaces deployment-meaningful columns only: namespace, cluster,
// desired / updated / ready / available replicas, and metadata age.
// observedGeneration is deliberately not a column: without the spec
// generation beside it the raw number is unactionable.

const KUBERNETES_DEPLOYMENT_SORT_KEYS = [
  'deployment',
  'namespace',
  'cluster',
  'desired',
  'updated',
  'ready',
  'available',
  'age',
] as const;

type KubernetesDeploymentSortKey = (typeof KUBERNETES_DEPLOYMENT_SORT_KEYS)[number];

// Replica counts sort on the same `?? 0` fallback the cells render, so a
// deployment displaying 0 orders as 0 instead of sinking as missing.
const getKubernetesDeploymentSortValue = (
  deployment: Resource,
  key: KubernetesDeploymentSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'deployment':
      return asTrimmedString(deployment.name) || deployment.id;
    case 'namespace':
      return asTrimmedString(deployment.kubernetes?.namespace) || null;
    case 'cluster':
      return kubernetesClusterLabel(deployment) || null;
    case 'desired':
      return deployment.kubernetes?.desiredReplicas ?? 0;
    case 'updated':
      return deployment.kubernetes?.updatedReplicas ?? 0;
    case 'ready':
      return deployment.kubernetes?.readyReplicas ?? 0;
    case 'available':
      return deployment.kubernetes?.availableReplicas ?? 0;
    case 'age':
      return getPlatformTableDateTimeSortValue(deployment.kubernetes?.createdAt);
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesDeploymentsTable: Component<{
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
    storageKey: 'kubernetesDeployments',
    sortKeys: KUBERNETES_DEPLOYMENT_SORT_KEYS,
    descendingFirst: ['desired', 'updated', 'ready', 'available', 'age'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(
      [...tableState.filtered()].sort(compareKubernetesDeployments),
      getKubernetesDeploymentSortValue,
    ),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-deployment-drawer' });
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
            searchPlaceholder="Search deployments"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="deployments"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No deployments match current filters"
              description="Adjust the search or status filter to see more deployments."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Deployments'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1320px]"
            header={
              <>
                {/*
                    Desktop widths: Deployment, Namespace, and Cluster take
                    the biggest shares because their content can be long.
                    The integer-count columns (Desired / Updated / Ready /
                    Available) trim to what their headers plus 1-2 digit
                    values need. Mobile widths are unchanged.
                  */}
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="deployment"
                  class="md:w-[25%]"
                >
                  Deployment
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="namespace"
                  class="hidden md:table-cell md:w-[20%]"
                >
                  Namespace
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="cluster"
                  class="hidden md:table-cell md:w-[17%]"
                >
                  Cluster
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="desired"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Desired
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="updated"
                  class="hidden md:table-cell md:w-[8%]"
                >
                  Updated
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
                  sortKey="available"
                  class="md:w-[9%]"
                >
                  Available
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="age"
                  class="md:w-[6%]"
                >
                  Age
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <For each={sortedRows()}>
                  {(deployment) => {
                    const name = () => asTrimmedString(deployment.name) || deployment.id;
                    const ns = () => formatPlatformTableTextValue(deployment.kubernetes?.namespace);
                    const cluster = () => kubernetesClusterLabel(deployment) || '—';
                    const indicator = () => mapKubernetesDeploymentStatus(deployment);
                    const detailRowId = () => drawer.detailRowId(deployment);
                    const isExpanded = () => drawer.isExpanded(deployment);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-deployment-row={deployment.id}
                          onClick={() => drawer.toggle(deployment)}
                          onKeyDown={drawer.handleActivationKey(deployment)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={name()}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(deployment)}
                              />
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                                ariaHidden
                              />
                              <ResourceNameWithWebInterfaceLink
                                name={name()}
                                url={deployment.customUrl}
                                class="min-w-0"
                                nameClass="truncate font-semibold text-base-content"
                              />
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {ns()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {cluster()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableNumberValue
                              value={deployment.kubernetes?.desiredReplicas ?? 0}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                          >
                            <PlatformTableNumberValue
                              value={deployment.kubernetes?.updatedReplicas ?? 0}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={deployment.kubernetes?.readyReplicas ?? 0}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <PlatformTableNumberValue
                              value={deployment.kubernetes?.availableReplicas ?? 0}
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <span title={deployment.kubernetes?.createdAt || ''}>
                              <PlatformTableRelativeTimeValue
                                value={deployment.kubernetes?.createdAt}
                              />
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={deployment}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={8}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(deployment)}
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

export default KubernetesDeploymentsTable;
