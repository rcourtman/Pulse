import { Show, createMemo, createResource, createSignal, type Component, type JSX } from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import CalendarIcon from 'lucide-solid/icons/calendar';
import ShieldCheckIcon from 'lucide-solid/icons/shield-check';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { useSearchParams } from '@solidjs/router';
import { FilterBar, type FilterDef, type FilterSelectOption } from '@/components/shared/FilterBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { apiFetch } from '@/utils/apiClient';
import {
  getRecoveryFilterDateLabel,
  recoveryDateKeyFromTimestamp,
} from '@/utils/recoveryDatePresentation';
import type {
  BackupTask,
  GuestSnapshot,
  PBSBackup,
  PBSBackupsPayload,
  PBSBackupsResponse,
  PVEBackupsPayload,
  PVEBackupsResponse,
  StorageBackup,
} from '@/types/api';
import type { Resource } from '@/types/resource';
import { BackupActivityChart } from './BackupActivityChart';
import {
  buildBackupActivityTimeline,
  type BackupActivityMetricMode,
  type BackupActivityRangeDays,
  type BackupActivitySegmentKind,
} from './proxmoxBackupActivityPresentation';
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  isCoverageAttention,
  recoverableArtifactMatchesSearch,
  type RecoverableArtifact,
} from './proxmoxBackupRecoveryModel';
import {
  COVERAGE_SORT_DEFAULT_DIRECTION,
  RECOVERABLE_SORT_DEFAULT_DIRECTION,
  cmpNumber,
  cmpString,
  type CoverageFilterValue,
  type CoverageSortKey,
  type RecoverableFilterValue,
  type RecoverableSortKey,
} from './proxmoxBackupsTableModel';
import {
  COVERAGE_FILTERS,
  RECOVERABLE_FILTERS,
  artifactStateLabel,
} from './proxmoxBackupsTableShared';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';
import { ProxmoxBackupServersTable } from './ProxmoxBackupServersTable';
import { ProxmoxCoverageTable } from './ProxmoxCoverageTable';
import { ProxmoxRecoverableTable } from './ProxmoxRecoverableTable';

// One backups surface, two operator views: a chronological recoverable-artifact
// feed for "what ran when", and a guest coverage table for "what is protected".

type BackupView = 'date' | 'guest';

async function fetchPVEBackups(): Promise<PVEBackupsPayload> {
  const response = await apiFetch('/api/backups/pve');
  if (!response.ok) {
    throw new Error(`Failed to load PVE backups (${response.status})`);
  }
  const payload = (await response.json()) as PVEBackupsResponse;
  return (
    payload?.data ?? {
      backupTasks: [],
      storageBackups: [],
      guestSnapshots: [],
    }
  );
}

async function fetchPBSBackups(): Promise<PBSBackupsPayload> {
  const response = await apiFetch('/api/backups/pbs');
  if (!response.ok) {
    throw new Error(`Failed to load PBS backups (${response.status})`);
  }
  const payload = (await response.json()) as PBSBackupsResponse;
  return payload?.data ?? { backups: [] };
}

