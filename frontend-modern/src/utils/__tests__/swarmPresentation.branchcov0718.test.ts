import { describe, expect, it } from 'vitest';
import {
  // Vocabulary constants — the sibling test never imports these directly;
  // each `export const` is its own coverage statement, so pinning them
  // guards against silent renames and exercises the per-symbol getter
  // coverage reports flag as "get".
  SWARM_DRAWER_TITLE,
  SWARM_DRAWER_SEARCH_PLACEHOLDER,
  SWARM_DRAWER_NO_CLUSTER_LABEL,
  SWARM_DRAWER_CLUSTER_PREFIX,
  SWARM_DRAWER_CLUSTER_ID_PREFIX,
  SWARM_DRAWER_ROLE_PREFIX,
  SWARM_DRAWER_STATE_PREFIX,
  SWARM_DRAWER_CONTROL_PREFIX,
  SWARM_DRAWER_CONTROL_AVAILABLE_LABEL,
  SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL,
  SWARM_DRAWER_COLUMN_SERVICE_LABEL,
  SWARM_DRAWER_COLUMN_STACK_LABEL,
  SWARM_DRAWER_COLUMN_IMAGE_LABEL,
  SWARM_DRAWER_COLUMN_MODE_LABEL,
  SWARM_DRAWER_COLUMN_DESIRED_LABEL,
  SWARM_DRAWER_COLUMN_RUNNING_LABEL,
  SWARM_DRAWER_COLUMN_UPDATE_LABEL,
  SWARM_DRAWER_COLUMN_PORTS_LABEL,
  formatSwarmClusterId,
  formatSwarmClusterSummary,
  formatSwarmControlLabel,
  formatSwarmRoleLabel,
  formatSwarmStateLabel,
  getSwarmDrawerPresentation,
} from '../swarmPresentation';

// Residual branch-coverage probes for the swarm-presentation module.
// The sibling test (swarmPresentation.test.ts) already exercises:
//   - getSwarmDrawerPresentation() via a single bulk toEqual,
//   - the happy trim-truthy arm + the empty-string falsy arm of every
//     formatSwarm* helper,
//   - the boolean true/false arms and the null arm of
//     formatSwarmControlLabel,
//   - both arms of getSwarmServicesEmptyState and the fixed return of
//     getSwarmServicesLoadingState.
// This file targets the residual:
//   (a) every exported SWARM_DRAWER_* constant imported and pinned, so the
//       per-symbol getter the coverage tool reports is exercised for each,
//   (b) every field of getSwarmDrawerPresentation() read individually (the
//       bulk toEqual in the sibling test reads the object shape but does
//       not register a dedicated property-get coverage hit per field),
//   (c) the null / undefined / whitespace-only / surrounding-whitespace
//       input variants of every formatSwarm* helper — these all funnel
//       through the `|| ''` coalesce + `.trim()` pipeline but are never
//       exercised by the sibling test,
//   (d) the non-boolean primitive arm of formatSwarmControlLabel's
//       typeof guard (sibling test only passes null, which is one
//       specific non-boolean; the typeof check also rejects undefined,
//       strings, and numbers).

