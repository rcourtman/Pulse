import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from 'solid-js';
import type { StorageSummaryChartsResponse, TimeRange } from '@/api/charts';
import type { SummaryTimeRange } from '@/components/shared/summaryTimeRange';
import { eventBus } from '@/stores/events';
import {
  fetchStorageSummaryAndCache,
  readStorageSummaryCache,
} from '@/utils/storageSummaryCache';

const POLL_INTERVAL_MS = 30_000;

type UseStorageSummaryChartsOptions = {
  timeRange: Accessor<SummaryTimeRange>;
  nodeId?: Accessor<string | null | undefined>;
  caller?: string;
};

export const useStorageSummaryCharts = (options: UseStorageSummaryChartsOptions) => {
  const [data, setData] = createSignal<StorageSummaryChartsResponse | null>(null);
  const [loaded, setLoaded] = createSignal(false);
  const [fetchFailed, setFetchFailed] = createSignal(false);
  const [orgVersion, setOrgVersion] = createSignal(0);

  const unsubscribeOrgSwitch = eventBus.on('org_switched', () => {
    setOrgVersion((value) => value + 1);
  });

  let activeFetchController: AbortController | null = null;
  let activeFetchRequest = 0;
  let refreshTimer: ReturnType<typeof setInterval> | undefined;

  const selectedRange = createMemo<TimeRange>(() => (options.timeRange() as TimeRange) || '1h');
  const selectedNodeId = createMemo(() => {
    const raw = options.nodeId?.()?.trim();
    return raw && raw !== 'all' ? raw : undefined;
  });

  const awaitAbortable = <T,>(promise: Promise<T>, signal: AbortSignal): Promise<T> => {
    if (signal.aborted) {
      return Promise.reject(new DOMException('Aborted', 'AbortError'));
    }
    return new Promise<T>((resolve, reject) => {
      const onAbort = () => reject(new DOMException('Aborted', 'AbortError'));
      signal.addEventListener('abort', onAbort, { once: true });
      promise.then(
        (value) => {
          signal.removeEventListener('abort', onAbort);
          resolve(value);
        },
        (error) => {
          signal.removeEventListener('abort', onAbort);
          reject(error);
        },
      );
    });
  };

  const fetchData = async (options_: { prioritize?: boolean } = {}) => {
    const prioritize = options_.prioritize === true;
    if (activeFetchController && !prioritize) return;
    if (activeFetchController && prioritize) {
      activeFetchController.abort();
    }

    const requestedRange = selectedRange();
    const requestedNodeId = selectedNodeId();
    const controller = new AbortController();
    const requestId = ++activeFetchRequest;
    activeFetchController = controller;

    try {
      const response = await awaitAbortable(
        fetchStorageSummaryAndCache(requestedRange, {
          caller: options.caller ?? 'useStorageSummaryCharts',
          nodeId: requestedNodeId,
        }),
        controller.signal,
      );
      if (requestId !== activeFetchRequest) return;
      setData(response);
      setFetchFailed(false);
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      if (requestId !== activeFetchRequest) return;
      setFetchFailed(true);
      const cached = readStorageSummaryCache(requestedRange, requestedNodeId);
      if (cached) {
        setData(cached);
      }
    } finally {
      if (activeFetchController === controller) {
        activeFetchController = null;
      }
      if (requestId === activeFetchRequest) {
        setLoaded(true);
      }
    }
  };

  createEffect(() => {
    const range = selectedRange();
    const nodeId = selectedNodeId();
    const _org = orgVersion();
    void _org;

    if (refreshTimer) {
      clearInterval(refreshTimer);
      refreshTimer = undefined;
    }

    const cached = readStorageSummaryCache(range, nodeId);
    if (cached) {
      setData(cached);
      setLoaded(true);
    } else {
      setData(null);
      setLoaded(false);
    }
    setFetchFailed(false);

    refreshTimer = setInterval(() => void fetchData(), POLL_INTERVAL_MS);
    void fetchData({ prioritize: true });

    onCleanup(() => {
      if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = undefined;
      }
    });
  });

  onCleanup(() => {
    activeFetchController?.abort();
    unsubscribeOrgSwitch();
  });

  return {
    data,
    loaded,
    fetchFailed,
  };
};

export default useStorageSummaryCharts;
