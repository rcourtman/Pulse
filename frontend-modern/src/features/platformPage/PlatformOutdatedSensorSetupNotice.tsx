import { Show, createMemo } from 'solid-js';
import { AlertTriangle, ArrowRight } from 'lucide-solid';
import { InlineNotice } from '@/components/shared/InlineNotice';
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
      <InlineNotice
        role="status"
        data-testid="platform-outdated-sensor-setup-notice"
        tone="warning"
        icon={<AlertTriangle aria-hidden="true" />}
        actionHref={props.actionHref}
        actionLabel={actionLabel()}
        actionIcon={<ArrowRight aria-hidden="true" />}
      >
        {message()}
      </InlineNotice>
    </Show>
  );
}

export default PlatformOutdatedSensorSetupNotice;
