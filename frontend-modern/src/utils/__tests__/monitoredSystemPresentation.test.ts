import { describe, expect, it } from 'vitest';

import {
  formatMonitoredSystemLegacyConnectionBreakdown,
  getMonitoredSystemBriefSummary,
  formatMonitoredSystemLimitSummary,
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemMigrationMessage,
  formatMonitoredSystemOverflowSummary,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemDisclosureDefinition,
  getMonitoredSystemDisclosureToggleLabel,
  getMonitoredSystemExplanationFallbackSummary,
  getMonitoredSystemLedgerDescription,
  getMonitoredSystemLedgerErrorState,
  getMonitoredSystemLedgerLoadingState,
  getMonitoredSystemLedgerPresentation,
  getMonitoredSystemLimitInstallCollectorsLabel,
  getMonitoredSystemLimitLearnMoreLabel,
  getMonitoredSystemLimitUpgradeLabel,
  getMonitoredSystemSourceLabel,
  getMonitoredSystemStatusFallbackSummary,
  getMonitoredSystemSurfaceTypeLabel,
} from '@/utils/monitoredSystemPresentation';

describe('monitoredSystemPresentation', () => {
  it('returns canonical ledger labels and fallback copy', () => {
    expect(getMonitoredSystemLedgerPresentation()).toEqual({
      briefSummary: 'Billing is based on monitored systems. Child resources are included.',
      sectionTitle: 'Monitored Systems',
      panelTitle: 'Monitored System Ledger',
      disclosureButtonLabel: 'View counting rules',
      disclosureHideLabel: 'Hide counting rules',
      disclosureDefinition:
        'A monitored system is a top-level machine or cluster Pulse actively monitors. Each system counts once no matter how Pulse collects it. Child resources like VMs, containers, pods, disks, backups, and services are included.',
      ledgerDescription:
        'Review the monitored systems currently counted against your Pulse Pro plan limit.',
      tableNameLabel: 'Name',
      tableStatusLabel: 'Status',
      tableLatestIncludedSignalLabel: 'Latest Included Signal',
      loadingState: {
        text: 'Loading monitored system usage…',
      },
      errorState: {
        title: 'Monitored system usage is temporarily unavailable.',
        retryingLabel: 'Trying again…',
        retryLabel: 'Try again',
      },
      countingDetailsCollapsedLabel: 'View counting details',
      countingDetailsExpandedLabel: 'Hide counting details',
      currentStatusHeading: 'Current status',
      latestIncludedSignalSummaryLabel: 'Latest included signal',
      includedCollectionPathsHeading: 'Included collection paths',
      emptyState: 'No monitored systems counted.',
      noIncludedSignalLabel: 'No included signal yet.',
      fallbackExplanationSummary: 'Pulse counts this top-level collection path as one monitored system.',
      statusSummaryByStatus: {
        online: 'All included top-level collection paths currently report online status.',
        warning:
          'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
        offline:
          'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
        unknown: 'Pulse cannot determine a canonical runtime status for this monitored system yet.',
      },
      limitBanner: {
        learnMoreLabel: 'Learn more',
        installCollectorsLabel: 'Install v6 collectors',
        upgradeLabel: 'Upgrade to add more',
        overflowSummaryPrefix: 'Includes 1 temporary onboarding slot',
        legacyConnectionSuffix:
          'that count once toward your monitored-system cap when the same top-level system is discovered canonically.',
      },
    });
    expect(getMonitoredSystemBriefSummary()).toBe(
      'Billing is based on monitored systems. Child resources are included.',
    );
    expect(getMonitoredSystemDisclosureToggleLabel(false)).toBe('View counting rules');
    expect(getMonitoredSystemDisclosureToggleLabel(true)).toBe('Hide counting rules');
    expect(getMonitoredSystemDisclosureDefinition()).toContain('top-level machine or cluster');
    expect(getMonitoredSystemLedgerDescription()).toBe(
      'Review the monitored systems currently counted against your Pulse Pro plan limit.',
    );
    expect(getMonitoredSystemLedgerLoadingState()).toEqual({
      text: 'Loading monitored system usage…',
    });
    expect(getMonitoredSystemLedgerErrorState()).toEqual({
      title: 'Monitored system usage is temporarily unavailable.',
      retryingLabel: 'Trying again…',
      retryLabel: 'Try again',
    });
    expect(getMonitoredSystemCountingDetailsToggleLabel(false)).toBe('View counting details');
    expect(getMonitoredSystemCountingDetailsToggleLabel(true)).toBe('Hide counting details');
    expect(getMonitoredSystemExplanationFallbackSummary()).toBe(
      'Pulse counts this top-level collection path as one monitored system.',
    );
    expect(getMonitoredSystemStatusFallbackSummary('online')).toBe(
      'All included top-level collection paths currently report online status.',
    );
    expect(getMonitoredSystemStatusFallbackSummary('warning')).toBe(
      'At least one included top-level collection path is degraded, so Pulse marks this monitored system as warning.',
    );
    expect(getMonitoredSystemStatusFallbackSummary('offline')).toBe(
      'At least one included source is offline or disconnected, so Pulse marks this monitored system as offline.',
    );
    expect(getMonitoredSystemStatusFallbackSummary()).toBe(
      'Pulse cannot determine a canonical runtime status for this monitored system yet.',
    );
  });

  it('returns canonical monitored-system limit warning copy', () => {
    expect(getMonitoredSystemLimitLearnMoreLabel()).toBe('Learn more');
    expect(getMonitoredSystemLimitInstallCollectorsLabel()).toBe('Install v6 collectors');
    expect(getMonitoredSystemLimitUpgradeLabel()).toBe('Upgrade to add more');
    expect(formatMonitoredSystemLimitSummary({ current: 5, limit: 6 })).toBe(
      'Monitored systems: 5/6',
    );
    expect(
      formatMonitoredSystemLegacyConnectionBreakdown({
        proxmox_nodes: 2,
        docker_hosts: 1,
        kubernetes_clusters: 0,
      }),
    ).toBe('2 Proxmox nodes, 1 Docker host');
    expect(
      formatMonitoredSystemMigrationMessage({
        proxmox_nodes: 2,
        docker_hosts: 1,
        kubernetes_clusters: 0,
      }),
    ).toBe(
      'You also have 3 resources connected via API or legacy collectors (2 Proxmox nodes, 1 Docker host) that count once toward your monitored-system cap when the same top-level system is discovered canonically.',
    );
    expect(formatMonitoredSystemOverflowSummary(14)).toBe(
      'Includes 1 temporary onboarding slot (14d remaining)',
    );
    expect(formatMonitoredSystemOverflowSummary(undefined)).toBe('');
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
