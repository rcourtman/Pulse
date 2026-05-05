import { describe, expect, it } from 'vitest';

import {
  buildMonitoredSystemImpactPreviewUnavailableState,
  formatMonitoredSystemImpactPreviewSummary,
  formatMonitoredSystemImpactPreviewUnavailableMessage,
  formatMonitoredSystemGroupedSourcesLabel,
  formatMonitoredSystemLedgerUnavailableMessage,
  formatMonitoredSystemLatestIncludedSignalSentence,
  formatMonitoredSystemSurfaceAttribution,
  getMonitoredSystemImpactPreviewTitle,
  getMonitoredSystemImpactPreviewUnavailableTitle,
  getMonitoredSystemBriefSummary,
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
  getMonitoredSystemSourceLabel,
  getMonitoredSystemStatusFallbackSummary,
  getMonitoredSystemSurfaceTypeLabel,
} from '@/utils/monitoredSystemPresentation';

describe('monitoredSystemPresentation', () => {
  it('returns canonical ledger labels and fallback copy without capacity policy copy', () => {
    expect(getMonitoredSystemBriefSummary()).toBe(
      'Pulse counts top-level monitored systems. Child resources underneath them are included.',
    );
    expect(getMonitoredSystemLedgerDescription()).toBe(
      'Review the top-level monitored systems Pulse has identified for reporting and support context.',
    );
    expect(getMonitoredSystemDisclosureToggleLabel(false)).toBe('View counting rules');
    expect(getMonitoredSystemDisclosureToggleLabel(true)).toBe('Hide counting rules');
    expect(getMonitoredSystemDisclosureDefinition()).toContain('Each root counts once');
    expect(getMonitoredSystemCountingDetailsToggleLabel(false)).toBe('View counting details');
    expect(getMonitoredSystemCountingDetailsToggleLabel(true)).toBe('Hide counting details');
    expect(getMonitoredSystemExplanationFallbackSummary()).toBe(
      'Pulse counts this top-level collection path as one monitored system.',
    );
    expect(getMonitoredSystemLedgerLoadingState().text).toContain('Loading monitored system usage');
    expect(getMonitoredSystemLedgerErrorState().retryLabel).toBe('Try again');
    expect(getMonitoredSystemLedgerUnavailableState().title).toBe(
      'Verifying monitored-system inventory',
    );
    expect(getMonitoredSystemLedgerPolicyLoadingState().title).toBe(
      'Checking monitored-system visibility',
    );
    expect(getMonitoredSystemLedgerHiddenState().title).toBe(
      'Monitored-system usage is hidden in demo mode',
    );
    expect(JSON.stringify(getMonitoredSystemLedgerPresentation())).not.toContain('capacity');
    expect(JSON.stringify(getMonitoredSystemLedgerPresentation())).not.toContain('limit');
  });

  it('returns status, source, type, and attribution labels', () => {
    expect(getMonitoredSystemStatusFallbackSummary('online')).toBe(
      'All included top-level collection paths currently report online status.',
    );
    expect(getMonitoredSystemStatusFallbackSummary('warning')).toContain('degraded');
    expect(getMonitoredSystemStatusFallbackSummary('offline')).toContain('offline or disconnected');
    expect(getMonitoredSystemStatusFallbackSummary()).toContain('cannot determine');
    expect(getMonitoredSystemSourceLabel('vmware')).toBe('VMware');
    expect(getMonitoredSystemSourceLabel('unknown')).toBe('');
    expect(getMonitoredSystemSurfaceTypeLabel('docker-host')).toBe('Docker Host');
    expect(getMonitoredSystemSurfaceTypeLabel('custom-system')).toBe('Custom System');
    expect(
      formatMonitoredSystemSurfaceAttribution({
        name: 'esx-a',
        type: 'host',
        source: 'vmware',
      }),
    ).toBe('esx-a (Host via VMware)');
    expect(formatMonitoredSystemGroupedSourcesLabel(1)).toBe('1 grouped source');
    expect(formatMonitoredSystemGroupedSourcesLabel(3)).toBe('3 grouped sources');
    expect(
      formatMonitoredSystemLatestIncludedSignalSentence({
        attribution: 'esx-a (Host via VMware)',
        relative: '2m ago',
      }),
    ).toBe('Latest included signal: esx-a (Host via VMware), reported 2m ago.');
  });

  it('returns impact preview copy without quota math', () => {
    expect(getMonitoredSystemImpactPreviewUnavailableTitle()).toBe(
      'Monitored-system verification is temporarily unavailable',
    );
    expect(getMonitoredSystemImpactPreviewTitle(null)).toBe('Monitored-system impact');
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: 4, projected_count: 5 })).toBe(
      'This change adds monitored systems',
    );
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: 4, projected_count: 3 })).toBe(
      'This change removes monitored systems',
    );
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: 4, projected_count: 4 })).toBe(
      'This change keeps monitored-system count unchanged',
    );
    expect(
      formatMonitoredSystemImpactPreviewSummary({
        current_count: 9,
        projected_count: 11,
      }),
    ).toBe(
      'Pulse currently counts 9 monitored systems. Saving this change would bring the count to 11 monitored systems (+2).',
    );
  });

  it('returns monitored-system unavailable copy', () => {
    expect(formatMonitoredSystemLedgerUnavailableMessage('supplemental_inventory_unsettled')).toBe(
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    );
    expect(
      formatMonitoredSystemImpactPreviewUnavailableMessage(
        'supplemental_inventory_rebuild_pending',
      ),
    ).toBe(
      'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view. You can still save the connection and review the impact in a moment.',
    );
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: ' supplemental_inventory_unsettled ',
      }),
    ).toEqual({
      reason: 'supplemental_inventory_unsettled',
      title: 'Monitored-system verification is temporarily unavailable',
      message:
        'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    });
  });
});
