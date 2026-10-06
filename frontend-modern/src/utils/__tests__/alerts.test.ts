import { describe, expect, it } from 'vitest';
import {
  getAlertsForResource,
  getAlertsForUnifiedResource,
  getAlertStyles,
  getUnifiedResourceAlertStyles,
} from '@/utils/alerts';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';

describe('getAlertStyles', () => {
  const createAlert = (overrides: Partial<Alert> = {}): Alert => ({
    id: 'alert-1',
    type: 'warning',
    level: 'warning',
    resourceId: 'resource-1',
    resourceName: 'Test Resource',
    node: 'node1',
    instance: 'qemu',
    message: 'Test alert',
    value: 80,
    threshold: 70,
    startTime: '2024-01-01T00:00:00Z',
    acknowledged: false,
    ...overrides,
  });

  const createActiveAlerts = (...alerts: Alert[]): Record<string, Alert> => {
    return alerts.reduce(
      (acc, alert) => {
        acc[alert.id] = alert;
        return acc;
      },
      {} as Record<string, Alert>,
    );
  };

  describe('when alerts are disabled', () => {
    it('returns no alert styles when alertsEnabled is false', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, false);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
      expect(result.rowClass).toBe('');
    });

    it('returns alert styles when alertsEnabled is undefined (defaults to enabled)', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, undefined);

      expect(result.hasAlert).toBe(true);
    });
  });

  describe('critical alerts', () => {
    it('returns critical styles for critical alert', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'critical' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(true);
      expect(result.alertCount).toBe(1);
      expect(result.severity).toBe('critical');
      expect(result.rowClass).toContain('bg-red-50');
      expect(result.indicatorClass).toContain('bg-red-500');
      expect(result.badgeClass).toContain('bg-red-100');
    });

    it('returns critical styles when both critical and warning alerts exist', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', level: 'critical' }),
        createAlert({ id: 'alert-2', level: 'warning' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.severity).toBe('critical');
    });
  });

  describe('warning alerts', () => {
    it('returns warning styles for warning alert', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'warning' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(true);
      expect(result.severity).toBe('warning');
      expect(result.rowClass).toContain('bg-yellow-50');
      expect(result.indicatorClass).toContain('bg-yellow-500');
      expect(result.badgeClass).toContain('bg-yellow-100');
    });
  });

  describe('informational alerts', () => {
    it('uses a distinct low-urgency blue resource treatment', () => {
      const alerts = createActiveAlerts(createAlert({ level: 'info' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(true);
      expect(result.severity).toBe('info');
      expect(result.rowClass).toContain('bg-blue-50');
      expect(result.indicatorClass).toContain('bg-blue-500');
      expect(result.badgeClass).toContain('bg-blue-100');
    });
  });

  it('matches alert styles through any canonical resource identity candidate', () => {
    const alerts = createActiveAlerts(
      createAlert({ resourceId: 'provider-source-id', level: 'critical' }),
    );

    expect(getAlertStyles(['canonical-id', 'provider-source-id'], alerts, true).severity).toBe(
      'critical',
    );
  });

  describe('alert counts', () => {
    it('counts multiple alerts for same resource', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', level: 'warning' }),
        createAlert({ id: 'alert-2', level: 'warning' }),
        createAlert({ id: 'alert-3', level: 'critical' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.alertCount).toBe(3);
    });

    it('counts unacknowledged alerts separately', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', acknowledged: false }),
        createAlert({ id: 'alert-2', acknowledged: true }),
        createAlert({ id: 'alert-3', acknowledged: false }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.unacknowledgedCount).toBe(2);
      expect(result.acknowledgedCount).toBe(1);
      expect(result.hasUnacknowledgedAlert).toBe(true);
    });

    it('detects acknowledged-only alerts', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', acknowledged: true }),
        createAlert({ id: 'alert-2', acknowledged: true }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAcknowledgedOnlyAlert).toBe(true);
      expect(result.hasUnacknowledgedAlert).toBe(false);
    });
  });

  describe('powered-off alerts', () => {
    it('detects powered-off alert type', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', type: 'powered-off' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(true);
      expect(result.hasNonPoweredOffAlert).toBe(false);
    });

    it('detects non-powered-off alert type', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', type: 'high-cpu' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(false);
      expect(result.hasNonPoweredOffAlert).toBe(true);
    });

    it('detects both powered-off and non-powered-off alerts', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', type: 'powered-off' }),
        createAlert({ id: 'alert-2', type: 'high-cpu' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasPoweredOffAlert).toBe(true);
      expect(result.hasNonPoweredOffAlert).toBe(true);
    });
  });

  describe('resource filtering', () => {
    it('only returns alerts for specified resource', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'alert-1', resourceId: 'resource-1', level: 'critical' }),
        createAlert({ id: 'alert-2', resourceId: 'resource-2', level: 'critical' }),
      );
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.alertCount).toBe(1);
      expect(result.severity).toBe('critical');
    });

    it('returns the matching alert records for canonical drawer presentation', () => {
      const alerts = createActiveAlerts(
        createAlert({ id: 'canonical', resourceId: 'resource-1', message: 'CPU is hot' }),
        createAlert({ id: 'legacy', resourceId: 'legacy-resource', message: 'Disk is full' }),
        createAlert({ id: 'other', resourceId: 'resource-2' }),
      );

      expect(
        getAlertsForResource(['resource-1', 'legacy-resource'], alerts, true).map(
          (alert) => alert.message,
        ),
      ).toEqual(['CPU is hot', 'Disk is full']);
      expect(getAlertsForResource(['resource-1'], alerts, false)).toEqual([]);
    });

    it('returns empty styles when no alerts for resource', () => {
      const alerts = createActiveAlerts(createAlert({ id: 'alert-1', resourceId: 'resource-2' }));
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
      expect(result.rowClass).toBe('');
    });
  });

  describe('empty alerts', () => {
    it('returns empty styles for empty alerts object', () => {
      const alerts = createActiveAlerts();
      const result = getAlertStyles('resource-1', alerts, true);

      expect(result.hasAlert).toBe(false);
      expect(result.alertCount).toBe(0);
      expect(result.severity).toBeNull();
    });
  });
});

