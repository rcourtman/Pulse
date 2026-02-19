import { Component, createSignal, createEffect, Suspense } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import FileTextIcon from 'lucide-solid/icons/file-text';
import TerminalIcon from 'lucide-solid/icons/terminal';

// Import the panels
import { DiagnosticsPanel } from '@/components/Settings/DiagnosticsPanel';
import { ReportingPanel } from '@/components/Settings/ReportingPanel';
import { SystemLogsPanel } from '@/components/Settings/SystemLogsPanel';

type OperationsTabId = 'diagnostics' | 'reporting' | 'logs';

export const OperationsPage: Component = () => {
    const location = useLocation();
    const navigate = useNavigate();

    // Parse active tab from URL path
    const getActiveTab = (): OperationsTabId => {
        const path = location.pathname.split('/').pop() || '';
        if (path === 'reporting') return 'reporting';
        if (path === 'logs') return 'logs';
        return 'diagnostics'; // default
    };

    const [activeTab, setActiveTabSignal] = createSignal<OperationsTabId>(getActiveTab());

    createEffect(() => {
        setActiveTabSignal(getActiveTab());
    });

    const handleTabChange = (tabId: OperationsTabId) => {
        navigate(`/operations/${tabId}`);
    };

    const tabs = [
        {
            id: 'diagnostics' as OperationsTabId,
            label: 'Diagnostics & Health',
            icon: ActivityIcon,
            desc: 'System health, connection tests, and troubleshooting',
        },
        {
            id: 'reporting' as OperationsTabId,
            label: 'Data Export & Reports',
            icon: FileTextIcon,
            desc: 'Export system metrics and configuration data',
        },
        {
            id: 'logs' as OperationsTabId,
            label: 'System Logs',
            icon: TerminalIcon,
            desc: 'View real-time Pulse system logs',
        },
    ];

    return (
        <div class="space-y-6">
            <div class="mb-4">
                <h1 class="text-3xl font-bold tracking-tight text-slate-900 dark:text-white mb-2">Operations</h1>
                <p class="text-slate-500 dark:text-slate-400 mt-1">Platform diagnostics, system logs, and data exports.</p>
            </div>

            {/* Modern Tabs Navigation */}
            <div class="mb-6">
                <nav class="flex space-x-2 bg-slate-100/80 dark:bg-slate-800/80 p-1.5 rounded-2xl sm:w-max backdrop-blur-sm border border-slate-200 dark:border-slate-700/50 shadow-inner overflow-x-auto scrollbar-hide" aria-label="Tabs" style="-webkit-overflow-scrolling: touch;">
                    {tabs.map((tab) => {
                        const isActive = () => activeTab() === tab.id;
                        return (
                            <button
                                onClick={() => handleTabChange(tab.id)}
                                class={`flex items-center gap-2.5 whitespace-nowrap px-4 py-2 rounded-xl font-medium text-sm transition-all outline-none relative overflow-hidden group ${isActive()
                                    ? 'bg-white text-blue-700 dark:bg-slate-700 dark:text-blue-300 shadow-sm border border-slate-200/50 dark:border-slate-600/50'
                                    : 'text-slate-600 hover:bg-white/60 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-700/50 dark:hover:text-slate-200'
                                    }`}
                                aria-current={isActive() ? 'page' : undefined}
                                title={tab.desc}
                            >
                                <tab.icon class={`w-4 h-4 transition-transform duration-200 ${isActive() ? 'text-blue-500 scale-110' : 'text-slate-400 group-hover:scale-110 group-hover:text-blue-500'}`} />
                                <span class="relative z-10">{tab.label}</span>
                            </button>
                        );
                    })}
                </nav>
            </div>

            {/* View Content */}
            <div class="mt-4 animate-fade-in animate-duration-200">
                <Suspense fallback={<div class="p-6 flex justify-center"><div class="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full"></div></div>}>
                    {activeTab() === 'diagnostics' && <DiagnosticsPanel />}
                    {activeTab() === 'reporting' && <ReportingPanel />}
                    {activeTab() === 'logs' && <SystemLogsPanel />}
                </Suspense>
            </div>
        </div>
    );
};

export default OperationsPage;
