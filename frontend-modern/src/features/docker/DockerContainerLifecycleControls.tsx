import { For, Match, Switch, createSignal, type Component } from 'solid-js';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import PlayIcon from 'lucide-solid/icons/play';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import SquareIcon from 'lucide-solid/icons/square';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { notificationStore } from '@/stores/notifications';
import type { Resource } from '@/types/resource';
import {
  DOCKER_CONTAINER_LIFECYCLE_ACTIONS,
  dockerContainerLifecycleName,
  dockerContainerRuntimeLabel,
  getDockerContainerLifecycleDisabledReason,
  type DockerContainerLifecycleAction,
} from './dockerContainerLifecycleActions';

export type DockerContainerLifecycleSurface = 'docker-page' | 'resource-detail';

export type DockerContainerLifecycleSettledContext = {
  action: DockerContainerLifecycleAction;
  actionId: string;
  resource: Resource;
};

export type DockerContainerLifecycleControlsProps = {
  resource: Resource;
  class?: string;
  surface?: DockerContainerLifecycleSurface;
  onActionSettled?: (context: DockerContainerLifecycleSettledContext) => void | Promise<void>;
};

const buttonBaseClass =
  'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded border text-muted transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 focus-visible:ring-offset-1 focus-visible:ring-offset-surface';
const enabledButtonClass =
  'border-border-subtle bg-surface hover:border-blue-400 hover:bg-blue-50 hover:text-blue-700 dark:hover:bg-blue-950/40 dark:hover:text-blue-300';
const confirmButtonClass =
  'border-amber-400 bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300';
const disabledButtonClass = 'cursor-not-allowed border-border-subtle bg-surface-alt opacity-55';
const runningButtonClass =
  'cursor-wait border-blue-400 bg-blue-50 text-blue-700 dark:bg-blue-950/40';
const successButtonClass =
  'border-emerald-400 bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40';

const newRequestId = (): string => {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID();
  }
  return `docker-container-action-${Date.now()}-${Math.random().toString(16).slice(2)}`;
};

const iconForAction = (action: DockerContainerLifecycleAction): Component<{ class?: string }> => {
  switch (action) {
    case 'start':
      return PlayIcon;
    case 'stop':
      return SquareIcon;
    case 'restart':
      return RotateCwIcon;
  }
};

const errorMessage = (error: unknown): string =>
  error instanceof Error && error.message.trim() ? error.message.trim() : 'Action failed';

const surfaceLabel = (surface: DockerContainerLifecycleSurface | undefined): string =>
  surface === 'resource-detail' ? 'resource details' : 'Docker page';

const requestedByForSurface = (surface: DockerContainerLifecycleSurface | undefined): string =>
  surface === 'resource-detail' ? 'ui:resource-detail' : 'ui:docker-page';

