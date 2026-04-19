import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { KPIStrip } from '../KPIStrip';

describe('KPIStrip', () => {
  it('keeps the workloads card explicitly discoverable for containers, VMs, and pods', () => {
    render(() => (
      <KPIStrip
        infrastructure={{ total: 5, online: 5 }}
        workloads={{ total: 33, running: 28 }}
        storage={{ capacityPercent: 27, totalUsed: 20, totalCapacity: 75 }}
        alerts={{ activeCritical: 1, activeWarning: 16, total: 17 }}
      />
    ));

    const workloadsLink = screen.getByRole('link', { name: /workloads/i });
    expect(workloadsLink.getAttribute('href')).toBe('/workloads');
    expect(screen.getByText('VMs, containers, and pods')).toBeInTheDocument();
    expect(screen.getByText('28')).toBeInTheDocument();
    expect(screen.getByText(/running/i)).toBeInTheDocument();
  });
});
