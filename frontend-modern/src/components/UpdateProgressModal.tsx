import { createSignal, Show, onCleanup, createEffect } from 'solid-js';
import { UpdatesAPI, type UpdateStatus } from '@/api/updates';
import { apiFetch } from '@/utils/apiClient';
import { logger } from '@/utils/logger';

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
                  logger.warn('Update completed but health check failed, assuming restart...', error);
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
      return (
        <svg class="w-12 h-12 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      );
    }

    if (isComplete() && !hasError()) {
      return (
        <svg class="w-12 h-12 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      );
    }

    return (
      <svg class="w-12 h-12 text-blue-500 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
    );
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

 return (
 <Show when={props.isOpen}>
 <div class="fixed inset-0 bg-black flex items-center justify-center z-50 p-4">
 <div class="bg-surface rounded-md shadow-sm max-w-2xl w-full">
 {/* Header */}
 <div class="px-6 py-4 border-b border-border">
 <div class="flex items-center justify-between">
 <h2 class="text-xl font-semibold text-base-content">
 Updating Pulse
 </h2>
 <Show when={isComplete()}>
 <button
 onClick={props.onClose}
 class=" hover: dark:hover:text-slate-300"
 >
 <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
 </svg>
 </button>
 </Show>
 </div>
 </div>

 {/* Body */}
 <div class="px-6 py-8">
 {/* Icon and Status */}
 <div class="flex flex-col items-center text-center space-y-4">
 {getStageIcon()}
 <div>
 <div class="text-lg font-medium text-base-content">
 {getStatusText()}
 </div>
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
                <div class="w-full bg-surface-hover rounded-full h-2">
                  <div
                    class="bg-blue-600 h-2 rounded-full transition-all duration-300"
                    style={{ width: `${status()!.progress}%` }}
                  />
                </div>
              </div>
            </Show>

            {/* Error Message */}
            <Show when={hasError() && status()?.error}>
              <div class="mt-6 bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded-md p-4">
                <div class="text-sm text-red-800 dark:text-red-200">
                  <div class="font-medium mb-1">Error Details:</div>
                  <div class="text-red-700 dark:text-red-300">{status()!.error}</div>
                </div>
              </div>
            </Show>

            {/* Warning / Info */}
            <Show when={!isComplete()}>
              <Show when={isRestarting()}>
                <div class="mt-6 bg-blue-50 dark:bg-blue-900 border border-blue-200 dark:border-blue-800 rounded-md p-3">
                  <div class="flex items-start gap-2">
                    <svg class="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <div class="flex-1">
                      <div class="text-sm text-blue-800 dark:text-blue-200">
                        <Show when={wsDisconnected()} fallback={
                          <span>Pulse is restarting with the new version...</span>
                        }>
                          <span>Waiting for Pulse to complete restart. This page will reload automatically.</span>
                        </Show>
                      </div>
                      <Show when={wsDisconnected() && healthCheckAttempts() > 5}>
                        <button
                          onClick={() => window.location.reload()}
                          class="mt-2 px-3 py-1.5 text-xs font-medium text-white bg-blue-600 hover:bg-blue-700 rounded transition-colors"
                        >
                          Reload Now
                        </button>
                      </Show>
                    </div>
                  </div>
                </div>
              </Show>
              <Show when={!isRestarting()}>
                <div class="mt-6 bg-yellow-50 dark:bg-yellow-900 border border-yellow-200 dark:border-yellow-800 rounded-md p-3">
                  <div class="flex items-start gap-2">
                    <svg class="w-5 h-5 text-yellow-600 dark:text-yellow-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                    </svg>
                    <div class="text-sm text-yellow-800 dark:text-yellow-200">
                      Please do not close this window or refresh the page during the update.
                    </div>
                  </div>
                </div>
              </Show>
            </Show>
          </div>

          {/* Footer */}
          <Show when={isComplete()}>
            <div class="px-6 py-4 bg-surface-alt border-t border-border flex items-center justify-end gap-3">
              <Show when={!hasError()}>
                <button
                  onClick={props.onViewHistory}
                  class="px-4 py-2 text-sm font-medium text-base-content hover:bg-surface-hover rounded-md transition-colors"
                >
                  View History
                </button>
              </Show>
              <Show when={hasError()}>
                <button
                  onClick={() => window.location.reload()}
                  class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors"
                >
                  Retry
                </button>
              </Show>
              <button
                onClick={props.onClose}
                class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors"
              >
                Close
              </button>
            </div>
          </Show>
        </div>
      </div>
    </Show>
  );
}
