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
import type { Resource } from '@/types/resource';
import {
  filterKubernetesResources,
  type KubernetesResourceStatusFilter,
} from './kubernetesPageModel';

// Kubernetes policy resources carry very different API fields. Keep this table
// on the documented NetworkPolicy, PDB, ResourceQuota, and LimitRange shapes
// instead of collapsing them into generic spec/detail text.

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const policyName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const policyKind = (resource: Resource): string => {
  if (resource.type === 'k8s-network-policy') return 'NetworkPolicy';
  if (resource.type === 'k8s-pod-disruption-budget') return 'PodDisruptionBudget';
  if (resource.type === 'k8s-resource-quota') return 'ResourceQuota';
  if (resource.type === 'k8s-limit-range') return 'LimitRange';
  return resource.kubernetes?.resourceKind || resource.type;
};

const policyScope = (resource: Resource): string => {
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

const plural = (count: number, singular: string, pluralLabel = `${singular}s`): string =>
  `${count} ${count === 1 ? singular : pluralLabel}`;

const policyTypes = (resource: Resource): { label: string; title: string } => {
  const explicit = summarizeValues(resource.kubernetes?.policyTypes);
  if (explicit.label !== '—') return explicit;

  const inferred = [
    typeof resource.kubernetes?.ingressRuleCount === 'number' ? 'Ingress' : undefined,
    typeof resource.kubernetes?.egressRuleCount === 'number' ? 'Egress' : undefined,
  ];
  return summarizeValues(inferred);
};

const quotaKeys = (resource: Resource): string[] => {
  const hard = resource.kubernetes?.hard ?? {};
  const used = resource.kubernetes?.used ?? {};
  return Array.from(new Set([...Object.keys(hard), ...Object.keys(used)])).sort();
};

const quotaUsage = (resource: Resource, visible = 2): { label: string; title: string } => {
  const keys = quotaKeys(resource);
  if (keys.length === 0) return { label: 'No quota values', title: '' };
  const hard = resource.kubernetes?.hard ?? {};
  const used = resource.kubernetes?.used ?? {};
  const values = keys.map((key) => `${key} ${used[key] ?? '0'}/${hard[key] ?? '—'}`);
  const shown = values.slice(0, visible);
  const suffix = values.length > shown.length ? ` +${values.length - shown.length}` : '';
  return { label: `${shown.join(', ')}${suffix}`, title: values.join(', ') };
};

const pdbBudget = (resource: Resource): string => {
  const parts = [
    resource.kubernetes?.minAvailable ? `min ${resource.kubernetes.minAvailable}` : undefined,
    resource.kubernetes?.maxUnavailable
      ? `max unavailable ${resource.kubernetes.maxUnavailable}`
      : undefined,
  ].filter(Boolean);
  return parts.join(', ') || 'Availability budget';
};

const pdbHealth = (resource: Resource): string => {
  const healthy =
    typeof resource.kubernetes?.currentHealthy === 'number' ||
    typeof resource.kubernetes?.desiredHealthy === 'number'
      ? `${resource.kubernetes?.currentHealthy ?? 0}/${resource.kubernetes?.desiredHealthy ?? 0} healthy`
      : undefined;
  const disruptions =
    typeof resource.kubernetes?.disruptionsAllowed === 'number'
      ? `${resource.kubernetes.disruptionsAllowed} disruptions`
      : undefined;
  const expected =
    typeof resource.kubernetes?.expectedPods === 'number'
      ? plural(resource.kubernetes.expectedPods, 'pod')
      : undefined;
  return [healthy, disruptions, expected].filter(Boolean).join(', ') || 'No status reported';
};

const networkPolicyRules = (resource: Resource): string => {
  const rules = [
    typeof resource.kubernetes?.ingressRuleCount === 'number'
      ? `${resource.kubernetes.ingressRuleCount} ingress`
      : undefined,
    typeof resource.kubernetes?.egressRuleCount === 'number'
      ? `${resource.kubernetes.egressRuleCount} egress`
      : undefined,
  ].filter(Boolean);
  return rules.join(', ') || 'No rules reported';
};

const policyShape = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-network-policy') {
    const types = policyTypes(resource);
    if (types.label !== '—') {
      return {
        label: `${types.label} policy`,
        title: types.title ? `${types.title} policy` : '',
      };
    }
    return { label: 'Network policy', title: '' };
  }
  if (resource.type === 'k8s-pod-disruption-budget') {
    return { label: 'Availability budget', title: '' };
  }
  if (resource.type === 'k8s-resource-quota') {
    return { label: 'Quota', title: '' };
  }
  if (resource.type === 'k8s-limit-range') {
    return { label: 'Limit range', title: '' };
  }
  return { label: textValue(resource.kubernetes?.resourceKind), title: '' };
};

