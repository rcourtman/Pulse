import { For, Show, createMemo } from 'solid-js';
import type { Component } from 'solid-js';
import { formatUptime } from '@/utils/format';
import { InlineDetailTableRow } from '@/components/shared/InlineDetailTableRow';
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
  getInfrastructureSystemIdentityBadges,
  getInfrastructureSystemTitleBadges,
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
import { UnifiedResourceSourceBadgeCell } from './UnifiedResourceSourceBadgeCell';
import {
  type UnifiedResourceTableProps,
  type UnifiedResourceTableState,
} from './useUnifiedResourceTableState';
import { getPBSTableRow, isResourceOnline } from './unifiedResourceTableModel';
import { buildServiceDetailLinks } from './serviceDetailLinks';
import { ResourceNameWithWebInterfaceLink } from '@/components/shared/WebInterfaceLink';

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
      <Table class={`whitespace-nowrap ${table.tableShellClass()}`}>
        <TableHeader>
          <TableRow class="bg-surface-alt text-muted border-b border-border">
            <TableHead
              class={`text-left pl-2 sm:pl-3 ${table.serviceResourceColumn().className}`}
              width={table.serviceResourceColumn().width}
            >
              {table.headerLabels().resource}
            </TableHead>
            <TableHead
              classList={{ hidden: !table.isServiceVisible('primary') }}
              class={table.serviceCountColumn().className}
              width={table.serviceCountColumn().width}
            >
              {table.headerLabels().datastores}
            </TableHead>
            <TableHead
              classList={{ hidden: !table.isServiceVisible('secondary') }}
              class={table.serviceCountColumn().className}
              width={table.serviceCountColumn().width}
            >
              {table.headerLabels().activity}
            </TableHead>
            <TableHead
              class={table.serviceHealthColumn().className}
              width={table.serviceHealthColumn().width}
            >
              {table.headerLabels().health}
            </TableHead>
            <TableHead
              classList={{ hidden: !table.isServiceVisible('secondary') }}
              class={table.serviceSourceColumn().className}
              width={table.serviceSourceColumn().width}
            >
              {table.headerLabels().source}
            </TableHead>
            <TableHead
              classList={{ hidden: !table.isServiceVisible('supplementary') }}
              class={table.uptimeColumn().className}
              width={table.uptimeColumn().width}
            >
              {table.headerLabels().uptime}
            </TableHead>
            <TableHead
              class={table.serviceActionColumn().className}
              width={table.serviceActionColumn().width}
            >
              {table.headerLabels().action}
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
              const displayName = createMemo(() => getPreferredInfrastructureDisplayName(resource));
              const serviceLink = createMemo(() => buildServiceDetailLinks(resource)[0] ?? null);
              const statusIndicator = createMemo(() =>
                getAgentStatusIndicator({ status: resource.status }),
              );
              const pbsRow = createMemo(() => getPBSTableRow(resource));
              const platformBadge = createMemo(() => getPlatformBadge(resource.platformType));
              const sourceBadge = createMemo(() => getSourceBadge(resource.sourceType));
              const unifiedSources = createMemo(() => table.getUnifiedSources(resource));
              const sourceBadges = createMemo(() => getUnifiedSourceBadges(unifiedSources()));
              const systemBadges = createMemo(() =>
                getInfrastructureSystemIdentityBadges(resource),
              );
              const systemTitleBadges = createMemo(() =>
                getInfrastructureSystemTitleBadges(systemBadges(), sourceBadges()),
              );
              const healthClass = createMemo(
                () =>
                  getServiceHealthSummaryPresentation(resource.status, pbsRow()?.health).textClass,
              );
              const activityClass = createMemo(() =>
                (pbsRow()?.activeTaskCount || 0) > 0
                  ? 'text-[11px] font-semibold text-emerald-700 dark:text-emerald-300'
                  : 'text-xs text-base-content',
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
                        <ResourceNameWithWebInterfaceLink
                          name={displayName()}
                          url={resource.customUrl}
                          class="min-w-0 flex-1"
                          nameClass="block min-w-0 flex-1 truncate font-medium text-[11px] text-base-content select-text"
                        />
                        <Show when={shouldShowResourceAlternateName(resource)}>
                          <span class="hidden min-w-0 max-w-[35%] shrink truncate text-[9px] text-muted lg:inline">
                            ({resource.name})
                          </span>
                        </Show>
                        <ResourceFacetSummary
                          recentChanges={resource.recentChanges}
                          counts={resource.facetCounts}
                          maxVisibleBadges={1}
                          class="hidden max-w-[48%] shrink-0 flex-nowrap overflow-hidden lg:flex"
                        />
                      </div>
                    </TableCell>

                    <TableCell classList={{ hidden: !table.isServiceVisible('primary') }}>
                      <div class="flex justify-center">
                        <Show
                          when={pbsRow()?.datastores != null}
                          fallback={
                            <span class="text-xs text-slate-400" aria-hidden="true">
                              —
                            </span>
                          }
                        >
                          <span class="text-xs text-base-content">{pbsRow()!.datastores}</span>
                        </Show>
                      </div>
                    </TableCell>

                    <TableCell classList={{ hidden: !table.isServiceVisible('secondary') }}>
                      <div class="flex justify-center">
                        <Show
                          when={pbsRow()?.activity}
                          fallback={
                            <span class="text-xs text-slate-400" aria-hidden="true">
                              —
                            </span>
                          }
                        >
                          <div
                            class="flex flex-col items-center leading-tight"
                            title={[pbsRow()!.activity, pbsRow()!.activityDetail]
                              .filter(Boolean)
                              .join(' · ')}
                          >
                            <span class={activityClass()}>{pbsRow()!.activity}</span>
                            <Show when={pbsRow()?.activityDetail}>
                              <span class="text-[10px] text-muted">{pbsRow()!.activityDetail}</span>
                            </Show>
                          </div>
                        </Show>
                      </div>
                    </TableCell>

                    <TableCell>
                      <div class="flex justify-center">
                        <Show
                          when={pbsRow()?.health}
                          fallback={
                            <span class="text-xs text-slate-400" aria-hidden="true">
                              —
                            </span>
                          }
                        >
                          <span class={`text-xs font-medium ${healthClass()}`}>
                            {pbsRow()!.health}
                          </span>
                        </Show>
                      </div>
                    </TableCell>

                    <TableCell classList={{ hidden: !table.isServiceVisible('secondary') }}>
                      <UnifiedResourceSourceBadgeCell
                        unifiedBadges={systemBadges()}
                        platformBadge={platformBadge()}
                        sourceBadge={sourceBadge()}
                        titleBadges={systemTitleBadges()}
                        layoutMode={table.layoutMode()}
                      />
                    </TableCell>

                    <TableCell
                      classList={{
                        hidden: !table.isServiceVisible('supplementary'),
                      }}
                    >
                      <div class="flex justify-center">
                        <Show
                          when={resource.uptime}
                          fallback={
                            <span class="text-xs text-slate-400" aria-hidden="true">
                              —
                            </span>
                          }
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
                          fallback={
                            <span class="text-xs text-slate-400" aria-hidden="true">
                              —
                            </span>
                          }
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
                    <InlineDetailTableRow
                      cellId={detailControlsId()}
                      cellClass="border-border-subtle shadow-inner"
                      colspan={7}
                      contentClass="px-4 py-4"
                      data-inline-detail-for={resource.id}
                    >
                      <ResourceDetailDrawer
                        resource={resource}
                        resolveResourceLabel={table.resolveResourceLabel}
                        onClose={() => tableProps.onExpandedResourceChange(null)}
                      />
                    </InlineDetailTableRow>
                  </Show>
                </>
              );
            }}
          </For>
        </TableBody>
      </Table>
    </Show>
  );
};
