import { createEffect, createMemo, createSignal } from 'solid-js';
import { ResourceActionsAPI } from '@/api/resourceActions';
import {
  clearContainerUpdateState,
  getContainerUpdateState,
  markContainerQueued,
  markContainerUpdateError,
  markContainerUpdateSuccess,
  updateStates,
} from '@/stores/containerUpdates';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
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
  const [localState, setLocalState] = createSignal<'idle' | 'confirming' | 'planning'>('idle');
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
  const isButtonDisabled = () => currentState() === 'updating' || !settingsLoaded();
  const buttonTooltip = () =>
    !settingsLoaded()
      ? 'Loading settings...'
      : getUpdateButtonTooltip({
          state: currentState(),
          updateStatus: props.updateStatus,
          storeState: storeState(),
          errorMessage: errorMessage(),
        });
  const buttonLabel = () => getUpdateButtonLabel(currentState(), settingsLoaded());

  // Updates run as audited actions: the confirming click plans an action for
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

  const handleClick = async (event: MouseEvent) => {
    event.stopPropagation();
    event.preventDefault();

    const state = currentState();
    if (state === 'updating' || state === 'success' || state === 'error') return;

    if (state === 'idle') {
      setLocalState('confirming');
      return;
    }

    if (state === 'confirming') {
      await planUpdateReview();
    }
  };

  const handleCancel = (event: MouseEvent) => {
    event.stopPropagation();
    event.preventDefault();
    setLocalState('idle');
    clearContainerUpdateState(props.agentId, props.containerId);
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
    handleCancel,
    handleClick,
    handleReviewChanged,
    handleReviewClosed,
    hasUpdate,
    isButtonDisabled,
    reviewDetail,
    settingsLoaded,
    shouldHideButton,
  };
}
