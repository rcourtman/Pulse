import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  buildStandalonePostureSummary,
  getStandaloneResourceStatusIndicator,
} from '../standalonePageModel';

const NOW_MS = Date.parse('2026-07-24T12:00:00Z');

const resource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'agent-1',
    name: overrides.name ?? overrides.id ?? 'agent-1',
    displayName: overrides.displayName ?? overrides.name ?? overrides.id ?? 'agent-1',
    type: overrides.type ?? 'agent',
    platformId: overrides.platformId ?? 'agent-1',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? NOW_MS,
    ...overrides,
  }) as Resource;

describe('standalonePageModel branch coverage', () => {
  it('treats an ambiguous availability correlation with fresh evidence as an unresolved-identity warning', () => {
    // Freshness is deliberately fresh (recent lastChecked + short poll
    // interval keep it inside the freshness window) so the warning can only
    // come from correlationState === 'ambiguous', and the label ternary falls
    // through to 'Identity unresolved'.
    const indicator = getStandaloneResourceStatusIndicator(
      resource({
        id: 'availability:ambiguous',
        type: 'network-endpoint',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetId: 'ambiguous',
          correlationState: 'ambiguous',
          available: true,
          lastChecked: '2026-07-24T11:59:00Z',
          pollIntervalSeconds: 60,
        },
      }),
      NOW_MS,
    );

    expect(indicator).toEqual({ variant: 'warning', label: 'Identity unresolved' });
  });

  it('treats an unresolved availability correlation with fresh evidence as an unresolved-identity warning', () => {
    const indicator = getStandaloneResourceStatusIndicator(
      resource({
        id: 'availability:unresolved',
        type: 'network-endpoint',
        platformType: 'availability',
        status: 'online',
        availability: {
          targetId: 'unresolved',
          correlationState: 'unresolved',
          available: true,
          lastChecked: '2026-07-24T11:59:00Z',
          pollIntervalSeconds: 60,
        },
      }),
      NOW_MS,
    );

    expect(indicator).toEqual({ variant: 'warning', label: 'Identity unresolved' });
  });

  it('treats an agent carrying the explicit stale flag as attention even when its last report is recent', () => {
    // lastSeen is one minute before now, far inside the 5-minute stale
    // window, so the time-based staleness expression cannot be the cause;
    // only resource.agent?.stale === true forces the warning here.
    const indicator = getStandaloneResourceStatusIndicator(
      resource({
        id: 'agent-flag-stale',
        type: 'agent',
        status: 'online',
        lastSeen: NOW_MS - 60_000,
        agent: { stale: true },
      }),
      NOW_MS,
    );

    expect(indicator).toEqual({ variant: 'warning', label: 'Stale' });
  });

  it('counts a status outside online/degraded/offline buckets as an unknown posture row', () => {
    // 'paused' is not in OFFLINE/DEGRADED/online sets, so the base indicator
    // resolves to the muted variant and the summary falls into the unknown
    // else arm. lastSeen is non-positive so latestUpdateAt must stay unset.
    const summary = buildStandalonePostureSummary(
      [
        resource({
          id: 'paused-endpoint',
          type: 'network-endpoint',
          status: 'paused',
          lastSeen: 0,
        }),
      ],
      NOW_MS,
    );

    expect(summary).toEqual({
      attention: 0,
      critical: 0,
      normal: 0,
      total: 1,
      unknown: 1,
      warning: 0,
    });
    expect(summary.latestUpdateAt).toBeUndefined();
  });
});
