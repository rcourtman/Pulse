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
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';

// Kubernetes storage is intentionally not rendered by the generic
// inventory table. StorageClass, PersistentVolume, and
// PersistentVolumeClaim expose different API fields, and operators need
// to see binding mode, reclaim policy, access modes, requested capacity,
// and claim/volume bindings without mentally decoding a catch-all
// "detail" column.

const ACCESS_MODE_LABELS: Record<string, string> = {
  ReadOnlyMany: 'ROX',
  ReadWriteMany: 'RWX',
  ReadWriteOnce: 'RWO',
  ReadWriteOncePod: 'RWOP',
};

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const storageName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const storageKind = (resource: Resource): string => {
  if (resource.type === 'k8s-storage-class') return 'StorageClass';
  if (resource.type === 'k8s-persistent-volume') return 'PV';
  if (resource.type === 'k8s-persistent-volume-claim') return 'PVC';
  return resource.kubernetes?.resourceKind || resource.type;
};

const storageScope = (resource: Resource): string => {
  const clusterScope =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  if (namespace) {
    return clusterScope ? `${clusterScope}/${namespace}` : namespace;
  }
  return clusterScope || 'Cluster';
};

const bindingOrPhase = (resource: Resource): string => {
  if (resource.type === 'k8s-storage-class') {
    return textValue(resource.kubernetes?.volumeBindingMode || 'Immediate');
  }
  return textValue(resource.kubernetes?.phase);
};

const storageClass = (resource: Resource): string => {
  if (resource.type === 'k8s-storage-class') return storageName(resource);
  return textValue(resource.kubernetes?.storageClass);
};

const capacityLabel = (resource: Resource): string => {
  const capacity = resource.kubernetes?.capacityBytes;
  const requested = resource.kubernetes?.requestedBytes;
  if (resource.type === 'k8s-persistent-volume-claim') {
    if (
      typeof capacity === 'number' &&
      capacity > 0 &&
      typeof requested === 'number' &&
      requested > 0
    ) {
      if (capacity !== requested) return `${formatBytes(capacity)} / ${formatBytes(requested)} req`;
      return formatBytes(capacity);
    }
    if (typeof requested === 'number' && requested > 0) return `${formatBytes(requested)} req`;
  }
  if (typeof capacity === 'number' && capacity > 0) return formatBytes(capacity);
  return '—';
};

const summarizeAccessModes = (
  accessModes: string[] | undefined,
): { label: string; title: string } => {
  const modes = (accessModes ?? [])
    .map((mode) => asTrimmedString(mode))
    .filter((mode): mode is string => typeof mode === 'string' && mode.length > 0);
  if (modes.length === 0) return { label: '—', title: '' };
  return {
    label: modes.map((mode) => ACCESS_MODE_LABELS[mode] || mode).join(', '),
    title: modes.join(', '),
  };
};

const policyLabel = (resource: Resource): { label: string; title: string } => {
  const accessModes = summarizeAccessModes(resource.kubernetes?.accessModes);
  const parts = accessModes.label === '—' ? [] : [accessModes.label];
  const titleParts = accessModes.title ? [accessModes.title] : [];
  const reclaimPolicy = asTrimmedString(resource.kubernetes?.reclaimPolicy);
  if (reclaimPolicy) {
    parts.push(reclaimPolicy);
    titleParts.push(`Reclaim: ${reclaimPolicy}`);
  }
  if (
    resource.type === 'k8s-storage-class' &&
    resource.kubernetes?.allowVolumeExpansion !== undefined
  ) {
    const expansion = resource.kubernetes.allowVolumeExpansion ? 'Expandable' : 'Fixed';
    parts.push(expansion);
    titleParts.push(`Volume expansion: ${expansion}`);
  }
  return { label: parts.join(' · ') || '—', title: titleParts.join(' · ') };
};

const bindingTarget = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-storage-class') {
    const provisioner = textValue(resource.kubernetes?.provisioner);
    const parameterKeys = resource.kubernetes?.parameterKeys ?? [];
    return {
      label: provisioner,
      title:
        parameterKeys.length > 0
          ? `${provisioner} · params: ${parameterKeys.join(', ')}`
          : provisioner,
    };
  }
  if (resource.type === 'k8s-persistent-volume') {
    const claim = [resource.kubernetes?.claimNamespace, resource.kubernetes?.claimName]
      .map((value) => asTrimmedString(value))
      .filter(Boolean)
      .join('/');
    return { label: claim || 'Unclaimed', title: claim || 'Unclaimed' };
  }
  const volumeName = asTrimmedString(resource.kubernetes?.volumeName);
  return { label: volumeName || 'Unbound', title: volumeName || 'Unbound' };
};

export const KubernetesStorageTable: Component<{
  resources: Resource[];
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-storage-drawer' });
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
            searchPlaceholder="Search storage inventory"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="storage resources"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No volume resources match current filters"
              description="Adjust the search or status filter to see more Kubernetes volume resources."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Storage Classes, Volumes, and Claims'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1120px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[19%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
                    Binding / phase
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Class
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}
                  >
                    Size
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[12%]`}
                  >
                    Access / policy
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Provider / binding
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => storageName(resource);
                    const kind = () => storageKind(resource);
                    const scope = () => storageScope(resource);
                    const policy = () => policyLabel(resource);
                    const target = () => bindingTarget(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-storage-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                          onKeyDown={drawer.handleActivationKey(resource)}
                          tabIndex={0}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex min-w-0 items-center gap-2">
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
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {kind()}
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
                            {bindingOrPhase(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            {storageClass(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {capacityLabel(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[12rem] truncate"
                              title={policy().title}
                            >
                              {policy().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[18rem] truncate"
                              title={target().title}
                            >
                              {target().label}
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
              </TableBody>
            </Table>
          </TableCard>
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesStorageTable;
