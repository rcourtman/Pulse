import { describe, expect, it } from 'vitest';

import {
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
  hasDefaultResourcePolicyPosture,
  getResourcePolicyTableBadges,
  getResourcePolicyGovernedSummary,
  getResourcePolicyDisplayLabel,
  getResourcePolicyRedactionSummaries,
  getResourcePolicyRoutingSummaries,
  getResourcePolicySensitivitySummaries,
  getResourcePolicyRedactionLabels,
  getResourceRedactionHintLabel,
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
  shouldShowResourceAlternateName,
} from '@/utils/resourcePolicyPresentation';

describe('resourcePolicyPresentation utils', () => {
  it('formats canonical policy labels', () => {
    expect(getResourceSensitivityLabel('restricted')).toBe('Restricted');
    expect(getResourceRoutingScopeLabel('local-only')).toBe('Local Only');
    expect(getResourceRedactionHintLabel('platform-id')).toBe('Platform ID');
  });

  it('formats canonical redaction labels from policy hints', () => {
    expect(
      getResourcePolicyRedactionLabels({
        sensitivity: 'sensitive',
        routing: {
          scope: 'local-first',
          redact: ['hostname', 'ip-address'],
        },
      }),
    ).toEqual(['Hostname', 'IP Address']);
  });

  it('suppresses the default internal cloud-summary posture in table rows', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: {
          scope: 'cloud-summary',
        },
      }),
    ).toBe(true);

    expect(
      getResourcePolicyTableBadges({
        sensitivity: 'internal',
        routing: {
          scope: 'cloud-summary',
        },
      }),
    ).toEqual([]);

    expect(
      getResourcePolicyTableBadges({
        sensitivity: 'sensitive',
        routing: {
          scope: 'local-first',
          redact: ['hostname'],
        },
      }).map((badge) => badge.label),
    ).toEqual(['Sensitive', 'Local First']);

    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'sensitive',
        routing: {
          scope: 'local-first',
          redact: ['hostname'],
        },
      }),
    ).toBe(false);
  });

  it('uses concise governed labels for redacted resources', () => {
    expect(
      getResourcePolicyDisplayLabel({
        name: 'sensitive-host',
        displayName: 'Sensitive Host',
        policy: {
          sensitivity: 'restricted',
          routing: {
            scope: 'local-only',
            redact: ['hostname', 'ip-address'],
          },
        },
        aiSafeSummary: 'restricted host summary safe for remote AI consumption',
      }),
    ).toBe('restricted host summary safe for remote AI consumption');

    expect(
      getResourcePolicyDisplayLabel({
        name: 'pbs-secret',
        displayName: 'PBS Secret',
        policy: {
          sensitivity: 'sensitive',
          routing: {
            scope: 'local-first',
            redact: ['hostname', 'platform-id'],
          },
        },
        aiSafeSummary:
          'backup server resource; status online; sources pbs; 1 child resources; redacted for cloud summary',
      }),
    ).toBe('backup server (online)');

    expect(
      getResourcePolicyDisplayLabel({
        name: 'storage-1',
        displayName: 'Storage 1',
        policy: {
          sensitivity: 'sensitive',
          routing: {
            scope: 'local-first',
            redact: ['path'],
          },
        },
      }),
    ).toBe('redacted by policy');
  });

  it('preserves the full governed summary for detail surfaces', () => {
    expect(
      getResourcePolicyGovernedSummary({
        name: 'pbs-secret',
        displayName: 'PBS Secret',
        policy: {
          sensitivity: 'sensitive',
          routing: {
            scope: 'local-first',
            redact: ['hostname', 'platform-id'],
          },
        },
        aiSafeSummary:
          'backup server resource; status online; sources pbs; 1 child resources; redacted for cloud summary',
      }),
    ).toBe(
      'backup server resource; status online; sources pbs; 1 child resources; redacted for cloud summary',
    );
  });

  it('hides raw alternate names when policy requires governed handling', () => {
    expect(
      shouldShowResourceAlternateName({
        name: 'sensitive-host',
        displayName: 'Sensitive Host',
        policy: {
          sensitivity: 'restricted',
          routing: {
            scope: 'local-only',
            redact: ['hostname'],
          },
        },
      }),
    ).toBe(false);

    expect(
      shouldShowResourceAlternateName({
        name: 'host-1',
        displayName: 'Host 1',
      }),
    ).toBe(true);
  });

  it('formats canonical policy count summaries', () => {
    expect(
      getResourcePolicySensitivitySummaries({
        total_resources: 3,
        sensitivity_counts: {
          public: 1,
          internal: 2,
        },
        routing_counts: {},
      }),
    ).toEqual([
      { label: 'Public', count: 1 },
      { label: 'Internal', count: 2 },
      { label: 'Sensitive', count: 0 },
      { label: 'Restricted', count: 0 },
    ]);

    expect(
      getResourcePolicyRoutingSummaries({
        total_resources: 3,
        sensitivity_counts: {},
        routing_counts: {
          'cloud-summary': 1,
          'local-first': 2,
        },
      }),
    ).toEqual([
      { label: 'Cloud Summary', count: 1 },
      { label: 'Local First', count: 2 },
      { label: 'Local Only', count: 0 },
    ]);

    expect(
      getResourcePolicyRedactionSummaries({
        total_resources: 3,
        sensitivity_counts: {},
        routing_counts: {
          'cloud-summary': 1,
        },
        redaction_counts: {
          hostname: 2,
          path: 1,
        },
      }),
    ).toEqual([
      { label: 'Hostname', count: 2 },
      { label: 'Path', count: 1 },
    ]);
  });

  it('exports canonical policy ordering', () => {
    expect(RESOURCE_POLICY_SENSITIVITY_ORDER).toEqual([
      'public',
      'internal',
      'sensitive',
      'restricted',
    ]);
    expect(RESOURCE_POLICY_ROUTING_ORDER).toEqual(['cloud-summary', 'local-first', 'local-only']);
    expect(RESOURCE_POLICY_REDACTION_ORDER).toEqual([
      'hostname',
      'ip-address',
      'platform-id',
      'alias',
      'path',
    ]);
  });
});
