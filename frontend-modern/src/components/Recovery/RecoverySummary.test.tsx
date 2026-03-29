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
    expect(screen.getByText('Posture')).toBeInTheDocument();
    expect(screen.getByText('Coverage')).toBeInTheDocument();
    expect(screen.getByText('Freshness')).toBeInTheDocument();
    expect(screen.getByText('Activity')).toBeInTheDocument();
    expect(screen.getAllByText(/attention/i).length).toBeGreaterThan(0);
    expect(screen.getByText(/recovery points/i)).toBeInTheDocument();
    expect(screen.getByText(/item types/i)).toBeInTheDocument();
    expect(screen.getByText('2 platforms')).toBeInTheDocument();
    expect(screen.getByText('Healthy 1 · Failed 1')).toBeInTheDocument();
    expect(screen.getByText('>7d 1 · Never Succeeded 1')).toBeInTheDocument();
    expect(screen.getByText('fresh in 24h')).toBeInTheDocument();
    expect(screen.getByText('Mar 9')).toBeInTheDocument();
    expect(screen.getByText('2 protected items')).toBeInTheDocument();
    expect(screen.getByText('need attention')).toBeInTheDocument();
    expect(screen.queryByText('protected items')).not.toBeInTheDocument();
    expect(screen.queryByText('Running')).not.toBeInTheDocument();
    expect(screen.queryByText('<7d')).not.toBeInTheDocument();
    expect(screen.queryByText('<1h')).not.toBeInTheDocument();
    expect(screen.queryByText('<24h')).not.toBeInTheDocument();
    expect(screen.queryByText('Days Active')).not.toBeInTheDocument();
    expect(screen.queryByText('Multi-platform')).not.toBeInTheDocument();
    expect(screen.queryByText('Primary Platform')).not.toBeInTheDocument();
    expect(screen.queryByText('Primary Item')).not.toBeInTheDocument();
    expect(screen.queryByText('Avg / Day')).not.toBeInTheDocument();
    expect(screen.queryByText('Peak Throughput')).not.toBeInTheDocument();
    expect(screen.queryByText('Peak Day')).not.toBeInTheDocument();
    expect(screen.queryByText('stale items')).not.toBeInTheDocument();
    expect(screen.queryByText('Platforms 2')).not.toBeInTheDocument();
    expect(screen.queryByText('Latest Activity Mar 9')).not.toBeInTheDocument();
    expect(screen.queryByText(/^Platforms$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Healthy$/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^Failed$/)).not.toBeInTheDocument();
  });

  it('uses the shared default summary density rather than a recovery-only compact override', () => {
    expect(recoverySummarySource).not.toContain('density="compact"');
    expect(recoverySummarySource).not.toContain('!p-1.5 sm:!p-2');
  });
});
