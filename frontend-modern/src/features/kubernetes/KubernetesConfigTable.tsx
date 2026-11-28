import { Show, createMemo, createSignal, type Component, type JSX } from 'solid-js';
import {
  compactDetailRows,
  makeDetailRow,
  type DetailSection,
} from '@/components/shared/DetailSectionTable';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
import { TableCell, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PlatformWindowedRows,
  PlatformResponsiveTableLabel,
  PlatformSortableTableHead,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  createPlatformTableSortState,
  formatPlatformTableTextValue,
  getPlatformTableCellClassForKind,
  type PlatformTableFilterOption,
  summarizePlatformTableValues,
  PlatformTableShell,
  type PlatformTableSortValue,
  withPlatformStatusCounts,
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
  matchesKubernetesConfigCategory,
  type KubernetesConfigCategoryFilter,
  type KubernetesResourceStatusFilter,
  isSingleKubernetesCluster,
} from './kubernetesPageModel';

const ROUTINE_METADATA_ONLY_COLLAPSE_THRESHOLD = 6;

const isRoutineMetadataOnlyConfig = (resource: Resource): boolean =>
  (resource.type === 'k8s-configmap' || resource.type === 'k8s-secret') &&
  resource.kubernetes?.metadataOnly === true &&
  getSimpleStatusIndicator(resource.status).variant === 'success';

const KUBERNETES_CONFIG_TABLE_CATEGORY_OPTIONS: PlatformTableFilterOption<KubernetesConfigCategoryFilter>[] =
  [
    { value: 'all', label: 'All' },
    { value: 'namespace', label: 'Namespaces' },
    { value: 'config', label: 'Config & Secrets' },
    { value: 'access', label: 'Access' },
  ];
import {
  KubernetesDetailRow,
  KubernetesRecordDetailPanel,
  KubernetesRowLead,
  createKubernetesDetailState,
  kubernetesLabelsSection,
  kubernetesObjectSection,
} from './KubernetesRecordDetail';
import { KubernetesScopeText } from './KubernetesScopeText';

// ConfigMaps and Secrets are intentionally rendered as API metadata. Pulse may
// know that keys exist, but metadata-only collection must never imply that
// payload data was read or expose key names in this table.

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

