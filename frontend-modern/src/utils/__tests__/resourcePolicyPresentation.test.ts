import { describe, expect, it } from 'vitest';

import {
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
});
