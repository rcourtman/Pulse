import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@solidjs/testing-library';
import type { AlertOverviewState } from '../useAlertOverviewState';
import type { AlertIncidentTimelineState } from '../useAlertIncidentTimelineState';
import { makeSystemAlert, SYSTEM_ALERT_TYPES } from '../__fixtures__/systemAlerts';

vi.mock('@solidjs/router', () => ({
  A: (props: any) => (
    <a href={props.href} title={props.title}>
      {props.children}
    </a>
  ),
}));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => null,
}));
vi.mock('../ResourceMonitoringPolicyAction', () => ({
  ResourceMonitoringPolicyAction: (props: any) => (
    <button data-platform-type={props.platformType}>Resource monitoring policy</button>
  ),
}));
vi.mock('../AlertSnoozeAction', () => ({ AlertSnoozeAction: () => null }));
import { AlertOverviewAlertCard } from '../AlertOverviewAlertCard';

const acknowledge = vi.fn();
const toggleTimeline = vi.fn();
const state = {
  tick: () => Date.now(),
  processingAlerts: () => new Set(),
  snoozeProcessingAlerts: () => new Set(),
  deliveryDiagnoses: () => ({}),
  handleAlertAcknowledgement: acknowledge,
} as unknown as AlertOverviewState;
const timelineState = {
  expandedIncidents: () => new Set(),
  toggleIncidentTimeline: toggleTimeline,
} as unknown as AlertIncidentTimelineState;
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('Pulse system-alert overview scope', () => {
  it.each(SYSTEM_ALERT_TYPES)('does not link %s to a fictional Proxmox resource', (type) => {
    const alert = makeSystemAlert(type);
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Pulse').closest('a')).toBeNull();
    expect(screen.getByText(alert.message)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'More' }));
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Acknowledge' }));
    expect(acknowledge).toHaveBeenCalledWith(alert);
    fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));
    expect(toggleTimeline).toHaveBeenCalledWith(alert.id, alert.id, alert.startTime);
  });

  it('ignores conflicting metric and resource hints on a marked system alert', () => {
    const alert = makeSystemAlert('future-system-condition', {
      id: 'legacy-id',
      resourceId: 'vm-wrong',
      threshold: 80,
      metadata: { systemAlert: true, resourceType: 'vm' },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Pulse').closest('a')).toBeNull();
    expect(screen.queryByText(/limit:/)).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'More' }));
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
  });

  it('preserves the actual link and metric limit for a monitored resource called Pulse', () => {
    const alert = makeSystemAlert('cpu', {
      id: 'resource-cpu',
      resourceId: 'vm-pulse',
      value: 92,
      threshold: 80,
      metadata: undefined,
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByRole('link', { name: 'Pulse' })).toHaveAttribute(
      'href',
      '/proxmox/overview',
    );
    expect(screen.getByText('limit: 80%')).toBeInTheDocument();
    // Monitoring policy is a secondary action behind the More disclosure.
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'More' }));
    expect(screen.getByText('Resource monitoring policy')).toBeInTheDocument();
  });

  it('links a vCenter alarm on a network to vSphere by its incident provider', () => {
    // The summary is the alarm's own name, so the message no longer says VMware.
    const alert = makeSystemAlert('resource-incident', {
      id: 'vmware-network-alarm',
      resourceId: 'network:vc-1:network-302',
      resourceName: 'Edge Stateful',
      message: 'Network packet loss above threshold',
      metadata: {
        resourceType: 'network',
        incidentProvider: 'vmware',
        incidentCode: 'vmware_alarm_state',
      },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByRole('link', { name: 'Edge Stateful' })).toHaveAttribute(
      'href',
      '/vmware/overview',
    );
  });

  it('links a vCenter health signal on an ESXi host to vSphere, not Machines', () => {
    // ESXi hosts are canonical agent resources the Machines page does not list.
    const alert = makeSystemAlert('resource-incident', {
      id: 'vmware-host-health',
      resourceId: 'agent:vc-1:host-107',
      resourceName: 'esxi-07.lab.local',
      message: 'vCenter health is yellow',
      metadata: {
        resourceType: 'agent',
        incidentProvider: 'vmware',
        incidentCode: 'vmware_health_state',
      },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByRole('link', { name: 'esxi-07.lab.local' })).toHaveAttribute(
      'href',
      '/vmware/overview',
    );
  });

  it.each([
    {
      name: 'a vCenter alarm on a VM whose message does not say VMware',
      resourceId: 'vm:vc-1:vm-2041',
      resourceName: 'sql-prod-01',
      message: 'Virtual machine memory usage on sql-prod-01 (red)',
      metadata: {
        resourceType: 'vm',
        incidentProvider: 'vmware',
        incidentCode: 'vmware_alarm_state',
        resourceSources: ['vmware'],
      },
      href: '/vmware/overview',
      platformType: 'vmware-vsphere',
    },
    {
      name: 'a TrueNAS pool incident',
      resourceId: 'storage:truenas-1:tank',
      resourceName: 'tank',
      message: 'Pool tank is degraded',
      metadata: {
        resourceType: 'storage',
        incidentProvider: 'truenas',
        incidentCode: 'pool_degraded',
        resourceSources: ['truenas'],
      },
      href: '/truenas/overview',
      platformType: 'truenas',
    },
    {
      name: 'a PBS datastore incident whose provider is Pulse itself',
      resourceId: 'storage:pbs-1:main',
      resourceName: 'main',
      message: 'Datastore main is nearly full',
      metadata: {
        resourceType: 'storage',
        incidentProvider: 'pulse',
        incidentCode: 'capacity_runway_low',
        resourceSources: ['pbs'],
      },
      href: '/proxmox/overview',
      platformType: 'proxmox-pbs',
    },
    {
      // Metric alerts carry a display label in resourceType; the backend
      // stamps the canonical platform beside it.
      name: 'a Kubernetes pod metric alert',
      resourceId: 'k8s:prod/ns:default/pod:api-7d9f',
      resourceName: 'api-7d9f',
      message: 'Kubernetes Pod api-7d9f disk at 93%',
      metadata: { resourceType: 'Kubernetes Pod', platformType: 'kubernetes' },
      href: '/kubernetes/overview',
      platformType: 'kubernetes',
    },
    {
      name: 'a vSphere VM metric alert',
      resourceId: 'vm:vc-1:vm-2041',
      resourceName: 'sql-prod-01',
      message: 'vSphere VM sql-prod-01 CPU at 95%',
      metadata: { resourceType: 'vSphere VM', platformType: 'vmware-vsphere' },
      href: '/vmware/overview',
      platformType: 'vmware-vsphere',
    },
    {
      name: 'a TrueNAS connection alert',
      resourceId: 'truenas:nas',
      resourceName: 'NAS',
      message: "Connection 'NAS' is unreachable",
      metadata: { resourceType: 'connection', connectionType: 'truenas', platformType: 'truenas' },
      href: '/truenas/overview',
      platformType: 'truenas',
    },
  ])(
    'links $name to its platform page and policy owner',
    ({ resourceId, resourceName, message, metadata, href, platformType }) => {
      const alert = makeSystemAlert('resource-incident', {
        id: resourceId,
        resourceId,
        resourceName,
        message,
        metadata,
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      expect(screen.getByRole('link', { name: resourceName })).toHaveAttribute('href', href);
      fireEvent.click(screen.getByRole('button', { name: 'More' }));
      expect(screen.getByText('Resource monitoring policy')).toHaveAttribute(
        'data-platform-type',
        platformType,
      );
    },
  );

  it.each([
    {
      name: 'an agent host metric alert to Machines',
      resourceId: 'agent:host-12',
      resourceType: 'agent',
      message: 'Usage at 92%',
      href: '/standalone/machines',
      platformType: 'agent',
    },
    {
      name: 'a TrueNAS pool metric alert to TrueNAS',
      resourceId: 'truenas-1:pool:tank',
      resourceType: 'truenas-pool',
      message: 'Usage at 92%',
      href: '/truenas/overview',
      platformType: 'truenas',
    },
    {
      // Guest ids embed the cluster and node names the user chose.
      name: 'a Proxmox VM alert on a node named docker-01 to Proxmox',
      resourceId: 'docker-01:docker-01:100',
      resourceType: 'vm',
      message: "VM 'billing-db-01' is powered off",
      href: '/proxmox/overview',
      platformType: 'proxmox',
    },
    {
      name: 'a Proxmox VM alert in a cluster named agent to Proxmox',
      resourceId: 'agent:pve1:100',
      resourceType: 'VM',
      message: 'VM CPU at 95%',
      href: '/proxmox/overview',
      platformType: 'proxmox',
    },
    {
      name: 'a Proxmox VM alert for a guest named vmware-test to Proxmox',
      resourceId: 'lab:pve1:101',
      resourceType: 'vm',
      message: "VM 'vmware-test' is powered off",
      href: '/proxmox/overview',
      platformType: 'proxmox',
    },
    {
      name: 'a Docker container alert to Docker by its type alone',
      resourceId: 'app-container-7c1e',
      resourceType: 'app-container',
      message: "Docker container 'nginx' is Paused",
      href: '/docker/overview',
      platformType: 'docker',
    },
  ])('links $name when metadata names no platform', (example) => {
    const alert = makeSystemAlert('usage', {
      id: example.resourceId,
      resourceId: example.resourceId,
      resourceName: 'monitored-resource',
      message: example.message,
      metadata: { resourceType: example.resourceType },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByRole('link', { name: 'monitored-resource' })).toHaveAttribute(
      'href',
      example.href,
    );
    fireEvent.click(screen.getByRole('button', { name: 'More' }));
    expect(screen.getByText('Resource monitoring policy')).toHaveAttribute(
      'data-platform-type',
      example.platformType,
    );
  });

  it('closes the open timeline when the disclosure is collapsed', () => {
    const alert = makeSystemAlert('cpu', { id: 'open-timeline', resourceId: 'vm-pulse' });
    const openTimeline = {
      expandedIncidents: () => new Set(['open-timeline']),
      toggleIncidentTimeline: toggleTimeline,
      incidentLoading: () => ({}),
      incidentErrors: () => ({}),
      incidentTimelines: () => ({}),
      eventFilters: () => new Set(),
      setEventFilters: vi.fn(),
      incidentNoteDrafts: () => ({}),
      incidentNoteSaving: () => new Set(),
    } as unknown as AlertIncidentTimelineState;
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={openTimeline} />
    ));
    // An open timeline keeps the disclosure open, so the control reads Less
    // and must close the timeline rather than do nothing.
    const less = screen.getByRole('button', { name: 'Less' });
    expect(less).toHaveAttribute('aria-expanded', 'true');
    fireEvent.click(less);
    expect(toggleTimeline).toHaveBeenCalledWith('open-timeline', 'open-timeline', alert.startTime);
  });
});
