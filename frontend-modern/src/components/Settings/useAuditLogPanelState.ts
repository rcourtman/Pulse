import {
  batch,
  createEffect,
  createMemo,
  createSignal,
  on,
  onCleanup,
  onMount,
  untrack,
} from 'solid-js';
import type { Accessor } from 'solid-js';
import { unwrap } from 'solid-js/store';
import { useLocation, useNavigate } from '@solidjs/router';
import {
  createLocalStorageBooleanSignal,
  createLocalStorageNumberSignal,
  STORAGE_KEYS,
} from '@/utils/localStorage';
import { apiErrorFromResponse, apiFetch, getOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { showSuccess } from '@/utils/toast';
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
  user?: string;
  ip: string;
  path?: string;
  success: boolean;
  details?: string;
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

export interface AuditLogPanelProps {
  currentUser?: string;
  accessKey?: string;
  canRead?: boolean;
}

export const useAuditLogPanelState = (props: AuditLogPanelProps = {}) => {
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
    const normalized = normalizeAuditPageSize(value);
    setStoredPageSize(normalized);
    setPageOffset(0);
    setPageInput('');
    void fetchAuditEvents({ limit: normalized, offset: 0 });
  };
  const [pageOffset, setPageOffset] = createLocalStorageNumberSignal(
    STORAGE_KEYS.AUDIT_PAGE_OFFSET,
    0,
  );
  const [verification, setVerification] = createSignal<Record<string, VerificationState>>({});
  const [verificationNotice, setVerificationNotice] = createSignal<{
    tone: 'success' | 'warning';
    message: string;
  } | null>(null);
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
  const [verifyCanceled, setVerifyCanceled] = createSignal(false);
  const [verifyControllers, setVerifyControllers] = createSignal<Record<string, AbortController>>(
    {},
  );
  let auditRequestController: AbortController | null = null;
  let auditRequestGeneration = 0;
  let verificationGeneration = 0;
  let verificationBatch: object | null = null;
  let autoVerifyHandle: ReturnType<typeof setTimeout> | undefined;
  let userFilterDebounceHandle: number | null = null;
  let disposed = false;

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

  const accessAllowed = () => auditLoggingEnabled() && props.canRead !== false;
  const currentReader = () => ({
    org: getOrgID() || 'default',
    user: props.currentUser,
    accessKey: props.accessKey,
    allowed: accessAllowed(),
  });
  const [reader, setReader] = createSignal(currentReader());
  let readerInitialized = false;
  const readerIsCurrent = () => {
    const current = currentReader();
    const accepted = reader();
    return (
      !disposed &&
      Object.keys(current).every(
        (key) => current[key as keyof typeof current] === accepted[key as keyof typeof accepted],
      )
    );
  };
  // Storage can change before its event arrives. A stale action/completion is
  // not permission to dispatch under the newly selected organisation.
  const ensureCurrentReader = () => {
    if (disposed) return false;
    if (readerIsCurrent()) return true;
    resetReader();
    return false;
  };
  const requestOptions = (controller: AbortController) => ({
    signal: controller.signal,
    expectedOrgID: reader().org,
    retry: false,
  });
  const stopVerification = (clearResults = false) => {
    verificationGeneration += 1;
    verificationBatch = null;
    if (autoVerifyHandle !== undefined) clearTimeout(autoVerifyHandle);
    autoVerifyHandle = undefined;
    for (const controller of Object.values(verifyControllers())) controller.abort();
    batch(() => {
      setVerifyControllers({});
      setVerifying({});
      setVerifyingAll(false);
      setVerificationNotice(null);
      if (clearResults) {
        setVerification({});
        setVerifyAllTotal(0);
        setVerifyAllDone(0);
        setVerifyCanceled(false);
      }
    });
  };
  const clearInventory = () => {
    stopVerification(true);
    setEvents([]);
    setTotalEvents(0);
    setIsPersistent(false);
    setError(null);
  };

  const fetchAuditEvents = async (options?: { limit?: number; offset?: number }) => {
    if (!ensureCurrentReader()) return;
    auditRequestController?.abort();
    auditRequestController = null;
    const requestGeneration = ++auditRequestGeneration;
    const acceptedReader = reader();
    batch(clearInventory);
    if (!accessAllowed()) {
      setLoading(!runtimeCapabilitiesLoaded());
      return;
    }

    const limit = options?.limit ?? pageSize();
    const offset = options?.offset ?? pageOffset();
    const controller = new AbortController();
    auditRequestController = controller;

    const isCurrentRead = () =>
      ensureCurrentReader() &&
      acceptedReader === reader() &&
      requestGeneration === auditRequestGeneration &&
      auditRequestController === controller;
    setLoading(true);

    try {
      const params = new URLSearchParams();
      params.set('limit', String(limit));
      params.set('offset', String(Math.max(0, offset)));
      if (eventFilter()) params.set('event', eventFilter());
      if (userFilter()) params.set('user', userFilter());
      if (successFilter() === 'success') params.set('success', 'true');
      if (successFilter() === 'failed') params.set('success', 'false');

      const response = await apiFetch(`/api/audit?${params.toString()}`, {
        ...requestOptions(controller),
      });
      if (!isCurrentRead()) return;
      if ([401, 402, 403].includes(response.status)) {
        setError('Audit log access could not be confirmed. Refresh after access is restored.');
        return;
      }
      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `Failed to fetch audit events: ${response.statusText}`,
        );
      }

      const data: AuditResponse = await response.json();
      if (!isCurrentRead()) return;
      const ids = new Set<string>();
      if (
        !data ||
        !Array.isArray(data.events) ||
        !Number.isSafeInteger(data.total) ||
        data.total < 0 ||
        data.total < data.events.length ||
        typeof data.persistentLogging !== 'boolean' ||
        data.events.some((event) => {
          if (
            !event ||
            typeof event.id !== 'string' ||
            !event.id.trim() ||
            ids.has(event.id) ||
            typeof event.timestamp !== 'string' ||
            typeof event.event !== 'string' ||
            (event.user !== undefined && typeof event.user !== 'string') ||
            typeof event.ip !== 'string' ||
            (event.path !== undefined && typeof event.path !== 'string') ||
            (event.details !== undefined && typeof event.details !== 'string') ||
            typeof event.success !== 'boolean' ||
            (event.signature !== undefined && typeof event.signature !== 'string')
          )
            return true;
          ids.add(event.id);
          return false;
        })
      ) {
        throw new Error('Audit log returned an invalid response');
      }

      if (data.total && offset >= data.total) {
        const maxOffset = Math.max(0, Math.floor((data.total - 1) / limit) * limit);
        if (maxOffset !== offset) {
          setPageOffset(maxOffset);
          await fetchAuditEvents({ limit, offset: maxOffset });
          return;
        }
      }

      setEvents(data.events);
      setIsPersistent(data.persistentLogging);
      setTotalEvents(data.total);

      if (data.persistentLogging && autoVerifyEnabled()) {
        const verificationLimit = autoVerifyLimit();
        if (verificationLimit <= 0) return;
        autoVerifyHandle = setTimeout(() => {
          autoVerifyHandle = undefined;
          if (
            !isMounted() ||
            !ensureCurrentReader() ||
            acceptedReader !== reader() ||
            requestGeneration !== auditRequestGeneration
          )
            return;
          void verifyAllEvents({ limit: verificationLimit, showToast: false });
        }, 0);
      }
    } catch (err) {
      if (!isCurrentRead() || (err as { name?: string })?.name === 'AbortError') {
        return;
      }
      const message = getAuditLogFetchErrorMessage(err);
      setEvents([]);
      setTotalEvents(0);
      setIsPersistent(false);
      setError(message);
    } finally {
      if (isCurrentRead()) {
        auditRequestController = null;
        setLoading(false);
      }
    }
  };

  const isCurrentEvent = (event: AuditEvent) =>
    events().some((current) => unwrap(current) === unwrap(event));

  const verifyEvent = async (event: AuditEvent) => {
    if (
      !ensureCurrentReader() ||
      !accessAllowed() ||
      loading() ||
      error() ||
      !isCurrentEvent(event) ||
      !event.signature ||
      verifyControllers()[event.id]
    )
      return;
    const acceptedReader = reader();
    const generation = verificationGeneration;
    const controller = new AbortController();
    const isCurrentVerification = () =>
      ensureCurrentReader() &&
      acceptedReader === reader() &&
      generation === verificationGeneration &&
      verifyControllers()[event.id] === controller &&
      isCurrentEvent(event);
    batch(() => {
      setVerificationNotice(null);
      setVerifying((prev) => ({ ...prev, [event.id]: true }));
      setVerifyControllers((prev) => ({ ...prev, [event.id]: controller }));
    });
    try {
      const response = await apiFetch(
        `/api/audit/${encodeURIComponent(event.id)}/verify`,
        requestOptions(controller),
      );
      if (!isCurrentVerification()) return;
      if ([401, 402, 403].includes(response.status)) {
        batch(() => {
          clearInventory();
          setError('Audit log access could not be confirmed. Refresh after access is restored.');
        });
        return;
      }
      if (!response.ok) throw new Error('Failed to verify signature. Refresh or retry.');
      const data: VerifyResponse = await response.json();
      if (!isCurrentVerification()) return;
      if (
        !data ||
        typeof data.available !== 'boolean' ||
        (data.available && typeof data.verified !== 'boolean') ||
        (data.message !== undefined && typeof data.message !== 'string')
      )
        throw new Error('Failed to verify signature. Refresh or retry.');
      const status: VerificationStatus = !data.available
        ? 'unavailable'
        : data.verified
          ? 'verified'
          : 'failed';
      setVerification((prev) => ({ ...prev, [event.id]: { status, message: data.message || '' } }));
    } catch (err) {
      if (!isCurrentVerification() || (err as { name?: string })?.name === 'AbortError') return;
      setVerification((prev) => ({
        ...prev,
        [event.id]: {
          status: 'error',
          message: 'Failed to verify signature. Refresh or retry.',
        },
      }));
    } finally {
      if (isCurrentVerification()) {
        batch(() => {
          setVerifying((prev) => ({ ...prev, [event.id]: false }));
          setVerifyControllers((prev) => {
            const next = { ...prev };
            delete next[event.id];
            return next;
          });
        });
      }
    }
  };

  const verifyAllEvents = async (options?: VerifyAllOptions) => {
    if (!ensureCurrentReader() || !accessAllowed() || loading() || error() || verificationBatch)
      return;
    const acceptedReader = reader();
    const generation = verificationGeneration;
    const batchIdentity = {};
    const isCurrentBatch = () =>
      ensureCurrentReader() &&
      acceptedReader === reader() &&
      generation === verificationGeneration &&
      verificationBatch === batchIdentity;
    const limit = options?.limit;
    let signedEvents = events().filter((event) => event.signature && !verifying()[event.id]);
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

    verificationBatch = batchIdentity;
    setVerifyingAll(true);
    setVerifyCanceled(false);
    setVerifyAllTotal(signedEvents.length);
    setVerifyAllDone(0);

    for (const event of signedEvents) {
      if (!isCurrentBatch()) return;
      await verifyEvent(event);
      if (!isCurrentBatch()) return;
      setVerifyAllDone((prev) => prev + 1);
    }

    verificationBatch = null;
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

      setVerificationNotice({
        tone: failed > 0 || errors > 0 ? 'warning' : 'success',
        message: `Signature verification completed. Verified ${verified}, failed ${failed}, errors ${errors}, unavailable ${unavailable}.`,
      });
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
    updateSearchParam((params) => {
      params.delete('event');
      params.delete('user');
      params.delete('success');
      params.delete('verification');
    });
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
    if (!verifyingAll()) return;
    batch(() => {
      stopVerification();
      setVerifyCanceled(true);
    });
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

  const resetReader = () => {
    auditRequestController?.abort();
    auditRequestController = null;
    auditRequestGeneration += 1;
    if (userFilterDebounceHandle !== null) window.clearTimeout(userFilterDebounceHandle);
    userFilterDebounceHandle = null;
    batch(() => {
      setReader(currentReader());
      clearInventory();
      if (readerInitialized) {
        resetPaging();
        updateSearchParam((params) => {
          params.delete('user');
        });
      }
      readerInitialized = true;
      setLoading(!runtimeCapabilitiesLoaded() || accessAllowed());
    });
    if (!disposed) void fetchAuditEvents();
  };
  createEffect(
    on(
      [runtimeCapabilitiesLoaded, accessAllowed, () => props.currentUser, () => props.accessKey],
      (current, previous) => {
        if (!previous || current.some((value, index) => value !== previous[index])) resetReader();
      },
    ),
  );
  const unsubscribeOrg = eventBus.on('org_switched', resetReader);

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
    if (!accessAllowed()) return;
    untrack(refetchFromFirstPage);
  });

  let initialSuccessFilterApplied = false;
  createEffect(() => {
    const value = successFilter();
    if (!initialSuccessFilterApplied) {
      initialSuccessFilterApplied = true;
      void value;
      return;
    }
    if (!accessAllowed()) return;
    untrack(refetchFromFirstPage);
  });

  let initialUserFilterApplied = false;
  createEffect(() => {
    const value = userFilter();
    if (!initialUserFilterApplied) {
      initialUserFilterApplied = true;
      void value;
      return;
    }
    if (!accessAllowed()) return;
    if (userFilterDebounceHandle !== null) {
      window.clearTimeout(userFilterDebounceHandle);
    }
    userFilterDebounceHandle = window.setTimeout(() => {
      userFilterDebounceHandle = null;
      refetchFromFirstPage();
    }, USER_FILTER_DEBOUNCE_MS);
  });

  onCleanup(() => {
    disposed = true;
    unsubscribeOrg();
    setIsMounted(false);
    auditRequestController?.abort();
    auditRequestController = null;
    auditRequestGeneration += 1;
    if (userFilterDebounceHandle !== null) window.clearTimeout(userFilterDebounceHandle);
    userFilterDebounceHandle = null;
    batch(clearInventory);
  });

  return {
    accessAllowed,
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
    verificationNotice,
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
