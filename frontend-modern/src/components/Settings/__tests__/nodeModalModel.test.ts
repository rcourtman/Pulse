import { describe, expect, it } from 'vitest';
import type { ClusterEndpoint } from '@/types/nodes';
import {
  applyClusterEndpointOverridesLocally,
  buildClusterEndpointOverridesPayload,
  PVE_MANUAL_PERMISSION_COMMAND,
} from '../nodeModalModel';

const endpoint = (nodeName: string, ipOverride?: string): ClusterEndpoint => ({
  nodeId: `node/${nodeName}`,
  nodeName,
  host: `https://${nodeName}.local:8006`,
  ip: '10.0.0.1',
  ipOverride,
  online: true,
  lastSeen: '',
});

describe('nodeModalModel', () => {
  it('prefers PVE 9 guest-agent privileges before legacy VM.Monitor', () => {
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.GuestAgent.Audit');
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.GuestAgent.FileRead');
    expect(PVE_MANUAL_PERMISSION_COMMAND).toContain('VM.Monitor');

    const guestBranch = PVE_MANUAL_PERMISSION_COMMAND.indexOf(
      'if [ "$HAS_GUEST_AGENT_AUDIT" = true ]; then',
    );
    const monitorBranch = PVE_MANUAL_PERMISSION_COMMAND.indexOf(
      'pveum role add PulseTmpVMMonitor -privs VM.Monitor',
    );

    expect(guestBranch).toBeGreaterThanOrEqual(0);
    expect(monitorBranch).toBeGreaterThanOrEqual(0);
    expect(guestBranch).toBeLessThan(monitorBranch);
  });
});

describe('buildClusterEndpointOverridesPayload', () => {
  it('includes only members whose value changed from the saved override', () => {
    const endpoints = [endpoint('pve1'), endpoint('pve2', '10.0.0.2')];
    const payload = buildClusterEndpointOverridesPayload(endpoints, {
      pve1: '10.0.0.11',
      pve2: '10.0.0.2',
    });
    expect(payload).toEqual([{ nodeName: 'pve1', ipOverride: '10.0.0.11' }]);
  });

  it('sends an empty value to clear a saved override', () => {
    const endpoints = [endpoint('pve2', '10.0.0.2')];
    const payload = buildClusterEndpointOverridesPayload(endpoints, { pve2: '' });
    expect(payload).toEqual([{ nodeName: 'pve2', ipOverride: '' }]);
  });

  it('returns undefined when nothing changed or there are no endpoints', () => {
    expect(buildClusterEndpointOverridesPayload([endpoint('pve1')], { pve1: '' })).toBeUndefined();
    expect(
      buildClusterEndpointOverridesPayload([endpoint('pve1')], { pve1: '   ' }),
    ).toBeUndefined();
    expect(buildClusterEndpointOverridesPayload([], { pve1: '10.0.0.1' })).toBeUndefined();
    expect(buildClusterEndpointOverridesPayload(undefined, {})).toBeUndefined();
  });

  it('ignores form entries for members no longer in the endpoint list', () => {
    const payload = buildClusterEndpointOverridesPayload([endpoint('pve1')], {
      removed: '10.0.0.9',
    });
    expect(payload).toBeUndefined();
  });
});

describe('applyClusterEndpointOverridesLocally', () => {
  it('patches named members and leaves the rest untouched', () => {
    const endpoints = [endpoint('pve1'), endpoint('pve2', '10.0.0.2')];
    const result = applyClusterEndpointOverridesLocally(endpoints, [
      { nodeName: 'pve1', ipOverride: '10.0.0.11' },
    ]);
    expect(result?.[0].ipOverride).toBe('10.0.0.11');
    expect(result?.[1]).toBe(endpoints[1]);
  });

  it('clears an override when the payload value is empty', () => {
    const result = applyClusterEndpointOverridesLocally(
      [endpoint('pve2', '10.0.0.2')],
      [{ nodeName: 'pve2', ipOverride: '' }],
    );
    expect(result?.[0].ipOverride).toBeUndefined();
  });

  it('passes endpoints through when there is nothing to apply', () => {
    const endpoints = [endpoint('pve1')];
    expect(applyClusterEndpointOverridesLocally(endpoints, undefined)).toBe(endpoints);
    expect(applyClusterEndpointOverridesLocally(undefined, [])).toBeUndefined();
  });
});
