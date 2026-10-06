import { cleanup, fireEvent, render, screen, within } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { VsphereActivityTable } from '@/features/vmware/VsphereActivityTable';
import { buildVmwareActivityRows, filterVmwareActivity } from '@/features/vmware/vmwarePageModel';
import type { Resource } from '@/types/resource';

const makeVm = (overrides: Partial<Resource> = {}): Resource =>
  ({
    id: 'vm-201',
    type: 'vm',
    name: 'warehouse-api-01',
    displayName: 'warehouse-api-01',
    status: 'online',
    platformType: 'vmware-vsphere',
    platformScopes: ['vmware-vsphere'],
    sourceType: 'api',
    vmware: {
      entityType: 'vm',
      managedObjectId: 'vm-201',
      connectionName: 'lab-vcenter',
      vcenterHost: 'vcsa.lab.local',
      datacenterName: 'Primary DC',
      clusterName: 'Production Cluster',
    },
    recentChanges: [
      {
        id: 'activity-task-reconfigure',
        observedAt: '2026-05-21T10:15:00Z',
        occurredAt: '2026-05-21T10:15:00Z',
        resourceId: 'vm-201',
        kind: 'activity',
        sourceType: 'platform_event',
        sourceAdapter: 'vmware_adapter',
        confidence: 'high',
        reason: 'Reconfigure virtual machine (error)',
        metadata: {
          activity_type: 'vmware_task',
          activity_native_id: 'task-901',
          activity_title: 'Reconfigure virtual machine',
          activity_state: 'error',
          activity_message: 'Permission denied while reconfiguring VM',
          vmwareTaskDescription: 'Reconfigure virtual machine CPU reservation',
          vmwareManagedObjectId: 'vm-201',
          vmwareEntityType: 'vm',
        },
      },
      {
        id: 'activity-event-powered-on',
        observedAt: '2026-05-21T10:05:00Z',
        occurredAt: '2026-05-21T10:05:00Z',
        resourceId: 'vm-201',
        kind: 'activity',
        sourceType: 'platform_event',
        sourceAdapter: 'vmware_adapter',
        confidence: 'high',
        actor: 'administrator@vsphere.local',
        reason: 'Virtual machine warehouse-api-01 was powered on',
        metadata: {
          activity_type: 'vmware_event',
          activity_native_id: 'event-501',
          // The provider titles an event with vCenter's message, keeping the
          // event class in vmwareEventType.
          activity_title: 'Virtual machine warehouse-api-01 was powered on',
          activity_message: 'Virtual machine warehouse-api-01 was powered on',
          vmwareEventType: 'VmPoweredOnEvent',
          vmwareEventMessage: 'Virtual machine warehouse-api-01 was powered on',
          vmwareEventUser: 'administrator@vsphere.local',
          vmwareManagedObjectId: 'vm-201',
          vmwareEntityType: 'vm',
        },
      },
    ],
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('VsphereActivityTable', () => {
  it('renders native vSphere activity rows with inline details', async () => {
    const activity = buildVmwareActivityRows([makeVm()]);

    render(() => (
      <VsphereActivityTable
        activity={activity}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
        showToolbar={false}
      />
    ));

    const table = screen.getByRole('table');
    expect(within(table).getByText('Resource')).toBeInTheDocument();
    expect(within(table).getByText('Type')).toBeInTheDocument();
    expect(within(table).getByText('Activity')).toBeInTheDocument();
    expect(within(table).getByText('State')).toBeInTheDocument();
    expect(within(table).getByText('Actor')).toBeInTheDocument();
    expect(within(table).getByText('vCenter')).toBeInTheDocument();
    expect(screen.getAllByText('warehouse-api-01')).toHaveLength(2);
    expect(screen.getByText('Reconfigure virtual machine')).toBeInTheDocument();
    expect(
      screen.getByTitle('Reconfigure virtual machine · Permission denied while reconfiguring VM'),
    ).toBeInTheDocument();
    expect(screen.getByText('Error')).toBeInTheDocument();
    expect(screen.getByText('Virtual machine warehouse-api-01 was powered on')).toBeInTheDocument();
    expect(screen.queryByText('VmPoweredOnEvent')).not.toBeInTheDocument();
    expect(screen.getByText('administrator@vsphere.local')).toBeInTheDocument();

    const row = screen.getByText('Reconfigure virtual machine').closest('tr');
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );

    await fireEvent.click(row!);

    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute('aria-expanded', 'true');
    const detail = within(screen.getByTestId('vsphere-activity-detail'));
    expect(detail.getByText('vSphere activity detail')).toBeInTheDocument();
    expect(detail.getByText('Managed object')).toBeInTheDocument();
    expect(detail.getAllByText('vm-201').length).toBeGreaterThan(0);
    expect(detail.getByText('Native ID')).toBeInTheDocument();
    expect(detail.getByText('task-901')).toBeInTheDocument();
    expect(detail.getByText('Description')).toBeInTheDocument();
    expect(detail.getByText('Reconfigure virtual machine CPU reservation')).toBeInTheDocument();

    await fireEvent.click(detail.getByRole('button', { name: /^Collapse .* details$/ }));

    expect(screen.queryByTestId('vsphere-activity-detail')).not.toBeInTheDocument();
    expect(row).not.toHaveAttribute('aria-expanded');
    expect(row?.querySelector('[data-row-action="true"]')).toHaveAttribute(
      'aria-expanded',
      'false',
    );
  });
  it("reads an event as vCenter's message and its time as an age", async () => {
    const vm = makeVm();
    const unset = {
      ...vm.recentChanges![1],
      id: 'activity-event-unset-time',
      observedAt: '1970-01-01T00:00:00Z',
      occurredAt: '1970-01-01T00:00:00Z',
      reason: 'Alarm status changed on warehouse-api-01',
      metadata: {
        ...vm.recentChanges![1].metadata,
        activity_native_id: 'event-502',
        // Stored before the provider titled events with vCenter's message.
        activity_title: 'AlarmStatusChangedEvent',
        activity_message: 'Alarm status changed on warehouse-api-01',
        vmwareEventType: 'AlarmStatusChangedEvent',
        vmwareEventMessage: 'Alarm status changed on warehouse-api-01',
      },
    };
    const activity = buildVmwareActivityRows([
      makeVm({ recentChanges: [...vm.recentChanges!, unset] }),
    ]);

    render(() => (
      <VsphereActivityTable
        activity={activity}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
        showToolbar={false}
      />
    ));

    // Searching by the event class still finds the row it no longer titles.
    expect(
      filterVmwareActivity(activity, 'VmPoweredOnEvent', 'all').map((row) => row.nativeId),
    ).toEqual(['event-501']);

    const powered = screen.getByText('Virtual machine warehouse-api-01 was powered on');
    // The hover title does not repeat the message the row already shows.
    expect(powered).toHaveAttribute(
      'title',
      'Virtual machine warehouse-api-01 was powered on · event-501',
    );
    const poweredRow = powered.closest('tr')!;
    const when = poweredRow.querySelector('td:last-child span[title]');
    expect(when?.textContent).toMatch(/ago$/);
    expect(when?.getAttribute('title')).toMatch(/2026/);

    const unsetRow = screen.getByText('Alarm status changed on warehouse-api-01').closest('tr')!;
    expect(unsetRow.querySelector('td:last-child')?.textContent).toBe('-');

    await fireEvent.click(poweredRow);
    const detail = within(screen.getByTestId('vsphere-activity-detail'));
    expect(detail.getByText('Event type')).toBeInTheDocument();
    expect(detail.getByText('VmPoweredOnEvent')).toBeInTheDocument();
    expect(detail.queryByText('Message')).not.toBeInTheDocument();
  });
  it('drops the vCenter column when one vCenter is connected', () => {
    render(() => (
      <VsphereActivityTable
        activity={buildVmwareActivityRows([makeVm()])}
        emptyIcon={<span />}
        emptyTitle="No activity"
        emptyDescription="No activity"
        showToolbar={false}
        singleVcenter
      />
    ));
    expect(
      within(screen.getByRole('table')).queryByRole('columnheader', { name: /vCenter/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText('lab-vcenter')).not.toBeInTheDocument();
  });
});
