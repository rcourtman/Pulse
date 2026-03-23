import { createEffect, createMemo, createSignal } from 'solid-js';
import { MonitoringAPI } from '@/api/monitoring';
import {
  clearContainerUpdateState,
  getContainerUpdateState,
  markContainerQueued,
  markContainerUpdateError,
  markContainerUpdateSuccess,
  updateStates,
} from '@/stores/containerUpdates';
import { areSystemSettingsLoaded, shouldHideDockerUpdateActions } from '@/stores/systemSettings';
import {
  getUpdateButtonLabel,
  getUpdateButtonTooltip,
  hasContainerUpdate,
  type UpdateButtonProps,
  type UpdateState,
} from './containerUpdateBadgeModel';

export function useContainerUpdateButtonState(props: UpdateButtonProps) {
  const [localState, setLocalState] = createSignal<'idle' | 'confirming'>('idle');
  const [errorMessage, setErrorMessage] = createSignal('');

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
    return localState();
  };

  createEffect(() => {
    const stored = storeState();
    if (stored && (stored.state === 'queued' || stored.state === 'updating')) {
      if (props.updateStatus?.updateAvailable === false) {
        markContainerUpdateSuccess(props.agentId, props.containerId);
      }
    }
  });

  const hasUpdate = () => hasContainerUpdate(props.updateStatus) || currentState() !== 'idle';
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
      markContainerQueued(props.agentId, props.containerId);
      setLocalState('idle');

      try {
        await MonitoringAPI.updateDockerContainer(
          props.agentId,
          props.containerId,
          props.containerName,
        );
        props.onUpdateTriggered?.();
      } catch (error) {
        const message = (error as Error).message || 'Failed to trigger update';
        setErrorMessage(message);
        markContainerUpdateError(props.agentId, props.containerId, message);
      }
    }
  };

  const handleCancel = (event: MouseEvent) => {
    event.stopPropagation();
    event.preventDefault();
    setLocalState('idle');
    clearContainerUpdateState(props.agentId, props.containerId);
  };

  return {
    buttonLabel,
    buttonTooltip,
    currentState,
    handleCancel,
    handleClick,
    hasUpdate,
    isButtonDisabled,
    settingsLoaded,
    shouldHideButton,
  };
}

export type ContainerUpdateButtonState = ReturnType<typeof useContainerUpdateButtonState>;
