import { For, Show, createMemo, type Component } from 'solid-js';
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
import {
  DockerResourceNameCell,
  dockerByteValue,
  dockerCpuValue,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import { compareDockerSwarmNodes, mapDockerSwarmNodeStatus } from './dockerPageModel';

export const DockerSwarmNodesTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterPlatformResources,
  });
  const sortedRows = createMemo(() => [...tableState.filtered()].sort(compareDockerSwarmNodes));

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
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
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
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Swarm Nodes'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1120px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                    Node
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>
                    Role
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                    Availability
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Reachability
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[16%]`}
                  >
                    Engine
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
                  >
                    CPUs
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                  >
                    Memory
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Address
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={sortedRows()}>
                  {(resource) => {
                    const managerReachability = () =>
                      dockerTextValue(
                        resource.docker?.leader ? 'leader' : resource.docker?.managerReachability,
                      );

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-docker-swarm-node-row={resource.id}
                      >
                        <DockerResourceNameCell
                          resource={resource}
                          indicator={mapDockerSwarmNodeStatus(resource)}
                        />
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {dockerTextValue(resource.docker?.nodeRole)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {dockerTextValue(resource.docker?.availability)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
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
                          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
                        >
                          {dockerCpuValue(resource.docker?.nanoCpus)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {dockerByteValue(resource.docker?.memoryBytes)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {dockerTextValue(
                            resource.docker?.address || resource.docker?.managerAddress,
                          )}
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

export default DockerSwarmNodesTable;
