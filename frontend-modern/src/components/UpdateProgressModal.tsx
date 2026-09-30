import { createSignal, Show, onCleanup, createEffect, untrack } from 'solid-js';
import { UpdatesAPI, type UpdateStatus } from '@/api/updates';
import AlertTriangleIcon from 'lucide-solid/icons/alert-triangle';
import CheckCircleIcon from 'lucide-solid/icons/check-circle';
import InfoIcon from 'lucide-solid/icons/info';
import { Dialog } from '@/components/shared/Dialog';
import { ActionIconButton, Button } from '@/components/shared/Button';
import { CalloutCard } from '@/components/shared/CalloutCard';
import { LoadingSpinner } from '@/components/shared/LoadingSpinner';
import { ProgressBar } from '@/components/shared/ProgressBar';
import { apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';
import { updateStore } from '@/stores/updates';
import {
  RESTART_SUSPECT_POLL_FAILURES,
  UPDATE_PROGRESS_STALL_TIMEOUT_MS,
  UPDATE_STATUS_POLL_INTERVAL_MS,
  UPDATE_STREAM_SILENCE_FALLBACK_MS,
  isLateUpdateStage,
  isUpdateInProgressStage,
  resolvePostUpdateReload,
  shouldApplyUpdateStatus,
} from '@/components/updateReadinessModel';
import XIcon from 'lucide-solid/icons/x';

interface UpdateProgressModalProps {
  isOpen: boolean;
  onClose: () => void;
  onViewHistory: () => void;
  connected?: () => boolean;
  reconnecting?: () => boolean;
}

export function UpdateProgressModal(props: UpdateProgressModalProps) {
  const [status, setStatus] = createSignal<UpdateStatus | null>(null);
  const [isComplete, setIsComplete] = createSignal(false);
  const [hasError, setHasError] = createSignal(false);
  const [isRestarting, setIsRestarting] = createSignal(false);
  const [wsDisconnected, setWsDisconnected] = createSignal(false);
  const [healthCheckAttempts, setHealthCheckAttempts] = createSignal(0);
  const [progressStalled, setProgressStalled] = createSignal(false);
  let pollInterval: number | undefined;
  let healthCheckTimer: number | undefined;
  let streamSilenceTimer: number | undefined;
  let progressStallTimer: number | undefined;
  let eventSource: EventSource | undefined;
  // The version that started this update. The backend keeps serving (and
  // answering health checks) for a grace period after reporting 'completed',
  // so "different version than this" is the only trustworthy restart signal.
  let preUpdateVersion: string | null = null;
  let sameVersionHealthyAttempts = 0;
  // Set while a terminal status is being resolved (version probe in flight),
  // so SSE and polling reporting the same terminal status act on it once.
  let resolvingTerminalStatus = false;
  // Bumped on every open/close so late async results from a previous open
  // cannot drive the current one.
  let session = 0;
  // True once the backend itself reported 'restarting' or 'completed'. Only
  // then may the modal stop listening for status and, as a last resort,
  // reload on an unchanged version. A restart inferred from failing
  // requests stays unconfirmed: the feeds keep running so a late progress
  // or error event can still correct it.
  let restartConfirmed = false;
  let consecutivePollFailures = 0;
  // Identifies the live health-check loop. Clearing the timer alone cannot
  // stop a probe already in flight from re-arming itself.
  let healthCheckLoop = 0;
  // Set when a version probe fails after confirmed completion: the old
  // process was seen going away, so the next healthy answer is the new one.
  let restartObserved = false;
  let baselineFetchInFlight = false;

  const resetModalState = () => {
    setStatus(null);
    setIsComplete(false);
    setHasError(false);
    setIsRestarting(false);
    setWsDisconnected(false);
    setHealthCheckAttempts(0);
    setProgressStalled(false);
    preUpdateVersion = updateStore.versionInfo()?.version ?? null;
    sameVersionHealthyAttempts = 0;
    resolvingTerminalStatus = false;
    restartConfirmed = false;
    consecutivePollFailures = 0;
    restartObserved = false;
    baselineFetchInFlight = false;
  };

  // The pre-update version is only trustworthy while the old process is
  // certainly the one answering: before any restart signal, and before a
  // terminal or fresh-process status.
  const canAdoptBaseline = () => {
    if (preUpdateVersion || restartConfirmed || isRestarting() || resolvingTerminalStatus) {
      return false;
    }
    const current = status()?.status;
    return current !== 'restarting' && current !== 'completed' && current !== 'idle';
  };

  const adoptBaseline = (version: string, source: string) => {
    if (!version || !canAdoptBaseline()) {
      return;
    }
    preUpdateVersion = version;
    logger.info('Captured pre-update version for restart detection', { version, source });
  };

  // The global watcher can open the modal before the update store has loaded
  // the running version. Without that baseline an old-process answer looks
  // like the new one, so fetch it directly while the old process is serving.
  const fetchBaseline = () => {
    if (baselineFetchInFlight || !canAdoptBaseline()) {
      return;
    }
    baselineFetchInFlight = true;
    const fetchSession = session;
    void (async () => {
      try {
        const response = await apiFetch('/api/version', { cache: 'no-store' });
        if (fetchSession !== session || !response.ok) {
          return;
        }
        const info = (await response.json()) as { version?: unknown };
        if (fetchSession === session && typeof info.version === 'string') {
          adoptBaseline(info.version, 'version-probe');
        }
      } catch (error) {
        logger.warn('Could not read the pre-update version, will retry', error);
      } finally {
        if (fetchSession === session) {
          baselineFetchInFlight = false;
        }
      }
    })();
  };

  // Probe the backend and reload only once it reports a different version
  // than the one that started the update (or the bounded fallback in the
  // model fires). Returns true when a reload was triggered.
  const attemptReadyReload = async (): Promise<boolean> => {
    try {
      const response = await apiFetch('/api/version', { cache: 'no-store' });
      if (!response.ok) {
        sameVersionHealthyAttempts = 0;
        if (restartConfirmed) {
          restartObserved = true;
        }
        return false;
      }
      const info = (await response.json()) as { version?: unknown };
      const reportedVersion = typeof info.version === 'string' ? info.version : '';
      const decision = resolvePostUpdateReload({
        preUpdateVersion,
        reportedVersion,
        sameVersionHealthyAttempts,
        completionConfirmed: restartConfirmed,
        restartObserved,
      });
      if (decision === 'reload') {
        logger.info('Backend ready after update, reloading...', {
          preUpdateVersion,
          reportedVersion,
        });
        window.location.reload();
        return true;
      }
      sameVersionHealthyAttempts += 1;
      return false;
    } catch (error) {
      // Connection refused here usually means the restart is actually
      // happening now; the pre-restart healthy answers no longer count.
      sameVersionHealthyAttempts = 0;
      if (restartConfirmed) {
        restartObserved = true;
      }
      logger.warn('Version probe failed while waiting for restart, will retry', error);
      return false;
    }
  };

  const clearHealthCheckTimer = () => {
    healthCheckLoop += 1;
    if (healthCheckTimer !== undefined) {
      clearTimeout(healthCheckTimer);
      healthCheckTimer = undefined;
    }
  };

  const clearPollInterval = () => {
    if (pollInterval !== undefined) {
      clearInterval(pollInterval);
      pollInterval = undefined;
    }
  };

  const clearStreamSilenceTimer = () => {
    if (streamSilenceTimer !== undefined) {
      clearTimeout(streamSilenceTimer);
      streamSilenceTimer = undefined;
    }
  };

  const clearProgressStallTimer = () => {
    if (progressStallTimer !== undefined) {
      clearTimeout(progressStallTimer);
      progressStallTimer = undefined;
    }
  };

  const closeSSE = () => {
    clearStreamSilenceTimer();
    if (!eventSource) {
      return;
    }
    eventSource.close();
    eventSource = undefined;
    logger.info('SSE connection closed');
  };

  // Stop the status feeds (stream and poll); restart detection and the stall
  // timer are managed separately.
  const stopStatusFeeds = () => {
    closeSSE();
    clearPollInterval();
  };

  const stopEverything = () => {
    stopStatusFeeds();
    clearHealthCheckTimer();
    clearProgressStallTimer();
  };

  // Any real movement (new stage, new progress, entering the restart) pushes
  // the "may have finished" fallback back out. Repeated identical answers do
  // not, so a stream or poll that keeps echoing one stage still times out.
  const armProgressStallTimer = () => {
    clearProgressStallTimer();
    setProgressStalled(false);
    progressStallTimer = window.setTimeout(() => {
      progressStallTimer = undefined;
      if (!isComplete()) {
        logger.warn('Update progress stalled, offering a manual reload');
        setProgressStalled(true);
      }
    }, UPDATE_PROGRESS_STALL_TIMEOUT_MS);
  };

  // A quiet stream is not a closed stream. If nothing arrives for a while,
  // poll as well rather than trusting the open EventSource.
  const armStreamSilenceWatchdog = () => {
    clearStreamSilenceTimer();
    streamSilenceTimer = window.setTimeout(() => {
      streamSilenceTimer = undefined;
      if (!eventSource || isComplete() || restartConfirmed) {
        return;
      }
      logger.warn('Update progress stream went quiet, polling status as well');
      startPolling();
    }, UPDATE_STREAM_SILENCE_FALLBACK_MS);
  };

  // The backend reported the restart itself: stop listening for status and
  // wait for the new version to serve.
  const enterConfirmedRestartPhase = () => {
    restartConfirmed = true;
    stopStatusFeeds();
    armProgressStallTimer();
    // A suspected restart already has a health-check loop running; it reads
    // the confirmation on its next probe.
    if (!isRestarting()) {
      setIsRestarting(true);
      startHealthCheckPolling();
    }
  };

  // Requests are failing late in the update, which usually means the old
  // process just exited. Probe for the new version, but keep the status
  // feeds running so a status from a still-running old process wins.
  const enterSuspectedRestartPhase = () => {
    if (isRestarting()) {
      return;
    }
    logger.warn('Status requests failing late in the update, probing for the restarted backend');
    setIsRestarting(true);
    startHealthCheckPolling();
  };

  const leaveSuspectedRestartPhase = () => {
    if (!isRestarting() || restartConfirmed) {
      return;
    }
    setIsRestarting(false);
    clearHealthCheckTimer();
    setHealthCheckAttempts(0);
  };

  const finishWithResult = (failed: boolean) => {
    stopStatusFeeds();
    clearHealthCheckTimer();
    clearProgressStallTimer();
    setProgressStalled(false);
    // A failure reported while a restart was only suspected ends that too.
    setIsRestarting(false);
    setIsComplete(true);
    if (failed) {
      setHasError(true);
    }
  };

  // Single entry point for statuses from the SSE stream and from polling.
  const handleStatus = (next: UpdateStatus) => {
    if (isComplete() || restartConfirmed || resolvingTerminalStatus) {
      return;
    }
    const previous = status();
    if (!shouldApplyUpdateStatus(previous, next)) {
      return;
    }

    if (next.status === 'idle') {
      handleIdleStatus(previous);
      return;
    }

    setStatus(next);
    if (!previous || previous.status !== next.status || previous.progress !== next.progress) {
      armProgressStallTimer();
    }
    // Still no baseline: retry while the old process is known to be serving.
    fetchBaseline();

    if (next.status === 'restarting') {
      enterConfirmedRestartPhase();
      return;
    }

    if (next.status === 'error' || (next.status === 'completed' && next.error)) {
      finishWithResult(true);
      return;
    }

    if (next.status === 'completed') {
      // Reported by the old process just before it exits. Reload once the
      // new version is serving.
      restartConfirmed = true;
      resolvingTerminalStatus = true;
      stopStatusFeeds();
      const probeSession = session;
      void attemptReadyReload().then((reloaded) => {
        if (reloaded || probeSession !== session) {
          return;
        }
        resolvingTerminalStatus = false;
        // Backend not on the new version yet — restart in progress.
        enterConfirmedRestartPhase();
      });
      return;
    }

    // Any other live status means the old process is still answering.
    leaveSuspectedRestartPhase();
  };

  // 'idle' is what a freshly started process reports, so it is the restart
  // signal when the stream was held open and never delivered 'completed'.
  // It is also what an update check reports, so it only ends the update
  // once the version has actually moved; otherwise the feeds keep running.
  const handleIdleStatus = (previous: UpdateStatus | null) => {
    const updateWasRunning = previous !== null && isUpdateInProgressStage(previous.status);
    resolvingTerminalStatus = true;
    if (!updateWasRunning) {
      stopStatusFeeds();
    }
    const probeSession = session;
    void attemptReadyReload().then((reloaded) => {
      if (reloaded || probeSession !== session) {
        return;
      }
      resolvingTerminalStatus = false;
      if (!updateWasRunning) {
        finishWithResult(false);
      }
    });
  };

  const setupSSE = () => {
    // Close existing connection if any
    closeSSE();

    try {
      // Create EventSource connection to SSE endpoint
      eventSource = new EventSource('/api/updates/stream');
      // Armed before the connection opens: a request that never connects is
      // as silent as one that connects and then stops delivering.
      armStreamSilenceWatchdog();

      eventSource.onopen = () => {
        logger.info('SSE connection established');
      };

      eventSource.onmessage = (event) => {
        armStreamSilenceWatchdog();
        try {
          handleStatus(JSON.parse(event.data) as UpdateStatus);
        } catch (error) {
          logger.error('Failed to parse SSE update status', error);
        }
      };

      eventSource.onerror = (error) => {
        logger.warn('SSE connection error, falling back to polling', error);
        closeSSE();
        // Fall back to polling
        startPolling();
      };
    } catch (error) {
      logger.error('Failed to setup SSE, falling back to polling', error);
      closeSSE();
      // Fall back to polling
      startPolling();
    }
  };

  const startPolling = () => {
    // Don't start polling if already polling
    if (pollInterval !== undefined || isComplete() || restartConfirmed) {
      return;
    }

    logger.info('Starting update status polling');
    void pollStatus();
    pollInterval = setInterval(pollStatus, UPDATE_STATUS_POLL_INTERVAL_MS) as unknown as number;
  };

  const pollStatus = async () => {
    const pollSession = session;
    try {
      const currentStatus = await UpdatesAPI.getUpdateStatus();
      if (pollSession !== session) {
        return;
      }
      consecutivePollFailures = 0;
      handleStatus(currentStatus);
    } catch (error) {
      if (pollSession !== session || isComplete() || restartConfirmed || resolvingTerminalStatus) {
        return;
      }
      consecutivePollFailures += 1;
      logger.warn('Failed to poll update status, will retry', {
        consecutiveFailures: consecutivePollFailures,
        error,
      });
      // One failed poll proves nothing: a transient network error or a 500
      // during a long download must not end the session. Treat failures as
      // restart evidence only when the stream has also closed, several polls
      // in a row failed, and the update had reached the stage that restarts.
      if (
        !eventSource &&
        consecutivePollFailures >= RESTART_SUSPECT_POLL_FAILURES &&
        isLateUpdateStage(status()?.status)
      ) {
        enterSuspectedRestartPhase();
      }
    }
  };

  const startHealthCheckPolling = () => {
    clearHealthCheckTimer();
    setHealthCheckAttempts(0);
    // clearHealthCheckTimer (also run on close and on leaving a suspected
    // restart) moves the loop id on, retiring this loop even mid-probe.
    const loop = healthCheckLoop;

    const checkHealth = async () => {
      healthCheckTimer = undefined;
      if (loop !== healthCheckLoop) {
        return;
      }
      if (await attemptReadyReload()) {
        return;
      }
      if (loop !== healthCheckLoop) {
        return;
      }

      const attempt = Math.min(healthCheckAttempts(), 3);
      const nextDelay = Math.min(2000 * Math.pow(2, attempt), 15000);
      setHealthCheckAttempts((current) => current + 1);
      healthCheckTimer = window.setTimeout(checkHealth, nextDelay);
    };

    // Start checking immediately
    healthCheckTimer = window.setTimeout(checkHealth, 0);
  };

  // Watch websocket status during restart
  createEffect(() => {
    if (!props.isOpen || !isRestarting()) return;

    const connected = props.connected?.();
    const reconnecting = props.reconnecting?.();

    // Track if websocket disconnected during restart
    if (connected === false && !reconnecting) {
      setWsDisconnected(true);
    }

    // If websocket reconnected after being disconnected, the backend is likely back
    if (wsDisconnected() && connected === true && !reconnecting) {
      logger.info('WebSocket reconnected after restart, verifying health...');
      // Give it a moment for the backend to fully initialize
      const reconnectTimer = window.setTimeout(async () => {
        if (!props.isOpen) return;
        // A reconnected websocket almost certainly means the new process is
        // up; attemptReadyReload still verifies the version before reloading.
        await attemptReadyReload();
      }, 1000);
      onCleanup(() => window.clearTimeout(reconnectTimer));
    }
  });

  // Start/stop SSE or polling based on modal visibility. Only isOpen is
  // tracked: the setup reads other signals (versionInfo, isComplete), and a
  // change in any of them must not tear down and restart a live session.
  createEffect(() => {
    const open = props.isOpen;
    untrack(() => {
      session += 1;
      stopEverything();
      if (open) {
        resetModalState();
        fetchBaseline();
        armProgressStallTimer();
        // Try SSE first; a stream error or prolonged silence adds polling.
        setupSSE();
      }
    });
  });

  // Take the baseline from the store as soon as it loads, if the modal opened
  // first. canAdoptBaseline refuses it once a restart may be under way.
  createEffect(() => {
    const version = updateStore.versionInfo()?.version;
    if (!props.isOpen || !version) {
      return;
    }
    untrack(() => adoptBaseline(version, 'update-store'));
  });

  onCleanup(() => {
    session += 1;
    stopEverything();
  });

  const getStageIcon = () => {
    const currentStatus = status();
    if (!currentStatus) return null;

    if (hasError()) {
      return <AlertTriangleIcon class="h-12 w-12 text-red-500" aria-hidden="true" />;
    }

    if (isComplete() && !hasError()) {
      return <CheckCircleIcon class="h-12 w-12 text-emerald-500" aria-hidden="true" />;
    }

    return <LoadingSpinner size="lg" tone="info" label="Update in progress" />;
  };

  const getStatusText = () => {
    const currentStatus = status();

    if (isRestarting()) {
      return 'Pulse is restarting...';
    }

    if (!currentStatus) return 'Initializing...';

    if (hasError()) {
      return 'Update Failed';
    }

    if (isComplete() && !hasError()) {
      return 'Update Completed Successfully';
    }

    return currentStatus.message || 'Updating...';
  };

  const handleClose = () => {
    props.onClose();
  };

  return (
    <Dialog
      isOpen={props.isOpen}
      onClose={handleClose}
      panelClass="max-w-2xl"
      closeOnBackdrop={true}
      ariaLabel="Updating Pulse"
    >
      <div class="w-full">
        {/* Header */}
        <div class="px-6 py-4 border-b border-border">
          <div class="flex items-center justify-between">
            <h2 class="text-xl font-semibold text-base-content">Updating Pulse</h2>
            <ActionIconButton
              onClick={handleClose}
              label={
                isComplete()
                  ? 'Close update progress'
                  : 'Hide update progress. The update continues server-side.'
              }
              tone="muted"
              size="md"
              type="button"
              title={
                isComplete()
                  ? 'Close update progress'
                  : 'Hide update progress. GlobalUpdateProgressWatcher keeps tracking the server-side update.'
              }
            >
              <XIcon class="h-5 w-5" aria-hidden="true" />
            </ActionIconButton>
          </div>
        </div>

        {/* Body */}
        <div class="px-6 py-8">
          {/* Icon and Status */}
          <div class="flex flex-col items-center text-center space-y-4">
            {getStageIcon()}
            <div>
              <div class="text-lg font-medium text-base-content">{getStatusText()}</div>
              <Show when={status()?.status && !isComplete()}>
                <div class="text-sm text-muted mt-1 capitalize">
                  {status()!.status.replace('-', ' ')}
                </div>
              </Show>
            </div>
          </div>

          {/* Progress Bar */}
          <Show when={!isComplete() && status()?.progress !== undefined}>
            <div class="mt-6">
              <div class="flex items-center justify-between text-sm text-muted mb-2">
                <span>Progress</span>
                <span>{status()!.progress}%</span>
              </div>
              <ProgressBar
                value={status()!.progress}
                class="h-2 rounded-full"
                fillClass="bg-blue-600"
              />
            </div>
          </Show>

          {/* Error Message */}
          <Show when={hasError() && status()?.error}>
            <CalloutCard
              tone="danger"
              scale="compact"
              padding="md"
              class="mt-6"
              icon={<AlertTriangleIcon class="h-5 w-5" aria-hidden="true" />}
              title="Error Details"
              description={<span class="text-sm">{status()!.error}</span>}
            />
          </Show>

          {/* Warning / Info */}
          <Show when={!isComplete()}>
            <Show when={isRestarting()}>
              <CalloutCard
                tone="info"
                scale="compact"
                padding="md"
                class="mt-6"
                icon={<InfoIcon class="h-5 w-5" aria-hidden="true" />}
                description={
                  <Show
                    when={wsDisconnected()}
                    fallback={
                      <span class="text-sm">Pulse is restarting with the new version...</span>
                    }
                  >
                    <span class="text-sm">
                      Waiting for Pulse to complete restart. This page will reload automatically.
                    </span>
                  </Show>
                }
              >
                <Show when={wsDisconnected() && healthCheckAttempts() > 5}>
                  <Button
                    onClick={() => window.location.reload()}
                    variant="primary"
                    size="sm"
                    class="mt-2"
                    type="button"
                  >
                    Reload Now
                  </Button>
                </Show>
              </CalloutCard>
            </Show>
            <Show when={!isRestarting() && !progressStalled()}>
              <CalloutCard
                tone="warning"
                scale="compact"
                padding="md"
                class="mt-6"
                icon={<AlertTriangleIcon class="h-5 w-5" aria-hidden="true" />}
                description={
                  <span class="text-sm">
                    Please do not close this window or refresh the page during the update.
                  </span>
                }
              />
            </Show>
            <Show when={progressStalled()}>
              <CalloutCard
                tone="warning"
                scale="compact"
                padding="md"
                class="mt-6"
                icon={<AlertTriangleIcon class="h-5 w-5" aria-hidden="true" />}
                title="No progress reported for a while"
                description={
                  <span class="text-sm">
                    The update may have finished already. Reload the page to check. Pulse keeps
                    updating on the server either way.
                  </span>
                }
              >
                <Button
                  onClick={() => window.location.reload()}
                  variant="primary"
                  size="sm"
                  class="mt-2"
                  type="button"
                >
                  Reload to check
                </Button>
              </CalloutCard>
            </Show>
          </Show>
        </div>

        {/* Footer */}
        <Show when={isComplete()}>
          <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-end gap-3">
            <Show when={!hasError()}>
              <Button onClick={props.onViewHistory} variant="ghost" size="md" type="button">
                View History
              </Button>
            </Show>
            <Show when={hasError()}>
              <Button
                onClick={() => window.location.reload()}
                variant="primary"
                size="md"
                type="button"
              >
                Retry
              </Button>
            </Show>
            <Button onClick={handleClose} variant="primary" size="md" type="button">
              Close
            </Button>
          </div>
        </Show>
      </div>
    </Dialog>
  );
}
