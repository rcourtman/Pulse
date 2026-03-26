import { For, Show } from 'solid-js';
import type { Accessor, Component } from 'solid-js';

import { RecoveryPointDetails } from '@/components/Recovery/RecoveryPointDetails';
import { EmptyState } from '@/components/shared/EmptyState';
import { getSourcePlatformBadge } from '@/components/shared/sourcePlatformBadges';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/shared/Table';
import type { ColumnDef } from '@/hooks/useColumnVisibility';
import { formatBytes } from '@/utils/format';
import { getRecoveryDrawerCloseButtonClass, getRecoveryEmptyStateActionClass } from '@/utils/recoveryActionPresentation';
import { getRecoveryArtifactModePresentation, type RecoveryArtifactMode } from '@/utils/recoveryArtifactModePresentation';
import {
  getRecoveryHistoryEmptyState,
  getRecoveryPointsLoadingState,
} from '@/utils/recoveryEmptyStatePresentation';
import { getRecoveryOutcomeBadgeClass } from '@/utils/recoveryOutcomePresentation';
import {
  getRecoveryPointDetailsSummary,
  getRecoveryPointModeLabel,
  getRecoveryPointRepositoryLabel,
  getRecoveryPointSubjectLabel,
  getRecoveryPointTimestampMs,
  normalizeRecoveryModeQueryValue,
} from '@/utils/recoveryRecordPresentation';
import {
  getRecoveryArtifactColumnHeaderClass,
  getRecoveryArtifactRowClass,
  getRecoveryEventTimeTextClass,
  getRecoverySubjectTypeBadgeClass,
  getRecoverySubjectTypeLabel,
  RECOVERY_GROUP_HEADER_ROW_CLASS,
  RECOVERY_GROUP_HEADER_TEXT_CLASS,
} from '@/utils/recoveryTablePresentation';
import { normalizeSourcePlatformQueryValue, getSourcePlatformLabel } from '@/utils/sourcePlatforms';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';
import type { RecoveryOutcome, RecoveryPoint } from '@/types/recovery';
import type { Resource } from '@/types/resource';
import { formatRecoveryTimeOnly } from '@/utils/recoveryDatePresentation';

type ArtifactMode = RecoveryArtifactMode;

export interface RecoveryPointGroup {
  key: string;
  label: string;
  tone: 'recent' | 'default';
  items: RecoveryPoint[];
}

