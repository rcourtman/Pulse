import { useRelativeTimeNow } from '@/utils/relativeTimeClock';
import { Show, createEffect, createMemo, createSignal, onCleanup, type Component } from 'solid-js';
import { eventBus } from '@/stores/events';
import { getOrgID } from '@/utils/apiClient';
import { useAvailabilitySettingsState } from './useAvailabilitySettingsState';
import type { Resource } from '@/types/resource';
import { isAgentlessAvailabilityResource } from '@/features/standalone/standalonePageModel';
import { useLocation, useNavigate } from '@solidjs/router';
import Plus from 'lucide-solid/icons/plus';
import RotateCw from 'lucide-solid/icons/rotate-cw';
import Activity from 'lucide-solid/icons/activity';
import X from 'lucide-solid/icons/x';
import { Button } from '@/components/shared/Button';
import { EmptyState } from '@/components/shared/EmptyState';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { Dialog } from '@/components/shared/Dialog';
import { MetadataBadge } from '@/components/shared/MetadataBadge';
import { PlatformWindowedList } from '@/features/platformPage/PlatformWindowedList';
import { useResources } from '@/hooks/useResources';
import { buildProbeAgentOptions } from '@/utils/availabilityProbeAgents';
import { type AvailabilityTarget, type AvailabilityTargetKind } from '@/api/availabilityTargets';
import { AvailabilityTargetSlot } from './ConnectionEditor/CredentialSlots/AvailabilityTargetSlot';
import {
  buildAvailabilitySettingsPath,
  buildAvailabilityTargetAddPath,
  getAvailabilityTargetAddKind,
  getAvailabilityTargetAddressLabel,
  getAvailabilityTargetKindLabel,
  getAvailabilityTargetMethodLabel,
  getAvailabilityTargetProbeSourceLabel,
  getAvailabilityTargetStatusClass,
  getAvailabilityTargetStatusLabel,
  getAvailabilityTargetStatusTitle,
  getAvailabilityTargetsSummary,
  shouldOpenAvailabilityTargetAddDialog,
} from './availabilitySettingsModel';

export interface AvailabilitySettingsPanelProps {
  currentUser?: string;
  accessKey?: string;
  canRead?: boolean;
}

export const AvailabilitySettingsPanel: Component<AvailabilitySettingsPanelProps> = (props) => {
  const [orgRevision, setOrgRevision] = createSignal(0);
  const [withdrawnReader, setWithdrawnReader] = createSignal<object | null>(null);
  onCleanup(eventBus.on('org_switched', () => setOrgRevision((value) => value + 1)));
  const reader = createMemo(
    (previous?: {
      org: string;
      revision: number;
      user?: string;
      access?: string;
      read: boolean;
    }) => {
      const next = {
        org: getOrgID() || 'default',
        revision: orgRevision(),
        user: props.currentUser,
        access: props.accessKey,
        read: props.canRead !== false,
      };
      return previous &&
        Object.keys(next).every(
          (key) => next[key as keyof typeof next] === previous[key as keyof typeof next],
        )
        ? previous
        : next;
    },
  );
  return (
    <Show
      when={reader().read && withdrawnReader() !== reader()}
      fallback={
        <SettingsPanel title="Availability checks">
          <p class="p-4 text-sm text-muted">
            {reader().read
              ? 'The session or organisation changed. Reload this page before using availability checks.'
              : 'Availability checks are not available to this session.'}
          </p>
        </SettingsPanel>
      }
    >
      <Show when={reader()} keyed>
        {(accepted) => (
          <AvailabilityWorkspace
            org={accepted.org}
            isCurrentReader={() => {
              if (reader() !== accepted) return false;
              if ((getOrgID() || 'default') !== accepted.org) {
                // Storage drift (including a 401 clearing auth) is not an
                // authenticated organisation switch. Withdraw, never fall back
                // to another organisation using this reader's capability.
                setWithdrawnReader(accepted);
                return false;
              }
              return true;
            }}
          />
        )}
      </Show>
    </Show>
  );
};

