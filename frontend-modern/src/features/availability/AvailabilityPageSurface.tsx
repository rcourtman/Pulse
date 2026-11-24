import { Show, createEffect, createMemo, createSignal, type Component } from 'solid-js';
import { useSearchParams } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import PauseIcon from 'lucide-solid/icons/pause';
import PencilIcon from 'lucide-solid/icons/pencil';
import PlayIcon from 'lucide-solid/icons/play';
import PlusIcon from 'lucide-solid/icons/plus';
import RefreshCwIcon from 'lucide-solid/icons/refresh-cw';
import SettingsIcon from 'lucide-solid/icons/settings';
import XIcon from 'lucide-solid/icons/x';
import {
  AvailabilityTargetsAPI,
  type AvailabilityTarget,
  type AvailabilityTargetKind,
} from '@/api/availabilityTargets';
import { Button, ButtonLink } from '@/components/shared/Button';
import { Dialog } from '@/components/shared/Dialog';
import { getDrawerHeaderActionButtonClass } from '@/components/shared/buttonModel';
import { AvailabilityTargetSlot } from '@/components/Settings/ConnectionEditor/CredentialSlots/AvailabilityTargetSlot';
import {
  buildAvailabilitySettingsPath,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetMethodLabel,
} from '@/components/Settings/availabilitySettingsModel';
import {
  PlatformErrorState,
  PlatformTableLoadingState,
  normalizePlatformResourceStatusFilter,
  type PlatformResourceStatusFilter,
} from '@/features/platformPage/sharedPlatformPage';
import {
  AvailabilityChecksTable,
  readSavedAvailabilityChecksView,
  resolveAvailabilityChecksView,
  saveAvailabilityChecksView,
  type AvailabilityChecksView,
} from '@/features/standalone/AvailabilityChecksTable';
import { StandalonePostureCard } from '@/features/standalone/StandalonePageSurface';
import {
  buildStandalonePageModel,
  buildStandalonePostureSummary,
} from '@/features/standalone/standalonePageModel';
import { useUnifiedResources } from '@/hooks/useUnifiedResources';
import { AVAILABILITY_QUERY_PARAMS } from '@/routing/resourceLinks';
import { notificationStore } from '@/stores/notifications';
import { sessionCanReadAvailabilitySettings } from '@/stores/sessionSettingsCapabilities';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { buildProbeAgentOptions } from '@/utils/availabilityProbeAgents';
import { getAvailabilityTargetAddLabel } from '@/utils/availabilityTargetKinds';
import { useRelativeTimeNow } from '@/utils/relativeTimeClock';

// Fetch availability endpoints alongside agent hosts so multi-location probe
// badges can name the reporting host without hydrating the full estate.
const AVAILABILITY_RESOURCE_QUERY = 'source=agent,availability&type=agent,network-endpoint';

type AvailabilityInlineDialogState =
  | { mode: 'add'; initialTargetKind: AvailabilityTargetKind }
  | { mode: 'edit'; targetId: string; target?: AvailabilityTarget; fallbackName: string }
  | null;

const availabilityIcon = () => <ActivityIcon class="h-6 w-6 text-slate-400" />;

const availabilityMetaFor = (resource: Resource): ResourceAvailabilityMeta | undefined =>
  resource.availability ??
  (resource.platformData?.availability as ResourceAvailabilityMeta | undefined);