export const ProxmoxBackupsTable: Component<{
  emptyIcon: JSX.Element;
  workloads?: readonly Resource[];
  servers?: readonly Resource[];
}> = (props) => {
  const [backups, { refetch }] = createResource<PVEBackupsPayload>(fetchPVEBackups);
  const [pbsBackups] = createResource<PBSBackupsPayload>(fetchPBSBackups);
  const { isMobile } = useBreakpoint();

  // Structured scope filters (node, type) live in the URL so the view is
  // shareable, survives reload, and can be captured by FilterBar saved views.
  // Search, the view toggle, and status facets stay ephemeral signals.
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = createSignal('');
  const [view, setView] = createSignal<BackupView>('date');
  const [coverageFilter, setCoverageFilter] = createSignal<CoverageFilterValue>('all');
  const [recoverableFilter, setRecoverableFilter] = createSignal<RecoverableFilterValue>('all');

  const nodeFilter = (): string => (typeof searchParams.node === 'string' ? searchParams.node : '');
  const setNodeFilter = (value: string): void => setSearchParams({ node: value || null });
  const nodeMatches = (node: string | undefined): boolean => {
    const selected = nodeFilter();
    return !selected || node === selected;
  };

  // Type scope normalizes vm/qemu and ct/lxc to a canonical token before
  // comparing, since the recovery model uses vm/ct/host.
  const typeFilter = (): string => (typeof searchParams.type === 'string' ? searchParams.type : '');
  const setTypeFilter = (value: string): void => setSearchParams({ type: value || null });
  const canonicalGuestType = (raw: string | undefined): string => {
    const t = (raw ?? '').toLowerCase();
    if (t === 'vm' || t === 'qemu') return 'vm';
    if (t === 'ct' || t === 'lxc') return 'ct';
    if (t === 'host') return 'host';
    return 'other';
  };
  const typeMatches = (raw: string | undefined): boolean => {
    const selected = typeFilter();
    return !selected || canonicalGuestType(raw) === selected;
  };
  const typeFilterOptions: FilterSelectOption[] = [
    { value: '', label: 'All types' },
    { value: 'vm', label: 'VMs' },
    { value: 'ct', label: 'Containers' },
    { value: 'host', label: 'Hosts' },
  ];

  const [coverageSortKey, setCoverageSortKey] = createSignal<CoverageSortKey>('posture');
  const [coverageSortDirection, setCoverageSortDirection] = createSignal<'asc' | 'desc'>('asc');
  const [expandedCoverageRows, setExpandedCoverageRows] = createSignal<ReadonlySet<string>>(
    new Set<string>(),
  );
  // Orphaned backups (records whose guest no longer exists in inventory) are
  // collapsed by default so the main table is the user's live guests, not a
  // pile of nameless dead records sorted to the top.
  const [showOrphaned, setShowOrphaned] = createSignal(false);

  // Chronological feed sort and activity-chart controls.
  const [recoverableSortKey, setRecoverableSortKey] = createSignal<RecoverableSortKey>('created');
  const [recoverableSortDirection, setRecoverableSortDirection] = createSignal<'asc' | 'desc'>(
    'desc',
  );
  const [chartRange, setChartRange] = createSignal<BackupActivityRangeDays>(30);
  const [selectedDateKey, setSelectedDateKey] = createSignal<string | null>(null);
  const [recoverableMetricMode, setRecoverableMetricMode] =
    createSignal<BackupActivityMetricMode>('count');

  const handleCoverageSort = (key: CoverageSortKey) => {
    if (coverageSortKey() === key) {
      setCoverageSortDirection(coverageSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setCoverageSortKey(key);
      setCoverageSortDirection(COVERAGE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const handleRecoverableSort = (key: RecoverableSortKey) => {
    if (recoverableSortKey() === key) {
      setRecoverableSortDirection(recoverableSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setRecoverableSortKey(key);
      setRecoverableSortDirection(RECOVERABLE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const toggleCoverageExpansion = (key: string) =>
    setExpandedCoverageRows((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const toggleDay = (key: string) =>
    setSelectedDateKey((current) => (current === key ? null : key));

  const pbsArtifacts = createMemo<PBSBackup[]>(() => pbsBackups()?.backups ?? []);
  const snapshots = createMemo<GuestSnapshot[]>(() => backups()?.guestSnapshots ?? []);
  const archives = createMemo<StorageBackup[]>(() => backups()?.storageBackups ?? []);
  const tasks = createMemo<BackupTask[]>(() => backups()?.backupTasks ?? []);

  // Render-time `now` snapshot so all age comparisons within a render share a
  // reference moment; not reactive to ticking time (fine for sysadmin grouping).
  const nowMs = createMemo(() => Date.now());

  const recoveryModel = createMemo(() =>
    buildProxmoxBackupRecoveryModel({
      workloads: props.workloads ?? [],
      pbsBackups: pbsArtifacts(),
      archives: archives(),
      snapshots: snapshots(),
      tasks: tasks(),
      nowMs: nowMs(),
    }),
  );

  // Node options for the scope filter: the distinct nodes across the workload
  // set, "All nodes" first. PBS artifacts carry no node, so this scopes the
  // node-bearing rows only.
  const nodeOptions = createMemo<FilterSelectOption[]>(() => {
    const nodes = new Set<string>();
    for (const row of recoveryModel().coverageRows) {
      const node = row.workload.node?.trim();
      if (node) nodes.add(node);
    }
    return [
      { value: '', label: 'All nodes' },
      ...[...nodes]
        .sort((a, b) => a.localeCompare(b))
        .map((node) => ({ value: node, label: node })),
    ];
  });

  // Source columns auto-hide when no workload anywhere has that data (computed
  // over the full set so columns don't flicker as filters change). A PBS-only
  // fleet drops the Archive and Snapshot columns.
  const coverageHasPbs = createMemo(() =>
    recoveryModel().coverageRows.some((row) => Boolean(row.latestPBS) || row.pbsCount > 0),
  );
  const coverageHasArchive = createMemo(() =>
    recoveryModel().coverageRows.some((row) => Boolean(row.latestArchive) || row.archiveCount > 0),
  );
  const coverageHasSnapshot = createMemo(() =>
    recoveryModel().coverageRows.some(
      (row) => Boolean(row.latestSnapshot) || row.snapshotCount > 0,
    ),
  );
  const coverageHasTask = createMemo(() =>
    recoveryModel().coverageRows.some((row) => Boolean(row.latestTask)),
  );

  const filteredCoverageRows = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = coverageFilter();
    const list = recoveryModel().coverageRows.filter((row) => {
      if (!nodeMatches(row.workload.node)) return false;
      if (!typeMatches(row.workload.type)) return false;
      if (filter === 'attention' && !isCoverageAttention(row.posture)) return false;
      if (filter === 'current' && row.posture !== 'current') return false;
      if (filter === 'uncovered' && row.posture !== 'uncovered') return false;
      return coverageRowMatchesSearch(row, term);
    });
    const sortKey = coverageSortKey();
    const direction = coverageSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'posture':
          return cmpNumber(a.postureRank, b.postureRank, direction);
        case 'workload':
          return cmpString(a.workload.label, b.workload.label, direction);
        case 'latest':
          return cmpNumber(a.latestRecovery?.createdMs, b.latestRecovery?.createdMs, direction);
        case 'pbs':
          return cmpNumber(a.latestPBS?.createdMs, b.latestPBS?.createdMs, direction);
        case 'archive':
          return cmpNumber(a.latestArchive?.createdMs, b.latestArchive?.createdMs, direction);
        case 'snapshot':
          return cmpNumber(a.latestSnapshot?.createdMs, b.latestSnapshot?.createdMs, direction);
        case 'task':
          return cmpNumber(a.latestTask?.startedMs, b.latestTask?.startedMs, direction);
      }
    });
    return list;
  });

  // The main table is the user's live inventory guests; orphaned backup records
  // (guest no longer exists) are partitioned out into their own collapsed
  // section so they don't dominate the top of the list with nameless rows.
  const liveCoverageRows = createMemo(() =>
    filteredCoverageRows().filter((row) => !row.isOrphaned),
  );
  const orphanedCoverageRows = createMemo(() =>
    filteredCoverageRows().filter((row) => row.isOrphaned),
  );
  const liveTotalCount = createMemo(
    () => recoveryModel().coverageRows.filter((row) => !row.isOrphaned).length,
  );
  const visibleLiveCount = createMemo(() => liveCoverageRows().length);
  const orphanedTotalCount = createMemo(
    () => recoveryModel().coverageRows.filter((row) => row.isOrphaned).length,
  );

  // Health strip counts live guests only — orphaned records are all stale and
  // would otherwise inflate the "attention" segment with dead data.
  const liveHealthSummary = createMemo(() => {
    const live = recoveryModel().coverageRows.filter((row) => !row.isOrphaned);
    return {
      current: live.filter((row) => row.posture === 'current').length,
      attention: live.filter((row) => isCoverageAttention(row.posture)).length,
      uncovered: live.filter((row) => row.posture === 'uncovered').length,
    };
  });

  // Recoverable artifact feed: PBS, PVE archive, and guest snapshot rows.
  const RECOVERABLE_SEGMENT_KINDS: readonly BackupActivitySegmentKind[] = [
    'pbs',
    'archive',
    'snapshot',
  ];

  const recoverableTimeline = createMemo(() =>
    buildBackupActivityTimeline<RecoverableArtifact>(
      chartRange(),
      recoveryModel().recoverableArtifacts,
      (artifact) => artifact.createdMs,
      (artifact) =>
        artifact.sourceKind === 'pbs'
          ? 'pbs'
          : artifact.sourceKind === 'archive'
            ? 'archive'
            : 'snapshot',
      {
        getValue:
          recoverableMetricMode() === 'volume'
            ? (artifact) => (artifact.size && artifact.size > 0 ? artifact.size : 0)
            : undefined,
      },
    ),
  );

  const filteredRecoverableArtifacts = createMemo(() => {
    const term = search().trim().toLowerCase();
    const filter = recoverableFilter();
    const dateKey = selectedDateKey();
    const list = recoveryModel().recoverableArtifacts.filter((artifact) => {
      if (!nodeMatches(artifact.workload.node)) return false;
      if (!typeMatches(artifact.workload.type)) return false;
      if (
        (filter === 'pbs' || filter === 'archive' || filter === 'snapshot') &&
        artifact.sourceKind !== filter
      ) {
        return false;
      }
      if (filter === 'verified' && artifact.verified !== true) return false;
      if (filter === 'unverified' && artifact.verified !== false) return false;
      if (
        dateKey &&
        (artifact.createdMs === undefined ||
          recoveryDateKeyFromTimestamp(artifact.createdMs) !== dateKey)
      ) {
        return false;
      }
      return recoverableArtifactMatchesSearch(artifact, term);
    });
    const sortKey = recoverableSortKey();
    const direction = recoverableSortDirection();
    list.sort((a, b) => {
      switch (sortKey) {
        case 'workload':
          return cmpString(a.workload.label, b.workload.label, direction);
        case 'source':
          return cmpString(a.sourceLabel, b.sourceLabel, direction);
        case 'location':
          return cmpString(a.location, b.location, direction);
        case 'created':
          return cmpNumber(a.createdMs, b.createdMs, direction);
        case 'size':
          return cmpNumber(a.size, b.size, direction);
        case 'state':
          return cmpString(artifactStateLabel(a), artifactStateLabel(b), direction);
      }
    });
    return list;
  });

  const recoverableSizeMaxBytes = createMemo(() => {
    let max = 0;
    for (const artifact of filteredRecoverableArtifacts()) {
      if (artifact.size && artifact.size > max) max = artifact.size;
    }
    return max;
  });

  const totalRecoverableCount = createMemo(() => recoveryModel().recoverableArtifacts.length);
  const visibleRecoverableCount = createMemo(() => filteredRecoverableArtifacts().length);

  // Shared scope filters plus the per-view inline facet.
  const buildBackupsFilters = (): FilterDef[] => {
    const filters: FilterDef[] = [
      {
        id: 'node',
        label: 'Node',
        group: 'scope',
        options: nodeOptions,
        value: nodeFilter,
        setValue: setNodeFilter,
        defaultValue: '',
      },
      {
        id: 'type',
        label: 'Type',
        group: 'scope',
        options: () => typeFilterOptions,
        value: typeFilter,
        setValue: setTypeFilter,
        defaultValue: '',
      },
    ];
    if (view() === 'date') {
      filters.push({
        id: 'source',
        label: 'Source',
        group: 'status',
        inline: true,
        options: () => RECOVERABLE_FILTERS,
        value: recoverableFilter,
        setValue: (v) => setRecoverableFilter(v as RecoverableFilterValue),
        defaultValue: 'all',
      });
    } else {
      filters.push({
        id: 'posture',
        label: 'Posture',
        group: 'status',
        inline: true,
        options: () => COVERAGE_FILTERS,
        value: coverageFilter,
        setValue: (v) => setCoverageFilter(v as CoverageFilterValue),
        defaultValue: 'all',
      });
    }
    return filters;
  };

  const viewButtonClass = (active: boolean): string =>
    `inline-flex min-h-8 items-center gap-1.5 rounded-sm px-3 text-xs font-medium transition-colors ${
      active ? 'bg-surface-hover text-base-content' : 'text-muted hover:text-base-content'
    }`;

  return (
    <Show
      when={!backups.error}
      fallback={
        <Card padding="lg">
          <EmptyState
            icon={props.emptyIcon}
            title="Could not load Proxmox backup inventory"
            description={(backups.error as Error | undefined)?.message ?? 'Refresh to retry.'}
            actions={
              <button
                type="button"
                onClick={() => void refetch()}
                class="inline-flex min-h-10 items-center rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-surface-hover"
              >
                Refresh
              </button>
            }
          />
        </Card>
      }
    >
      <Show
        when={backups() !== undefined}
        fallback={
          <Card padding="lg">
            <EmptyState
              icon={props.emptyIcon}
              title="Loading Proxmox backup inventory"
              description="Reading PBS artifacts, PVE snapshots, archives and recent backup tasks."
            />
          </Card>
        }
      >
        <div class="space-y-3">
          <Show when={(props.servers?.length ?? 0) > 0}>
            <ProxmoxBackupServersTable servers={props.servers ?? []} backups={pbsArtifacts()} />
          </Show>

          <Show
            when={view() === 'guest'}
            fallback={
              <div class="flex flex-wrap items-center gap-x-2 gap-y-1 px-1 text-[11px] text-muted">
                <span class="font-semibold uppercase tracking-[0.18em]">Backup health</span>
                <span class="text-base-content tabular-nums">{liveTotalCount()} guests</span>
                <span aria-hidden="true">·</span>
                <span class="tabular-nums">
                  {recoveryModel().coverageSummary.recoverableArtifacts} restore points
                </span>
                <Show when={liveHealthSummary().attention > 0}>
                  <span aria-hidden="true">·</span>
                  <span class="text-amber-600 tabular-nums dark:text-amber-300">
                    {liveHealthSummary().attention} need attention
                  </span>
                </Show>
                <Show when={orphanedTotalCount() > 0}>
                  <span aria-hidden="true">·</span>
                  <span class="tabular-nums">{orphanedTotalCount()} orphaned</span>
                </Show>
              </div>
            }
          >
            <ProxmoxBackupsCoverageStrip
              title="Backup health"
              tail={
                <span>
                  {liveTotalCount()} guests · {recoveryModel().coverageSummary.recoverableArtifacts}{' '}
                  restore points
                  <Show when={recoveryModel().coverageSummary.withPBS > 0}>
                    {' · '}
                    {recoveryModel().coverageSummary.withPBS} with PBS
                  </Show>
                  <Show when={orphanedTotalCount() > 0}>
                    {' · '}
                    {orphanedTotalCount()} orphaned
                  </Show>
                </span>
              }
              segments={[
                {
                  key: 'current',
                  value: liveHealthSummary().current,
                  label: 'current',
                  toneClass: 'bg-emerald-500',
                },
                {
                  key: 'attention',
                  value: liveHealthSummary().attention,
                  label: 'attention',
                  toneClass: 'bg-amber-500',
                  muted: liveHealthSummary().attention === 0,
                },
                {
                  key: 'uncovered',
                  value: liveHealthSummary().uncovered,
                  label: 'uncovered',
                  toneClass: 'bg-red-500',
                  muted: liveHealthSummary().uncovered === 0,
                },
              ]}
            />
          </Show>

          <div
            role="group"
            aria-label="Backups view"
            class="inline-flex items-center gap-1 rounded-md border border-border bg-surface p-1"
          >
            <button
              type="button"
              class={viewButtonClass(view() === 'date')}
              aria-pressed={view() === 'date'}
              onClick={() => setView('date')}
            >
              <CalendarIcon class="h-4 w-4" aria-hidden="true" />
              <span>By date</span>
            </button>
            <button
              type="button"
              class={viewButtonClass(view() === 'guest')}
              aria-pressed={view() === 'guest'}
              onClick={() => setView('guest')}
            >
              <ShieldCheckIcon class="h-4 w-4" aria-hidden="true" />
              <span>By guest</span>
            </button>
          </div>

          <Show when={view() === 'date' && recoveryModel().recoverableArtifacts.length > 0}>
            <BackupActivityChart
              title={
                recoverableMetricMode() === 'volume' ? 'Backup volume per day' : 'Backups per day'
              }
              noun="artifact"
              segmentKinds={RECOVERABLE_SEGMENT_KINDS}
              range={chartRange}
              onRangeChange={setChartRange}
              timeline={recoverableTimeline}
              selectedDateKey={selectedDateKey}
              onToggleDay={toggleDay}
              metricToggle={{ mode: recoverableMetricMode, onChange: setRecoverableMetricMode }}
            />
          </Show>

          <FilterBar
            role="group"
            ariaLabel="Backups filters"
            isMobile={isMobile}
            search={{
              value: search,
              setValue: setSearch,
              placeholder: 'Search backups by workload, node, source, or status',
            }}
            filters={buildBackupsFilters()}
            savedViewsKey="proxmox-backups"
            searchTrailing={
              <Show when={view() === 'date' && selectedDateKey() !== null}>
                <button
                  type="button"
                  onClick={() => setSelectedDateKey(null)}
                  class="inline-flex items-center gap-1 rounded-full border border-blue-300 bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 transition-colors hover:bg-blue-100 dark:border-blue-700 dark:bg-blue-900/30 dark:text-blue-200 dark:hover:bg-blue-900/50"
                  aria-label="Clear date filter"
                >
                  <span class="uppercase tracking-wide text-[9px] text-blue-600 dark:text-blue-300">
                    Date
                  </span>
                  <span class="font-mono tabular-nums">
                    {getRecoveryFilterDateLabel(selectedDateKey()!)}
                  </span>
                  <span aria-hidden="true">×</span>
                </button>
              </Show>
            }
            viewOptionsTrailing={
              <span class="whitespace-nowrap text-xs font-medium text-muted">
                <Show
                  when={view() === 'date'}
                  fallback={
                    <Show
                      when={visibleLiveCount() !== liveTotalCount()}
                      fallback={<>{liveTotalCount()} guests</>}
                    >
                      {visibleLiveCount()} of {liveTotalCount()} guests
                    </Show>
                  }
                >
                  <Show
                    when={visibleRecoverableCount() !== totalRecoverableCount()}
                    fallback={<>{totalRecoverableCount()} backups</>}
                  >
                    {visibleRecoverableCount()} of {totalRecoverableCount()} backups
                  </Show>
                </Show>
              </span>
            }
          />

          <Show when={view() === 'date'}>
            <ProxmoxRecoverableTable
              artifacts={filteredRecoverableArtifacts()}
              hasAnyArtifacts={recoveryModel().recoverableArtifacts.length > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle="No backups yet"
              emptyDescription="PBS snapshots, PVE backup files, and guest snapshots will appear here once backups run."
              sortKey={recoverableSortKey}
              sortDirection={recoverableSortDirection}
              onSort={handleRecoverableSort}
              sizeMaxBytes={recoverableSizeMaxBytes()}
              groupByDay
            />
          </Show>

          <Show when={view() === 'guest'}>
            <ProxmoxCoverageTable
              rows={liveCoverageRows()}
              hasAnyRows={liveTotalCount() > 0}
              emptyIcon={props.emptyIcon}
              emptyTitle="No workload coverage"
              emptyDescription="VM and container restore posture will appear here once backup data exists."
              sortKey={coverageSortKey}
              sortDirection={coverageSortDirection}
              onSort={handleCoverageSort}
              expandedKeys={expandedCoverageRows()}
              onToggleExpand={toggleCoverageExpansion}
              showPbsColumn={coverageHasPbs()}
              showArchiveColumn={coverageHasArchive()}
              showSnapshotColumn={coverageHasSnapshot()}
              showTaskColumn={coverageHasTask()}
            />

            <Show when={orphanedTotalCount() > 0}>
              <div class="rounded-lg border border-border-subtle bg-surface-alt/25">
                <button
                  type="button"
                  onClick={() => setShowOrphaned((v) => !v)}
                  class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left"
                  aria-expanded={showOrphaned()}
                >
                  <span class="flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.18em] text-muted">
                    <ChevronRightIcon
                      class={`h-3.5 w-3.5 transition-transform ${showOrphaned() ? 'rotate-90' : ''}`}
                      aria-hidden="true"
                    />
                    {orphanedTotalCount()} orphaned{' '}
                    {orphanedTotalCount() === 1 ? 'backup' : 'backups'}
                  </span>
                  <span class="text-[11px] text-muted">guest no longer exists in inventory</span>
                </button>
                <Show when={showOrphaned()}>
                  <div class="border-t border-border-subtle p-2">
                    <ProxmoxCoverageTable
                      rows={orphanedCoverageRows()}
                      hasAnyRows={orphanedTotalCount() > 0}
                      emptyIcon={props.emptyIcon}
                      emptyTitle="No orphaned backups"
                      emptyDescription="Backups whose guest no longer exists will appear here."
                      sortKey={coverageSortKey}
                      sortDirection={coverageSortDirection}
                      onSort={handleCoverageSort}
                      expandedKeys={expandedCoverageRows()}
                      onToggleExpand={toggleCoverageExpansion}
                      showPbsColumn={coverageHasPbs()}
                      showArchiveColumn={coverageHasArchive()}
                      showSnapshotColumn={coverageHasSnapshot()}
                      showTaskColumn={coverageHasTask()}
                    />
                  </div>
                </Show>
              </div>
            </Show>
          </Show>
        </div>
      </Show>
    </Show>
  );
};

export default ProxmoxBackupsTable;
