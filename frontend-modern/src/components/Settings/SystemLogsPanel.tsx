import { Component, createSignal, onMount, onCleanup, For } from 'solid-js';
import SettingsPanel from '@/components/shared/SettingsPanel';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import Download from 'lucide-solid/icons/download';
import Pause from 'lucide-solid/icons/pause';
import Play from 'lucide-solid/icons/play';
import Trash2 from 'lucide-solid/icons/trash-2';
import Terminal from 'lucide-solid/icons/terminal';

export const SystemLogsPanel: Component = () => {
    const [logs, setLogs] = createSignal<string[]>([]);
    const [isPaused, setIsPaused] = createSignal(false);
    const [level, setLevel] = createSignal('info');
    const [isLoading, setIsLoading] = createSignal(true);

    let logContainer: HTMLDivElement | undefined;
    let eventSource: EventSource | null = null;
    const MAX_LOGS = 1000;

    const fetchLevel = async () => {
        try {
            const res = await apiFetchJSON('/api/logs/level') as { level?: string };
            if (res.level) setLevel(res.level);
        } catch (e) {
            logger.error('Failed to fetch log level', e);
        }
    };

    const connectStream = () => {
        // Use relative path which works with the proxy setup
        const url = '/api/logs/stream';

        eventSource = new EventSource(url);

        eventSource.onmessage = (event) => {
            if (isPaused()) return;

            const cleanData = event.data;

            setLogs(prev => {
                const newLogs = [...prev, cleanData];
                if (newLogs.length > MAX_LOGS) {
                    return newLogs.slice(newLogs.length - MAX_LOGS);
                }
                return newLogs;
            });

            // Auto-scroll
            if (logContainer) {
                logContainer.scrollTop = logContainer.scrollHeight;
            }
        };

        eventSource.onerror = () => {
            // Browser handles reconnection, but let's log it
            logger.debug("SSE stream disconnected, reconnecting...");
        };
    };

    onMount(async () => {
        await fetchLevel();
        connectStream();
        setIsLoading(false);
    });

    onCleanup(() => {
        if (eventSource) {
            eventSource.close();
        }
    });

    const handleLevelChange = async (newLevel: string) => {
        try {
            await apiFetchJSON('/api/logs/level', {
                method: 'POST',
                body: JSON.stringify({ level: newLevel })
            });
            setLevel(newLevel);
            notificationStore.success(`Log level set to ${newLevel}`);
        } catch (e) {
            logger.error('Error setting log level', e);
            notificationStore.error('Failed to set log level');
        }
    };

    const handleDownload = () => {
        window.location.href = '/api/logs/download';
    }

    return (
        <div class="space-y-6">
            <SettingsPanel
                title="System Logs"
                description="View real-time system logs and download support bundles."
                icon={<Terminal class="w-5 h-5" strokeWidth={2} />}
                bodyClass="space-y-4"
            >
                {/* Controls */}
                <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                    <div class="flex items-center space-x-3">
                        <label class="text-sm font-medium text-slate-700 dark:text-slate-300">Log Level:</label>
                        <select
                            value={level()}
                            onChange={(e) => handleLevelChange(e.currentTarget.value)}
                            class="form-select text-sm py-1.5 px-3 rounded-md border-gray-300 dark:border-gray-600 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 focus:ring-primary-500 focus:border-primary-500"
                        >
                            <option value="debug">Debug</option>
                            <option value="info">Info</option>
                            <option value="warn">Warn</option>
                            <option value="error">Error</option>
                        </select>
                    </div>

                    <div class="flex items-center space-x-2">
                        <button
                            onClick={() => setIsPaused(!isPaused())}
                            class={`p-2 rounded transition-colors ${isPaused()
                                ? 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400'
                                : 'hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-400'
                                }`}
                            title={isPaused() ? "Resume Stream" : "Pause Stream"}
                        >
                            {isPaused() ? <Play size={18} /> : <Pause size={18} />}
                        </button>
                        <button
                            onClick={() => setLogs([])}
                            class="p-2 rounded hover:bg-slate-100 dark:hover:bg-slate-700 text-slate-600 dark:text-slate-400 transition-colors"
                            title="Clear Logs"
                        >
                            <Trash2 size={18} />
                        </button>
                        <div class="h-6 w-px bg-slate-200 dark:bg-slate-700 mx-2"></div>
                        <button
                            onClick={handleDownload}
                            class="flex items-center space-x-2 px-3 py-1.5 bg-primary-600 text-white rounded-md hover:bg-primary-700 text-sm font-medium transition-colors shadow-sm"
                        >
                            <Download size={16} />
                            <span>Support Bundle</span>
                        </button>
                    </div>
                </div>

                {/* Terminal View */}
                <div
                    ref={logContainer}
                    class="bg-slate-950 text-slate-300 font-mono text-xs p-4 rounded-lg h-[500px] overflow-y-auto whitespace-pre-wrap leading-relaxed shadow-inner border border-slate-800 scrollbar-thin scrollbar-thumb-slate-700 scrollbar-track-transparent"
                >
                    <For each={logs()}>{(log) =>
                        <div class="border-b border-slate-900/50 last:border-0 pb-0.5 mb-0.5 hover:bg-slate-900/50 px-1 -mx-1 rounded">
                            {/* Basic highlighting for log levels */}
                            {log.includes('"level":"error"') || log.includes('ERR') || log.includes('[ERROR]') ? (
                                <span class="text-red-400">{log}</span>
                            ) : log.includes('"level":"warn"') || log.includes('WRN') || log.includes('[WARN]') ? (
                                <span class="text-amber-400">{log}</span>
                            ) : log.includes('"level":"debug"') || log.includes('DBG') || log.includes('[DEBUG]') ? (
                                <span class="text-blue-400">{log}</span>
                            ) : (
                                <span class="text-slate-300">{log}</span>
                            )}
                        </div>
                    }</For>

                    {logs().length === 0 && !isLoading() && (
                        <div class="h-full flex flex-col items-center justify-center text-slate-600">
                            <Terminal size={48} class="mb-4 opacity-50" />
                            <p>Waiting for logs...</p>
                        </div>
                    )}
                </div>

                <div class="text-xs text-slate-500 dark:text-slate-400 flex justify-between px-1">
                    <span>Buffer: {logs().length} / {MAX_LOGS} lines</span>
                    <span class="flex items-center gap-2">
                        <div class={`w-2 h-2 rounded-full ${isPaused() ? 'bg-amber-500' : 'bg-green-500 animate-pulse'}`}></div>
                        {isPaused() ? "Stream Paused" : "Live"}
                    </span>
                </div>
            </SettingsPanel>
        </div>
    );
};
