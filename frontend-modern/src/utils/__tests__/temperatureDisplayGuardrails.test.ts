import { describe, expect, it } from 'vitest';

import nodeDrawerOverviewSource from '@/components/Workloads/NodeDrawerOverview.tsx?raw';
import temperatureGaugeSource from '@/components/shared/TemperatureGauge.tsx?raw';
import proxmoxNodesTableSource from '@/features/proxmox/ProxmoxNodesTable.tsx?raw';

// Docker host and agent machine temperatures have render tests that use the
// alert thresholds; these surfaces do not yet.
describe('temperature display guardrails', () => {
  it('keeps colored temperature displays threshold-aware', () => {
    expect(proxmoxNodesTableSource).toContain('temperatureThresholds={temperatureThresholds()}');
    expect(nodeDrawerOverviewSource).toContain('getTemperatureTextClass(value, thresholds)');
    expect(nodeDrawerOverviewSource).not.toContain('getTemperatureTextClass(value)');
    expect(temperatureGaugeSource).toContain('props.metric');
  });
});