describe('swarmPresentation.branchcov0718', () => {
  describe('exported vocabulary constants', () => {
    // Each constant below is its own coverage statement; importing and
    // asserting the literal pins the canonical UI copy and protects
    // against silent renames.
    it('exposes the drawer title and search placeholder', () => {
      expect(SWARM_DRAWER_TITLE).toBe('Swarm');
      expect(SWARM_DRAWER_SEARCH_PLACEHOLDER).toBe('Search services...');
    });

    it('exposes the no-cluster and cluster-prefix vocabulary', () => {
      expect(SWARM_DRAWER_NO_CLUSTER_LABEL).toBe('No Swarm cluster detected');
      expect(SWARM_DRAWER_CLUSTER_PREFIX).toBe('Cluster:');
      expect(SWARM_DRAWER_CLUSTER_ID_PREFIX).toBe('Cluster ID:');
    });

    it('exposes the role / state / control prefix vocabulary', () => {
      expect(SWARM_DRAWER_ROLE_PREFIX).toBe('Role:');
      expect(SWARM_DRAWER_STATE_PREFIX).toBe('State:');
      expect(SWARM_DRAWER_CONTROL_PREFIX).toBe('Control:');
    });

    it('exposes the control availability status vocabulary', () => {
      expect(SWARM_DRAWER_CONTROL_AVAILABLE_LABEL).toBe('available');
      expect(SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL).toBe('unavailable');
    });

    it('exposes every service-table column label', () => {
      expect(SWARM_DRAWER_COLUMN_SERVICE_LABEL).toBe('Service');
      expect(SWARM_DRAWER_COLUMN_STACK_LABEL).toBe('Stack');
      expect(SWARM_DRAWER_COLUMN_IMAGE_LABEL).toBe('Image');
      expect(SWARM_DRAWER_COLUMN_MODE_LABEL).toBe('Mode');
      expect(SWARM_DRAWER_COLUMN_DESIRED_LABEL).toBe('Desired');
      expect(SWARM_DRAWER_COLUMN_RUNNING_LABEL).toBe('Running');
      expect(SWARM_DRAWER_COLUMN_UPDATE_LABEL).toBe('Update');
      expect(SWARM_DRAWER_COLUMN_PORTS_LABEL).toBe('Ports');
    });
  });

  describe('getSwarmDrawerPresentation — per-field property gets', () => {
    // The sibling test asserts the whole shape via toEqual; reading each
    // field individually registers a dedicated property-get coverage hit
    // and guards against the wrapper drifting away from any single
    // underlying constant.
    const presentation = getSwarmDrawerPresentation();

    it('wires title and searchPlaceholder fields', () => {
      expect(presentation.title).toBe(SWARM_DRAWER_TITLE);
      expect(presentation.title).toBe('Swarm');
      expect(presentation.searchPlaceholder).toBe(SWARM_DRAWER_SEARCH_PLACEHOLDER);
      expect(presentation.searchPlaceholder).toBe('Search services...');
    });

    it('wires noClusterLabel and the cluster/cluster-id prefixes', () => {
      expect(presentation.noClusterLabel).toBe(SWARM_DRAWER_NO_CLUSTER_LABEL);
      expect(presentation.noClusterLabel).toBe('No Swarm cluster detected');
      expect(presentation.clusterPrefix).toBe(SWARM_DRAWER_CLUSTER_PREFIX);
      expect(presentation.clusterPrefix).toBe('Cluster:');
      expect(presentation.clusterIdPrefix).toBe(SWARM_DRAWER_CLUSTER_ID_PREFIX);
      expect(presentation.clusterIdPrefix).toBe('Cluster ID:');
    });

    it('wires the role / state / control prefixes', () => {
      expect(presentation.rolePrefix).toBe(SWARM_DRAWER_ROLE_PREFIX);
      expect(presentation.rolePrefix).toBe('Role:');
      expect(presentation.statePrefix).toBe(SWARM_DRAWER_STATE_PREFIX);
      expect(presentation.statePrefix).toBe('State:');
      expect(presentation.controlPrefix).toBe(SWARM_DRAWER_CONTROL_PREFIX);
      expect(presentation.controlPrefix).toBe('Control:');
    });

    it('wires the control availability status labels', () => {
      expect(presentation.controlAvailableLabel).toBe(SWARM_DRAWER_CONTROL_AVAILABLE_LABEL);
      expect(presentation.controlAvailableLabel).toBe('available');
      expect(presentation.controlUnavailableLabel).toBe(SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL);
      expect(presentation.controlUnavailableLabel).toBe('unavailable');
    });

    it('wires every service-table column label field', () => {
      expect(presentation.serviceColumnLabel).toBe(SWARM_DRAWER_COLUMN_SERVICE_LABEL);
      expect(presentation.serviceColumnLabel).toBe('Service');
      expect(presentation.stackColumnLabel).toBe(SWARM_DRAWER_COLUMN_STACK_LABEL);
      expect(presentation.stackColumnLabel).toBe('Stack');
      expect(presentation.imageColumnLabel).toBe(SWARM_DRAWER_COLUMN_IMAGE_LABEL);
      expect(presentation.imageColumnLabel).toBe('Image');
      expect(presentation.modeColumnLabel).toBe(SWARM_DRAWER_COLUMN_MODE_LABEL);
      expect(presentation.modeColumnLabel).toBe('Mode');
      expect(presentation.desiredColumnLabel).toBe(SWARM_DRAWER_COLUMN_DESIRED_LABEL);
      expect(presentation.desiredColumnLabel).toBe('Desired');
      expect(presentation.runningColumnLabel).toBe(SWARM_DRAWER_COLUMN_RUNNING_LABEL);
      expect(presentation.runningColumnLabel).toBe('Running');
      expect(presentation.updateColumnLabel).toBe(SWARM_DRAWER_COLUMN_UPDATE_LABEL);
      expect(presentation.updateColumnLabel).toBe('Update');
      expect(presentation.portsColumnLabel).toBe(SWARM_DRAWER_COLUMN_PORTS_LABEL);
      expect(presentation.portsColumnLabel).toBe('Ports');
    });
  });

  describe('formatSwarmClusterSummary — coalesce + trim edge variants', () => {
    // Sibling test passes 'Prod' (truthy post-trim) and '' (falsy post-trim).
    // Residual: the `clusterName || ''` coalesce arm (null/undefined inputs),
    // the whitespace-only post-trim falsy arm, and the surrounding-whitespace
    // post-trim truthy arm.
    it('returns the no-cluster label for null/undefined', () => {
      expect(formatSwarmClusterSummary(null)).toBe(SWARM_DRAWER_NO_CLUSTER_LABEL);
      expect(formatSwarmClusterSummary(undefined)).toBe(SWARM_DRAWER_NO_CLUSTER_LABEL);
    });

    it('returns the no-cluster label for whitespace-only input', () => {
      expect(formatSwarmClusterSummary('   ')).toBe(SWARM_DRAWER_NO_CLUSTER_LABEL);
      expect(formatSwarmClusterSummary('\t\n')).toBe(SWARM_DRAWER_NO_CLUSTER_LABEL);
    });

    it('trims surrounding whitespace before joining with the prefix', () => {
      expect(formatSwarmClusterSummary('  Prod  ')).toBe('Cluster: Prod');
      expect(formatSwarmClusterSummary('\tStaging\n')).toBe('Cluster: Staging');
    });
  });

  describe('formatSwarmClusterId — coalesce + trim edge variants', () => {
    it('returns empty string for null/undefined', () => {
      expect(formatSwarmClusterId(null)).toBe('');
      expect(formatSwarmClusterId(undefined)).toBe('');
    });

    it('returns empty string for whitespace-only input', () => {
      expect(formatSwarmClusterId('   ')).toBe('');
      expect(formatSwarmClusterId('\t\n')).toBe('');
    });

    it('trims surrounding whitespace before joining with the prefix', () => {
      expect(formatSwarmClusterId('  abc123  ')).toBe('Cluster ID: abc123');
      expect(formatSwarmClusterId('\tdef456\n')).toBe('Cluster ID: def456');
    });
  });

  describe('formatSwarmRoleLabel — coalesce + trim edge variants', () => {
    it('returns empty string for null/undefined', () => {
      expect(formatSwarmRoleLabel(null)).toBe('');
      expect(formatSwarmRoleLabel(undefined)).toBe('');
    });

    it('returns empty string for whitespace-only input', () => {
      expect(formatSwarmRoleLabel('   ')).toBe('');
      expect(formatSwarmRoleLabel('\t\n')).toBe('');
    });

    it('trims surrounding whitespace before joining with the prefix', () => {
      expect(formatSwarmRoleLabel('  manager  ')).toBe('Role: manager');
      expect(formatSwarmRoleLabel('\tworker\n')).toBe('Role: worker');
    });
  });

  describe('formatSwarmStateLabel — coalesce + trim edge variants', () => {
    it('returns empty string for null/undefined', () => {
      expect(formatSwarmStateLabel(null)).toBe('');
      expect(formatSwarmStateLabel(undefined)).toBe('');
    });

    it('returns empty string for whitespace-only input', () => {
      expect(formatSwarmStateLabel('   ')).toBe('');
      expect(formatSwarmStateLabel('\t\n')).toBe('');
    });

    it('trims surrounding whitespace before joining with the prefix', () => {
      expect(formatSwarmStateLabel('  active  ')).toBe('State: active');
      expect(formatSwarmStateLabel('\tpaused\n')).toBe('State: paused');
    });
  });

  describe('formatSwarmControlLabel — typeof guard residual arms', () => {
    // Sibling test passes true/false (the boolean arms of the ternary) and
    // null (one specific non-boolean). The typeof guard also rejects
    // undefined, strings, and numbers — exercise each of those to take
    // every arm the guard offers.
    it('returns empty string for undefined', () => {
      expect(formatSwarmControlLabel(undefined)).toBe('');
    });

    it('returns empty string for non-boolean primitive inputs', () => {
      const stringInput = 'true' as unknown as Parameters<typeof formatSwarmControlLabel>[0];
      const numberInput = 1 as unknown as Parameters<typeof formatSwarmControlLabel>[0];
      const objectInput = {} as unknown as Parameters<typeof formatSwarmControlLabel>[0];
      expect(formatSwarmControlLabel(stringInput)).toBe('');
      expect(formatSwarmControlLabel(numberInput)).toBe('');
      expect(formatSwarmControlLabel(objectInput)).toBe('');
    });

    it('still returns the canonical copy for genuine boolean inputs', () => {
      // Guard against an over-eager typeof change: the boolean path must
      // still interpolate the canonical available/unavailable labels.
      expect(formatSwarmControlLabel(true)).toBe(
        `${SWARM_DRAWER_CONTROL_PREFIX} ${SWARM_DRAWER_CONTROL_AVAILABLE_LABEL}`,
      );
      expect(formatSwarmControlLabel(false)).toBe(
        `${SWARM_DRAWER_CONTROL_PREFIX} ${SWARM_DRAWER_CONTROL_UNAVAILABLE_LABEL}`,
      );
    });
  });
});
