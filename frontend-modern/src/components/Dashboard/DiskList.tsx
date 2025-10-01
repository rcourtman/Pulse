import { For, Show, createSignal } from 'solid-js';
import type { Disk } from '@/types/api';
import { formatBytes } from '@/utils/format';
import { MetricBar } from './MetricBar';

interface DiskListProps {
  disks: Disk[];
  diskStatusReason?: string;
}

export function DiskList(props: DiskListProps) {
  const [expanded, setExpanded] = createSignal(false);

  const getDiskStatusTooltip = () => {
    const reason = props.diskStatusReason;

    switch (reason) {
      case 'agent-not-running':
        return 'Guest agent not running. Install and start qemu-guest-agent in the VM.';
      case 'agent-timeout':
        return 'Guest agent timeout. Agent may need to be restarted.';
      case 'permission-denied':
        return 'Permission denied. Check that your Pulse user/token has VM.Monitor permission (PVE 8) or VM.GuestAgent.Audit permission (PVE 9).';
      case 'agent-disabled':
        return 'Guest agent is disabled in VM configuration. Enable it in VM Options.';
      case 'no-filesystems':
        return 'No filesystems found. VM may be booting or using a Live ISO.';
      case 'special-filesystems-only':
        return 'Only special filesystems detected (ISO/squashfs). This is normal for Live systems.';
      case 'agent-error':
        return 'Error communicating with guest agent.';
      case 'no-data':
        return 'No disk data available from Proxmox API.';
      default:
        return 'Disk stats unavailable. Guest agent may not be installed.';
    }
  };

  return (
    <Show
      when={props.disks && props.disks.length > 0}
      fallback={
        <span class="text-gray-400 text-sm cursor-help" title={getDiskStatusTooltip()}>
          -
        </span>
      }
    >
      <div class="flex flex-col gap-1">
        {/* Show first disk or aggregated view when collapsed */}
        <Show when={!expanded() && props.disks.length === 1}>
          <MetricBar
            value={(props.disks[0].used / props.disks[0].total) * 100}
            label={`${((props.disks[0].used / props.disks[0].total) * 100).toFixed(0)}%`}
            sublabel={`${formatBytes(props.disks[0].used)}/${formatBytes(props.disks[0].total)}`}
            type="disk"
          />
        </Show>

        <Show when={!expanded() && props.disks.length > 1}>
          <div class="flex items-center gap-2">
            <MetricBar
              value={
                (props.disks.reduce((acc, d) => acc + d.used, 0) /
                  props.disks.reduce((acc, d) => acc + d.total, 0)) *
                100
              }
              label={`${(
                (props.disks.reduce((acc, d) => acc + d.used, 0) /
                  props.disks.reduce((acc, d) => acc + d.total, 0)) *
                100
              ).toFixed(0)}%`}
              sublabel={`${formatBytes(
                props.disks.reduce((acc, d) => acc + d.used, 0)
              )}/${formatBytes(props.disks.reduce((acc, d) => acc + d.total, 0))}`}
              type="disk"
            />
            <button
              onClick={() => setExpanded(true)}
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline whitespace-nowrap"
              title="Show individual disks"
            >
              {props.disks.length} disks
            </button>
          </div>
        </Show>

        {/* Expanded view showing all individual disks */}
        <Show when={expanded()}>
          <div class="flex flex-col gap-1">
            <For each={props.disks}>
              {(disk) => (
                <div class="flex items-center gap-2">
                  <div class="flex-1">
                    <MetricBar
                      value={(disk.used / disk.total) * 100}
                      label={`${((disk.used / disk.total) * 100).toFixed(0)}%`}
                      sublabel={`${formatBytes(disk.used)}/${formatBytes(disk.total)}`}
                      type="disk"
                    />
                  </div>
                  <span
                    class="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap"
                    title={disk.type ? `${disk.type} filesystem` : undefined}
                  >
                    {disk.mountpoint || disk.device || 'Unknown'}
                  </span>
                </div>
              )}
            </For>
            <button
              onClick={() => setExpanded(false)}
              class="text-xs text-blue-600 dark:text-blue-400 hover:underline text-left"
            >
              Collapse
            </button>
          </div>
        </Show>
      </div>
    </Show>
  );
}
