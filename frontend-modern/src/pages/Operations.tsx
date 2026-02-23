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
                <h1 class="text-3xl font-bold tracking-tight text-base-content mb-2">Operations</h1>
                <p class="text-muted mt-1">Platform diagnostics, system logs, and data exports.</p>
            </div>

            {/* Modern Tabs Navigation */}
            <div class="mb-6">
                <nav class="flex space-x-2 bg-surface-alt p-1.5 rounded-md sm:w-max border border-border overflow-x-auto scrollbar-hide" aria-label="Tabs" style="-webkit-overflow-scrolling: touch;">
                    {tabs.map((tab) => {
                        const isActive = () => activeTab() === tab.id;
                        return (
                            <button
                                onClick={() => handleTabChange(tab.id)}
                                class={`flex items-center gap-2.5 whitespace-nowrap px-4 py-2 rounded-md font-medium text-sm transition-all outline-none relative overflow-hidden group ${isActive()
 ? 'bg-surface text-base-content shadow-sm border border-border'
 : 'text-muted hover:bg-surface hover:text-base-content border border-transparent'
 }`}
                                aria-current={isActive() ? 'page' : undefined}
                                title={tab.desc}
                            >
                                <tab.icon class={`w-4 h-4 transition-transform duration-200 ${isActive() ? 'text-blue-500 scale-110' : 'text-muted group-hover:scale-110 group-hover:text-blue-500'}`} />
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
