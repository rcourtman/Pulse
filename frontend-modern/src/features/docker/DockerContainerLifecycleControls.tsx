import {
  For,
  Match,
  Show,
  Switch,
  createEffect,
  createSignal,
  onCleanup,
  type Component,
} from 'solid-js';
import { Portal } from 'solid-js/web';
import Loader2Icon from 'lucide-solid/icons/loader-2';
import MoreHorizontalIcon from 'lucide-solid/icons/more-horizontal';
import PlayIcon from 'lucide-solid/icons/play';
import RotateCwIcon from 'lucide-solid/icons/rotate-cw';
import SquareIcon from 'lucide-solid/icons/square';
import { ResourceActionsAPI } from '@/api/resourceActions';
import { ActionReviewDialog } from '@/features/actions/ActionReviewDialog';
import { useMenuButton } from '@/components/shared/useMenuButton';
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
  collapsed?: boolean;
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
  const [menuOpen, setMenuOpen] = createSignal(false);
  const [menuPosition, setMenuPosition] = createSignal({ left: 0, top: 0 });
  let menuTriggerRef: HTMLButtonElement | undefined;
  let menuRef: HTMLDivElement | undefined;
  const menuButton = useMenuButton({
    isOpen: menuOpen,
    setOpen: setMenuOpen,
    trigger: () => menuTriggerRef,
    menu: () => menuRef,
  });

  const updateMenuPosition = () => {
    if (!menuTriggerRef || typeof window === 'undefined') return;
    const rect = menuTriggerRef.getBoundingClientRect();
    const menuWidth = 208;
    const estimatedMenuHeight = 132;
    setMenuPosition({
      left: Math.max(8, Math.min(window.innerWidth - menuWidth - 8, rect.right - menuWidth)),
      top: Math.max(8, Math.min(window.innerHeight - estimatedMenuHeight - 8, rect.bottom + 4)),
    });
  };

  createEffect(() => {
    if (!menuOpen() || typeof window === 'undefined') return;
    updateMenuPosition();
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (target instanceof Element && target.closest('[data-docker-lifecycle-menu-root]')) return;
      menuButton.closeMenu();
    };
    window.addEventListener('resize', updateMenuPosition);
    window.addEventListener('scroll', updateMenuPosition, true);
    document.addEventListener('pointerdown', closeOnOutsidePointer);
    onCleanup(() => {
      window.removeEventListener('resize', updateMenuPosition);
      window.removeEventListener('scroll', updateMenuPosition, true);
      document.removeEventListener('pointerdown', closeOnOutsidePointer);
    });
  });

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
    if (lastError()) return `${label} ${containerName}. Last error: ${lastError()}`;
    return `Review ${action} for ${containerName}`;
  };

  return (
    <div
      class={`inline-flex items-center justify-end gap-1 ${props.class ?? ''}`.trim()}
      data-prevent-toggle
      data-docker-container-actions-surface={props.surface ?? 'docker-page'}
      data-docker-lifecycle-menu-root
    >
      <Show
        when={props.collapsed}
        fallback={
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
        }
      >
        <button
          ref={menuTriggerRef}
          type="button"
          id={menuButton.triggerId}
          class={`inline-flex h-7 w-7 shrink-0 items-center justify-center rounded border ${enabledButtonClass}`}
          title={`Container actions for ${dockerContainerLifecycleName(props.resource)}`}
          aria-label={`Container actions for ${dockerContainerLifecycleName(props.resource)}`}
          aria-haspopup="menu"
          aria-expanded={menuOpen() ? 'true' : 'false'}
          aria-controls={menuButton.menuId}
          onMouseDown={(event) => event.stopPropagation()}
          onKeyDown={(event) => {
            event.stopPropagation();
            menuButton.handleTriggerKeyDown(event);
          }}
          onClick={(event) => {
            event.stopPropagation();
            updateMenuPosition();
            menuButton.toggleMenu();
          }}
        >
          <MoreHorizontalIcon class="h-4 w-4" aria-hidden="true" />
        </button>

        <Show when={menuOpen()}>
          <Portal mount={document.body}>
            <div
              ref={menuRef}
              id={menuButton.menuId}
              data-prevent-toggle
              data-docker-lifecycle-menu-root
              role="menu"
              aria-labelledby={menuButton.triggerId}
              class="fixed z-[9999] w-52 rounded-md border border-border bg-surface p-1 text-left shadow-lg"
              style={{ left: `${menuPosition().left}px`, top: `${menuPosition().top}px` }}
              onMouseDown={(event) => event.stopPropagation()}
              onClick={(event) => event.stopPropagation()}
              onKeyDown={(event) => {
                event.stopPropagation();
                menuButton.handleMenuKeyDown(event);
              }}
            >
              <For each={DOCKER_CONTAINER_LIFECYCLE_ACTIONS}>
                {(spec) => {
                  const disabled = () =>
                    Boolean(
                      getDockerContainerLifecycleDisabledReason(props.resource, spec.action),
                    ) ||
                    (planningAction() !== null && planningAction() !== spec.action);
                  const Icon = iconForAction(spec.action);
                  return (
                    <button
                      type="button"
                      role="menuitem"
                      tabindex="-1"
                      class="flex w-full items-center gap-2 rounded px-2 py-2 text-left text-xs font-medium text-base-content transition-colors hover:bg-surface-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/60 disabled:cursor-not-allowed disabled:opacity-50"
                      disabled={disabled()}
                      title={titleForAction(spec.action, spec.label)}
                      aria-label={titleForAction(spec.action, spec.label)}
                      data-docker-container-action={spec.action}
                      onClick={(event) => {
                        event.stopPropagation();
                        menuButton.closeMenu(true);
                        void prepareLifecycleReview(spec.action);
                      }}
                    >
                      <Show
                        when={planningAction() === spec.action}
                        fallback={<Icon class="h-3.5 w-3.5" aria-hidden="true" />}
                      >
                        <Loader2Icon class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                      </Show>
                      <span>{spec.label}</span>
                    </button>
                  );
                }}
              </For>
            </div>
          </Portal>
        </Show>
      </Show>
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
