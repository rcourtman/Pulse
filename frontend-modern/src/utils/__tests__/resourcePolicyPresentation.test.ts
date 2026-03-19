import { describe, expect, it } from 'vitest';

import {
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
  getResourcePolicyRedactionSummaries,
  getResourcePolicyRoutingSummaries,
  getResourcePolicySensitivitySummaries,
  getResourcePolicyRedactionLabels,
  getResourceRedactionHintLabel,
  getResourceRoutingScopeLabel,
  getResourceSensitivityLabel,
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
          allowCloudSummary: true,
          allowCloudRawSignals: false,
          redact: ['hostname', 'ip-address'],
        },
      }),
    ).toEqual(['Hostname', 'IP Address']);
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
