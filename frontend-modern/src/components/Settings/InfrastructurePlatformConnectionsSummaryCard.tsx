import type { Component } from 'solid-js';
import { Card } from '@/components/shared/Card';

interface InfrastructurePlatformConnectionsSummaryCardProps {
  pveCount: number;
  pbsCount: number;
  pmgCount: number;
  truenasCount: number;
  truenasAvailable: boolean;
  onManagePlatformConnections: () => void;
}

export const InfrastructurePlatformConnectionsSummaryCard: Component<
  InfrastructurePlatformConnectionsSummaryCardProps
> = (props) => {
  return (
    <Card padding="lg" class="rounded-xl border border-border shadow-sm">
      <div class="space-y-4">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-base font-semibold text-base-content">Platform connections</h3>
            <p class="text-sm text-muted">
              Manage the API-backed platforms Pulse polls directly. Proxmox VE, PBS, PMG, and
              TrueNAS all live in the same shared platform-connections workspace.
            </p>
          </div>
          <button
            type="button"
            onClick={props.onManagePlatformConnections}
            class="inline-flex min-h-10 sm:min-h-9 items-center justify-center rounded-md border border-border px-3 py-2 text-sm font-medium text-base-content transition-colors hover:bg-surface-hover"
          >
            Open platform connections
          </button>
        </div>

        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div
            class="rounded-lg border border-border bg-surface-alt px-4 py-3"
            data-testid="platform-connections-pve"
          >
            <div class="text-sm font-medium text-base-content">PVE</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pveCount}</div>
          </div>
          <div
            class="rounded-lg border border-border bg-surface-alt px-4 py-3"
            data-testid="platform-connections-pbs"
          >
            <div class="text-sm font-medium text-base-content">PBS</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pbsCount}</div>
          </div>
          <div
            class="rounded-lg border border-border bg-surface-alt px-4 py-3"
            data-testid="platform-connections-pmg"
          >
            <div class="text-sm font-medium text-base-content">PMG</div>
            <div class="mt-1 text-xl font-semibold text-base-content">{props.pmgCount}</div>
          </div>
          <div
            class="rounded-lg border border-border bg-surface-alt px-4 py-3"
            data-testid="platform-connections-truenas"
          >
            <div class="text-sm font-medium text-base-content">TrueNAS</div>
            <div
              class={`mt-1 ${props.truenasAvailable ? 'text-xl font-semibold text-base-content' : 'text-sm font-medium text-muted'}`}
            >
              {props.truenasAvailable ? props.truenasCount : 'Disabled'}
            </div>
            <p class="mt-1 text-xs text-muted">
              {props.truenasAvailable
                ? 'API-backed NAS connections'
                : 'Enable the TrueNAS integration to add API-backed NAS systems.'}
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
};

export default InfrastructurePlatformConnectionsSummaryCard;
