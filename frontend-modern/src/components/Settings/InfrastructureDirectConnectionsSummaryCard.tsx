import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';

interface InfrastructureDirectConnectionsSummaryCardProps {
  pveCount: number;
  pbsCount: number;
  pmgCount: number;
  onManageDirectConnections: () => void;
}

export const InfrastructureDirectConnectionsSummaryCard: Component<
  InfrastructureDirectConnectionsSummaryCardProps
> = (props) => {
  return (
    <Card padding="lg" class="rounded-xl border border-border shadow-sm">
      <div class="space-y-4">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-base font-semibold text-base-content">Direct Proxmox connections</h3>
            <p class="text-sm text-muted">
              Review fallback Proxmox coverage separately from agent-managed hosts, then open the
              shared platform-connections workspace for Proxmox and TrueNAS integrations.
            </p>
          </div>
          <button
            type="button"
            onClick={props.onManageDirectConnections}
            class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
          >
            Open platform connections
          </button>
        </div>

        <div class="grid gap-3 sm:grid-cols-3">
          <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
            <div class="text-sm font-medium text-base-content">PVE</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pveCount}</div>
          </div>
          <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
            <div class="text-sm font-medium text-base-content">PBS</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pbsCount}</div>
          </div>
          <div class="rounded-lg border border-border bg-surface-alt px-4 py-3">
            <div class="text-sm font-medium text-base-content">PMG</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pmgCount}</div>
          </div>
        </div>
      </div>
    </Card>
  );
};

export default InfrastructureDirectConnectionsSummaryCard;
