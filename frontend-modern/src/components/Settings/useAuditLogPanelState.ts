import { createEffect, createMemo, createSignal, onCleanup, onMount, untrack } from 'solid-js';
import type { Accessor } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageNumberSignal,
  STORAGE_KEYS,
} from '@/utils/localStorage';
import { apiErrorFromResponse, apiFetch } from '@/utils/apiClient';
import { showSuccess, showToast, showWarning } from '@/utils/toast';
import { hasFeature, runtimeCapabilitiesLoaded } from '@/stores/license';
import { getUpgradeActionDestination } from '@/stores/licenseCommercial';
import { presentationPolicyHidesUpgradePrompts } from '@/stores/sessionPresentationPolicy';
import { getRuntimeCapabilityBlock, loadRuntimeCapabilities } from '@/stores/license';
import { resolveUpgradeDestination } from '@/utils/upgradeNavigation';
import { getAuditLogFetchErrorMessage } from '@/utils/auditLogPresentation';

export interface AuditEvent {
  id: string;
  timestamp: string;
  event: string;
  user: string;
  ip: string;
  path: string;
  success: boolean;
  details: string;
  signature?: string;
}

interface AuditResponse {
  events: AuditEvent[];
  total: number;
  persistentLogging: boolean;
}

interface VerifyResponse {
  available: boolean;
  verified?: boolean;
  message?: string;
}

type VerificationStatus = 'verified' | 'failed' | 'unavailable' | 'error';

export type VerificationState = {
  status: VerificationStatus;
  message: string;
};

type AuditFilterChipKey = 'event' | 'user' | 'success' | 'verification';

type AuditFilterChip = {
  label: string;
  key: AuditFilterChipKey;
};

type VerifyAllOptions = {
  limit?: number;
  showToast?: boolean;
  resume?: boolean;
};

const ALLOWED_VERIFICATION_FILTERS = new Set(['all', 'needs', 'verified', 'failed']);
const ALLOWED_SUCCESS_FILTERS = new Set(['all', 'success', 'failed']);
const ALLOWED_EVENT_FILTERS = new Set(['', 'login', 'logout', 'config_change', 'startup']);
const USER_FILTER_DEBOUNCE_MS = 300;
const DEFAULT_AUDIT_PAGE_SIZE = 100;
const ALLOWED_AUDIT_PAGE_SIZES = new Set([25, 50, 100, 200]);

const parseEventFilter = (raw: string | null | undefined): string =>
  ALLOWED_EVENT_FILTERS.has(raw ?? '') ? (raw ?? '') : '';
const parseSuccessFilter = (raw: string | null | undefined): string =>
  ALLOWED_SUCCESS_FILTERS.has(raw ?? '') ? (raw ?? 'all') : 'all';
const parseVerificationFilter = (raw: string | null | undefined): string =>
  ALLOWED_VERIFICATION_FILTERS.has(raw ?? '') ? (raw ?? 'all') : 'all';
const normalizeAuditPageSize = (value: number): number =>
  ALLOWED_AUDIT_PAGE_SIZES.has(value) ? value : DEFAULT_AUDIT_PAGE_SIZE;

