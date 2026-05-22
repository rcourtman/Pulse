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
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_CARD_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  platformChipStatusDot,
  type PlatformTableFilterOption,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  filterVmwareNetworks,
  type VmwareNetworkStatusFilter,
} from './vmwarePageModel';

const VSPHERE_NETWORK_STATUS_OPTIONS: PlatformTableFilterOption<VmwareNetworkStatusFilter>[] = [
  { value: 'all', label: 'All' },
  {
    value: 'healthy',
    label: 'Healthy',
    tone: 'success',
    leading: platformChipStatusDot('bg-emerald-500'),
  },
  {
    value: 'attention',
    label: 'Attention',
    tone: 'warning',
    leading: platformChipStatusDot('bg-amber-500'),
  },
  { value: 'unknown', label: 'Unknown' },
];

const networkName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const networkType = (resource: Resource): string =>
  asTrimmedString(resource.vmware?.networkType) || '—';

const compactList = (values: Array<string | undefined>): string[] =>
  values.map((value) => asTrimmedString(value)).filter((value): value is string => Boolean(value));

const summarizeValues = (
  values: string[],
  empty = '—',
  visibleCount = 2,
): { label: string; title: string } => {
  if (values.length === 0) return { label: empty, title: '' };
  const visible = values.slice(0, visibleCount);
  const suffix = values.length > visible.length ? ` +${values.length - visible.length}` : '';
  return { label: `${visible.join(', ')}${suffix}`, title: values.join(', ') };
};

const hostSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(compactList(resource.vmware?.networkHostNames ?? []), '—', 2);

const vmSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(compactList(resource.vmware?.networkVmNames ?? []), '—', 2);

const vmCount = (resource: Resource): number =>
  resource.vmware?.networkVmNames?.length ?? resource.vmware?.networkVmIds?.length ?? 0;

export const VsphereNetworksTable: Component<{
  networks: Resource[];
  scope: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.networks,
    initialStatus: 'all' as VmwareNetworkStatusFilter,
    filter: filterVmwareNetworks,
  });
  const drawer = createPlatformResourceDetailState({ idPrefix: 'vsphere-network-drawer' });
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.scope);

  return (
    <Show
      when={props.networks.length > 0}
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
            searchPlaceholder="Search vSphere networks"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={VSPHERE_NETWORK_STATUS_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="networks"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No networks match current filters"
              description="Adjust the search or status filter to see more vSphere networks."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title="Networks" />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1040px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[24%]`}>
                    Network
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[13%]`}>
                    Type
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
                  >
                    Hosts
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} hidden md:table-cell md:w-[7%]`}
                  >
                    VMs
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden lg:table-cell md:w-[18%]`}
                  >
                    Connected VMs
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
                  >
                    Datacenter
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(network) => {
                    const hosts = createMemo(() => hostSummary(network));
                    const vms = createMemo(() => vmSummary(network));
                    const indicator = () => getSimpleStatusIndicator(network.status);
                    const name = () => networkName(network);
                    const datacenter = () => asTrimmedString(network.vmware?.datacenterName) || '—';
                    const networkSubtitle = () =>
                      asTrimmedString(network.vmware?.managedObjectId) ||
                      asTrimmedString(network.vmware?.folderName) ||
                      asTrimmedString(network.vmware?.vcenterHost) ||
                      '';
                    const networkTitle = () =>
                      [name(), networkSubtitle()].filter(Boolean).join(' · ') || name();
                    const detailRowId = () => drawer.detailRowId(network);
                    const isExpanded = () => drawer.isExpanded(network);
                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-vsphere-network-row={network.id}
                          onClick={() => drawer.toggle(network)}
                          onKeyDown={drawer.handleActivationKey(network)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
                              <StatusDot
                                size="sm"
                                variant={indicator().variant}
                                title={indicator().label}
                              />
                              <span
                                class="truncate font-medium text-base-content"
                                title={networkTitle()}
                              >
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <span class="font-mono text-[11px] text-base-content">
                              {networkType(network)}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={hosts().title}
                          >
                            <span class="block truncate">{hosts().label}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} hidden text-base-content tabular-nums md:table-cell`}
                          >
                            {vmCount(network)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content lg:table-cell`}
                            title={vms().title}
                          >
                            <span class="block truncate">{vms().label}</span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                            title={datacenter()}
                          >
                            <span class="block truncate">{datacenter()}</span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={network}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={6}
                          resolveResourceLabel={resolveResourceLabel}
                          onClose={() => drawer.close(network)}
                        />
                      </>
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

export default VsphereNetworksTable;