export const AvailabilityPageSurface: Component = () => {
  const now = useRelativeTimeNow();
  const [searchParams, setSearchParams] = useSearchParams();
  const { resources, loading, error, refetch } = useUnifiedResources({
    query: AVAILABILITY_RESOURCE_QUERY,
    cacheKey: 'availability-workspace',
    initialHydration: 'prefer-ws-then-rest',
  });

  const [initialLoadComplete, setInitialLoadComplete] = createSignal(false);
  const [savedView, setSavedView] = createSignal<AvailabilityChecksView | undefined>(
    readSavedAvailabilityChecksView(),
  );
  const [targetsById, setTargetsById] = createSignal<Map<string, AvailabilityTarget>>(new Map());
  const [dialog, setDialog] = createSignal<AvailabilityInlineDialogState>(null);
  const [pendingActionId, setPendingActionId] = createSignal<string | null>(null);
  const [testingTargetId, setTestingTargetId] = createSignal<string | null>(null);
  const [deleteConfirmingId, setDeleteConfirmingId] = createSignal<string | null>(null);
  const [deleteError, setDeleteError] = createSignal<string | null>(null);
  const [historyRefreshToken, setHistoryRefreshToken] = createSignal(0);

  const canManage = createMemo(() => sessionCanReadAvailabilitySettings());
  const model = createMemo(() => buildStandalonePageModel(resources()));
  const checks = createMemo(() => model().availabilityChecks);
  const probeAgentOptions = createMemo(() => buildProbeAgentOptions(model().machines));
  const posture = createMemo(() => buildStandalonePostureSummary(checks(), now()));

  const searchFilter = createMemo(() => {
    const value = searchParams[AVAILABILITY_QUERY_PARAMS.query];
    return typeof value === 'string' ? value : '';
  });
  const setSearchFilter = (value: string) => {
    setSearchParams({ [AVAILABILITY_QUERY_PARAMS.query]: value || null }, { replace: true });
  };
  const statusFilter = createMemo(() =>
    normalizePlatformResourceStatusFilter(searchParams[AVAILABILITY_QUERY_PARAMS.status]),
  );
  const setStatusFilter = (status: PlatformResourceStatusFilter) => {
    setSearchParams(
      { [AVAILABILITY_QUERY_PARAMS.status]: status === 'all' ? null : status },
      { replace: true },
    );
  };
  const resetFilters = () => {
    setSearchParams(
      {
        [AVAILABILITY_QUERY_PARAMS.query]: null,
        [AVAILABILITY_QUERY_PARAMS.status]: null,
      },
      { replace: true },
    );
  };
  const view = createMemo<AvailabilityChecksView>(() =>
    resolveAvailabilityChecksView(
      searchParams[AVAILABILITY_QUERY_PARAMS.view],
      checks().length,
      savedView(),
    ),
  );
  const setView = (nextView: AvailabilityChecksView) => {
    saveAvailabilityChecksView(nextView);
    setSavedView(nextView);
    setSearchParams({ [AVAILABILITY_QUERY_PARAMS.view]: nextView }, { replace: true });
  };
  const focusedCheckId = createMemo(() => {
    const value = searchParams[AVAILABILITY_QUERY_PARAMS.check];
    return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
  });

  const loadTargets = async (): Promise<Map<string, AvailabilityTarget>> => {
    if (!canManage()) return targetsById();
    try {
      const list = await AvailabilityTargetsAPI.list();
      const map = new Map(list.map((target) => [target.id, target]));
      setTargetsById(map);
      return map;
    } catch {
      return targetsById();
    }
  };

  createEffect(() => {
    if (!loading() && !initialLoadComplete()) {
      setInitialLoadComplete(true);
    }
  });

  createEffect(() => {
    if (canManage()) {
      void loadTargets();
    }
  });

  const refreshAvailabilityData = async () => {
    await Promise.all([refetch(), loadTargets()]);
    setHistoryRefreshToken((token) => token + 1);
  };

  const openAddDialog = (targetKind: AvailabilityTargetKind = 'service') => {
    setDeleteConfirmingId(null);
    setDeleteError(null);
    setDialog({ mode: 'add', initialTargetKind: targetKind });
  };

  const openEditDialog = async (resource: Resource) => {
    const targetId = availabilityMetaFor(resource)?.targetId;
    if (!targetId) return;
    setDeleteConfirmingId(null);
    setDeleteError(null);
    let target = targetsById().get(targetId);
    if (!target) {
      const refreshed = await loadTargets();
      target = refreshed.get(targetId);
    }
    setDialog({
      mode: 'edit',
      targetId,
      target,
      fallbackName: resource.name,
    });
  };

  const closeDialog = () => {
    setDialog(null);
    setDeleteConfirmingId(null);
    setDeleteError(null);
  };

  const handleSaved = () => {
    closeDialog();
    void refreshAvailabilityData();
  };

  const handleTestCheck = async (resource: Resource) => {
    const targetId = availabilityMetaFor(resource)?.targetId;
    if (!targetId) return;
    setTestingTargetId(targetId);
    try {
      const result = await AvailabilityTargetsAPI.testSaved(targetId);
      if (result.outcome === 'reachable') {
        const latency =
          typeof result.latencyMillis === 'number' && Number.isFinite(result.latencyMillis)
            ? ` (${Math.round(result.latencyMillis)} ms)`
            : '';
        notificationStore.success(`${resource.name} answered${latency}`);
      } else {
        notificationStore.warning(
          `${resource.name}: ${result.error?.trim() || 'check did not answer'}`,
        );
      }
      await refreshAvailabilityData();
    } catch (err) {
      notificationStore.error(
        err instanceof Error ? err.message : `Could not test ${resource.name}.`,
      );
    } finally {
      setTestingTargetId(null);
    }
  };

  const handleToggleEnabled = async (targetId: string, name: string, currentlyEnabled: boolean) => {
    setPendingActionId(targetId);
    try {
      await AvailabilityTargetsAPI.update(targetId, { enabled: !currentlyEnabled });
      notificationStore.success(currentlyEnabled ? `${name} paused` : `${name} resumed`);
      await refreshAvailabilityData();
      const currentDialog = dialog();
      if (currentDialog?.mode === 'edit' && currentDialog.targetId === targetId) {
        const updated = targetsById().get(targetId);
        if (updated) {
          setDialog({ mode: 'edit', targetId, target: updated, fallbackName: name });
        }
      }
    } catch (err) {
      notificationStore.error(err instanceof Error ? err.message : `Could not update ${name}.`);
    } finally {
      setPendingActionId(null);
    }
  };

  const handleRequestDelete = async (targetId: string, name: string) => {
    if (deleteConfirmingId() !== targetId) {
      setDeleteConfirmingId(targetId);
      setDeleteError(null);
      return;
    }
    setPendingActionId(targetId);
    setDeleteError(null);
    try {
      await AvailabilityTargetsAPI.remove(targetId);
      notificationStore.success(`${name} removed`);
      closeDialog();
      await refreshAvailabilityData();
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : `Could not remove ${name}.`);
    } finally {
      setPendingActionId(null);
    }
  };

  const renderDetailActions = (resource: Resource) => {
    if (!canManage()) return null;
    const targetId = availabilityMetaFor(resource)?.targetId;
    if (!targetId) return null;
    const target = () => targetsById().get(targetId);
    const enabled = () => target()?.enabled ?? true;
    const isTesting = () => testingTargetId() === targetId;
    const isPending = () => pendingActionId() === targetId;
    const actionClass = getDrawerHeaderActionButtonClass();

    return (
      <>
        <button
          type="button"
          class={actionClass}
          disabled={isTesting() || isPending()}
          onClick={() => void handleTestCheck(resource)}
          aria-label={isTesting() ? `Testing ${resource.name}` : `Test ${resource.name} now`}
          title={isTesting() ? 'Testing check' : 'Test now'}
          data-testid="availability-drawer-test-button"
        >
          <RefreshCwIcon
            class={`h-3.5 w-3.5 ${isTesting() ? 'animate-spin' : ''}`}
            aria-hidden="true"
          />
          <span class="hidden sm:inline">{isTesting() ? 'Testing' : 'Test now'}</span>
        </button>
        <button
          type="button"
          class={actionClass}
          disabled={isPending() || isTesting()}
          onClick={() => void handleToggleEnabled(targetId, resource.name, enabled())}
          aria-label={enabled() ? `Pause ${resource.name}` : `Resume ${resource.name}`}
          title={enabled() ? 'Pause check' : 'Resume check'}
          data-testid="availability-drawer-toggle-button"
        >
          <Show when={enabled()} fallback={<PlayIcon class="h-3.5 w-3.5" aria-hidden="true" />}>
            <PauseIcon class="h-3.5 w-3.5" aria-hidden="true" />
          </Show>
          <span class="hidden sm:inline">{enabled() ? 'Pause' : 'Resume'}</span>
        </button>
        <button
          type="button"
          class={actionClass}
          disabled={isPending()}
          onClick={() => void openEditDialog(resource)}
          aria-label={`Edit ${resource.name}`}
          title="Edit check"
          data-testid="availability-drawer-edit-button"
        >
          <PencilIcon class="h-3.5 w-3.5" aria-hidden="true" />
          <span class="hidden sm:inline">Edit</span>
        </button>
      </>
    );
  };

  const dialogTitle = createMemo(() => {
    const current = dialog();
    if (!current) return '';
    if (current.mode === 'edit') {
      return `Edit ${current.target?.name ?? current.fallbackName}`;
    }
    return getAvailabilityTargetAddLabel(current.initialTargetKind);
  });

  const dialogDescription = createMemo(() => {
    const current = dialog();
    if (!current) return '';
    if (current.mode === 'edit' && current.target) {
      return `${getAvailabilityTargetMethodLabel(current.target)} · ${getAvailabilityTargetAddressLabel(current.target)}`;
    }
    return 'Monitor websites, APIs, routers, and devices with ICMP ping, TCP, UDP, HTTP/HTTPS, and TLS certificate checks.';
  });

  const showLoading = createMemo(
    () => loading() && !initialLoadComplete() && checks().length === 0,
  );

  return (
    <div data-testid="availability-page" class="pulse-wide-data-surface space-y-3">
      <Show
        when={!showLoading()}
        fallback={
          <PlatformTableLoadingState
            title="Loading availability checks"
            description="Pulse is loading availability checks and recent state history."
          />
        }
      >
        <Show
          when={!error()}
          fallback={
            <PlatformErrorState
              title="Could not load availability checks"
              description="Refresh the resource snapshot or check the API connection state."
              onRefresh={() => void refetch()}
            />
          }
        >
          <div class="space-y-3">
            <Show when={checks().length > 0}>
              <StandalonePostureCard
                label="Availability"
                noun="check"
                summary={posture()}
                actions={
                  <Show when={canManage()}>
                    <>
                      <Button
                        type="button"
                        variant="primary"
                        size="sm"
                        class="gap-1.5"
                        onClick={() => openAddDialog('service')}
                        data-testid="availability-posture-add-button"
                      >
                        <PlusIcon class="h-3.5 w-3.5" />
                        Add check
                      </Button>
                      <ButtonLink
                        href={buildAvailabilitySettingsPath()}
                        variant="secondary"
                        size="sm"
                        class="gap-1.5"
                      >
                        <SettingsIcon class="h-3.5 w-3.5" />
                        Settings
                      </ButtonLink>
                    </>
                  </Show>
                }
              />
            </Show>

            <AvailabilityChecksTable
              resources={checks()}
              probeAgentOptions={probeAgentOptions()}
              view={view()}
              onViewChange={setView}
              focusCheckId={focusedCheckId()}
              externalSearch={searchFilter}
              onExternalSearchChange={setSearchFilter}
              externalStatus={statusFilter}
              onExternalStatusChange={setStatusFilter}
              onResetFilters={resetFilters}
              canManage={canManage()}
              hideHeaderAddCheck={true}
              onAddCheck={canManage() ? () => openAddDialog('service') : undefined}
              renderDetailActions={renderDetailActions}
              historyRefreshToken={historyRefreshToken()}
              emptyIcon={availabilityIcon()}
              emptyTitle="No availability checks yet"
              emptyDescription="Add ICMP ping, TCP, UDP, or HTTP/HTTPS checks (with automatic TLS certificate expiry monitoring) to track uptime for websites, services, routers, and devices."
            />
          </div>
        </Show>
      </Show>

      <Show when={dialog()}>
        {(current) => {
          const editTarget = () =>
            current().mode === 'edit'
              ? (current() as { target?: AvailabilityTarget }).target
              : undefined;
          const editId = () =>
            current().mode === 'edit' ? (current() as { targetId: string }).targetId : null;
          const editName = () =>
            current().mode === 'edit'
              ? (editTarget()?.name ?? (current() as { fallbackName: string }).fallbackName)
              : '';

          return (
            <Dialog
              isOpen={true}
              onClose={closeDialog}
              panelClass="max-w-4xl"
              ariaLabel={dialogTitle()}
            >
              <div class="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
                <div class="min-w-0">
                  <h2 class="text-base font-semibold text-base-content">{dialogTitle()}</h2>
                  <p class="mt-0.5 text-xs text-muted">{dialogDescription()}</p>
                </div>
                <button
                  type="button"
                  onClick={closeDialog}
                  class="rounded-md p-1 text-muted transition-colors hover:bg-surface-hover hover:text-base-content"
                  aria-label="Close availability check dialog"
                >
                  <XIcon class="h-4 w-4" />
                </button>
              </div>
              <div class="flex-1 overflow-y-auto px-5 py-4">
                <AvailabilityTargetSlot
                  editingTargetId={editId()}
                  initialTargetKind={
                    current().mode === 'add'
                      ? (current() as { initialTargetKind: AvailabilityTargetKind })
                          .initialTargetKind
                      : undefined
                  }
                  onCancel={closeDialog}
                  onSaved={handleSaved}
                  connectionEnabled={editTarget()?.enabled ?? true}
                  togglePending={editId() !== null && pendingActionId() === editId()}
                  onToggleEnabled={
                    editId()
                      ? () =>
                          void handleToggleEnabled(
                            editId()!,
                            editName(),
                            editTarget()?.enabled ?? true,
                          )
                      : undefined
                  }
                  deletePending={editId() !== null && pendingActionId() === editId()}
                  deleteConfirming={editId() !== null && deleteConfirmingId() === editId()}
                  deleteError={deleteError()}
                  onDelete={
                    editId() ? () => void handleRequestDelete(editId()!, editName()) : undefined
                  }
                />
              </div>
            </Dialog>
          );
        }}
      </Show>
    </div>
  );
};

export default AvailabilityPageSurface;
