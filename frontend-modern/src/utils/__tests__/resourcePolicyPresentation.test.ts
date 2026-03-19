import { describe, expect, it } from 'vitest';

import {
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
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
