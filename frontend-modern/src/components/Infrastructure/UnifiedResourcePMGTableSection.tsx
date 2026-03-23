import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import { formatUptime } from '@/utils/format';
import { StatusDot } from '@/components/shared/StatusDot';
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
import { getPreferredResourceDisplayName } from '@/utils/resourceIdentity';
import { shouldShowResourceAlternateName } from '@/utils/resourcePolicyPresentation';
import { ResourceDetailDrawer } from './ResourceDetailDrawer';
import { ResourceFacetSummary } from './ResourceFacetSummary';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { getPMGTableRow, isResourceOnline } from './unifiedResourceTableModel';
import { buildServiceDetailLinks } from './serviceDetailLinks';

interface UnifiedResourcePMGTableSectionProps {
  tableProps: UnifiedResourceTableProps;
  table: UnifiedResourceTableState;
}

export const UnifiedResourcePMGTableSection: Component<UnifiedResourcePMGTableSectionProps> = (
  props,
) => {
  const { table, tableProps } = props;

  return (
    <Show when={table.sortedPMGResources().length > 0}>
      <div class="border-b border-border bg-surface-hover px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-muted">
        PMG Services
      </div>
      <div class="overflow-x-auto">
        <Table
          class="whitespace-nowrap min-w-[max-content]"
          style={{
            'table-layout': 'fixed',
            'min-width': table.isMobile() ? '100%' : 'max-content',
          }}
        >
          <TableHeader>
            <TableRow class="bg-surface-alt text-muted border-b border-border">
              <TableHead class="text-left pl-2 sm:pl-3" style={table.resourceColumnStyle()}>
                Resource
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('primary') && !table.isMobile() }}
                style={table.serviceQueueColumnStyle()}
              >
                Queue
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}
                style={table.serviceQueueColumnStyle()}
              >
                Deferred
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('supplementary') && !table.isMobile() }}
                style={table.serviceQueueColumnStyle()}
              >
                Hold
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}
                style={table.serviceCountColumnStyle()}
              >
                Nodes
              </TableHead>
              <TableHead style={table.serviceHealthColumnStyle()}>Health</TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}
                style={table.sourceColumnStyle()}
              >
                Source
              </TableHead>
              <TableHead
                classList={{ hidden: !table.isVisible('supplementary') && !table.isMobile() }}
                style={table.uptimeColumnStyle()}
              >
                Uptime
              </TableHead>
              <TableHead style={table.serviceActionColumnStyle()}>Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <For each={table.sortedPMGResources()}>
              {(resource) => {
                const isExpanded = createMemo(() => tableProps.expandedResourceId === resource.id);
                const isHighlighted = createMemo(
                  () => tableProps.highlightedResourceId === resource.id,
                );
                const displayName = createMemo(() => getPreferredResourceDisplayName(resource));
                const serviceLink = createMemo(
                  () => buildServiceDetailLinks(resource)[0] ?? null,
                );
                const statusIndicator = createMemo(() =>
                  getAgentStatusIndicator({ status: resource.status }),
                );
                const pmgRow = createMemo(() => getPMGTableRow(resource));
                const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
                const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
                const unifiedSourceBadges = createMemo(() =>
                  getUnifiedSourceBadges(table.getUnifiedSources(resource)),
                );
                const hasUnifiedSources = createMemo(() => unifiedSourceBadges().length > 0);
                const healthClass = createMemo(
                  () =>
                    getServiceHealthSummaryPresentation(resource.status, pmgRow()?.health)
                      .textClass,
                );
                const queueClass = createMemo(() =>
                  (pmgRow()?.deferred || 0) + (pmgRow()?.hold || 0) > 0
                    ? 'text-amber-600 dark:text-amber-400'
                    : 'text-base-content',
                );

                const rowClass = createMemo(() => {
                  const baseBorder = 'border-b border-border-subtle';
                  const baseHover = `cursor-pointer transition-all duration-200 relative hover:shadow-sm group ${baseBorder}`;

                  if (isExpanded()) {
                    return `cursor-pointer transition-all duration-200 relative hover:shadow-sm z-10 group bg-blue-50 dark:bg-blue-900 ${baseBorder}`;
                  }

                  let className = baseHover;
                  if (isHighlighted()) {
                    className +=
                      ' bg-blue-50 dark:bg-blue-900 ring-1 ring-blue-300 dark:ring-blue-600';
                  }
                  if (tableProps.hoveredResourceId === resource.id) {
                    className += ' bg-blue-100 dark:bg-blue-800';
                  }
                  if (!isResourceOnline(resource)) {
                    className += ' opacity-60';
                  }

                  return className;
                });

                return (
                  <>
                    <TableRow
                      ref={(el) => table.registerRowRef(resource.id, el)}
                      class={rowClass()}
                      style={{ 'min-height': '32px' }}
                      onClick={() => table.toggleExpand(resource.id)}
                      onMouseEnter={() => tableProps.onHoverChange?.(resource.id)}
                      onMouseLeave={() => tableProps.onHoverChange?.(null)}
                    >
                      <TableCell class="pr-1.5 sm:pr-2 py-0.5 align-middle overflow-hidden pl-2 sm:pl-3">
                        <div class="flex items-center gap-1.5 min-w-0">
                          <div
                            class={`shrink-0 transition-transform duration-200 ${isExpanded() ? 'rotate-90' : ''}`}
                          >
                            <svg
                              class="w-3.5 h-3.5 text-muted group-hover:text-base-content"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                            >
                              <path
                                stroke-linecap="round"
                                stroke-linejoin="round"
                                stroke-width="2"
                                d="M9 5l7 7-7 7"
                              />
                            </svg>
                          </div>
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
                            when={pmgRow()?.queue != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class={`text-xs font-medium ${queueClass()}`}>{pmgRow()!.queue}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={pmgRow()?.deferred != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content">{pmgRow()!.deferred}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('supplementary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={pmgRow()?.hold != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content">{pmgRow()!.hold}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell classList={{ hidden: !table.isVisible('secondary') && !table.isMobile() }}>
                        <div class="flex justify-center">
                          <Show
                            when={pmgRow()?.nodes != null}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class="text-xs text-base-content">{pmgRow()!.nodes}</span>
                          </Show>
                        </div>
                      </TableCell>

                      <TableCell>
                        <div class="flex justify-center">
                          <Show
                            when={pmgRow()?.health}
                            fallback={<span class="text-xs text-slate-400">—</span>}
                          >
                            <span class={`text-xs font-medium ${healthClass()}`}>
                              {pmgRow()!.health}
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
                      <TableRow>
                        <TableCell
                          colspan={9}
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