const policySpec = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-network-policy') {
    return { label: networkPolicyRules(resource), title: networkPolicyRules(resource) };
  }
  if (resource.type === 'k8s-pod-disruption-budget') {
    return { label: pdbBudget(resource), title: pdbBudget(resource) };
  }
  if (resource.type === 'k8s-resource-quota') {
    const count = quotaKeys(resource).length;
    return {
      label: count > 0 ? plural(count, 'hard limit') : 'No hard limits',
      title: quotaUsage(resource, Number.MAX_SAFE_INTEGER).title,
    };
  }
  if (resource.type === 'k8s-limit-range') {
    return summarizeValues(resource.kubernetes?.limitTypes, 3);
  }
  return { label: '—', title: '' };
};

const policyState = (resource: Resource): { label: string; title: string } => {
  if (resource.type === 'k8s-network-policy') {
    return { label: 'Namespace isolation', title: '' };
  }
  if (resource.type === 'k8s-pod-disruption-budget') {
    const health = pdbHealth(resource);
    return { label: health, title: health };
  }
  if (resource.type === 'k8s-resource-quota') {
    return quotaUsage(resource);
  }
  if (resource.type === 'k8s-limit-range') {
    return { label: 'Namespace defaults', title: '' };
  }
  return { label: textValue(resource.status), title: '' };
};

const labelSummary = (resource: Resource): { label: string; title: string } =>
  summarizeValues(resource.tags);

export const KubernetesPolicyTable: Component<{
  resources: Resource[];
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  title?: string;
  showToolbar?: boolean;
}> = (props) => {
  const tableState = createPlatformTableFilterState({
    resources: () => props.resources,
    initialStatus: 'all' as KubernetesResourceStatusFilter,
    filter: filterKubernetesResources,
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
            searchPlaceholder="Search policy resources"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="policy resources"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No policy resources match current filters"
              description="Adjust the search or status filter to see more Kubernetes policy resources."
            />
          }
        >
          <TableCard class={PLATFORM_TABLE_CARD_CLASS}>
            <TableCardHeader
              title={
                props.title ??
                'NetworkPolicies, PodDisruptionBudgets, ResourceQuotas, and LimitRanges'
              }
            />
            <Table class="min-w-full table-fixed text-xs md:min-w-[1180px]">
              <TableHeader>
                <TableRow class={PLATFORM_TABLE_HEADER_ROW_CLASS}>
                  <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[17%]`}>
                    Resource
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[14%]`}>
                    Kind
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[15%]`}
                  >
                    Scope
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[15%]`}>
                    Policy shape
                  </TableHead>
                  <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[16%]`}>
                    Spec / limits
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[14%]`}
                  >
                    Observed state
                  </TableHead>
                  <TableHead
                    class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[9%]`}
                  >
                    Labels
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class={PLATFORM_TABLE_BODY_CLASS}>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => policyName(resource);
                    const scope = () => policyScope(resource);
                    const shape = () => policyShape(resource);
                    const spec = () => policySpec(resource);
                    const state = () => policyState(resource);
                    const labels = () => labelSummary(resource);

                    return (
                      <TableRow
                        class="text-[11px] sm:text-xs"
                        data-kubernetes-policy-row={resource.id}
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
                          {policyKind(resource)}
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
                          <span class="inline-block max-w-[13rem] truncate" title={shape().title}>
                            {shape().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                        >
                          <span class="inline-block max-w-[16rem] truncate" title={spec().title}>
                            {spec().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[14rem] truncate" title={state().title}>
                            {state().label}
                          </span>
                        </TableCell>
                        <TableCell
                          class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                        >
                          <span class="inline-block max-w-[8rem] truncate" title={labels().title}>
                            {labels().label}
                          </span>
                        </TableCell>
                      </TableRow>
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

export default KubernetesPolicyTable;
