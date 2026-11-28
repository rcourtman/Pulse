import { createMemo, type Component, type JSX } from 'solid-js';
import { TableCell, TableRow } from '@/components/shared/Table';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableCountRatioValue,
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
  mapKubernetesInventoryStatus,
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

const resourceName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const networkKind = (resource: Resource): string => {
  if (resource.type === 'k8s-ingress') return 'Ingress';
  if (resource.type === 'k8s-endpoint-slice') return 'EndpointSlice';
  return resource.kubernetes?.resourceKind || resource.type;
};

const portLabel = (resource: Resource): { label: string; title: string } => {
  if (resource.kubernetes?.endpointPorts?.length) {
    return summarizePlatformTableValues(
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
  formatPlatformTableTextValue(resource.kubernetes?.className || resource.kubernetes?.addressType);

const addressOrHosts = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-ingress') {
    return summarizePlatformTableValues([
      ...(resource.kubernetes?.hosts ?? []),
      ...(resource.kubernetes?.addresses ?? []),
    ]);
  }
  const ready = resource.kubernetes?.readyEndpointCount;
  const total = resource.kubernetes?.endpointCount;
  if (typeof ready === 'number' || typeof total === 'number') {
    const label = formatPlatformTableCountRatioValue(ready, total, { suffix: 'ready' });
    return {
      label,
      title: label,
    };
  }
  return summarizePlatformTableValues(resource.kubernetes?.addresses);
};

const targetSummary = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-ingress') {
    const rules = resource.kubernetes?.ingressRuleCount;
    const hosts = summarizePlatformTableValues(resource.kubernetes?.hosts);
    const hostCount = resource.kubernetes?.hosts?.length ?? 0;
    const label =
      typeof rules === 'number'
        ? `${rules} rule${rules === 1 ? '' : 's'}`
        : hostCount > 0
          ? `${hostCount} host${hostCount === 1 ? '' : 's'}`
          : hosts.label;
    return { label, title: hosts.title || label };
  }
  const service = formatPlatformTableTextValue(resource.kubernetes?.serviceName);
  const ready = resource.kubernetes?.readyEndpointCount;
  const total = resource.kubernetes?.endpointCount;
  const endpointLabel =
    typeof ready === 'number' || typeof total === 'number'
      ? formatPlatformTableCountRatioValue(ready, total, { suffix: 'ready' })
      : '';
  return {
    label: [service, endpointLabel].filter((value) => value && value !== '—').join(' · ') || '—',
    title: [service, endpointLabel].filter((value) => value && value !== '—').join(' · '),
  };
};

// Address / hosts, Ports, and Targets are composite summaries with no single
// scalar to order on, so they stay non-sortable.
const KUBERNETES_NETWORKING_SORT_KEYS = ['resource', 'kind', 'scope', 'typeClass'] as const;

type KubernetesNetworkingSortKey = (typeof KUBERNETES_NETWORKING_SORT_KEYS)[number];

const getKubernetesNetworkingSortValue = (
  resource: Resource,
  key: KubernetesNetworkingSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'resource':
      return resourceName(resource);
    case 'kind':
      return networkKind(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'typeClass': {
      const typeClass = typeOrClass(resource);
      return typeClass === '—' ? null : typeClass;
    }
    default:
      key satisfies never;
      return null;
  }
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
  const singleCluster = createMemo(() => isSingleKubernetesCluster(props.resources));
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const drawer = createKubernetesDetailState('kubernetes-networking-drawer');
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesNetworking',
    sortKeys: KUBERNETES_NETWORKING_SORT_KEYS,
  });
  const sortedRows = createMemo(() =>
    sort.sortRows(tableState.filtered(), getKubernetesNetworkingSortValue),
  );

  return (
    <KubernetesTableFrame
      hasResources={props.resources.length > 0}
      emptyIcon={props.emptyIcon}
      emptyTitle={props.emptyTitle}
      emptyDescription={props.emptyDescription}
      showToolbar={props.showToolbar}
      rowNoun="network resources"
      searchPlaceholder="Search networking inventory"
      filteredEmptyNoun="Kubernetes network resources"
      tableState={tableState}
    >
      <PlatformTableShell
        title={props.title ?? 'Ingresses and EndpointSlices'}
        tableClass="min-w-full table-fixed text-xs md:min-w-[1180px]"
        header={
          <>
            <PlatformSortableTableHead
              kind="name"
              sort={sort}
              sortKey="resource"
              class="kubernetes-network-name-column platform-table-mobile-w-30 md:w-[19%]"
            >
              Resource
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="kind"
              class="kubernetes-network-kind-column platform-table-mobile-w-15 md:w-[12%]"
            >
              Kind
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="scope"
              class="platform-table-mobile-w-20 md:w-[14%]"
            >
              Scope
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              sortKey="typeClass"
              class="kubernetes-network-class-column platform-table-mobile-w-15 md:w-[12%]"
            >
              <PlatformResponsiveTableLabel compact="Cls" full="Type / class" />
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="hidden md:table-cell md:w-[18%]"
            >
              Address / hosts
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="kubernetes-network-ports-column platform-table-mobile-w-20 md:w-[12%]"
            >
              Ports
            </PlatformSortableTableHead>
            <PlatformSortableTableHead
              kind="text"
              sort={sort}
              class="hidden md:table-cell md:w-[13%]"
            >
              Targets
            </PlatformSortableTableHead>
          </>
        }
        body={
          <>
            <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
              {(resource) => {
                const indicator = () => mapKubernetesInventoryStatus(resource);
                const name = () => resourceName(resource);
                const address = () => addressOrHosts(resource);
                const ports = () => portLabel(resource);
                const targets = () => targetSummary(resource);
                const isExpanded = () => drawer.isExpanded(resource);

                return (
                  <>
                    <TableRow
                      class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                      data-kubernetes-networking-row={resource.id}
                      onClick={() => drawer.toggle(resource)}
                    >
                      <TableCell class={getPlatformTableCellClassForKind('name')}>
                        <KubernetesRowLead
                          resource={resource}
                          drawer={drawer}
                          label={name()}
                          status={indicator()}
                          staleScope="networking"
                        >
                          <span
                            class="kubernetes-network-name-value block truncate font-semibold text-base-content"
                            title={name()}
                          >
                            {name()}
                          </span>
                        </KubernetesRowLead>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        {networkKind(resource)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
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
                        {typeOrClass(resource)}
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        <span class="block truncate" title={address().title}>
                          {address().label}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                      >
                        <span class="block truncate" title={ports().title}>
                          {ports().label}
                        </span>
                      </TableCell>
                      <TableCell
                        class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                      >
                        <span class="block truncate" title={targets().title}>
                          {targets().label}
                        </span>
                      </TableCell>
                    </TableRow>
                    <KubernetesDetailRow
                      resource={resource}
                      statusIndicator={indicator()}
                      drawer={drawer}
                      colSpan={7}
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

export default KubernetesNetworkingTable;
