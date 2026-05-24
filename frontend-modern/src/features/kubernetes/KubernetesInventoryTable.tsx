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

type KubernetesInventoryVariant =
  | 'controllers'
  | 'services'
  | 'storage'
  | 'networking'
  | 'config'
  | 'policy'
  | 'autoscaling'
  | 'events';

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';
const numberValue = (value: number | undefined): JSX.Element => (
  <span class="tabular-nums">{value ?? 0}</span>
);
const byteValue = (value: number | undefined): string =>
  typeof value === 'number' && value > 0 ? formatBytes(value) : '—';
const joinValues = (values: readonly (string | undefined)[] | undefined): string =>
  (values ?? [])
    .map((value) => asTrimmedString(value))
    .filter((value): value is string => typeof value === 'string' && value.length > 0)
    .join(', ') || '—';

const k8sKind = (resource: Resource): string =>
  resource.kubernetes?.resourceKind || getResourceTypeLabel(resource.type) || resource.type;

const tableTitle = (variant: KubernetesInventoryVariant, explicit?: string): string => {
  if (explicit) return explicit;
  switch (variant) {
    case 'controllers':
      return 'Controllers';
    case 'services':
      return 'Services';
    case 'storage':
      return 'Storage';
    case 'networking':
      return 'Networking';
    case 'config':
      return 'Config';
    case 'policy':
      return 'Policy';
    case 'autoscaling':
      return 'Autoscaling';
    case 'events':
      return 'Events';
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
    <Show when={props.variant === 'storage'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[12%]`}>
        Phase / mode
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
      >
        Class
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[12%]`}>
        Size
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[21%]`}
      >
        Detail
      </TableHead>
    </Show>
    <Show when={props.variant === 'services' || props.variant === 'networking'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>Type</TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[20%]`}
      >
        Address
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[25%]`}
      >
        Ports / Hosts
      </TableHead>
    </Show>
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
    <Show when={props.variant === 'config'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>State</TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[31%]`}
      >
        Detail
      </TableHead>
    </Show>
    <Show when={props.variant === 'policy'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[15%]`}>Spec</TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[30%]`}
      >
        Detail
      </TableHead>
    </Show>
    <Show when={props.variant === 'autoscaling'}>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[22%]`}
      >
        Target
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Min
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Max
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Current
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}>
        Desired
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[19%]`}
      >
        Metrics
      </TableHead>
    </Show>
    <Show when={props.variant === 'events'}>
      <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[13%]`}>Reason</TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[18%]`}
      >
        Object
      </TableHead>
      <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[8%]`}>
        Count
      </TableHead>
      <TableHead
        class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[30%]`}
      >
        Message
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
  const address = () =>
    textValue(
      props.resource.kubernetes?.clusterIp ||
        props.resource.kubernetes?.addresses?.[0] ||
        props.resource.kubernetes?.externalIps?.[0] ||
        (typeof props.resource.kubernetes?.endpointCount === 'number'
          ? `${props.resource.kubernetes.endpointCount} endpoints`
          : undefined),
    );
  const ports = () =>
    joinValues(
      props.resource.kubernetes?.servicePorts?.length
        ? props.resource.kubernetes.servicePorts.map((port) => {
            const protocol = port.protocol ? `/${port.protocol.toLowerCase()}` : '';
            const target = port.targetPort ? `:${port.targetPort}` : '';
            return port.port ? `${port.port}${target}${protocol}` : undefined;
          })
        : props.resource.kubernetes?.endpointPorts?.length
          ? props.resource.kubernetes.endpointPorts.map((port) => {
              const protocol = port.protocol ? `/${port.protocol.toLowerCase()}` : '';
              return port.port ? `${port.port}${protocol}` : undefined;
            })
          : props.resource.kubernetes?.hosts?.length
            ? props.resource.kubernetes.hosts
            : props.resource.kubernetes?.policyTypes?.length
              ? props.resource.kubernetes.policyTypes
              : [
                  typeof props.resource.kubernetes?.ingressRuleCount === 'number'
                    ? `${props.resource.kubernetes.ingressRuleCount} ingress`
                    : undefined,
                  typeof props.resource.kubernetes?.egressRuleCount === 'number'
                    ? `${props.resource.kubernetes.egressRuleCount} egress`
                    : undefined,
                ],
    );
  const networkType = () =>
    textValue(
      props.resource.kubernetes?.serviceType ||
        props.resource.kubernetes?.className ||
        props.resource.kubernetes?.addressType ||
        props.resource.kubernetes?.policyTypes?.join(', '),
    );
  const storagePhase = () =>
    textValue(
      props.resource.kubernetes?.phase ||
        props.resource.kubernetes?.volumeBindingMode ||
        props.resource.kubernetes?.reclaimPolicy,
    );
  const storageClass = () =>
    textValue(props.resource.kubernetes?.storageClass || props.resource.name);
  const storageDetail = () =>
    textValue(
      props.resource.kubernetes?.volumeName ||
        [props.resource.kubernetes?.claimNamespace, props.resource.kubernetes?.claimName]
          .filter(Boolean)
          .join('/') ||
        props.resource.kubernetes?.provisioner ||
        props.resource.kubernetes?.parameterKeys?.join(', '),
    );
  const configState = () =>
    textValue(
      props.resource.kubernetes?.phase ||
        (props.resource.type === 'k8s-configmap'
          ? props.resource.kubernetes?.metadataOnly
            ? 'Metadata-only'
            : props.resource.kubernetes?.immutable
              ? 'Immutable'
              : 'Mutable'
          : undefined) ||
        (props.resource.type === 'k8s-secret'
          ? props.resource.kubernetes?.metadataOnly
            ? 'Metadata-only'
            : props.resource.kubernetes?.secretType ||
              (props.resource.kubernetes?.immutable ? 'Immutable' : 'Mutable')
          : undefined) ||
        (props.resource.type === 'k8s-serviceaccount'
          ? props.resource.kubernetes?.automountServiceAccountToken === false
            ? 'No auto token'
            : 'Auto token'
          : undefined),
    );
  const configDetail = () =>
    textValue(
      (props.resource.kubernetes?.metadataOnly ? 'Payload omitted' : undefined) ||
        props.resource.kubernetes?.dataKeys?.join(', ') ||
        props.resource.kubernetes?.binaryDataKeys?.join(', ') ||
        props.resource.kubernetes?.imagePullSecrets?.join(', ') ||
        (typeof props.resource.kubernetes?.secretCount === 'number'
          ? `${props.resource.kubernetes.secretCount} secrets`
          : undefined) ||
        props.resource.kubernetes?.clusterName,
    );
  const quotaDetail = () => {
    const hard = props.resource.kubernetes?.hard ?? {};
    const used = props.resource.kubernetes?.used ?? {};
    const keys = Object.keys(hard).sort();
    if (keys.length === 0) return undefined;
    return keys
      .slice(0, 4)
      .map((key) => `${key} ${used[key] ?? '0'}/${hard[key]}`)
      .join(', ');
  };
  const policySpec = () =>
    textValue(
      props.resource.kubernetes?.policyTypes?.join(', ') ||
        props.resource.kubernetes?.limitTypes?.join(', ') ||
        (props.resource.type === 'k8s-pod-disruption-budget'
          ? [
              props.resource.kubernetes?.minAvailable
                ? `min ${props.resource.kubernetes.minAvailable}`
                : undefined,
              props.resource.kubernetes?.maxUnavailable
                ? `max unavailable ${props.resource.kubernetes.maxUnavailable}`
                : undefined,
            ]
              .filter(Boolean)
              .join(', ')
          : undefined) ||
        (props.resource.type === 'k8s-resource-quota' ? 'Quota' : undefined),
    );
  const policyDetail = () =>
    textValue(
      quotaDetail() ||
        (props.resource.type === 'k8s-pod-disruption-budget'
          ? `${props.resource.kubernetes?.currentHealthy ?? 0}/${props.resource.kubernetes?.desiredHealthy ?? 0} healthy, ${props.resource.kubernetes?.disruptionsAllowed ?? 0} disruptions`
          : undefined) ||
        [
          typeof props.resource.kubernetes?.ingressRuleCount === 'number'
            ? `${props.resource.kubernetes.ingressRuleCount} ingress`
            : undefined,
          typeof props.resource.kubernetes?.egressRuleCount === 'number'
            ? `${props.resource.kubernetes.egressRuleCount} egress`
            : undefined,
        ]
          .filter(Boolean)
          .join(', ') ||
        props.resource.kubernetes?.limitTypes?.join(', '),
    );
  const autoscalingTarget = () =>
    textValue(
      [props.resource.kubernetes?.targetKind, props.resource.kubernetes?.targetName]
        .filter(Boolean)
        .join('/'),
    );
  const autoscalingMetrics = () => textValue(props.resource.kubernetes?.metricTypes?.join(', '));
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
      <Show when={props.variant === 'storage'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {storagePhase()}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {storageClass()}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {byteValue(
            props.resource.kubernetes?.capacityBytes || props.resource.kubernetes?.requestedBytes,
          )}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {storageDetail()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'services' || props.variant === 'networking'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {networkType()}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {address()}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {ports()}
        </TableCell>
      </Show>
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
      <Show when={props.variant === 'config'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {configState()}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {configDetail()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'policy'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {policySpec()}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {policyDetail()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'autoscaling'}>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {autoscalingTarget()}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.kubernetes?.minReplicas)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.kubernetes?.maxReplicas)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.kubernetes?.currentReplicas)}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.kubernetes?.desiredReplicas)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {autoscalingMetrics()}
        </TableCell>
      </Show>
      <Show when={props.variant === 'events'}>
        <TableCell class={`${getPlatformTableCellClassForKind('text')} text-base-content`}>
          {textValue(props.resource.kubernetes?.reason)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          {textValue(
            [props.resource.kubernetes?.involvedKind, props.resource.kubernetes?.involvedName]
              .filter(Boolean)
              .join('/'),
          )}
        </TableCell>
        <TableCell class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}>
          {numberValue(props.resource.kubernetes?.count)}
        </TableCell>
        <TableCell
          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
        >
          <span
            class="inline-block max-w-[28rem] truncate"
            title={textValue(props.resource.kubernetes?.message)}
          >
            {textValue(props.resource.kubernetes?.message)}
          </span>
        </TableCell>
      </Show>
    </TableRow>
  );
};

export default KubernetesInventoryTable;