describe('getAlertsForUnifiedResource', () => {
  // Identity shapes below are copied from a mock-mode /api/state payload.
  const alert = (id: string, resourceId: string, overrides: Partial<Alert> = {}): Alert => ({
    id,
    type: 'cpu',
    level: 'warning',
    resourceId,
    resourceName: resourceId,
    node: '',
    instance: '',
    message: id,
    value: 0,
    threshold: 0,
    startTime: '2026-10-06T10:00:00Z',
    acknowledged: false,
    ...overrides,
  });
  const byId = (...alerts: Alert[]): Record<string, Alert> =>
    Object.fromEntries(alerts.map((entry) => [entry.id, entry]));
  const resource = (overrides: Partial<Resource>): Resource =>
    ({
      id: 'resource-1',
      type: 'agent',
      name: 'resource-1',
      displayName: 'resource-1',
      platformId: 'resource-1',
      platformType: 'agent',
      sourceType: 'agent',
      status: 'online',
      lastSeen: 0,
      ...overrides,
    }) as Resource;
  const ids = (alerts: Alert[]) => alerts.map((entry) => entry.id);

  it('matches a standalone agent and the components nested under its agent key', () => {
    const host = resource({
      id: 'agent-acdfdee2953587fb',
      agent: { agentId: 'host-linux-1' },
      metricsTarget: { resourceType: 'agent', resourceId: 'host-linux-1' },
      canonicalIdentity: {
        displayName: 'Apollo-114',
        primaryId: 'agent:host-linux-1',
        aliases: ['agent:host-linux-1', 'host-linux-1', 'apollo-114'],
      },
    } as Partial<Resource>);
    const active = byId(
      alert('cpu', 'agent:host-linux-1'),
      alert('disk', 'agent:host-linux-1/disk:data', { level: 'critical' }),
      alert('raid', 'agent:host-linux-1/raid:md0'),
      alert('sibling', 'agent:host-linux-10'),
      alert('sibling-disk', 'agent:host-linux-10/disk:data'),
      alert('hostname', 'apollo-114'),
    );

    expect(ids(getAlertsForUnifiedResource(host, active, true))).toEqual(['disk', 'cpu', 'raid']);
    expect(getAlertsForUnifiedResource(host, active, false)).toEqual([]);
  });

  it('matches a Proxmox node agent by node id and by its linked agent key', () => {
    const node = resource({
      id: 'agent-85d79ff9b7a2cc0e',
      platformType: 'proxmox-pve',
      agent: { agentId: 'host-node-mock-cluster-1-pve2' },
      proxmox: { sourceId: 'mock-cluster-1-pve2', nodeName: 'pve2' },
      canonicalIdentity: {
        displayName: 'West Production B',
        primaryId: 'node:mock-cluster-1-pve2',
        aliases: ['node:mock-cluster-1-pve2', 'pve2', 'agent:host-node-mock-cluster-1-pve2'],
      },
    } as Partial<Resource>);
    const active = byId(
      alert('node-memory', 'mock-cluster-1-pve2'),
      alert('agent-temp', 'agent:host-node-mock-cluster-1-pve2'),
      alert('pool', 'mock-cluster-1-pve2-local-zfs/zfs-pool:local-zfs', { node: 'pve2' }),
      alert('bare-node-name', 'pve2'),
    );

    expect(ids(getAlertsForUnifiedResource(node, active, true))).toEqual([
      'node-memory',
      'agent-temp',
    ]);
  });

  it('matches a Docker host, its containers and services, and its reporting agent', () => {
    const dockerHost = resource({
      id: 'agent-aa22ff2b2bc3257d',
      type: 'docker-host',
      platformType: 'docker',
      docker: { hostSourceId: 'orion-2-mock' },
      metricsTarget: { resourceType: 'docker-host', resourceId: 'orion-2-mock' },
      canonicalIdentity: {
        displayName: 'Ops Services 01',
        primaryId: 'docker-host:orion-2-mock',
        aliases: ['docker-host:orion-2-mock', 'orion-2-mock', 'agent:agent-5d2100295642fccc'],
      },
    } as Partial<Resource>);
    const active = byId(
      alert('offline', 'docker:orion-2-mock', { level: 'critical' }),
      alert('service', 'docker:orion-2-mock/service/svc-backend-1'),
      alert('agent-cpu', 'agent:agent-5d2100295642fccc'),
      alert('other-host', 'docker:orion-20-mock/abc'),
    );

    expect(ids(getAlertsForUnifiedResource(dockerHost, active, true))).toEqual([
      'offline',
      'service',
      'agent-cpu',
    ]);
  });

  it('matches PBS and PMG instances by their metrics target', () => {
    const pbs = resource({
      id: 'pbs-4fcc01f0e6db7b83',
      type: 'pbs',
      metricsTarget: { resourceType: 'agent', resourceId: 'pbs-pbs-docker' },
      canonicalIdentity: { displayName: 'pbs-docker', primaryId: 'agent:pbs-pbs-docker' },
    } as Partial<Resource>);
    const pmg = resource({
      id: 'pmg-abe41cf2c6a8ff47',
      type: 'pmg',
      metricsTarget: { resourceType: 'agent', resourceId: 'pmg-main' },
      canonicalIdentity: { displayName: 'mail-gateway-eu', primaryId: 'agent:pmg-main' },
    } as Partial<Resource>);
    const active = byId(
      alert('pbs-memory', 'pbs-pbs-docker'),
      alert('pmg-queue', 'pmg-main'),
      alert('pbs-datastore', 'pbs-pbs-docker-arr-secondary'),
    );

    expect(ids(getAlertsForUnifiedResource(pbs, active, true))).toEqual(['pbs-memory']);
    expect(ids(getAlertsForUnifiedResource(pmg, active, true))).toEqual(['pmg-queue']);
  });

  it('matches provider incidents on the unified id without borrowing a parent agent key', () => {
    const pod = resource({
      id: 'pod-9bd9679901d26654',
      type: 'pod',
      canonicalIdentity: {
        displayName: 'checkout-api',
        primaryId: 'pod:k8s:k8s-production-1:pod:nginx',
        aliases: ['k8s-production-1', 'agent:k8s-production-1-agent'],
      },
    } as Partial<Resource>);
    const active = byId(
      alert('pod-incident', 'pod-9bd9679901d26654'),
      alert('cluster-agent', 'agent:k8s-production-1-agent'),
    );

    expect(ids(getAlertsForUnifiedResource(pod, active, true))).toEqual(['pod-incident']);
  });

  describe('getUnifiedResourceAlertStyles', () => {
    // Mock "Ops Services 01": the Docker alerts carry the hostname as their
    // node, which is neither the row's id nor its display name.
    const dockerHost = resource({
      id: 'agent-aa22ff2b2bc3257d',
      type: 'docker-host',
      name: 'Ops Services 01',
      displayName: 'Ops Services 01',
      platformType: 'docker',
      docker: { hostSourceId: 'orion-2-mock', hostname: 'ops-services-01' },
      canonicalIdentity: {
        displayName: 'Ops Services 01',
        primaryId: 'docker-host:orion-2-mock',
        aliases: ['docker-host:orion-2-mock', 'orion-2-mock'],
      },
    } as Partial<Resource>);
    const serviceAlert = (id: string, overrides: Partial<Alert> = {}) =>
      alert(id, `docker:orion-2-mock/service/${id}`, {
        type: 'docker-service-health',
        node: 'ops-services-01',
        ...overrides,
      });

    it('tints a Docker host row for the service alerts its drawer lists', () => {
      const active = byId(serviceAlert('svc-1'), serviceAlert('svc-2'), serviceAlert('svc-3'));

      const styles = getUnifiedResourceAlertStyles(dockerHost, active, true);
      expect(styles.hasUnacknowledgedAlert).toBe(true);
      expect(styles.severity).toBe('warning');
      expect(styles.unacknowledgedCount).toBe(3);
      // The row's previous id + display-name match found none of them.
      expect(getAlertStyles(dockerHost.id, active, true, 'Ops Services 01').hasAlert).toBe(false);
    });

    it('tints a machine row for an alert on its agent key', () => {
      const machine = resource({
        id: 'agent-acdfdee2953587fb',
        name: 'Apollo-114',
        agent: { agentId: 'host-linux-1' },
      } as Partial<Resource>);
      const active = byId(
        alert('memory', 'agent:host-linux-1', { type: 'memory' }),
        alert('disk', 'agent:host-linux-1/disk:data', { level: 'critical' }),
      );

      const styles = getUnifiedResourceAlertStyles(machine, active, true);
      expect(styles.severity).toBe('critical');
      expect(styles.alertCount).toBe(2);
    });

    it('leaves the row untinted when every open alert is acknowledged', () => {
      const active = byId(serviceAlert('svc-1', { acknowledged: true, level: 'critical' }));

      const styles = getUnifiedResourceAlertStyles(dockerHost, active, true);
      expect(styles.hasUnacknowledgedAlert).toBe(false);
      expect(styles.hasAcknowledgedOnlyAlert).toBe(true);
      expect(styles.severity).toBeNull();
    });

    it('reports nothing while alert detection is off', () => {
      const styles = getUnifiedResourceAlertStyles(dockerHost, byId(serviceAlert('svc-1')), false);
      expect(styles.hasAlert).toBe(false);
      expect(styles.rowClass).toBe('');
    });
  });
});
