import { describe, expect, it } from 'vitest';

import type { MonitoredSystemImpactPreviewSummaryInput } from '@/utils/monitoredSystemPresentation';
import {
  buildMonitoredSystemImpactPreviewUnavailableState,
  formatMonitoredSystemImpactPreviewSummary,
  formatMonitoredSystemImpactPreviewUnavailableMessage,
  formatMonitoredSystemSurfaceAttribution,
  formatMonitoredSystemUsageUnavailableMessage,
  getMonitoredSystemImpactPreviewTitle,
  getMonitoredSystemSourceLabel,
  getMonitoredSystemSurfaceTypeLabel,
} from '@/utils/monitoredSystemPresentation';

describe('getMonitoredSystemSourceLabel (branch coverage)', () => {
  it('maps each canonical source token to its display label', () => {
    expect(getMonitoredSystemSourceLabel('agent')).toBe('Agent');
    expect(getMonitoredSystemSourceLabel('docker')).toBe('Docker');
    expect(getMonitoredSystemSourceLabel('kubernetes')).toBe('Kubernetes');
    expect(getMonitoredSystemSourceLabel('multiple')).toBe('Multiple Sources');
    expect(getMonitoredSystemSourceLabel('pbs')).toBe('PBS');
    expect(getMonitoredSystemSourceLabel('pmg')).toBe('PMG');
    expect(getMonitoredSystemSourceLabel('proxmox')).toBe('Proxmox');
    expect(getMonitoredSystemSourceLabel('truenas')).toBe('TrueNAS');
  });

  it('normalizes case and surrounding whitespace before matching', () => {
    expect(getMonitoredSystemSourceLabel('DOCKER')).toBe('Docker');
    expect(getMonitoredSystemSourceLabel('  Multiple  ')).toBe('Multiple Sources');
  });

  it('returns an empty string for an empty or undefined source', () => {
    expect(getMonitoredSystemSourceLabel('')).toBe('');
    expect(getMonitoredSystemSourceLabel(undefined)).toBe('');
  });

  it('passes an unrecognized source through trimmed (without lower-casing) on the default arm', () => {
    expect(getMonitoredSystemSourceLabel('BackupNinja')).toBe('BackupNinja');
    expect(getMonitoredSystemSourceLabel('  HybridCloud  ')).toBe('HybridCloud');
  });
});

describe('getMonitoredSystemSurfaceTypeLabel (branch coverage)', () => {
  it('maps each canonical surface type token to its display label', () => {
    expect(getMonitoredSystemSurfaceTypeLabel('agent')).toBe('Host');
    expect(getMonitoredSystemSurfaceTypeLabel('host')).toBe('Host');
    expect(getMonitoredSystemSurfaceTypeLabel('kubernetes-cluster')).toBe('Kubernetes Cluster');
    expect(getMonitoredSystemSurfaceTypeLabel('pbs-server')).toBe('PBS Server');
    expect(getMonitoredSystemSurfaceTypeLabel('pmg-server')).toBe('PMG Server');
    expect(getMonitoredSystemSurfaceTypeLabel('proxmox-node')).toBe('Proxmox Node');
    expect(getMonitoredSystemSurfaceTypeLabel('truenas-system')).toBe('TrueNAS System');
  });

  it('returns the generic "System" label for an empty or undefined type', () => {
    expect(getMonitoredSystemSurfaceTypeLabel('')).toBe('System');
    expect(getMonitoredSystemSurfaceTypeLabel(undefined)).toBe('System');
  });

  it('title-cases a custom type with dashes and underscores on the default arm', () => {
    expect(getMonitoredSystemSurfaceTypeLabel('custom_backend')).toBe('Custom Backend');
    expect(getMonitoredSystemSurfaceTypeLabel('weird--type__x')).toBe('Weird Type X');
  });

  it('normalizes case before matching a canonical token', () => {
    expect(getMonitoredSystemSurfaceTypeLabel('AGENT')).toBe('Host');
  });
});

