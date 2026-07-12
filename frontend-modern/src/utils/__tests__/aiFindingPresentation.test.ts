import { describe, expect, it } from 'vitest';

import type { UnifiedFinding } from '@/stores/aiIntelligence';
import {
  getFindingEvidencePresentation,
  getFindingResourceCriticalitySortOrder,
  sortFindingsForAttentionQueue,
} from '@/utils/aiFindingPresentation';

describe('getFindingEvidencePresentation', () => {
  it('presents bounded update evidence without package inventory or fingerprints', () => {
    const evidence = getFindingEvidencePresentation(makeFinding({ key: 'apt-host-updates', evidence: 'pending_updates=6 inventory=sha256:secret checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z reboot_required=true' }));
    expect(evidence).toContain('6 operating system updates were pending');
    expect(evidence).toContain('Pulse received that observation');
    expect(evidence).toContain('reboot required: Yes');
    expect(evidence).toContain('No reboot is authorized by this finding or action');
    expect(evidence).not.toContain('sha256');
    expect(evidence).not.toContain('inventory');
  });

  it('presents cleanup pressure without exposing the raw fingerprint', () => {
    const evidence = getFindingEvidencePresentation(makeFinding({ key: 'apt-package-cache-pressure', evidence: 'reclaimable_bytes=104857600 filesystem_usage=91.5 fingerprint=sha256:secret checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z' }));
    expect(evidence).toContain('100 MB of downloaded package data');
    expect(evidence).toContain('91.5% full');
    expect(evidence).not.toContain('fingerprint');
    expect(evidence).not.toContain('sha256');
  });

  it('fails closed instead of partially interpreting malformed APT evidence', () => {
    expect(getFindingEvidencePresentation(makeFinding({ key: 'apt-host-updates', evidence: 'pending_updates=6 inventory=leaked' }))).toContain('could not safely present');
  });

  it.each([
    ['unsafe update count', 'apt-host-updates', 'pending_updates=999999999999999999999 inventory=sha256:x checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z reboot_required=false'],
    ['invalid update timestamp', 'apt-host-updates', 'pending_updates=6 inventory=sha256:x checked_at=not-a-time received_at=2026-07-12T10:05:00Z reboot_required=false'],
    ['dotted usage', 'apt-package-cache-pressure', 'reclaimable_bytes=100 filesystem_usage=9.1.5 fingerprint=sha256:x checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z'],
    ['NaN usage', 'apt-package-cache-pressure', 'reclaimable_bytes=100 filesystem_usage=NaN fingerprint=sha256:x checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z'],
    ['usage above 100', 'apt-package-cache-pressure', 'reclaimable_bytes=100 filesystem_usage=100.1 fingerprint=sha256:x checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z'],
    ['unsafe reclaimable bytes', 'apt-package-cache-pressure', 'reclaimable_bytes=999999999999999999999 filesystem_usage=91.5 fingerprint=sha256:x checked_at=2026-07-12T10:00:00Z received_at=2026-07-12T10:05:00Z'],
    ['invalid cleanup timestamp', 'apt-package-cache-pressure', 'reclaimable_bytes=100 filesystem_usage=91.5 fingerprint=sha256:x checked_at=2026-07-12T10:00:00Z received_at=not-a-time'],
  ] as const)('uses the bounded fallback for %s', (_name, key, evidence) => {
    expect(getFindingEvidencePresentation(makeFinding({ key, evidence }))).toContain('could not safely present');
  });
});

function makeFinding(overrides: Partial<UnifiedFinding>): UnifiedFinding {
  return {
    id: overrides.id ?? 'finding',
    source: 'ai-patrol',
    resourceId: overrides.resourceId ?? 'vm:101',
    resourceName: overrides.resourceName ?? 'db-primary',
    resourceType: overrides.resourceType ?? 'vm',
    category: 'performance',
    severity: overrides.severity ?? 'warning',
    title: overrides.title ?? 'CPU saturated',
    description: overrides.description ?? 'CPU is high',
    detectedAt: overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    lastSeenAt: overrides.lastSeenAt ?? overrides.detectedAt ?? '2026-06-30T08:00:00Z',
    status: overrides.status ?? 'active',
    investigationOutcome: overrides.investigationOutcome ?? 'fix_failed',
    ...overrides,
  };
}

describe('getFindingResourceCriticalitySortOrder', () => {
  it('orders explicit resource priority around the default posture', () => {
    expect(getFindingResourceCriticalitySortOrder('high')).toBeLessThan(
      getFindingResourceCriticalitySortOrder('medium'),
    );
    expect(getFindingResourceCriticalitySortOrder('medium')).toBeLessThan(
      getFindingResourceCriticalitySortOrder(undefined),
    );
    expect(getFindingResourceCriticalitySortOrder(undefined)).toBeLessThan(
      getFindingResourceCriticalitySortOrder('low'),
    );
  });
});

describe('sortFindingsForAttentionQueue', () => {
  it('uses resource criticality before recency for same-severity findings', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({
        id: 'low-newest',
        resourceCriticality: 'low',
        lastSeenAt: '2026-06-30T08:30:00Z',
      }),
      makeFinding({
        id: 'default-middle',
        lastSeenAt: '2026-06-30T08:20:00Z',
      }),
      makeFinding({
        id: 'high-oldest',
        resourceCriticality: 'high',
        lastSeenAt: '2026-06-30T08:10:00Z',
      }),
    ]);

    expect(sorted.map((finding) => finding.id)).toEqual([
      'high-oldest',
      'default-middle',
      'low-newest',
    ]);
  });

  it('does not allow resource criticality to outrank severity', () => {
    const sorted = sortFindingsForAttentionQueue([
      makeFinding({
        id: 'warning-high',
        severity: 'warning',
        resourceCriticality: 'high',
      }),
      makeFinding({
        id: 'critical-low',
        severity: 'critical',
        resourceCriticality: 'low',
      }),
    ]);

    expect(sorted.map((finding) => finding.id)).toEqual(['critical-low', 'warning-high']);
  });
});
