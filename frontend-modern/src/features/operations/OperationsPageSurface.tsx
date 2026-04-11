import { Show, Suspense, createEffect, createMemo, type Component, type JSX } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import ActivityIcon from 'lucide-solid/icons/activity';
import FileTextIcon from 'lucide-solid/icons/file-text';
import TerminalIcon from 'lucide-solid/icons/terminal';
import { DiagnosticsPanel } from '@/components/Settings/DiagnosticsPanel';
import { ReportingPanel } from '@/components/Settings/ReportingPanel';
import { SystemLogsPanel } from '@/components/Settings/SystemLogsPanel';
import { Subtabs, type SubtabOption } from '@/components/shared/Subtabs';
import { DASHBOARD_PATH } from '@/routing/resourceLinks';
import { presentationPolicyIsDemoMode } from '@/stores/sessionPresentationPolicy';
import {
  buildOperationsPath,
  getOperationsTabFromPath,
  operationsSurfaceHiddenInDemoMode,
  OPERATIONS_TABS,
  type OperationsTabId,
} from '@/features/operations/operationsPageModel';

const operationsTabIcons: Record<OperationsTabId, Component<{ class?: string }>> = {
  diagnostics: ActivityIcon,
  reporting: FileTextIcon,
  logs: TerminalIcon,
};

export function OperationsPageSurface() {
  const location = useLocation();
  const navigate = useNavigate();

  const hiddenInDemoMode = createMemo(() =>
    operationsSurfaceHiddenInDemoMode(presentationPolicyIsDemoMode()),
  );
  const activeTab = createMemo(() => getOperationsTabFromPath(location.pathname));

  createEffect(() => {
    if (hiddenInDemoMode()) {
      navigate(DASHBOARD_PATH, { replace: true });
    }
  });

  const tabs = createMemo<SubtabOption[]>(() =>
    hiddenInDemoMode()
      ? []
      : OPERATIONS_TABS.map((tab) => {
          const Icon = operationsTabIcons[tab.id];
          return {
            value: tab.id,
            label: (
              <span class="inline-flex items-center gap-2.5" title={tab.description}>
                <Icon class="h-4 w-4" />
                <span>{tab.label}</span>
              </span>
            ) satisfies JSX.Element,
          };
        }),
  );

  const handleTabChange = (tabId: string) => {
    navigate(buildOperationsPath(tabId as OperationsTabId));
  };

  return (
    <Show when={!hiddenInDemoMode()}>
      <div class="space-y-6">
        <div class="mb-6">
          <Subtabs
            value={activeTab()}
            onChange={handleTabChange}
            tabs={tabs()}
            ariaLabel="Operations"
            class="rounded-md border border-border bg-surface-alt p-1.5 sm:w-max"
            listClass="gap-2 overflow-x-auto scrollbar-hide"
            tabClass="min-h-10 whitespace-nowrap rounded-md border border-transparent px-4 py-2 text-sm"
          />
        </div>

        <div class="mt-4 animate-fade-in animate-duration-200">
          <Suspense
            fallback={
              <div class="flex justify-center p-6">
                <div class="h-6 w-6 animate-spin rounded-full border-2 border-blue-500 border-t-transparent"></div>
              </div>
            }
          >
            {activeTab() === 'diagnostics' && <DiagnosticsPanel />}
            {activeTab() === 'reporting' && <ReportingPanel />}
            {activeTab() === 'logs' && <SystemLogsPanel />}
          </Suspense>
        </div>
      </div>
    </Show>
  );
}

export default OperationsPageSurface;
