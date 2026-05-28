import type { Component } from 'solid-js';
import { For, Show, createMemo, createResource, createSignal, createEffect } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { apiFetchJSON } from '@/utils/apiClient';
import { Card } from '@/components/shared/Card';
import {
  filterGroupClass,
  filterLabelClass,
  filterSelectClass,
} from '@/components/shared/FilterToolbar';
import { SearchInput } from '@/components/shared/SearchInput';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from '@/components/shared/Table';
import { EmptyState } from '@/components/shared/EmptyState';
import { buildKubernetesPath } from '@/routing/resourceLinks';
import {
  PLATFORM_TABLE_BODY_CLASS,
  PLATFORM_TABLE_HEADER_ROW_CLASS,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getK8sDeploymentsDrawerPresentation,
  getK8sDeploymentsEmptyState,
  getK8sDeploymentsLoadingState,
} from '@/utils/k8sDeploymentPresentation';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';

const PAGE_LIMIT = 100;
const MAX_PAGES = 20;

type K8sDeploymentResource = {
  id: string;
  name?: string;
  status?: string;
  kubernetes?: {
    namespace?: string;
    desiredReplicas?: number;
    updatedReplicas?: number;
    readyReplicas?: number;
    availableReplicas?: number;
  };
};

type ResourcesListResponse = {
  data?: K8sDeploymentResource[];
  meta?: {
    totalPages?: number;
  };
};

const buildDeploymentsUrl = (cluster: string, page: number) => {
  const params = new URLSearchParams();
  params.set('type', 'k8s-deployment');
  params.set('cluster', cluster);
  params.set('page', String(page));
  params.set('limit', String(PAGE_LIMIT));
  return `/api/resources?${params.toString()}`;
};

const fetchAllDeployments = async (cluster: string): Promise<K8sDeploymentResource[]> => {
  const first = await apiFetchJSON<ResourcesListResponse>(buildDeploymentsUrl(cluster, 1), {
    cache: 'no-store',
  });
  const firstData = Array.isArray(first?.data) ? first.data : [];
  const totalPages = Number.isFinite(first?.meta?.totalPages)
    ? Math.max(1, Number(first.meta?.totalPages))
    : 1;

  const deployments: K8sDeploymentResource[] = [...firstData];

  const cappedPages = Math.min(totalPages, MAX_PAGES);
  if (cappedPages > 1) {
    const requests: Array<Promise<ResourcesListResponse>> = [];
    for (let page = 2; page <= cappedPages; page += 1) {
      requests.push(
        apiFetchJSON<ResourcesListResponse>(buildDeploymentsUrl(cluster, page), {
          cache: 'no-store',
        }),
      );
    }

    const settled = await Promise.allSettled(requests);
    for (const result of settled) {
      if (result.status !== 'fulfilled') continue;
      const data = Array.isArray(result.value?.data) ? result.value.data : [];
      deployments.push(...data);
    }
  }

  return Array.from(new Map(deployments.map((d) => [d.id, d])).values());
};

