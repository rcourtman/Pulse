import { render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RecoverySummary } from './RecoverySummary';
import recoverySummarySource from './RecoverySummary.tsx?raw';

describe('RecoverySummary', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders the recovery overview inside the shared summary panel style', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-03-09T12:00:00Z'));

    render(() => (
      <RecoverySummary
        rollups={() => [
          {
            rollupId: 'alpha',
            lastOutcome: 'success',
            lastSuccessAt: '2026-03-09T11:30:00Z',
            platforms: ['proxmox-pbs'],
            display: { subjectType: 'proxmox-vm' },
          },
          {
            rollupId: 'beta',
            lastOutcome: 'failed',
            lastAttemptAt: '2026-03-08T11:00:00Z',
            lastSuccessAt: null,
            platforms: ['proxmox-pve'],
            subjectRef: { type: 'truenas-dataset' },
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

    expect(screen.getByTestId('recovery-summary')).toBeInTheDocument();
    expect(screen.getByText('Recovery Posture')).toBeInTheDocument();
    expect(screen.getByText('Protected Footprint')).toBeInTheDocument();
    expect(screen.getByText('Freshness')).toBeInTheDocument();
    expect(screen.getByText('Recent History')).toBeInTheDocument();
    expect(screen.getAllByText(/attention/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/recovery points/i)).toBeInTheDocument();
    expect(screen.getByText(/item types/i)).toBeInTheDocument();
    expect(screen.getByText('Primary Item')).toBeInTheDocument();
    expect(screen.getByText('Primary Platform')).toBeInTheDocument();
    expect(screen.getByText('Avg / Day')).toBeInTheDocument();
    expect(screen.getByText('Healthy')).toBeInTheDocument();
    expect(screen.getByText('Failed')).toBeInTheDocument();
    expect(screen.getByText('<24h')).toBeInTheDocument();
    expect(screen.getByText('>7d')).toBeInTheDocument();
    expect(screen.getByText('fresh in 24h')).toBeInTheDocument();
    expect(screen.getByText('stale items')).toBeInTheDocument();
    expect(screen.getByText('Latest Activity')).toBeInTheDocument();
    expect(screen.getByText('2 protected')).toBeInTheDocument();
    expect(screen.getAllByText(/Never Succeeded/i).length).toBeGreaterThan(0);
    expect(screen.getByText('need attention')).toBeInTheDocument();
    expect(screen.queryByText('protected items')).not.toBeInTheDocument();
    expect(screen.queryByText('Running')).not.toBeInTheDocument();
    expect(screen.queryByText('<7d')).not.toBeInTheDocument();
    expect(screen.queryByText('Peak Throughput')).not.toBeInTheDocument();
  });

  it('uses the shared default summary density rather than a recovery-only compact override', () => {
    expect(recoverySummarySource).not.toContain('density="compact"');
    expect(recoverySummarySource).not.toContain('!p-1.5 sm:!p-2');
  });
});
