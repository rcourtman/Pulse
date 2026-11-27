import { Show, type JSX } from 'solid-js';
import type { FilterDef } from '@/components/shared/FilterBar';
import { StatusDot } from '@/components/shared/StatusDot';
import {
  PlatformTableEmptyState,
  PlatformTableShell,
  PlatformTableToolbar,
  formatPlatformTableTitleCaseValue,
  getPlatformTableFiniteMetric,
  type PlatformTableFilterOption,
  withPlatformStatusCounts,
} from '@/features/platformPage/sharedPlatformPage';
import {
  getPlatformInlineStateClass,
  getPlatformStatusTextToneClass,
} from '@/features/platformPage/platformStatusTone';
import {
  PlatformResourceDetailTableRow,
  PlatformResourceDetailToggleButton,
  createPlatformResourceDetailState,
  type PlatformResourceDetailState,
} from '@/features/platformPage/PlatformResourceDetailTableRow';
import type { Resource } from '@/types/resource';
import { asTrimmedString } from '@/utils/stringUtils';
import type { StatusIndicator, StatusIndicatorVariant } from '@/utils/status';
import { getTrueNASStaleNotice, type TrueNASStaleNotice } from './truenasPageModel';

export const createTrueNASDetailState = (idPrefix: string): PlatformResourceDetailState =>
  createPlatformResourceDetailState({ idPrefix });

export type TrueNASStateNote = {
  short: string | null;
  full: string;
  tone: 'danger' | 'warning';
};

export const resolveTrueNASAttentionNote = (
  resource: Resource,
  rawState: string | undefined,
): TrueNASStateNote => {
  const stale = getTrueNASStaleNotice(resource);
  if (stale) return { short: 'not updated', full: stale.detail, tone: 'warning' };
  const state = asTrimmedString(rawState);
  return {
    short: state ? null : 'not reported',
    full: state
      ? `TrueNAS reports ${formatPlatformTableTitleCaseValue(state)}`
      : 'TrueNAS has not reported a state',
    tone: 'warning',
  };
};

export function TrueNASStateNoteDetails(props: {
  note: TrueNASStateNote | null;
  labelClass: string;
  label: string;
  title?: string;
  dataAttr: 'service' | 'vm' | 'share';
  shareVariant?: StatusIndicatorVariant;
}) {
  return (
    <div
      class="inline-flex max-w-full min-w-0 items-center gap-1.5"
      title={props.title ?? props.note?.full}
      data-truenas-service-state-note={props.dataAttr === 'service' ? props.note?.tone : undefined}
      data-truenas-vm-state-note={props.dataAttr === 'vm' ? props.note?.tone : undefined}
      data-truenas-share-state={props.dataAttr === 'share' ? props.shareVariant : undefined}
    >
      <span class={props.labelClass}>{props.label}</span>
      <Show when={props.note?.short}>
        {(short) => (
          <span
            class={`min-w-0 truncate text-[11px] font-medium ${getPlatformStatusTextToneClass(props.note!.tone)}`}
          >
            {short()}
          </span>
        )}
      </Show>
      <Show when={props.note}>{(current) => <span class="sr-only">{current().full}</span>}</Show>
    </div>
  );
}

export function TrueNASPhoneStateNote(props: {
  note: TrueNASStateNote | null;
  stateLabel: string;
  dataAttr: 'service' | 'vm';
}) {
  const tone = () => (props.note ? props.note.tone : 'none');
  return (
    <span
      class={`platform-table-phone-only-inline text-[11px] ${getPlatformInlineStateClass(Boolean(props.note))} ${props.note ? getPlatformStatusTextToneClass(props.note.tone) : ''}`}
      data-truenas-service-state-phone={props.dataAttr === 'service' ? tone() : undefined}
      data-truenas-vm-state-phone={props.dataAttr === 'vm' ? tone() : undefined}
    >
      <span class="min-w-0 truncate">
        {[props.stateLabel, props.note?.short].filter(Boolean).join(' · ')}
      </span>
    </span>
  );
}

export type TrueNASCommonTableProps = {
  emptyIcon: JSX.Element;
  emptyTitle: string;
  emptyDescription: string;
  showToolbar?: boolean;
};

export function TrueNASStaleBadge(props: {
  notice: TrueNASStaleNotice | null;
  attr: 'system' | 'storage';
}) {
  return (
    <Show when={props.notice}>
      {(notice) => (
        <span
          class="block truncate text-[10px] leading-tight font-medium text-amber-800 dark:text-amber-300"
          title={notice().detail}
          data-truenas-system-stale={props.attr === 'system' ? 'true' : undefined}
          data-truenas-storage-stale={props.attr === 'storage' ? 'true' : undefined}
        >
          {notice().badge}
        </span>
      )}
    </Show>
  );
}

export const getTrueNASMemoryPercentSortValue = (resource: Resource): number | null => {
  const total = getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
  if (total > 0) return ((getPlatformTableFiniteMetric(resource.memory?.used) ?? 0) / total) * 100;
  return getPlatformTableFiniteMetric(resource.memory?.current) ?? null;
};