const AvailabilityWorkspace: Component<{ org: string; isCurrentReader: () => boolean }> = (
  props,
) => {
  const now = useRelativeTimeNow();
  const location = useLocation();
  const navigate = useNavigate();
  const { resources } = useResources();
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
  const {
    targets,
    loading,
    loaded,
    loadError,
    error,
    dialog,
    setDialog,
    pendingActionId,
    deleteConfirmingId,
    setDeleteConfirmingId,
    canAct,
    isCurrent,
    rejectAccess,
    loadTargets,
    toggleTarget,
    testTarget,
    requestRemoveTarget,
    openEditDialog,
  } = useAvailabilitySettingsState(props);
  const sortedTargets = createMemo(() =>
    [...targets()].sort((a, b) => a.name.localeCompare(b.name)),
  );
  const openAddDialog = (targetKind: AvailabilityTargetKind = 'service') => {
    if (!canAct()) return;
    setDeleteConfirmingId(null);
    navigate(buildAvailabilityTargetAddPath(targetKind), { scroll: false });
  };
  const closeDialog = (replace = false) => {
    if (!isCurrent()) return;
    setDialog(null);
    setDeleteConfirmingId(null);
    if (location.pathname === buildAvailabilitySettingsPath() && location.search) {
      navigate(buildAvailabilitySettingsPath(), { replace, scroll: false });
    }
  };
  const handleSaved = () => {
    if (!isCurrent()) return;
    void loadTargets();
    closeDialog(true);
  };

  createEffect(() => {
    if (canAct() && shouldOpenAvailabilityTargetAddDialog(location.pathname, location.search)) {
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
                {loaded()
                  ? getAvailabilityTargetsSummary(targets(), resourceForTarget, now())
                  : loading()
                    ? 'Loading availability checks…'
                    : 'Availability checks unavailable'}
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
                class="min-h-11 gap-2 sm:min-h-9"
                onClick={() => void loadTargets()}
                disabled={loading() || Boolean(pendingActionId())}
              >
                <RotateCw class={`h-4 w-4 ${loading() ? 'animate-spin' : ''}`} />
                {loadError() ? 'Retry' : 'Refresh'}
              </Button>
              <Button
                type="button"
                variant="primary"
                size="mdCompact"
                class="min-h-11 gap-2 sm:min-h-9"
                onClick={() => openAddDialog('service')}
                disabled={!canAct()}
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
                class="border-b border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-800 dark:border-rose-900/25 dark:bg-rose-950/25 dark:text-rose-200"
              >
                {message()}
              </div>
            )}
          </Show>

          <Show
            when={sortedTargets().length > 0}
            fallback={
              <Show when={loaded() && !loading() && !loadError()}>
                <EmptyState
                  variant="panel"
                  icon={<Activity class="h-5 w-5" />}
                  title="No availability checks yet"
                  description="Add a ping, TCP port, MQTT broker, ESPHome device, HTTP health check, or another endpoint that only needs reachability monitoring."
                  actions={
                    <Button
                      type="button"
                      variant="primary"
                      size="mdCompact"
                      class="min-h-9! gap-2"
                      onClick={() => openAddDialog('service')}
                      disabled={!canAct()}
                    >
                      <Plus class="h-4 w-4" />
                      Add service/device check
                    </Button>
                  }
                />
              </Show>
            }
          >
            <PlatformWindowedList
              items={sortedTargets}
              estimatedItemHeight={92}
              enableThreshold={24}
              windowSize={32}
            >
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
                    <div class="flex flex-wrap items-center gap-2 md:justify-end">
                      <Button
                        type="button"
                        variant="outline"
                        size="xs"
                        class="min-h-11 sm:min-h-8"
                        onClick={() => void testTarget(target)}
                        disabled={!canAct()}
                      >
                        {pending() ? 'Testing...' : 'Test'}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="xs"
                        class="min-h-11 sm:min-h-8"
                        onClick={() => void toggleTarget(target)}
                        disabled={!canAct()}
                      >
                        {target.enabled ? 'Pause' : 'Resume'}
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="xs"
                        class="min-h-11 sm:min-h-8"
                        onClick={() => openEditDialog(target)}
                        disabled={!canAct()}
                      >
                        Manage
                      </Button>
                    </div>
                  </div>
                );
              }}
            </PlatformWindowedList>
          </Show>
        </div>
      </SettingsPanel>

      <Show when={dialog()}>
        {(dialogAccessor) => {
          const current = dialogAccessor();
          const editingTarget = current.mode === 'edit' ? current.target : null;
          const currentTarget = () =>
            editingTarget ? targets().find((target) => target.id === editingTarget.id) : undefined;
          return (
            <Dialog
              isOpen={true}
              onClose={() => closeDialog(true)}
              ariaLabel={dialogTitle()}
              panelClass="max-w-4xl"
            >
              <div class="flex min-h-0 flex-col">
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
                <div class="min-h-0 flex-1 overflow-y-auto p-4">
                  <AvailabilityTargetSlot
                    reader={{ org: props.org, isCurrent }}
                    blocked={!canAct()}
                    onAccessRejected={rejectAccess}
                    editingTargetId={editingTarget?.id}
                    initialTargetKind={
                      current.mode === 'add' ? current.initialTargetKind : undefined
                    }
                    onCancel={() => closeDialog(true)}
                    onSaved={handleSaved}
                    onToggleEnabled={
                      editingTarget
                        ? () => {
                            const target = currentTarget();
                            if (target) void toggleTarget(target);
                          }
                        : undefined
                    }
                    togglePending={editingTarget ? pendingActionId() === editingTarget.id : false}
                    connectionEnabled={currentTarget()?.enabled}
                    onDelete={
                      editingTarget
                        ? () => {
                            const target = currentTarget();
                            if (target) void requestRemoveTarget(target);
                          }
                        : undefined
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
