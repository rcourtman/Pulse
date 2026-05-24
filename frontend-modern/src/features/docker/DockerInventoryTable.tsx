import { For, Show, type Component, type JSX } from 'solid-js';
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
import { formatBytes } from '@/utils/format';
import { getSimpleStatusIndicator } from '@/utils/status';
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
import type { Resource } from '@/types/resource';

type DockerInventoryVariant =
  | 'images'
  | 'volumes'
  | 'networks'
  | 'nodes'
  | 'tasks'
  | 'secrets'
  | 'configs';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';
const numberValue = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);
const byteValue = (value: number | undefined): string =>
  typeof value === 'number' && value > 0 ? formatBytes(value) : '—';
const cpuValue = (nanoCpus: number | undefined): string => {
  if (typeof nanoCpus !== 'number' || nanoCpus <= 0) return '—';
  const cpus = nanoCpus / 1_000_000_000;
  return cpus >= 10 ? `${Math.round(cpus)}` : cpus.toFixed(cpus % 1 === 0 ? 0 : 1);
};

const joinValues = (values: readonly (string | undefined)[] | undefined, empty = '—'): string => {
  const joined = (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .join(', ');
  return joined || empty;
};

const dockerTableTitle = (variant: DockerInventoryVariant, explicit?: string): string => {
  if (explicit) return explicit;
  switch (variant) {
    case 'images':
      return 'Images';
    case 'volumes':
      return 'Volumes';
    case 'networks':
      return 'Networks';
    case 'nodes':
      return 'Swarm Nodes';
    case 'tasks':
      return 'Swarm Tasks';
    case 'secrets':
      return 'Swarm Secrets';
    case 'configs':
      return 'Swarm Configs';
  }
};

const searchPlaceholder = (variant: DockerInventoryVariant): string => {
  switch (variant) {
    case 'images':
      return 'Search images';
    case 'volumes':
      return 'Search volumes';
    case 'networks':
      return 'Search networks';
    case 'nodes':
      return 'Search Swarm nodes';
    case 'tasks':
      return 'Search Swarm tasks';
    case 'secrets':
      return 'Search Swarm secrets';
    case 'configs':
      return 'Search Swarm configs';
  }
};

export const DockerInventoryTable: Component<{
  resources: Resource[];
  variant: DockerInventoryVariant;
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
            searchPlaceholder={searchPlaceholder(props.variant)}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun={props.variant}
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={`No ${props.variant} match current filters`}
              description="Adjust the search or status filter to see more rows."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={dockerTableTitle(props.variant, props.title)} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1080px]">
              <TableHeader>
                <DockerInventoryHeader variant={props.variant} />
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => <DockerInventoryRow resource={resource} variant={props.variant} />}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

const DockerInventoryHeader: Component<{ variant: DockerInventoryVariant }> = (props) => {
  if (props.variant === 'images') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[31%]`}>
          Image
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[29%]`}
        >
          Digests
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[12%]`}>
          Size
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
          In Use
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
        >
          Host
        </TableHead>
      </TableRow>
    );
  }
  if (props.variant === 'volumes') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[24%]`}>
          Volume
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
          Driver
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
        >
          Scope
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
          Size
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
        >
          Refs
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[36%]`}
        >
          Mountpoint
        </TableHead>
      </TableRow>
    );
  }
  if (props.variant === 'networks') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[25%]`}>
          Network
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
          Driver
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
        >
          Scope
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[31%]`}
        >
          Subnets
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[18%]`}>Host</TableHead>
      </TableRow>
    );
  }
  if (props.variant === 'nodes') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>Node</TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>Role</TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
          Availability
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
        >
          Reachability
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
        >
          Engine
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[8%]`}
        >
          CPUs
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[16%]`}>
          Memory
        </TableHead>
      </TableRow>
    );
  }
  if (props.variant === 'secrets') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[30%]`}>
          Secret
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>
          Driver
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
        >
          Template
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[20%]`}
        >
          Labels
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>Host</TableHead>
      </TableRow>
    );
  }
  if (props.variant === 'configs') {
    return (
      <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
        <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[34%]`}>
          Config
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[18%]`}>
          Template
        </TableHead>
        <TableHead
          class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[28%]`}
        >
          Labels
        </TableHead>
        <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[20%]`}>Host</TableHead>
      </TableRow>
    );
  }
  return (
    <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
      <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>Task</TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[22%]`}
      >
        Service
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
        Desired
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[18%]`}>
        Current
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[24%]`}
      >
        Node
      </TableHead>
    </TableRow>
  );
};

const DockerInventoryRow: Component<{ resource: Resource; variant: DockerInventoryVariant }> = (
  props,
) => {
  const indicator = () => getSimpleStatusIndicator(props.resource.status);
  const name = () => asTrimmedString(props.resource.name) || props.resource.id;
  const host = () => textValue(props.resource.docker?.hostname);
  const labelSummary = () => {
    const labels = props.resource.docker?.labels;
    if (!labels || Object.keys(labels).length === 0) return '—';
    return Object.entries(labels)
      .slice(0, 3)
      .map(([key, value]) => `${key}=${value}`)
      .join(', ');
  };

  return (
    <TableRow class="text-[11px] sm:text-xs">
      <TableCell class={getPlatformTableCellClassForKind('name')}>
        <div class="flex min-w-0 items-center gap-2">
          <StatusDot
            size="sm"
            variant={indicator().variant}
            title={props.resource.status || 'unknown'}
            ariaHidden
          />
          <span class="truncate font-semibold text-base-content" title={name()}>
            {name()}
          </span>
        </div>
      </TableCell>
      <Show when={props.variant === 'images'}>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          <span
            class="inline-block max-w-[24rem] truncate"
            title={joinValues(props.resource.docker?.repoDigests)}
          >
            {joinValues(props.resource.docker?.repoDigests)}
          </span>
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {byteValue(props.resource.docker?.sizeBytes)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.docker?.imageContainers)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {host()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'volumes'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.driver)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.scope)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {byteValue(props.resource.docker?.sizeBytes)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
        >
          {numberValue(props.resource.docker?.refCount)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          <span
            class="inline-block max-w-[28rem] truncate"
            title={textValue(props.resource.docker?.mountpoint)}
          >
            {textValue(props.resource.docker?.mountpoint)}
          </span>
        </TableCell>
      </Show>
      <Show when={props.variant === 'networks'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.driver)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.scope)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {joinValues(props.resource.docker?.subnets?.map((subnet) => subnet.subnet))}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {host()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'tasks'}>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.serviceName)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.desiredState)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.currentState)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.nodeName || props.resource.docker?.nodeId)}
        </TableCell>
      </Show>
      <Show when={props.variant === 'nodes'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.nodeRole)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.availability)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(
            props.resource.docker?.leader ? 'leader' : props.resource.docker?.managerReachability,
          )}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.engineVersion || props.resource.docker?.runtimeVersion)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content md:table-cell`}
        >
          {cpuValue(props.resource.docker?.nanoCpus)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {byteValue(props.resource.docker?.memoryBytes)}
        </TableCell>
      </Show>
      <Show when={props.variant === 'secrets'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.driver)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(props.resource.docker?.templatingDriver)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          <span class="inline-block max-w-[18rem] truncate" title={labelSummary()}>
            {labelSummary()}
          </span>
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {host()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'configs'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.docker?.templatingDriver)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          <span class="inline-block max-w-[24rem] truncate" title={labelSummary()}>
            {labelSummary()}
          </span>
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {host()}
        </TableCell>
      </Show>
    </TableRow>
  );
};

export default DockerInventoryTable;
