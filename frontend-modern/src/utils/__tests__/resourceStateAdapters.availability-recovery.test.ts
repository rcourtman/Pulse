import { describe, expect, it } from 'vitest';
import type { Resource, ResourceAvailabilityMeta } from '@/types/resource';
import { getAvailabilityProbePresentation } from '@/utils/availabilityProbePresentation';
import {
  mergeCanonicalResourceSnapshot,
  mergeCanonicalResourceDeltaSnapshot,
} from '../resourceStateAdapters';

// REST rows and the websocket baseline the store rebuilds from merge-patch
// deltas carry the availability summary as one whole check record whose
// failure, location and certificate fields are omitempty, so a field missing
// from a facet that is present means "no longer true", not "unchanged".
describe('resourceStateAdapters availability summary facet', () => {
  const failingHttpsCheck: ResourceAvailabilityMeta = {
    targetId: 'shop-https',
    name: 'Shop HTTPS',
    targetKind: 'service',
    address: 'shop.example.test',
    protocol: 'https',
    port: 443,
    path: '/health',
    probeOutcome: 'failed',
    transportOutcome: 'reachable',
    applicationOutcome: 'failed',
    applicationStatusCode: 503,
    applicationFailureCode: 'http_status',
    aggregateState: 'degraded',
    disagreement: true,
    expectedLocations: 2,
    reportingLocations: 2,
    enabled: true,
    available: false,
    lastChecked: '2026-10-07T12:00:00Z',
    lastSuccess: '2026-10-07T11:55:00Z',
    consecutiveFailures: 3,
    lastError: 'HTTP 503 Service Unavailable',
    failureThreshold: 3,
    pollIntervalSeconds: 30,
    timeoutMillis: 5000,
    correlationState: 'standalone',
  };
  const recoveredHttpsCheck: ResourceAvailabilityMeta = {
    targetId: 'shop-https',
    name: 'Shop HTTPS',
    targetKind: 'service',
    address: 'shop.example.test',
    protocol: 'https',
    port: 443,
    path: '/health',
    probeOutcome: 'reachable',
    transportOutcome: 'reachable',
    applicationOutcome: 'passed',
    applicationStatusCode: 200,
    aggregateState: 'healthy',
    expectedLocations: 2,
    reportingLocations: 2,
    enabled: true,
    available: true,
    lastChecked: '2026-10-07T12:01:00Z',
    lastSuccess: '2026-10-07T12:01:00Z',
    latencyMillis: 41,
    failureThreshold: 3,
    pollIntervalSeconds: 30,
    timeoutMillis: 5000,
    correlationState: 'standalone',
  };
  const endpointRow = (availability: ResourceAvailabilityMeta): Resource =>
    ({
      id: 'network-endpoint-shop',
      type: 'network-endpoint',
      name: 'Shop HTTPS',
      displayName: 'Shop HTTPS',
      platformId: 'shop-https',
      platformType: 'availability',
      sourceType: 'api',
      sources: ['availability'],
      status: availability.available ? 'online' : 'offline',
      lastSeen: Date.parse(availability.lastChecked ?? '2026-10-07T12:00:00Z'),
      availability,
    }) as Resource;

  it('drops a recovered check’s failure fields on snapshot and delta merges', () => {
    const [previous] = mergeCanonicalResourceSnapshot([endpointRow(failingHttpsCheck)], []);
    const incoming = endpointRow(recoveredHttpsCheck);

    const [snapshot] = mergeCanonicalResourceSnapshot([structuredClone(incoming)], [previous]);
    const [delta] = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(incoming)],
      [previous],
      new Set([incoming.id]),
      new Map([[incoming.id, ['status', 'lastSeen', 'availability']]]),
    );

    for (const merged of [snapshot, delta]) {
      expect(merged.availability).toEqual(recoveredHttpsCheck);
      expect((merged.platformData as Record<string, unknown> | undefined)?.availability).toEqual(
        recoveredHttpsCheck,
      );
      const presentation = getAvailabilityProbePresentation(
        merged,
        new Date('2026-10-07T12:01:10Z'),
      );
      expect(presentation?.detailLabel).not.toContain('failures');
      expect(presentation?.detailLabel).not.toContain('503');
    }
  });

  it('replaces a machine summary that switches to another attached check', () => {
    const passingTcpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-postgres',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 Postgres',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'tcp',
      port: 5432,
      probeOutcome: 'reachable',
      transportOutcome: 'reachable',
      aggregateState: 'healthy',
      expectedLocations: 2,
      reportingLocations: 2,
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      lastSuccess: '2026-10-07T12:00:00Z',
      latencyMillis: 3,
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    const failingIcmpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-uptime-ping',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 ping',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'icmp',
      probeOutcome: 'failed',
      transportOutcome: 'unreachable',
      enabled: true,
      available: false,
      lastChecked: '2026-10-07T12:01:00Z',
      consecutiveFailures: 3,
      lastError: 'ping timed out',
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    // The backend orders checks by target id and, while both pass, keeps the
    // first as the summary; a confirmed ping outage outranks it.
    const passingIcmpCheck: ResourceAvailabilityMeta = {
      targetId: 'db-1-uptime-ping',
      linkedResourceId: 'agent-db-1',
      name: 'db-1 ping',
      targetKind: 'machine',
      address: '192.0.2.40',
      protocol: 'icmp',
      probeOutcome: 'reachable',
      transportOutcome: 'reachable',
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      lastSuccess: '2026-10-07T12:00:00Z',
      latencyMillis: 1,
      failureThreshold: 3,
      pollIntervalSeconds: 30,
      timeoutMillis: 2000,
      correlationState: 'attached',
    };
    const machineRow = (checks: ResourceAvailabilityMeta[], summary: ResourceAvailabilityMeta) =>
      ({
        id: 'agent-db-1',
        type: 'agent',
        name: 'db-1',
        displayName: 'db-1',
        platformId: 'db-1',
        platformType: 'agent',
        sourceType: 'agent',
        sources: ['agent', 'availability'],
        status: 'online',
        lastSeen: Date.parse('2026-10-07T12:01:00Z'),
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
        availability: summary,
        availabilityChecks: checks,
      }) as Resource;

    const [previous] = mergeCanonicalResourceSnapshot(
      [machineRow([passingTcpCheck, passingIcmpCheck], passingTcpCheck)],
      [],
    );
    const incoming = machineRow([passingTcpCheck, failingIcmpCheck], failingIcmpCheck);

    const [snapshot] = mergeCanonicalResourceSnapshot([structuredClone(incoming)], [previous]);
    const [delta] = mergeCanonicalResourceDeltaSnapshot(
      [structuredClone(incoming)],
      [previous],
      new Set([incoming.id]),
      new Map([[incoming.id, ['availability']]]),
    );

    for (const merged of [snapshot, delta]) {
      expect(merged.availability).toEqual(failingIcmpCheck);
      expect((merged.platformData as Record<string, unknown> | undefined)?.availability).toEqual(
        failingIcmpCheck,
      );
      expect(merged.availabilityChecks).toEqual([passingTcpCheck, failingIcmpCheck]);
    }
  });

  it('drops the plural check mirror when the availability source is withdrawn', () => {
    const check: ResourceAvailabilityMeta = {
      targetId: 'db-1-icmp',
      linkedResourceId: 'agent-db-1',
      address: '192.0.2.40',
      protocol: 'icmp',
      enabled: true,
      available: true,
      lastChecked: '2026-10-07T12:00:00Z',
      correlationState: 'attached',
    };
    // REST hydration mirrors both facets into platformData.
    const restRow = {
      id: 'agent-db-1',
      type: 'agent',
      name: 'db-1',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent', 'availability'],
      status: 'online',
      lastSeen: Date.parse('2026-10-07T12:00:00Z'),
      availability: check,
      availabilityChecks: [check],
      platformData: {
        sources: ['agent', 'availability'],
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
        availability: check,
        availabilityChecks: [check],
      },
    } as unknown as Resource;
    // The check was deleted: the realtime row no longer lists the source.
    const realtimeRow = {
      id: 'agent-db-1',
      type: 'agent',
      name: 'db-1',
      platformType: 'agent',
      sourceType: 'agent',
      sources: ['agent'],
      status: 'online',
      lastSeen: Date.parse('2026-10-07T12:01:00Z'),
      platformData: {
        sources: ['agent'],
        agent: { agentId: 'agent-db-1', hostname: 'db-1' },
      },
    } as unknown as Resource;

    const [merged] = mergeCanonicalResourceSnapshot([realtimeRow], [restRow]);
    // Hydration merges the realtime projection over the REST cache once more.
    const [rehydrated] = mergeCanonicalResourceSnapshot([merged], [restRow]);

    for (const row of [merged, rehydrated]) {
      expect(row.availability).toBeUndefined();
      expect(row.availabilityChecks).toBeUndefined();
      expect((row.platformData as Record<string, unknown>).availability).toBeUndefined();
      expect((row.platformData as Record<string, unknown>).availabilityChecks).toBeUndefined();
    }
  });
});
