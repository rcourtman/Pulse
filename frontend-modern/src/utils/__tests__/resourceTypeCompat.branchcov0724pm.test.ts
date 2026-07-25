import { describe, expect, it } from 'vitest';
import { canonicalizeFrontendResourceType } from '@/utils/resourceTypeCompat';

describe('resourceTypeCompat.branchcov0724pm', () => {
  it('rejects non-string input by returning undefined', () => {
    expect(canonicalizeFrontendResourceType(123)).toBeUndefined();
    expect(canonicalizeFrontendResourceType(0)).toBeUndefined();
    expect(canonicalizeFrontendResourceType(true)).toBeUndefined();
    expect(canonicalizeFrontendResourceType(null)).toBeUndefined();
    expect(canonicalizeFrontendResourceType(undefined)).toBeUndefined();
    expect(canonicalizeFrontendResourceType({ type: 'agent' })).toBeUndefined();
    expect(canonicalizeFrontendResourceType(['agent'])).toBeUndefined();
  });

  it('rejects empty and whitespace-only strings by returning undefined', () => {
    expect(canonicalizeFrontendResourceType('')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('   ')).toBeUndefined();
    expect(canonicalizeFrontendResourceType('\t\n')).toBeUndefined();
  });

  it('canonicalizes the remaining hyphenated / unseparated legacy aliases', () => {
    expect(canonicalizeFrontendResourceType('dockerhost')).toBe('docker-host');
    expect(canonicalizeFrontendResourceType('swarm-secret')).toBe('docker-secret');
    expect(canonicalizeFrontendResourceType('swarm-config')).toBe('docker-config');
    expect(canonicalizeFrontendResourceType('kubernetes-cluster')).toBe('k8s-cluster');
  });

  it('identity-maps canonical type strings that only appear in the second switch arm', () => {
    expect(canonicalizeFrontendResourceType('agent')).toBe('agent');
    expect(canonicalizeFrontendResourceType('storage')).toBe('storage');
    expect(canonicalizeFrontendResourceType('disk')).toBe('disk');
    expect(canonicalizeFrontendResourceType('docker-host')).toBe('docker-host');
  });

  it('maps the bare "node" legacy alias to agent', () => {
    expect(canonicalizeFrontendResourceType('node')).toBe('agent');
  });
});
