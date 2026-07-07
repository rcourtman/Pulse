import { describe, expect, it } from 'vitest';

import nodeDrawerOverviewSource from '@/components/Workloads/NodeDrawerOverview.tsx?raw';
import workloadPanelSource from '@/components/Workloads/WorkloadPanel.tsx?raw';
import nodeGroupHeaderSource from '@/components/shared/NodeGroupHeader.tsx?raw';
import temperatureGaugeSource from '@/components/shared/TemperatureGauge.tsx?raw';
import dockerHostDrawerOverviewSource from '@/features/docker/DockerHostDrawerOverview.tsx?raw';
import agentsMachinesTableSource from '@/features/standalone/AgentsMachinesTable.tsx?raw';

describe('temperature display guardrails', () => {
  it('keeps colored temperature displays threshold-aware', () => {
    expect(nodeGroupHeaderSource).toContain('temperatureThresholds');
    expect(workloadPanelSource).toContain('getNodeTemperatureThresholds(node)');
    expect(nodeDrawerOverviewSource).toContain('getTemperatureTextClass(value, thresholds)');
    expect(dockerHostDrawerOverviewSource).toContain('temperatureThresholds()');
    expect(agentsMachinesTableSource).toContain('getAgentMachineTemperatureMetric');
    expect(agentsMachinesTableSource).toContain('thresholds={temperatureThresholds()}');
    expect(temperatureGaugeSource).toContain('props.metric');
  });

  it('does not reintroduce the previous local temperature severity calls', () => {
    expect(nodeGroupHeaderSource).not.toContain('getTemperatureTextClass(cpuTemperature())');
    expect(workloadPanelSource).not.toContain('getTemperatureTextClass(temperature)}');
    expect(nodeDrawerOverviewSource).not.toContain('getTemperatureTextClass(value)');
    expect(dockerHostDrawerOverviewSource).not.toContain('getTemperatureTextClass(temperature)');
    expect(agentsMachinesTableSource).not.toContain('<TemperatureGauge value={value()} />');
  });
});
