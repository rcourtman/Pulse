import { describe, expect, it } from 'vitest';

import proLicensePanelStateSource from '@/components/Settings/useProLicensePanelState.ts?raw';
import monitoredSystemLedgerPanelSource from '@/components/Settings/MonitoredSystemLedgerPanel.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import licensePresentationSource from '@/utils/licensePresentation.ts?raw';
import monitoredSystemPresentationSource from '@/utils/monitoredSystemPresentation.ts?raw';
import {
  buildMonitoredSystemCapacitySectionModel,
  buildMonitoredSystemAdmissionPreviewUnavailableState,
  formatMonitoredSystemAdmissionPreviewSummary,
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
  getMonitoredSystemAdmissionPreviewTitle,
  getMonitoredSystemAdmissionPreviewUnavailableTitle,
  getMonitoredSystemCountingDetailsToggleLabel,
  getMonitoredSystemDisclosureDefinition,
  getMonitoredSystemDisclosureToggleLabel,
  getMonitoredSystemExplanationFallbackSummary,
  getMonitoredSystemLimitCapacityStatusSummary,
  getMonitoredSystemLimitContextSummary,
  getMonitoredSystemLedgerDescription,
  getMonitoredSystemLedgerErrorState,
  getMonitoredSystemLedgerHiddenState,
  getMonitoredSystemLedgerLoadingState,
  getMonitoredSystemLedgerPresentation,
  getMonitoredSystemLedgerPolicyLoadingState,
  getMonitoredSystemLedgerUnavailableState,
  getMonitoredSystemLimitInstallCollectorsLabel,
  getMonitoredSystemLimitReviewPolicyLabel,
  getMonitoredSystemLimitUnavailableReason,
  getMonitoredSystemLimitUsageSummary,
  getMonitoredSystemSourceLabel,
  resolveMonitoredSystemCapacityStatus,
  getMonitoredSystemStatusFallbackSummary,
  getMonitoredSystemSurfaceTypeLabel,
  isMonitoredSystemAdmissionPreviewResolvedSafely,
  isMonitoredSystemLimitUrgent,
  isMonitoredSystemLimitUsageAvailable,
} from '@/utils/monitoredSystemPresentation';

