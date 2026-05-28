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
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import {
  PlatformResourceDetailTableRow,
  createPlatformResourceDetailState,
  createPlatformResourceLabelResolver,
  getPlatformResourceDetailRowClass,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import {
  filterKubernetesResources,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const resourceName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const networkKind = (resource: Resource): string => {
  if (resource.type === 'k8s-ingress') return 'Ingress';
  if (resource.type === 'k8s-endpoint-slice') return 'EndpointSlice';
  return resource.kubernetes?.resourceKind || resource.type;
};

const scopeLabel = (resource: Resource): string => {
  const cluster =
    asTrimmedString(resource.kubernetes?.clusterId) ||
    asTrimmedString(resource.kubernetes?.clusterName);
  const namespace = asTrimmedString(resource.kubernetes?.namespace);
  if (namespace) return cluster ? `${cluster}/${namespace}` : namespace;
  return cluster || 'Cluster';
};

const summarizeValues = (
  values: readonly (string | undefined)[] | undefined,
  visible = 2,
): { label: string; title: string } => {
  const normalized = (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => typeof value === 'string' && value.length > 0);
  if (normalized.length === 0) return { label: '—', title: '' };
  const shown = normalized.slice(0, visible);
  const suffix = normalized.length > shown.length ? ` +${normalized.length - shown.length}` : '';
  return { label: `${shown.join(', ')}${suffix}`, title: normalized.join(', ') };
};

const portLabel = (resource: Resource): { label: string; title: string } => {
  if (resource.kubernetes?.endpointPorts?.length) {
    return summarizeValues(
      resource.kubernetes.endpointPorts.map((port) => {
        if (!port.port) return undefined;
        const protocol = port.protocol ? `/${port.protocol.toLowerCase()}` : '';
        const appProtocol = port.appProtocol ? ` ${port.appProtocol}` : '';
        return `${port.port}${protocol}${appProtocol}`;
      }),
    );
  }
  return { label: '—', title: '' };
};

const typeOrClass = (resource: Resource): string =>
  textValue(resource.kubernetes?.className || resource.kubernetes?.addressType);

const addressOrHosts = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-ingress') {
    return summarizeValues([
      ...(resource.kubernetes?.hosts ?? []),
      ...(resource.kubernetes?.addresses ?? []),
    ]);
  }
  const ready = resource.kubernetes?.readyEndpointCount;
  const total = resource.kubernetes?.endpointCount;
  if (typeof ready === 'number' || typeof total === 'number') {
    const readyValue = ready ?? 0;
    const totalValue = total ?? readyValue;
    return {
      label: `${readyValue}/${totalValue} ready`,
      title: `${readyValue}/${totalValue} ready`,
    };
  }
  return summarizeValues(resource.kubernetes?.addresses);
};

const targetSummary = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-ingress') {
    const rules = resource.kubernetes?.ingressRuleCount;
    const hosts = summarizeValues(resource.kubernetes?.hosts);
    const hostCount = resource.kubernetes?.hosts?.length ?? 0;
    const label =
      typeof rules === 'number'
        ? `${rules} rule${rules === 1 ? '' : 's'}`
        : hostCount > 0
          ? `${hostCount} host${hostCount === 1 ? '' : 's'}`
          : hosts.label;
    return { label, title: hosts.title || label };
  }
  const service = textValue(resource.kubernetes?.serviceName);
  const ready = resource.kubernetes?.readyEndpointCount;
  const total = resource.kubernetes?.endpointCount;
  const endpointLabel =
    typeof ready === 'number' || typeof total === 'number'
      ? `${ready ?? 0}/${total ?? ready ?? 0} ready`
      : '';
  return {
    label: [service, endpointLabel].filter((value) => value && value !== '—').join(' · ') || '—',
    title: [service, endpointLabel].filter((value) => value && value !== '—').join(' · '),
  };
};

export const KubernetesNetworkingTable: Component<{
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-networking-drawer' });
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
            searchPlaceholder="Search networking inventory"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="network resources"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No network resources match current filters"
              description="Adjust the search or status filter to see more Kubernetes network resources."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader title={props.title ?? 'Ingresses and EndpointSlices'} />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1180px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[19%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                    Type / class
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
                  >
                    Address / hosts
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
                    Ports
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Targets
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => resourceName(resource);
                    const scope = () => scopeLabel(resource);
                    const address = () => addressOrHosts(resource);
                    const ports = () => portLabel(resource);
                    const targets = () => targetSummary(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-networking-row={resource.id}
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
                            {networkKind(resource)}
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
                            {typeOrClass(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[16rem] truncate"
                              title={address().title}
                            >
                              {address().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={ports().title}>
                              {ports().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span
                              class="inline-block max-w-[13rem] truncate"
                              title={targets().title}
                            >
                              {targets().label}
                            </span>
                          </TableCell>
                        </TableRow>
                        <PlatformResourceDetailTableRow
                          resource={resource}
                          open={isExpanded()}
                          detailRowId={detailRowId()}
                          colSpan={7}
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

export default KubernetesNetworkingTable;
