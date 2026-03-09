import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';
import { RecoverySummary } from './RecoverySummary';

describe('RecoverySummary', () => {
  it('renders an operational overview instead of mini summary charts', () => {
    render(() => (
      <RecoverySummary
        rollups={() => [
          {
            rollupId: 'alpha',
            lastOutcome: 'success',
            lastSuccessAt: '2026-03-09T11:30:00Z',
            providers: ['proxmox-pbs'],
          },
          {
            rollupId: 'beta',
            lastOutcome: 'failed',
            lastAttemptAt: '2026-03-08T11:00:00Z',
            lastSuccessAt: null,
            providers: ['proxmox-pve'],
          },
        ]}
        series={() => [
          { day: '2026-03-07', total: 1, snapshot: 1, local: 0, remote: 0 },
          { day: '2026-03-08', total: 3, snapshot: 1, local: 1, remote: 1 },
          { day: '2026-03-09', total: 2, snapshot: 1, local: 0, remote: 1 },
        ]}
        seriesLoaded={() => true}
        summary={() => ({
          total: 2,
          counts: {
            success: 1,
            warning: 0,
            failed: 1,
            running: 0,
            unknown: 0,
          },
          stale: 1,
          neverSucceeded: 1,
        })}
        timeRange={() => '30d'}
      />
    ));

    expect(screen.getByText('Recovery Overview')).toBeInTheDocument();
    expect(screen.getByText('Protection Posture')).toBeInTheDocument();
    expect(screen.getByText('Freshness Distribution')).toBeInTheDocument();
    expect(screen.getByText('Attention Queue')).toBeInTheDocument();
    expect(screen.getByText('2 protected items')).toBeInTheDocument();
    expect(screen.getByText('1 healthy')).toBeInTheDocument();
    expect(screen.getByText('Never Succeeded')).toBeInTheDocument();
  });
});
