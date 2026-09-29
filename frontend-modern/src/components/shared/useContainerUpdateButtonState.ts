import { createMemo, createSignal } from 'solid-js';
import { ResourceActionsAPI } from '@/api/resourceActions';
import {
  getContainerUpdateState,
  markContainerQueued,
  markContainerUpdateError,
  markContainerUpdateInconclusive,
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
  isContainerUpdatePinned,
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
          return 'queued';
        case 'updating':
          return 'updating';
        case 'success':
          return 'success';
        case 'error':
          return 'error';
        case 'inconclusive':
          return 'inconclusive';
      }
    }

    if (props.externalState === 'updating' || props.externalState === 'queued') return 'updating';
    if (props.externalState === 'error') return 'error';
    const local = localState();
    return local === 'planning' ? 'updating' : local;
  };

  const hasUpdate = () =>
    hasContainerUpdate(props.updateStatus) ||
    hasContainerUpdateError(props.updateStatus) ||
    hasContainerUpdateCurrent(props.updateStatus) ||
    isContainerUpdatePinned(props.updateStatus) ||
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
    currentState() === 'updating' ||
    ((currentState() === 'queued' || currentState() === 'inconclusive') &&
      !storeState()?.actionId) ||
    !settingsLoaded() ||
    isUpdateUnavailable();
  const buttonTooltip = () => {
    if (!settingsLoaded()) return 'Loading settings...';
    const refusal = updateUnavailableReason();
    if (refusal) return `Update unavailable: ${refusal}`;
    const state = currentState();
    const tooltip = getUpdateButtonTooltip({
      state,
      updateStatus: props.updateStatus,
      storeState: storeState(),
      errorMessage: errorMessage(),
    });
    return (state === 'queued' || state === 'inconclusive') && errorMessage()
      ? `${errorMessage()} ${tooltip}`
      : tooltip;
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
    if (state === 'queued' || state === 'inconclusive') {
      const actionId = storeState()?.actionId;
      if (!actionId) return;
      try {
        const detail = await ResourceActionsAPI.getAction(actionId);
        setReviewDetail(detail);
        handleReviewChanged(detail);
        setErrorMessage('');
      } catch {
        setErrorMessage('Could not re-read the action. No new update was sent.');
      }
      return;
    }
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
      markContainerQueued(props.agentId, props.containerId, detail.audit.id);
      props.onUpdateTriggered?.();
      return;
    }
    if (!ACTION_TERMINAL_STATES.includes(state)) return;
    if (state === 'completed') {
      markContainerUpdateSuccess(props.agentId, props.containerId);
      props.onUpdateTriggered?.();
    } else if (state === 'failed') {
      if (detail.audit.result?.actionResultV2?.execution.reasonCode === 'operator_force_failed') {
        markContainerUpdateInconclusive(props.agentId, props.containerId, detail.audit.id);
        return;
      }
      const message = 'The update action failed. Open Actions for the audit trail.';
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