export const K8sDeploymentsDrawer: Component<{
  cluster: string;
  initialNamespace?: string | null;
}> = (props) => {
  const navigate = useNavigate();
  const [namespace, setNamespace] = createSignal('');
  const [search, setSearch] = createSignal('');
  const [lastAppliedNamespace, setLastAppliedNamespace] = createSignal('');
  const drawerPresentation = getK8sDeploymentsDrawerPresentation();

  const clusterName = createMemo(() => asTrimmedString(props.cluster) ?? '');

  const [deployments] = createResource(
    clusterName,
    async (cluster) => {
      if (!cluster) return [];
      return fetchAllDeployments(cluster);
    },
    { initialValue: [] },
  );

  const namespaceOptions = createMemo(() => {
    const set = new Set<string>();
    for (const dep of deployments()) {
      const ns = asTrimmedString(dep.kubernetes?.namespace) ?? '';
      if (ns) set.add(ns);
    }
    return Array.from(set).sort((a, b) => a.localeCompare(b));
  });

  createEffect(() => {
    // Allow other drawer tabs (e.g., Namespaces) to prefill the namespace filter.
    const next = asTrimmedString(props.initialNamespace) ?? '';
    if (!next) return;
    if (next.toLowerCase() === (asTrimmedString(lastAppliedNamespace()) ?? '').toLowerCase())
      return;
    const exists = namespaceOptions().some((ns) => ns.toLowerCase() === next.toLowerCase());
    if (exists) {
      setNamespace(next);
      setLastAppliedNamespace(next);
    }
  });

  createEffect(() => {
    const current = asTrimmedString(namespace()) ?? '';
    if (!current) return;
    const exists = namespaceOptions().some((ns) => ns.toLowerCase() === current.toLowerCase());
    if (!exists) {
      setNamespace('');
    }
  });

  const filteredDeployments = createMemo(() => {
    const ns = asTrimmedString(namespace()) ?? '';
    const term = (asTrimmedString(search()) ?? '').toLowerCase();

    return deployments()
      .filter((dep) => {
        if (ns && (asTrimmedString(dep.kubernetes?.namespace) ?? '') !== ns) return false;
        if (!term) return true;
        const name = asTrimmedString(dep.name) || dep.id;
        return name.toLowerCase().includes(term);
      })
      .sort((a, b) => {
        const aName = (asTrimmedString(a.name) || a.id).toLowerCase();
        const bName = (asTrimmedString(b.name) || b.id).toLowerCase();
        if (aName !== bName) return aName < bName ? -1 : 1;
        return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
      });
  });

  const openPods = (_ns?: string) => {
    if (!clusterName()) return;
    navigate(buildKubernetesPath('workloads'));
  };

  const headingId = () => `k8s-deployments-drawer-heading-${clusterName() || 'cluster'}`;

  return (
    <section class="space-y-3" aria-labelledby={headingId()}>
      <h2 id={headingId()} class="sr-only">
        {clusterName() || 'Kubernetes cluster'} deployments
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

            <Show when={namespaceOptions().length > 0}>
              <div class={filterGroupClass}>
                <label for="k8s-deployments-namespace" class={filterLabelClass}>
                  {drawerPresentation.namespaceFilterLabel}
                </label>
                <select
                  id="k8s-deployments-namespace"
                  class={`${filterSelectClass} min-w-[10rem]`}
                  value={namespace()}
                  aria-label={drawerPresentation.namespaceFilterLabel}
                  onChange={(e) => setNamespace(e.currentTarget.value)}
                >
                  <option value="">{drawerPresentation.allNamespacesLabel}</option>
                  <For each={namespaceOptions()}>{(ns) => <option value={ns}>{ns}</option>}</For>
                </select>
              </div>
            </Show>

            <button
              type="button"
              onClick={() => openPods(namespace() || undefined)}
              class="rounded-md border border-border px-3 py-1 text-xs font-semibold shadow-sm hover:bg-surface-hover"
            >
              {drawerPresentation.openPodsLabel}
            </button>
          </div>
        </div>
      </Card>

      <Show
        when={deployments.loading}
        fallback={
          <Show
            when={filteredDeployments().length > 0}
            fallback={
              <Card padding="lg">
                <EmptyState {...getK8sDeploymentsEmptyState(deployments().length > 0)} />
              </Card>
            }
          >
            <Card padding="none" tone="card" class="overflow-hidden">
              <Table class="min-w-full table-fixed text-xs md:min-w-[820px]">
                <TableHeader>
                  <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                    <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[26%]`}>
                      {drawerPresentation.deploymentColumnLabel}
                    </TableHead>
                    <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>
                      {drawerPresentation.namespaceColumnLabel}
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                    >
                      {drawerPresentation.desiredColumnLabel}
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                    >
                      {drawerPresentation.updatedColumnLabel}
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                    >
                      {drawerPresentation.readyColumnLabel}
                    </TableHead>
                    <TableHead
                      class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[10%]`}
                    >
                      {drawerPresentation.availableColumnLabel}
                    </TableHead>
                    <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[18%]`}>
                      {drawerPresentation.actionsColumnLabel}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                  <For each={filteredDeployments()}>
                    {(dep) => {
                      const name = () => asTrimmedString(dep.name) || dep.id;
                      const ns = () => asTrimmedString(dep.kubernetes?.namespace) || '—';
                      const desired = () => dep.kubernetes?.desiredReplicas ?? 0;
                      const updated = () => dep.kubernetes?.updatedReplicas ?? 0;
                      const ready = () => dep.kubernetes?.readyReplicas ?? 0;
                      const available = () => dep.kubernetes?.availableReplicas ?? 0;
                      const status = () => getSimpleStatusIndicator(dep.status);

                      return (
                        <TableRow class="text-[11px] sm:text-xs">
                          <TableCell class={getPlatformTableCellClassForKind('name')}>
                            <div class="flex items-center gap-2 min-w-0">
                              <StatusDot
                                size="sm"
                                variant={status().variant}
                                title={dep.status || 'unknown'}
                                ariaHidden
                              />
                              <span class="font-semibold text-base-content truncate" title={name()}>
                                {name()}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="block truncate" title={ns()}>
                              {ns()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {desired()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {updated()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {ready()}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content tabular-nums`}
                          >
                            {available()}
                          </TableCell>
                          <TableCell class={getPlatformTableCellClassForKind('text')}>
                            <button
                              type="button"
                              onClick={() => openPods(dep.kubernetes?.namespace)}
                              class="rounded-md border border-border bg-surface px-2 py-1 text-[11px] font-semibold text-base-content shadow-sm hover:bg-surface-hover"
                            >
                              {drawerPresentation.viewPodsLabel}
                            </button>
                          </TableCell>
                        </TableRow>
                      );
                    }}
                  </For>
                </TableBody>
              </Table>
            </Card>
          </Show>
        }
      >
        <Card padding="lg">
          <EmptyState {...getK8sDeploymentsLoadingState()} />
        </Card>
      </Show>
    </section>
  );
};

export default K8sDeploymentsDrawer;
