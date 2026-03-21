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

  const setLogContainer = (element: HTMLDivElement | undefined) => {
    logContainer = element;
  };

  const scrollToBottom = () => {
    if (logContainer) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
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

      setLogs((prev) => {
        const next = [...prev, event.data];
        if (next.length > MAX_LOGS) {
          return next.slice(next.length - MAX_LOGS);
        }
        return next;
      });

      scrollToBottom();
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
    setIsPaused((prev) => !prev);
  };

  const clearLogs = () => {
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
