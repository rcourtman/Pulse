import { describe, expect, it } from 'vitest';

import proLicensePanelStateSource from '@/components/Settings/useProLicensePanelState.ts?raw';
import monitoredSystemLedgerPanelSource from '@/components/Settings/MonitoredSystemLedgerPanel.tsx?raw';
import monitoredSystemLimitWarningBannerModelSource from '@/components/shared/monitoredSystemLimitWarningBannerModel.ts?raw';
import commercialBillingModelSource from '@/utils/commercialBillingModel.ts?raw';
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
        'Review the top-level monitored systems Pulse has identified for reporting, migration continuity, and support context.',
      tableNameLabel: 'Name',
      tableStatusLabel: 'Status',
      tableLatestIncludedSignalLabel: 'Latest Included Signal',
      countedSystemBadgeLabel: 'Counts as 1 monitored system',
      groupedSourcesHeading: 'Grouped sources',
      countingExplanationHeading: 'Why this counts',
      continuityHeading: 'Legacy continuity',
      continuityPlanLimitLabel: 'Plan baseline',
      continuityEffectiveLimitLabel: 'Current baseline',
      continuityGrandfatheredFloorLabel: 'Observed legacy estate',
      continuityCaptureLabel: 'Verification',
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
          'Pulse waits for the session visibility state before loading monitored-system usage details.',
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
        reviewPolicyLabel: 'Review continuity',
        installCollectorsLabel: 'Install v6 collectors',
        overflowSummaryPrefix: 'A temporary setup slot is active',
        legacyConnectionSuffix:
          'that are folded into the canonical monitored-system ledger when the same top-level system is discovered canonically.',
      },
      admissionPreview: {
        requiredTitle: 'Preview monitored-system impact before saving',
        requiredMessage:
          'Pulse must preview the monitored-system impact for this platform connection before it can be saved.',
        fallbackTitle: 'Monitored-system impact',
        exceedsPolicyTitle: 'This change needs continuity review before saving',
        addsSystemsTitle: 'This change adds monitored systems',
        removesSystemsTitle: 'This change removes monitored systems',
        unchangedTitle: 'This change keeps monitored-system count unchanged',
        unavailableTitle: 'Monitored-system verification is temporarily unavailable',
        unavailableFallbackMessage:
          'Pulse cannot verify monitored-system impact right now, so this connection cannot be saved yet. Retry preview in a moment.',
        unavailableUnsettledMessage:
          'Pulse is still settling provider-owned inventory for this platform connection, so the monitored-system check is not safe yet. Retry preview after the first baseline finishes.',
        unavailableRebuildPendingMessage:
          'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view, so this connection cannot be saved yet. Retry preview in a moment.',
        saveBlockedLimitMessage: 'This change needs monitored-system review before saving',
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
      'Review the top-level monitored systems Pulse has identified for reporting, migration continuity, and support context.',
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
        'Pulse waits for the session visibility state before loading monitored-system usage details.',
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

    const paidLimitPhrases = [
      'active monitored-system policy',
      'additional monitored-system admissions',
      'finite policy',
      'policy boundary',
      'capacity is available',
      'Grandfathered monitored-system floor',
      'effective monitored-system limit',
      'Plan Monitored System Limit',
      'Effective Monitored System Limit',
      'Included Monitored Systems',
      'remaining before',
      'Over policy',
    ];
    for (const source of [
      commercialBillingModelSource,
      licensePresentationSource,
      monitoredSystemLedgerPanelSource,
      monitoredSystemLimitWarningBannerModelSource,
      monitoredSystemPresentationSource,
      proLicensePanelStateSource,
    ]) {
      for (const phrase of paidLimitPhrases) {
        expect(source).not.toContain(phrase);
      }
    }
  });

  it('returns canonical monitored-system limit warning copy', () => {
    expect(getMonitoredSystemLimitReviewPolicyLabel()).toBe('Review continuity');
    expect(getMonitoredSystemLimitInstallCollectorsLabel()).toBe('Install v6 collectors');
    expect(formatMonitoredSystemLimitSummary({ current: 5, limit: 6 })).toBe(
      '5 monitored systems.',
    );
    expect(formatMonitoredSystemLimitSummary({ current: 16, limit: 5, state: 'enforced' })).toBe(
      'Continuity review needed. 16 monitored systems.',
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
    ).toBe('Continuity verification pending. 16 monitored systems.');
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
        { label: 'Baseline', value: '5' },
        { label: 'Status', value: 'Continuity review' },
      ],
      statusMessage: 'Existing monitoring remains visible. New top-level additions need review.',
      detailMessage:
        'Review the legacy continuity state before adding another top-level monitored system.',
      explanation: {
        label: 'Why does this need review?',
        body: 'Pulse has already identified 16 monitored systems for this installation. Existing monitoring remains visible, but new top-level additions are paused until this legacy continuity state is reviewed.',
      },
    });
  });

  it('does not build a monitored-system capacity section for unmetered or healthy self-hosted states', () => {
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
    expect(
      buildMonitoredSystemCapacitySectionModel({
        current: 7,
        limit: 10,
        current_available: true,
        state: 'ok',
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
    ).toBe('Healthy');
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
      'Existing monitoring remains visible. New top-level additions are paused until this legacy continuity state is reviewed.',
    );
    expect(
      getMonitoredSystemLimitContextSummary({
        current: 16,
        limit: 5,
        current_available: true,
        state: 'enforced',
      }),
    ).toBe(
      'Existing monitoring remains visible. New top-level additions are paused until this legacy continuity state is reviewed.',
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
      'Pulse is verifying legacy v5 continuity for this installation. Existing monitoring remains visible while new top-level additions wait for verification to finish.',
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
        'Pulse must preview the monitored-system impact for this platform connection before it can be saved.',
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
      'Pulse cannot verify monitored-system impact right now, so this connection cannot be saved yet. Retry preview in a moment.',
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
      'Pulse must preview the monitored-system impact for this platform connection before it can be saved.',
    );
    expect(
      getMonitoredSystemAdmissionPreviewSaveBlockedMessage({
        preview: { would_exceed_limit: true },
      }),
    ).toBe('This change needs monitored-system review before saving');
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
    ).toBe('This change needs continuity review before saving');
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
      'Pulse currently counts 9 monitored systems. Saving this change would bring the count to 11 monitored systems (+2), above the current verified baseline of 10 monitored systems.',
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
