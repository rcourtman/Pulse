import { describe, expect, it } from 'vitest';
import {
  formatSwarmClusterId,
  formatSwarmClusterSummary,
  formatSwarmControlLabel,
  formatSwarmRoleLabel,
  formatSwarmStateLabel,
  getSwarmDrawerPresentation,
  getSwarmServicesEmptyState,
  getSwarmServicesLoadingState,
} from '../swarmPresentation';

describe('swarmPresentation', () => {
  it('returns canonical swarm drawer presentation', () => {
    expect(getSwarmDrawerPresentation()).toEqual({
      title: 'Swarm',
      searchPlaceholder: 'Search services...',
      noClusterLabel: 'No Swarm cluster detected',
      clusterPrefix: 'Cluster:',
      clusterIdPrefix: 'Cluster ID:',
      rolePrefix: 'Role:',
      statePrefix: 'State:',
      controlPrefix: 'Control:',
      controlAvailableLabel: 'available',
      controlUnavailableLabel: 'unavailable',
      serviceColumnLabel: 'Service',
      stackColumnLabel: 'Stack',
      imageColumnLabel: 'Image',
      modeColumnLabel: 'Mode',
      desiredColumnLabel: 'Desired',
      runningColumnLabel: 'Running',
      updateColumnLabel: 'Update',
      portsColumnLabel: 'Ports',
    });
  });

  it('formats canonical swarm drawer labels', () => {
    expect(formatSwarmClusterSummary('Prod')).toBe('Cluster: Prod');
    expect(formatSwarmClusterSummary('')).toBe('No Swarm cluster detected');
    expect(formatSwarmClusterId('abc123')).toBe('Cluster ID: abc123');
    expect(formatSwarmClusterId('')).toBe('');
    expect(formatSwarmRoleLabel('manager')).toBe('Role: manager');
    expect(formatSwarmRoleLabel('')).toBe('');
    expect(formatSwarmStateLabel('active')).toBe('State: active');
    expect(formatSwarmStateLabel('')).toBe('');
    expect(formatSwarmControlLabel(true)).toBe('Control: available');
    expect(formatSwarmControlLabel(false)).toBe('Control: unavailable');
    expect(formatSwarmControlLabel(null)).toBe('');
  });

  it('returns canonical swarm empty-state copy', () => {
    expect(getSwarmServicesEmptyState(true)).toEqual({
      title: 'No services match your filters',
      description: 'Try clearing the search.',
    });
    expect(getSwarmServicesEmptyState(false)).toEqual({
      title: 'No Swarm services found',
      description:
        'Enable Swarm service collection in the container runtime agent (includeServices) and wait for the next report.',
    });
    expect(getSwarmServicesLoadingState()).toEqual({
      text: 'Loading Swarm services...',
    });
  });
});
