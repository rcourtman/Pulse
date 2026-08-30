import { describe, expect, it } from 'vitest';
import type { AvailabilityTarget } from '@/api/availabilityTargets';
import type { AvailabilityProbeSuggestion, DiscoverySummary } from '@/types/discovery';
import {
  buildAvailabilityTargetFromProposal,
  findAvailabilityProposalDuplicate,
  isAvailabilityProposalDismissed,
  reviewableAvailabilitySummaries,
} from '../availabilityProposalModel';

const proposal = (
  overrides: Partial<AvailabilityProbeSuggestion> = {},
): AvailabilityProbeSuggestion => ({
  protocol: 'http',
  address: 'Grafana.Local.',
  port: 3000,
  path: '/',
  service_name: 'Grafana',
  reason: 'service default: grafana',
  evidence_fingerprint: 'sha256:grafana',
  ...overrides,
});

const target = (overrides: Partial<AvailabilityTarget> = {}): AvailabilityTarget => ({
  id: 'existing',
  name: 'Existing Grafana check',
  targetKind: 'service',
  address: 'grafana.local',
  protocol: 'http',
  port: 3000,
  enabled: true,
  ...overrides,
});

describe('availabilityProposalModel', () => {
  it('builds an explicit active application contract attached to the canonical resource', () => {
    expect(
      buildAvailabilityTargetFromProposal({
        proposal: proposal(),
        canonicalResourceId: 'app-container:grafana',
        name: 'Customer dashboard',
        intervalSeconds: 300,
        probeAgentId: 'edge-agent',
      }),
    ).toMatchObject({
      name: 'Customer dashboard',
      targetKind: 'service',
      address: 'Grafana.Local.',
      protocol: 'http',
      port: 3000,
      path: '/',
      linkedResourceId: 'app-container:grafana',
      enabled: true,
      pollIntervalSeconds: 300,
      probeAgentId: 'edge-agent',
      http: {
        method: 'GET',
        authentication: { type: 'none' },
        expectedStatusMin: 200,
        expectedStatusMax: 399,
      },
    });
  });

  it('deduplicates equivalent endpoints before resource-level coverage warnings', () => {
    expect(
      findAvailabilityProposalDuplicate(proposal(), 'app-container:grafana', [
        target({ linkedResourceId: 'another-resource' }),
      ]),
    ).toMatchObject({ kind: 'endpoint', target: { id: 'existing' } });

    expect(
      findAvailabilityProposalDuplicate(proposal(), 'app-container:grafana', [
        target({ address: 'other.local', linkedResourceId: 'app-container:grafana' }),
      ]),
    ).toMatchObject({ kind: 'resource', target: { id: 'existing' } });
  });

  it('binds dismissal to the exact evidence and sorts reviewable suggestions first', () => {
    const dismissed: DiscoverySummary = {
      id: 'docker:host:grafana',
      resource_type: 'app-container',
      resource_id: 'grafana',
      target_id: 'host',
      hostname: 'grafana',
      service_type: 'grafana',
      service_name: 'Grafana',
      service_version: '',
      category: 'monitoring',
      confidence: 0.95,
      has_user_notes: false,
      updated_at: '2026-08-30T12:00:00Z',
      suggested_availability_probe: proposal(),
      dismissed_availability_probe_fingerprint: 'sha256:grafana',
    };
    const reviewable: DiscoverySummary = {
      ...dismissed,
      id: 'docker:host:redis',
      resource_id: 'redis',
      service_name: 'Redis',
      suggested_availability_probe: proposal({
        protocol: 'tcp',
        address: 'redis.local',
        port: 6379,
        service_name: 'Redis',
        evidence_fingerprint: 'sha256:redis',
      }),
      dismissed_availability_probe_fingerprint: 'sha256:old-redis',
    };

    expect(isAvailabilityProposalDismissed(dismissed)).toBe(true);
    expect(isAvailabilityProposalDismissed(reviewable)).toBe(false);
    expect(reviewableAvailabilitySummaries([dismissed, reviewable]).map((item) => item.id)).toEqual(
      [reviewable.id, dismissed.id],
    );
  });
});
