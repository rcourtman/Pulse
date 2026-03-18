import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';
import { AgentProfilesPanel } from './AgentProfilesPanel';
import type { ProxmoxSettingsPanelProps } from './ProxmoxSettingsPanel';
import { InfrastructureOperationsController } from './InfrastructureOperationsController';

interface InfrastructureReportingPanelProps extends ProxmoxSettingsPanelProps {
  onManageDirectConnections: () => void;
}

export const InfrastructureReportingPanel: Component<InfrastructureReportingPanelProps> = (
  props,
) => (
  <div class="space-y-6">
    <InfrastructureOperationsController embedded showInstaller={false} />

    <div class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
      <Card padding="lg" class="rounded-xl border border-border shadow-sm">
        <div class="space-y-4">
          <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-base-content">Direct Proxmox connections</h3>
              <p class="text-sm text-muted">
                Review fallback direct coverage separately from agent-managed hosts.
              </p>
            </div>
            <button
              type="button"
              onClick={props.onManageDirectConnections}
              class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
            >
              Manage direct connections
            </button>
          </div>

          <div class="grid gap-3 sm:grid-cols-3">
            <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
              <div class="text-sm font-medium text-base-content">PVE</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {props.pveNodes().length}
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
              <div class="text-sm font-medium text-base-content">PBS</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {props.pbsNodes().length}
              </div>
            </div>
            <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
              <div class="text-sm font-medium text-base-content">PMG</div>
              <div class="mt-1 text-xl font-semibold text-base-content">
                {props.pmgNodes().length}
              </div>
            </div>
          </div>
        </div>
      </Card>
    </div>

    <AgentProfilesPanel />
  </div>
);

export default InfrastructureReportingPanel;
