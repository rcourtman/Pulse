import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import { formatUptime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  buildSummaryDisclosureControlsId,
  createSummaryInteractiveRowPreviewHandlers,
} from '@/components/shared/summaryInteractionA11y';
import { SummaryRowActionButton } from '@/components/shared/SummaryRowActionButton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import {
  getPlatformBadge,
  getSourceBadge,
  getUnifiedSourceBadges,
} from '@/utils/resourceBadgePresentation';
import { getAgentStatusIndicator } from '@/utils/status';
import { getServiceHealthSummaryPresentation } from '@/utils/serviceHealthPresentation';
import { getPreferredInfrastructureDisplayName } from '@/utils/resourceIdentity';
import { shouldShowResourceAlternateName } from '@/utils/resourcePolicyPresentation';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { getPBSTableRow, isResourceOnline } from './unifiedResourceTableModel';
import { buildServiceDetailLinks } from './serviceDetailLinks';

interface UnifiedResourcePBSTableSectionProps {
  tableProps: UnifiedResourceTableProps;
  table: UnifiedResourceTableState;
}

export const UnifiedResourcePBSTableSection: Component<UnifiedResourcePBSTableSectionProps> = (
  props,
) => {
  const { table, tableProps } = props;

  return (
    <Show when={table.sortedPBSResources().length > 0}>
      <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
        PBS Services
      </div>
      <div class="overflow-x-auto">
        <Table class={`whitespace-nowrap ${table.tableShellClass()}`}>
          <TableHeader>
            <TableRow class="bg-surface-alt text-muted border-b border-border">
              <TableHead
                class={`text-left pl-2 sm:pl-3 ${table.resourceColumn().className}`}
                width={table.resourceColumn().width}
              >
                Resource
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('primary') && !table.isMobile() }}
                class={table.serviceCountColumn().className}
                width={table.serviceCountColumn().width}
              >
                Datastores
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}
                class={table.serviceCountColumn().className}
                width={table.serviceCountColumn().width}
              >
                Jobs
              </TableHead>
              <TableHead
                class={table.serviceHealthColumn().className}
                width={table.serviceHealthColumn().width}
              >
                Health
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}
                class={table.sourceColumn().className}
                width={table.sourceColumn().width}
              >
                Source
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('supplementary') && !table.isMobile() }}
                class={table.uptimeColumn().className}
                width={table.uptimeColumn().width}
              >
                Uptime
              </TableHead>
              <TableHead
                class={table.serviceActionColumn().className}
                width={table.serviceActionColumn().width}
              >
                Action
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <For each={table.sortedPBSResources()}>
              {(resource) => {
                const isExpanded = createMemo(() => tableProps.expandedResourceId === resource.id);
                const isHighlighted = createMemo(
                  () => tableProps.highlightedResourceId === resource.id,
                );
                const displayName = createMemo(() =>
                  getPreferredInfrastructureDisplayName(resource),
                );
                const serviceLink = createMemo(
                  () => buildServiceDetailLinks(resource)[0] ?? null,
                );
                const statusIndicator = createMemo(() =>
                  getAgentStatusIndicator({ status: resource.status }),
                );
                const pbsRow = createMemo(() => getPBSTableRow(resource));
                const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                const unifiedSourceBadges = createMemo(() =>
                  getUnifiedSourceBadges(table.getUnifiedSources(resource)),
                );
                const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                const healthClass = createMemo(
                  () =>
                    getServiceHealthSummaryPresentation(resource.status, pbsRow()?.health)
                      .textClass,
                );

                const rowClass = createMemo(() => {
                  const baseBorder = 'border-b border-border-subtle';
                  const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                  if (isExpanded()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900 ${baseBorder}`;
                  }

                  let className = baseHover;
                  if (!isResourceOnline(resource)) {
                    className += ' opacity-60';
                  }

                  return className;
                });
                const detailControlsId = createMemo(() =>
                  buildSummaryDisclosureControlsId(resource.id),
                );
                const resourceRowInteraction = createSummaryInteractiveRowPreviewHandlers({
                  onPreview: () => tableProps.onHoverChange?.(resource.id),
                  onPreviewClear: () => tableProps.onHoverChange?.(null),
                });

                return (
                  <>
                    <TableRow
                      data-summary-series-id={resource.id}
                      data-summary-row-active={
                        (tableProps.hoveredResourceId === resource.id || isHighlighted()) &&
                        !isExpanded()
                          ? 'true'
                          : 'false'
                      }
                      class={`${rowClass()} h-8`}
                      onClick={() => table.toggleExpand(resource.id)}
                      {...resourceRowInteraction}
                    >
                      <TableCell class="pr-1.5 sm:pr-2 py-0.5 align-middle overflow-hidden pl-2 sm:pl-3">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <SummaryRowActionButton
                            kind="disclosure"
                            subjectLabel={displayName()}
                            expanded={isExpanded()}
                            controlsId={detailControlsId()}
                            onAction={() => table.toggleExpand(resource.id)}
                            onPreviewClear={() => tableProps.onHoverChange?.(null)}
                          />
                          <StatusDot
                            variant={statusIndicator().variant}
                            title={statusIndicator().label}
                            ariaLabel={statusIndicator().label}
                            size="xs"
                          />
                          <span
                            class="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                            title={displayName()}
                          >
                            {displayName()}
                          </span>
                          <Show when={shouldShowResourceAlternateName(resource)}>
                            <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-muted lg:inline">
                              ({resource.name})
                            </span>
                          </Show>
                          <ResourceFacetSummary
                            recentChanges={resource.recentChanges}
                            counts={resource.facetCounts}
                            class="mt-0.5"
                          />
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('primary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={pbsRow()?.datastores != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content">{pbsRow()!.datastores}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={pbsRow()?.jobs != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content">{pbsRow()!.jobs}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div class="flex justify-center">
                          <Show
                            when={pbsRow()?.health}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class={`text-xs font-medium ${healthClass()}`}>
                              {pbsRow()!.health}
                            </span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}>
                        <div class="flex items-center justify-center gap-1">
                          <Show
                            when={hasUnifiedSources()}
                            fallback={
                              <>
                                <Show when={platformBadge()}>
                                  {(badge) => (
                                    <span class={badge().classes} title={badge().title}>
                                      {badge().label}
                                    </span>
                                  )}
                                </Show>
                                <Show when={sourceBadge()}>
                                  {(badge) => (
                                    <span class={badge().classes} title={badge().title}>
                                      {badge().label}
                                    </span>
                                  )}
                                </Show>
                              </>
                            }
                          >
                            <For each={unifiedSourceBadges()}>
                              {(badge) => (
                                <span class={badge.classes} title={badge.title}>
                                  {badge.label}
                                </span>
                              )}
                            </For>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('supplementary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={resource.uptime}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content whitespace-nowrap">
                              {formatUptime(resource.uptime ?? 0)}
                            </span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div class="flex justify-center">
                          <Show
                            when={serviceLink()}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            {(link) => (
                              <a
                                href={link().href}
                                class="inline-flex items-center rounded border border-blue-200 bg-blue-50 px-1.5 py-0.5 text-[10px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900 dark:text-blue-200 dark:hover:bg-blue-800"
                                title={link().label}
                                aria-label={link().ariaLabel}
                                onClick={(event) => event.stopPropagation()}
                              >
                                {link().compactLabel}
                              </a>
                            )}
                          </Show>
                        </div>
                      </TableCell>
                    </TableRow>
                    <Show when={isExpanded()}>
                      <TableRow data-inline-detail-for={resource.id}>
                        <TableCell
                          id={detailControlsId()}
                          colspan={7}
                          class="bg-surface-alt px-4 py-4 border-b border-border-subtle shadow-inner"
                        >
                          <ResourceDetailDrawer
                            resource={resource}
                            resolveResourceLabel={table.resolveResourceLabel}
                            onClose={() => tableProps.onExpandedResourceChange(null)}
                          />
                        </TableCell>
                      </TableRow>
                    </Show>
                  </>
                );
              }}
            </For>
          </TableBody>
        </Table>
      </div>
    </Show>
  );
};
