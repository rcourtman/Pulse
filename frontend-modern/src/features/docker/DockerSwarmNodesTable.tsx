import { Show, createMemo, type Component } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  getPlatformTableCellClassForKind,
  PlatformTableShell,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import {
  DockerNativeDetailPanel,
  DockerResourceNameCell,
  dockerByteValue,
  dockerCpuValue,
  dockerResourceName,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import {
  compareDockerSwarmNodes,
  filterDockerResources,
  mapDockerSwarmNodeStatus,
  type DockerResourceStatusFilter,
} from './dockerPageModel';
import type { Resource } from '@/types/resource';

const DOCKER_SWARM_NODE_SORT_KEYS = [
  'node',
  'role',
  'availability',
  'reachability',
  'engine',
  'cpus',
  'memory',
  'address',
] as const;

type DockerSwarmNodeSortKey = (typeof DOCKER_SWARM_NODE_SORT_KEYS)[number];

const getDockerSwarmNodeSortValue = (
  resource: Resource,
  key: DockerSwarmNodeSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'node':
      return dockerResourceName(resource);
    case 'role':
      return asTrimmedString(resource.docker?.nodeRole) || null;
    case 'availability':
      return asTrimmedString(resource.docker?.availability) || null;
    case 'reachability':
      return (
        asTrimmedString(
          resource.docker?.leader ? 'leader' : resource.docker?.managerReachability,
        ) || null
      );
    case 'engine':
      return (
        asTrimmedString(resource.docker?.engineVersion || resource.docker?.runtimeVersion) || null
      );
    case 'cpus':
      return typeof resource.docker?.nanoCpus === 'number' && resource.docker.nanoCpus > 0
        ? resource.docker.nanoCpus
        : null;
    case 'memory':
      return typeof resource.docker?.memoryBytes === 'number' && resource.docker.memoryBytes > 0
        ? resource.docker.memoryBytes
        : null;
    case 'address':
      return asTrimmedString(resource.docker?.address || resource.docker?.managerAddress) || null;
    default:
      key satisfies never;
      return null;
  }
};

export const DockerSwarmNodesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as DockerResourceStatusFilter,
    filter: filterDockerResources,
  });
  const sort = createPlatformTableSortState({
    storageKey: 'dockerSwarmNodes',
    sortKeys: DOCKER_SWARM_NODE_SORT_KEYS,
    descendingFirst: ['cpus', 'memory'],
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(
      [...tableState.filtered()].sort(compareDockerSwarmNodes),
      getDockerSwarmNodeSortValue,
    ),
  );
  const drawer = createPlatformResourceDetailState({ idPrefix: 'docker-swarm-node-detail' });

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
            searchPlaceholder="Search Swarm nodes"
            searchSuggestions={tableState.searchSuggestions}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={withPlatformStatusCounts(
              PLATFORM_HEALTH_FILTER_OPTIONS,
              tableState.countForStatus,
            )}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="nodes"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No Swarm nodes match current filters"
              description="Adjust the search or status filter to see more Docker Swarm nodes."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'Swarm Nodes'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1120px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="node"
                  class="platform-table-mobile-w-30 w-[32%] md:w-[20%]"
                >
                  Node
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="role"
                  class="platform-table-mobile-w-15 w-[15%] md:w-[10%]"
                >
                  Role
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="availability"
                  class="platform-table-phone-hidden md:w-[12%]"
                >
                  <PlatformResponsiveTableLabel compact="Avail" full="Availability" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="reachability"
                  class="platform-table-mobile-w-15 w-[15%] md:w-[14%]"
                >
                  Reachability
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="engine"
                  class="hidden md:table-cell md:w-[16%]"
                >
                  Engine
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="cpus"
                  class="platform-table-narrow-hidden w-[12%] sm:hidden md:table-cell md:w-[8%]"
                >
                  CPUs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="numeric-value"
                  sort={sort}
                  sortKey="memory"
                  class="platform-table-mobile-w-15 w-[15%] md:w-[10%]"
                >
                  <PlatformResponsiveTableLabel compact="Mem" full="Memory" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="address"
                  class="hidden sm:table-cell md:w-[10%]"
                >
                  Address
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(resource) => {
                    const managerReachability = () =>
                      dockerTextValue(
                        resource.docker?.leader ? 'leader' : resource.docker?.managerReachability,
                      );

                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          data-docker-swarm-node-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <DockerResourceNameCell
                            resource={resource}
                            indicator={mapDockerSwarmNodeStatus(resource)}
                            detailToggle={
                              <PlatformResourceDetailToggleButton
                                expanded={isExpanded()}
                                resourceLabel={dockerResourceName(resource)}
                                controlsId={detailRowId()}
                                onToggle={() => drawer.toggle(resource)}
                              />
                            }
                          />
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.nodeRole)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            {dockerTextValue(resource.docker?.availability)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {managerReachability()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {dockerTextValue(
                              resource.docker?.engineVersion || resource.docker?.runtimeVersion,
                            )}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} platform-table-narrow-hidden sm:hidden text-base-content md:table-cell`}
                          >
                            {dockerCpuValue(resource.docker?.nanoCpus)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {dockerByteValue(resource.docker?.memoryBytes)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content sm:table-cell`}
                          >
                            {dockerTextValue(
                              resource.docker?.address || resource.docker?.managerAddress,
                            )}
                          </TableCell>
                        </TableRow>
                        <Show when={isExpanded()}>
                          <InlineDetailTableRow
                            cellId={detailRowId()}
                            colspan={8}
                            data-inline-docker-swarm-node-detail-for={resource.id}
                          >
                            <DockerNativeDetailPanel
                              title={dockerResourceName(resource)}
                              fields={[
                                ['Role', dockerTextValue(resource.docker?.nodeRole)],
                                ['Availability', dockerTextValue(resource.docker?.availability)],
                                ['Reachability', managerReachability()],
                                [
                                  'Engine',
                                  dockerTextValue(
                                    resource.docker?.engineVersion ||
                                      resource.docker?.runtimeVersion,
                                  ),
                                ],
                                ['CPUs', dockerCpuValue(resource.docker?.nanoCpus)],
                                ['Memory', dockerByteValue(resource.docker?.memoryBytes)],
                                [
                                  'Address',
                                  dockerTextValue(
                                    resource.docker?.address || resource.docker?.managerAddress,
                                  ),
                                ],
                              ]}
                            />
                          </InlineDetailTableRow>
                        </Show>
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default DockerSwarmNodesTable;
