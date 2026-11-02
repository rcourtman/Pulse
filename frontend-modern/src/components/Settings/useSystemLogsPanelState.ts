import { batch, createEffect, createSignal, onCleanup, onMount, untrack } from 'solid-js';
import { apiFetchJSON, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import {
  SYSTEM_LOG_LEVEL_OPTIONS,
  SYSTEM_LOGS_PANEL_COPY,
  type SystemLogConnectionState,
} from '@/utils/systemLogsPresentation';

const MAX_LOGS = 1000;

export interface SystemLogsPanelProps {
  currentUser?: string;
  accessKey?: string;
  canRead?: boolean;
}

export function useSystemLogsPanelState(props: SystemLogsPanelProps = {}) {
  const [buffer, setBuffer] = createSignal<string[]>([]);
  const [isPaused, setIsPaused] = createSignal(false);
  const [level, setLevel] = createSignal('info');
  const [isLoading, setIsLoading] = createSignal(false);
  const [changingLevel, setChangingLevel] = createSignal(false);
  const [notice, setNotice] = createSignal('');
  const [error, setError] = createSignal('');
  const [connectionState, setConnectionState] =
    createSignal<SystemLogConnectionState>('connecting');

  const reader = () => ({
    org: getOrgID() || 'default',
    user: props.currentUser,
    access: props.accessKey,
    allowed: props.canRead !== false,
  });
  const [context, setContext] = createSignal(reader());
  let logContainer: HTMLDivElement | undefined;
  let eventSource: EventSource | null = null;
  let levelRequest: AbortController | undefined;
  let writeRequest: AbortController | undefined;
  let disposed = false;
  let renderFrame: number | null = null;
  // A bounded ring also caps pending work when a background tab stops frames.
  let pendingLogs: string[] = [];
  let pendingCount = 0;
  let pendingNext = 0;

  const isCurrentReader = () => {
    const current = reader();
    const accepted = context();
    return (
      !disposed &&
      current.allowed &&
      accepted.allowed &&
      current.org === accepted.org &&
      current.user === accepted.user &&
      current.access === accepted.access
    );
  };
  const logs = () => (isCurrentReader() ? buffer() : []);
  const canUseControls = () => isCurrentReader() && connectionState() === 'streaming';

  const resetPending = () => {
    pendingLogs = [];
    pendingCount = 0;
    pendingNext = 0;
    if (renderFrame !== null) {
      cancelAnimationFrame(renderFrame);
      renderFrame = null;
    }
  };
  const clearLogs = () => {
    resetPending();
    setBuffer([]);
  };
  const disconnect = (message = '') => {
    eventSource?.close();
    eventSource = null;
    levelRequest?.abort();
    levelRequest = undefined;
    writeRequest?.abort();
    writeRequest = undefined;
    batch(() => {
      clearLogs();
      setIsLoading(false);
      setChangingLevel(false);
      setConnectionState('disconnected');
      setNotice('');
      setError(message);
    });
  };
  const resetReader = () => {
    disconnect();
    batch(() => {
      setContext(reader());
      setLevel('info');
      setNotice(SYSTEM_LOGS_PANEL_COPY.readerChanged);
    });
    // Reader/workspace changes never reopen a privileged stream automatically.
  };
  createEffect(() => {
    const next = reader();
    const previous = untrack(context);
    if (
      previous.org !== next.org ||
      previous.user !== next.user ||
      previous.access !== next.access ||
      previous.allowed !== next.allowed
    )
      resetReader();
  });
  const unsubscribe = eventBus.on('org_switched', resetReader);

  const setLogContainer = (element: HTMLDivElement | undefined) => {
    logContainer = element;
  };
  const flushPendingLogs = () => {
    if (!canUseControls() || pendingCount === 0) return;
    const incoming = Array.from(
      { length: pendingCount },
      (_, index) => pendingLogs[(pendingNext - pendingCount + MAX_LOGS + index) % MAX_LOGS],
    );
    resetPending();
    setBuffer((prev) => [
      ...prev.slice(Math.max(0, prev.length + incoming.length - MAX_LOGS)),
      ...incoming,
    ]);
    if (logContainer) logContainer.scrollTop = logContainer.scrollHeight;
  };

  const connectStream = (acceptedReader: ReturnType<typeof reader>) => {
    const source = new EventSource('/api/logs/stream');
    eventSource = source;
    const isCurrent = () =>
      isCurrentReader() && context() === acceptedReader && eventSource === source;
    source.onopen = () => {
      if (!isCurrent()) return;
      batch(() => {
        setConnectionState('streaming');
        setNotice('');
        setError('');
      });
    };
    source.onmessage = (event) => {
      if (!isCurrent()) return;
      setConnectionState('streaming');
      if (isPaused()) return;
      pendingLogs[pendingNext] = event.data;
      pendingNext = (pendingNext + 1) % MAX_LOGS;
      pendingCount = Math.min(pendingCount + 1, MAX_LOGS);
      if (renderFrame === null) {
        const frame = requestAnimationFrame(() => {
          if (renderFrame !== frame || !isCurrent()) return;
          renderFrame = null;
          flushPendingLogs();
        });
        renderFrame = frame;
      }
    };
    source.onerror = () => {
      if (!isCurrent()) return;
      // EventSource does not expose the HTTP error or its cause. Withdraw
      // private buffered lines even on transient EOF; keep native reconnects.
      clearLogs();
      if (source.readyState === EventSource.CLOSED) {
        disconnect(SYSTEM_LOGS_PANEL_COPY.connectionError);
      } else {
        batch(() => {
          setConnectionState('reconnecting');
          setNotice(SYSTEM_LOGS_PANEL_COPY.reconnecting);
        });
      }
    };
  };

  const reconnect = async () => {
    if (!isCurrentReader() || eventSource || levelRequest) return;
    const acceptedReader = context();
    const request = new AbortController();
    levelRequest = request;
    const isCurrent = () =>
      isCurrentReader() && context() === acceptedReader && levelRequest === request;
    batch(() => {
      clearLogs();
      setIsLoading(true);
      setConnectionState('connecting');
      setError('');
      setNotice('');
    });
    try {
      const res = await apiFetchJSON<{ level: string }>('/api/logs/level', {
        signal: request.signal,
        expectedOrgID: acceptedReader.org,
        retry: false,
      });
      if (!isCurrent()) return;
      if (!res || !SYSTEM_LOG_LEVEL_OPTIONS.some((option) => option.value === res.level))
        throw new Error('Invalid log level');
      setLevel(res.level);
      connectStream(acceptedReader);
    } catch {
      if (isCurrent()) disconnect(SYSTEM_LOGS_PANEL_COPY.connectionError);
    } finally {
      if (levelRequest === request) {
        levelRequest = undefined;
        setIsLoading(false);
      }
    }
  };

  const handleLevelChange = async (newLevel: string) => {
    if (
      !canUseControls() ||
      writeRequest ||
      !SYSTEM_LOG_LEVEL_OPTIONS.some((option) => option.value === newLevel)
    )
      return;
    const acceptedReader = context();
    const request = new AbortController();
    writeRequest = request;
    const isCurrent = () =>
      canUseControls() && context() === acceptedReader && writeRequest === request;
    setChangingLevel(true);
    setNotice('');
    try {
      await apiFetchJSON('/api/logs/level', {
        method: 'POST',
        body: JSON.stringify({ level: newLevel }),
        signal: request.signal,
        expectedOrgID: acceptedReader.org,
        retry: false,
      });
      if (!isCurrent()) return;
      setLevel(newLevel);
      setNotice(`Log level set to ${newLevel}`);
    } catch {
      if (isCurrent()) disconnect(SYSTEM_LOGS_PANEL_COPY.levelError);
    } finally {
      if (writeRequest === request) {
        writeRequest = undefined;
        setChangingLevel(false);
      }
    }
  };

  const handleDownload = () => {
    if (!canUseControls()) return;
    // A top-level navigation cancels WebKit's live EventSource without an
    // error callback. Use a same-origin download, preserving native streaming
    // of large support archives rather than buffering them in JavaScript.
    const anchor = document.createElement('a');
    anchor.href = '/api/logs/download';
    anchor.download = '';
    document.body.appendChild(anchor);
    try {
      anchor.click();
    } finally {
      anchor.remove();
    }
  };
  const togglePaused = () => {
    if (!isPaused()) flushPendingLogs();
    setIsPaused((prev) => !prev);
  };
  onMount(() => void reconnect());
  onCleanup(() => {
    disposed = true;
    unsubscribe();
    disconnect();
  });

  return {
    canUseControls,
    canReconnect: () => isCurrentReader() && connectionState() === 'disconnected',
    changingLevel,
    clearLogs,
    connectionState,
    error: () => (isCurrentReader() ? error() : ''),
    handleDownload,
    handleLevelChange,
    isLoading,
    isPaused,
    level,
    logs,
    maxLogs: MAX_LOGS,
    notice: () => (isCurrentReader() ? notice() : ''),
    reconnect,
    setLogContainer,
    togglePaused,
  };
}
