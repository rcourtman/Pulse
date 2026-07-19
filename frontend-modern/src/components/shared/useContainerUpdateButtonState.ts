import { createEffect, createMemo, createSignal } from 'solid-js';
import { ResourceActionsAPI } from '@/api/resourceActions';
import {
  getContainerUpdateState,
  markContainerQueued,
  markContainerUpdateError,
  markContainerUpdateSuccess,
  updateStates,
} from '@/stores/containerUpdates';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import { getActionReadinessRefusal } from '@/utils/actionReadiness';
import type { ActionDetailResponse } from '@/types/actionAudit';
import {
  getUpdateButtonLabel,
  getUpdateButtonTooltip,
  getUpdatePlanErrorMessage,
  hasContainerUpdate,
  hasContainerUpdateCurrent,
  hasContainerUpdateError,
  type UpdateButtonProps,
  type UpdateState,
} from './containerUpdateBadgeModel';

const newUpdateRequestId = (): string =>
  typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `container-update-${Date.now()}-${Math.random().toString(16).slice(2)}`;

const ACTION_TERMINAL_STATES = ['completed', 'failed', 'rejected', 'expired'];

export function useContainerUpdateButtonState(props: UpdateButtonProps) {
  const [localState, setLocalState] = createSignal<'idle' | 'planning'>('idle');
  const [errorMessage, setErrorMessage] = createSignal('');
  const [reviewDetail, setReviewDetail] = createSignal<ActionDetailResponse | null>(null);

  const settingsLoaded = () => areSystemSettingsLoaded();
  const shouldHideButton = () => shouldHideDockerUpdateActions();

  const storeState = createMemo(() => {
    updateStates();
    return getContainerUpdateState(props.agentId, props.containerId);
  });

  const currentState = (): UpdateState => {
    const stored = storeState();
    if (stored) {
      switch (stored.state) {
        case 'queued':
        case 'updating':
          return 'updating';
        case 'success':
          return 'success';
        case 'error':
          return 'error';
      }
    }

    if (props.externalState === 'updating' || props.externalState === 'queued') return 'updating';
    if (props.externalState === 'error') return 'error';
    const local = localState();
    return local === 'planning' ? 'updating' : local;
  };

  createEffect(() => {
    const stored = storeState();
    if (stored && (stored.state === 'queued' || stored.state === 'updating')) {
      if (props.updateStatus?.updateAvailable === false) {
        markContainerUpdateSuccess(props.agentId, props.containerId);
      }
    }
  });

  const hasUpdate = () =>
    hasContainerUpdate(props.updateStatus) ||
    hasContainerUpdateError(props.updateStatus) ||
    hasContainerUpdateCurrent(props.updateStatus) ||
    currentState() !== 'idle';

  // Server-evaluated refusal for the update capability (agent disconnected,
  // agent too old, stale inventory). Mirrors the lifecycle buttons: render
  // disabled with the reason instead of letting the click fail at plan time.
  // Only gates the actionable states; in-flight and settled states keep their
  // own presentation.
  const updateUnavailableReason = (): string | undefined => {
    if (currentState() !== 'idle') return undefined;
    return getActionReadinessRefusal(props.actionReadiness, 'update');
  };
  const isUpdateUnavailable = () => Boolean(updateUnavailableReason());

  const isButtonDisabled = () =>
    currentState() === 'updating' || !settingsLoaded() || isUpdateUnavailable();
  const buttonTooltip = () => {
    if (!settingsLoaded()) return 'Loading settings...';
    const refusal = updateUnavailableReason();
    if (refusal) return `Update unavailable: ${refusal}`;
    return getUpdateButtonTooltip({
      state: currentState(),
      updateStatus: props.updateStatus,
      storeState: storeState(),
      errorMessage: errorMessage(),
    });
  };
  const buttonLabel = () => getUpdateButtonLabel(currentState(), settingsLoaded());

  // Updates run as audited actions: the update click plans an action for
  // the container's update capability and opens the review dialog, which owns
  // approval and execution. The legacy direct-update endpoint is retired.
  const planUpdateReview = async () => {
    const resourceId = (props.resourceId ?? '').trim();
    if (!resourceId) {
      const message = 'Container update action is unavailable for this row.';
      setErrorMessage(message);
      markContainerUpdateError(props.agentId, props.containerId, message);
      return;
    }
    setLocalState('planning');
    try {
      const plan = await ResourceActionsAPI.planAction({
        requestId: newUpdateRequestId(),
        resourceId,
        capabilityName: 'update',
        params: {},
        reason: `Update container ${props.containerName} to its latest image.`,
        requestedBy: 'ui:container-update',
      });
      if (!plan.allowed) {
        throw new Error(plan.message || 'Pulse refused the update plan.');
      }
      setReviewDetail(await ResourceActionsAPI.getAction(plan.actionId));
      setLocalState('idle');
    } catch (error) {
      const message = getUpdatePlanErrorMessage(error);
      setErrorMessage(message);
      setLocalState('idle');
      markContainerUpdateError(props.agentId, props.containerId, message);
    }
  };

  // One click plans the governed action and opens the review dialog; the
  // dialog is the confirmation surface, so no in-row confirming hop exists.
  const handleClick = async (event: MouseEvent) => {
    event.stopPropagation();
    event.preventDefault();

    const state = currentState();
    if (state === 'updating' || state === 'success' || state === 'error') return;
    if (isUpdateUnavailable()) return;

    if (state === 'idle') {
      await planUpdateReview();
    }
  };

  const handleReviewClosed = () => {
    setReviewDetail(null);
  };

  const handleReviewChanged = (detail: ActionDetailResponse) => {
    setReviewDetail(detail);
    const state = detail.audit.state;
    if (state === 'executing') {
      markContainerQueued(props.agentId, props.containerId);
      props.onUpdateTriggered?.();
      return;
    }
    if (!ACTION_TERMINAL_STATES.includes(state)) return;
    if (state === 'completed') {
      markContainerQueued(props.agentId, props.containerId);
      props.onUpdateTriggered?.();
    } else if (state === 'failed') {
      const message = 'The update action failed; open Actions for the audit trail.';
      setErrorMessage(message);
      markContainerUpdateError(props.agentId, props.containerId, message);
    }
  };

  return {
    buttonLabel,
    buttonTooltip,
    currentState,
    handleClick,
    handleReviewChanged,
    handleReviewClosed,
    hasUpdate,
    isButtonDisabled,
    isUpdateUnavailable,
    reviewDetail,
    settingsLoaded,
    shouldHideButton,
  };
}
