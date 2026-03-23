import { describe, expect, it } from 'vitest';

import {
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemExplanationFallbackSummary,
  getMonitoredSystemLedgerPresentation,
  getMonitoredSystemSourceLabel,
  getMonitoredSystemStatusFallbackSummary,
  getMonitoredSystemSurfaceTypeLabel,
} from '@/utils/monitoredSystemPresentation';

describe('monitoredSystemPresentation', () => {
  it('returns canonical ledger labels and fallback copy', () => {
    expect(getMonitoredSystemLedgerPresentation()).toEqual({
      sectionTitle: 'Monitored Systems',
      panelTitle: 'Monitored System Ledger',
      tableNameLabel: 'Name',
      tableStatusLabel: 'Status',
      tableLatestIncludedSignalLabel: 'Latest Included Signal',
      countingDetailsCollapsedLabel: 'View counting details',
      countingDetailsExpandedLabel: 'Hide counting details',
      currentStatusHeading: 'Current status',
      latestIncludedSignalSummaryLabel: 'Latest included signal',
      includedCollectionPathsHeading: 'Included collection paths',
      emptyState: 'No monitored systems counted.',
      noIncludedSignalLabel: 'No included signal yet.',
      fallbackExplanationSummary: 'Pulse counts this top-level collection path as one monitored system.',
      fallbackStatusSummary:
        'Pulse cannot determine a canonical runtime status for this monitored system yet.',
    });
    expect(getMonitoredSystemCountingDetailsToggleLabel(false)).toBe('View counting details');
    expect(getMonitoredSystemCountingDetailsToggleLabel(true)).toBe('Hide counting details');
    expect(getMonitoredSystemExplanationFallbackSummary()).toBe(
      'Pulse counts this top-level collection path as one monitored system.',
    );
    expect(getMonitoredSystemStatusFallbackSummary()).toBe(
      'Pulse cannot determine a canonical runtime status for this monitored system yet.',
    );
  });

  it('returns customer-facing source and type labels', () => {
    expect(getMonitoredSystemSourceLabel('agent')).toBe('Agent');
    expect(getMonitoredSystemSourceLabel('pbs')).toBe('PBS');
    expect(getMonitoredSystemSourceLabel('')).toBe('');
    expect(getMonitoredSystemSurfaceTypeLabel('docker-host')).toBe('Docker Host');
    expect(getMonitoredSystemSurfaceTypeLabel('proxmox-node')).toBe('Proxmox Node');
    expect(getMonitoredSystemSurfaceTypeLabel(undefined)).toBe('System');
    expect(getMonitoredSystemSurfaceTypeLabel('custom_cluster')).toBe('Custom Cluster');
  });

  it('formats included signal attribution and summary sentences', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution({
        name: 'tower',
        type: 'pbs-server',
        source: 'pbs',
      }),
    ).toBe('tower (PBS Server via PBS)');
    expect(
      formatMonitoredSystemSurfaceAttribution({
        name: 'tower',
        type: 'host',
        source: 'host',
      }),
    ).toBe('tower (Host)');
    expect(
      formatMonitoredSystemLatestIncludedSignalSentence({
        attribution: 'tower (PBS Server via PBS)',
        relative: '2m ago',
      }),
    ).toBe('Latest included signal: tower (PBS Server via PBS), reported 2m ago.');
  });
});
