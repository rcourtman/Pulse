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
  kubernetesScopeLabel,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// ConfigMaps and Secrets are intentionally rendered as API metadata. Pulse may
// know that keys exist, but metadata-only collection must never imply that
// payload data was read or expose key names in this table.

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const configName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const configKind = (resource: Resource): string => {
  if (resource.type === 'k8s-namespace') return 'Namespace';
  if (resource.type === 'k8s-configmap') return 'ConfigMap';
  if (resource.type === 'k8s-secret') return 'Secret';
  if (resource.type === 'k8s-serviceaccount') return 'ServiceAccount';
  if (resource.type === 'k8s-role') return 'Role';
  if (resource.type === 'k8s-cluster-role') return 'ClusterRole';
  if (resource.type === 'k8s-role-binding') return 'RoleBinding';
  if (resource.type === 'k8s-cluster-role-binding') return 'ClusterRoleBinding';
  return resource.kubernetes?.resourceKind || resource.type;
};

const plural = (count: number, singular: string, pluralLabel = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : pluralLabel}`;

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

const lifecycleOrTrust = (resource: Resource): string => {
  if (resource.type === 'k8s-namespace') {
    return textValue(resource.kubernetes?.phase);
  }
  if (resource.type === 'k8s-configmap') {
    const parts = [
      resource.kubernetes?.metadataOnly ? 'Metadata-only' : undefined,
      resource.kubernetes?.immutable ? 'Immutable' : undefined,
      !resource.kubernetes?.metadataOnly && !resource.kubernetes?.immutable ? 'Mutable' : undefined,
    ].filter(Boolean);
    return parts.join(' · ') || '—';
  }
  if (resource.type === 'k8s-secret') {
    const parts = [
      resource.kubernetes?.metadataOnly ? 'Metadata-only' : undefined,
      asTrimmedString(resource.kubernetes?.secretType),
      resource.kubernetes?.immutable ? 'Immutable' : undefined,
      !resource.kubernetes?.metadataOnly &&
      !resource.kubernetes?.secretType &&
      !resource.kubernetes?.immutable
        ? 'Mutable'
        : undefined,
    ].filter(Boolean);
    return parts.join(' · ') || '—';
  }
  if (resource.type === 'k8s-serviceaccount') {
    if (resource.kubernetes?.automountServiceAccountToken === false) return 'No auto token';
    if (resource.kubernetes?.automountServiceAccountToken === true) return 'Auto token';
    return 'Default token';
  }
  if (resource.type === 'k8s-role' || resource.type === 'k8s-cluster-role') {
    const rules = resource.kubernetes?.ruleCount ?? 0;
    const aggregated =
      resource.type === 'k8s-cluster-role' &&
      resource.kubernetes?.aggregationLabels &&
      Object.keys(resource.kubernetes.aggregationLabels).length > 0;
    const parts = [plural(rules, 'rule'), aggregated ? 'Aggregated' : undefined].filter(Boolean);
    return parts.join(' · ');
  }
  if (resource.type === 'k8s-role-binding' || resource.type === 'k8s-cluster-role-binding') {
    const kind = asTrimmedString(resource.kubernetes?.roleKind);
    const name = asTrimmedString(resource.kubernetes?.roleName);
    if (kind && name) return `${kind}/${name}`;
    return kind || name || '—';
  }
  return textValue(resource.status);
};

const dataShape = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-role-binding' || resource.type === 'k8s-cluster-role-binding') {
    const subjectCount = resource.kubernetes?.subjectCount ?? 0;
    if (subjectCount === 0) {
      return { label: 'No subjects', title: '' };
    }
    const kinds = summarizeValues(resource.kubernetes?.subjectKinds);
    const label =
      kinds.label !== '—'
        ? `${plural(subjectCount, 'subject')} · ${kinds.label}`
        : plural(subjectCount, 'subject');
    return { label, title: kinds.title || label };
  }
  if (resource.type !== 'k8s-configmap' && resource.type !== 'k8s-secret') {
    return { label: '—', title: '' };
  }
  if (resource.kubernetes?.metadataOnly) {
    return {
      label: 'Payload omitted',
      title: 'Payload omitted by metadata-only collection',
    };
  }
  const dataKeyCount = resource.kubernetes?.dataKeys?.length ?? 0;
  const binaryKeyCount = resource.kubernetes?.binaryDataKeys?.length ?? 0;
  const parts = [
    dataKeyCount > 0 ? plural(dataKeyCount, 'data key', 'data keys') : undefined,
    binaryKeyCount > 0 ? plural(binaryKeyCount, 'binary key', 'binary keys') : undefined,
  ].filter(Boolean);
  return {
    label: parts.join(', ') || 'No keys reported',
    title: parts.join(', '),
  };
};

const serviceAccountRefs = (resource: Resource): { label: string; title: string } => {
  if (resource.type !== 'k8s-serviceaccount') return { label: '—', title: '' };
  const imagePullSecrets = summarizeValues(resource.kubernetes?.imagePullSecrets);
  const parts = [
    typeof resource.kubernetes?.secretCount === 'number'
      ? plural(resource.kubernetes.secretCount, 'secret')
      : undefined,
    imagePullSecrets.label !== '—' ? `pull: ${imagePullSecrets.label}` : undefined,
  ].filter(Boolean);
  return {
    label: parts.join(' · ') || 'No refs reported',
    title: [
      typeof resource.kubernetes?.secretCount === 'number' ? parts[0] : undefined,
      imagePullSecrets.title,
    ]
      .filter(Boolean)
      .join(' · '),
  };
};

const labelSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(resource.tags);

export const KubernetesConfigTable: Component<{
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-config-drawer' });
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
            searchPlaceholder="Search config inventory"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="config resources"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No config resources match current filters"
              description="Adjust the search or status filter to see more Kubernetes configuration resources."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader
              title={props.title ?? 'Namespaces, ConfigMaps, Secrets, ServiceAccounts, RBAC'}
            />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1160px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[18%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[13%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[16%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>
                    Lifecycle / trust
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
                    Data shape
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[13%]`}
                  >
                    Token refs
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[10%]`}
                  >
                    Labels
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => configName(resource);
                    const kind = () => configKind(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const state = () => lifecycleOrTrust(resource);
                    const data = () => dataShape(resource);
                    const refs = () => serviceAccountRefs(resource);
                    const labels = () => labelSummary(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                      <TableRow
                        class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                        aria-controls={isExpanded() ? detailRowId() : undefined}
                        aria-expanded={isExpanded() ? 'true' : 'false'}
                        data-kubernetes-config-row={resource.id}
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
                          <span class="inline-block max-w-[13rem] truncate" title={scope()}>
                            {scope()}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          {state()}
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <span class="inline-block max-w-[13rem] truncate" title={data().title}>
                            {data().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[13rem] truncate" title={refs().title}>
                            {refs().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[9rem] truncate" title={labels().title}>
                            {labels().label}
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

export default KubernetesConfigTable;
