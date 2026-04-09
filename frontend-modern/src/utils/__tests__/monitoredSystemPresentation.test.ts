import { describe, expect, it } from 'vitest';

import proLicensePanelStateSource from '@/components/Settings/useProLicensePanelState.ts?raw';
import monitoredSystemLedgerPanelSource from '@/components/Settings/MonitoredSystemLedgerPanel.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import monitoredSystemPresentationSource from '@/utils/monitoredSystemPresentation.ts?raw';
import {
  buildMonitoredSystemAdmissionPreviewUnavailableState,
  formatMonitoredSystemAdmissionPreviewUnavailableMessage,
  formatMonitoredSystemGroupedSourcesLabel,
  formatMonitoredSystemLegacyConnectionBreakdown,
  formatMonitoredSystemUsageUnavailableMessage,
  getMonitoredSystemBriefSummary,
  formatMonitoredSystemLimitSummary,
  formatMonitoredSystemLedgerUnavailableMessage,
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemMigrationMessage,
  formatMonitoredSystemOverflowSummary,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemAdmissionPreviewRequiredState,
  getMonitoredSystemAdmissionPreviewSaveBlockedMessage,
  getMonitoredSystemAdmissionPreviewUnavailableTitle,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemDisclosureDefinition,
  getMonitoredSystemDisclosureToggleLabel,
  getMonitoredSystemExplanationFallbackSummary,
  getMonitoredSystemLedgerDescription,
  getMonitoredSystemLedgerErrorState,
  getMonitoredSystemLedgerHiddenState,
  getMonitoredSystemLedgerLoadingState,
  getMonitoredSystemLedgerPresentation,
  getMonitoredSystemLedgerPolicyLoadingState,
  getMonitoredSystemLedgerUnavailableState,
  getMonitoredSystemLimitInstallCollectorsLabel,
  getMonitoredSystemLimitRemainingCapacity,
  getMonitoredSystemLimitLearnMoreLabel,
  getMonitoredSystemLimitUnavailableReason,
  getMonitoredSystemLimitUpgradeLabel,
  getMonitoredSystemLimitUsageSummary,
  getMonitoredSystemSourceLabel,
  getMonitoredSystemStatusFallbackSummary,
  getMonitoredSystemSurfaceTypeLabel,
  isMonitoredSystemAdmissionPreviewResolvedSafely,
  isMonitoredSystemLimitUrgent,
  isMonitoredSystemLimitUsageAvailable,
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
      countedSystemBadgeLabel: 'Counts as 1 monitored system',
      groupedSourcesHeading: 'Grouped sources',
      countingExplanationHeading: 'Why this counts',
      continuityHeading: 'Plan continuity',
      continuityPlanLimitLabel: 'Plan limit',
      continuityEffectiveLimitLabel: 'Effective limit',
      continuityGrandfatheredFloorLabel: 'Grandfathered floor',
      continuityCaptureLabel: 'Continuity capture',
      continuityCapturePendingLabel: 'Pending',
      continuityCaptureCapturedLabel: 'Captured',
      usageVerifyingLabel: 'Verifying…',
      remainingCapacityUnavailableLabel: 'Unavailable',
      unlimitedLimitLabel: 'Unlimited',
      loadingState: {
        text: 'Loading monitored system usage…',
      },
      errorState: {
        title: 'Monitored system usage is temporarily unavailable.',
        retryingLabel: 'Trying again…',
        retryLabel: 'Try again',
      },
      unavailableState: {
        title: 'Verifying monitored-system inventory',
        fallbackMessage:
          'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
        unsettledMessage:
          'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
        rebuildPendingMessage:
          'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
      },
      policyLoadingState: {
        title: 'Checking monitored-system visibility',
        message:
          'Pulse waits for the session presentation policy before loading usage or plan-limit data.',
      },
      hiddenState: {
        title: 'Monitored-system usage is hidden in demo mode',
        message:
          'The public demo uses sample infrastructure data, so Pulse hides counted-system totals, plan limits, and upgrade actions instead of creating a demo license.',
      },
      countingDetailsCollapsedLabel: 'View counting details',
      countingDetailsExpandedLabel: 'Hide counting details',
      currentStatusHeading: 'Current status',
      latestIncludedSignalSummaryLabel: 'Latest included signal',
      includedCollectionPathsHeading: 'Included collection paths',
      emptyState: 'No monitored systems counted.',
      noIncludedSignalLabel: 'No included signal yet.',
      fallbackExplanationSummary:
        'Pulse counts this top-level collection path as one monitored system.',
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
      admissionPreview: {
        requiredTitle: 'Preview monitored-system impact before saving',
        requiredMessage:
          'Pulse must verify monitored-system capacity for this platform connection before it can be saved.',
        unavailableTitle: 'Monitored-system capacity is temporarily unavailable',
        unavailableFallbackMessage:
          'Pulse cannot verify monitored-system capacity right now, so this connection cannot be saved yet. Retry preview in a moment.',
        unavailableUnsettledMessage:
          'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
        unavailableRebuildPendingMessage:
          'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
        saveBlockedLimitMessage: 'This change would exceed your monitored-system limit',
        saveBlockedLoadingMessage: 'Wait for the monitored-system impact preview to finish',
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
    expect(getMonitoredSystemLedgerUnavailableState()).toEqual({
      title: 'Verifying monitored-system inventory',
      fallbackMessage:
        'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
      unsettledMessage:
        'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
      rebuildPendingMessage:
        'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
    });
    expect(getMonitoredSystemLedgerPolicyLoadingState()).toEqual({
      title: 'Checking monitored-system visibility',
      message:
        'Pulse waits for the session presentation policy before loading usage or plan-limit data.',
    });
    expect(getMonitoredSystemLedgerHiddenState()).toEqual({
      title: 'Monitored-system usage is hidden in demo mode',
      message:
        'The public demo uses sample infrastructure data, so Pulse hides counted-system totals, plan limits, and upgrade actions instead of creating a demo license.',
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

  it('keeps monitored-system usage availability on the shared presentation helper', () => {
    expect(monitoredSystemPresentationSource).toContain(
      'export function isMonitoredSystemLimitUsageAvailable',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemLimitUsageSummary',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function getMonitoredSystemLimitRemainingCapacity',
    );

    for (const source of [
      licensePresentationSource,
      monitoredSystemLedgerPanelSource,
      monitoredSystemLimitWarningBannerModelSource,
      proLicensePanelStateSource,
    ]) {
      expect(source).not.toContain('current_available !== false');
    }
    expect(proLicensePanelStateSource).not.toContain("'Verifying…'");
    expect(proLicensePanelStateSource).not.toContain("'Unavailable'");
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

  it('centralizes monitored-system limit availability and capacity presentation', () => {
    const unavailableLimit = {
      current: 0,
      limit: 10,
      current_available: false,
      current_unavailable_reason: 'supplemental_inventory_unsettled',
      state: 'enforced',
    };

    expect(isMonitoredSystemLimitUsageAvailable(unavailableLimit)).toBe(false);
    expect(getMonitoredSystemLimitUnavailableReason(unavailableLimit)).toBe(
      'supplemental_inventory_unsettled',
    );
    expect(getMonitoredSystemLimitUsageSummary(unavailableLimit)).toBe('Verifying…');
    expect(getMonitoredSystemLimitRemainingCapacity(unavailableLimit)).toBe('Unavailable');
    expect(isMonitoredSystemLimitUrgent(unavailableLimit)).toBe(false);
    expect(
      formatMonitoredSystemUsageUnavailableMessage(unavailableLimit.current_unavailable_reason),
    ).toBe(
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    );

    expect(
      getMonitoredSystemLimitUsageSummary({
        current: 7,
        limit: 10,
        current_available: true,
      }),
    ).toBe('7 / 10');
    expect(
      getMonitoredSystemLimitRemainingCapacity({
        current: 7,
        limit: 10,
        current_available: true,
      }),
    ).toBe(3);
    expect(
      getMonitoredSystemLimitRemainingCapacity({
        current: 7,
        limit: 0,
        current_available: true,
      }),
    ).toBe('Unlimited');
    expect(
      isMonitoredSystemLimitUrgent({
        current: 9,
        limit: 10,
        current_available: true,
        state: 'warning',
      }),
    ).toBe(true);
  });

  it('returns canonical monitored-system admission unavailable copy', () => {
    expect(getMonitoredSystemAdmissionPreviewRequiredState()).toEqual({
      title: 'Preview monitored-system impact before saving',
      message:
        'Pulse must verify monitored-system capacity for this platform connection before it can be saved.',
    });
    expect(getMonitoredSystemAdmissionPreviewUnavailableTitle()).toBe(
      'Monitored-system capacity is temporarily unavailable',
    );
    expect(
      formatMonitoredSystemAdmissionPreviewUnavailableMessage('supplemental_inventory_unsettled'),
    ).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    );
    expect(
      formatMonitoredSystemAdmissionPreviewUnavailableMessage(
        'supplemental_inventory_rebuild_pending',
      ),
    ).toBe(
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
    );
    expect(
      formatMonitoredSystemAdmissionPreviewUnavailableMessage('monitor_state_unavailable'),
    ).toBe(
      'Pulse cannot verify monitored-system capacity right now, so this connection cannot be saved yet. Retry preview in a moment.',
    );
    expect(
      buildMonitoredSystemAdmissionPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: ' supplemental_inventory_unsettled ',
      }),
    ).toEqual({
      reason: 'supplemental_inventory_unsettled',
      title: 'Monitored-system capacity is temporarily unavailable',
      message:
        'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
    });
    expect(
      buildMonitoredSystemAdmissionPreviewUnavailableState({
        code: 'provider_failed',
        reason: 'supplemental_inventory_unsettled',
      }),
    ).toBeNull();
    expect(
      isMonitoredSystemAdmissionPreviewResolvedSafely({
        preview: { would_exceed_limit: false },
      }),
    ).toBe(true);
    expect(
      isMonitoredSystemAdmissionPreviewResolvedSafely({
        preview: { would_exceed_limit: true },
      }),
    ).toBe(false);
    expect(getMonitoredSystemAdmissionPreviewSaveBlockedMessage({ preview: null })).toBe(
      'Pulse must verify monitored-system capacity for this platform connection before it can be saved.',
    );
    expect(
      getMonitoredSystemAdmissionPreviewSaveBlockedMessage({
        preview: { would_exceed_limit: true },
      }),
    ).toBe('This change would exceed your monitored-system limit');
    expect(
      getMonitoredSystemAdmissionPreviewSaveBlockedMessage({
        preview: { would_exceed_limit: false },
      }),
    ).toBeNull();
  });

  it('returns canonical monitored-system ledger unavailable copy', () => {
    expect(formatMonitoredSystemLedgerUnavailableMessage('supplemental_inventory_unsettled')).toBe(
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    );
    expect(
      formatMonitoredSystemLedgerUnavailableMessage('supplemental_inventory_rebuild_pending'),
    ).toBe(
      'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
    );
    expect(formatMonitoredSystemLedgerUnavailableMessage('monitor_state_unavailable')).toBe(
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    );
  });

  it('returns customer-facing source and type labels', () => {
    expect(getMonitoredSystemSourceLabel('agent')).toBe('Agent');
    expect(getMonitoredSystemSourceLabel('multiple')).toBe('Multiple Sources');
    expect(getMonitoredSystemSourceLabel('pbs')).toBe('PBS');
    expect(getMonitoredSystemSourceLabel('vmware')).toBe('VMware');
    expect(getMonitoredSystemSourceLabel('')).toBe('');
    expect(getMonitoredSystemSurfaceTypeLabel('agent')).toBe('Host');
    expect(getMonitoredSystemSurfaceTypeLabel('docker-host')).toBe('Docker Host');
    expect(getMonitoredSystemSurfaceTypeLabel('proxmox-node')).toBe('Proxmox Node');
    expect(getMonitoredSystemSurfaceTypeLabel(undefined)).toBe('System');
    expect(getMonitoredSystemSurfaceTypeLabel('custom_cluster')).toBe('Custom Cluster');
  });

  it('formats included signal attribution and summary sentences', () => {
    expect(formatMonitoredSystemGroupedSourcesLabel(1)).toBe('1 grouped source');
    expect(formatMonitoredSystemGroupedSourcesLabel(2)).toBe('2 grouped sources');
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
      formatMonitoredSystemSurfaceAttribution({
        name: 'esxi-01',
        type: 'host',
        source: 'vmware',
      }),
    ).toBe('esxi-01 (Host via VMware)');
    expect(
      formatMonitoredSystemSurfaceAttribution({
        name: 'tower',
        type: 'truenas-system',
        source: 'multiple',
      }),
    ).toBe('tower (TrueNAS System via Multiple Sources)');
    expect(
      formatMonitoredSystemLatestIncludedSignalSentence({
        attribution: 'tower (PBS Server via PBS)',
        relative: '2m ago',
      }),
    ).toBe('Latest included signal: tower (PBS Server via PBS), reported 2m ago.');
  });
});
