import { describe, expect, it } from 'vitest';
import {
  formatDiscoveryReadinessBriefingLine,
  formatReadinessAge,
  getDiscoveryReadinessPresentation,
} from '@/utils/resourceDiscoveryReadiness';

describe('resourceDiscoveryReadiness', () => {
  it('formats age labels without depending on wall-clock time', () => {
    expect(formatReadinessAge(30)).toBe('under a minute old');
    expect(formatReadinessAge(60)).toBe('1 minute old');
    expect(formatReadinessAge(3600)).toBe('1 hour old');
    expect(formatReadinessAge(86400)).toBe('1 day old');
  });

  it('presents fresh discovery with service and fact detail', () => {
    const presentation = getDiscoveryReadinessPresentation({
      state: 'fresh',
      reason: 'Discovery data is within the configured freshness window.',
      serviceName: 'Home Assistant',
      factCount: 7,
      ageSeconds: 120,
    });

    expect(presentation?.statusLabel).toBe('Discovery fresh');
    expect(presentation?.tone).toBe('success');
    expect(presentation?.detail).toContain('Home Assistant');
    expect(presentation?.detail).toContain('7 facts');
    expect(presentation?.detail).toContain('2 minutes old');
  });

  it('presents missing discovery as actionable absence', () => {
    const presentation = getDiscoveryReadinessPresentation({
      state: 'missing',
      reason: 'Discovery has not run for this resource.',
    });

    expect(presentation?.shortLabel).toBe('None');
    expect(presentation?.tone).toBe('muted');
    expect(presentation?.title).toContain('Discovery has not run');
  });

  it('renders the Assistant briefing line from readiness metadata', () => {
    expect(
      formatDiscoveryReadinessBriefingLine({
        state: 'stale',
        serviceName: 'Home Assistant',
        factCount: 5,
      }),
    ).toBe('Discovery data: Discovery stale, service Home Assistant, 5 facts');
  });
});
