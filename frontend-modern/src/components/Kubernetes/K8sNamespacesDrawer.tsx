import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import { SearchInput } from '@/components/shared/SearchInput';
import { TableRow, TableHead, TableCell } from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { StatusDot } from '@/components/shared/StatusDot';
import { buildKubernetesPath } from '@/routing/resourceLinks';
import {
  PlatformDetailTable,
  PlatformDetailTableBody,
  PlatformDetailTableHeader,
  PlatformResponsiveTableLabel,
  formatPlatformTableIntegerValue,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getK8sNamespacesDrawerPresentation,
  getK8sNamespacesEmptyState,
  getK8sNamespacesFailureState,
  getK8sNamespacesLoadingState,
} from '@/utils/k8sNamespacePresentation';
import { getNamespaceCountsIndicator, type NamespaceCounts } from '@/utils/k8sStatusPresentation';
import { asTrimmedString } from '@/utils/stringUtils';

type NamespaceRow = {
  namespace: string;
  pods: NamespaceCounts;
  deployments: NamespaceCounts;
};

type NamespacesResponse = {
  cluster: string;
  data: NamespaceRow[];
};

export const K8sNamespacesDrawer: Component<{
  cluster: string;
  onOpenDeployments?: (namespace: string | null) => void;
}> = (props) => {
  const navigate = useNavigate();
  const [search, setSearch] = createSignal('');
  const drawerPresentation = getK8sNamespacesDrawerPresentation();

  const clusterName = createMemo(() => asTrimmedString(props.cluster) ?? '');

  const [namespaces] = createResource(
    clusterName,
    async (cluster) => {
      if (!cluster) return { cluster: '', data: [] } as NamespacesResponse;
      const params = new URLSearchParams();
      params.set('cluster', cluster);
      return apiFetchJSON<NamespacesResponse>(
        `/api/resources/k8s/namespaces?${params.toString()}`,
        { cache: 'no-store' },
      );
    },
    { initialValue: { cluster: '', data: [] } as NamespacesResponse },
  );

  const loadError = createMemo(() => {
    const err = namespaces.error;
    if (!err) return '';
    return (err as Error)?.message || 'Failed to fetch namespaces';
  });

  const rows = createMemo(() => (Array.isArray(namespaces()?.data) ? namespaces()!.data : []));

  const filteredRows = createMemo(() => {
    const term = (asTrimmedString(search()) ?? '').toLowerCase();
    if (!term) return rows();
    return rows().filter((row) => row.namespace.toLowerCase().includes(term));
  });

  const openPods = (_namespace: string | null) => {
    if (!clusterName()) return;
    navigate(buildKubernetesPath('workloads'));
  };

  const headingId = () => `k8s-namespaces-drawer-heading-${clusterName() || 'cluster'}`;

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <h2 id={headingId()} class="sr-only">
        {clusterName() || 'Kubernetes cluster'} namespaces
      </h2>
      <Card padding="md">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <div class="text-sm font-semibold text-base-content">{drawerPresentation.title}</div>
            <div class="text-xs text-muted">{drawerPresentation.description}</div>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <div class="w-[12rem]">
              <SearchInput
                value={search}
                onChange={setSearch}
                placeholder={drawerPresentation.searchPlaceholder}
                inputClass="py-1 text-xs font-medium shadow-sm"
                typeToSearch
                clearOnEscape
              />
            </div>
            <button
              type="button"
              onClick={() => openPods(null)}
              class="rounded-md border border-border bg-surface px-3 py-1 text-xs font-semibold shadow-sm hover:bg-surface-hover"
            >
              {drawerPresentation.openAllPodsLabel}
            </button>
          </div>
        </div>
      </Card>

      <Show
        when={!namespaces.loading}
        fallback={
          <Card padding="lg">
            <EmptyState {...getK8sNamespacesLoadingState()} />
          </Card>
        }
      >
        <Show
          when={!loadError()}
          fallback={
            <div role="alert" aria-live="assertive">
              <Card padding="lg" tone="danger">
                <EmptyState {...getK8sNamespacesFailureState(loadError())} tone="danger" />
              </Card>
            </div>
          }
        >
          <Show
            when={filteredRows().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState {...getK8sNamespacesEmptyState(rows().length > 0)} />
              </Card>
            }
          >
            <Card padding="none" tone="card" class="overflow-hidden">
              <PlatformDetailTable class="min-w-0 table-fixed text-xs">
                <PlatformDetailTableHeader>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[22%]`}>
                    {drawerPresentation.namespaceColumnLabel}
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[18%]`}
                  >
                    {drawerPresentation.podsColumnLabel}
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('numeric-value')} platform-table-mobile-w-15 md:w-[22%]`}
                  >
                    {drawerPresentation.deploymentsColumnLabel}
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} platform-table-mobile-w-30 md:w-[38%]`}
                  >
                    {drawerPresentation.actionsColumnLabel}
                  </TableHead>
                </PlatformDetailTableHeader>
                <PlatformDetailTableBody>
                  <For each={filteredRows()}>
                    {(row) => {
                      const podIndicator = () => getNamespaceCountsIndicator(row.pods);
                      return (
                        <TableRow class="text-[11px] sm:text-xs">
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex items-center gap-2 min-w-0">
                              <StatusDot
                                size="sm"
                                variant={podIndicator().variant}
                                title={podIndicator().label}
                                ariaHidden
                              />
                              <span
                                class="font-semibold text-base-content truncate"
                                title={row.namespace}
                              >
                                {row.namespace}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <span class="font-semibold tabular-nums">
                              {formatPlatformTableIntegerValue(row.pods.total ?? 0, '0')}
                            </span>
                            <span class="ml-2 text-[11px] text-muted">
                              {row.pods.offline > 0
                                ? `${formatPlatformTableIntegerValue(row.pods.offline, '0')} off`
                                : ''}
                              {row.pods.warning > 0
                                ? `${row.pods.offline > 0 ? ' · ' : ''}${formatPlatformTableIntegerValue(row.pods.warning, '0')} warn`
                                : ''}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            <span class="font-semibold tabular-nums">
                              {formatPlatformTableIntegerValue(row.deployments.total ?? 0, '0')}
                            </span>
                            <span class="ml-2 text-[11px] text-muted">
                              {row.deployments.warning > 0
                                ? `${formatPlatformTableIntegerValue(row.deployments.warning, '0')} warn`
                                : ''}
                              {row.deployments.offline > 0
                                ? `${row.deployments.warning > 0 ? ' · ' : ''}${formatPlatformTableIntegerValue(row.deployments.offline, '0')} off`
                                : ''}
                            </span>
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <div class="flex flex-wrap items-center gap-2">
                              <button
                                type="button"
                                aria-label={drawerPresentation.openPodsLabel}
                                onClick={() => openPods(row.namespace)}
                                class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-surface-hover"
                              >
                                <PlatformResponsiveTableLabel
                                  compact="Pods"
                                  full={drawerPresentation.openPodsLabel}
                                />
                              </button>
                              <Show when={props.onOpenDeployments}>
                                <button
                                  type="button"
                                  aria-label={drawerPresentation.viewDeploymentsLabel}
                                  onClick={() => props.onOpenDeployments?.(row.namespace)}
                                  class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-surface-hover"
                                >
                                  <PlatformResponsiveTableLabel
                                    compact="Deployments"
                                    full={drawerPresentation.viewDeploymentsLabel}
                                  />
                                </button>
                              </Show>
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    }}
                  </For>
                </PlatformDetailTableBody>
              </PlatformDetailTable>
            </Card>
          </Show>
        </Show>
      </Show>
    </section>
  );
};

export default K8sNamespacesDrawer;
