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
import { getResourceTypeLabel } from '@/utils/resourceTypePresentation';
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

type KubernetesInventoryVariant = 'controllers';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';
const numberValue = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);

const k8sKind = (resource: Resource): string =>
  resource.kubernetes?.resourceKind || getResourceTypeLabel(resource.type) || resource.type;

const tableTitle = (variant: KubernetesInventoryVariant, explicit?: string): string => {
  if (explicit) return explicit;
  switch (variant) {
    case 'controllers':
      return 'Controllers';
  }
};

export const KubernetesInventoryTable: Component<{
  resources: Resource[];
  variant: KubernetesInventoryVariant;
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
            searchPlaceholder={`Search ${tableTitle(props.variant).toLowerCase()}`}
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun={tableTitle(props.variant).toLowerCase()}
          />
        </Show>
        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title={`No ${tableTitle(props.variant).toLowerCase()} match current filters`}
              description="Adjust the search or status filter to see more rows."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={tableTitle(props.variant, props.title)} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1080px]">
              <TableHeader>
                <KubernetesInventoryHeader variant={props.variant} />
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => (
                    <KubernetesInventoryRow resource={resource} variant={props.variant} />
                  )}
                </For>
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

const KubernetesInventoryHeader: Component<{ variant: KubernetesInventoryVariant }> = (props) => (
  <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
    <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[25%]`}>Name</TableHead>
    <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>Kind</TableHead>
    <TableHead
      class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[16%]`}
    >
      Namespace
    </TableHead>
    <Show when={props.variant === 'controllers'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Desired
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Ready
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[25%]`}
      >
        Detail
      </TableHead>
    </Show>
  </TableRow>
);

const KubernetesInventoryRow: Component<{
  resource: Resource;
  variant: KubernetesInventoryVariant;
}> = (props) => {
  const indicator = () => getSimpleStatusIndicator(props.resource.status);
  const name = () => asTrimmedString(props.resource.name) || props.resource.id;
  const namespace = () => textValue(props.resource.kubernetes?.namespace);
  const kind = () => k8sKind(props.resource);
  const desired = () =>
    props.resource.kubernetes?.desiredReplicas ??
    props.resource.kubernetes?.desiredNumberScheduled ??
    props.resource.kubernetes?.active;
  const ready = () =>
    props.resource.kubernetes?.readyReplicas ??
    props.resource.kubernetes?.numberReady ??
    props.resource.kubernetes?.succeeded;
  const detail = () =>
    textValue(
      props.resource.kubernetes?.schedule ||
        props.resource.kubernetes?.serviceName ||
        props.resource.kubernetes?.reason,
    );

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
      <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
        {kind()}
      </TableCell>
      <TableCell
        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
      >
        {namespace()}
      </TableCell>
      <Show when={props.variant === 'controllers'}>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(desired())}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(ready())}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {detail()}
        </TableCell>
      </Show>
    </TableRow>
  );
};

export default KubernetesInventoryTable;
