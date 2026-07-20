import { For, Match, Switch, createSignal, type Component } from 'solid-js';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import PlayIcon from 'lucide-solid/icons/play';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import SquareIcon from 'lucide-solid/icons/square';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { ActionReviewDialog } from '@/features/actions/ActionReviewDialog';
import { notificationStore } from '@/stores/notifications';
import type { ActionDetailResponse } from '@/types/actionAudit';
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
  'inline-flex h-10 w-10 shrink-0 items-center justify-center rounded border text-muted transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 focus-visible:ring-offset-1 focus-visible:ring-offset-surface sm:h-7 sm:w-7';
const enabledButtonClass =
  'border-border-subtle bg-surface hover:border-blue-400 hover:bg-blue-50 hover:text-blue-700 dark:hover:bg-blue-950/40 dark:hover:text-blue-300';
const disabledButtonClass = 'cursor-not-allowed border-border-subtle bg-surface-alt opacity-55';
const runningButtonClass =
  'cursor-wait border-blue-400 bg-blue-50 text-blue-700 dark:bg-blue-950/40';

const newRequestId = (): string =>
  typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `docker-container-action-${Date.now()}-${Math.random().toString(16).slice(2)}`;

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

const surfaceLabel = (surface?: DockerContainerLifecycleSurface): string =>
  surface === 'resource-detail' ? 'resource details' : 'Docker page';
const requestedByForSurface = (surface?: DockerContainerLifecycleSurface): string =>
  surface === 'resource-detail' ? 'ui:resource-detail' : 'ui:docker-page';

export const DockerContainerLifecycleControls: Component<DockerContainerLifecycleControlsProps> = (
  props,
) => {
  const [planningAction, setPlanningAction] = createSignal<DockerContainerLifecycleAction | null>(
    null,
  );
  const [reviewAction, setReviewAction] = createSignal<DockerContainerLifecycleAction | null>(null);
  const [reviewDetail, setReviewDetail] = createSignal<ActionDetailResponse | null>(null);
  const [lastError, setLastError] = createSignal('');

  const prepareLifecycleReview = async (action: DockerContainerLifecycleAction) => {
    const disabledReason = getDockerContainerLifecycleDisabledReason(props.resource, action);
    if (disabledReason || planningAction()) return;
    const containerName = dockerContainerLifecycleName(props.resource);
    const runtimeLabel = dockerContainerRuntimeLabel(props.resource);
    const reason = `${action} ${runtimeLabel} container ${containerName} from the ${surfaceLabel(props.surface)}.`;
    setPlanningAction(action);
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
      if (!plan.allowed) throw new Error(plan.message || 'Pulse refused the action plan.');
      setReviewAction(action);
      setReviewDetail(await ResourceActionsAPI.getAction(plan.actionId));
    } catch (error) {
      const message =
        error instanceof Error && error.message.trim()
          ? error.message.trim()
          : 'Action review could not be prepared.';
      setLastError(message);
      notificationStore.error(message);
    } finally {
      setPlanningAction(null);
    }
  };

  const titleForAction = (action: DockerContainerLifecycleAction, label: string): string => {
    const disabledReason = getDockerContainerLifecycleDisabledReason(props.resource, action);
    const containerName = dockerContainerLifecycleName(props.resource);
    if (disabledReason) return `${label} unavailable: ${disabledReason}`;
    if (planningAction() === action) return `Preparing review for ${action} ${containerName}`;
    if (lastError()) return `${label} ${containerName}; last error: ${lastError()}`;
    return `Review ${action} for ${containerName}`;
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
            (planningAction() !== null && planningAction() !== spec.action);
          const Icon = iconForAction(spec.action);
          return (
            <button
              type="button"
              class={`${buttonBaseClass} ${planningAction() === spec.action ? runningButtonClass : disabled() ? disabledButtonClass : enabledButtonClass}`}
              disabled={disabled()}
              title={titleForAction(spec.action, spec.label)}
              aria-label={titleForAction(spec.action, spec.label)}
              data-docker-container-action={spec.action}
              onMouseDown={(event) => event.stopPropagation()}
              onKeyDown={(event) => event.stopPropagation()}
              onClick={(event) => {
                event.stopPropagation();
                void prepareLifecycleReview(spec.action);
              }}
            >
              <Switch>
                <Match when={planningAction() === spec.action}>
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
      <ActionReviewDialog
        detail={reviewDetail()}
        onClose={() => setReviewDetail(null)}
        onChanged={async (detail) => {
          setReviewDetail(detail);
          const action = reviewAction();
          if (
            !action ||
            !['completed', 'failed', 'rejected', 'expired'].includes(detail.audit.state)
          )
            return;
          try {
            await props.onActionSettled?.({
              action,
              actionId: detail.audit.id,
              resource: props.resource,
            });
          } catch {
            notificationStore.warning(
              'Action recorded. Refresh container inventory to see the latest state.',
            );
          }
        }}
      />
    </div>
  );
};

export default DockerContainerLifecycleControls;
