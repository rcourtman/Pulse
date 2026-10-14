import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@solidjs/testing-library';
import type { AlertOverviewState } from '../useAlertOverviewState';
import type { AlertIncidentTimelineState } from '../useAlertIncidentTimelineState';
import { makeSystemAlert, SYSTEM_ALERT_TYPES } from '../__fixtures__/systemAlerts';
import { temperatureStore } from '@/utils/temperature';

vi.mock('@solidjs/router', () => ({
  A: (props: any) => (
    <a href={props.href} title={props.title}>
      {props.children}
    </a>
  ),
}));
vi.mock('@/components/Alerts/InvestigateAlertButton', () => ({
  InvestigateAlertButton: () => <button>Investigate</button>,
}));
vi.mock('../ResourceMonitoringPolicyAction', () => ({
  ResourceMonitoringPolicyAction: (props: any) => (
    <button data-platform-type={props.platformType}>Resource monitoring policy</button>
  ),
}));
vi.mock('../AlertSnoozeAction', () => ({ AlertSnoozeAction: () => <button>Snooze</button> }));
import { AlertOverviewAlertCard } from '../AlertOverviewAlertCard';

const acknowledge = vi.fn();
const toggleTimeline = vi.fn();
const state = {
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
    expect(screen.queryByText(/Alert level/)).toBeNull();
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
    expect(screen.getByText('Alert level 80%')).toBeInTheDocument();
    // Monitoring policy is a secondary action behind the More disclosure.
    expect(screen.queryByText('Resource monitoring policy')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'More' }));
    expect(screen.getByText('Resource monitoring policy')).toBeInTheDocument();
  });

  it('leads a threshold alert held under its trigger with the reading now', () => {
    // minipc on 2026-10-06: the card said "Node temperature at 80.0°C" while
    // the node read 72°C, because the message keeps the last breach.
    const now = Date.now();
    const alert = makeSystemAlert('temperature', {
      id: 'homelab-minipc::metric-threshold:temperature',
      resourceId: 'homelab-minipc',
      resourceName: 'minipc',
      node: 'minipc',
      message: 'Node temperature at 80.0°C',
      value: 80,
      threshold: 80,
      lastSeen: new Date(now - 5 * 60_000).toISOString(),
      metadata: { resourceType: 'node' },
      metricStatus: {
        phase: 'recovering',
        value: 72,
        unit: '°C',
        observedAt: new Date(now).toISOString(),
        trigger: 80,
        recovery: 75,
        recoveryDelaySeconds: 300,
        recoveryElapsedSeconds: 120,
      },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Temperature 72°C now, recovering')).toHaveAttribute(
      'title',
      expect.stringMatching(/^Last reading at or above 80°C: 80°C, /),
    );
    expect(
      screen.getByText('Clears after 5 minutes at 75°C or lower, 2 minutes so far.'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Node temperature at 80.0°C')).toBeNull();
    expect(screen.getByText('Alert level 80°C')).toBeInTheDocument();
    expect(screen.getByText('Clear level 75°C')).toBeInTheDocument();
  });

  it('labels a rate alert level in its own unit, not a guessed percent', () => {
    const alert = makeSystemAlert('diskWrite', {
      id: 'vm-101::metric-threshold:diskWrite',
      resourceId: 'vm-101',
      resourceName: 'db-01',
      message: 'VM diskWrite at 240.0 MB/s',
      value: 240,
      threshold: 200,
      metadata: { resourceType: 'vm' },
      metricStatus: {
        phase: 'breaching',
        value: 240,
        unit: 'MB/s',
        observedAt: new Date().toISOString(),
        trigger: 200,
        recovery: 200,
      },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Disk Write 240 MB/s, above the 200 MB/s alert level')).toBeVisible();
    expect(screen.getByText('Alert level 200 MB/s')).toBeInTheDocument();
    // Clearing at the trigger itself needs no second level.
    expect(screen.queryByText(/Clear level/)).toBeNull();
  });

  it('does not guess a percent for a threshold stated in its own message', () => {
    const alert = makeSystemAlert('queue-age', {
      id: 'pmg-eu::queue-age',
      resourceId: 'pmg-eu',
      resourceName: 'mail-gateway-eu',
      message: 'PMG mail-gateway-eu has messages queued for 57 minutes (threshold: 30 minutes)',
      value: 57,
      threshold: 30,
      metadata: { resourceType: 'pmg' },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText(alert.message)).toBeInTheDocument();
    expect(screen.queryByText(/Alert level/)).toBeNull();
  });

  it('keeps the level label for a percent threshold that has no live status', () => {
    const alert = makeSystemAlert('disk-wearout', {
      id: 'disk-wearout-nvme0',
      resourceId: 'host-1/disk:nvme0n1',
      resourceName: 'nvme0n1',
      message: 'SSD nvme0n1 has less than 10% life remaining',
      value: 7,
      threshold: 10,
      metadata: { resourceType: 'physical_disk' },
    });
    render(() => (
      <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
    ));
    expect(screen.getByText('Alert level 10%')).toBeInTheDocument();
  });

  it('shows a temperature level without a live status in the viewer unit', () => {
    // Right after a restart an alert has no live status until it is evaluated.
    temperatureStore.setUnit('fahrenheit');
    try {
      const alert = makeSystemAlert('temperature', {
        id: 'homelab-minipc::metric-threshold:temperature',
        resourceId: 'homelab-minipc',
        resourceName: 'minipc',
        message: 'Node temperature at 80.0°C',
        value: 80,
        threshold: 80,
        metadata: { resourceType: 'node' },
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      expect(screen.getByText('Alert level 176°F')).toBeInTheDocument();
    } finally {
      temperatureStore.setUnit('celsius');
    }
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
      // A guest alert also names the guest for the workloads table to open.
      href: '/vmware/overview?resource=vm%3Avc-1%3Avm-2041',
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
      // Storage resolves the datastore's id through the registry and opens it.
      href: '/proxmox/storage?resource=storage%3Apbs-1%3Amain',
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
      href: '/proxmox/overview?resource=docker-01%3Adocker-01%3A100',
      platformType: 'proxmox',
    },
    {
      name: 'a Proxmox VM alert in a cluster named agent to Proxmox',
      resourceId: 'agent:pve1:100',
      resourceType: 'VM',
      message: 'VM CPU at 95%',
      href: '/proxmox/overview?resource=agent%3Apve1%3A100',
      platformType: 'proxmox',
    },
    {
      name: 'a Proxmox VM alert for a guest named vmware-test to Proxmox',
      resourceId: 'lab:pve1:101',
      resourceType: 'vm',
      message: "VM 'vmware-test' is powered off",
      href: '/proxmox/overview?resource=lab%3Apve1%3A101',
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

  describe('action placement by width', () => {
    const desktopWidth = window.innerWidth;
    const setWidth = (width: number) =>
      Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        writable: true,
        value: width,
      });
    afterEach(() => setWidth(desktopWidth));

    it('keeps Snooze and Investigate on the card from the sm breakpoint up', () => {
      setWidth(640);
      const alert = makeSystemAlert('cpu', {
        id: 'wide-card',
        resourceId: 'vm-pulse',
        metadata: { resourceType: 'vm' },
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      expect(screen.getByRole('button', { name: 'Snooze' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Investigate' })).toBeInTheDocument();
      fireEvent.click(screen.getByRole('button', { name: 'More' }));
      expect(screen.getAllByRole('button', { name: 'Snooze' })).toHaveLength(1);
    });

    it('moves Investigate behind More on a phone and keeps Snooze on the row', () => {
      // A 375px phone showed about three alerts per screen while four actions
      // wrapped onto two rows under each one.
      setWidth(375);
      const alert = makeSystemAlert('cpu', {
        id: 'phone-card',
        resourceId: 'vm-pulse',
        metadata: { resourceType: 'vm' },
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      expect(screen.getByRole('button', { name: 'Acknowledge' })).toBeInTheDocument();
      const snooze = screen.getByRole('button', { name: 'Snooze' });
      expect(screen.queryByRole('button', { name: 'Investigate' })).toBeNull();
      const more = screen.getByRole('button', { name: 'More' });
      expect(more).toHaveAttribute('aria-expanded', 'false');
      // A collapsed disclosure renders no region, so it names none.
      expect(more).not.toHaveAttribute('aria-controls');
      fireEvent.click(more);
      expect(more).toHaveAttribute('aria-expanded', 'true');
      const panel = document.getElementById(more.getAttribute('aria-controls') ?? '');
      expect(panel).not.toBeNull();
      for (const name of ['Investigate', 'Timeline', 'Resource monitoring policy']) {
        expect(panel).toContainElement(screen.getByRole('button', { name }));
      }
      expect(panel).not.toContainElement(snooze);
      expect(screen.getAllByRole('button', { name: 'Snooze' })).toHaveLength(1);
    });

    it('moves the actions when a mounted card crosses the breakpoint', async () => {
      // Rotating a phone crosses sm without remounting the overview.
      setWidth(375);
      const alert = makeSystemAlert('cpu', {
        id: 'rotating-card',
        resourceId: 'vm-pulse',
        metadata: { resourceType: 'vm' },
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      expect(screen.queryByRole('button', { name: 'Investigate' })).toBeNull();
      // Snooze owns its dialog state, so it must survive the crossing as the
      // same element.
      const snooze = screen.getByRole('button', { name: 'Snooze' });
      setWidth(800);
      window.dispatchEvent(new Event('resize'));
      await waitFor(() =>
        expect(screen.getByRole('button', { name: 'Investigate' })).toBeInTheDocument(),
      );
      expect(screen.getByRole('button', { name: 'Snooze' })).toBe(snooze);
      setWidth(375);
      window.dispatchEvent(new Event('resize'));
      await waitFor(() => expect(screen.queryByRole('button', { name: 'Investigate' })).toBeNull());
      expect(screen.getByRole('button', { name: 'Snooze' })).toBe(snooze);
    });
  });

  describe('resource links', () => {
    const linkFor = (overrides: Parameters<typeof makeSystemAlert>[1]) => {
      const alert = makeSystemAlert('powered-off', {
        id: `${overrides?.resourceId}::powered-state`,
        ...overrides,
      });
      render(() => (
        <AlertOverviewAlertCard alert={alert} state={state} timelineState={timelineState} />
      ));
      return screen.getByRole('link', { name: alert.resourceName }).getAttribute('href');
    };

    it('opens the Proxmox guest a VM or container alert is about', () => {
      // The workloads table resolves the alert's source id through the
      // resource registry, opens that guest and scrolls to it.
      expect(
        linkFor({
          resourceId: 'mock-cluster-1-pve1-105',
          resourceName: 'postgres-replica-01',
          metadata: { resourceType: 'vm' },
        }),
      ).toBe('/proxmox/overview?resource=mock-cluster-1-pve1-105');
      cleanup();
      expect(
        linkFor({
          resourceId: 'mock-cluster-1-pve2-121',
          resourceName: 'dev-portal-01',
          metadata: { resourceType: 'container' },
        }),
      ).toBe('/proxmox/overview?resource=mock-cluster-1-pve2-121');
    });

    it('opens a vSphere VM on the vSphere page', () => {
      expect(
        linkFor({
          resourceId: 'vm-vsphere-42',
          resourceName: 'web-01',
          metadata: { resourceType: 'vm', platformType: 'vmware-vsphere' },
        }),
      ).toBe('/vmware/overview?resource=vm-vsphere-42');
    });

    it('opens the guest a filesystem alert carries', () => {
      // The alert's own `<guest>-disk-<key>` id names no row; the backend
      // stamps the guest it belongs to.
      expect(
        linkFor({
          resourceId: 'mock-cluster-1-pve1-105-disk-root',
          resourceName: 'postgres-replica-01',
          metadata: {
            resourceType: 'vm',
            mountpoint: '/',
            diskIndex: 0,
            guestResourceId: 'mock-cluster-1-pve1-105',
          },
        }),
      ).toBe('/proxmox/overview?resource=mock-cluster-1-pve1-105');
    });

    it('keeps the platform page for a guest filesystem or an orphaned backup', () => {
      // Per-filesystem alerts raised before the backend stamped the owning
      // guest name only `<guest>-disk-<key>`, which no row answers to.
      expect(
        linkFor({
          resourceId: 'mock-cluster-1-pve1-105-disk-root',
          resourceName: 'postgres-replica-01 (/)',
          metadata: { resourceType: 'vm', mountpoint: '/', diskIndex: 0 },
        }),
      ).toBe('/proxmox/overview');
      cleanup();
      expect(
        linkFor({
          resourceId: 'backup-subject:pve:ct:404',
          resourceName: 'retired-ct',
          metadata: { resourceType: 'system-container' },
        }),
      ).toBe('/proxmox/overview');
    });

    it('opens the storage a ZFS pool or device alert names', () => {
      // "<storage>/zfs-pool:..." ids name nothing the registry can open.
      expect(
        linkFor({
          resourceId: 'pve5-local-zfs/zfs-pool:local-zfs/device:sdb2',
          resourceName: 'west-e-service-pool (local-zfs, sdb2)',
          metadata: { resourceType: 'storage', storageResourceId: 'pve5-local-zfs' },
        }),
      ).toBe('/proxmox/storage?resource=pve5-local-zfs');
      cleanup();
      // Raised before the stamp: the Storage page, without naming a pool.
      expect(
        linkFor({
          resourceId: 'pve5-local-zfs/zfs-pool:local-zfs',
          resourceName: 'west-e-service-pool (local-zfs)',
          metadata: { resourceType: 'storage' },
        }),
      ).toBe('/proxmox/storage');
    });

    it('opens a storage pool on Storage and keeps Docker on its platform page', () => {
      expect(
        linkFor({
          resourceId: 'docker:nebula-1-mock/nebula-1-c94124aef15b',
          resourceName: 'grafana-agent',
          metadata: { resourceType: 'app-container' },
        }),
      ).toBe('/docker/overview');
      cleanup();
      expect(
        linkFor({
          resourceId: 'mock-cluster-1-pve1-local',
          resourceName: 'west-a-iso-library',
          metadata: { resourceType: 'storage' },
        }),
      ).toBe('/proxmox/storage?resource=mock-cluster-1-pve1-local');
    });
  });
});
