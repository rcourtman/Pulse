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

  // Container updates now run as audited per-container actions; the legacy
  // bulk endpoint is retired and there is no reviewed bulk flow yet, so this
  // reports honestly instead of queuing something that cannot run.
  const queueDockerUpdateAll = async () => {
    resetDockerActionFeedback();
    const hostId = options.dockerHostSourceId();
    if (!hostId) return false;
    setConfirmUpdateAll(false);
    setDockerActionNote(
      'Updates now run as reviewed per-container actions: use the Update button on each container.',
    );
    return false;
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
