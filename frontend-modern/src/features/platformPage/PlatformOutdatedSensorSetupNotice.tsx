import { Show, createMemo } from 'solid-js';
import { AlertTriangle, ArrowRight } from 'lucide-solid';
import type { OutdatedSensorSetupNode } from './sensorSetup';

type PlatformOutdatedSensorSetupNoticeProps = {
  // Nodes whose SSH temperature setup predates the pulse-sensors wrapper.
  nodes: OutdatedSensorSetupNode[];
  actionHref?: string;
  actionLabel?: string;
};

// Inline notice shown on the Proxmox page when a node's SSH temperature
// monitoring was set up by a pre-v6.0.0-rc.6 setup script. That setup locks
// the SSH key to `sensors -j`, so SATA/SAS disk temperatures silently never
// appear (only NVMe shows, via kernel hwmon). Rendered only when an affected
// node actually has SATA/SAS disks without temperatures, so the page stays
// clean in the healthy case. Sibling of PlatformOutdatedAgentNotice.
export function PlatformOutdatedSensorSetupNotice(props: PlatformOutdatedSensorSetupNoticeProps) {
  const count = createMemo(() => props.nodes.length);
  const names = createMemo(() => props.nodes.map((node) => node.name).join(', '));
  const actionLabel = createMemo(() => props.actionLabel || 'Open Infrastructure settings');

  const message = createMemo(() => {
    if (count() === 1) {
      return `${props.nodes[0].name} is using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures. Re-run the node setup script to upgrade it.`;
    }
    return `${count()} nodes are using an older temperature monitoring setup that cannot read SATA/SAS disk temperatures. Re-run the node setup script on each to upgrade them. Affected: ${names()}.`;
  });

  return (
    <Show when={count() > 0}>
      <div
        role="status"
        data-testid="platform-outdated-sensor-setup-notice"
        class="flex items-start gap-2 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-200"
      >
        <AlertTriangle class="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <div class="min-w-0 flex-1 space-y-1">
          <div>{message()}</div>
          <Show when={props.actionHref}>
            {(href) => (
              <a
                href={href()}
                class="inline-flex items-center gap-1 text-xs font-semibold text-amber-900 underline-offset-2 hover:underline dark:text-amber-100"
              >
                <span>{actionLabel()}</span>
                <ArrowRight class="h-3.5 w-3.5" aria-hidden="true" />
              </a>
            )}
          </Show>
        </div>
      </div>
    </Show>
  );
}

export default PlatformOutdatedSensorSetupNotice;
