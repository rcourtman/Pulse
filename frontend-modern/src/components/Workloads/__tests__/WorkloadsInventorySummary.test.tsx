import { cleanup, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';
import { WorkloadsInventorySummary } from '../WorkloadsInventorySummary';

describe('WorkloadsInventorySummary', () => {
  afterEach(cleanup);

  it('shows labelled estate totals and two explicit distributions', () => {
    render(() => (
      <WorkloadsInventorySummary
        containerLabel="LXCs"
        topology={{ nodes: 32, clusters: 5, standalone: 2 }}
        stats={{
          total: 718,
          running: 635,
          degraded: 36,
          stopped: 47,
          vms: 283,
          containers: 300,
          appContainers: 15,
          pods: 120,
        }}
      />
    ));

    const summary = screen.getByRole('region', { name: 'Estate overview' });
    expect(summary).toHaveTextContent('718Workloads');
    expect(summary).toHaveTextContent('32Nodes');
    expect(summary).toHaveTextContent('5Clusters');
    expect(summary).toHaveTextContent('2Standalone');
    expect(summary).toHaveTextContent('283 VMs');
    expect(summary).toHaveTextContent('315 LXCs');
    expect(summary).toHaveTextContent('120 pods');
    expect(
      within(summary).getByRole('img', {
        name: 'Workload mix: 283 VMs, 315 LXCs, 120 pods',
      }),
    ).toBeInTheDocument();
    expect(
      within(summary).getByRole('img', {
        name: 'Health: 635 running, 36 attention, 47 stopped',
      }),
    ).toBeInTheDocument();
  });

  it('omits empty type categories without losing status context', () => {
    render(() => (
      <WorkloadsInventorySummary
        stats={{
          total: 10,
          running: 10,
          degraded: 0,
          stopped: 0,
          vms: 10,
          containers: 0,
          appContainers: 0,
          pods: 0,
        }}
      />
    ));

    const summary = screen.getByRole('region', { name: 'Estate overview' });
    expect(summary).toHaveTextContent('10 VMs');
    expect(summary).not.toHaveTextContent('containers');
    expect(summary).not.toHaveTextContent('pods');
    expect(summary).toHaveTextContent('10Running');
  });
});
