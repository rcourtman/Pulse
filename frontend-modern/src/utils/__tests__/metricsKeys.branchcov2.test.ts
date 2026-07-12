/**
 * Branch-coverage tests for the private `toMetricResourceKind` helper.
 *
 * `toMetricResourceKind` is not exported, so we exercise every branch
 * indirectly through the public `buildMetricKeyForUnifiedResource`, which
 * calls it and prefixes its result onto `{kind}:{id}`. Asserting the full
 * built key gives us a concrete observable for each switch arm.
 *
 * The function has two switch statements:
 *   1. Over `resource.metricsTarget?.resourceType` (skipped when
 *      `metricsTarget` is absent OR its `resourceType` matches no case).
 *   2. Over `resource.type`, with a `default -> 'agent'` fallback.
 *
 * The sibling test (`metricsKeys.test.ts`) only covers:
 *   - first switch:  `docker-host`
 *   - second switch: `k8s-cluster` (via an unmatched `'k8s'` target), `default`
 * This file targets every other arm.
 */
import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { buildMetricKeyForUnifiedResource } from '@/utils/metricsKeys';

/**
 * Build a minimal resource shape that satisfies the
 * `Pick<Resource, 'id' | 'type' | 'metricsTarget'>` contract used by
 * `buildMetricKeyForUnifiedResource`. Cast through `unknown` because we
 * intentionally populate only the three relevant fields.
 */
function makeResource(
  input: Pick<Resource, 'id' | 'type' | 'metricsTarget'>,
): Pick<Resource, 'id' | 'type' | 'metricsTarget'> {
  return input;
}