export const DockerContainerLifecycleControls: Component<DockerContainerLifecycleControlsProps> = (
  props,
) => {
  const [confirmingAction, setConfirmingAction] =
    createSignal<DockerContainerLifecycleAction | null>(null);
  const [runningAction, setRunningAction] = createSignal<DockerContainerLifecycleAction | null>(
    null,
  );
  const [completedAction, setCompletedAction] = createSignal<DockerContainerLifecycleAction | null>(
    null,
  );
  const [lastError, setLastError] = createSignal('');

  const executeLifecycleAction = async (action: DockerContainerLifecycleAction) => {
    const disabledReason = getDockerContainerLifecycleDisabledReason(props.resource, action);
    if (disabledReason || runningAction()) return;
    if (confirmingAction() !== action) {
      setConfirmingAction(action);
      setLastError('');
      return;
    }

    const containerName = dockerContainerLifecycleName(props.resource);
    const runtimeLabel = dockerContainerRuntimeLabel(props.resource);
    const reason = `${action} ${runtimeLabel} container ${containerName} from the ${surfaceLabel(
      props.surface,
    )}.`;
    setRunningAction(action);
    setConfirmingAction(null);
    setLastError('');

    try {
      const plan = await ResourceActionsAPI.planAction({
        requestId: newRequestId(),
        resourceId: props.resource.id,
        capabilityName: action,
        params: {},
        reason,
        requestedBy: requestedByForSurface(props.surface),
      });
      if (!plan.allowed) {
        throw new Error(plan.message || 'Pulse refused the action plan.');
      }
      if (plan.requiresApproval) {
        await ResourceActionsAPI.decideAction(plan.actionId, 'approved', reason);
      }
      const result = await ResourceActionsAPI.executeAction(plan.actionId, reason);
      if (result.result && !result.result.success) {
        throw new Error(result.result.errorMessage || 'The action did not complete successfully.');
      }
      try {
        await props.onActionSettled?.({
          action,
          actionId: plan.actionId,
          resource: props.resource,
        });
      } catch {
        notificationStore.warning('Action requested. Refresh container inventory to verify state.');
      }
      setCompletedAction(action);
      window.setTimeout(
        () => setCompletedAction((current) => (current === action ? null : current)),
        2000,
      );
      notificationStore.success(`${runtimeLabel} container ${containerName}: ${action} requested`);
    } catch (error) {
      const message = errorMessage(error);
      setLastError(message);
      notificationStore.error(message);
    } finally {
      setRunningAction(null);
    }
  };

  const titleForAction = (action: DockerContainerLifecycleAction, label: string): string => {
    const disabledReason = getDockerContainerLifecycleDisabledReason(props.resource, action);
    const containerName = dockerContainerLifecycleName(props.resource);
    if (disabledReason) return `${label} unavailable: ${disabledReason}`;
    if (runningAction() === action) return `${label} ${containerName} through governed action`;
    if (confirmingAction() === action) return `Click again to ${action} ${containerName}`;
    if (lastError()) return `${label} ${containerName}; last error: ${lastError()}`;
    return `${label} ${containerName} through governed action`;
  };

  const classForAction = (action: DockerContainerLifecycleAction): string => {
    const disabledReason = getDockerContainerLifecycleDisabledReason(props.resource, action);
    if (runningAction() === action) return `${buttonBaseClass} ${runningButtonClass}`;
    if (completedAction() === action) return `${buttonBaseClass} ${successButtonClass}`;
    if (disabledReason || runningAction()) return `${buttonBaseClass} ${disabledButtonClass}`;
    if (confirmingAction() === action) return `${buttonBaseClass} ${confirmButtonClass}`;
    return `${buttonBaseClass} ${enabledButtonClass}`;
  };

  return (
    <div
      class={`inline-flex items-center justify-end gap-1 ${props.class ?? ''}`.trim()}
      data-prevent-toggle
      data-docker-container-actions-surface={props.surface ?? 'docker-page'}
    >
      <For each={DOCKER_CONTAINER_LIFECYCLE_ACTIONS}>
        {(spec) => {
          const disabled = () =>
            Boolean(getDockerContainerLifecycleDisabledReason(props.resource, spec.action)) ||
            (runningAction() !== null && runningAction() !== spec.action);
          const Icon = iconForAction(spec.action);

          return (
            <button
              type="button"
              class={classForAction(spec.action)}
              disabled={disabled()}
              title={titleForAction(spec.action, spec.label)}
              aria-label={titleForAction(spec.action, spec.label)}
              data-docker-container-action={spec.action}
              onMouseDown={(event) => event.stopPropagation()}
              onKeyDown={(event) => event.stopPropagation()}
              onClick={(event) => {
                event.stopPropagation();
                void executeLifecycleAction(spec.action);
              }}
            >
              <Switch>
                <Match when={runningAction() === spec.action}>
                  <Loader2Icon class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                </Match>
                <Match when={true}>
                  <Icon class="h-3.5 w-3.5" aria-hidden="true" />
                </Match>
              </Switch>
            </button>
          );
        }}
      </For>
    </div>
  );
};

export default DockerContainerLifecycleControls;
