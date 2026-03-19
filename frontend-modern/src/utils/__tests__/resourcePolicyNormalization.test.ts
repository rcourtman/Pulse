import { describe, expect, it } from 'vitest';

import {
  normalizeResourcePolicyAISafeSummary,
  normalizeResourcePolicy,
  normalizeResourcePolicyRedactionHints,
  normalizeResourcePolicyRoutingScope,
  normalizeResourcePolicySensitivity,
} from '../resourcePolicyNormalization';

describe('resourcePolicyNormalization', () => {
  it('normalizes canonical policy fields from mixed-case API payloads', () => {
    expect(normalizeResourcePolicySensitivity('  PUBLIC ')).toBe('public');
    expect(normalizeResourcePolicyRoutingScope('LOCAL-FIRST')).toBe('local-first');
    expect(
      normalizeResourcePolicyRedactionHints(['HOSTNAME', ' ip-address ', 'platform-id']),
    ).toEqual(['hostname', 'ip-address', 'platform-id']);

    expect(
      normalizeResourcePolicy({
        sensitivity: 'restricted',
        routing: {
          scope: 'Local-Only',
          allowCloudSummary: true,
          allowCloudRawSignals: false,
          redact: ['alias', 'PATH'],
        },
      }),
    ).toEqual({
      sensitivity: 'restricted',
      routing: {
        scope: 'local-only',
        allowCloudSummary: true,
        allowCloudRawSignals: false,
        redact: ['alias', 'path'],
      },
    });
  });

  it('rejects incomplete policy payloads', () => {
    expect(normalizeResourcePolicySensitivity(undefined)).toBeUndefined();
    expect(normalizeResourcePolicyRoutingScope('')).toBeUndefined();
    expect(normalizeResourcePolicyRedactionHints([])).toBeUndefined();
    expect(normalizeResourcePolicy({ sensitivity: 'internal' })).toBeUndefined();
  });

  it('normalizes aiSafeSummary to a trimmed string', () => {
    expect(normalizeResourcePolicyAISafeSummary('  safe summary  ')).toBe('safe summary');
    expect(normalizeResourcePolicyAISafeSummary('   ')).toBeUndefined();
    expect(normalizeResourcePolicyAISafeSummary(undefined)).toBeUndefined();
  });
});
