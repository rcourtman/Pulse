import { createEffect, createSignal, Show, For } from 'solid-js';
import type { UpdatePlan } from '@/api/updates';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import ArrowRightIcon from 'lucide-solid/icons/arrow-right';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import ClockIcon from 'lucide-solid/icons/clock';
import LockIcon from 'lucide-solid/icons/lock';
import XIcon from 'lucide-solid/icons/x';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { Dialog } from '@/components/shared/Dialog';
import { UpdateReadinessChecksPanel, getUpdateReadinessCounts } from './UpdateReadinessChecksPanel';

interface UpdateConfirmationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  currentVersion: string;
  latestVersion: string;
  plan: UpdatePlan;
  isApplying: boolean;
  isPrerelease?: boolean;
  isMajorUpgrade?: boolean;
  warning?: string;
}

export function UpdateConfirmationModal(props: UpdateConfirmationModalProps) {
  const [acknowledged, setAcknowledged] = createSignal(false);
  const handleClose = () => {
    if (!props.isApplying) {
      props.onClose();
    }
  };

  createEffect(() => {
    if (!props.isOpen) {
      setAcknowledged(false);
    }
  });

  const readiness = () => props.plan.readiness;
  const readinessBlocked = () => getUpdateReadinessCounts(readiness()).isBlocked;

  const handleConfirm = () => {
    if (acknowledged() && !props.isApplying && !readinessBlocked()) {
      props.onConfirm();
    }
  };
  const warningTone = () => (props.isMajorUpgrade ? 'warning' : 'info');
  const warningTitle = () =>
    props.isMajorUpgrade && props.isPrerelease
      ? 'Major Version Pre-Release'
      : props.isMajorUpgrade
        ? 'Major Version Upgrade'
        : 'Pre-Release Build';

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={handleClose}
      panelClass="max-w-2xl"
      closeOnBackdrop={!props.isApplying}
      ariaLabel="Confirm update"
    >
      <div class="w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div class="px-6 py-4 border-b border-border">
          <div class="flex items-center justify-between">
            <h2 class="text-xl font-semibold text-base-content">Confirm Update</h2>
            <ActionIconButton
              onClick={handleClose}
              label="Close confirmation"
              title="Close"
              tone="muted"
              size="md"
              disabled={props.isApplying}
              type="button"
            >
              <XIcon class="h-5 w-5" aria-hidden="true" />
            </ActionIconButton>
          </div>
        </div>

        {/* Body */}
        <div class="px-6 py-4 space-y-4">
          {/* Version Jump */}
          <CalloutCard tone="info" scale="compact" padding="md" title="Version Update">
            <div class="flex items-center gap-3 text-sm text-blue-800 dark:text-blue-200">
              <span class="font-mono text-sm">{props.currentVersion}</span>
              <ArrowRightIcon class="h-4 w-4" aria-hidden="true" />
              <span class="font-mono text-sm font-semibold">{props.latestVersion}</span>
            </div>
          </CalloutCard>

          {/* Major version / pre-release warning */}
          <Show when={props.warning}>
            <CalloutCard
              tone={warningTone()}
              scale="compact"
              padding="md"
              icon={<AlertTriangleIcon class="h-5 w-5" aria-hidden="true" />}
              title={warningTitle()}
              description={<span class="text-sm">{props.warning}</span>}
            />
          </Show>

          {/* Preflight upgrade readiness checks */}
          <Show when={readiness()}>
            {(current) => <UpdateReadinessChecksPanel readiness={current()} />}
          </Show>

          {/* Estimated Time */}
          <Show when={props.plan.estimatedTime}>
            <div class="flex items-center gap-2 text-sm text-muted">
              <ClockIcon class="h-4 w-4" aria-hidden="true" />
              <span>Estimated time: {props.plan.estimatedTime}</span>
            </div>
          </Show>

          {/* Prerequisites */}
          <Show when={props.plan.prerequisites && props.plan.prerequisites.length > 0}>
            <div>
              <div class="text-sm font-medium text-base-content mb-2">Prerequisites</div>
              <ul class="space-y-2">
                <For each={props.plan.prerequisites}>
                  {(prerequisite) => (
                    <li class="flex items-start gap-2 text-sm text-base-content">
                      <AlertTriangleIcon
                        class="mt-0.5 h-4 w-4 shrink-0 text-amber-500"
                        aria-hidden="true"
                      />
                      <span>{prerequisite}</span>
                    </li>
                  )}
                </For>
              </ul>
            </div>
          </Show>

          {/* Root Required Warning */}
          <Show when={props.plan.requiresRoot}>
            <CalloutCard
              tone="warning"
              scale="compact"
              padding="md"
              icon={<LockIcon class="h-5 w-5" aria-hidden="true" />}
              title="Root access required"
              description={
                <span class="text-sm">
                  This update requires elevated privileges to modify system files.
                </span>
              }
            />
          </Show>

          {/* Rollback Support */}
          <Show when={props.plan.rollbackSupport}>
            <div class="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
              <CheckCircleIcon class="h-4 w-4" aria-hidden="true" />
              <span>Automatic backup will be created</span>
            </div>
          </Show>

          {/* Acknowledgement Checkbox */}
          <div class="pt-4 border-t border-border">
            <label
              class={`flex items-start gap-3 ${readinessBlocked() ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'}`}
            >
              <input
                type="checkbox"
                checked={acknowledged()}
                onChange={(e) => setAcknowledged(e.currentTarget.checked)}
                class="mt-1 w-4 h-4 text-blue-600 bg-surface-alt rounded-sm focus:ring-blue-500 focus:ring-2"
                disabled={props.isApplying || readinessBlocked()}
              />
              <span class="text-sm text-base-content">
                I understand that Pulse will be temporarily unavailable during the update process.
                {props.plan.rollbackSupport && ' A backup will be created automatically.'}
              </span>
            </label>
            <Show when={readinessBlocked()}>
              <p class="mt-2 text-xs font-medium text-red-700 dark:text-red-300">
                Resolve blocked upgrade checks before starting this update.
              </p>
            </Show>
          </div>
        </div>

        {/* Footer */}
        <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-end gap-3">
          <Button
            onClick={handleClose}
            disabled={props.isApplying}
            variant="ghost"
            size="md"
            type="button"
          >
            Cancel
          </Button>
          <Button
            onClick={handleConfirm}
            disabled={!acknowledged() || props.isApplying || readinessBlocked()}
            isLoading={props.isApplying}
            variant="primary"
            size="md"
            type="button"
          >
            <span>
              {props.isApplying
                ? 'Starting...'
                : readinessBlocked()
                  ? 'Install blocked'
                  : 'Start Update'}
            </span>
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
