import { For, Show, type Component } from 'solid-js';
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
import {
  DockerResourceNameCell,
  dockerHostName,
  dockerJoinValues,
  dockerNumberValue,
  dockerTextValue,
  type DockerNativeTableProps,
} from './DockerNativeTableShared';
import type { Resource } from '@/types/resource';

type DockerPort = NonNullable<NonNullable<Resource['docker']>['ports']>[number];
type DockerNetwork = NonNullable<NonNullable<Resource['docker']>['networks']>[number];
type DockerMount = NonNullable<NonNullable<Resource['docker']>['mounts']>[number];

const numberSearchValue = (value: number | undefined): string | undefined =>
  typeof value === 'number' ? String(value) : undefined;

const portLabel = (port: DockerPort): string => {
  const protocol = asTrimmedString(port.protocol)?.toLowerCase() || 'tcp';
  const privatePort = numberSearchValue(port.privatePort);
  const publicPort = numberSearchValue(port.publicPort);
  const ip = asTrimmedString(port.ip);

  if (privatePort && publicPort) {
    return `${ip ? `${ip}:` : ''}${publicPort}->${privatePort}/${protocol}`;
  }

  if (privatePort) return `${privatePort}/${protocol}`;
  if (publicPort) return `${ip ? `${ip}:` : ''}${publicPort}/${protocol}`;
  return protocol;
};

const portsSummary = (resource: Resource): string =>
  dockerJoinValues(resource.docker?.ports?.map((port) => portLabel(port)));

const networkLabel = (network: DockerNetwork): string => {
  const name = asTrimmedString(network.name);
  const address = asTrimmedString(network.ipv4) || asTrimmedString(network.ipv6);
  if (name && address) return `${name} ${address}`;
  return name || address || '';
};

const networksSummary = (resource: Resource): string =>
  dockerJoinValues(resource.docker?.networks?.map((network) => networkLabel(network)));

const mountLabel = (mount: DockerMount): string => {
  const type = asTrimmedString(mount.type);
  const destination = asTrimmedString(mount.destination);
  const source = asTrimmedString(mount.source);
  const mode = asTrimmedString(mount.mode) || (mount.rw === false ? 'ro' : '');
  const endpoint = destination || source;
  if (!endpoint) return type || '';
  const prefix = type ? `${type}:` : '';
  const suffix = mode ? ` (${mode})` : '';
  return `${prefix}${endpoint}${suffix}`;
};

const mountsSummary = (resource: Resource): string =>
  dockerJoinValues(resource.docker?.mounts?.map((mount) => mountLabel(mount)));

const runtimeSummary = (resource: Resource): string => {
  const runtime = asTrimmedString(resource.docker?.runtime);
  const version = asTrimmedString(
    resource.docker?.runtimeVersion || resource.docker?.dockerVersion,
  );
  if (runtime && version) return `${runtime} ${version}`;
  return runtime || version || '—';
};

const containerState = (resource: Resource): string =>
  dockerTextValue(resource.docker?.containerState || resource.status);

const updateStatusLabel = (resource: Resource): string => {
  const update = resource.docker?.updateStatus;
  if (!update) return '—';
  if (asTrimmedString(update.error)) return 'Error';
  return update.updateAvailable ? 'Available' : 'Current';
};

const updateStatusTitle = (resource: Resource): string => {
  const update = resource.docker?.updateStatus;
  if (!update) return 'No image update check reported';

  return dockerJoinValues(
    [
      updateStatusLabel(resource),
      update.error,
      update.currentDigest ? `current ${update.currentDigest}` : undefined,
      update.latestDigest ? `latest ${update.latestDigest}` : undefined,
      update.lastChecked ? `checked ${update.lastChecked}` : undefined,
    ],
    updateStatusLabel(resource),
  );
};

