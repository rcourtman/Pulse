import { Component, For, Show, createEffect, createMemo, createSignal } from 'solid-js';
import { useLocation, useNavigate } from '@solidjs/router';
import { Dialog } from '@/components/shared/Dialog';
import { presentationPolicyIsReadOnly } from '@/stores/sessionPresentationPolicy';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import { ADD_SYSTEM_CHOICES, type AddSystemChoice } from './AddSystemPicker';
import { ConnectionsTable, type ConnectionsTableHeaderAction } from './ConnectionsTable';
import {
  buildInfrastructureSystemRows,
  type InfrastructureSystemRow,
  type SystemManageAction,
} from './connectionsTableModel';
import { InfrastructureActiveRowDetails } from './InfrastructureActiveRowDetails';
import { InfrastructureInstallerSection } from './InfrastructureInstallerSection';
import { InfrastructureIgnoredRowDetails } from './InfrastructureIgnoredRowDetails';
import { InfrastructureStopMonitoringDialog } from './InfrastructureStopMonitoringDialog';
import { ProxmoxSettingsPanel } from './ProxmoxSettingsPanel';
import { TrueNASSettingsPanel } from './TrueNASSettingsPanel';
import { VMwareSettingsPanel } from './VMwareSettingsPanel';
import {
  buildInfrastructureWorkspacePath,
  deriveAddStepFromLegacyPath,
  type InfrastructureAddStep,
  type InfrastructurePanelStep,
} from './infrastructureWorkspaceModel';
import type { InfrastructurePlatformSettingsProps } from './proxmoxSettingsModel';
import type { NodeType } from './infrastructureSettingsModel';
import {
  InfrastructureOperationsStateProvider,
  useInfrastructureOperationsContext,
} from './useInfrastructureOperationsState';

export type InfrastructureWorkspaceProps = InfrastructurePlatformSettingsProps;

const STEP_TITLES: Record<InfrastructureAddStep, string> = {
  agent: 'Install on a host',
  pve: 'Proxmox VE',
  pbs: 'Proxmox Backup Server',
  pmg: 'Proxmox Mail Gateway',
  truenas: 'TrueNAS SCALE',
  vmware: 'VMware vSphere / ESXi',
};