const lifecycleOrTrust = (resource: Resource): string => {
  if (resource.type === 'k8s-namespace') {
    return formatPlatformTableTextValue(resource.kubernetes?.phase);
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
  return formatPlatformTableTextValue(resource.status);
};

const dataShape = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-role-binding' || resource.type === 'k8s-cluster-role-binding') {
    const subjectCount = resource.kubernetes?.subjectCount ?? 0;
    if (subjectCount === 0) {
      return { label: 'No subjects', title: '' };
    }
    const kinds = summarizePlatformTableValues(resource.kubernetes?.subjectKinds);
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
  const imagePullSecrets = summarizePlatformTableValues(resource.kubernetes?.imagePullSecrets);
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
  summarizePlatformTableValues(resource.tags);

const yesNo = (value: boolean | undefined): string | null =>
  typeof value === 'boolean' ? (value ? 'Yes' : 'No') : null;

// What the object holds, without its payload: key counts for Secrets (never
// key names or values), key names for a ConfigMap Pulse read in full, the
// rule and subject counts RBAC objects report.
const configContentSection = (resource: Resource): DetailSection | null => {
  const k = resource.kubernetes;
  const dataKeys = k?.dataKeys ?? [];
  const binaryKeys = k?.binaryDataKeys ?? [];
  switch (resource.type) {
    case 'k8s-configmap':
    case 'k8s-secret': {
      const showNames = resource.type === 'k8s-configmap' && !k?.metadataOnly;
      return {
        label: resource.type === 'k8s-secret' ? 'Secret' : 'ConfigMap',
        rows: compactDetailRows([
          resource.type === 'k8s-secret'
            ? makeDetailRow('Type', asTrimmedString(k?.secretType), { wrap: true })
            : null,
          makeDetailRow('Data', dataShape(resource).label, { wrap: true }),
          showNames && dataKeys.length > 0
            ? makeDetailRow('Data keys', dataKeys.join(', '), { wrap: true })
            : null,
          showNames && binaryKeys.length > 0
            ? makeDetailRow('Binary keys', binaryKeys.join(', '), { wrap: true })
            : null,
          makeDetailRow('Immutable', yesNo(k?.immutable)),
          k?.metadataOnly ? makeDetailRow('Collection', 'Metadata only') : null,
        ]),
      };
    }
    case 'k8s-serviceaccount':
      return {
        label: 'Service account',
        rows: compactDetailRows([
          makeDetailRow('Token', lifecycleOrTrust(resource)),
          typeof k?.secretCount === 'number'
            ? makeDetailRow('Secrets', plural(k.secretCount, 'secret'))
            : null,
          makeDetailRow('Image pull secrets', (k?.imagePullSecrets ?? []).join(', '), {
            wrap: true,
          }),
        ]),
      };
    case 'k8s-role':
    case 'k8s-cluster-role': {
      const aggregation = Object.entries(k?.aggregationLabels ?? {});
      return {
        label: 'Rules',
        rows: compactDetailRows([
          makeDetailRow('Rules', plural(k?.ruleCount ?? 0, 'rule')),
          makeDetailRow(
            'Aggregates',
            aggregation.map(([key, value]) => (value ? `${key}=${value}` : key)).join(', '),
            { wrap: true },
          ),
        ]),
      };
    }
    case 'k8s-role-binding':
    case 'k8s-cluster-role-binding':
      return {
        label: 'Binding',
        rows: compactDetailRows([
          makeDetailRow(
            'Role',
            lifecycleOrTrust(resource) === '—' ? null : lifecycleOrTrust(resource),
            {
              wrap: true,
            },
          ),
          makeDetailRow('Subjects', plural(k?.subjectCount ?? 0, 'subject')),
          makeDetailRow('Subject kinds', (k?.subjectKinds ?? []).join(', '), { wrap: true }),
        ]),
      };
    default:
      return null;
  }
};

const configSummary = (resource: Resource): string =>
  [lifecycleOrTrust(resource), dataShape(resource).label, serviceAccountRefs(resource).label]
    .filter((part) => part && part !== '—')
    .join(' · ');

const buildConfigDetailSections = (
  resource: Resource,
  now: number,
): Array<DetailSection | null> => [
  kubernetesObjectSection(resource, configKind(resource), now),
  configContentSection(resource),
  kubernetesLabelsSection(resource),
];

// Namespaces carry alerts of their own, so they keep the full object drawer;
// every other row here is a record about the cluster's configuration.
const hasObjectDrawer = (resource: Resource): boolean => resource.type === 'k8s-namespace';

// Data shape, Token refs, and Labels are composite summaries with no single
// scalar to order on, so they stay non-sortable. Lifecycle / trust is a
// single categorical string per row, which makes sorting group like rows.
const KUBERNETES_CONFIG_SORT_KEYS = ['resource', 'kind', 'scope', 'lifecycle'] as const;

type KubernetesConfigSortKey = (typeof KUBERNETES_CONFIG_SORT_KEYS)[number];

const getKubernetesConfigSortValue = (
  resource: Resource,
  key: KubernetesConfigSortKey,
): PlatformTableSortValue => {
  switch (key) {
    case 'resource':
      return configName(resource);
    case 'kind':
      return configKind(resource);
    case 'scope':
      return kubernetesScopeLabel(resource);
    case 'lifecycle': {
      const state = lifecycleOrTrust(resource);
      return state === '—' ? null : state;
    }
    default:
      key satisfies never;
      return null;
  }
};

export const KubernetesConfigTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
  externalSearch?: () => string;
  externalStatus?: () => KubernetesResourceStatusFilter;
  showRoutineMetadataOnly?: boolean;
}> = (props) => {
  const singleCluster = createMemo(() => isSingleKubernetesCluster(props.resources));
  const [categoryFilter, setCategoryFilter] = createSignal<KubernetesConfigCategoryFilter>('all');
  const [showAllMetadataOnly, setShowAllMetadataOnly] = createSignal(false);
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: (resources, search, status) =>
      filterKubernetesResources(resources, search, status).filter((resource) =>
        props.showToolbar === false
          ? true
          : matchesKubernetesConfigCategory(resource, categoryFilter()),
      ),
    externalSearch: props.externalSearch,
    externalStatus: props.externalStatus,
  });
  const countForCategory = (category: KubernetesConfigCategoryFilter): number =>
    filterKubernetesResources(props.resources, tableState.search(), 'all').filter((resource) =>
      matchesKubernetesConfigCategory(resource, category),
    ).length;
  const routineMetadataOnlySummary = createMemo(() => {
    const routine = tableState.filtered().filter(isRoutineMetadataOnlyConfig);
    let configMaps = 0;
    let secrets = 0;
    const namespaces = new Set<string>();
    for (const item of routine) {
      if (item.type === 'k8s-configmap') configMaps += 1;
      else if (item.type === 'k8s-secret') secrets += 1;
      const ns = asTrimmedString(item.kubernetes?.namespace);
      if (ns) namespaces.add(ns);
    }
    return {
      total: routine.length,
      configMaps,
      secrets,
      namespaceCount: namespaces.size,
    };
  });
  const shouldCollapseRoutineMetadataOnly = createMemo(() => {
    if (props.showRoutineMetadataOnly) return false;
    if (showAllMetadataOnly()) return false;
    if (tableState.search().trim().length > 0) return false;
    if (props.showToolbar !== false && categoryFilter() === 'config') return false;
    const summary = routineMetadataOnlySummary();
    if (summary.total < ROUTINE_METADATA_ONLY_COLLAPSE_THRESHOLD) return false;
    return tableState.filtered().length > summary.total;
  });
  const visibleRows = createMemo(() => {
    const filtered = tableState.filtered();
    if (!shouldCollapseRoutineMetadataOnly()) return filtered;
    return filtered.filter((resource) => !isRoutineMetadataOnlyConfig(resource));
  });
  const drawer = createKubernetesDetailState('kubernetes-config-drawer');
  const resolveResourceLabel = createPlatformResourceLabelResolver(() => props.resources);
  const sort = createPlatformTableSortState({
    storageKey: 'kubernetesConfig',
    sortKeys: KUBERNETES_CONFIG_SORT_KEYS,
  });
  const sortedRows = createMemo(() => sort.sortRows(visibleRows(), getKubernetesConfigSortValue));

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
            searchSuggestions={tableState.searchSuggestions}
            status={categoryFilter()}
            onStatusChange={setCategoryFilter}
            statusOptions={withPlatformStatusCounts(
              KUBERNETES_CONFIG_TABLE_CATEGORY_OPTIONS,
              countForCategory,
            )}
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
          <Show
            when={routineMetadataOnlySummary().total >= ROUTINE_METADATA_ONLY_COLLAPSE_THRESHOLD}
          >
            <div
              class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border bg-surface-alt/70 px-3 py-2 text-xs text-muted"
              data-k8s-metadata-only-summary
            >
              <span>
                {shouldCollapseRoutineMetadataOnly() ? 'Hiding' : 'Showing'}{' '}
                <strong class="font-medium text-base-content">
                  {routineMetadataOnlySummary().total} metadata-only
                </strong>{' '}
                objects ({routineMetadataOnlySummary().configMaps} ConfigMaps,{' '}
                {routineMetadataOnlySummary().secrets} Secrets
                <Show when={routineMetadataOnlySummary().namespaceCount > 0}>
                  {' '}
                  across {routineMetadataOnlySummary().namespaceCount}{' '}
                  {routineMetadataOnlySummary().namespaceCount === 1 ? 'namespace' : 'namespaces'}
                </Show>
                ) with payload collection omitted.
              </span>
              <Show
                when={!props.showRoutineMetadataOnly && tableState.search().trim().length === 0}
              >
                <button
                  type="button"
                  class="shrink-0 font-medium text-primary hover:underline"
                  onClick={() => setShowAllMetadataOnly((current) => !current)}
                >
                  {shouldCollapseRoutineMetadataOnly()
                    ? `Show all ${routineMetadataOnlySummary().total}`
                    : 'Hide routine metadata-only'}
                </button>
              </Show>
            </div>
          </Show>
          <PlatformTableShell
            title={props.title ?? 'Namespaces, ConfigMaps, Secrets, ServiceAccounts, RBAC'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1160px]"
            header={
              <>
                <PlatformSortableTableHead
                  kind="name"
                  sort={sort}
                  sortKey="resource"
                  class="platform-table-mobile-w-30 md:w-[27%]"
                  bandWidth={35}
                >
                  Resource
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="kind"
                  class="platform-table-mobile-w-15 md:w-[15%]"
                  bandWidth={16}
                >
                  Kind
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="scope"
                  class="platform-table-mobile-w-20 md:w-[10%]"
                  bandWidth={14}
                >
                  Scope
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  sortKey="lifecycle"
                  class="platform-table-phone-hidden md:w-[17%]"
                  bandWidth={16}
                >
                  <PlatformResponsiveTableLabel
                    compact="Status"
                    full="Lifecycle / trust"
                    compactInBand
                  />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="platform-table-mobile-w-15 md:w-[14%]"
                  bandWidth={19}
                >
                  <PlatformResponsiveTableLabel compact="Data" full="Data shape" />
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[13%]"
                >
                  Token refs
                </PlatformSortableTableHead>
                <PlatformSortableTableHead
                  kind="text"
                  sort={sort}
                  class="hidden md:table-cell md:w-[10%]"
                >
                  Labels
                </PlatformSortableTableHead>
              </>
            }
            body={
              <>
                <PlatformWindowedRows items={sortedRows} estimatedRowHeight={32}>
                  {(resource) => {
                    const indicator = () => mapKubernetesInventoryStatus(resource);
                    const name = () => configName(resource);
                    const kind = () => configKind(resource);
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
                          data-kubernetes-config-row={resource.id}
                          onClick={() => drawer.toggle(resource)}
                        >
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <KubernetesRowLead
                              resource={resource}
                              drawer={drawer}
                              label={name()}
                              status={indicator()}
                              staleScope="config"
                            />
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {kind()}
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
                            class={`${getPlatformTableCellClassForKind('text')} platform-table-phone-hidden text-base-content`}
                          >
                            {state()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="block truncate" title={data().title}>
                              {data().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="block truncate" title={refs().title}>
                              {refs().label}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="block truncate" title={labels().title}>
                              {labels().label}
                            </span>
                          </TableCell>
                        </TableRow>
                        <Show
                          when={hasObjectDrawer(resource)}
                          fallback={
                            <Show when={isExpanded()}>
                              <InlineDetailTableRow
                                cellId={detailRowId()}
                                colspan={7}
                                data-inline-kubernetes-config-detail-for={resource.id}
                              >
                                <KubernetesRecordDetailPanel
                                  testId="kubernetes-config-detail"
                                  resource={resource}
                                  title={name()}
                                  status={indicator()}
                                  summary={configSummary(resource)}
                                  sections={(now) => buildConfigDetailSections(resource, now)}
                                  onClose={() => drawer.close(resource)}
                                />
                              </InlineDetailTableRow>
                            </Show>
                          }
                        >
                          <KubernetesDetailRow
                            resource={resource}
                            statusIndicator={indicator()}
                            drawer={drawer}
                            colSpan={7}
                            resolveResourceLabel={resolveResourceLabel}
                          />
                        </Show>
                      </>
                    );
                  }}
                </PlatformWindowedRows>
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesConfigTable;