describe('monitoredSystemPresentation', () => {
  it('returns canonical ledger labels and fallback copy', () => {
    expect(getMonitoredSystemLedgerPresentation()).toEqual({
      briefSummary:
        'Pulse counts top-level monitored systems. Child resources underneath them are included.',
      sectionTitle: 'Monitored Systems',
      panelTitle: 'Monitored System Ledger',
      disclosureButtonLabel: 'View counting rules',
      disclosureHideLabel: 'Hide counting rules',
      disclosureDefinition:
        'A monitored system is a top-level monitored root such as a Docker host, Kubernetes cluster, Proxmox node, standalone host, or TrueNAS system. Each root counts once no matter how Pulse collects it. Child resources like VMs, containers, pods, disks, backups, and services underneath that root are included.',
      ledgerDescription:
        'Review the top-level monitored systems Pulse has identified for reporting and any applicable policy.',
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
      unlimitedLimitLabel: 'Not metered',
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
          'Pulse waits for the session presentation policy before loading monitored-system usage details.',
      },
      hiddenState: {
        title: 'Monitored-system usage is hidden in demo mode',
        message:
          'The public demo uses sample infrastructure data, so Pulse hides counted-system totals and billing actions instead of creating a demo license.',
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
        reviewPolicyLabel: 'Review policy',
        installCollectorsLabel: 'Install v6 collectors',
        overflowSummaryPrefix: 'A temporary setup slot is active',
        legacyConnectionSuffix:
          'that are folded into the canonical monitored-system ledger when the same top-level system is discovered canonically.',
      },
      admissionPreview: {
        requiredTitle: 'Preview monitored-system impact before saving',
        requiredMessage:
          'Pulse must verify the monitored-system policy for this platform connection before it can be saved.',
        fallbackTitle: 'Monitored-system impact',
        exceedsPolicyTitle: 'This change exceeds the active monitored-system policy',
        addsSystemsTitle: 'This change adds monitored systems',
        removesSystemsTitle: 'This change removes monitored systems',
        unchangedTitle: 'This change keeps monitored-system count unchanged',
        unavailableTitle: 'Monitored-system verification is temporarily unavailable',
        unavailableFallbackMessage:
          'Pulse cannot verify monitored-system policy right now, so this connection cannot be saved yet. Retry preview in a moment.',
        unavailableUnsettledMessage:
          'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
        unavailableRebuildPendingMessage:
          'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
        saveBlockedLimitMessage: 'This change would exceed the active monitored-system policy',
        saveBlockedLoadingMessage: 'Wait for the monitored-system impact preview to finish',
      },
    });
    expect(getMonitoredSystemBriefSummary()).toBe(
      'Pulse counts top-level monitored systems. Child resources underneath them are included.',
    );
    expect(getMonitoredSystemDisclosureToggleLabel(false)).toBe('View counting rules');
    expect(getMonitoredSystemDisclosureToggleLabel(true)).toBe('Hide counting rules');
    expect(getMonitoredSystemDisclosureDefinition()).toContain('Docker host');
    expect(getMonitoredSystemDisclosureDefinition()).toContain('Proxmox node');
    expect(getMonitoredSystemLedgerDescription()).toBe(
      'Review the top-level monitored systems Pulse has identified for reporting and any applicable policy.',
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
        'Pulse waits for the session presentation policy before loading monitored-system usage details.',
    });
    expect(getMonitoredSystemLedgerHiddenState()).toEqual({
      title: 'Monitored-system usage is hidden in demo mode',
      message:
        'The public demo uses sample infrastructure data, so Pulse hides counted-system totals and billing actions instead of creating a demo license.',
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
      'export function getMonitoredSystemLimitCapacityStatusSummary',
    );
    expect(monitoredSystemPresentationSource).toContain(
      'export function resolveMonitoredSystemCapacityStatus',
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
    expect(getMonitoredSystemLimitReviewPolicyLabel()).toBe('Review policy');
    expect(getMonitoredSystemLimitInstallCollectorsLabel()).toBe('Install v6 collectors');
    expect(formatMonitoredSystemLimitSummary({ current: 5, limit: 6 })).toBe(
      '1 remaining. 5 monitored, 6 included.',
    );
    expect(formatMonitoredSystemLimitSummary({ current: 16, limit: 5, state: 'enforced' })).toBe(
      'Over policy by 11. 16 monitored, 5 included.',
    );
    expect(
      formatMonitoredSystemLimitSummary(
        { current: 16, limit: 5, state: 'enforced' },
        {
          mode: 'over_limit_frozen',
          urgency: 'enforced',
          current: 16,
          limit: 5,
          current_available: true,
          available_slots: 0,
          overage: 11,
          reason: 'legacy_migration_capture_pending',
          blocks_new_systems: true,
          existing_monitoring_continues: true,
        },
      ),
    ).toBe('Continuity verification pending. 16 monitored, 5 included.');
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
      'You also have 3 resources connected via API or legacy collectors (2 Proxmox nodes, 1 Docker host) that are folded into the canonical monitored-system ledger when the same top-level system is discovered canonically.',
    );
    expect(formatMonitoredSystemOverflowSummary(14)).toBe(
      'A temporary setup slot is active (14d remaining)',
    );
    expect(formatMonitoredSystemOverflowSummary(undefined)).toBe('');
  });

  it('builds a monitored-system capacity section model for the plan surface', () => {
    expect(
      buildMonitoredSystemCapacitySectionModel({
        current: 16,
        limit: 5,
        current_available: true,
        state: 'enforced',
      }),
    ).toEqual({
      stats: [
        { label: 'Monitored', value: '16 monitored systems' },
        { label: 'Included', value: '5' },
        { label: 'Status', value: 'Over policy by 11' },
      ],
      statusMessage: 'Existing monitoring continues. Additional monitored systems are paused.',
      detailMessage:
        'Reduce usage or resolve the applicable policy before adding another monitored system.',
      explanation: {
        label: 'Why is this over policy?',
        body: 'This installation was already monitoring 16 monitored systems before Pulse paused net-new monitored-system admissions at the active finite policy boundary. Pulse keeps those existing systems visible, but additional monitored systems stay paused until usage is reduced or the policy changes.',
      },
    });
  });

  it('does not build a monitored-system capacity section for uncapped plans', () => {
    expect(
      buildMonitoredSystemCapacitySectionModel(undefined, {
        mode: 'unlimited',
        urgency: 'ok',
        current: 12,
        limit: 0,
        current_available: true,
        available_slots: 0,
        overage: 0,
        blocks_new_systems: false,
        existing_monitoring_continues: true,
      }),
    ).toBeNull();
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
    expect(getMonitoredSystemLimitCapacityStatusSummary(unavailableLimit)).toBe('Unavailable');
    expect(getMonitoredSystemLimitContextSummary(unavailableLimit)).toBe(
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    );
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
    ).toBe('7 monitored systems');
    expect(
      getMonitoredSystemLimitCapacityStatusSummary({
        current: 7,
        limit: 10,
        current_available: true,
      }),
    ).toBe('3 remaining');
    expect(
      getMonitoredSystemLimitCapacityStatusSummary({
        current: 7,
        limit: 0,
        current_available: true,
      }),
    ).toBe('Not metered');
    expect(
      getMonitoredSystemLimitContextSummary({
        current: 10,
        limit: 10,
        current_available: true,
        state: 'enforced',
      }),
    ).toBe(
      'This finite policy includes 10. Existing monitoring continues; additional monitored systems stay paused until capacity is available or the policy changes.',
    );
    expect(
      getMonitoredSystemLimitContextSummary({
        current: 16,
        limit: 5,
        current_available: true,
        state: 'enforced',
      }),
    ).toBe(
      'This finite policy includes 5. This installation is already over policy by 11 because it was monitoring above that boundary before additional admissions paused. Existing monitoring continues; additional monitored systems stay paused until usage is reduced or the policy changes.',
    );
    expect(
      getMonitoredSystemLimitContextSummary(
        {
          current: 16,
          limit: 5,
          current_available: true,
          state: 'enforced',
        },
        {
          mode: 'over_limit_frozen',
          urgency: 'enforced',
          current: 16,
          limit: 5,
          current_available: true,
          available_slots: 0,
          overage: 11,
          reason: 'legacy_migration_capture_pending',
          blocks_new_systems: true,
          existing_monitoring_continues: true,
        },
      ),
    ).toBe(
      'This finite policy includes 5. Pulse is still verifying the migrated v5 continuity floor for this installation. Existing monitoring continues while additional monitored-system admissions pause until continuity capture finishes.',
    );
    expect(
      isMonitoredSystemLimitUrgent({
        current: 9,
        limit: 10,
        current_available: true,
        state: 'warning',
      }),
    ).toBe(true);
    expect(
      resolveMonitoredSystemCapacityStatus(undefined, {
        current: 16,
        limit: 5,
        current_available: true,
        state: 'enforced',
      }),
    ).toMatchObject({
      mode: 'over_limit_frozen',
      overage: 11,
      reason: 'preexisting_usage',
      blocks_new_systems: true,
      existing_monitoring_continues: true,
    });
  });

  it('returns canonical monitored-system admission unavailable copy', () => {
    expect(getMonitoredSystemAdmissionPreviewRequiredState()).toEqual({
      title: 'Preview monitored-system impact before saving',
      message:
        'Pulse must verify the monitored-system policy for this platform connection before it can be saved.',
    });
    expect(getMonitoredSystemAdmissionPreviewUnavailableTitle()).toBe(
      'Monitored-system verification is temporarily unavailable',
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
      'Pulse cannot verify monitored-system policy right now, so this connection cannot be saved yet. Retry preview in a moment.',
    );
    expect(
      buildMonitoredSystemAdmissionPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: ' supplemental_inventory_unsettled ',
      }),
    ).toEqual({
      reason: 'supplemental_inventory_unsettled',
      title: 'Monitored-system verification is temporarily unavailable',
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
      'Pulse must verify the monitored-system policy for this platform connection before it can be saved.',
    );
    expect(
      getMonitoredSystemAdmissionPreviewSaveBlockedMessage({
        preview: { would_exceed_limit: true },
      }),
    ).toBe('This change would exceed the active monitored-system policy');
    expect(
      getMonitoredSystemAdmissionPreviewSaveBlockedMessage({
        preview: { would_exceed_limit: false },
      }),
    ).toBeNull();
  });

  it('returns neutral monitored-system admission preview titles', () => {
    expect(getMonitoredSystemAdmissionPreviewTitle(null)).toBe('Monitored-system impact');
    expect(
      getMonitoredSystemAdmissionPreviewTitle({
        current_count: 4,
        projected_count: 5,
        would_exceed_limit: false,
      }),
    ).toBe('This change adds monitored systems');
    expect(
      getMonitoredSystemAdmissionPreviewTitle({
        current_count: 4,
        projected_count: 3,
        would_exceed_limit: false,
      }),
    ).toBe('This change removes monitored systems');
    expect(
      getMonitoredSystemAdmissionPreviewTitle({
        current_count: 4,
        projected_count: 4,
        would_exceed_limit: false,
      }),
    ).toBe('This change keeps monitored-system count unchanged');
    expect(
      getMonitoredSystemAdmissionPreviewTitle({
        current_count: 4,
        projected_count: 11,
        would_exceed_limit: true,
      }),
    ).toBe('This change exceeds the active monitored-system policy');
  });

  it('formats monitored-system admission preview summaries without quota math', () => {
    expect(
      formatMonitoredSystemAdmissionPreviewSummary({
        current_count: 4,
        projected_count: 4,
        limit: 10,
      }),
    ).toBe(
      'Pulse currently counts 4 monitored systems. Saving this change would keep the count at 4 monitored systems.',
    );
    expect(
      formatMonitoredSystemAdmissionPreviewSummary({
        current_count: 9,
        projected_count: 11,
        limit: 10,
        would_exceed_limit: true,
      }),
    ).toBe(
      'Pulse currently counts 9 monitored systems. Saving this change would bring the count to 11 monitored systems (+2), above the active policy of 10 monitored systems.',
    );
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
