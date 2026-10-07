import { describe, expect, it } from 'vitest';

import {
  RESOURCE_POLICY_REDACTION_ORDER,
  RESOURCE_POLICY_ROUTING_ORDER,
  RESOURCE_POLICY_SENSITIVITY_ORDER,
  hasDefaultResourcePolicyPosture,
  getResourcePolicyGovernedSummary,
  getResourcePolicyDisplayLabel,
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
          redact: ['hostname', 'ip-address'],
        },
      }),
    ).toEqual(['Hostname', 'IP Address']);
  });

  it('recognizes only the canonical default policy posture', () => {
    expect(
      hasDefaultResourcePolicyPosture({
        sensitivity: 'internal',
        routing: {
          scope: 'cloud-summary',
        },
      }),
    ).toBe(true);

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
