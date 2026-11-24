import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onMount,
  type Component,
} from 'solid-js';
import type { Resource } from '@/types/resource';
import { isAgentlessAvailabilityResource } from '@/features/standalone/standalonePageModel';
import { useLocation, useNavigate } from '@solidjs/router';
import { INFRASTRUCTURE_MAP_PATH } from '@/routing/resourceLinks';
import { getAvailabilityTargetAddLabel, isNetworkGearKind } from '@/utils/availabilityTargetKinds';
import { isFeatureAvailable } from '@/utils/featureReleases';
import Plus from 'lucide-solid/icons/plus';
import RotateCw from 'lucide-solid/icons/rotate-cw';
import Activity from 'lucide-solid/icons/activity';
import X from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { EmptyState } from '@/components/shared/EmptyState';
import { Dialog } from '@/components/shared/Dialog';
import { FormSelect } from '@/components/shared/FormSelect';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { SearchField } from '@/components/shared/SearchField';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import { useResources } from '@/hooks/useResources';
import { notificationStore } from '@/stores/notifications';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { buildProbeAgentOptions } from '@/utils/availabilityProbeAgents';
import {
  AvailabilityTargetsAPI,
  type AvailabilityTarget,
  type AvailabilityTargetKind,
} from '@/api/availabilityTargets';
import { AvailabilityTargetSlot } from './ConnectionEditor/CredentialSlots/AvailabilityTargetSlot';
import {
  buildAvailabilitySettingsPath,
  buildAvailabilityTargetAddPath,
  getAvailabilityTargetAddKind,
  getAvailabilityTargetAddPrefill,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetHealth,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetProbeSourceLabel,
  getAvailabilityTargetStatusClass,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetStatusTitle,
  getAvailabilityTargetsSummary,
  type AvailabilityTargetAddPrefill,
  shouldOpenAvailabilityTargetAddDialog,
} from './availabilitySettingsModel';

type AvailabilityDialogState =
  | {
      mode: 'add';
      initialTargetKind?: AvailabilityTargetKind;
      prefill: AvailabilityTargetAddPrefill;
    }
  | { mode: 'edit'; target: AvailabilityTarget }
  | null;

type PendingAvailabilityAction = {
  id: string;
  kind: 'test' | 'toggle' | 'delete';
} | null;

type AvailabilityStatusFilter = 'all' | 'attention' | 'paused';

const sortTargets = (targets: readonly AvailabilityTarget[]): AvailabilityTarget[] =>
  [...targets].sort((left, right) => left.name.localeCompare(right.name));

