import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { ResourceGraphSummary } from '../ResourceGraphSummary';

describe('ResourceGraphSummary', () => {
  it('renders learned correlations with canonical labels and totals', () => {
    render(() => (
      <ResourceGraphSummary
        title="Learned correlations"
        correlations={[
          {
            source_id: 'storage-1',
            source_name: 'Storage 1',
            source_type: 'storage',
            target_id: 'host-1',
            target_name: 'Host 1',
            target_type: 'vm',
            event_pattern: 'disk_full -> restart',
            occurrences: 2,
            avg_delay: 125000000000,
            confidence: 0.875,
            last_seen: '2026-03-01T00:15:00Z',
            description: 'Disk pressure often precedes restarts',
          },
        ]}
        summaryText="5 total"
      />
    ));

    expect(screen.getByText('Learned correlations')).toBeInTheDocument();
    expect(screen.getByText('5 total')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open source resource Storage 1 in Infrastructure' }))
      .toHaveAttribute('href', '/infrastructure?resource=storage-1');
    expect(screen.getByRole('link', { name: 'Open target resource Host 1 in Infrastructure' }))
      .toHaveAttribute('href', '/infrastructure?resource=host-1');
    expect(screen.getByText('Disk Full → Restart')).toBeInTheDocument();
    expect(screen.getByText(/2 occurrences · avg delay 2m · 88% confidence/)).toBeInTheDocument();
    expect(screen.getByText('Disk pressure often precedes restarts')).toBeInTheDocument();
    expect(screen.queryByText(/last seen/i)).toBeNull();
  });

  it('renders graph context with dependency and dependent links', () => {
    render(() => (
      <ResourceGraphSummary
        title="Graph context"
        dependencies={['storage-1']}
        dependents={['vm-child']}
        correlations={[
          {
            source_id: 'storage-1',
            source_name: 'Storage 1',
            source_type: 'storage',
            target_id: 'host-1',
            target_name: 'Host 1',
            target_type: 'vm',
            event_pattern: 'disk_full -> restart',
            occurrences: 2,
            avg_delay: 125000000000,
            confidence: 0.875,
            last_seen: '2026-03-01T00:15:00Z',
            description: 'Disk pressure often precedes restarts',
          },
        ]}
        showLastSeen
      />
    ));

    expect(screen.getByText('Graph context')).toBeInTheDocument();
    expect(screen.getByText('1 dependency · 1 dependent · 1 correlation')).toBeInTheDocument();
    expect(screen.getByText('Depends on')).toBeInTheDocument();
    expect(screen.getByText('Used by')).toBeInTheDocument();
    expect(screen.getByText('Correlations')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Open dependency resource storage-1 in Infrastructure' }))
      .toHaveAttribute('href', '/infrastructure?resource=storage-1');
    expect(screen.getByRole('link', { name: 'Open dependent resource vm-child in Infrastructure' }))
      .toHaveAttribute('href', '/infrastructure?resource=vm-child');
    expect(screen.getByText(/last seen/i)).toBeInTheDocument();
  });
});
