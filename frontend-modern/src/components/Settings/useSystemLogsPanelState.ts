import { createSignal, onCleanup, onMount } from 'solid-js';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';

const MAX_LOGS = 1000;

export function useSystemLogsPanelState() {
  const [logs, setLogs] = createSignal<string[]>([]);
  const [isPaused, setIsPaused] = createSignal(false);
  const [level, setLevel] = createSignal('info');
  const [isLoading, setIsLoading] = createSignal(true);

  let logContainer: HTMLDivElement | undefined;
  let eventSource: EventSource | null = null;
  let disposed = false;
  let renderFrame: number | null = null;
  // A bounded ring also caps pending work when a background tab stops frames.
  // Receiving a line never clones the visible buffer or forces layout.
  let pendingLogs: string[] = [];
  let pendingCount = 0;
  let pendingNext = 0;

  const resetPending = () => {
    pendingLogs = [];
    pendingCount = 0;
    pendingNext = 0;
    if (renderFrame !== null) {
      cancelAnimationFrame(renderFrame);
      renderFrame = null;
    }
  };

  const setLogContainer = (element: HTMLDivElement | undefined) => {
    logContainer = element;
  };

  const scrollToBottom = () => {
    if (logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  };

  const flushPendingLogs = () => {
    if (disposed || pendingCount === 0) return;
    const incoming = Array.from(
      { length: pendingCount },
      (_, index) => pendingLogs[(pendingNext - pendingCount + MAX_LOGS + index) % MAX_LOGS],
    );
    resetPending();
    setLogs((prev) => [
      ...prev.slice(Math.max(0, prev.length + incoming.length - MAX_LOGS)),
      ...incoming,
    ]);
    // Solid updates the rows synchronously; measure after the single publish.
    scrollToBottom();
  };

  const fetchLevel = async () => {
    try {
      const res = (await apiFetchJSON('/api/logs/level')) as { level?: string };
      if (res.level && !disposed) setLevel(res.level);
    } catch (error) {
      logger.error('Failed to fetch log level', error);
    }
  };

  const connectStream = () => {
    if (disposed) return;

    eventSource = new EventSource('/api/logs/stream');

    eventSource.onmessage = (event) => {
      if (disposed || isPaused()) return;

      pendingLogs[pendingNext] = event.data;
      pendingNext = (pendingNext + 1) % MAX_LOGS;
      pendingCount = Math.min(pendingCount + 1, MAX_LOGS);
      if (renderFrame === null) {
        renderFrame = requestAnimationFrame(() => {
          renderFrame = null;
          flushPendingLogs();
        });
      }
    };

    eventSource.onerror = () => {
      if (disposed) return;
      logger.debug('SSE stream disconnected, reconnecting...');
    };
  };

  const handleLevelChange = async (newLevel: string) => {
    try {
      await apiFetchJSON('/api/logs/level', {
        method: 'POST',
        body: JSON.stringify({ level: newLevel }),
      });
      setLevel(newLevel);
      notificationStore.success(`Log level set to ${newLevel}`);
    } catch (error) {
      logger.error('Error setting log level', error);
      notificationStore.error('Failed to set log level');
    }
  };

  const handleDownload = () => {
    window.location.href = '/api/logs/download';
  };

  const togglePaused = () => {
    // Lines accepted before Pause belong in the view; paused arrivals do not.
    if (!isPaused()) flushPendingLogs();
    setIsPaused((prev) => !prev);
  };

  const clearLogs = () => {
    resetPending();
    setLogs([]);
  };

  onMount(() => {
    void (async () => {
      await fetchLevel();
      if (disposed) return;
      connectStream();
      setIsLoading(false);
    })();
  });

  onCleanup(() => {
    disposed = true;
    resetPending();
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
  });

  return {
    clearLogs,
    handleDownload,
    handleLevelChange,
    isLoading,
    isPaused,
    level,
    logs,
    maxLogs: MAX_LOGS,
    setLogContainer,
    togglePaused,
  };
}