describe('metricsKeys.toMetricResourceKind — branch coverage (branchcov2)', () => {
  describe('first switch — metricsTarget.resourceType arms', () => {
    it("maps metricsTarget 'vm' onto the 'vm' kind", () => {
      // First switch case: `case 'vm': return 'vm'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'fallback-id-ignored',
            type: 'agent',
            metricsTarget: { resourceType: 'vm', resourceId: 'vm-100' },
          }),
        ),
      ).toBe('vm:vm-100');
    });

    it("maps metricsTarget 'system-container' onto the 'container' kind", () => {
      // First switch case: `case 'system-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'vm',
            metricsTarget: { resourceType: 'system-container', resourceId: 'lxc-200' },
          }),
        ),
      ).toBe('container:lxc-200');
    });

    it("maps metricsTarget 'oci-container' onto the 'container' kind", () => {
      // First switch case: `case 'oci-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'vm',
            metricsTarget: { resourceType: 'oci-container', resourceId: 'oci-300' },
          }),
        ),
      ).toBe('container:oci-300');
    });

    it("maps metricsTarget 'app-container' onto the 'container' kind", () => {
      // First switch case: `case 'app-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'vm',
            metricsTarget: { resourceType: 'app-container', resourceId: 'app-ctr-1' },
          }),
        ),
      ).toBe('container:app-ctr-1');
    });

    it("maps metricsTarget 'k8s-cluster' onto the 'k8s' kind", () => {
      // First switch case: `case 'k8s-cluster': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'agent',
            metricsTarget: { resourceType: 'k8s-cluster', resourceId: 'cluster-1' },
          }),
        ),
      ).toBe('k8s:cluster-1');
    });

    it("maps metricsTarget 'k8s-node' onto the 'k8s' kind", () => {
      // First switch case: `case 'k8s-node': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'agent',
            metricsTarget: { resourceType: 'k8s-node', resourceId: 'node-7' },
          }),
        ),
      ).toBe('k8s:node-7');
    });

    it("maps metricsTarget 'pod' onto the 'k8s' kind", () => {
      // First switch case: `case 'pod': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'agent',
            metricsTarget: { resourceType: 'pod', resourceId: 'pod-abcd' },
          }),
        ),
      ).toBe('k8s:pod-abcd');
    });

    it("maps metricsTarget 'agent' onto the 'agent' kind", () => {
      // First switch case: `case 'agent': return 'agent'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'unused',
            type: 'vm',
            metricsTarget: { resourceType: 'agent', resourceId: 'agent-9' },
          }),
        ),
      ).toBe('agent:agent-9');
    });
  });

  describe('first switch — fall-through to second switch', () => {
    it('falls through when metricsTarget is absent (optional-chain undefined arm)', () => {
      // `resource.metricsTarget?.resourceType` evaluates to undefined, so
      // no first-switch case matches and the second switch runs.
      // `type: 'vm'` hits the `case 'vm'` second-switch arm.
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'vm-200',
            type: 'vm',
          }),
        ),
      ).toBe('vm:vm-200');
    });

    it('falls through when metricsTarget.resourceType is a recognised-but-unhandled value', () => {
      // 'k8s-deployment' is part of MetricsHistoryTargetResourceType but has
      // NO case in the first switch, so execution falls through to switch #2.
      // `type: 'docker-host'` then hits `case 'docker-host'` in switch #2.
      // Note: buildMetricKeyForUnifiedResource still prefers the metricsTarget
      // resourceId, so the suffix is the target id, not resource.id.
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'host-1',
            type: 'docker-host',
            metricsTarget: { resourceType: 'k8s-deployment', resourceId: 'deploy-x' },
          }),
        ),
      ).toBe('dockerHost:deploy-x');
    });
  });

  describe('second switch — resource.type arms', () => {
    it("maps type 'docker-host' onto the 'dockerHost' kind", () => {
      // Second switch case: `case 'docker-host': return 'dockerHost'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'host-2',
            type: 'docker-host',
          }),
        ),
      ).toBe('dockerHost:host-2');
    });

    it("maps type 'k8s-cluster' onto the 'k8s' kind", () => {
      // Second switch case: `case 'k8s-cluster': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'cluster-2',
            type: 'k8s-cluster',
          }),
        ),
      ).toBe('k8s:cluster-2');
    });

    it("maps type 'k8s-node' onto the 'k8s' kind", () => {
      // Second switch case: `case 'k8s-node': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'node-2',
            type: 'k8s-node',
          }),
        ),
      ).toBe('k8s:node-2');
    });

    it("maps type 'pod' onto the 'k8s' kind", () => {
      // Second switch case: `case 'pod': return 'k8s'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'pod-2',
            type: 'pod',
          }),
        ),
      ).toBe('k8s:pod-2');
    });

    it("maps type 'system-container' onto the 'container' kind", () => {
      // Second switch case: `case 'system-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'ct-300',
            type: 'system-container',
          }),
        ),
      ).toBe('container:ct-300');
    });

    it("maps type 'oci-container' onto the 'container' kind", () => {
      // Second switch case: `case 'oci-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'oci-400',
            type: 'oci-container',
          }),
        ),
      ).toBe('container:oci-400');
    });

    it("maps type 'app-container' onto the 'container' kind", () => {
      // Second switch case: `case 'app-container': return 'container'`
      expect(
        buildMetricKeyForUnifiedResource(
          makeResource({
            id: 'app-ctr-2',
            type: 'app-container',
          }),
        ),
      ).toBe('container:app-ctr-2');
    });
  });

  describe('second switch — default fallback arm', () => {
    it("defaults an unhandled resource type onto the 'agent' kind", () => {
      // `type: 'k8s-deployment'` is a valid ResourceType but has no case in
      // switch #2, so the `default` arm returns 'agent'. Cast through
      // `unknown` because not every literal ResourceType is individually
      // modelled in this fixture.
      const resource = makeResource({
        id: 'deploy-1',
        type: 'k8s-deployment' as unknown as Resource['type'],
      });
      expect(buildMetricKeyForUnifiedResource(resource)).toBe('agent:deploy-1');
    });

    it("defaults a totally foreign resource type onto the 'agent' kind", () => {
      // Defensive: a never-modelled string literal still hits `default`.
      const resource = makeResource({
        id: 'weird-1',
        type: 'something-not-in-the-union' as unknown as Resource['type'],
      });
      expect(buildMetricKeyForUnifiedResource(resource)).toBe('agent:weird-1');
    });
  });
});