export const AvailabilitySettingsPanel: Component = () => {
  const now = useRelativeTimeNow();
  const location = useLocation();
  const navigate = useNavigate();
  const { resources } = useResources();
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());
  const probeAgentOptions = createMemo(() => buildProbeAgentOptions(resources()));
  // The Machines availability tab reads these same resources, so the counts and
  // colours here come from the same status logic when they are loaded.
  const availabilityResourceByTargetId = createMemo(() => {
    const byId = new Map<string, Resource>();
    for (const resource of resources()) {
      if (!isAgentlessAvailabilityResource(resource)) continue;
      const targetId = resource.availability?.targetId;
      if (targetId) byId.set(targetId, resource);
    }
    return byId;
  });
  const resourceForTarget = (target: AvailabilityTarget) =>
    availabilityResourceByTargetId().get(target.id);
  const [targets, setTargets] = createSignal<AvailabilityTarget[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [dialog, setDialog] = createSignal<AvailabilityDialogState>(null);
  const [pendingAction, setPendingAction] = createSignal<PendingAvailabilityAction>(null);
  const [deleteConfirmingId, setDeleteConfirmingId] = createSignal<string | null>(null);
  const [dialogActionError, setDialogActionError] = createSignal<string | null>(null);
  const [searchQuery, setSearchQuery] = createSignal('');
  const [statusFilter, setStatusFilter] = createSignal<AvailabilityStatusFilter>('all');
  const [kindFilter, setKindFilter] = createSignal<string>('all');

  const sortedTargets = createMemo(() => sortTargets(targets()));
  const availableKindLabels = createMemo(() => {
    const labels = new Set<string>();
    for (const target of targets()) {
      labels.add(getAvailabilityTargetKindLabel(target));
    }
    return Array.from(labels).sort((left, right) => left.localeCompare(right));
  });
  const filteredTargets = createMemo(() => {
    const query = searchQuery().trim().toLowerCase();
    const status = statusFilter();
    const kind = kindFilter();
    return sortedTargets().filter((target) => {
      if (status === 'paused' && target.enabled) return false;
      if (status === 'attention') {
        const health = getAvailabilityTargetHealth(target, resourceForTarget(target), now());
        if (health !== 'attention' && health !== 'offline') return false;
      }
      if (kind !== 'all' && getAvailabilityTargetKindLabel(target) !== kind) return false;
      if (!query) return true;
      const haystack = [
        target.name,
        target.address,
        getAvailabilityTargetKindLabel(target),
        getAvailabilityTargetMethodLabel(target),
        getAvailabilityTargetAddressLabel(target),
      ]
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  });

  const loadTargets = async () => {
    setLoading(true);
    setError(null);
    try {
      setTargets(await AvailabilityTargetsAPI.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load availability checks.');
    } finally {
      setLoading(false);
    }
  };

  const openAddDialog = (targetKind: AvailabilityTargetKind = 'service') => {
    if (readOnly()) return;
    setDeleteConfirmingId(null);
    setDialogActionError(null);
    navigate(buildAvailabilityTargetAddPath(targetKind), { scroll: false });
  };

  const closeDialog = (replace = false) => {
    const current = dialog();
    setDialog(null);
    setDeleteConfirmingId(null);
    setDialogActionError(null);
    // The map sent the owner here to add one thing. Saved or not, that is
    // where they were, and where what they added is drawn.
    if (
      current?.mode === 'add' &&
      current.prefill.from === 'map' &&
      isFeatureAvailable('infrastructureMap') &&
      // Only while this page is still the one they are on: a save that lands
      // after they have gone elsewhere must not pull them back.
      location.pathname.replace(/\/+$/, '') === buildAvailabilitySettingsPath()
    ) {
      navigate(INFRASTRUCTURE_MAP_PATH, { replace: true });
      return;
    }
    if (location.pathname === buildAvailabilitySettingsPath() && location.search) {
      navigate(buildAvailabilitySettingsPath(), { replace, scroll: false });
    }
  };

  const handleSaved = () => {
    void loadTargets();
    closeDialog(true);
  };

  const toggleTarget = async (target: AvailabilityTarget) => {
    if (readOnly()) return;
    setPendingAction({ id: target.id, kind: 'toggle' });
    setError(null);
    setDialogActionError(null);
    try {
      await AvailabilityTargetsAPI.update(target.id, { enabled: !target.enabled });
      await loadTargets();
      const currentDialog = dialog();
      if (currentDialog?.mode === 'edit' && currentDialog.target.id === target.id) {
        const updated = targets().find((item) => item.id === target.id);
        if (updated) setDialog({ mode: 'edit', target: updated });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to update availability check.';
      if (dialog()?.mode === 'edit') {
        setDialogActionError(message);
      } else {
        setError(message);
      }
    } finally {
      setPendingAction(null);
    }
  };

  const testTarget = async (target: AvailabilityTarget) => {
    if (readOnly()) return;
    setPendingAction({ id: target.id, kind: 'test' });
    setError(null);
    try {
      const result = await AvailabilityTargetsAPI.testSaved(target.id);
      if (result.outcome === 'indeterminate') {
        notificationStore.info(
          `${target.name}: open or filtered (${result.latencyMillis} ms)`,
          4000,
        );
      } else if (result.success) {
        notificationStore.success(`${target.name}: reachable in ${result.latencyMillis} ms`, 3500);
      } else {
        notificationStore.error(
          `${target.name}: ${result.error || 'Availability test failed.'}`,
          5000,
        );
      }
      await loadTargets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Availability test failed.');
    } finally {
      setPendingAction(null);
    }
  };

  const requestRemoveTarget = async (target: AvailabilityTarget) => {
    if (readOnly()) return;
    if (deleteConfirmingId() !== target.id) {
      setDeleteConfirmingId(target.id);
      setDialogActionError(null);
      return;
    }

    setPendingAction({ id: target.id, kind: 'delete' });
    setError(null);
    setDialogActionError(null);
    try {
      await AvailabilityTargetsAPI.remove(target.id);
      await loadTargets();
      closeDialog(true);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to remove availability check.';
      setDialogActionError(message);
    } finally {
      setPendingAction(null);
    }
  };

  createEffect(() => {
    if (readOnly()) {
      setDialog(null);
      return;
    }
    if (shouldOpenAvailabilityTargetAddDialog(location.pathname, location.search)) {
      setDialog({
        mode: 'add',
        initialTargetKind: getAvailabilityTargetAddKind(location.pathname, location.search),
        prefill: getAvailabilityTargetAddPrefill(location.pathname, location.search),
      });
      return;
    }

    const current = dialog();
    if (current?.mode === 'add') {
      setDialog(null);
    }
  });

  onMount(() => {
    void loadTargets();
  });

  const dialogTitle = createMemo(() => {
    const current = dialog();
    if (current?.mode === 'edit') return `Manage ${current.target.name}`;
    return getAvailabilityTargetAddLabel(current?.initialTargetKind);
  });

  const dialogDescription = createMemo(() => {
    const current = dialog();
    if (current?.mode === 'edit') {
      return `${getAvailabilityTargetMethodLabel(current.target)} · ${getAvailabilityTargetAddressLabel(current.target)}`;
    }
    if (current?.mode === 'add' && current.initialTargetKind === 'machine') {
      return 'Use reachability checks for servers, desktops, laptops, and other computers that do not run Pulse Agent.';
    }
    if (
      current?.mode === 'add' &&
      isNetworkGearKind(current.initialTargetKind) &&
      isFeatureAvailable('infrastructureMap')
    ) {
      return 'Pulse cannot see network gear by itself. A check gives it a live status and draws it on the map as the device it is.';
    }
    return 'Use ICMP ping, TCP port, UDP, or HTTP/HTTPS checks for devices and services that cannot run Pulse Agent.';
  });

  const isEmptyState = () => !loading() && !error() && targets().length === 0;

  return (
    <div class="space-y-6">
      <SettingsPanel noPadding>
        <div class="border-b border-border bg-surface px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-base-content">
                {getAvailabilityTargetsSummary(targets(), resourceForTarget, now())}
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">
                Monitor endpoint-only devices and services with ICMP, TCP, UDP, and HTTP/HTTPS
                probes.
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                variant="secondary"
                size="mdCompact"
                class="min-h-11 gap-2 sm:min-h-9"
                onClick={() => void loadTargets()}
                disabled={loading()}
              >
                <RotateCw class={`h-4 w-4 ${loading() ? 'animate-spin' : ''}`} />
                Refresh
              </Button>
              <Show when={!readOnly() && !isEmptyState()}>
                <Button
                  type="button"
                  variant="primary"
                  size="mdCompact"
                  class="min-h-11 gap-2 sm:min-h-9"
                  onClick={() => openAddDialog('service')}
                >
                  <Plus class="h-4 w-4" />
                  Add service/device check
                </Button>
              </Show>
            </div>
          </div>
        </div>

        <Show when={targets().length >= 4}>
          <div class="flex flex-col gap-2 border-b border-border bg-surface-alt/40 px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between">
            <div class="w-full sm:max-w-xs">
              <SearchField
                value={searchQuery()}
                onChange={setSearchQuery}
                title="Search availability checks"
                placeholder="Search checks by name or address..."
              />
            </div>
            <div class="flex flex-wrap items-center gap-1.5">
              <button
                type="button"
                aria-pressed={statusFilter() === 'all'}
                onClick={() => setStatusFilter('all')}
                class={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                  statusFilter() === 'all'
                    ? 'bg-blue-600 text-white'
                    : 'border border-border bg-surface text-muted hover:text-base-content'
                }`}
              >
                All
              </button>
              <button
                type="button"
                aria-pressed={statusFilter() === 'attention'}
                onClick={() =>
                  setStatusFilter((current) => (current === 'attention' ? 'all' : 'attention'))
                }
                class={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                  statusFilter() === 'attention'
                    ? 'bg-amber-600 text-white'
                    : 'border border-border bg-surface text-muted hover:text-base-content'
                }`}
              >
                Needs attention
              </button>
              <button
                type="button"
                aria-pressed={statusFilter() === 'paused'}
                onClick={() =>
                  setStatusFilter((current) => (current === 'paused' ? 'all' : 'paused'))
                }
                class={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                  statusFilter() === 'paused'
                    ? 'bg-slate-600 text-white'
                    : 'border border-border bg-surface text-muted hover:text-base-content'
                }`}
              >
                Paused
              </button>
              <Show when={availableKindLabels().length > 1}>
                <FormSelect
                  label="Filter by target kind"
                  labelClass="sr-only"
                  fieldBaseClass="contents"
                  value={kindFilter()}
                  onChange={(event) => setKindFilter(event.currentTarget.value)}
                  selectBaseClass="rounded-md border border-border bg-surface px-2 py-1 text-xs text-base-content"
                >
                  <option value="all">All kinds</option>
                  <For each={availableKindLabels()}>
                    {(kind) => <option value={kind}>{kind}</option>}
                  </For>
                </FormSelect>
              </Show>
            </div>
          </div>
        </Show>

        <div class="divide-y divide-border">
          <Show when={error()}>
            {(message) => (
              <div
                role="alert"
                class="flex flex-col gap-2 border-b border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900/25 dark:bg-rose-950/25 dark:text-rose-200 sm:flex-row sm:items-center sm:justify-between"
              >
                <span>{message()}</span>
                <Show when={targets().length === 0}>
                  <Button
                    type="button"
                    variant="secondary"
                    size="xs"
                    class="self-start sm:self-center"
                    onClick={() => void loadTargets()}
                  >
                    Retry
                  </Button>
                </Show>
              </div>
            )}
          </Show>

          <Show when={loading() && targets().length === 0}>
            <div class="p-4 text-sm text-muted">Loading availability checks...</div>
          </Show>

          <Show when={isEmptyState()}>
            <EmptyState
              variant="panel"
              icon={<Activity class="h-5 w-5" />}
              title="No availability checks yet"
              description="Add a ping, TCP port, MQTT broker, ESPHome device, HTTP health check, or another endpoint that only needs reachability monitoring."
              actions={
                !readOnly() ? (
                  <Button
                    type="button"
                    variant="primary"
                    size="mdCompact"
                    class="min-h-9! gap-2"
                    onClick={() => openAddDialog('service')}
                  >
                    <Plus class="h-4 w-4" />
                    Add service/device check
                  </Button>
                ) : undefined
              }
            />
          </Show>

          <Show when={targets().length > 0 && filteredTargets().length === 0}>
            <div class="space-y-2 px-4 py-8 text-center text-sm text-muted">
              <p>No availability checks match your current filter.</p>
              <Button
                type="button"
                variant="outline"
                size="xs"
                onClick={() => {
                  setSearchQuery('');
                  setStatusFilter('all');
                  setKindFilter('all');
                }}
              >
                Clear filters
              </Button>
            </div>
          </Show>

          <Show when={filteredTargets().length > 0}>
            <PlatformWindowedList
              items={filteredTargets}
              estimatedItemHeight={92}
              enableThreshold={24}
              windowSize={32}
            >
              {(target) => {
                const isPending = () => pendingAction()?.id === target.id;
                const isTesting = () => isPending() && pendingAction()?.kind === 'test';
                const isToggling = () => isPending() && pendingAction()?.kind === 'toggle';
                return (
                  <div class="grid gap-3 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                    <div class="min-w-0 space-y-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <div class="truncate text-sm font-semibold text-base-content">
                          {target.name}
                        </div>
                        <span
                          class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${getAvailabilityTargetStatusClass(target, resourceForTarget(target), now())}`}
                          title={getAvailabilityTargetStatusTitle(
                            target,
                            resourceForTarget(target),
                            now(),
                          )}
                        >
                          {getAvailabilityTargetStatusLabel(
                            target,
                            resourceForTarget(target),
                            now(),
                          )}
                        </span>
                        <Show
                          when={getAvailabilityTargetProbeSourceLabel(target, probeAgentOptions())}
                        >
                          {(sourceLabel) => (
                            <MetadataBadge
                              tone="muted"
                              size="xs"
                              appearance="outline"
                              data-availability-probe-source={target.status?.probeAgentId}
                            >
                              {sourceLabel()}
                            </MetadataBadge>
                          )}
                        </Show>
                      </div>
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
                        <span>{getAvailabilityTargetKindLabel(target)}</span>
                        <span aria-hidden="true">·</span>
                        <span>{getAvailabilityTargetMethodLabel(target)}</span>
                        <span aria-hidden="true">·</span>
                        <span class="break-all">{getAvailabilityTargetAddressLabel(target)}</span>
                      </div>
                    </div>
                    <Show when={!readOnly()}>
                      <div class="flex flex-wrap items-center gap-2 md:justify-end">
                        <Button
                          type="button"
                          variant="outline"
                          size="xs"
                          class="min-h-11 sm:min-h-8"
                          onClick={() => void testTarget(target)}
                          disabled={isPending()}
                        >
                          {isTesting() ? 'Testing...' : 'Test'}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="xs"
                          class="min-h-11 sm:min-h-8"
                          onClick={() => void toggleTarget(target)}
                          disabled={isPending()}
                        >
                          {isToggling()
                            ? target.enabled
                              ? 'Pausing...'
                              : 'Resuming...'
                            : target.enabled
                              ? 'Pause'
                              : 'Resume'}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="xs"
                          class="min-h-11 sm:min-h-8"
                          onClick={() => {
                            setDeleteConfirmingId(null);
                            setDialogActionError(null);
                            setDialog({ mode: 'edit', target });
                          }}
                        >
                          Manage
                        </Button>
                      </div>
                    </Show>
                  </div>
                );
              }}
            </PlatformWindowedList>
          </Show>
        </div>
      </SettingsPanel>

      <Show when={dialog()}>
        {(dialogAccessor) => {
          const editingTarget = () => {
            const current = dialogAccessor();
            return current.mode === 'edit' ? current.target : null;
          };
          const addState = () => {
            const current = dialogAccessor();
            return current.mode === 'add' ? current : null;
          };
          return (
            <Dialog
              isOpen={true}
              onClose={() => closeDialog(true)}
              ariaLabel={dialogTitle()}
              panelClass="max-w-4xl"
            >
              <div class="flex h-full min-h-0 flex-col">
                <div class="flex shrink-0 items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
                  <div class="space-y-1">
                    <h2 class="text-base font-semibold text-base-content">{dialogTitle()}</h2>
                    <p class="text-sm text-muted">{dialogDescription()}</p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="iconMd"
                    onClick={() => closeDialog(true)}
                    aria-label="Close availability check dialog"
                  >
                    <X class="h-4 w-4" />
                  </Button>
                </div>
                <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
                  <AvailabilityTargetSlot
                    editingTargetId={editingTarget()?.id}
                    initialTarget={editingTarget()}
                    initialTargetKind={addState()?.initialTargetKind}
                    initialName={addState()?.prefill.name}
                    initialAddress={addState()?.prefill.address}
                    onCancel={() => closeDialog(true)}
                    onSaved={handleSaved}
                    onToggleEnabled={
                      editingTarget()
                        ? () => {
                            const target = editingTarget();
                            if (target) void toggleTarget(target);
                          }
                        : undefined
                    }
                    togglePending={
                      editingTarget()
                        ? pendingAction()?.id === editingTarget()!.id &&
                          pendingAction()?.kind === 'toggle'
                        : false
                    }
                    connectionEnabled={editingTarget()?.enabled}
                    onDelete={
                      editingTarget()
                        ? () => {
                            const target = editingTarget();
                            if (target) void requestRemoveTarget(target);
                          }
                        : undefined
                    }
                    deletePending={
                      editingTarget()
                        ? pendingAction()?.id === editingTarget()!.id &&
                          pendingAction()?.kind === 'delete'
                        : false
                    }
                    deleteConfirming={
                      editingTarget() ? deleteConfirmingId() === editingTarget()!.id : false
                    }
                    deleteError={dialogActionError()}
                  />
                </div>
              </div>
            </Dialog>
          );
        }}
      </Show>
    </div>
  );
};
