import { cleanup, render, screen } from '@solidjs/testing-library';
import { afterEach, describe, expect, it } from 'vitest';

import { TrueNASVirtualMachinesTable } from '@/features/truenas/TrueNASVirtualMachinesTable';
import type { Resource } from '@/types/resource';

const makeVM = (
  id: string,
  vm: NonNullable<NonNullable<Resource['truenas']>['vm']>,
  overrides: Partial<Resource> = {},
): Resource =>
  ({
    id,
    type: 'vm',
    name: id,
    displayName: id,
    status: vm.state === 'RUNNING' ? 'online' : 'offline',
    platformType: 'truenas',
    platformScopes: ['truenas'],
    truenas: { vm: { name: id, ...vm } },
    ...overrides,
  }) as Resource;

afterEach(() => {
  cleanup();
});

describe('TrueNASVirtualMachinesTable', () => {
  it('flags only the stopped VMs nobody chose to stop and keeps configuration in the drawer', () => {
    const vms = [
      makeVM('windows-lab', {
        state: 'RUNNING',
        autostart: true,
        bootloader: 'UEFI',
        diskCount: 1,
        nicCount: 1,
      }),
      makeVM('ubuntu-build', { state: 'STOPPED', bootloader: 'UEFI', diskCount: 1 }),
      makeVM('router', { state: 'STOPPED', autostart: true, bootloader: 'UEFI' }),
      makeVM('lab-paused', { state: 'PAUSED' }),
    ];
    const { container } = render(() => (
      <TrueNASVirtualMachinesTable
        vms={vms}
        scope={vms}
        emptyIcon={<span />}
        emptyTitle="No VMs"
        emptyDescription="No VMs"
        showToolbar={false}
      />
    ));

    const noteFor = (name: string) =>
      screen.getByText(name).closest('tr')?.querySelector('[data-truenas-vm-state-note]');

    expect(noteFor('router')).toHaveTextContent('Stoppedshould be running');
    expect(noteFor('router')).toHaveAttribute('title', 'Set to start at boot but stopped');
    // A phone hides State, so the state and its note join the name on its line.
    const routerPhone = screen
      .getByText('router')
      .closest('td')
      ?.querySelector('[data-truenas-vm-state-phone]');
    expect(routerPhone).toHaveTextContent('Stopped · should be running');
    expect(routerPhone).not.toHaveClass('sr-only');
    expect(routerPhone?.parentElement).toHaveClass('items-center');
    // A running VM reads in its green dot; screen readers keep the words.
    expect(
      screen.getByText('windows-lab').closest('td')?.querySelector('[data-truenas-vm-state-phone]'),
    ).toHaveClass('sr-only');
    expect(noteFor('ubuntu-build')).toBeNull();
    expect(noteFor('windows-lab')).toBeNull();
    expect(noteFor('lab-paused')).toHaveAttribute('title', 'TrueNAS reports Paused');

    const headers = [...container.querySelectorAll('thead th')].map((th) => th.textContent?.trim());
    expect(headers).not.toContain('Boot');
    expect(headers).not.toContain('Devices');
    expect(container.querySelector('tbody')).not.toHaveTextContent('UEFI');
  });

  it('surfaces top-level sourceStatus staleness on VMs and clears it on live recovery', () => {
    const vms = [
      makeVM('stale-vm', { state: 'RUNNING' }, { sourceStatus: { truenas: { status: 'stale' } } }),
      makeVM(
        'recovered-vm',
        { state: 'RUNNING' },
        {
          sourceStatus: { truenas: { status: 'online' } },
          platformData: { sourceStatus: { truenas: { status: 'stale' } } },
        },
      ),
    ];

    render(() => (
      <TrueNASVirtualMachinesTable
        vms={vms}
        scope={vms}
        emptyIcon={<span />}
        emptyTitle="No VMs"
        emptyDescription="No VMs"
        showToolbar={false}
      />
    ));

    const staleNote = screen
      .getByText('stale-vm')
      .closest('tr')
      ?.querySelector('[data-truenas-vm-state-note]');
    expect(staleNote).toHaveTextContent('Runningnot updated');
    expect(staleNote).toHaveAttribute('title', 'TrueNAS has not updated this recently');

    const recoveredNote = screen
      .getByText('recovered-vm')
      .closest('tr')
      ?.querySelector('[data-truenas-vm-state-note]');
    expect(recoveredNote).toBeNull();
  });
});