const dockerContainerSearchCandidates = (resource: Resource): Array<string | undefined> => [
  resource.name,
  resource.displayName,
  resource.id,
  resource.parentName,
  resource.platformId,
  resource.platformType,
  resource.agent?.hostname,
  resource.identity?.hostname,
  resource.canonicalIdentity?.displayName,
  resource.canonicalIdentity?.hostname,
  resource.canonicalIdentity?.primaryId,
  ...(resource.canonicalIdentity?.aliases ?? []),
  resource.docker?.containerId,
  resource.docker?.displayName,
  resource.docker?.hostname,
  resource.docker?.runtime,
  resource.docker?.runtimeVersion,
  resource.docker?.dockerVersion,
  resource.docker?.image,
  resource.docker?.imageId,
  resource.docker?.containerState,
  resource.docker?.health,
  numberSearchValue(resource.docker?.restartCount),
  numberSearchValue(resource.docker?.exitCode),
  updateStatusLabel(resource),
  resource.docker?.updateStatus?.error,
  resource.docker?.updateStatus?.currentDigest,
  resource.docker?.updateStatus?.latestDigest,
  ...(resource.docker?.ports?.flatMap((port) => [
    portLabel(port),
    port.ip,
    numberSearchValue(port.privatePort),
    numberSearchValue(port.publicPort),
    port.protocol,
  ]) ?? []),
  ...(resource.docker?.networks?.flatMap((network) => [
    networkLabel(network),
    network.name,
    network.ipv4,
    network.ipv6,
  ]) ?? []),
  ...(resource.docker?.mounts?.flatMap((mount) => [
    mountLabel(mount),
    mount.type,
    mount.source,
    mount.destination,
    mount.mode,
    mount.rw === false ? 'read-only' : mount.rw === true ? 'read-write' : undefined,
  ]) ?? []),
  ...(resource.tags ?? []),
];

const matchesDockerContainerSearch = (resource: Resource, search: string): boolean => {
  const needle = search.trim().toLowerCase();
  if (!needle) return true;

  return dockerContainerSearchCandidates(resource)
    .filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
    .some((value) => value.toLowerCase().includes(needle));
};

const filterDockerContainerResources = (
  resources: Resource[],
  search: string,
  status: PlatformResourceStatusFilter,
): Resource[] =>
  filterPlatformResources(resources, '', status).filter((resource) =>
    matchesDockerContainerSearch(resource, search),
  );

export const DockerContainersTable: Component<DockerNativeTableProps> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as PlatformResourceStatusFilter,
    filter: filterDockerContainerResources,
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
            searchPlaceholder="Search containers"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="containers"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No containers match current filters"
              description="Adjust the search or status filter to see more Docker containers."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Containers'} />
            <Table class="min-w-[790px] table-fixed text-xs md:min-w-[1680px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} w-[220px]`}>
                    Container
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[140px]`}
                  >
                    Host
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[130px]`}
                  >
                    Runtime
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} w-[260px]`}>
                    Image
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} w-[100px]`}>
                    State
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[100px]`}
                  >
                    Health
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} w-[110px]`}
                  >
                    Restarts
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[190px]`}
                  >
                    Ports
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[150px]`}
                  >
                    Networks
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[180px]`}
                  >
                    Mounts
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} w-[100px]`}>
                    Updates
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const image = () => dockerTextValue(resource.docker?.image);
                    const state = () => containerState(resource);
                    const health = () => dockerTextValue(resource.docker?.health);
                    const runtime = () => runtimeSummary(resource);
                    const host = () => dockerHostName(resource);
                    const ports = () => portsSummary(resource);
                    const networks = () => networksSummary(resource);
                    const mounts = () => mountsSummary(resource);
                    const updates = () => updateStatusLabel(resource);
                    const updateTitle = () => updateStatusTitle(resource);

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-docker-container-row={resource.id}
                      >
                        <DockerResourceNameCell resource={resource} />
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={host()}>
                            {host()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={runtime()}>
                            {runtime()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <span class="block max-w-full truncate" title={image()}>
                            {image()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {state()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          {health()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                        >
                          {dockerNumberValue(resource.docker?.restartCount)}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span
                            class="block max-w-full truncate font-mono text-[11px]"
                            title={ports()}
                          >
                            {ports()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={networks()}>
                            {networks()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="block max-w-full truncate" title={mounts()}>
                            {mounts()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <span class="block max-w-full truncate" title={updateTitle()}>
                            {updates()}
                          </span>
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

export default DockerContainersTable;
