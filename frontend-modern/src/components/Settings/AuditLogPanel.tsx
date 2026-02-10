import { createSignal, Show, For, onMount, createMemo, onCleanup, createEffect } from 'solid-js';
import Shield from 'lucide-solid/icons/shield';
import CheckCircle from 'lucide-solid/icons/check-circle';
import XCircle from 'lucide-solid/icons/x-circle';
import RefreshCw from 'lucide-solid/icons/refresh-cw';
import Filter from 'lucide-solid/icons/filter';
import Info from 'lucide-solid/icons/info';
import Play from 'lucide-solid/icons/play';
import X from 'lucide-solid/icons/x';
import ShieldAlert from 'lucide-solid/icons/shield-alert';
import { showTooltip, hideTooltip } from '@/components/shared/Tooltip';
import Toggle from '@/components/shared/Toggle';
import {
    createLocalStorageBooleanSignal,
    createLocalStorageNumberSignal,
    createLocalStorageStringSignal,
    STORAGE_KEYS,
} from '@/utils/localStorage';
import { showSuccess, showWarning, showToast } from '@/utils/toast';
import { getUpgradeActionUrlOrFallback, hasFeature, loadLicenseStatus, licenseLoaded } from '@/stores/license';

interface AuditEvent {
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

type VerificationState = {
    status: 'verified' | 'failed' | 'unavailable' | 'error';
    message: string;
};

export default function AuditLogPanel() {
    const [events, setEvents] = createSignal<AuditEvent[]>([]);
    const [totalEvents, setTotalEvents] = createSignal(0);
    const [loading, setLoading] = createSignal(true);
    const [error, setError] = createSignal<string | null>(null);
    const [isPersistent, setIsPersistent] = createSignal(false);
    const [eventFilter, setEventFilter] = createLocalStorageStringSignal(
        STORAGE_KEYS.AUDIT_EVENT_FILTER,
        '',
    );
    const [userFilter, setUserFilter] = createLocalStorageStringSignal(
        STORAGE_KEYS.AUDIT_USER_FILTER,
        '',
    );
    const [successFilter, setSuccessFilter] = createLocalStorageStringSignal(
        STORAGE_KEYS.AUDIT_SUCCESS_FILTER,
        'all',
    );
    const [verificationFilter, setVerificationFilter] = createLocalStorageStringSignal(
        STORAGE_KEYS.AUDIT_VERIFICATION_FILTER,
        'all',
    );
    const allowedVerificationFilters = new Set(['all', 'needs', 'verified', 'failed']);
    const allowedSuccessFilters = new Set(['all', 'success', 'failed']);
    const [pageSize, setPageSize] = createLocalStorageNumberSignal(
        STORAGE_KEYS.AUDIT_PAGE_SIZE,
        100,
    );
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
    const [verifyControllers, setVerifyControllers] = createSignal<Record<string, AbortController>>({});

    const fetchAuditEvents = async (options?: { limit?: number; offset?: number }) => {
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

            const response = await fetch(`/api/audit?${params.toString()}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch audit events: ${response.statusText}`);
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
                const limit = autoVerifyLimit();
                if (limit <= 0) return;
                setTimeout(() => {
                    if (!isMounted()) return;
                    void verifyAllEvents({ limit, showToast: false });
                }, 0);
            }
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'Unknown error';
            setError(msg);
            showWarning('Audit Log Error', msg);
        } finally {
            setLoading(false);
        }
    };

    const verifyEvent = async (event: AuditEvent) => {
        if (!event.signature) return;
        setVerifying((prev) => ({ ...prev, [event.id]: true }));
        try {
            const controller = new AbortController();
            setVerifyControllers((prev) => ({ ...prev, [event.id]: controller }));
            if (!isMounted()) {
                controller.abort();
            }
            const response = await fetch(`/api/audit/${event.id}/verify`, { signal: controller.signal });
            if (!response.ok) {
                throw new Error(`Failed to verify signature: ${response.statusText}`);
            }
            const data: VerifyResponse = await response.json();
            let status: VerificationState['status'] = 'unavailable';
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

    const verifyAllEvents = async (options?: { limit?: number; showToast?: boolean; resume?: boolean }) => {
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
                const controllers = verifyControllers();
                for (const controller of Object.values(controllers)) {
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
                if (!state) {
                    continue;
                }
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

    onMount(() => {
        setIsMounted(true);
        loadLicenseStatus();
        fetchAuditEvents();
    });

    createEffect(() => {
        const current = verificationFilter();
        if (!allowedVerificationFilters.has(current)) {
            setVerificationFilter('all');
        }
    });

    createEffect(() => {
        const current = successFilter();
        if (!allowedSuccessFilters.has(current)) {
            setSuccessFilter('all');
        }
    });

    onCleanup(() => {
        setIsMounted(false);
        setCancelVerifyAll(true);
        const controllers = verifyControllers();
        for (const controller of Object.values(controllers)) {
            controller.abort();
        }
    });

    const formatTimestamp = (ts: string) => {
        const date = new Date(ts);
        return date.toLocaleString();
    };

    const getEventIcon = (_event: string, success: boolean) => {
        if (!success) return <XCircle class="w-4 h-4 text-rose-400" />;
        return <CheckCircle class="w-4 h-4 text-emerald-400" />;
    };

    const getEventTypeBadge = (event: string) => {
        const colors: Record<string, string> = {
            login: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
            logout: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
            config_change: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
            startup: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
            oidc_token_refresh: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
        };
        return colors[event] || 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
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
    const hasNextPage = () => events().length === pageSize();
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

    const activeFilterChips = () => {
        const chips: { label: string; key: 'event' | 'user' | 'success' | 'verification' }[] = [];
        if (eventFilter()) chips.push({ label: `Event: ${eventFilter()}`, key: 'event' });
        if (userFilter()) chips.push({ label: `User: ${userFilter()}`, key: 'user' });
        if (successFilter() !== 'all') chips.push({ label: `Success: ${successFilter()}`, key: 'success' });
        if (verificationFilter() !== 'all') chips.push({ label: `Verification: ${verificationFilter()}`, key: 'verification' });
        return chips;
    };

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

    const getVerificationBadge = (state?: VerificationState) => {
        if (!state) {
            return { label: 'Not checked', class: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300' };
        }
        switch (state.status) {
            case 'verified':
                return { label: 'Verified', class: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' };
            case 'failed':
                return { label: 'Failed', class: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' };
            case 'error':
                return { label: 'Error', class: 'bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200' };
            default:
                return { label: 'Unavailable', class: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300' };
        }
    };

    return (
        <div class="space-y-6">
            {/* Header */}
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <Shield class="w-6 h-6 text-gray-500" />
                    <h2 class="text-xl font-semibold text-gray-900 dark:text-white">Audit Log</h2>
                </div>
                <div class="flex items-center gap-2">
                    <button
                        onClick={() => fetchAuditEvents()}
                        disabled={loading()}
                        class="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
                        Refresh
                    </button>
                    <button
                        onClick={() => verifyAllEvents({ showToast: true })}
                        disabled={!isPersistent() || loading() || verifyingAll() || !hasSignedEvents()}
                        class="flex items-center gap-2 px-3 py-2 text-sm font-medium text-blue-700 dark:text-blue-200 bg-blue-50 dark:bg-blue-900/40 border border-blue-200 dark:border-blue-700 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/60 disabled:opacity-50"
                    >
                        <CheckCircle class="w-4 h-4" />
                        {verifyAllLabel()}
                    </button>
                    <button
                        onClick={() => setCancelVerifyAll(true)}
                        disabled={!verifyingAll()}
                        class="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        Cancel
                    </button>
                    <button
                        onClick={() => verifyAllEvents({ showToast: true, resume: true })}
                        disabled={verifyingAll() || !hasResumeEvents()}
                        class="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-600 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                        onMouseEnter={(e) => {
                            if (!hasResumeEvents()) return;
                            const rect = e.currentTarget.getBoundingClientRect();
                            showTooltip(
                                'Retries failed, error, or unchecked signatures on this page.',
                                rect.left + rect.width / 2,
                                rect.top,
                                { align: 'center', direction: 'up', maxWidth: 260 },
                            );
                        }}
                        onMouseLeave={() => hideTooltip()}
                    >
                        <Play class="w-4 h-4" />
                        Resume{resumeCount() > 0 ? ` (${resumeCount()})` : ''}
                    </button>
                    <Show when={verifyCanceled() && !verifyingAll()}>
                        <span class="text-xs text-amber-600 dark:text-amber-400">
                            Verification canceled
                        </span>
                    </Show>
                </div>
            </div>

            {/* Upgrade CTA */}
            <Show when={licenseLoaded() && !hasFeature('audit_logging') && !loading()}>
                <div class="p-6 bg-gray-50 dark:bg-gray-800/40 border border-gray-200 dark:border-gray-700 rounded-xl">
                    <div class="flex flex-col sm:flex-row items-center gap-4">
                        <div class="flex-1">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Audit Logging</h3>
                            <p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
                                Persistent, searchable audit logs with cryptographic signature verification.
                            </p>
                        </div>
                        <a
                            href={getUpgradeActionUrlOrFallback('audit_logging')}
                            target="_blank"
                            class="px-5 py-2.5 text-sm font-semibold bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                        >
                            Upgrade to Pro
                        </a>
                    </div>
                </div>
            </Show>

            {/* Filters */}
            <Show when={isPersistent()}>
                <div class="flex flex-wrap gap-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                    <div class="flex items-center gap-2">
                        <Filter class="w-4 h-4 text-gray-400" />
                        <span class="text-sm font-medium text-gray-700 dark:text-gray-300">Filters:</span>
                    </div>
                    <select
                        value={eventFilter()}
                        onChange={(e) => setEventFilter(e.currentTarget.value)}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                        <option value="">All Events</option>
                        <option value="login">Login</option>
                        <option value="logout">Logout</option>
                        <option value="config_change">Config Change</option>
                        <option value="startup">Startup</option>
                    </select>
                    <input
                        type="text"
                        placeholder="Filter by user..."
                        value={userFilter()}
                        onInput={(e) => setUserFilter(e.currentTarget.value)}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400"
                    />
                    <select
                        value={successFilter()}
                        onChange={(e) => setSuccessFilter(e.currentTarget.value)}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                        <option value="all">All</option>
                        <option value="success">Success Only</option>
                        <option value="failed">Failed Only</option>
                    </select>
                    <select
                        value={verificationFilter()}
                        onChange={(e) => setVerificationFilter(e.currentTarget.value)}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                        <option value="all">All Verification</option>
                        <option value="needs">Needs Verification</option>
                        <option value="verified">Verified</option>
                        <option value="failed">Failed/Error</option>
                    </select>
                    <select
                        value={String(pageSize())}
                        onChange={(e) => setPageSize(Number(e.currentTarget.value))}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                        <option value="25">25 / page</option>
                        <option value="50">50 / page</option>
                        <option value="100">100 / page</option>
                        <option value="200">200 / page</option>
                    </select>
                    <button
                        onClick={() => {
                            setPageOffset(0);
                            void fetchAuditEvents({ offset: 0 });
                        }}
                        class="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg"
                    >
                        Apply
                    </button>
                    <button
                        onClick={() => {
                            const hadFilters = activeFilterCount() > 0;
                            setEventFilter('');
                            setUserFilter('');
                            setSuccessFilter('all');
                            setVerificationFilter('all');
                            setPageOffset(0);
                            void fetchAuditEvents({ offset: 0 });
                            if (hadFilters) {
                                showSuccess('Audit filters cleared');
                            }
                        }}
                        class="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600"
                    >
                        Clear{activeFilterCount() > 0 ? ` (${activeFilterCount()})` : ''}
                    </button>
                </div>
                <Show when={activeFilterCount() > 0}>
                    <div class="mt-2 flex flex-wrap gap-2">
                        <For each={activeFilterChips()}>
                            {(chip) => (
                                <button
                                    type="button"
                                    onClick={() => {
                                        if (chip.key === 'event') setEventFilter('');
                                        if (chip.key === 'user') setUserFilter('');
                                        if (chip.key === 'success') setSuccessFilter('all');
                                        if (chip.key === 'verification') setVerificationFilter('all');
                                        setPageOffset(0);
                                        void fetchAuditEvents({ offset: 0 });
                                    }}
                                    class="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-full bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200 hover:bg-gray-300 dark:hover:bg-gray-600"
                                    title="Click to clear filter"
                                    aria-label={`Clear ${chip.label}`}
                                    onMouseEnter={(e) => {
                                        const rect = e.currentTarget.getBoundingClientRect();
                                        showTooltip('Click to clear filter', rect.left + rect.width / 2, rect.top, {
                                            align: 'center',
                                            direction: 'up',
                                        });
                                    }}
                                    onMouseLeave={() => hideTooltip()}
                                    onKeyDown={(e) => {
                                        if (e.key !== 'Enter' && e.key !== ' ') return;
                                        e.preventDefault();
                                        if (chip.key === 'event') setEventFilter('');
                                        if (chip.key === 'user') setUserFilter('');
                                        if (chip.key === 'success') setSuccessFilter('all');
                                        if (chip.key === 'verification') setVerificationFilter('all');
                                        setPageOffset(0);
                                        void fetchAuditEvents({ offset: 0 });
                                    }}
                                >
                                    {chip.label}
                                    <X class="w-3 h-3 text-gray-500 dark:text-gray-300" />
                                </button>
                            )}
                        </For>
                    </div>
                </Show>
            </Show>

            {/* Verification Preferences */}
            <Show when={isPersistent()}>
                <div class="flex items-center justify-between gap-4 px-1">
                    <Toggle
                        label="Auto-verify signatures"
                        description="Verify signatures after loading audit events."
                        checked={autoVerifyEnabled()}
                        onChange={(e) => setAutoVerifyEnabled(e.currentTarget.checked)}
                    />
                    <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                        <label class="flex items-center gap-2">
                            <span>Auto-verify limit</span>
                            <input
                                type="number"
                                min="0"
                                value={autoVerifyLimit()}
                                disabled={!autoVerifyEnabled()}
                                onInput={(e) => {
                                    const raw = Number(e.currentTarget.value);
                                    if (!Number.isFinite(raw)) {
                                        setAutoVerifyLimit(0);
                                        return;
                                    }
                                    const clamped = Math.max(0, Math.min(500, Math.floor(raw)));
                                    setAutoVerifyLimit(clamped);
                                }}
                                class="w-20 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-xs text-gray-900 dark:text-gray-100"
                            />
                        </label>
                        <span class="text-gray-500 dark:text-gray-400">0 disables, max 500</span>
                        <span
                            class="inline-flex items-center text-gray-400 cursor-help"
                            onMouseEnter={(e) => {
                                const rect = e.currentTarget.getBoundingClientRect();
                                showTooltip(
                                    'Large verification batches can add load. Use a smaller limit on busy systems.',
                                    rect.left + rect.width / 2,
                                    rect.top,
                                    { align: 'center', direction: 'up', maxWidth: 260 },
                                );
                            }}
                            onMouseLeave={() => hideTooltip()}
                        >
                            <Info class="w-3 h-3" />
                        </span>
                        <button
                            onClick={() => {
                                setAutoVerifyEnabled(true);
                                setAutoVerifyLimit(50);
                                setPageSize(100);
                                setPageOffset(0);
                                setPageInput('');
                                showSuccess('Audit preferences reset');
                            }}
                            class="ml-2 px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-800"
                        >
                            Reset preferences
                        </button>
                    </div>
                </div>
            </Show>

            {/* Error State */}
            <Show when={error()}>
                <div class="p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg text-red-700 dark:text-red-300">
                    {error()}
                </div>
            </Show>

            {/* Loading State */}
            <Show when={loading()}>
                <div class="flex items-center justify-center py-12">
                    <RefreshCw class="w-8 h-8 text-blue-500 animate-spin" />
                </div>
            </Show>

            {/* Events Table */}
            <Show when={!loading() && isPersistent() && filteredEvents().length > 0}>
                <div class="flex flex-wrap items-center gap-3 text-xs text-gray-600 dark:text-gray-300">
                    <span>Total: {totalEvents()}</span>
                    <span>Signed: {verificationSummary().signed}</span>
                    <span>Verified: {verificationSummary().verified}</span>
                    <span>Failed: {verificationSummary().failed}</span>
                    <span>Errors: {verificationSummary().error}</span>
                    <span>Unavailable: {verificationSummary().unavailable}</span>
                    <span>Unchecked: {verificationSummary().unchecked}</span>
                    <span
                        class="inline-flex items-center gap-1 text-gray-500 dark:text-gray-400 cursor-help"
                        onMouseEnter={(e) => {
                            const rect = e.currentTarget.getBoundingClientRect();
                            const content = [
                                'Unsigned: no signature stored for this event',
                                'Unavailable: verification not supported by the logger',
                                'Not checked: signature exists but has not been verified',
                                'Failed: signature mismatch or tampering detected',
                                'Error: verification request failed',
                            ].join('\n');
                            showTooltip(content, rect.left + rect.width / 2, rect.top, {
                                align: 'center',
                                direction: 'up',
                                maxWidth: 320,
                            });
                        }}
                        onMouseLeave={() => hideTooltip()}
                    >
                        <Info class="w-3 h-3" />
                        Legend
                    </span>
                    <span class="ml-auto text-gray-500 dark:text-gray-400">
                        {pageRangeText()} • Page {pageNumber()} of {totalPages()}
                    </span>
                </div>
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                        <thead class="bg-gray-50 dark:bg-gray-800">
                            <tr>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Status</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Timestamp</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Event</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">User</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">IP</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Details</th>
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Verification</th>
                            </tr>
                        </thead>
                        <tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                            <For each={filteredEvents()}>
                                {(event) => (
                                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-800">
                                        <td class="px-4 py-3 whitespace-nowrap">
                                            {getEventIcon(event.event, event.success)}
                                        </td>
                                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-600 dark:text-gray-300">
                                            {formatTimestamp(event.timestamp)}
                                        </td>
                                        <td class="px-4 py-3 whitespace-nowrap">
                                            <span class={`px-2 py-1 text-xs font-medium rounded-full ${getEventTypeBadge(event.event)}`}>
                                                {event.event}
                                            </span>
                                        </td>
                                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-900 dark:text-white">
                                            {event.user || '-'}
                                        </td>
                                        <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400 font-mono">
                                            {event.ip || '-'}
                                        </td>
                                        <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300 max-w-xs truncate">
                                            {event.details || '-'}
                                        </td>
                                        <td class="px-4 py-3 whitespace-nowrap">
                                            <Show
                                                when={event.signature}
                                                fallback={<span class="text-gray-400">Unsigned</span>}
                                            >
                                                {(() => {
                                                    const badge = getVerificationBadge(verification()[event.id]);
                                                    return (
                                                        <div class="flex items-center gap-2">
                                                            <Show
                                                                when={verifying()[event.id]}
                                                                fallback={
                                                                    <span class={`px-2 py-0.5 text-xs font-medium rounded-full ${badge.class}`}>
                                                                        {badge.label}
                                                                    </span>
                                                                }
                                                            >
                                                                <span class="text-xs text-gray-500 dark:text-gray-400">Verifying…</span>
                                                            </Show>
                                                            <button
                                                                onClick={() => verifyEvent(event)}
                                                                disabled={verifying()[event.id]}
                                                                class="text-xs font-medium text-blue-600 dark:text-blue-400 hover:underline disabled:opacity-50"
                                                            >
                                                                Verify
                                                            </button>
                                                        </div>
                                                    );
                                                })()}
                                            </Show>
                                        </td>
                                    </tr>
                                )}
                            </For>
                        </tbody>
                    </table>
                </div>
            </Show>

            {/* Empty State */}
            <Show when={!loading() && isPersistent() && filteredEvents().length === 0}>
                <div class="text-center py-12 px-4 bg-gray-50/50 dark:bg-gray-800/30 rounded-xl border border-dashed border-gray-200 dark:border-gray-700">
                    <div class="flex flex-col items-center max-w-sm mx-auto">
                        <Show
                            when={activeFilterCount() > 0}
                            fallback={<Shield class="w-12 h-12 text-gray-300 dark:text-gray-600 mb-4" />}
                        >
                            <ShieldAlert class="w-12 h-12 text-blue-300 dark:text-blue-900 mb-4" />
                        </Show>
                        <h3 class="text-lg font-medium text-gray-900 dark:text-white">No audit events found</h3>
                        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                            {activeFilterCount() > 0
                                ? "No events match your current filters. Try adjusting or clearing them."
                                : "Audit logging is active, but no events have been recorded yet."}
                        </p>
                        <Show when={activeFilterCount() > 0}>
                            <button
                                onClick={() => {
                                    setEventFilter('');
                                    setUserFilter('');
                                    setSuccessFilter('all');
                                    setVerificationFilter('all');
                                    setPageOffset(0);
                                    void fetchAuditEvents({ offset: 0 });
                                }}
                                class="mt-6 px-4 py-2 text-sm font-medium text-blue-600 dark:text-blue-400 border border-blue-200 dark:border-blue-800 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/30"
                            >
                                Clear all filters
                            </button>
                        </Show>
                    </div>
                </div>
            </Show>

            {/* Pagination */}
            <Show when={!loading() && isPersistent()}>
                <div class="flex items-center justify-end gap-2">
                    <div class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
                        <label class="flex items-center gap-2">
                            <span>Page</span>
                            <input
                                type="number"
                                min="1"
                                value={pageInput()}
                                onInput={(e) => setPageInput(e.currentTarget.value)}
                                onKeyDown={(e) => {
                                    if (e.key !== 'Enter') return;
                                    const parsed = Number(pageInput());
                                    if (!Number.isFinite(parsed)) return;
                                    const clamped = Math.max(1, Math.min(totalPages(), Math.floor(parsed)));
                                    const nextOffset = (clamped - 1) * pageSize();
                                    setPageOffset(nextOffset);
                                    void fetchAuditEvents({ offset: nextOffset });
                                }}
                                class="w-16 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 px-2 py-1 text-xs text-gray-900 dark:text-gray-100"
                            />
                        </label>
                        <button
                            onClick={() => {
                                const parsed = Number(pageInput());
                                if (!Number.isFinite(parsed)) return;
                                const clamped = Math.max(1, Math.min(totalPages(), Math.floor(parsed)));
                                const nextOffset = (clamped - 1) * pageSize();
                                setPageOffset(nextOffset);
                                void fetchAuditEvents({ offset: nextOffset });
                            }}
                            class="px-2 py-1 text-xs font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600"
                        >
                            Go
                        </button>
                    </div>
                    <button
                        onClick={() => {
                            setPageOffset(0);
                            void fetchAuditEvents({ offset: 0 });
                        }}
                        disabled={pageOffset() === 0}
                        class="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        First
                    </button>
                    <button
                        onClick={() => {
                            const nextOffset = Math.max(0, pageOffset() - pageSize());
                            setPageOffset(nextOffset);
                            void fetchAuditEvents({ offset: nextOffset });
                        }}
                        disabled={pageOffset() === 0}
                        class="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        Previous
                    </button>
                    <button
                        onClick={() => {
                            const nextOffset = pageOffset() + pageSize();
                            setPageOffset(nextOffset);
                            void fetchAuditEvents({ offset: nextOffset });
                        }}
                        disabled={!hasNextPage()}
                        class="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        Next
                    </button>
                    <button
                        onClick={() => {
                            const lastOffset = Math.max(0, (totalPages() - 1) * pageSize());
                            setPageOffset(lastOffset);
                            void fetchAuditEvents({ offset: lastOffset });
                        }}
                        disabled={!hasNextPage()}
                        class="px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                    >
                        Last
                    </button>
                </div>
            </Show>
        </div>
    );
}
