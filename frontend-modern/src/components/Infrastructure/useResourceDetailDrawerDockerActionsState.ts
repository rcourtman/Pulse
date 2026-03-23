import { createEffect, createSignal, type Accessor } from 'solid-js';
import { MonitoringAPI } from '@/api/monitoring';

interface UseResourceDetailDrawerDockerActionsStateOptions {
  dockerHostSourceId: Accessor<string | null>;
  dockerUpdatesAvailable: Accessor<number>;
}

const formatActionError = (error: unknown, fallback: string): string =>
  error instanceof Error && error.message ? error.message : fallback;

export const useResourceDetailDrawerDockerActionsState = (
  options: UseResourceDetailDrawerDockerActionsStateOptions,
) => {
  const [showDockerUpdateControls, setShowDockerUpdateControls] = createSignal(false);
  const [dockerActionError, setDockerActionError] = createSignal('');
  const [dockerActionNote, setDockerActionNote] = createSignal('');
  const [confirmUpdateAll, setConfirmUpdateAll] = createSignal(false);
  const [dockerActionBusy, setDockerActionBusy] = createSignal(false);

  const resetDockerActionFeedback = () => {
    setDockerActionError('');
    setDockerActionNote('');
  };

  createEffect(() => {
    if (!showDockerUpdateControls() || options.dockerHostSourceId() === null) {
      setConfirmUpdateAll(false);
    }
  });

  createEffect(() => {
    if (options.dockerUpdatesAvailable() <= 0) {
      setConfirmUpdateAll(false);
    }
  });

  const toggleDockerUpdateControls = () => {
    setShowDockerUpdateControls((value) => !value);
  };

  const queueDockerUpdateCheck = async () => {
    resetDockerActionFeedback();
    setConfirmUpdateAll(false);
    const hostId = options.dockerHostSourceId();
    if (!hostId) return false;
    try {
      setDockerActionBusy(true);
      await MonitoringAPI.checkDockerUpdates(hostId);
      setDockerActionNote('Check queued.');
      return true;
    } catch (error) {
      setDockerActionError(formatActionError(error, 'Failed to queue check'));
      return false;
    } finally {
      setDockerActionBusy(false);
    }
  };

  const queueDockerUpdateAll = async () => {
    resetDockerActionFeedback();
    const hostId = options.dockerHostSourceId();
    if (!hostId) return false;

    if (!confirmUpdateAll()) {
      setConfirmUpdateAll(true);
      setDockerActionNote(
        `Click again to update ${options.dockerUpdatesAvailable()} containers.`,
      );
      return false;
    }

    try {
      setDockerActionBusy(true);
      await MonitoringAPI.updateAllDockerContainers(hostId);
      setDockerActionNote('Update queued.');
      return true;
    } catch (error) {
      setDockerActionError(formatActionError(error, 'Failed to queue update'));
      return false;
    } finally {
      setDockerActionBusy(false);
      setConfirmUpdateAll(false);
    }
  };

  return {
    showDockerUpdateControls,
    dockerActionError,
    dockerActionNote,
    confirmUpdateAll,
    dockerActionBusy,
    toggleDockerUpdateControls,
    queueDockerUpdateCheck,
    queueDockerUpdateAll,
  };
};

export type UseResourceDetailDrawerDockerActionsStateResult = ReturnType<
  typeof useResourceDetailDrawerDockerActionsState
>;
