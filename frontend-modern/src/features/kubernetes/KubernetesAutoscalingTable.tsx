import { For, Show, type Component, type JSX } from 'solid-js';
import { StatusDot } from '@/components/shared/StatusDot';
import { TableCell, TableHead, TableRow } from '@/components/shared/Table';
import { getSimpleStatusIndicator } from '@/utils/status';
import { asTrimmedString } from '@/utils/stringUtils';
import {
  PLATFORM_HEALTH_FILTER_OPTIONS,
  PlatformTableEmptyState,
  PlatformTableToolbar,
  createPlatformTableFilterState,
  getPlatformTableCellClassForKind,
  getPlatformTableHeadClassForKind,
  PlatformTableShell,
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

const textValue = (value: string | undefined): string => asTrimmedString(value) || '—';

const autoscalerName = (resource: Resource): string =>
  asTrimmedString(resource.displayName) || asTrimmedString(resource.name) || resource.id;

const targetRef = (resource: Resource): string =>
  textValue(
    [resource.kubernetes?.targetKind, resource.kubernetes?.targetName].filter(Boolean).join('/'),
  );

const metricSources = (resource: Resource): { label: string; title: string } => {
  const sources = (resource.kubernetes?.metricTypes ?? [])
    .map((source) => asTrimmedString(source))
    .filter((source): source is string => typeof source === 'string' && source.length > 0);
  if (sources.length === 0) return { label: 'Default CPU', title: 'Default CPU utilization' };
  const shown = sources.slice(0, 3);
  const suffix = sources.length > shown.length ? ` +${sources.length - shown.length}` : '';
  return { label: `${shown.join(', ')}${suffix}`, title: sources.join(', ') };
};

const labelSummary = (resource: Resource): { label: string; title: string } => {
  const labels = (resource.tags ?? [])
    .map((label) => asTrimmedString(label))
    .filter((label): label is string => typeof label === 'string' && label.length > 0);
  if (labels.length === 0) return { label: '—', title: '' };
  const shown = labels.slice(0, 2);
  const suffix = labels.length > shown.length ? ` +${labels.length - shown.length}` : '';
  return { label: `${shown.join(', ')}${suffix}`, title: labels.join(', ') };
};

const numberValue = (value: number | undefined): JSX.Element =>
  typeof value === 'number' ? <span class="tabular-nums">{value}</span> : <span>—</span>;

const bounds = (resource: Resource): string => {
  const min = resource.kubernetes?.minReplicas;
  const max = resource.kubernetes?.maxReplicas;
  if (typeof min !== 'number' && typeof max !== 'number') return '—';
  return `${min ?? '—'}-${max ?? '—'}`;
};

export const KubernetesAutoscalingTable: Component<{
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
  const drawer = createPlatformResourceDetailState({ idPrefix: 'kubernetes-autoscaling-drawer' });
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
            searchPlaceholder="Search autoscalers"
            status={tableState.status()}
            onStatusChange={tableState.setStatus}
            statusOptions={PLATFORM_HEALTH_FILTER_OPTIONS}
            visible={tableState.visible()}
            total={tableState.total()}
            rowNoun="autoscalers"
          />
        </Show>

        <Show
          when={tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.emptyIcon}
              title="No autoscalers match current filters"
              description="Adjust the search or status filter to see more Kubernetes autoscalers."
            />
          }
        >
          <PlatformTableShell
            title={props.title ?? 'HorizontalPodAutoscalers'}
            tableClass="min-w-full table-fixed text-xs md:min-w-[1080px]"
            header={
              <>
                <TableHead class={`${getPlatformTableHeadClassForKind('name')} md:w-[20%]`}>
                  Autoscaler
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[16%]`}
                >
                  Scope
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[18%]`}>
                  Scale target
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[10%]`}>
                  Bounds
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[9%]`}>
                  Current
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('numeric-value')} md:w-[9%]`}>
                  Desired
                </TableHead>
                <TableHead class={`${getPlatformTableHeadClassForKind('text')} md:w-[11%]`}>
                  Metrics
                </TableHead>
                <TableHead
                  class={`${getPlatformTableHeadClassForKind('text')} hidden md:table-cell md:w-[7%]`}
                >
                  Labels
                </TableHead>
              </>
            }
            body={
              <>
                <For each={tableState.filtered()}>
                  {(resource) => {
                    const indicator = () => getSimpleStatusIndicator(resource.status);
                    const name = () => autoscalerName(resource);
                    const scope = () => kubernetesScopeLabel(resource);
                    const target = () => targetRef(resource);
                    const metrics = () => metricSources(resource);
                    const labels = () => labelSummary(resource);
                    const detailRowId = () => drawer.detailRowId(resource);
                    const isExpanded = () => drawer.isExpanded(resource);

                    return (
                      <>
                        <TableRow
                          class={`${getPlatformResourceDetailRowClass(isExpanded())} text-[11px] sm:text-xs`}
                          aria-controls={isExpanded() ? detailRowId() : undefined}
                          aria-expanded={isExpanded() ? 'true' : 'false'}
                          data-kubernetes-autoscaling-row={resource.id}
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
                            class={`${getPlatformTableCellClassForKind('text')} hidden text-base-content md:table-cell`}
                          >
                            <span class="inline-block max-w-[12rem] truncate" title={scope()}>
                              {scope()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span class="inline-block max-w-[14rem] truncate" title={target()}>
                              {target()}
                            </span>
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            {bounds(resource)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {numberValue(resource.kubernetes?.currentReplicas)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('numeric-value')} text-base-content`}
                          >
                            {numberValue(resource.kubernetes?.desiredReplicas)}
                          </TableCell>
                          <TableCell
                            class={`${getPlatformTableCellClassForKind('text')} text-base-content`}
                          >
                            <span
                              class="inline-block max-w-[10rem] truncate"
                              title={metrics().title}
                            >
                              {metrics().label}
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
              </>
            }
          />
        </Show>
      </div>
    </Show>
  );
};

export default KubernetesAutoscalingTable;
