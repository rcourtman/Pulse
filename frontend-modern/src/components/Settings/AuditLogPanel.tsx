import { createSignal, Show, For, onMount } from 'solid-js';
import { Shield, CheckCircle, XCircle, AlertTriangle, RefreshCw, Filter } from 'lucide-solid';

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

export default function AuditLogPanel() {
    const [events, setEvents] = createSignal<AuditEvent[]>([]);
    const [loading, setLoading] = createSignal(true);
    const [error, setError] = createSignal<string | null>(null);
    const [isPersistent, setIsPersistent] = createSignal(false);
    const [eventFilter, setEventFilter] = createSignal('');
    const [userFilter, setUserFilter] = createSignal('');
    const [successFilter, setSuccessFilter] = createSignal<'all' | 'success' | 'failed'>('all');

    const fetchAuditEvents = async () => {
        setLoading(true);
        setError(null);
        try {
            const params = new URLSearchParams();
            params.set('limit', '100');
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
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Unknown error');
        } finally {
            setLoading(false);
        }
    };

    onMount(() => {
        fetchAuditEvents();
    });

    const formatTimestamp = (ts: string) => {
        const date = new Date(ts);
        return date.toLocaleString();
    };

    const getEventIcon = (_event: string, success: boolean) => {
        if (!success) return <XCircle class="w-4 h-4 text-red-500" />;
        return <CheckCircle class="w-4 h-4 text-green-500" />;
    };

    const getEventTypeBadge = (event: string) => {
        const colors: Record<string, string> = {
            login: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
            logout: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
            config_change: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
            startup: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
            oidc_token_refresh: 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200',
        };
        return colors[event] || 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200';
    };

    return (
        <div class="space-y-6">
            {/* Header */}
            <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                    <Shield class="w-6 h-6 text-indigo-500" />
                    <h2 class="text-xl font-semibold text-gray-900 dark:text-white">Audit Log</h2>
                    <Show when={isPersistent()}>
                        <span class="px-2 py-1 text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 rounded-full">
                            Enterprise
                        </span>
                    </Show>
                </div>
                <button
                    onClick={fetchAuditEvents}
                    disabled={loading()}
                    class="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                >
                    <RefreshCw class={`w-4 h-4 ${loading() ? 'animate-spin' : ''}`} />
                    Refresh
                </button>
            </div>

            {/* OSS Notice */}
            <Show when={!isPersistent() && !loading()}>
                <div class="flex items-start gap-3 p-4 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-lg">
                    <AlertTriangle class="w-5 h-5 text-amber-500 flex-shrink-0 mt-0.5" />
                    <div>
                        <h3 class="font-medium text-amber-800 dark:text-amber-200">Console Logging Only</h3>
                        <p class="text-sm text-amber-700 dark:text-amber-300 mt-1">
                            Audit events are logged to the console/file but not stored for querying.
                            Upgrade to Pulse Pro for persistent audit logs with signature verification.
                        </p>
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
                        onChange={(e) => setSuccessFilter(e.currentTarget.value as any)}
                        class="px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                    >
                        <option value="all">All</option>
                        <option value="success">Success Only</option>
                        <option value="failed">Failed Only</option>
                    </select>
                    <button
                        onClick={fetchAuditEvents}
                        class="px-3 py-1.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-700 rounded-lg"
                    >
                        Apply
                    </button>
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
                    <RefreshCw class="w-8 h-8 text-indigo-500 animate-spin" />
                </div>
            </Show>

            {/* Events Table */}
            <Show when={!loading() && isPersistent() && events().length > 0}>
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
                                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Signed</th>
                            </tr>
                        </thead>
                        <tbody class="bg-white dark:bg-gray-900 divide-y divide-gray-200 dark:divide-gray-700">
                            <For each={events()}>
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
                                            <Show when={event.signature} fallback={<span class="text-gray-400">-</span>}>
                                                <CheckCircle class="w-4 h-4 text-green-500" />
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
            <Show when={!loading() && isPersistent() && events().length === 0}>
                <div class="text-center py-12 text-gray-500 dark:text-gray-400">
                    <Shield class="w-12 h-12 mx-auto mb-4 opacity-50" />
                    <p>No audit events found</p>
                </div>
            </Show>
        </div>
    );
}
