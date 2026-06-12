import {
  For,
  Show,
  createEffect,
  createMemo,
  createSignal,
  onMount,
  type Component,
} from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import Plus from 'lucide-solid/icons/plus';
import RotateCw from 'lucide-solid/icons/rotate-cw';
import Activity from 'lucide-solid/icons/activity';
import X from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Dialog } from '@/components/shared/Dialog';
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
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetStatusClass,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetsSummary,
  shouldOpenAvailabilityTargetAddDialog,
} from './availabilitySettingsModel';

const rowActionButtonClass =
  'inline-flex min-h-8 items-center justify-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover disabled:cursor-not-allowed disabled:opacity-60';
const closeButtonClass =
  'inline-flex h-9 w-9 items-center justify-center rounded-md border border-border text-base-content transition-colors hover:bg-surface-hover';

type AvailabilityDialogState =
  | { mode: 'add'; initialTargetKind?: AvailabilityTargetKind }
  | { mode: 'edit'; target: AvailabilityTarget }
  | null;

const sortTargets = (targets: readonly AvailabilityTarget[]): AvailabilityTarget[] =>
  [...targets].sort((left, right) => left.name.localeCompare(right.name));

export const AvailabilitySettingsPanel: Component = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [targets, setTargets] = createSignal<AvailabilityTarget[]>([]);
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal<string | null>(null);
  const [dialog, setDialog] = createSignal<AvailabilityDialogState>(null);
  const [pendingActionId, setPendingActionId] = createSignal<string | null>(null);
  const [deleteConfirmingId, setDeleteConfirmingId] = createSignal<string | null>(null);
  const sortedTargets = createMemo(() => sortTargets(targets()));

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
    setDeleteConfirmingId(null);
    navigate(buildAvailabilityTargetAddPath(targetKind), { scroll: false });
  };

  const closeDialog = (replace = false) => {
    setDialog(null);
    setDeleteConfirmingId(null);
    if (location.pathname === buildAvailabilitySettingsPath() && location.search) {
      navigate(buildAvailabilitySettingsPath(), { replace, scroll: false });
    }
  };

  const handleSaved = () => {
    void loadTargets();
    closeDialog(true);
  };

  const toggleTarget = async (target: AvailabilityTarget) => {
    setPendingActionId(target.id);
    setError(null);
    try {
      await AvailabilityTargetsAPI.update(target.id, { enabled: !target.enabled });
      await loadTargets();
      const currentDialog = dialog();
      if (currentDialog?.mode === 'edit' && currentDialog.target.id === target.id) {
        const updated = targets().find((item) => item.id === target.id);
        if (updated) setDialog({ mode: 'edit', target: updated });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update availability check.');
    } finally {
      setPendingActionId(null);
    }
  };

  const testTarget = async (target: AvailabilityTarget) => {
    setPendingActionId(target.id);
    setError(null);
    try {
      await AvailabilityTargetsAPI.testSaved(target.id);
      await loadTargets();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Availability test failed.');
    } finally {
      setPendingActionId(null);
    }
  };

  const requestRemoveTarget = async (target: AvailabilityTarget) => {
    if (deleteConfirmingId() !== target.id) {
      setDeleteConfirmingId(target.id);
      return;
    }

    setPendingActionId(target.id);
    setError(null);
    try {
      await AvailabilityTargetsAPI.remove(target.id);
      await loadTargets();
      closeDialog(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove availability check.');
    } finally {
      setPendingActionId(null);
    }
  };

  createEffect(() => {
    if (shouldOpenAvailabilityTargetAddDialog(location.pathname, location.search)) {
      setDialog({
        mode: 'add',
        initialTargetKind: getAvailabilityTargetAddKind(location.pathname, location.search),
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
    if (current?.mode === 'add' && current.initialTargetKind === 'machine') {
      return 'Add machine check';
    }
    if (
      current?.mode === 'add' &&
      (current.initialTargetKind === 'service' || current.initialTargetKind === 'device')
    ) {
      return 'Add service/device check';
    }
    return 'Add availability check';
  });

  const dialogDescription = createMemo(() => {
    const current = dialog();
    if (current?.mode === 'edit') {
      return `${getAvailabilityTargetMethodLabel(current.target)} · ${getAvailabilityTargetAddressLabel(current.target)}`;
    }
    if (current?.mode === 'add' && current.initialTargetKind === 'machine') {
      return 'Use reachability checks for servers, desktops, laptops, and other computers that do not run Pulse Agent.';
    }
    return 'Use ICMP ping, TCP port, or HTTP checks for devices and services that cannot run Pulse Agent.';
  });

  return (
    <div class="space-y-6">
      <SettingsPanel title="Availability checks" noPadding>
        <div class="border-b border-border bg-surface px-4 py-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-base-content">
                {getAvailabilityTargetsSummary(targets())}
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">
                Monitor endpoint-only devices and services with ICMP, TCP, and HTTP probes.
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <Button
                type="button"
                variant="secondary"
                size="mdCompact"
                class="min-h-9 gap-2"
                onClick={() => void loadTargets()}
                disabled={loading()}
              >
                <RotateCw class={`h-4 w-4 ${loading() ? 'animate-spin' : ''}`} />
                Refresh
              </Button>
              <Button
                type="button"
                variant="primary"
                size="mdCompact"
                class="min-h-9 gap-2"
                onClick={() => openAddDialog('service')}
              >
                <Plus class="h-4 w-4" />
                Add service/device check
              </Button>
            </div>
          </div>
        </div>

        <div class="divide-y divide-border">
          <Show when={loading() && targets().length === 0}>
            <div class="p-4 text-sm text-muted">Loading availability checks...</div>
          </Show>

          <Show when={error()}>
            {(message) => (
              <div
                role="alert"
                class="border-b border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-200"
              >
                {message()}
              </div>
            )}
          </Show>

          <Show
            when={sortedTargets().length > 0}
            fallback={
              <div class="flex flex-col items-center justify-center gap-3 px-4 py-12 text-center">
                <div class="flex h-11 w-11 items-center justify-center rounded-md border border-border bg-surface-alt text-muted">
                  <Activity class="h-5 w-5" />
                </div>
                <div class="space-y-1">
                  <div class="text-sm font-semibold text-base-content">
                    No availability checks yet
                  </div>
                  <p class="max-w-xl text-sm leading-6 text-muted">
                    Add a ping, TCP port, MQTT broker, ESPHome device, HTTP health check, or another
                    endpoint that only needs reachability monitoring.
                  </p>
                </div>
                <Button
                  type="button"
                  variant="primary"
                  size="mdCompact"
                  class="min-h-9 gap-2"
                  onClick={() => openAddDialog('service')}
                >
                  <Plus class="h-4 w-4" />
                  Add service/device check
                </Button>
              </div>
            }
          >
            <For each={sortedTargets()}>
              {(target) => {
                const pending = () => pendingActionId() === target.id;
                return (
                  <div class="grid gap-3 px-4 py-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                    <div class="min-w-0 space-y-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <div class="truncate text-sm font-semibold text-base-content">
                          {target.name}
                        </div>
                        <span
                          class={`inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium ${getAvailabilityTargetStatusClass(target)}`}
                        >
                          {getAvailabilityTargetStatusLabel(target)}
                        </span>
                      </div>
                      <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
                        <span>{getAvailabilityTargetKindLabel(target)}</span>
                        <span aria-hidden="true">·</span>
                        <span>{getAvailabilityTargetMethodLabel(target)}</span>
                        <span aria-hidden="true">·</span>
                        <span class="break-all">{getAvailabilityTargetAddressLabel(target)}</span>
                      </div>
                    </div>
                    <div class="flex flex-wrap items-center gap-2 md:justify-end">
                      <button
                        type="button"
                        onClick={() => void testTarget(target)}
                        disabled={pending()}
                        class={rowActionButtonClass}
                      >
                        {pending() ? 'Testing...' : 'Test'}
                      </button>
                      <button
                        type="button"
                        onClick={() => void toggleTarget(target)}
                        disabled={pending()}
                        class={rowActionButtonClass}
                      >
                        {target.enabled ? 'Pause' : 'Resume'}
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setDeleteConfirmingId(null);
                          setDialog({ mode: 'edit', target });
                        }}
                        class={rowActionButtonClass}
                      >
                        Manage
                      </button>
                    </div>
                  </div>
                );
              }}
            </For>
          </Show>
        </div>
      </SettingsPanel>

      <Show when={dialog()}>
        {(dialogAccessor) => {
          const current = dialogAccessor();
          const editingTarget = current.mode === 'edit' ? current.target : null;
          return (
            <Dialog
              isOpen={true}
              onClose={() => closeDialog(true)}
              ariaLabel={dialogTitle()}
              panelClass="max-w-4xl"
            >
              <div class="flex h-full min-h-0 flex-col">
                <div class="flex items-start justify-between gap-4 border-b border-border bg-surface-alt px-4 py-4 sm:px-6">
                  <div class="space-y-1">
                    <h2 class="text-base font-semibold text-base-content">{dialogTitle()}</h2>
                    <p class="text-sm text-muted">{dialogDescription()}</p>
                  </div>
                  <button
                    type="button"
                    onClick={() => closeDialog(true)}
                    class={closeButtonClass}
                    aria-label="Close availability check dialog"
                  >
                    <X class="h-4 w-4" />
                  </button>
                </div>
                <div class="min-h-0 flex-1 overflow-y-auto p-4">
                  <AvailabilityTargetSlot
                    editingTargetId={editingTarget?.id}
                    initialTargetKind={
                      current.mode === 'add' ? current.initialTargetKind : undefined
                    }
                    onCancel={() => closeDialog(true)}
                    onSaved={handleSaved}
                    onToggleEnabled={
                      editingTarget ? () => void toggleTarget(editingTarget) : undefined
                    }
                    togglePending={editingTarget ? pendingActionId() === editingTarget.id : false}
                    connectionEnabled={editingTarget?.enabled}
                    onDelete={
                      editingTarget ? () => void requestRemoveTarget(editingTarget) : undefined
                    }
                    deletePending={editingTarget ? pendingActionId() === editingTarget.id : false}
                    deleteConfirming={
                      editingTarget ? deleteConfirmingId() === editingTarget.id : false
                    }
                    deleteError={null}
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