export const getTrueNASMemoryDisplay = (resource: Resource) => {
  const total = getPlatformTableFiniteMetric(resource.memory?.total) ?? 0;
  const used = getPlatformTableFiniteMetric(resource.memory?.used) ?? 0;
  const percentOnly =
    total > 0 ? undefined : getPlatformTableFiniteMetric(resource.memory?.current);
  return { total, used, percentOnly, hasMetric: total > 0 || percentOnly !== undefined };
};

export function TrueNASRowLead<T extends { id: string }>(props: {
  item: T;
  drawer: PlatformResourceDetailState;
  label: string;
  variant: StatusIndicatorVariant;
  dotTitle: string;
  dotAriaHidden?: boolean;
  pulse?: boolean;
  class?: string;
  indentDepth?: number;
  children: JSX.Element;
}) {
  return (
    <div
      class={`flex min-w-0 items-center gap-2 ${props.class ?? ''}`}
      data-truenas-storage-indent-depth={props.indentDepth}
    >
      <PlatformResourceDetailToggleButton
        expanded={props.drawer.isExpanded(props.item)}
        resourceLabel={props.label}
        controlsId={props.drawer.detailRowId(props.item)}
        onToggle={() => props.drawer.toggle(props.item)}
      />
      <StatusDot
        size="sm"
        variant={props.variant}
        pulse={props.pulse}
        title={props.dotTitle}
        ariaHidden={props.dotAriaHidden}
      />
      {props.children}
    </div>
  );
}

export function TrueNASResourceDetailRow(props: {
  resource: Resource;
  indicator: StatusIndicator;
  drawer: PlatformResourceDetailState;
  colSpan: number;
  resolveResourceLabel: (resourceId: string) => string | undefined;
}) {
  return (
    <PlatformResourceDetailTableRow
      resource={props.resource}
      statusIndicator={props.indicator}
      open={props.drawer.isExpanded(props.resource)}
      detailRowId={props.drawer.detailRowId(props.resource)}
      colSpan={props.colSpan}
      resolveResourceLabel={props.resolveResourceLabel}
      onClose={() => props.drawer.close(props.resource)}
    />
  );
}

type TrueNASTableFilterState<TStatus extends string> = {
  search: () => string;
  setSearch: (value: string) => void;
  searchSuggestions: Parameters<typeof PlatformTableToolbar>[0]['searchSuggestions'];
  status: () => TStatus;
  setStatus: (value: TStatus) => void;
  filtered: () => readonly unknown[];
  countForStatus: (value: TStatus) => number;
  visible: () => number;
  total: () => number;
};

export function TrueNASTableFrame<TStatus extends string>(props: {
  hasItems: boolean;
  tableState: TrueNASTableFilterState<TStatus>;
  common: TrueNASCommonTableProps;
  filteredEmptyTitle?: string;
  filteredEmptyDescription?: string;
  searchPlaceholder: string;
  statusOptions: PlatformTableFilterOption<TStatus>[];
  countForStatus?: (value: TStatus) => number;
  precounted?: boolean;
  filters?: FilterDef[];
  rowNoun: string;
  hasActiveFilters?: boolean;
  onResetFilters?: () => void;
  notice?: JSX.Element;
  title: string;
  actions?: JSX.Element;
  tableClass: string;
  header: JSX.Element;
  body: JSX.Element;
}) {
  const noun = () => props.rowNoun;
  return (
    <Show
      when={props.hasItems}
      fallback={
        <PlatformTableEmptyState
          icon={props.common.emptyIcon}
          title={props.common.emptyTitle}
          description={props.common.emptyDescription}
        />
      }
    >
      <div class="space-y-3">
        {props.notice}
        <Show when={props.common.showToolbar !== false}>
          <PlatformTableToolbar
            search={props.tableState.search}
            onSearchChange={props.tableState.setSearch}
            searchPlaceholder={props.searchPlaceholder}
            searchSuggestions={props.tableState.searchSuggestions}
            status={props.tableState.status()}
            onStatusChange={props.tableState.setStatus}
            statusOptions={
              props.precounted
                ? props.statusOptions
                : withPlatformStatusCounts(
                    props.statusOptions,
                    props.countForStatus ?? props.tableState.countForStatus,
                  )
            }
            filters={props.filters}
            visible={props.tableState.visible()}
            total={props.tableState.total()}
            rowNoun={noun()}
            hasActiveFilters={props.hasActiveFilters}
            onResetFilters={props.onResetFilters}
          />
        </Show>

        <Show
          when={props.tableState.filtered().length > 0}
          fallback={
            <PlatformTableEmptyState
              icon={props.common.emptyIcon}
              title={props.filteredEmptyTitle ?? `No ${noun()} match current filters`}
              description={
                props.filteredEmptyDescription ??
                `Adjust the search or status filter to see more TrueNAS ${noun()}.`
              }
            />
          }
        >
          <PlatformTableShell
            title={props.title}
            actions={props.actions}
            tableClass={props.tableClass}
            header={props.header}
            body={props.body}
          />
        </Show>
      </div>
    </Show>
  );
}
