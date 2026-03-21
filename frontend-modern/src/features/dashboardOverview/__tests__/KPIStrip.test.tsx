import { describe, expect, it } from 'vitest';
import { render, screen } from '@solidjs/testing-library';
import { KPIStrip } from '../KPIStrip';

describe('KPIStrip', () => {
  it('renders alert summary counts', () => {
    render(() => (
      <KPIStrip
        infrastructure={{ total: 8, online: 7 }}
        workloads={{ total: 12, running: 10 }}
        storage={{ capacityPercent: 63, totalUsed: 630, totalCapacity: 1000 }}
        alerts={{ activeCritical: 2, activeWarning: 3, total: 5 }}
      />
    ));

    expect(screen.getByText('Alerts')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText(/critical ·/)).toBeInTheDocument();
  });
});
