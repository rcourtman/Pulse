import { render, screen } from '@solidjs/testing-library';
import { describe, expect, it } from 'vitest';

import { ResourceCorrelationSummary } from '../ResourceCorrelationSummary';

describe('ResourceCorrelationSummary', () => {
  it('renders learned correlations with canonical labels and totals', () => {
    render(() => (
      <ResourceCorrelationSummary
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
          {
            source_id: 'storage-2',
            source_name: 'Storage 2',
            source_type: 'storage',
            target_id: 'host-2',
            target_name: 'Host 2',
            target_type: 'vm',
            event_pattern: 'ALERT → ALERT',
            occurrences: 1,
            avg_delay: 125000000000,
            confidence: 0.5,
            last_seen: '2026-03-01T00:10:00Z',
            description: 'Alerts often cluster together',
          },
        ]}
        summaryText="5 total"
      />
    ));

    expect(screen.getByText('Learned correlations')).toBeInTheDocument();
    expect(screen.getByText('5 total')).toBeInTheDocument();
    // Cross-jump links to the legacy /infrastructure surface were retired;
    // labels render as plain text by default in the platform-first layout.
    expect(screen.queryAllByRole('link')).toHaveLength(0);
    expect(screen.getByText('Storage 1')).toBeInTheDocument();
    expect(screen.getByText('Host 1')).toBeInTheDocument();
    expect(screen.getByText('Disk Full → Restart')).toBeInTheDocument();
    const alertPattern = screen.getByText('Alert → Alert');
    expect(alertPattern).toBeInTheDocument();
    expect(alertPattern.className).not.toContain('uppercase');
    expect(screen.getByText(/2 occurrences · avg delay 2m · 88% confidence/)).toBeInTheDocument();
    expect(screen.getByText('Disk pressure often precedes restarts')).toBeInTheDocument();
    expect(screen.queryByText(/last seen/i)).toBeNull();
  });

  it('renders correlation context with dependency and dependent links', () => {
    render(() => (
      <ResourceCorrelationSummary
        title="Correlation context"
        relationships={[
          {
            sourceId: 'node:pve-1',
            targetId: 'vm-child',
            type: 'runs_on',
            confidence: 1,
            active: true,
            discoverer: 'proxmox_adapter',
            observedAt: '2026-03-18T12:00:00Z',
            lastSeenAt: '2026-03-18T12:05:00Z',
          },
        ]}
        dependencies={['storage-1']}
        dependents={['vm-child']}
        resolveResourceLabel={(resourceId) =>
          resourceId === 'node:pve-1'
            ? 'PVE 1'
            : resourceId === 'storage-1'
              ? 'Storage 1 alias'
              : resourceId === 'vm-child'
                ? 'VM Child'
                : resourceId
        }
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

    expect(screen.getByText('Correlation context')).toBeInTheDocument();
    expect(
      screen.getByText('1 canonical relationship · 1 dependency · 1 dependent · 1 correlation'),
    ).toBeInTheDocument();
    expect(screen.getByText('Canonical relationships')).toBeInTheDocument();
    expect(screen.getByText('Depends on')).toBeInTheDocument();
    expect(screen.getByText('Used by')).toBeInTheDocument();
    expect(screen.getByText('Correlations')).toBeInTheDocument();
    // Cross-jump links to /infrastructure were retired; labels render as
    // plain text by default in the platform-first layout.
    expect(screen.queryAllByRole('link')).toHaveLength(0);
    expect(screen.getByText('PVE 1')).toBeInTheDocument();
    expect(screen.getAllByText('VM Child').length).toBeGreaterThan(0);
    expect(screen.getByText('Storage 1 alias')).toBeInTheDocument();
    const relationshipType = screen.getByText('Runs On');
    expect(relationshipType).toBeInTheDocument();
    expect(relationshipType.className).not.toContain('uppercase');
    expect(screen.getByText(/100% confidence · Proxmox Adapter · last seen/)).toBeInTheDocument();
    expect(screen.getAllByText(/last seen/i).length).toBeGreaterThanOrEqual(2);
  });
});
