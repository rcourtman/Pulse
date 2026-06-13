import { createSignal, Show, onCleanup, createEffect } from 'solid-js';
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
  let pollInterval: number | undefined;
  let healthCheckTimer: number | undefined;
  let eventSource: EventSource | undefined;

  const resetModalState = () => {
    setStatus(null);
    setIsComplete(false);
    setHasError(false);
    setIsRestarting(false);
    setWsDisconnected(false);
    setHealthCheckAttempts(0);
  };

  const clearHealthCheckTimer = () => {
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

  const closeSSE = () => {
    if (!eventSource) {
      return;
    }
    eventSource.close();
    eventSource = undefined;
    logger.info('SSE connection closed');
  };

  const setupSSE = () => {
    // Close existing connection if any
    closeSSE();

    try {
      // Create EventSource connection to SSE endpoint
      eventSource = new EventSource('/api/updates/stream');

      eventSource.onopen = () => {
        logger.info('SSE connection established');
      };

      eventSource.onmessage = (event) => {
        try {
          const updateStatus = JSON.parse(event.data) as UpdateStatus;
          setStatus(updateStatus);

          // Check if restarting
          if (updateStatus.status === 'restarting') {
            setIsRestarting(true);
            closeSSE();
            startHealthCheckPolling();
            return;
          }

          // Check if complete or error
          if (
            updateStatus.status === 'completed' ||
            updateStatus.status === 'idle' ||
            updateStatus.status === 'error'
          ) {
            if (updateStatus.status === 'completed' && !updateStatus.error) {
              closeSSE();
              // Verify backend health and reload
              apiFetch('/api/health', { cache: 'no-store' })
                .then((healthCheck) => {
                  if (healthCheck.ok) {
                    logger.info('Update completed, backend healthy, reloading...');
                    window.location.reload();
                  } else {
                    // Health check failed, assume restart in progress
                    setIsRestarting(true);
                    startHealthCheckPolling();
                  }
                })
                .catch((error) => {
                  logger.warn(
                    'Update completed but health check failed, assuming restart...',
                    error,
                  );
                  setIsRestarting(true);
                  startHealthCheckPolling();
                });
              return;
            }

            setIsComplete(true);
            if (updateStatus.status === 'error' || updateStatus.error) {
              setHasError(true);
            }
            closeSSE();
          }
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
    if (pollInterval !== undefined) {
      return;
    }

    logger.info('Starting status polling (SSE not available)');
    pollStatus();
    pollInterval = setInterval(pollStatus, 2000) as unknown as number;
  };

  const pollStatus = async () => {
    try {
      const currentStatus = await UpdatesAPI.getUpdateStatus();
      setStatus(currentStatus);

      // Check if restarting
      if (currentStatus.status === 'restarting') {
        setIsRestarting(true);
        clearPollInterval();
        // Start health check polling
        startHealthCheckPolling();
        return;
      }

      // Check if complete or error
      if (
        currentStatus.status === 'completed' ||
        currentStatus.status === 'idle' ||
        currentStatus.status === 'error'
      ) {
        // If completed successfully, verify backend health and reload to get new version
        if (currentStatus.status === 'completed' && !currentStatus.error) {
          clearPollInterval();
          // Verify backend is healthy and reload
          try {
            const healthCheck = await apiFetch('/api/health', { cache: 'no-store' });
            if (healthCheck.ok) {
              logger.info('Update completed, backend healthy, reloading...');
              window.location.reload();
              return;
            }
          } catch (error) {
            logger.warn('Update completed but health check failed, assuming restart...', error);
          }
          // If health check failed, assume restart in progress
          setIsRestarting(true);
          startHealthCheckPolling();
          return;
        }

        setIsComplete(true);
        if (currentStatus.status === 'error' || currentStatus.error) {
          setHasError(true);
        }
        clearPollInterval();
      }
    } catch (error) {
      logger.error('Failed to poll update status', error);
      // If we get errors during update, assume we're restarting
      const currentStatus = status();
      const shouldAssumeRestart =
        !isRestarting() &&
        (!currentStatus || (currentStatus.status !== 'idle' && currentStatus.status !== 'error'));

      if (shouldAssumeRestart) {
        if (!currentStatus) {
          setStatus({
            status: 'restarting',
            progress: 95,
            message: 'Restarting service...',
            updatedAt: new Date().toISOString(),
          });
        }
        setIsRestarting(true);
        clearPollInterval();
        startHealthCheckPolling();
      }
    }
  };

  const startHealthCheckPolling = () => {
    clearHealthCheckTimer();
    setHealthCheckAttempts(0);

    const checkHealth = async () => {
      let isHealthy = false;

      try {
        const response = await apiFetch('/api/health', { cache: 'no-store' });
        if (response.ok) {
          isHealthy = true;
        }
      } catch (error) {
        logger.warn('Health check request failed, will retry', error);
      }

      if (isHealthy) {
        // Backend is back! Reload the page to get the new version
        logger.info('Backend is healthy again, reloading...');
        window.location.reload();
        return;
      }

      const attempt = Math.min(healthCheckAttempts(), 3);
      const nextDelay = Math.min(2000 * Math.pow(2, attempt), 15000);
      setHealthCheckAttempts((current) => current + 1);
      clearHealthCheckTimer();
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
        try {
          const response = await apiFetch('/api/health', { cache: 'no-store' });
          if (response.ok) {
            logger.info('Backend healthy after websocket reconnect, reloading...');
            window.location.reload();
          }
        } catch (_error) {
          logger.warn('Health check failed after websocket reconnect, will keep trying');
        }
      }, 1000);
      onCleanup(() => window.clearTimeout(reconnectTimer));
    }
  });

  // Start/stop SSE or polling based on modal visibility
  createEffect(() => {
    if (props.isOpen) {
      resetModalState();
      // Try SSE first, will fall back to polling if it fails
      setupSSE();
    } else {
      // Stop everything when modal closes
      closeSSE();
      clearPollInterval();
      clearHealthCheckTimer();
    }
  });

  onCleanup(() => {
    closeSSE();
    clearPollInterval();
    clearHealthCheckTimer();
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
            <Show when={!isRestarting()}>
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
