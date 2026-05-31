import {
  Show,
  createMemo,
  createResource,
  createSignal,
  type Component,
  type JSX,
} from 'solid-js';
import ChevronRightIcon from 'lucide-solid/icons/chevron-right';
import { Card } from '@/components/shared/Card';
import { EmptyState } from '@/components/shared/EmptyState';
import { useSearchParams } from '@solidjs/router';
import { FilterBar, type FilterDef, type FilterSelectOption } from '@/components/shared/FilterBar';
import { useBreakpoint } from '@/hooks/useBreakpoint';
import { apiFetch } from '@/utils/apiClient';
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
import {
  buildProxmoxBackupRecoveryModel,
  coverageRowMatchesSearch,
  isCoverageAttention,
} from './proxmoxBackupRecoveryModel';
import {
  COVERAGE_SORT_DEFAULT_DIRECTION,
  cmpNumber,
  cmpString,
  type CoverageFilterValue,
  type CoverageSortKey,
} from './proxmoxBackupsTableModel';
import { COVERAGE_FILTERS } from './proxmoxBackupsTableShared';
import { ProxmoxBackupsCoverageStrip } from './ProxmoxBackupsCoverageStrip';
import { ProxmoxBackupServersTable } from './ProxmoxBackupServersTable';
import { ProxmoxCoverageTable } from './ProxmoxCoverageTable';

// The Proxmox backups surface answers one operator question: "is every guest
// backed up, recently, and did the last job work?" That question IS the whole
// surface — one row per workload, a posture dot, the latest restore point, and
// the per-source evidence (PBS snapshots, PVE archives, guest snapshots) tucked
// into each row's expansion for anyone who wants to drill in.
//
// The earlier per-source browsers (PBS artifacts / Snapshots / Backup files),
// the "Restore points" and "Job history" tabs, and the per-day activity charts
// were removed deliberately. A monitor is not a console: the live PBS and PVE
// web UIs already own the forensic per-artifact view, and reproducing it
// read-only here only buried the one question a backup monitor exists to
// answer. Nothing was deleted that a power user needs — the evidence lives in
// the row expansion (demote, not delete).

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
  // Search and the posture facet stay ephemeral signals.
  const [searchParams, setSearchParams] = useSearchParams();
  const [search, setSearch] = createSignal('');
  const [coverageFilter, setCoverageFilter] = createSignal<CoverageFilterValue>('all');

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

  const handleCoverageSort = (key: CoverageSortKey) => {
    if (coverageSortKey() === key) {
      setCoverageSortDirection(coverageSortDirection() === 'asc' ? 'desc' : 'asc');
    } else {
      setCoverageSortKey(key);
      setCoverageSortDirection(COVERAGE_SORT_DEFAULT_DIRECTION[key]);
    }
  };
  const toggleCoverageExpansion = (key: string) =>
    setExpandedCoverageRows((current) => {
      const next = new Set(current);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

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
      ...[...nodes].sort((a, b) => a.localeCompare(b)).map((node) => ({ value: node, label: node })),
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

  // FilterBar catalog: Node + Type scope filters (in the "+ Filter" menu) and
  // the posture facet as an inline segmented control, matching the overview /
  // workloads / storage pages.
  const buildBackupsFilters = (): FilterDef[] => [
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
    {
      id: 'posture',
      label: 'Posture',
      group: 'status',
      inline: true,
      options: () => COVERAGE_FILTERS,
      value: coverageFilter,
      setValue: (v) => setCoverageFilter(v as CoverageFilterValue),
      defaultValue: 'all',
    },
  ];

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
            <ProxmoxBackupServersTable servers={props.servers ?? []} />
          </Show>

          <ProxmoxBackupsCoverageStrip
            title="Backup health"
            tail={
              <span>
                {liveTotalCount()} guests ·{' '}
                {recoveryModel().coverageSummary.recoverableArtifacts} restore points
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

          <FilterBar
            role="group"
            ariaLabel="Backups filters"
            isMobile={isMobile}
            search={{
              value: search,
              setValue: setSearch,
              placeholder: 'Search backups by workload, node, or status',
            }}
            filters={buildBackupsFilters()}
            savedViewsKey="proxmox-backups"
            viewOptionsTrailing={
              <span class="whitespace-nowrap text-xs font-medium text-muted">
                <Show
                  when={visibleLiveCount() !== liveTotalCount()}
                  fallback={<>{liveTotalCount()} guests</>}
                >
                  {visibleLiveCount()} of {liveTotalCount()} guests
                </Show>
              </span>
            }
          />

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
                <span class="text-[11px] text-muted">
                  guest no longer exists in inventory
                </span>
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
        </div>
      </Show>
    </Show>
  );
};

export default ProxmoxBackupsTable;