export const useAuditLogPanelState = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const [events, setEvents] = createSignal<AuditEvent[]>([]);
  const [totalEvents, setTotalEvents] = createSignal(0);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);
  const [isPersistent, setIsPersistent] = createSignal(false);

  const updateSearchParam = (mutate: (params: URLSearchParams) => void): void => {
    const params = new URLSearchParams(location.search);
    mutate(params);
    const query = params.toString();
    navigate(`${location.pathname}${query ? `?${query}` : ''}`, { replace: true });
  };

  const eventFilter: Accessor<string> = () =>
    parseEventFilter(new URLSearchParams(location.search).get('event'));
  const setEventFilter = (value: string): void => {
    updateSearchParam((params) => {
      if (parseEventFilter(value) === '') params.delete('event');
      else params.set('event', value);
    });
  };
  const userFilter: Accessor<string> = () => new URLSearchParams(location.search).get('user') ?? '';
  const setUserFilter = (value: string): void => {
    updateSearchParam((params) => {
      if (value === '') params.delete('user');
      else params.set('user', value);
    });
  };
  const successFilter: Accessor<string> = () =>
    parseSuccessFilter(new URLSearchParams(location.search).get('success'));
  const setSuccessFilter = (value: string): void => {
    updateSearchParam((params) => {
      if (parseSuccessFilter(value) === 'all') params.delete('success');
      else params.set('success', value);
    });
  };
  const verificationFilter: Accessor<string> = () =>
    parseVerificationFilter(new URLSearchParams(location.search).get('verification'));
  const setVerificationFilter = (value: string): void => {
    updateSearchParam((params) => {
      if (parseVerificationFilter(value) === 'all') params.delete('verification');
      else params.set('verification', value);
    });
  };
  const [storedPageSize, setStoredPageSize] = createLocalStorageNumberSignal(
    STORAGE_KEYS.AUDIT_PAGE_SIZE,
    DEFAULT_AUDIT_PAGE_SIZE,
  );
  const pageSize = () => normalizeAuditPageSize(storedPageSize());
  const setPageSize = (value: number): void => {
    setStoredPageSize(normalizeAuditPageSize(value));
  };
  const [pageOffset, setPageOffset] = createLocalStorageNumberSignal(
    STORAGE_KEYS.AUDIT_PAGE_OFFSET,
    0,
  );
  const [verification, setVerification] = createSignal<Record<string, VerificationState>>({});
  const [verifying, setVerifying] = createSignal<Record<string, boolean>>({});
  const [verifyingAll, setVerifyingAll] = createSignal(false);
  const [verifyAllTotal, setVerifyAllTotal] = createSignal(0);
  const [verifyAllDone, setVerifyAllDone] = createSignal(0);
  const [autoVerifyEnabled, setAutoVerifyEnabled] = createLocalStorageBooleanSignal(
    STORAGE_KEYS.AUDIT_AUTO_VERIFY,
    true,
  );
  const [autoVerifyLimit, setAutoVerifyLimit] = createLocalStorageNumberSignal(
    STORAGE_KEYS.AUDIT_AUTO_VERIFY_LIMIT,
    50,
  );
  const [pageInput, setPageInput] = createSignal('');
  const [isMounted, setIsMounted] = createSignal(false);
  const [cancelVerifyAll, setCancelVerifyAll] = createSignal(false);
  const [verifyCanceled, setVerifyCanceled] = createSignal(false);
  const [verifyControllers, setVerifyControllers] = createSignal<Record<string, AbortController>>(
    {},
  );

  const auditLoggingEnabled = createMemo(
    () => runtimeCapabilitiesLoaded() && hasFeature('audit_logging'),
  );
  const auditLoggingRuntimeBlock = createMemo(() => getRuntimeCapabilityBlock('audit_logging'));
  const paidRuntimeRequired = createMemo(
    () => auditLoggingRuntimeBlock()?.reason === 'paid_runtime_required',
  );
  const showUpgradePrompts = () =>
    !paidRuntimeRequired() && !presentationPolicyHidesUpgradePrompts();
  const showFeatureGateAction = () => paidRuntimeRequired() || showUpgradePrompts();
  const showUpgradePaywall = createMemo(
    () => runtimeCapabilitiesLoaded() && !auditLoggingEnabled() && !loading(),
  );
  const upgradeDestination = createMemo(() =>
    paidRuntimeRequired()
      ? resolveUpgradeDestination(auditLoggingRuntimeBlock()?.action_url)
      : getUpgradeActionDestination('audit_logging'),
  );
  const upgradeActionLabel = () => (paidRuntimeRequired() ? 'Download Pulse Pro' : 'View plans');

  const fetchAuditEvents = async (options?: { limit?: number; offset?: number }) => {
    if (!auditLoggingEnabled()) {
      setEvents([]);
      setTotalEvents(0);
      setIsPersistent(false);
      setError(null);
      setLoading(false);
      return;
    }

    const limit = options?.limit ?? pageSize();
    const offset = options?.offset ?? pageOffset();

    setLoading(true);
    setError(null);
    setVerification({});
    setVerifying({});
    setVerifyAllTotal(0);
    setVerifyAllDone(0);

    try {
      const params = new URLSearchParams();
      params.set('limit', String(limit));
      params.set('offset', String(Math.max(0, offset)));
      if (eventFilter()) params.set('event', eventFilter());
      if (userFilter()) params.set('user', userFilter());
      if (successFilter() === 'success') params.set('success', 'true');
      if (successFilter() === 'failed') params.set('success', 'false');

      const response = await apiFetch(`/api/audit?${params.toString()}`);
      if (response.status === 402) {
        setEvents([]);
        setTotalEvents(0);
        setIsPersistent(false);
        setError(null);
        return;
      }
      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `Failed to fetch audit events: ${response.statusText}`,
        );
      }

      const data: AuditResponse = await response.json();
      setEvents(data.events || []);
      setIsPersistent(data.persistentLogging);
      setTotalEvents(data.total ?? 0);

      if (data.total && offset >= data.total) {
        const maxOffset = Math.max(0, Math.floor((data.total - 1) / limit) * limit);
        if (maxOffset !== offset) {
          setPageOffset(maxOffset);
          void fetchAuditEvents({ limit, offset: maxOffset });
          return;
        }
      }

      if (data.persistentLogging && autoVerifyEnabled()) {
        const verificationLimit = autoVerifyLimit();
        if (verificationLimit <= 0) return;
        setTimeout(() => {
          if (!isMounted()) return;
          void verifyAllEvents({ limit: verificationLimit, showToast: false });
        }, 0);
      }
    } catch (err) {
      const message = getAuditLogFetchErrorMessage(err);
      if (typeof message === 'string' && /feature not included in license/i.test(message)) {
        setEvents([]);
        setTotalEvents(0);
        setIsPersistent(false);
        setError(null);
        return;
      }
      setError(message);
      showWarning('Audit Log Error', message);
    } finally {
      setLoading(false);
    }
  };

  const verifyEvent = async (event: AuditEvent) => {
    if (!auditLoggingEnabled() || !event.signature) return;

    setVerifying((prev) => ({ ...prev, [event.id]: true }));

    try {
      const controller = new AbortController();
      setVerifyControllers((prev) => ({ ...prev, [event.id]: controller }));
      if (!isMounted()) {
        controller.abort();
      }

      const response = await apiFetch(`/api/audit/${event.id}/verify`, {
        signal: controller.signal,
      });
      if (!response.ok) {
        throw new Error(`Failed to verify signature: ${response.statusText}`);
      }

      const data: VerifyResponse = await response.json();
      let status: VerificationStatus = 'unavailable';
      if (!data.available) {
        status = 'unavailable';
      } else if (data.verified) {
        status = 'verified';
      } else {
        status = 'failed';
      }

      setVerification((prev) => ({
        ...prev,
        [event.id]: { status, message: data.message || '' },
      }));
    } catch (err) {
      if ((err as { name?: string })?.name === 'AbortError') {
        return;
      }

      setVerification((prev) => ({
        ...prev,
        [event.id]: {
          status: 'error',
          message: err instanceof Error ? err.message : 'Unknown error',
        },
      }));
    } finally {
      setVerifying((prev) => ({ ...prev, [event.id]: false }));
      setVerifyControllers((prev) => {
        const next = { ...prev };
        delete next[event.id];
        return next;
      });
    }
  };

  const verifyAllEvents = async (options?: VerifyAllOptions) => {
    if (!auditLoggingEnabled()) return;

    const limit = options?.limit;
    let signedEvents = events().filter((event) => event.signature);
    if (options?.resume) {
      signedEvents = signedEvents.filter((event) => {
        const state = verification()[event.id];
        return !state || state.status === 'failed' || state.status === 'error';
      });
    }
    if (limit !== undefined) {
      signedEvents = signedEvents.slice(0, Math.max(0, limit));
    }
    if (signedEvents.length === 0) return;

    setVerifyingAll(true);
    setVerifyCanceled(false);
    setCancelVerifyAll(false);
    setVerifyAllTotal(signedEvents.length);
    setVerifyAllDone(0);

    for (const event of signedEvents) {
      if (cancelVerifyAll()) {
        setVerifyingAll(false);
        setVerifyCanceled(true);
        if (options?.showToast) {
          showToast('info', 'Signature verification canceled');
        }
        for (const controller of Object.values(verifyControllers())) {
          controller.abort();
        }
        return;
      }

      await verifyEvent(event);
      setVerifyAllDone((prev) => prev + 1);
    }

    setVerifyingAll(false);
    setVerifyCanceled(false);

    if (options?.showToast) {
      let verified = 0;
      let failed = 0;
      let errors = 0;
      let unavailable = 0;

      for (const event of signedEvents) {
        const state = verification()[event.id];
        if (!state) continue;
        switch (state.status) {
          case 'verified':
            verified += 1;
            break;
          case 'failed':
            failed += 1;
            break;
          case 'error':
            errors += 1;
            break;
          case 'unavailable':
            unavailable += 1;
            break;
          default:
            break;
        }
      }

      if (failed > 0 || errors > 0) {
        showWarning(
          'Signature verification completed',
          `Verified ${verified}, failed ${failed}, errors ${errors}, unavailable ${unavailable}.`,
        );
      } else {
        showSuccess('Signature verification completed', `Verified ${verified} events.`);
      }
    }
  };

  const hasSignedEvents = () => events().some((event) => event.signature);
  const hasResumeEvents = () =>
    events().some((event) => {
      if (!event.signature) return false;
      const state = verification()[event.id];
      return !state || state.status === 'failed' || state.status === 'error';
    });
  const resumeCount = () =>
    events().filter((event) => {
      if (!event.signature) return false;
      const state = verification()[event.id];
      return !state || state.status === 'failed' || state.status === 'error';
    }).length;
  const verifyAllLabel = () => {
    if (!verifyingAll()) return 'Verify All';
    if (verifyAllTotal() === 0) return 'Verifying…';
    return `Verifying ${verifyAllDone()} of ${verifyAllTotal()}`;
  };
  const hasNextPage = () => pageNumber() < totalPages();
  const pageNumber = () => Math.floor(pageOffset() / pageSize()) + 1;
  const totalPages = () => Math.max(1, Math.ceil(totalEvents() / pageSize()));
  const pageRangeText = () => {
    if (totalEvents() === 0) return 'Showing 0 of 0';
    const start = pageOffset() + 1;
    const end = Math.min(totalEvents(), pageOffset() + events().length);
    return `Showing ${start}-${end} of ${totalEvents()}`;
  };

  const verificationSummary = createMemo(() => {
    const summary = {
      total: events().length,
      signed: 0,
      verified: 0,
      failed: 0,
      error: 0,
      unavailable: 0,
      unchecked: 0,
    };

    for (const event of events()) {
      if (!event.signature) continue;
      summary.signed += 1;
      const state = verification()[event.id];
      if (!state) {
        summary.unchecked += 1;
        continue;
      }
      switch (state.status) {
        case 'verified':
          summary.verified += 1;
          break;
        case 'failed':
          summary.failed += 1;
          break;
        case 'error':
          summary.error += 1;
          break;
        case 'unavailable':
          summary.unavailable += 1;
          break;
        default:
          summary.unchecked += 1;
      }
    }

    return summary;
  });

  const activeFilterCount = () => {
    let count = 0;
    if (eventFilter()) count += 1;
    if (userFilter()) count += 1;
    if (successFilter() !== 'all') count += 1;
    if (verificationFilter() !== 'all') count += 1;
    return count;
  };

  const activeFilterChips = createMemo<AuditFilterChip[]>(() => {
    const chips: AuditFilterChip[] = [];
    if (eventFilter()) chips.push({ label: `Event: ${eventFilter()}`, key: 'event' });
    if (userFilter()) chips.push({ label: `User: ${userFilter()}`, key: 'user' });
    if (successFilter() !== 'all') {
      chips.push({ label: `Success: ${successFilter()}`, key: 'success' });
    }
    if (verificationFilter() !== 'all') {
      chips.push({ label: `Verification: ${verificationFilter()}`, key: 'verification' });
    }
    return chips;
  });

  const filteredEvents = createMemo(() => {
    const filter = verificationFilter();
    if (filter === 'all') return events();
    return events().filter((event) => {
      if (!event.signature) return false;
      const state = verification()[event.id];
      if (!state) {
        return filter === 'needs';
      }
      if (filter === 'verified') return state.status === 'verified';
      if (filter === 'failed') return state.status === 'failed' || state.status === 'error';
      return false;
    });
  });

  const resetPaging = () => {
    setPageOffset(0);
    setPageInput('');
  };

  const refetchFromFirstPage = () => {
    resetPaging();
    void fetchAuditEvents({ offset: 0 });
  };

  const clearFilters = () => {
    const hadFilters = activeFilterCount() > 0;
    setEventFilter('');
    setUserFilter('');
    setSuccessFilter('all');
    setVerificationFilter('all');
    if (hadFilters) {
      showSuccess('Audit filters cleared');
    }
  };

  const clearFilterChip = (key: AuditFilterChipKey) => {
    if (key === 'event') setEventFilter('');
    if (key === 'user') setUserFilter('');
    if (key === 'success') setSuccessFilter('all');
    if (key === 'verification') setVerificationFilter('all');
  };

  const resetPreferences = () => {
    setAutoVerifyEnabled(true);
    setAutoVerifyLimit(50);
    setPageSize(100);
    resetPaging();
    showSuccess('Audit preferences reset');
  };

  const goToOffset = (offset: number) => {
    const maxOffset = Math.max(0, (totalPages() - 1) * pageSize());
    const nextOffset = Math.min(maxOffset, Math.max(0, offset));
    setPageOffset(nextOffset);
    void fetchAuditEvents({ offset: nextOffset });
  };

  const submitPageInput = () => {
    const parsed = Number(pageInput());
    if (!Number.isFinite(parsed)) return;
    const clamped = Math.max(1, Math.min(totalPages(), Math.floor(parsed)));
    goToOffset((clamped - 1) * pageSize());
  };

  const refresh = () => {
    void fetchAuditEvents();
  };

  const verifyAll = () => {
    void verifyAllEvents({ showToast: true });
  };

  const resumeVerification = () => {
    void verifyAllEvents({ showToast: true, resume: true });
  };

  const cancelVerification = () => {
    setCancelVerifyAll(true);
  };

  const goToFirstPage = () => goToOffset(0);
  const goToPreviousPage = () => goToOffset(pageOffset() - pageSize());
  const goToNextPage = () => goToOffset(pageOffset() + pageSize());
  const goToLastPage = () => goToOffset((totalPages() - 1) * pageSize());

  onMount(() => {
    setIsMounted(true);
    void loadRuntimeCapabilities();

    if (typeof window === 'undefined') return;
    const params = new URLSearchParams(window.location.search);
    let mutated = false;
    const migrate = (
      key: keyof typeof STORAGE_KEYS,
      paramKey: string,
      parser: (raw: string | null) => string,
      defaultValue: string,
    ) => {
      if (params.has(paramKey)) return;
      const legacy = window.localStorage.getItem(STORAGE_KEYS[key]);
      const parsed = parser(legacy);
      if (parsed !== defaultValue && legacy === parsed) {
        params.set(paramKey, parsed);
        mutated = true;
      }
    };
    migrate('AUDIT_EVENT_FILTER', 'event', parseEventFilter, '');
    if (!params.has('user')) {
      const legacy = window.localStorage.getItem(STORAGE_KEYS.AUDIT_USER_FILTER);
      if (typeof legacy === 'string' && legacy !== '') {
        params.set('user', legacy);
        mutated = true;
      }
    }
    migrate('AUDIT_SUCCESS_FILTER', 'success', parseSuccessFilter, 'all');
    migrate('AUDIT_VERIFICATION_FILTER', 'verification', parseVerificationFilter, 'all');
    if (mutated) {
      navigate(`${window.location.pathname}?${params.toString()}`, { replace: true });
    }
  });

  createEffect(() => {
    if (!runtimeCapabilitiesLoaded()) {
      setLoading(true);
      return;
    }
    if (!hasFeature('audit_logging')) {
      setEvents([]);
      setTotalEvents(0);
      setIsPersistent(false);
      setError(null);
      setLoading(false);
      return;
    }
    untrack(() => {
      void fetchAuditEvents();
    });
  });

  // Live-apply: server-side filters (event, success) trigger a fresh fetch from
  // the first page immediately on change. userFilter debounces to avoid one
  // fetch per keystroke. verificationFilter is purely client-side; only
  // touches the local filteredEvents memo.
  let initialEventFilterApplied = false;
  createEffect(() => {
    const value = eventFilter();
    if (!initialEventFilterApplied) {
      initialEventFilterApplied = true;
      void value;
      return;
    }
    if (!auditLoggingEnabled()) return;
    refetchFromFirstPage();
  });

  let initialSuccessFilterApplied = false;
  createEffect(() => {
    const value = successFilter();
    if (!initialSuccessFilterApplied) {
      initialSuccessFilterApplied = true;
      void value;
      return;
    }
    if (!auditLoggingEnabled()) return;
    refetchFromFirstPage();
  });

  let userFilterDebounceHandle: number | null = null;
  let initialUserFilterApplied = false;
  createEffect(() => {
    const value = userFilter();
    if (!initialUserFilterApplied) {
      initialUserFilterApplied = true;
      void value;
      return;
    }
    if (!auditLoggingEnabled()) return;
    if (userFilterDebounceHandle !== null) {
      window.clearTimeout(userFilterDebounceHandle);
    }
    userFilterDebounceHandle = window.setTimeout(() => {
      userFilterDebounceHandle = null;
      refetchFromFirstPage();
    }, USER_FILTER_DEBOUNCE_MS);
  });

  onCleanup(() => {
    setIsMounted(false);
    setCancelVerifyAll(true);
    if (userFilterDebounceHandle !== null) {
      window.clearTimeout(userFilterDebounceHandle);
      userFilterDebounceHandle = null;
    }
    for (const controller of Object.values(verifyControllers())) {
      controller.abort();
    }
  });

  return {
    activeFilterChips,
    activeFilterCount,
    auditLoggingEnabled,
    autoVerifyEnabled,
    autoVerifyLimit,
    cancelVerification,
    clearFilterChip,
    clearFilters,
    error,
    eventFilter,
    events,
    filteredEvents,
    goToFirstPage,
    goToLastPage,
    goToNextPage,
    goToPreviousPage,
    hasNextPage,
    hasResumeEvents,
    hasSignedEvents,
    isPersistent,
    loading,
    pageInput,
    pageNumber,
    pageOffset,
    pageRangeText,
    pageSize,
    refresh,
    resumeCount,
    resumeVerification,
    resetPreferences,
    setAutoVerifyEnabled,
    setAutoVerifyLimit,
    setEventFilter,
    setPageInput,
    setPageSize,
    setSuccessFilter,
    setUserFilter,
    setVerificationFilter,
    showUpgradePaywall,
    showUpgradePrompts,
    showFeatureGateAction,
    submitPageInput,
    successFilter,
    totalEvents,
    totalPages,
    upgradeDestination,
    upgradeActionLabel,
    paidRuntimeRequired,
    verification,
    verificationFilter,
    verificationSummary,
    verifyAll,
    verifyAllDone,
    verifyAllLabel,
    verifyAllTotal,
    verifyCanceled,
    verifyEvent,
    verifying,
    verifyingAll,
    userFilter,
  };
};