describe('formatMonitoredSystemUsageUnavailableMessage (branch coverage)', () => {
  it('returns the rebuild-pending message for the rebuild reason', () => {
    expect(
      formatMonitoredSystemUsageUnavailableMessage('supplemental_inventory_rebuild_pending'),
    ).toBe(
      'Pulse has collected provider-owned inventory and is rebuilding the canonical monitored-system ledger. Usage will appear when that rebuild finishes.',
    );
  });

  it('falls back to the generic message for an unrecognized, empty, or missing reason', () => {
    expect(formatMonitoredSystemUsageUnavailableMessage('some_other_reason')).toBe(
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    );
    expect(formatMonitoredSystemUsageUnavailableMessage('')).toBe(
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    );
    expect(formatMonitoredSystemUsageUnavailableMessage(undefined)).toBe(
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    );
    expect(formatMonitoredSystemUsageUnavailableMessage('   ')).toBe(
      'Pulse cannot currently verify monitored-system usage for this installation. Refresh after the monitoring runtime settles.',
    );
  });

  it('matches the unsettled reason case-insensitively', () => {
    expect(formatMonitoredSystemUsageUnavailableMessage('SUPPLEMENTAL_INVENTORY_UNSETTLED')).toBe(
      'Pulse is still collecting the first provider-owned inventory baseline. The monitored-system ledger will appear after that baseline completes.',
    );
  });
});

describe('formatMonitoredSystemImpactPreviewUnavailableMessage (branch coverage)', () => {
  it('returns the unsettled impact message for the unsettled reason', () => {
    expect(
      formatMonitoredSystemImpactPreviewUnavailableMessage('supplemental_inventory_unsettled'),
    ).toBe(
      'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    );
  });

  it('falls back to the generic impact message for an unrecognized or missing reason', () => {
    expect(formatMonitoredSystemImpactPreviewUnavailableMessage('unrecognized_reason')).toBe(
      'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    );
    expect(formatMonitoredSystemImpactPreviewUnavailableMessage(undefined)).toBe(
      'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    );
    expect(formatMonitoredSystemImpactPreviewUnavailableMessage('')).toBe(
      'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    );
  });
});

describe('buildMonitoredSystemImpactPreviewUnavailableState (branch coverage)', () => {
  it('returns null when the error code does not match', () => {
    expect(buildMonitoredSystemImpactPreviewUnavailableState({ code: 'other_error' })).toBeNull();
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({ code: null, reason: 'supplemental_inventory_unsettled' }),
    ).toBeNull();
  });

  it('matches the unavailable error code case-insensitively', () => {
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({
        code: 'MONITORED_SYSTEM_USAGE_UNAVAILABLE',
        reason: 'supplemental_inventory_unsettled',
      }),
    ).toEqual({
      reason: 'supplemental_inventory_unsettled',
      title: 'Monitored-system verification is temporarily unavailable',
      message:
        'Pulse is still settling provider-owned inventory for this platform connection. You can still save the connection and review the impact after the first baseline finishes.',
    });
  });

  it('coerces a null or whitespace-only reason to null and uses the fallback message', () => {
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: undefined,
      }),
    ).toEqual({
      reason: null,
      title: 'Monitored-system verification is temporarily unavailable',
      message:
        'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    });
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: '   ',
      }),
    ).toEqual({
      reason: null,
      title: 'Monitored-system verification is temporarily unavailable',
      message:
        'Pulse cannot verify monitored-system impact right now. You can still save the connection and review the impact after inventory refreshes.',
    });
  });

  it('uses the rebuild-pending message for the rebuild reason', () => {
    expect(
      buildMonitoredSystemImpactPreviewUnavailableState({
        code: 'monitored_system_usage_unavailable',
        reason: 'supplemental_inventory_rebuild_pending',
      }),
    ).toEqual({
      reason: 'supplemental_inventory_rebuild_pending',
      title: 'Monitored-system verification is temporarily unavailable',
      message:
        'Pulse has settled provider-owned inventory and is rebuilding the canonical monitored-system view. You can still save the connection and review the impact in a moment.',
    });
  });
});