const InfrastructureWorkspaceContent: Component<InfrastructureWorkspaceProps> = (props) => {
  const navigate = useNavigate();
  const location = useLocation();
  const state = useInfrastructureOperationsContext();

  const [panelStep, setPanelStep] = createSignal<InfrastructurePanelStep | null>(null);
  const [showAgentProfiles, setShowAgentProfiles] = createSignal(false);
  const readOnly = createMemo(() => presentationPolicyIsReadOnly());

  // Redirect legacy deep links and open the appropriate panel.
  createEffect(() => {
    const path = location.pathname;
    if (path === '/settings/infrastructure') return;

    const step = deriveAddStepFromLegacyPath(path);
    navigate(buildInfrastructureWorkspacePath(), { replace: true });
    if (step && !readOnly()) {
      setPanelStep(step);
    }
  });

  // Auto-open the agent installer when a setup handoff is waiting.
  createEffect(() => {
    if (state.setupHandoff?.() && panelStep() === null && !readOnly()) {
      setPanelStep('agent');
    }
  });

  // Close add panel in read-only mode.
  createEffect(() => {
    if (readOnly() && panelStep() !== null) {
      setPanelStep(null);
    }
  });

  const rows = createMemo<InfrastructureSystemRow[]>(() =>
    buildInfrastructureSystemRows({
      activeRows: state.activeRows(),
      monitoringStoppedRows: state.monitoringStoppedRows(),
    }),
  );

  const headerActions = createMemo<ConnectionsTableHeaderAction[]>(() =>
    readOnly()
      ? []
      : [
          {
            label: 'Add infrastructure',
            onSelect: () => {
              setPanelStep('pick');
              setShowAgentProfiles(false);
            },
            tone: 'primary' as const,
          },
        ],
  );

  const handleManageAction = (action: SystemManageAction) => {
    switch (action.kind) {
      case 'inventory-active':
        state.setExpandedRowKey(action.rowKey);
        state.setSelectedIgnoredRowKey(null);
        return;
      case 'inventory-ignored':
        state.setExpandedRowKey(null);
        state.setSelectedIgnoredRowKey(action.rowKey);
        return;
      default:
        return;
    }
  };

  const handleAddSystem = (choice: AddSystemChoice) => {
    setShowAgentProfiles(false);
    setPanelStep(choice.kind as InfrastructureAddStep);
  };

  const closePanel = () => {
    setPanelStep(null);
    setShowAgentProfiles(false);
  };

  const goBackToPick = () => {
    setShowAgentProfiles(false);
    setPanelStep('pick');
  };

  // For Proxmox: selectedAgent is derived from the active panel step.
  const proxmoxAgent = createMemo<NodeType>(() => {
    const step = panelStep();
    if (step === 'pbs') return 'pbs';
    if (step === 'pmg') return 'pmg';
    return 'pve';
  });

  const onSelectProxmoxAgent = (agent: NodeType) => {
    setPanelStep(agent as InfrastructureAddStep);
  };

  const isProxmoxStep = () => {
    const step = panelStep();
    return step === 'pve' || step === 'pbs' || step === 'pmg';
  };

  const stepTitle = () => {
    const step = panelStep();
    if (!step || step === 'pick') return 'Add infrastructure';
    return STEP_TITLES[step];
  };

  return (
    <div class="space-y-8">
      <ConnectionsTable
        rows={rows}
        headerActions={headerActions()}
        onManageRow={(row) => handleManageAction(row.manage)}
      />

      {/* Add-infrastructure panel — right drawer */}
      <Dialog
        isOpen={panelStep() !== null}
        onClose={closePanel}
        layout="drawer-right"
        panelClass="max-w-[820px]"
        ariaLabel={stepTitle()}
      >
        <div class="flex h-full flex-col overflow-hidden">
          {/* Drawer header */}
          <div class="flex shrink-0 items-center justify-between gap-3 border-b border-border px-4 py-3">
            <div class="flex items-center gap-2">
              <Show when={panelStep() !== 'pick'}>
                <button
                  type="button"
                  onClick={goBackToPick}
                  class="inline-flex items-center gap-1 rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                >
                  ← Choose different type
                </button>
              </Show>
              <span class="text-sm font-semibold text-base-content">{stepTitle()}</span>
            </div>
            <div class="flex items-center gap-2">
              <Show when={panelStep() === 'agent'}>
                <button
                  type="button"
                  onClick={() => setShowAgentProfiles((v) => !v)}
                  class="inline-flex items-center rounded-md border border-border px-2.5 py-1 text-xs font-medium text-base-content transition-colors hover:bg-surface-hover"
                >
                  {showAgentProfiles() ? 'Hide agent profiles' : 'Manage agent profiles'}
                </button>
              </Show>
              <button
                type="button"
                onClick={closePanel}
                class="rounded-md p-1 text-muted hover:bg-surface-hover hover:text-base-content"
                aria-label="Close"
              >
                <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </button>
            </div>
          </div>

          {/* Drawer content */}
          <div class="flex-1 overflow-y-auto">
            {/* Step 1: picker */}
            <Show when={panelStep() === 'pick'}>
              <div class="space-y-4 p-4">
                <p class="text-sm text-muted">
                  Choose the system or platform you want Pulse to start monitoring.
                </p>
                <ul class="divide-y divide-border rounded-md border border-border">
                  <For each={ADD_SYSTEM_CHOICES}>
                    {(choice) => (
                      <li>
                        <button
                          type="button"
                          onClick={() => handleAddSystem(choice)}
                          class="flex w-full items-start justify-between gap-4 px-4 py-3 text-left hover:bg-surface-hover"
                        >
                          <div>
                            <div class="text-sm font-semibold text-base-content">
                              {choice.title}
                            </div>
                            <div class="mt-0.5 text-xs text-muted">{choice.description}</div>
                          </div>
                          <span
                            class={`inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium ${
                              choice.method === 'api'
                                ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-100'
                                : 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-100'
                            }`}
                          >
                            {choice.methodLabel}
                          </span>
                        </button>
                      </li>
                    )}
                  </For>
                </ul>
              </div>
            </Show>

            {/* Step 2a: agent installer */}
            <Show when={panelStep() === 'agent'}>
              <div class="space-y-4 p-4">
                <Show when={showAgentProfiles()}>
                  <div class="rounded-xl border border-border bg-surface p-4 shadow-sm">
                    <div class="mb-4 space-y-1">
                      <div class="text-base font-semibold text-base-content">Agent profiles</div>
                      <div class="text-sm text-muted">
                        Manage reusable install defaults for agent-based systems.
                      </div>
                    </div>
                    <AgentProfilesPanel />
                  </div>
                </Show>
                <InfrastructureInstallerSection />
              </div>
            </Show>

            {/* Step 2b: Proxmox (PVE / PBS / PMG) */}
            <Show when={isProxmoxStep()}>
              <div class="p-4">
                <ProxmoxSettingsPanel
                  {...props}
                  selectedAgent={proxmoxAgent}
                  onSelectAgent={onSelectProxmoxAgent}
                  embedded
                />
              </div>
            </Show>

            {/* Step 2c: TrueNAS */}
            <Show when={panelStep() === 'truenas'}>
              <div class="p-4">
                <TrueNASSettingsPanel state={props.trueNASSettings} />
              </div>
            </Show>

            {/* Step 2d: VMware */}
            <Show when={panelStep() === 'vmware'}>
              <div class="p-4">
                <VMwareSettingsPanel state={props.vmwareSettings} />
              </div>
            </Show>
          </div>
        </div>
      </Dialog>

      {/* Active system detail drawer */}
      <Dialog
        isOpen={Boolean(state.selectedActiveRow())}
        onClose={() => state.setExpandedRowKey(null)}
        layout="drawer-right"
        panelClass="max-w-[760px]"
        ariaLabel="Reporting item details"
      >
        <Show when={state.selectedActiveRow()}>
          {(rowAccessor) => <InfrastructureActiveRowDetails rowAccessor={rowAccessor} />}
        </Show>
      </Dialog>

      {/* Ignored system detail drawer */}
      <Dialog
        isOpen={Boolean(state.selectedIgnoredRow())}
        onClose={() => state.setSelectedIgnoredRowKey(null)}
        layout="drawer-right"
        panelClass="max-w-[760px]"
        ariaLabel="Ignored item details"
      >
        <Show when={state.selectedIgnoredRow()}>
          {(rowAccessor) => <InfrastructureIgnoredRowDetails rowAccessor={rowAccessor} />}
        </Show>
      </Dialog>

      <InfrastructureStopMonitoringDialog />
    </div>
  );
};

export const InfrastructureWorkspace: Component<InfrastructureWorkspaceProps> = (props) => {
  return (
    <InfrastructureOperationsStateProvider embedded>
      <InfrastructureWorkspaceContent {...props} />
    </InfrastructureOperationsStateProvider>
  );
};