interface RecoveryPointsMeta {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

export interface RecoveryPointsModel {
  meta: Accessor<RecoveryPointsMeta>;
  response: {
    loading: boolean;
    error: unknown;
  };
}

interface RecoveryHistoryTableProps {
  currentPage: Accessor<number>;
  groupedByDay: Accessor<RecoveryPointGroup[]>;
  hasActiveArtifactFilters: Accessor<boolean>;
  mobileVisibleArtifactColumns: Accessor<ColumnDef[]>;
  recoveryPoints: RecoveryPointsModel;
  resetAllArtifactFilters: () => void;
  resourcesById: Accessor<Map<string, Resource>>;
  selectedPoint: Accessor<RecoveryPoint | null>;
  setCurrentPage: (value: number) => void;
  tableColumnCount: Accessor<number>;
  tableMinWidth: Accessor<string>;
  toggleSelectedPoint: (point: RecoveryPoint) => void;
  totalPages: Accessor<number>;
  clearSelectedPoint: () => void;
}

export const RecoveryHistoryTable: Component<RecoveryHistoryTableProps> = (props) => (
  <Show
    when={props.groupedByDay().length > 0}
    fallback={
      <div class="p-6">
        <Show
          when={props.recoveryPoints.response.loading}
          fallback={
            <EmptyState
              {...getRecoveryHistoryEmptyState()}
              actions={
                <Show when={props.hasActiveArtifactFilters()}>
                  <button
                    type="button"
                    onClick={props.resetAllArtifactFilters}
                    class={getRecoveryEmptyStateActionClass()}
                  >
                    Clear filters
                  </button>
                </Show>
              }
            />
          }
        >
          <div class="text-sm text-muted">{getRecoveryPointsLoadingState().text}</div>
        </Show>
      </div>
    }
  >
    <div class="overflow-x-auto">
      <Table
        class="w-full border-collapse text-xs whitespace-nowrap"
        style={{ 'min-width': props.tableMinWidth(), 'table-layout': 'fixed' }}
      >
        <TableHeader>
          <TableRow class="bg-surface-alt text-muted border-b border-border">
            <For each={props.mobileVisibleArtifactColumns()}>
              {(column) => (
                <TableHead
                  class={`py-0.5 px-3 text-left text-[11px] sm:text-xs font-medium uppercase tracking-wider whitespace-nowrap ${getRecoveryArtifactColumnHeaderClass(
                    column.id,
                  )}`}
                >
                  {column.label}
                </TableHead>
              )}
            </For>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <For each={props.groupedByDay()}>
            {(group) => (
              <>
                <TableRow class={RECOVERY_GROUP_HEADER_ROW_CLASS}>
                  <TableCell colSpan={props.tableColumnCount()} class={RECOVERY_GROUP_HEADER_TEXT_CLASS}>
                    <div class="flex items-center justify-between gap-3">
                      <div class="flex min-w-0 items-center gap-2">
                        <span class="truncate" title={group.label}>
                          {group.label}
                        </span>
                        <Show when={group.tone === 'recent'}>
                          <span class="inline-flex rounded px-1.5 py-px text-[9px] font-medium border border-border bg-surface">
                            recent
                          </span>
                        </Show>
                      </div>
                      <span class="shrink-0 font-mono text-[10px] tabular-nums text-muted">
                        {group.items.length}
                      </span>
                    </div>
                  </TableCell>
                </TableRow>

                <For each={group.items}>
                  {(point) => {
                    const resourceIndex = props.resourcesById();
                    const subject = getRecoveryPointSubjectLabel(point, resourceIndex);
                    const tsMs = getRecoveryPointTimestampMs(point);
                    const timeOnly =
                      point.completedAt && Number.isFinite(tsMs)
                        ? formatRecoveryTimeOnly(tsMs)
                        : '—';
                    const subjectType = getRecoverySubjectTypeLabel(point);
                    const provider = normalizeSourcePlatformQueryValue(
                      String(point.provider || '').trim(),
                    );
                    const mode =
                      normalizeRecoveryModeQueryValue(String(point.mode || '').toLowerCase()) ||
                      'local';
                    const outcome =
                      (String(point.outcome || 'unknown').toLowerCase() as RecoveryOutcome) ||
                      'unknown';
                    const repoLabel = getRecoveryPointRepositoryLabel(point);
                    const detailsSummary = getRecoveryPointDetailsSummary(point);
                    const entityId = String(point.entityId || '').trim();
                    const cluster = String(point.cluster || '').trim();
                    const nodeAgent = String(point.node || '').trim();
                    const namespace = String(point.namespace || '').trim();

                    return (
                      <>
                        <TableRow
                          class={`cursor-pointer ${getRecoveryArtifactRowClass(
                            props.selectedPoint()?.id === point.id,
                          )}`}
                          onClick={() => props.toggleSelectedPoint(point)}
                        >
                          <For each={props.mobileVisibleArtifactColumns()}>
                            {(column) => {
                              switch (column.id) {
                                case 'time':
                                  return (
                                    <TableCell
                                      class={`whitespace-nowrap px-3 py-0.5 text-right font-mono text-[11px] tabular-nums ${getRecoveryEventTimeTextClass(
                                        tsMs,
                                      )}`}
                                    >
                                      {timeOnly}
                                    </TableCell>
                                  );
                                case 'type':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                      <Show when={subjectType} fallback={<span class="text-muted">—</span>}>
                                        <span
                                          class={`inline-flex min-w-[2.75rem] justify-center rounded px-1.5 py-px text-[9px] font-medium leading-none ${getRecoverySubjectTypeBadgeClass(
                                            point,
                                          )}`}
                                        >
                                          {subjectType}
                                        </span>
                                      </Show>
                                    </TableCell>
                                  );
                                case 'subject':
                                  return (
                                    <TableCell
                                      class="max-w-[420px] whitespace-nowrap px-3 py-0.5 text-base-content"
                                      title={subject}
                                    >
                                      <div class="flex min-w-0 max-w-full items-center gap-2">
                                        <span class="min-w-0 flex-1 truncate font-medium">
                                          {subject}
                                        </span>
                                        <span class="inline-flex shrink-0 items-center gap-1">
                                          <Show when={point.immutable === true}>
                                            <svg
                                              class="h-3 w-3 text-emerald-500 dark:text-emerald-400"
                                              fill="none"
                                              stroke="currentColor"
                                              viewBox="0 0 24 24"
                                              aria-hidden="true"
                                            >
                                              <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                stroke-width="2"
                                                d="M12 3l7 4v5c0 5-3.5 7.5-7 9-3.5-1.5-7-4-7-9V7l7-4z"
                                              />
                                            </svg>
                                          </Show>
                                          <Show when={point.encrypted === true}>
                                            <svg
                                              class="h-3 w-3 text-amber-500 dark:text-amber-400"
                                              fill="currentColor"
                                              viewBox="0 0 20 20"
                                              aria-hidden="true"
                                            >
                                              <path
                                                fill-rule="evenodd"
                                                d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2V7a3 3 0 016 0z"
                                                clip-rule="evenodd"
                                              />
                                            </svg>
                                          </Show>
                                        </span>
                                      </div>
                                    </TableCell>
                                  );
                                case 'entityId':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono tabular-nums">
                                      {entityId || '—'}
                                    </TableCell>
                                  );
                                case 'cluster':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                      {cluster || '—'}
                                    </TableCell>
                                  );
                                case 'nodeAgent':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                      {nodeAgent || '—'}
                                    </TableCell>
                                  );
                                case 'namespace':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-[11px] text-muted font-mono">
                                      {namespace || '—'}
                                    </TableCell>
                                  );
                                case 'source': {
                                  const badge = getSourcePlatformBadge(provider);
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                      <span
                                        class={`${badge?.classes || ''} inline-flex min-w-[3.25rem] justify-center px-1.5 py-px text-[9px] font-medium`}
                                      >
                                        {badge?.label || getSourcePlatformLabel(provider)}
                                      </span>
                                    </TableCell>
                                  );
                                }
                                case 'verified':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                      {typeof point.verified === 'boolean' ? (
                                        point.verified ? (
                                          <span
                                            class="inline-flex min-w-[1.25rem] items-center justify-center text-green-600 dark:text-green-400"
                                            title="Verified"
                                          >
                                            <svg
                                              class="h-3.5 w-3.5"
                                              fill="none"
                                              stroke="currentColor"
                                              viewBox="0 0 24 24"
                                            >
                                              <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                stroke-width="2.5"
                                                d="M5 13l4 4L19 7"
                                              />
                                            </svg>
                                          </span>
                                        ) : (
                                          <span
                                            class="inline-flex min-w-[1.25rem] items-center justify-center text-amber-500 dark:text-amber-400"
                                            title="Unverified"
                                          >
                                            <svg
                                              class="h-3.5 w-3.5"
                                              fill="none"
                                              stroke="currentColor"
                                              viewBox="0 0 24 24"
                                            >
                                              <path
                                                stroke-linecap="round"
                                                stroke-linejoin="round"
                                                stroke-width="2.5"
                                                d="M12 9v2m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"
                                              />
                                            </svg>
                                          </span>
                                        )
                                      ) : (
                                        <span class="text-muted">—</span>
                                      )}
                                    </TableCell>
                                  );
                                case 'size':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-right font-mono text-[11px] tabular-nums text-muted">
                                      {point.sizeBytes && point.sizeBytes > 0
                                        ? formatBytes(point.sizeBytes)
                                        : '—'}
                                    </TableCell>
                                  );
                                case 'method':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                      <span
                                        class={`inline-flex min-w-[3.5rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoveryArtifactModePresentation(
                                          mode as ArtifactMode,
                                        ).badgeClassName}`}
                                      >
                                        {getRecoveryPointModeLabel(point.mode)}
                                      </span>
                                    </TableCell>
                                  );
                                case 'repository':
                                  return (
                                    <TableCell
                                      class="max-w-[220px] truncate whitespace-nowrap px-3 py-0.5 text-[11px] leading-4 text-base-content"
                                      title={repoLabel}
                                    >
                                      {repoLabel || '—'}
                                    </TableCell>
                                  );
                                case 'details':
                                  return (
                                    <TableCell
                                      class="max-w-[280px] truncate whitespace-nowrap px-3 py-0.5 text-[10px] leading-4 text-muted"
                                      title={detailsSummary}
                                    >
                                      {detailsSummary || '—'}
                                    </TableCell>
                                  );
                                case 'outcome':
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-center">
                                      <span
                                        class={`inline-flex min-w-[4.5rem] justify-center rounded px-1.5 py-px text-[9px] font-medium ${getRecoveryOutcomeBadgeClass(
                                          outcome,
                                        )}`}
                                      >
                                        {titleCaseDelimitedLabel(outcome)}
                                      </span>
                                    </TableCell>
                                  );
                                default:
                                  return (
                                    <TableCell class="whitespace-nowrap px-3 py-0.5 text-muted">
                                      -
                                    </TableCell>
                                  );
                              }
                            }}
                          </For>
                        </TableRow>

                        <Show when={props.selectedPoint()?.id === point.id}>
                          <TableRow>
                            <TableCell
                              colSpan={props.tableColumnCount()}
                              class="bg-surface-alt px-0 sm:px-4 py-4 relative"
                            >
                              <div class="flex items-center justify-between px-4 pb-2 mb-2 border-b border-border">
                                <h2 class="text-sm font-semibold text-base-content">
                                  Recovery Point Details
                                </h2>
                                <button
                                  type="button"
                                  onClick={(event) => {
                                    event.stopPropagation();
                                    props.clearSelectedPoint();
                                  }}
                                  class={getRecoveryDrawerCloseButtonClass()}
                                  aria-label="Close details"
                                >
                                  <svg
                                    class="h-5 w-5"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                  >
                                    <path
                                      stroke-linecap="round"
                                      stroke-linejoin="round"
                                      stroke-width="2"
                                      d="M6 18L18 6M6 6l12 12"
                                    />
                                  </svg>
                                </button>
                              </div>
                              <div class="px-4">
                                <RecoveryPointDetails point={point} />
                              </div>
                            </TableCell>
                          </TableRow>
                        </Show>
                      </>
                    );
                  }}
                </For>
              </>
            )}
          </For>
        </TableBody>
      </Table>
    </div>

    <div class="flex items-center justify-between gap-2 px-3 py-2 text-xs text-muted border-t border-border">
      <div>
        <Show
          when={(props.recoveryPoints.meta().total || 0) > 0}
          fallback={<span>Showing 0 of 0 recovery points</span>}
        >
          <span>
            Showing {(props.recoveryPoints.meta().page - 1) * props.recoveryPoints.meta().limit + 1} -{' '}
            {Math.min(
              props.recoveryPoints.meta().page * props.recoveryPoints.meta().limit,
              props.recoveryPoints.meta().total,
            )}{' '}
            of {props.recoveryPoints.meta().total} recovery points
          </span>
        </Show>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          disabled={props.currentPage() <= 1}
          onClick={() => props.setCurrentPage(Math.max(1, props.currentPage() - 1))}
          class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
        >
          Prev
        </button>
        <span>
          Page {props.currentPage()} / {props.totalPages()}
        </span>
        <button
          type="button"
          disabled={props.currentPage() >= props.totalPages()}
          onClick={() => props.setCurrentPage(Math.min(props.totalPages(), props.currentPage() + 1))}
          class="rounded-md border border-border bg-surface px-2 py-1 text-xs font-medium text-base-content disabled:opacity-50"
        >
          Next
        </button>
      </div>
    </div>
  </Show>
);