describe('getMonitoredSystemImpactPreviewTitle (branch coverage)', () => {
  it('falls back to the default title for an undefined preview', () => {
    expect(getMonitoredSystemImpactPreviewTitle(undefined)).toBe('Monitored-system impact');
  });

  it('falls back to current when projected_count is absent or non-finite (delta 0)', () => {
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: 7 })).toBe(
      'This change keeps monitored-system count unchanged',
    );
    expect(
      getMonitoredSystemImpactPreviewTitle({ current_count: 7, projected_count: Number.NaN }),
    ).toBe('This change keeps monitored-system count unchanged');
  });

  it('clamps a negative current count to zero before computing delta', () => {
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: -3, projected_count: 1 })).toBe(
      'This change adds monitored systems',
    );
  });

  it('clamps a negative projected count to zero so a drop registers as a removal', () => {
    expect(getMonitoredSystemImpactPreviewTitle({ current_count: 1, projected_count: -2 })).toBe(
      'This change removes monitored systems',
    );
  });

  it('treats a non-finite current count as zero', () => {
    expect(
      getMonitoredSystemImpactPreviewTitle({
        current_count: Number.NaN,
        projected_count: Number.NaN,
      }),
    ).toBe('This change keeps monitored-system count unchanged');
  });
});

describe('formatMonitoredSystemImpactPreviewSummary (branch coverage)', () => {
  it('describes an unchanged count and uses the singular form for one system', () => {
    expect(
      formatMonitoredSystemImpactPreviewSummary({ current_count: 3, projected_count: 3 }),
    ).toBe(
      'Pulse currently counts 3 monitored systems. Saving this change would keep the count at 3 monitored systems.',
    );
    expect(
      formatMonitoredSystemImpactPreviewSummary({ current_count: 1, projected_count: 1 }),
    ).toBe(
      'Pulse currently counts 1 monitored system. Saving this change would keep the count at 1 monitored system.',
    );
  });

  it('renders a negative delta without a leading plus sign', () => {
    expect(
      formatMonitoredSystemImpactPreviewSummary({ current_count: 5, projected_count: 2 }),
    ).toBe(
      'Pulse currently counts 5 monitored systems. Saving this change would bring the count to 2 monitored systems (-3).',
    );
  });

  it('falls back to current when projected_count is absent (delta 0)', () => {
    expect(
      formatMonitoredSystemImpactPreviewSummary({ current_count: 4 } as MonitoredSystemImpactPreviewSummaryInput),
    ).toBe(
      'Pulse currently counts 4 monitored systems. Saving this change would keep the count at 4 monitored systems.',
    );
  });
});

describe('formatMonitoredSystemSurfaceAttribution (branch coverage)', () => {
  it('omits the "via" clause when the source label is empty', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution({ name: 'node-1', type: 'host', source: '' }),
    ).toBe('node-1 (Host)');
  });

  it('omits the "via" clause when the source is undefined', () => {
    expect(formatMonitoredSystemSurfaceAttribution({ name: 'node-1', type: 'host' })).toBe(
      'node-1 (Host)',
    );
  });

  it('omits the "via" clause when the source label matches the type label case-insensitively', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution({ name: 'pve', type: 'host', source: 'Host' }),
    ).toBe('pve (Host)');
  });

  it('falls back to "Unnamed source" when the name is empty or whitespace', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution({ name: '', type: 'host', source: 'vmware' }),
    ).toBe('Unnamed source (Host via VMware)');
    expect(
      formatMonitoredSystemSurfaceAttribution({ name: '   ', type: 'docker-host', source: 'docker' }),
    ).toBe('Unnamed source (Docker Host via Docker)');
  });

  it('renders both type and source when they differ', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution({
        name: 'cluster-1',
        type: 'kubernetes-cluster',
        source: 'kubernetes',
      }),
    ).toBe('cluster-1 (Kubernetes Cluster via Kubernetes)');
  });

  it('falls back to "Unnamed source" when the name property is missing', () => {
    expect(
      formatMonitoredSystemSurfaceAttribution(
        { type: 'host', source: 'vmware' } as Parameters<typeof formatMonitoredSystemSurfaceAttribution>[0],
      ),
    ).toBe('Unnamed source (Host via VMware)');
  });
});
